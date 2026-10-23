package review

import (
	"strings"
	"testing"
)

func TestNormalizeCode(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"CRLF", "  a\r\n\r\nb\r\n", "  a\n\nb\n"},
		{"preserve whitespace", "\t a  \n\n", "\t a  \n\n"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeCode(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateRequest(t *testing.T) {
	for _, tc := range []struct {
		name, code, language string
		invalid              bool
	}{
		{"go", "package main", "go", false},
		{"python", "print(1)", "python", false},
		{"empty", "", "go", true},
		{"blank", " \t\r\n", "go", true},
		{"unsupported", "x", "javascript", true},
		{"case sensitive", "x", "Go", true},
		{"missing language", "x", "", true},
		{"200 lines", strings.Repeat("x\n", 199) + "x", "go", false},
		{"201 lines", strings.Repeat("x\n", 200) + "x", "go", true},
		{"trailing empty line counts", strings.Repeat("x\n", 200), "go", true},
		{"byte boundary", strings.Repeat("x", 12288), "go", false},
		{"over byte boundary", strings.Repeat("x", 12289), "go", true},
		{"unicode byte boundary", strings.Repeat("é", 6144), "python", false},
		{"unicode over boundary", strings.Repeat("é", 6145), "python", true},
		{"CRLF normalized before byte limit", strings.Repeat("x", 12287) + "\r\n", "go", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRequest(Request{Code: tc.code, Language: tc.language}); (err != nil) != tc.invalid {
				t.Fatalf("error = %v, want invalid = %v", err, tc.invalid)
			}
		})
	}
}

func TestValidateResult(t *testing.T) {
	valid := Finding{StartLine: 1, EndLine: 3, Explanation: "The last element is skipped.", SuggestedFix: "for x in items:\n    print(x)"}
	for _, tc := range []struct {
		name    string
		finding Finding
	}{
		{"zero start", Finding{0, 3, valid.Explanation, valid.SuggestedFix}},
		{"negative start", Finding{-1, 3, valid.Explanation, valid.SuggestedFix}},
		{"reversed range", Finding{3, 2, valid.Explanation, valid.SuggestedFix}},
		{"outside source", Finding{1, 4, valid.Explanation, valid.SuggestedFix}},
		{"blank explanation", Finding{1, 3, " \n", valid.SuggestedFix}},
		{"blank fix", Finding{1, 3, valid.Explanation, " \t"}},
		{"backtick fence", Finding{1, 3, valid.Explanation, "```python\nprint(1)\n```"}},
		{"tilde fence", Finding{1, 3, valid.Explanation, "~~~python\nprint(1)\n~~~"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResult(Result{Findings: []Finding{valid, tc.finding}}, 3); err == nil {
				t.Fatal("expected rejection of the entire result")
			}
		})
	}
	for _, tc := range []struct {
		name     string
		findings []Finding
		invalid  bool
	}{
		{"empty array", []Finding{}, false},
		{"nil findings", nil, true},
		{"inclusive range", []Finding{valid}, false},
		{"five findings", []Finding{valid, valid, valid, valid, valid}, false},
		{"six findings", []Finding{valid, valid, valid, valid, valid, valid}, true},
		{"fence characters inside code", []Finding{{1, 1, "An example explanation.", "print(\"```\")"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResult(Result{Findings: tc.findings}, 3); (err != nil) != tc.invalid {
				t.Fatalf("error = %v, want invalid = %v", err, tc.invalid)
			}
		})
	}
}
