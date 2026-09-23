package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const maxAPIResponseBytes = 1 << 20

var errAPIResponseLimit = errors.New("API response limit exceeded")

// One guard must wrap the entire API router so every API path shares its
// admission and rate budgets. Health checks and static files stay outside it.
func protectAPI(next http.Handler) http.Handler {
	return newAPIGuard(apiLimitConfig{
		rate: 50, burst: 100, active: 64, timeout: 10 * time.Second, now: time.Now,
	}).wrap(next)
}

type apiLimitConfig struct {
	rate    float64
	burst   int
	active  int
	timeout time.Duration
	now     func() time.Time
}

type apiGuard struct {
	config apiLimitConfig
	active chan struct{}
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newAPIGuard(config apiLimitConfig) *apiGuard {
	return &apiGuard{
		config: config, active: make(chan struct{}, config.active),
		tokens: float64(config.burst), last: config.now(),
	}
}

// retryAfter consumes one token only for an admitted request. Rejected work
// never reaches the session or history handlers.
func (g *apiGuard) retryAfter() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.config.now()
	if elapsed := now.Sub(g.last); elapsed > 0 {
		g.tokens = math.Min(float64(g.config.burst), g.tokens+elapsed.Seconds()*g.config.rate)
		g.last = now
	}
	if g.tokens < 1 {
		return int(math.Ceil((1 - g.tokens) / g.config.rate))
	}
	g.tokens--
	return 0
}

func (g *apiGuard) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := func(status int, category string) {
			log.Printf("api status=%d category=%s duration=%s", status, category, time.Since(start).Round(time.Millisecond))
		}
		select {
		case g.active <- struct{}{}:
		default:
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			record(http.StatusServiceUnavailable, "saturated")
			return
		}
		if retry := g.retryAfter(); retry > 0 {
			<-g.active
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			apiError(w, http.StatusTooManyRequests, "RATE_LIMITED")
			record(http.StatusTooManyRequests, "rate_limited")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), g.config.timeout)
		defer cancel()
		finished := make(chan guardedResponse, 1)
		go func() {
			response := guardedResponse{failure: "panic"}
			defer func() {
				if recover() != nil {
					response = guardedResponse{failure: "panic"}
				}
				// A timed-out handler may still be unwinding. Its slot stays held
				// until it actually exits, even after the client gets a 503.
				<-g.active
				finished <- response
			}()
			buffer := &bufferedAPIResponse{header: make(http.Header)}
			next.ServeHTTP(buffer, r.WithContext(ctx))
			if buffer.overflow {
				response.failure = "response_limit"
			} else {
				response = guardedResponse{writer: buffer}
			}
		}()

		select {
		case response := <-finished:
			if ctx.Err() != nil {
				apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
				record(http.StatusServiceUnavailable, contextCategory(ctx.Err()))
				return
			}
			if response.failure != "" {
				status := http.StatusServiceUnavailable
				if response.failure == "panic" {
					status = http.StatusInternalServerError
				}
				apiError(w, status, "SERVICE_UNAVAILABLE")
				record(status, response.failure)
				return
			}
			status := response.writer.writeTo(w)
			category := "ok"
			if status >= 500 {
				category = "service_error"
			} else if status >= 400 {
				category = "client_error"
			}
			record(status, category)
		case <-ctx.Done():
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			record(http.StatusServiceUnavailable, contextCategory(ctx.Err()))
		}
	})
}

func contextCategory(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "cancelled"
}

type guardedResponse struct {
	writer  *bufferedAPIResponse
	failure string
}

type bufferedAPIResponse struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	overflow bool
}

func (b *bufferedAPIResponse) Header() http.Header { return b.header }

func (b *bufferedAPIResponse) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferedAPIResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	if b.overflow || len(p) > maxAPIResponseBytes-b.body.Len() {
		b.overflow = true
		return 0, errAPIResponseLimit
	}
	return b.body.Write(p)
}

func (b *bufferedAPIResponse) writeTo(w http.ResponseWriter) int {
	for name, values := range b.header {
		w.Header()[name] = values
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = b.body.WriteTo(w)
	return status
}

// The HTTP server's default error logger includes raw panic values and request
// addresses. Keep its diagnostic category without printing those values.
type safeServerLog struct{}

func (safeServerLog) Write(p []byte) (int, error) {
	log.Print("http category=server_error")
	return len(p), nil
}

// Acquire before Accept: the process cannot hold more than max accepted
// connections, including idle and hijacked connections. Closing the listener
// wakes Accept even when all slots are occupied during shutdown.
func limitConnections(listener net.Listener, max int) net.Listener {
	return &boundedListener{Listener: listener, active: make(chan struct{}, max), closed: make(chan struct{})}
}

type boundedListener struct {
	net.Listener
	active chan struct{}
	closed chan struct{}
	once   sync.Once
}

func (l *boundedListener) Accept() (net.Conn, error) {
	select {
	case <-l.closed:
		return nil, net.ErrClosed
	case l.active <- struct{}{}:
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		<-l.active
		return nil, err
	}
	return &boundedConn{Conn: conn, release: func() { <-l.active }}, nil
}

func (l *boundedListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.closed)
		err = l.Listener.Close()
	})
	return err
}

type boundedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *boundedConn) Close() error {
	var err error
	c.once.Do(func() {
		err = c.Conn.Close()
		c.release()
	})
	return err
}
