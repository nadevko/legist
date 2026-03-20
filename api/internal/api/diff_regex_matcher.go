package api

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffMatchRegexRule is one compiled regex rule from DIFF_MATCH_REGEX_FILE.
// File format: TAG|REGEXP (one rule per line).
type DiffMatchRegexRule struct {
	Tag     string
	Pattern *regexp.Regexp
}

// loadDiffMatchRegexRules parses and compiles regex rules from a newline-delimited file.
func loadDiffMatchRegexRules(path string) ([]DiffMatchRegexRule, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read regex file: %w", err)
	}

	lines := strings.Split(string(raw), "\n")
	rules := make([]DiffMatchRegexRule, 0, len(lines))
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Allow comments.
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid regex rule format at line %d: expected TAG|REGEXP", idx+1)
		}
		tag := strings.TrimSpace(parts[0])
		pat := strings.TrimSpace(parts[1])
		if tag == "" || pat == "" {
			continue
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp at line %d (tag=%q): %w", idx+1, tag, err)
		}
		rules = append(rules, DiffMatchRegexRule{Tag: tag, Pattern: re})
	}
	return rules, nil
}

// diffMatchRegexMatchesDelta checks whether regex rules match at least one changed fragment
// between leftContent and rightContent.
//
// Uses diff-match-patch to get text deltas and checks regex against changed fragments
// expanded by small word context windows.
func diffMatchRegexMatchesDelta(leftContent, rightContent string, rules []DiffMatchRegexRule) bool {
	if len(rules) == 0 {
		return false
	}
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(leftContent, rightContent, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	const ctxWords = 2
	const maxFragments = 64
	fragmentsChecked := 0

	for idx, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		frag := strings.TrimSpace(d.Text)
		if frag == "" {
			continue
		}
		withCtx := expandDiffFragmentWithContext(diffs, idx, ctxWords, frag)
		if matchFragments(withCtx, rules) {
			return true
		}
		fragmentsChecked++
		if fragmentsChecked >= maxFragments {
			return false
		}
	}
	return false
}

func matchFragments(fragment string, rules []DiffMatchRegexRule) bool {
	if fragment == "" {
		return false
	}
	for _, r := range rules {
		if r.Pattern.MatchString(fragment) {
			return true
		}
	}
	return false
}

func expandDiffFragmentWithContext(diffs []diffmatchpatch.Diff, idx int, ctxWords int, core string) string {
	left := takeLastWords(previousEqualText(diffs, idx), ctxWords)
	right := takeFirstWords(nextEqualText(diffs, idx), ctxWords)
	return strings.TrimSpace(strings.TrimSpace(left) + " " + core + " " + strings.TrimSpace(right))
}

func previousEqualText(diffs []diffmatchpatch.Diff, idx int) string {
	for i := idx - 1; i >= 0; i-- {
		if diffs[i].Type == diffmatchpatch.DiffEqual {
			return diffs[i].Text
		}
	}
	return ""
}

func nextEqualText(diffs []diffmatchpatch.Diff, idx int) string {
	for i := idx + 1; i < len(diffs); i++ {
		if diffs[i].Type == diffmatchpatch.DiffEqual {
			return diffs[i].Text
		}
	}
	return ""
}

func takeLastWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) <= n {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts[len(parts)-n:], " ")
}

func takeFirstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) <= n {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts[:n], " ")
}

