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
	Chunks      []Chunk        `json:"chunks,omitempty"`
	Children    []Section      `json:"children,omitempty"`
	References  []TLCReference `json:"references,omitempty"`
}

// Chunk points to a text fragment inside canonical plain text.
type Chunk struct {
	Text       string `json:"text"`
	PlainStart int    `json:"plain_start"` // rune offset, inclusive
	PlainEnd   int    `json:"plain_end"`   // rune offset, exclusive
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
	Sections  []Section
	PlainText string
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

// ParsedFile is what gets written to DATA_PATH/legistoso/{file_id}.
// Combines the Document section tree with AKN-shaped metadata.
type ParsedFile struct {
	FileID        string     `json:"file_id"`
	DocumentID    string     `json:"document_id"`
	Meta          ParsedMeta `json:"meta"`
	Sections      []Section  `json:"sections"`
	PlainTextPath string     `json:"plain_text_path"`
	PlainTextLen  int        `json:"plain_text_len"`

	ParsedAt      time.Time `json:"parsed_at"`
	ParserVersion string    `json:"parser_version"`
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
