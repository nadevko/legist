package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/fumiama/go-docx"
)

// Стили заголовков Word → уровень вложенности.
var headingStyles = map[string]int{
	"heading 1": 0,
	"heading 2": 1,
	"heading 3": 2,
	"heading 4": 3,
	// русскоязычные стили
	"заголовок 1": 0,
	"заголовок 2": 1,
	"заголовок 3": 2,
	"заголовок 4": 3,
}

type docxParser struct{}

func (p *docxParser) Parse(r io.ReaderAt, size int64) (*Document, error) {
	doc, err := docx.Parse(r, size)
	if err != nil {
		return nil, fmt.Errorf("parse docx: %w", err)
	}

	result := &Document{}
	var stack []*Section // стек для построения иерархии

	counter := &idCounter{}

	for _, item := range doc.Document.Body.Items {
		para, ok := item.(*docx.Paragraph)
		if !ok {
			continue
		}

		text := extractParaText(para)
		if strings.TrimSpace(text) == "" {
			continue
		}

		style := strings.ToLower(strings.TrimSpace(para.Properties.Style.Val))
		level, isHeading := headingStyles[style]

		if isHeading {
			sec := Section{
				ID:    counter.next(level),
				Label: text,
				Level: level,
			}

			// обрезаем стек до текущего уровня
			stack = stack[:min(len(stack), level)]

			if len(stack) == 0 {
				result.Sections = append(result.Sections, sec)
				stack = append(stack, &result.Sections[len(result.Sections)-1])
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, sec)
				stack = append(stack, &parent.Children[len(parent.Children)-1])
			}
		} else {
			// обычный параграф — добавляем текст к текущей секции
			if len(stack) > 0 {
				cur := stack[len(stack)-1]
				if cur.Text == "" {
					cur.Text = text
				} else {
					cur.Text += "\n" + text
				}
			} else {
				// текст до первого заголовка — создаём безымянную секцию
				if len(result.Sections) == 0 || result.Sections[0].Label != "" {
					sec := Section{
						ID:    counter.next(0),
						Label: "",
						Level: 0,
					}
					result.Sections = append([]Section{sec}, result.Sections...)
				}
				result.Sections[0].Text += text + "\n"
			}
		}
	}

	// первый заголовок уровня 0 — название документа
	if len(result.Sections) > 0 && result.Sections[0].Level == 0 && len(result.Sections[0].Children) == 0 {
		result.Title = result.Sections[0].Label
	}

	return result, nil
}

func extractParaText(para *docx.Paragraph) string {
	var b strings.Builder
	for _, item := range para.Children {
		switch v := item.(type) {
		case *docx.Run:
			for _, child := range v.Children {
				if t, ok := child.(*docx.Text); ok {
					b.WriteString(t.Text)
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// idCounter генерирует иерархические ID секций: s1, s1.1, s1.1.2
type idCounter struct {
	counters [4]int
}

func (c *idCounter) next(level int) string {
	c.counters[level]++
	// сбрасываем счётчики глубже
	for i := level + 1; i < len(c.counters); i++ {
		c.counters[i] = 0
	}
	parts := make([]string, level+1)
	for i := 0; i <= level; i++ {
		parts[i] = fmt.Sprintf("%d", c.counters[i])
	}
	return "s" + strings.Join(parts, ".")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
