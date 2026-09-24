package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithClientRoutesAndAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":           "<!doctype html><title>Client</title>",
		"assets/client.js":     "console.log('real asset')",
		".env":                 "secret",
		"assets/.private.json": "secret",
		"assets/private.js":    "public asset",
		"database.sqlite":      "secret",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, directory := range []string{"private", "data", "state"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, directory, "secret.txt"), []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "private"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}

	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Method", r.Method)
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, r.URL.Path)
	})
	handler, err := withClient(api, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method string
		path   string
		status int
		body   string
		api    bool
	}{
		{http.MethodPost, "/api", http.StatusAccepted, "/api", true},
		{http.MethodDelete, "/api/calculations", http.StatusAccepted, "/api/calculations", true},
		{http.MethodHead, "/health", http.StatusAccepted, "", true},
		{http.MethodGet, "/health/ready", http.StatusAccepted, "/health/ready", true},
		{http.MethodGet, "/", http.StatusOK, "<!doctype html><title>Client</title>", false},
		{http.MethodGet, "/index.html", http.StatusOK, "<!doctype html><title>Client</title>", false},
		{http.MethodHead, "/assets/client.js", http.StatusOK, "", false},
		{http.MethodGet, "/assets/client.js", http.StatusOK, "console.log('real asset')", false},
		{http.MethodGet, "/settings/history", http.StatusOK, "<!doctype html><title>Client</title>", false},
		{http.MethodGet, "/settings/", http.StatusOK, "<!doctype html><title>Client</title>", false},
		{http.MethodGet, "/assets/missing.js", http.StatusNotFound, "", false},
		{http.MethodGet, "/assets/missing", http.StatusNotFound, "", false},
		{http.MethodGet, "/assets/", http.StatusNotFound, "", false},
		{http.MethodGet, "/favicon.ico", http.StatusNotFound, "", false},
		{http.MethodGet, "/.env", http.StatusNotFound, "", false},
		{http.MethodGet, "/assets/.private.json", http.StatusNotFound, "", false},
		{http.MethodGet, "/private/secret.txt", http.StatusNotFound, "", false},
		{http.MethodGet, "/data/secret.txt", http.StatusNotFound, "", false},
		{http.MethodGet, "/state/secret.txt", http.StatusNotFound, "", false},
		{http.MethodGet, "/database.sqlite", http.StatusNotFound, "", false},
		{http.MethodGet, "/outside.txt", http.StatusNotFound, "", false},
		{http.MethodGet, "/alias/secret.txt", http.StatusNotFound, "", false},
		{http.MethodGet, "/../index.html", http.StatusNotFound, "", false},
		{http.MethodGet, "/%2e%2e/index.html", http.StatusNotFound, "", false},
		{http.MethodPost, "/settings", http.StatusNotFound, "", false},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %q)", response.Code, tc.status, response.Body.String())
			}
			if tc.body != "" && response.Body.String() != tc.body {
				t.Fatalf("body = %q, want %q", response.Body.String(), tc.body)
			}
			if tc.method == http.MethodHead && !tc.api && response.Body.Len() != 0 {
				t.Fatalf("HEAD asset returned a body: %q", response.Body.String())
			}
			if tc.status == http.StatusNotFound && strings.Contains(response.Body.String(), "secret") {
				t.Fatal("private content escaped into response")
			}
			if tc.api && response.Header().Get("X-API-Method") != tc.method {
				t.Fatal("API request was not delegated unchanged")
			}
		})
	}
}

func TestWithClientRequiresRealIndex(t *testing.T) {
	root := t.TempDir()
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if _, err := withClient(api, root); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("missing index error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "index.html"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := withClient(api, root); err == nil || !strings.Contains(err.Error(), "regular index.html") {
		t.Fatalf("directory index error = %v", err)
	}
}

func TestWithClientRejectsDatabaseAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<title>Client</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("asset request reached API handler")
	})
	handler, err := withClient(api, root)
	if err != nil {
		t.Fatal(err)
	}

	denied := []string{"database.sqlite-backup", "database.db-journal-notes"}
	for _, extension := range []string{".sqlite", ".sqlite3", ".db", ".db3"} {
		for _, sidecar := range []string{"", "-wal", "-shm", "-journal"} {
			denied = append(denied, "database"+extension+sidecar)
		}
	}
	for _, name := range denied {
		for _, name := range []string{name, strings.ToUpper(name)} {
			t.Run(name, func(t *testing.T) {
				if err := os.WriteFile(filepath.Join(root, "assets", name), []byte("database secret"), 0o600); err != nil {
					t.Fatal(err)
				}
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					request := httptest.NewRequest(method, "/assets/"+name, nil)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)
					if response.Code != http.StatusNotFound {
						t.Fatalf("%s status = %d, want 404 (body %q)", method, response.Code, response.Body.String())
					}
					if strings.Contains(response.Body.String(), "database secret") {
						t.Fatalf("%s exposed database content", method)
					}
				}
			})
		}
	}

	for _, name := range []string{
		"database.sqlite3-wal.js",
		"database.db3-journal.txt",
	} {
		t.Run(name, func(t *testing.T) {
			content := "ordinary asset: " + name
			if err := os.WriteFile(filepath.Join(root, "assets", name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/assets/"+name, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Body.String() != content {
				t.Fatalf("status = %d, body = %q; want 200 and %q", response.Code, response.Body.String(), content)
			}
		})
	}
}
