// Package review defines the provider-independent review contract and validation.
package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	MaxCodeBytes = 12 * 1024
	MaxLines     = 200
	MaxFindings  = 5
)

type Request struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

type Finding struct {
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Explanation  string `json:"explanation"`
	SuggestedFix string `json:"suggested_fix"`
}

type Result struct {
	Findings []Finding `json:"findings"`
}

type Reviewer interface {
	Review(context.Context, Request) (Result, error)
}

// NormalizeCode preserves indentation, blank lines, and trailing newlines.
func NormalizeCode(code string) string {
	return strings.ReplaceAll(code, "\r\n", "\n")
}

// ValidateRequest checks normalized source without modifying the caller's request.
// Callers should also normalize the source they retain for review and line references.
func ValidateRequest(req Request) error {
	if req.Language != "go" && req.Language != "python" {
		return errors.New("language must be go or python")
	}
	code := NormalizeCode(req.Code)
	if strings.TrimSpace(code) == "" {
		return errors.New("code must not be blank")
	}
	if len(code) > MaxCodeBytes {
		return fmt.Errorf("code must not exceed %d bytes", MaxCodeBytes)
	}
	// A final newline creates an empty final line, matching strings.Split.
	if strings.Count(code, "\n")+1 > MaxLines {
		return fmt.Errorf("code must not exceed %d lines", MaxLines)
	}
	return nil
}

// ValidateResult validates structure, not the correctness of a finding or fix.
// lineCount is the number of lines in the normalized submitted source.
func ValidateResult(result Result, lineCount int) error {
	if result.Findings == nil {
		return errors.New("findings must be an array, not null or missing")
	}
	if len(result.Findings) > MaxFindings {
		return fmt.Errorf("review must not exceed %d findings", MaxFindings)
	}
	for i, finding := range result.Findings {
		if finding.StartLine < 1 || finding.EndLine < finding.StartLine || finding.EndLine > lineCount {
			return fmt.Errorf("finding %d has an invalid line range", i+1)
		}
		if strings.TrimSpace(finding.Explanation) == "" {
			return fmt.Errorf("finding %d must have an explanation", i+1)
		}
		if strings.TrimSpace(finding.SuggestedFix) == "" {
			return fmt.Errorf("finding %d must have a suggested fix", i+1)
		}
		for _, line := range strings.Split(finding.SuggestedFix, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
				return fmt.Errorf("finding %d suggested fix must not contain Markdown fences", i+1)
			}
		}
	}
	return nil
}
