package review

import (
	"context"
	"slices"
	"strings"
	"sync"
)

const maxCachedReviews = 100

type cachedReviewer struct {
	upstream Reviewer
	mu       sync.Mutex
	results  map[Request]Result
}

// NewCachedReviewer caches valid successful reviews for this process's lifetime.
func NewCachedReviewer(upstream Reviewer) Reviewer {
	return &cachedReviewer{upstream: upstream, results: make(map[Request]Result)}
}

func (c *cachedReviewer) Review(ctx context.Context, req Request) (Result, error) {
	req.Code = NormalizeCode(req.Code)
	c.mu.Lock()
	result, ok := c.results[req]
	c.mu.Unlock()
	if ok {
		return Result{Findings: slices.Clone(result.Findings)}, nil
	}

	// Concurrent misses may call upstream independently; never hold the lock over I/O.
	result, err := c.upstream.Review(ctx, req)
	if err != nil || ValidateResult(result, strings.Count(req.Code, "\n")+1) != nil {
		return result, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.results[req]; !exists && len(c.results) >= maxCachedReviews {
		// Any existing entry can be evicted; no recency bookkeeping is needed.
		for key := range c.results {
			delete(c.results, key)
			break
		}
	}
	// Keep cached slices separate from both the provider and callers.
	c.results[req] = Result{Findings: slices.Clone(result.Findings)}
	return result, nil
}
