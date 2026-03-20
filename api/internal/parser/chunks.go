package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ChunkWeightConfig struct {
	Critical  float64
	Main      float64
	Standard  float64
	Technical float64
	MaxCap    float64
}

var (
	criticalChunkRe  = regexp.MustCompile(`(?i)штраф|ответственност|срок|запрещается`)
	technicalChunkRe = regexp.MustCompile(`(?i)вступает в силу|приложени|утратил силу`)
	mainChunkRe      = regexp.MustCompile(`(?i)прав[ао]|обязан|полномоч`)
)

func assignChunkWeights(doc *Document, cfg ChunkWeightConfig) {
	if doc == nil {
		return
	}
	if cfg.Critical <= 0 {
		cfg.Critical = 3.0
	}
	if cfg.Main <= 0 {
		cfg.Main = 2.0
	}
	if cfg.Standard <= 0 {
		cfg.Standard = 1.0
	}
	if cfg.Technical <= 0 {
		cfg.Technical = 0.5
	}
	if cfg.MaxCap <= 0 {
		cfg.MaxCap = 3.0
	}

	contentIdx := 0
	var walk func([]Section)
	walk = func(sections []Section) {
		for i := range sections {
			for j := range sections[i].Chunks {
				if contentIdx >= len(doc.ChunkContent) {
					return
				}
				text := doc.ChunkContent[contentIdx]
				contentIdx++
				w := cfg.Standard
				switch {
				case criticalChunkRe.MatchString(text):
					w = cfg.Critical
				case technicalChunkRe.MatchString(text):
					w = cfg.Technical
				case mainChunkRe.MatchString(text):
					w = cfg.Main
				}
				if w > cfg.MaxCap {
					w = cfg.MaxCap
				}
				if w <= 0 {
					w = cfg.Standard
				}
				sections[i].Chunks[j].Weight = w
			}
			walk(sections[i].Children)
		}
	}
	walk(doc.Sections)
}

// assignChunkOffsets assigns plain rune offsets for each chunk text within doc.PlainText.
// Offsets are computed from the exact match range in plain text.
// Returns an error when a chunk text can't be located in PlainText while preserving order.
func assignChunkOffsets(doc *Document) error {
	if doc == nil || doc.PlainText == "" {
		return nil
	}
	cursorByte := 0
	cursorRune := 0
	contentIdx := 0

	var walk func([]Section) error
	walk = func(sections []Section) error {
		for i := range sections {
			s := &sections[i]
			for j := range s.Chunks {
				ch := &s.Chunks[j]
				ch.SectionID = s.ID
				ch.SectionNum = s.Num

				if contentIdx >= len(doc.ChunkContent) {
					return fmt.Errorf("chunk offsets: content index out of range (%d >= %d)", contentIdx, len(doc.ChunkContent))
				}
				norm := NormalizePlainText(doc.ChunkContent[contentIdx])
				doc.ChunkContent[contentIdx] = norm
				contentIdx++
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

	if err := walk(doc.Sections); err != nil {
		return err
	}
	if contentIdx != len(doc.ChunkContent) {
		return fmt.Errorf("chunk offsets: unconsumed chunk_content entries (%d of %d)", contentIdx, len(doc.ChunkContent))
	}
	return nil
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
