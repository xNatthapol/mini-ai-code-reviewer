package review

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

type cacheReviewerFunc func(context.Context, Request) (Result, error)

func (f cacheReviewerFunc) Review(ctx context.Context, req Request) (Result, error) {
	return f(ctx, req)
}

func TestCacheKeys(t *testing.T) {
	calls := 0
	cache := NewCachedReviewer(cacheReviewerFunc(func(_ context.Context, req Request) (Result, error) {
		calls++
		if req.Code == "a\r\nb" {
			t.Fatal("provider received unnormalized code")
		}
		return Result{Findings: []Finding{}}, nil
	}))
	for _, tc := range []struct {
		language, code string
		calls          int
	}{
		{"go", "a\r\nb", 1},
		{"go", "a\nb", 1},
		{"python", "a\nb", 2},
		{"go", "a\nb ", 3},
	} {
		result, err := cache.Review(context.Background(), Request{Language: tc.language, Code: tc.code})
		if err != nil || result.Findings == nil || len(result.Findings) != 0 {
			t.Fatalf("result = %+v, error = %v", result, err)
		}
		if calls != tc.calls {
			t.Fatalf("calls = %d, want %d", calls, tc.calls)
		}
	}
}

func TestCacheDoesNotStoreFailures(t *testing.T) {
	upstreamError := errors.New("provider failed")
	for _, tc := range []struct {
		name   string
		result Result
		err    error
	}{
		{"provider error", Result{}, upstreamError},
		{"invalid result", Result{Findings: []Finding{{StartLine: 2, EndLine: 2, Explanation: "bug", SuggestedFix: "fix"}}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cache := NewCachedReviewer(cacheReviewerFunc(func(context.Context, Request) (Result, error) { calls++; return tc.result, tc.err }))
			for range 2 {
				result, err := cache.Review(context.Background(), Request{Language: "go", Code: "x"})
				if !errors.Is(err, tc.err) || len(result.Findings) != len(tc.result.Findings) {
					t.Fatalf("result/error changed: %+v, %v", result, err)
				}
			}
			if calls != 2 {
				t.Fatalf("calls = %d, want 2", calls)
			}
		})
	}
}

func TestCacheCopiesFindings(t *testing.T) {
	original := Result{Findings: []Finding{{StartLine: 1, EndLine: 1, Explanation: "bug", SuggestedFix: "fix"}}}
	calls := 0
	cache := NewCachedReviewer(cacheReviewerFunc(func(context.Context, Request) (Result, error) { calls++; return original, nil }))
	req := Request{Language: "go", Code: "x"}
	first, _ := cache.Review(context.Background(), req)
	first.Findings[0].Explanation = "changed by caller"
	original.Findings[0].SuggestedFix = "changed by provider"
	second, _ := cache.Review(context.Background(), req)
	if second.Findings[0].Explanation != "bug" || second.Findings[0].SuggestedFix != "fix" {
		t.Fatalf("cached result mutated: %+v", second)
	}
	second.Findings[0].Explanation = "changed on cache hit"
	third, _ := cache.Review(context.Background(), req)
	if third.Findings[0].Explanation != "bug" || calls != 1 {
		t.Fatalf("result = %+v, calls = %d", third, calls)
	}
}

func TestCacheConcurrentRequests(t *testing.T) {
	var calls atomic.Int32
	cache := NewCachedReviewer(cacheReviewerFunc(func(context.Context, Request) (Result, error) {
		calls.Add(1)
		return Result{Findings: []Finding{}}, nil
	}))
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(func() {
			req := Request{Language: "go", Code: fmt.Sprintf("code %d", i%10)}
			if _, err := cache.Review(context.Background(), req); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	before := calls.Load()
	for i := range 10 {
		_, err := cache.Review(context.Background(), Request{Language: "go", Code: fmt.Sprintf("code %d", i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != before {
		t.Fatal("completed results were not cached")
	}
}

func TestCacheEvictsWhenFull(t *testing.T) {
	calls := 0
	cache := NewCachedReviewer(cacheReviewerFunc(func(context.Context, Request) (Result, error) { calls++; return Result{Findings: []Finding{}}, nil }))
	for i := range 101 {
		_, _ = cache.Review(context.Background(), Request{Language: "go", Code: fmt.Sprintf("code %d", i)})
	}
	_, _ = cache.Review(context.Background(), Request{Language: "go", Code: "code 100"})
	if calls != 101 {
		t.Fatal("newest result was not cached")
	}
	for i := range 100 {
		_, _ = cache.Review(context.Background(), Request{Language: "go", Code: fmt.Sprintf("code %d", i)})
	}
	if calls == 101 {
		t.Fatal("cache kept all 101 entries")
	}
}
