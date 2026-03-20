package parser

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var dehyphenRe = regexp.MustCompile(`(\pL)[\-‐‑‒–—]\s+(\pL)`)
var whitespaceRe = regexp.MustCompile(`[ \t\f\v]+`)

const plainLineWidth = 80

// NormalizePlainText makes plain text canonical:
// 1) removes soft hyphenation artifacts, 2) normalizes spaces, 3) wraps by width.
func NormalizePlainText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	// Join words split by hyphen + whitespace/newline.
	for {
		next := dehyphenRe.ReplaceAllString(raw, "${1}${2}")
		if next == raw {
			break
		}
		raw = next
	}

	lines := strings.Split(raw, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = whitespaceRe.ReplaceAllString(strings.TrimSpace(line), " ")
		if line == "" {
			continue
		}
		normalized = append(normalized, wrapLine(line, plainLineWidth)...)
	}
	return strings.Join(normalized, "\n")
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return nil
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return nil
	}
	var out []string
	cur := words[0]
	curLen := utf8.RuneCountInString(cur)
	for _, w := range words[1:] {
		wLen := utf8.RuneCountInString(w)
		if curLen+1+wLen <= width {
			cur += " " + w
			curLen += 1 + wLen
			continue
		}
		out = append(out, cur)
		cur = w
		curLen = wLen
	}
	out = append(out, cur)
	return out
}

func firstNRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
