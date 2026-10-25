package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

func TestReadConfig(t *testing.T) {
	for _, tc := range []struct {
		name, key, model, port string
		invalid                bool
	}{
		{"defaults", "key", "", "", false},
		{"overrides", "key", "configured-model", "9090", false},
		{"missing key", "", "", "", true},
		{"blank key", "  ", "", "", true},
		{"non numeric port", "key", "", "abc", true},
		{"zero port", "key", "", "0", true},
		{"negative port", "key", "", "-1", true},
		{"port too large", "key", "", "65536", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GROQ_API_KEY", tc.key)
			t.Setenv("GROQ_MODEL", tc.model)
			t.Setenv("PORT", tc.port)
			cfg, err := readConfig()
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v want invalid=%v", err, tc.invalid)
			}
			if tc.invalid {
				return
			}
			wantModel, wantPort := tc.model, tc.port
			if wantModel == "" {
				wantModel = "openai/gpt-oss-120b"
			}
			if wantPort == "" {
				wantPort = "8080"
			}
			if cfg.apiKey != tc.key || cfg.model != wantModel || cfg.port != wantPort {
				t.Fatal("configuration not loaded correctly")
			}
		})
	}
}

type fakeReviewer struct{ calls int }

func (f *fakeReviewer) Review(context.Context, review.Request) (review.Result, error) {
	f.calls++
	return review.Result{Findings: []review.Finding{}}, nil
}

func TestServerRoutes(t *testing.T) {
	for _, tc := range []struct {
		method, path  string
		status, calls int
	}{
		{"POST", "/lint", 200, 1},
		{"GET", "/lint", 405, 0},
		{"POST", "/missing", 404, 0},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			reviewer := &fakeReviewer{}
			server := newServer(config{port: "9090"}, reviewer)
			if server.Addr != "127.0.0.1:9090" {
				t.Fatalf("unexpected address: %s", server.Addr)
			}
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"code":"x","language":"go"}`)))
			if w.Code != tc.status || reviewer.calls != tc.calls {
				t.Fatalf("status=%d calls=%d", w.Code, reviewer.calls)
			}
			if tc.status == 200 && strings.TrimSpace(w.Body.String()) != `{"findings":[]}` {
				t.Fatalf("unexpected body: %s", w.Body)
			}
			if tc.status == 405 && w.Header().Get("Allow") != "POST" {
				t.Error("missing Allow header")
			}
		})
	}
}
