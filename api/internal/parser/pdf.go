package parser

import (
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// Heading patterns for Belarusian НПА. Order matters — more specific first.
var headingPatterns = []struct {
	re    *regexp.Regexp
	level int
}{
	// Глава 1 / Раздел I
	{regexp.MustCompile(`(?i)^(глава|раздел)\s+[\dIVXivx]+`), 0},
	// Статья 1 / Артыкул 1
	{regexp.MustCompile(`(?i)^(стаття|статья|артыкул)\s+\d+`), 1},
	// 1.1. sub-clause (check before simple 1.)
	{regexp.MustCompile(`^\d+\.\d+\.?\s+`), 2},
	// 1. clause starting with uppercase
	{regexp.MustCompile(`^\d+\.\s+[А-ЯЁA-Z]`), 2},
	// 1) or а) — part
	{regexp.MustCompile(`^(\d+|[а-яa-z])\)\s+`), 3},
}

type pdfParser struct{}

// Parse implements Parser. Reads all bytes from r and pipes them to pdftotext.
// Prefer pdfParseByPath when the file path is available — it's more reliable.
func (p *pdfParser) Parse(r io.ReaderAt, size int64) (*Document, error) {
	buf := make([]byte, size)
	if _, err := r.ReadAt(buf, 0); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read pdf: %w", err)
	}
	cmd := exec.Command("pdftotext", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = strings.NewReader(string(buf))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}
	return parseText(string(out)), nil
}

// pdfParseByPath is preferred when the file path is available.
// Passes the path directly to pdftotext — avoids reading all bytes into memory
// and is more reliable across poppler versions that don't support stdin.
func pdfParseByPath(path string) (*Document, error) {
	out, err := exec.Command("pdftotext", "-enc", "UTF-8", path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}
	return parseText(string(out)), nil
}

// parseText builds a Document from flat pdftotext output.
func parseText(text string) *Document {
	lines := strings.Split(text, "\n")
	result := &Document{}
	counter := &idCounter{}

	type entry struct{ idx int }
	var stack []entry

	getSection := func() *Section {
		sec := &result.Sections[stack[0].idx]
		for _, e := range stack[1:] {
			sec = &sec.Children[e.idx]
		}
		return sec
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		level, isHeading := detectHeading(line)

		if isHeading {
			sec := Section{
				ID:    counter.next(level),
				Label: line,
				Level: level,
			}
			if level < len(stack) {
				stack = stack[:level]
			}
			if len(stack) == 0 {
				result.Sections = append(result.Sections, sec)
				stack = append(stack, entry{len(result.Sections) - 1})
			} else {
				parent := getSection()
				parent.Children = append(parent.Children, sec)
				stack = append(stack, entry{len(parent.Children) - 1})
			}
		} else if len(stack) > 0 {
			cur := getSection()
			if cur.Text == "" {
				cur.Text = line
			} else {
				cur.Text += " " + line
			}
		}
	}

	return result
}

func detectHeading(line string) (level int, ok bool) {
	for _, p := range headingPatterns {
		if p.re.MatchString(line) {
			return p.level, true
		}
	}
	return 0, false
}
