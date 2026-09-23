package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func advertisedCapabilities(t *testing.T) contracts.Capabilities {
	t.Helper()

	var body []byte
	for _, cookie := range []string{"", strings.Repeat("a", 64)} {
		request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
		if cookie != "" {
			request.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
		}
		response := httptest.NewRecorder()
		api{statisticsEnabled: true}.capabilities(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("status = %d, content type = %q", response.Code, response.Header().Get("Content-Type"))
		}
		if cookies := response.Result().Cookies(); len(cookies) != 0 {
			t.Fatalf("capabilities set session cookies: %v", cookies)
		}
		if body != nil && !bytes.Equal(body, response.Body.Bytes()) {
			t.Fatalf("capabilities changed with session cookie: %s != %s", body, response.Body.Bytes())
		}
		body = response.Body.Bytes()
	}

	var advertised contracts.Capabilities
	if err := json.Unmarshal(body, &advertised); err != nil {
		t.Fatal(err)
	}
	return advertised
}

func TestCapabilitiesAdvertiseExecutableCore(t *testing.T) {
	advertised := advertisedCapabilities(t)
	engine := calculation.New()
	evaluate := func(expression, want string, unit contracts.AngleUnit) {
		t.Helper()
		result, err := engine.Evaluate(t.Context(), calculation.Input{Expression: expression, AngleUnit: unit})
		if err != nil || result.Outcome.Kind != "success" || result.Outcome.Value != want {
			t.Errorf("%q (%s) = %+v, err %v; want %s", expression, unit, result.Outcome, err, want)
		}
	}

	operators := map[string]struct{ expression, want string }{
		"+": {"2+3", "5"}, "-": {"2-3", "-1"}, "*": {"2*3", "6"},
		"/": {"6/3", "2"}, "^": {"2^3", "8"},
	}
	for _, operator := range advertised.Operators {
		example, ok := operators[operator]
		if !ok {
			t.Fatalf("no example for advertised operator %q", operator)
		}
		evaluate(example.expression, example.want, advertised.DefaultAngleUnit)
	}

	type example struct {
		args []string
		want string
	}
	functions := map[string][]example{
		"sqrt": {{[]string{"9"}, "3"}},
		"abs":  {{[]string{"-3"}, "3"}},
		"exp":  {{[]string{"0"}, "1"}},
		"ln":   {{[]string{"e"}, "1"}},
		"log":  {{[]string{"100"}, "2"}, {[]string{"8", "2"}, "3"}},
		"sin":  {{[]string{"90"}, "1"}},
		"cos":  {{[]string{"0"}, "1"}},
		"tan":  {{[]string{"0"}, "0"}},
		"asin": {{[]string{"1"}, "90"}},
		"acos": {{[]string{"1"}, "0"}},
		"atan": {{[]string{"1"}, "45"}},
	}
	for function, arities := range advertised.Functions {
		examples := functions[function]
		if len(examples) != len(arities) {
			t.Fatalf("advertised %q arities %v have no matching examples", function, arities)
		}
		for i, arity := range arities {
			if len(examples[i].args) != arity {
				t.Fatalf("%q advertises arity %d, example has %d", function, arity, len(examples[i].args))
			}
			evaluate(function+"("+strings.Join(examples[i].args, ",")+")", examples[i].want, advertised.DefaultAngleUnit)
		}
	}
	evaluate("sin(pi/2)", "1", contracts.Radians)
}

func TestCapabilitiesAdvertisedBudgetsMatchEngineBoundaries(t *testing.T) {
	limits := advertisedCapabilities(t).Limits
	engine := calculation.New()

	tokensAtLimit := "+" + strings.Repeat("1+", (limits.Tokens-2)/2) + "1"
	for _, tc := range []struct {
		name, accepted, rejected string
	}{
		{"UTF-16 length",
			"1" + strings.Repeat(" ", limits.ExpressionLength-1),
			"1" + strings.Repeat(" ", limits.ExpressionLength)},
		{"tokens", tokensAtLimit, "+" + tokensAtLimit},
		{"nesting",
			strings.Repeat("(", limits.Nesting) + "1" + strings.Repeat(")", limits.Nesting),
			strings.Repeat("(", limits.Nesting+1) + "1" + strings.Repeat(")", limits.Nesting+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Evaluate(t.Context(), calculation.Input{Expression: tc.accepted, AngleUnit: contracts.Degrees})
			if err != nil || result.Outcome.Kind != "success" {
				t.Fatalf("accepted boundary: outcome %+v, err %v", result.Outcome, err)
			}
			_, err = engine.Evaluate(t.Context(), calculation.Input{Expression: tc.rejected, AngleUnit: contracts.Degrees})
			if !errors.Is(err, calculation.ErrExpressionLimit) {
				t.Errorf("past boundary: err %v, want ErrExpressionLimit", err)
			}
		})
	}

	// A supplementary character uses two UTF-16 units, not four UTF-8 bytes.
	atUTF16Limit := "1" + strings.Repeat(" ", limits.ExpressionLength-3) + "😀"
	result, err := engine.Evaluate(t.Context(), calculation.Input{Expression: atUTF16Limit, AngleUnit: contracts.Degrees})
	if err != nil || result.Outcome.Error == nil || result.Outcome.Error.Code != "SYNTAX_ERROR" {
		t.Errorf("UTF-16 boundary: outcome %+v, err %v; want syntax error, not length limit", result.Outcome, err)
	}
	_, err = engine.Evaluate(t.Context(), calculation.Input{Expression: " " + atUTF16Limit, AngleUnit: contracts.Degrees})
	if !errors.Is(err, calculation.ErrExpressionLimit) {
		t.Errorf("past UTF-16 boundary: err %v, want ErrExpressionLimit", err)
	}
}
