package parser

import (
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// Паттерны для определения заголовков в белорусских НПА.
// Порядок важен — более специфичные паттерны первее.
var headingPatterns = []struct {
	re    *regexp.Regexp
	level int
}{
	// Глава 1, ГЛАВА I
	{regexp.MustCompile(`(?i)^(глава|раздел)\s+[\dIVXivx]+[.\s]`), 0},
	// Статья 1., Артыкул 1.
	{regexp.MustCompile(`(?i)^(стаття|статья|артыкул)\s+\d+[.\s]`), 1},
	// 1. Пункт (цифра с точкой в начале строки)
	{regexp.MustCompile(`^\d+\.\s+[А-ЯЁA-Z]`), 2},
	// 1.1. Подпункт
	{regexp.MustCompile(`^\d+\.\d+\.\s+`), 2},
	// 1) или а) — часть
	{regexp.MustCompile(`^(\d+|[а-яa-z])\)\s+`), 3},
}

type pdfParser struct{}

func (p *pdfParser) Parse(r io.ReaderAt, size int64) (*Document, error) {
	// pdftotext читает из stdin, пишет в stdout
	// -layout сохраняет пространственное расположение текста
	// -enc UTF-8 для корректной кириллицы
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", "-", "-")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	// копируем файл в stdin
	go func() {
		defer stdin.Close()
		buf := make([]byte, 32*1024)
		var offset int64
		for {
			n, err := r.ReadAt(buf, offset)
			if n > 0 {
				stdin.Write(buf[:n])
				offset += int64(n)
			}
			if err != nil {
				break
			}
		}
	}()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}

	return parseText(string(out)), nil
}

// parseText строит иерархию Document из плоского текста.
func parseText(text string) *Document {
	lines := strings.Split(text, "\n")
	result := &Document{}
	var stack []*Section
	counter := &idCounter{}

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

			stack = stack[:min(len(stack), level)]

			if len(stack) == 0 {
				result.Sections = append(result.Sections, sec)
				stack = append(stack, &result.Sections[len(result.Sections)-1])
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, sec)
				stack = append(stack, &parent.Children[len(parent.Children)-1])
			}
		} else if len(stack) > 0 {
			cur := stack[len(stack)-1]
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
