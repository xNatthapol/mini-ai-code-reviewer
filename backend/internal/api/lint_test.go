package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

type reviewerFunc func(context.Context, review.Request) (review.Result, error)

func (f reviewerFunc) Review(ctx context.Context, req review.Request) (review.Result, error) {
	return f(ctx, req)
}

func TestLintSuccess(t *testing.T) {
	for _, findings := range [][]review.Finding{
		{},
		{{StartLine: 1, EndLine: 2, Explanation: "The divisor can be zero.", SuggestedFix: "if divisor == 0:\n    return None"}},
	} {
		t.Run(fmt.Sprintf("%d findings", len(findings)), func(t *testing.T) {
			calls := 0
			type contextKey struct{}
			handler := NewHandler(reviewerFunc(func(ctx context.Context, req review.Request) (review.Result, error) {
				calls++
				if req != (review.Request{Code: "  x\n\n", Language: "python"}) {
					t.Errorf("unexpected request: %#v", req)
				}
				if ctx.Value(contextKey{}) != "request-value" {
					t.Error("request context not propagated")
				}
				return review.Result{Findings: findings}, nil
			}))
			req := httptest.NewRequest(http.MethodPost, "/lint", strings.NewReader(`{"code":"  x\r\n\r\n","language":"python"}`))
			req = req.WithContext(context.WithValue(req.Context(), contextKey{}, "request-value"))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if w.Header().Get("Content-Type") != "application/json" {
				t.Fatal("expected JSON content type")
			}
			var got review.Result
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Findings, findings) {
				t.Fatalf("got %#v, want %#v", got.Findings, findings)
			}
			if calls != 1 {
				t.Fatalf("reviewer called %d times", calls)
			}
		})
	}
}

func TestLintRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"empty body", "", 400, "invalid_request"},
		{"malformed", "{", 400, "invalid_request"},
		{"null", "null", 400, "invalid_request"},
		{"missing code", `{"language":"go"}`, 400, "invalid_request"},
		{"missing language", `{"code":"x"}`, 400, "invalid_request"},
		{"blank", `{"code":" \n","language":"go"}`, 400, "invalid_request"},
		{"unsupported", `{"code":"x","language":"js"}`, 400, "invalid_request"},
		{"wrong type", `{"code":42,"language":"go"}`, 400, "invalid_request"},
		{"unknown field", `{"code":"x","language":"go","extra":true}`, 400, "invalid_request"},
		{"second object", `{"code":"x","language":"go"}{}`, 400, "invalid_request"},
		{"trailing garbage", `{"code":"x","language":"go"}oops`, 400, "invalid_request"},
		{"source too large", `{"code":"` + strings.Repeat("x", 12289) + `","language":"go"}`, 400, "invalid_request"},
		{"too many lines", `{"code":"` + strings.Repeat(`x\n`, 200) + `","language":"go"}`, 400, "invalid_request"},
		{"body too large", strings.Repeat(" ", 128*1024+1), 413, "request_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(reviewerFunc(func(context.Context, review.Request) (review.Result, error) {
				t.Fatal("invalid request reached reviewer")
				return review.Result{}, nil
			}))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/lint", strings.NewReader(tc.body)))
			assertError(t, w, tc.status, tc.code)
		})
	}
}

func TestLintErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result review.Result
		err    error
		status int
		code   string
	}{
		{"provider failure", review.Result{}, errors.New("secret upstream token"), 502, "review_failed"},
		{"wrapped timeout", review.Result{}, fmt.Errorf("upstream: %w", context.DeadlineExceeded), 504, "review_timeout"},
		{"missing findings", review.Result{}, nil, 502, "invalid_review"},
		{"invalid range", review.Result{Findings: []review.Finding{{StartLine: 1, EndLine: 2, Explanation: "reason", SuggestedFix: "fix"}}}, nil, 502, "invalid_review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(reviewerFunc(func(context.Context, review.Request) (review.Result, error) { return tc.result, tc.err }))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/lint", strings.NewReader(`{"code":"x","language":"go"}`)))
			assertError(t, w, tc.status, tc.code)
			if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "upstream") {
				t.Fatal("internal error exposed")
			}
		})
	}
}

func TestLintHandlerLeavesRoutingToCaller(t *testing.T) {
	handler := NewHandler(reviewerFunc(func(context.Context, review.Request) (review.Result, error) {
		return review.Result{Findings: []review.Finding{}}, nil
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("PUT", "/another-path", strings.NewReader(`{"code":"x","language":"go"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("handler performed routing: status=%d", w.Code)
	}
}

func TestLintBodyBoundary(t *testing.T) {
	payload := `{"code":"x","language":"go"}`
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("extra bytes %d", extra), func(t *testing.T) {
			calls := 0
			handler := NewHandler(reviewerFunc(func(context.Context, review.Request) (review.Result, error) {
				calls++
				return review.Result{Findings: []review.Finding{}}, nil
			}))
			body := payload + strings.Repeat(" ", 128*1024-len(payload)+extra)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("POST", "/lint", strings.NewReader(body)))
			if extra == 0 {
				if w.Code != 200 || calls != 1 {
					t.Fatalf("status=%d calls=%d", w.Code, calls)
				}
			} else {
				assertError(t, w, 413, "request_too_large")
				if calls != 0 {
					t.Fatal("oversized request reached reviewer")
				}
			}
		})
	}
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatal("error must be JSON")
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != code || strings.TrimSpace(body.Error.Message) == "" {
		t.Fatalf("unexpected error body: %s", w.Body)
	}
}
