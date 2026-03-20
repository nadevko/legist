package parser

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// assignChunkOffsets assigns plain rune offsets for each chunk.Content within doc.PlainText.
// Offsets are computed from the exact match range in plain text.
// Returns an error when a chunk text can't be located in PlainText while preserving order.
func assignChunkOffsets(doc *Document) error {
	if doc == nil || doc.PlainText == "" {
		return nil
	}
	cursorByte := 0
	cursorRune := 0

	var walk func([]Section) error
	walk = func(sections []Section) error {
		for i := range sections {
			s := &sections[i]
			for j := range s.Chunks {
				ch := &s.Chunks[j]
				ch.SectionID = s.ID
				ch.SectionNum = s.Num

				norm := NormalizePlainText(ch.Content)
				ch.Content = norm
				if norm == "" {
					ch.PlainStart = cursorRune
					ch.PlainEnd = cursorRune
					continue
				}

				off := strings.Index(doc.PlainText[cursorByte:], norm)
				if off < 0 {
					return fmt.Errorf(
						"chunk offsets: cannot find chunk in plain text (cursorByte=%d norm=%q)",
						cursorByte,
						firstN(norm, 64),
					)
				}

				startByte := cursorByte + off
				endByte := startByte + len(norm)

				startRune := runeCount(doc.PlainText[:startByte])
				endRune := startRune + utf8.RuneCountInString(norm)

				ch.PlainStart = startRune
				ch.PlainEnd = endRune

				cursorByte = endByte
				cursorRune = endRune
			}

			if err := walk(s.Children); err != nil {
				return err
			}
		}
		return nil
	}

	return walk(doc.Sections)
}

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
