package groq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

func completion(content, finish string, refusal any) string {
	data, err := json.Marshal(map[string]any{
		"id": "test-completion", "object": "chat.completion", "model": "test-model",
		"choices": []any{map[string]any{
			"index": 0, "finish_reason": finish,
			"message": map[string]any{"role": "assistant", "content": content, "refusal": refusal},
		}},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
	})
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestReviewRequestAndSuccess(t *testing.T) {
	for _, content := range []string{
		`{"findings":[]}`,
		`{"findings":[{"start_line":1,"end_line":2,"explanation":"A zero divisor raises an error.","suggested_fix":"if divisor == 0:\n    return None"}]}`,
	} {
		t.Run(content, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/openai/v1/chat/completions" {
					t.Errorf("unexpected route: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing authentication or JSON header")
				}
				var sent struct {
					Model               string                           `json:"model"`
					Messages            []struct{ Role, Content string } `json:"messages"`
					MaxCompletionTokens int                              `json:"max_completion_tokens"`
					ResponseFormat      struct {
						Type       string `json:"type"`
						JSONSchema struct {
							Strict bool           `json:"strict"`
							Schema map[string]any `json:"schema"`
						} `json:"json_schema"`
					} `json:"response_format"`
				}
				if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
					t.Error(err)
					return
				}
				if sent.Model != "test-model" || sent.MaxCompletionTokens <= 0 {
					t.Error("missing configured model or output bound")
				}
				if len(sent.Messages) != 2 {
					t.Errorf("messages=%d", len(sent.Messages))
					return
				}
				if sent.Messages[0].Role != "system" || strings.Contains(sent.Messages[0].Content, "UNTRUSTED_COMMENT") {
					t.Error("source leaked into fixed instructions")
				}
				if sent.Messages[1].Role != "user" || sent.Messages[1].Content != "Language: python\nNumbered source:\n1 |   x\n2 | # UNTRUSTED_COMMENT\n3 | " {
					t.Errorf("unexpected source message: %q", sent.Messages[1].Content)
				}
				format := sent.ResponseFormat
				if format.Type != "json_schema" || !format.JSONSchema.Strict {
					t.Error("strict schema not requested")
				}
				schema := format.JSONSchema.Schema
				if schema["type"] != "object" || schema["additionalProperties"] != false || !reflect.DeepEqual(schema["required"], []any{"findings"}) {
					t.Errorf("invalid root schema: %#v", schema)
				}
				properties, ok := schema["properties"].(map[string]any)
				if !ok {
					t.Fatal("missing schema properties")
				}
				findings, ok := properties["findings"].(map[string]any)
				if !ok {
					t.Fatal("missing findings schema")
				}
				item, ok := findings["items"].(map[string]any)
				if !ok {
					t.Fatal("missing finding schema")
				}
				if item["additionalProperties"] != false || !reflect.DeepEqual(item["required"], []any{"start_line", "end_line", "explanation", "suggested_fix"}) {
					t.Error("finding schema permits missing or extra fields")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, completion(content, "stop", nil))
			}))
			defer server.Close()
			client := NewClient(server.Client(), "test-key", "test-model")
			client.endpoint = server.URL + "/openai/v1/chat/completions"
			got, err := client.Review(context.Background(), review.Request{Code: "  x\r\n# UNTRUSTED_COMMENT\r\n", Language: "python"})
			if err != nil {
				t.Fatal(err)
			}
			var want review.Result
			if err := json.Unmarshal([]byte(content), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v want %#v", got, want)
			}
			if calls != 1 {
				t.Fatalf("requests=%d", calls)
			}
		})
	}
}

func TestReviewRejectsProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"malformed envelope", "{", 200},
		{"null envelope", "null", 200},
		{"no choices", `{"choices":[]}`, 200},
		{"multiple envelopes", completion(`{"findings":[]}`, "stop", nil) + `{}`, 200},
		{"truncated", completion(`{"findings":[]}`, "length", nil), 200},
		{"refused", completion(`{"findings":[]}`, "stop", "Cannot review"), 200},
		{"empty content", completion("", "stop", nil), 200},
		{"malformed content", completion("{", "stop", nil), 200},
		{"extra content", completion(`{"findings":[]} {}`, "stop", nil), 200},
		{"unknown field", completion(`{"findings":[],"summary":"ok"}`, "stop", nil), 200},
		{"missing findings", completion(`{}`, "stop", nil), 200},
		{"null findings", completion(`{"findings":null}`, "stop", nil), 200},
		{"null finding", completion(`{"findings":[null]}`, "stop", nil), 200},
		{"missing line", completion(`{"findings":[{"end_line":1,"explanation":"reason","suggested_fix":"fix"}]}`, "stop", nil), 200},
		{"null fix", completion(`{"findings":[{"start_line":1,"end_line":1,"explanation":"reason","suggested_fix":null}]}`, "stop", nil), 200},
		{"fractional line", completion(`{"findings":[{"start_line":1.5,"end_line":2,"explanation":"reason","suggested_fix":"fix"}]}`, "stop", nil), 200},
		{"out of range", completion(`{"findings":[{"start_line":1,"end_line":9,"explanation":"reason","suggested_fix":"fix"}]}`, "stop", nil), 200},
		{"oversized response", strings.Repeat(" ", 256*1024+1), 200},
		{"rate limit", `{"error":{"message":"secret-provider-detail"}}`, 429},
		{"unauthorized", `{"error":{"message":"secret-provider-detail"}}`, 401},
		{"upstream error", `{"error":{"message":"secret-provider-detail"}}`, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := NewClient(server.Client(), "test-key", "test-model")
			client.endpoint = server.URL
			_, err := client.Review(context.Background(), review.Request{Code: "x", Language: "go"})
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), "secret-provider-detail") {
				t.Fatal("provider body leaked")
			}
			if calls != 1 {
				t.Fatalf("requests=%d; retries are not allowed", calls)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReviewDeadline(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining <= 0 || remaining > 30*time.Second {
			t.Errorf("unexpected deadline: %v", remaining)
		}
		return nil, context.DeadlineExceeded
	})}, "key", "model")
	_, err := client.Review(context.Background(), review.Request{Code: "x", Language: "go"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline not preserved: %v", err)
	}
}

func TestReviewHonorsCallerCancellation(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}, "key", "model")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Review(ctx, review.Request{Code: "x", Language: "go"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not preserved: %v", err)
	}
}
