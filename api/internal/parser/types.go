package parser

import (
	"fmt"
	"strings"
	"time"
)

// SectionType is the structural type of a document element.
type SectionType string

const (
	SectionChapter   SectionType = "chapter"
	SectionArticle   SectionType = "article"
	SectionClause    SectionType = "clause"
	SectionSubClause SectionType = "subclause"
	SectionPart      SectionType = "part"
	SectionUnknown   SectionType = "unknown"
)

// TLCReference is an ontological entity (AKN Top Level Class).
type TLCReference struct {
	EID     string `json:"eId"`
	Href    string `json:"href"`
	ShowAs  string `json:"showAs"`
	Type    string `json:"type"` // TLCOrganization|TLCPerson|TLCConcept|TLCEvent|TLCTerm
	Article string `json:"article,omitempty"`
}

// LifecycleEvent is one event in the document's legislative history.
type LifecycleEvent struct {
	Type   string `json:"type"` // generation|amendment|repeal|suspension
	Date   string `json:"date"` // YYYY-MM-DD
	Source string `json:"source,omitempty"`
}

// PassiveModification records how another act has modified this document.
type PassiveModification struct {
	Type    string `json:"type"` // substitution|insertion|deletion|split|merge
	Source  string `json:"source"`
	Section string `json:"section,omitempty"`
	Date    string `json:"date,omitempty"`
}

// Section is one structural element of the document.
type Section struct {
	ID          string         `json:"id"`
	Num         string         `json:"num"`
	Heading     string         `json:"heading,omitempty"`
	Label       string         `json:"label"`
	SectionType SectionType    `json:"section_type"`
	Level       int            `json:"level"`
	Path        []string       `json:"path,omitempty"`
	Chunks      []ChunkRef     `json:"chunks,omitempty"`
	Children    []Section      `json:"children,omitempty"`
	References  []TLCReference `json:"references,omitempty"`
}

// ChunkRef stores chunk placement metadata in canonical plain text.
type ChunkRef struct {
	PlainStart int    `json:"plain_start"` // rune offset, inclusive
	PlainEnd   int    `json:"plain_end"`   // rune offset, exclusive
	Weight     float64 `json:"weight,omitempty"`
	SectionID  string `json:"section_id,omitempty"`
	SectionNum string `json:"section_num,omitempty"`
}

// MatchKey returns a stable key for structural diffing.
func (s *Section) MatchKey() string {
	if s.Num != "" {
		return string(s.SectionType) + ":" + s.Num
	}
	return s.Label
}

// Document is the raw parse result — section tree only, no AKN metadata.
// Returned by ParseFile / docxParser.Parse / pdfParser.Parse.
// The pipeline in pipeline.go combines it with LLM-extracted metadata
// to produce a ParsedFile written to parsed.json.
type Document struct {
	Sections     []Section
	ChunkContent []string // DFS order; aligns with sections[].chunks[] traversal
	PlainText    string
}

func (d *Document) Flatten() []Section {
	var out []Section
	var walk func([]Section)
	walk = func(ss []Section) {
		for _, s := range ss {
			out = append(out, s)
			walk(s.Children)
		}
	}
	walk(d.Sections)
	return out
}

// appendChunk returns a chunk ref with the given line text (offsets filled in assignChunkOffsets).
func (d *Document) appendChunk(line string) ChunkRef {
	d.ChunkContent = append(d.ChunkContent, line)
	return ChunkRef{}
}

func (d *Document) FlattenLeaves() []Section {
	var out []Section
	var walk func([]Section)
	walk = func(ss []Section) {
		for _, s := range ss {
			if len(s.Children) == 0 {
				out = append(out, s)
			} else {
				walk(s.Children)
			}
		}
	}
	walk(d.Sections)
	return out
}

// ParsedFile is what gets written to DATA_PATH/lessed/{file_id} (media type application/lessed).
// Combines the Document section tree with AKN-shaped metadata.
type ParsedFile struct {
	FileID        string     `json:"file_id"`
	DocumentID    string     `json:"document_id"`
	Meta          ParsedMeta `json:"meta"`
	Sections      []Section  `json:"sections"`
	ChunkContent  []string   `json:"chunk_content"`
	PlainTextPath string     `json:"plain_text_path"`
	PlainTextLen  int        `json:"plain_text_len"`

	ParsedAt      time.Time `json:"parsed_at"`
	ParserVersion string    `json:"parser_version"`

	// ChunkEmbeddings — one vector per chunk in chunk_content order (DFS section order).
	ChunkEmbeddings [][]float64 `json:"chunk_embeddings,omitempty"`
	EmbeddingModel  string      `json:"embedding_model,omitempty"`
	EmbeddingShortChunkPrefixMaxChars int    `json:"embedding_short_chunk_prefix_max_chars,omitempty"`
	EmbeddingContextHash              string `json:"embedding_context_hash,omitempty"`
}

// FlattenChunkSectionIDs returns section_id for each chunk in DFS order.
func FlattenChunkSectionIDs(sections []Section) []string {
	var out []string
	var walk func([]Section)
	walk = func(ss []Section) {
		for i := range ss {
			for _, ch := range ss[i].Chunks {
				out = append(out, ch.SectionID)
			}
			walk(ss[i].Children)
		}
	}
	walk(sections)
	return out
}

// FlattenChunkWeights returns chunk weights in DFS order.
func FlattenChunkWeights(sections []Section) []float64 {
	var out []float64
	var walk func([]Section)
	walk = func(ss []Section) {
		for i := range ss {
			for _, ch := range ss[i].Chunks {
				w := ch.Weight
				if w <= 0 {
					w = 1.0
				}
				out = append(out, w)
			}
			walk(ss[i].Children)
		}
	}
	walk(sections)
	return out
}

// EmbeddingsCurrent reports whether stored embeddings match chunk contents and expectedModel.
func (pf *ParsedFile) EmbeddingsCurrent(expectedModel string, expectedPrefixLimit int, expectedContextHash string) bool {
	texts := pf.ChunkContent
	n := len(texts)
	if n == 0 {
		return len(pf.ChunkEmbeddings) == 0
	}
	if pf.EmbeddingModel != expectedModel {
		return false
	}
	if pf.EmbeddingShortChunkPrefixMaxChars != expectedPrefixLimit {
		return false
	}
	if pf.EmbeddingContextHash != expectedContextHash {
		return false
	}
	if len(pf.ChunkEmbeddings) != n {
		return false
	}
	for i, t := range texts {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len(pf.ChunkEmbeddings[i]) == 0 {
			return false
		}
	}
	return true
}

// ParsedMeta is the assembled AKN metadata for one file/version.
type ParsedMeta struct {
	// FRBRWork — from Document (required for diff)
	Subtype  string `json:"subtype"`
	Number   string `json:"number"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Country  string `json:"country"`
	Name     string `json:"name,omitempty"`
	NPALevel int    `json:"npa_level"`

	// FRBRExpression — from File (optional)
	VersionDate   string `json:"version_date,omitempty"`
	VersionNumber string `json:"version_number,omitempty"`
	VersionLabel  string `json:"version_label,omitempty"`
	Language      string `json:"language,omitempty"`

	// Publication — from File (optional)
	PubName   string `json:"pub_name,omitempty"`
	PubDate   string `json:"pub_date,omitempty"`
	PubNumber string `json:"pub_number,omitempty"`

	// Enrichment extracted by LLM — stored in parsed.json only, not DB columns
	Lifecycle            []LifecycleEvent      `json:"lifecycle,omitempty"`
	PassiveModifications []PassiveModification `json:"passive_modifications,omitempty"`
	References           []TLCReference        `json:"references,omitempty"`
	Keywords             []string              `json:"keywords,omitempty"`
}

// idCounter generates hierarchical section IDs: s1, s1.1, s1.1.2
type idCounter struct {
	counters [8]int
}

func (c *idCounter) next(level int) string {
	if level >= len(c.counters) {
		level = len(c.counters) - 1
	}
	c.counters[level]++
	for i := level + 1; i < len(c.counters); i++ {
		c.counters[i] = 0
	}
	parts := make([]string, level+1)
	for i := 0; i <= level; i++ {
		parts[i] = fmt.Sprintf("%d", c.counters[i])
	}
	return "s" + strings.Join(parts, ".")
}
