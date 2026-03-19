package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/fumiama/go-docx"
)

// Heading style name → nesting level.
var headingStyles = map[string]int{
	"heading 1": 0, "heading 2": 1, "heading 3": 2, "heading 4": 3,
	"заголовок 1": 0, "заголовок 2": 1, "заголовок 3": 2, "заголовок 4": 3,
}

type docxParser struct{}

func (p *docxParser) Parse(r io.ReaderAt, size int64) (*Document, error) {
	doc, err := docx.Parse(r, size)
	if err != nil {
		return nil, fmt.Errorf("parse docx: %w", err)
	}

	result := &Document{}
	counter := &idCounter{}

	// Stack stores index path from root: stack[0].idx is index in result.Sections,
	// stack[1].idx is index in result.Sections[stack[0].idx].Children, etc.
	// Storing indices instead of pointers avoids invalidation when slices grow.
	type entry struct{ idx int }
	var stack []entry

	getSection := func() *Section {
		sec := &result.Sections[stack[0].idx]
		for _, e := range stack[1:] {
			sec = &sec.Children[e.idx]
		}
		return sec
	}

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
				cur.Text = text
			} else {
				cur.Text += "\n" + text
			}
		}
		// Text before any heading is ignored — uncommon in НПА.
	}

	return result, nil
}

func extractParaText(para *docx.Paragraph) string {
	var b strings.Builder
	for _, item := range para.Children {
		if run, ok := item.(*docx.Run); ok {
			for _, child := range run.Children {
				if t, ok := child.(*docx.Text); ok {
					b.WriteString(t.Text)
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}
