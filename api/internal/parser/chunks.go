package parser

import (
	"strings"
	"unicode/utf8"
)

func assignChunkOffsets(doc *Document) {
	if doc == nil || doc.PlainText == "" {
		return
	}
	cursorByte := 0
	cursorRune := 0
	var walk func([]Section)
	walk = func(sections []Section) {
		for i := range sections {
			s := &sections[i]
			for j := range s.Chunks {
				ch := &s.Chunks[j]
				norm := NormalizePlainText(ch.Text)
				ch.Text = norm
				ch.SectionID = s.ID
				ch.SectionNum = s.Num
				if norm == "" {
					ch.PlainStart = cursorRune
					ch.PlainEnd = cursorRune
					continue
				}
				off := strings.Index(doc.PlainText[cursorByte:], norm)
				if off < 0 {
					ch.PlainStart = cursorRune
					ch.PlainEnd = cursorRune
					continue
				}
				startByte := cursorByte + off
				endByte := startByte + len(norm)
				ch.PlainStart = runeCount(doc.PlainText[:startByte])
				ch.PlainEnd = ch.PlainStart + utf8.RuneCountInString(norm)
				cursorByte = endByte
				cursorRune = ch.PlainEnd
			}
			walk(s.Children)
		}
	}
	walk(doc.Sections)
}

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}
