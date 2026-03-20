package parser

import (
	"regexp"
	"strings"
)

// CompiledOmitRule is one compiled regex rule used to clean chunk text.
// Enabled/disabled is handled by the caller when building compiled rules list.
type CompiledOmitRule struct {
	re *regexp.Regexp
}

// CompileOmitRules compiles omit regex patterns once per pipeline run.
func CompileOmitRules(patterns []string) ([]CompiledOmitRule, error) {
	out := make([]CompiledOmitRule, 0, len(patterns))
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, CompiledOmitRule{re: re})
	}
	return out, nil
}

// CleanText applies omit rules to the input chunk content.
// Semantics required for embedding + diff consistency:
// - replace matches with spaces
// - then TrimSpace
// - then collapse consecutive whitespace into a single space
// So a chunk that becomes only spaces will become "" and can be treated as "skipped".
func CleanText(text string, omitRules []CompiledOmitRule) string {
	cleaned := text
	for _, r := range omitRules {
		if r.re == nil {
			continue
		}
		cleaned = r.re.ReplaceAllString(cleaned, " ")
	}
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	// Collapse any whitespace (space/newline/tab) sequences.
	return strings.Join(strings.Fields(cleaned), " ")
}

