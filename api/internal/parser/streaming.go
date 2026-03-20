package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SSE stage constants.
const (
	StageParsing      = "parsing_started"
	StageLLMRequested = "llm_requested"
	StageLLMSkipped   = "llm_skipped"
	StageLLMDone      = "llm_done"
	StageSaving            = "saving"
	StageEmbeddingStarted  = "embedding_started"
	StageEmbedding         = "embedding"
	StageEmbeddingDone     = "embedding_done"
	StageMatching          = "matching"
	StageDone              = "done"
	StageFailed            = "failed"
)

// ParseProgress is emitted on each pipeline stage transition.
type ParseProgress struct {
	Stage         string   `json:"stage"`
	Message       string   `json:"message"`
	SectionsFound int      `json:"sections_found,omitempty"`
	MetaScore     int      `json:"meta_score,omitempty"`
	MetaOK        *bool    `json:"meta_ok,omitempty"`
	MissingFields []string `json:"missing_fields,omitempty"`
	Error         string   `json:"error,omitempty"`

	EmbeddingPercent int `json:"embedding_percent,omitempty"`
	ChunksEmbedded   int `json:"chunks_embedded,omitempty"`
	ChunksTotal      int `json:"chunks_total,omitempty"`
}

type ProgressFunc func(ParseProgress)

type metaResult struct {
	meta LLMMeta
	ok   bool
}

// PipelineConfig carries all inputs for the processing pipeline.
type PipelineConfig struct {
	// File identifiers
	FileID     string
	DocumentID string
	Path       string
	DataPath   string
	MimeType   string

	// AKN Work-level fields already known (from Document at pipeline start).
	// These are what we have BEFORE LLM. Empty string = unknown.
	KnownSubtype string
	KnownNumber  string
	KnownAuthor  string
	KnownDate    string
	KnownCountry string
	KnownName    string

	// AKN Expression-level fields already known (from File).
	KnownVersionDate   string
	KnownVersionNumber string
	KnownVersionLabel  string
	KnownLanguage      string
	KnownPubName       string
	KnownPubDate       string
	KnownPubNumber     string

	// LLM settings
	MetaExtractor MetaExtractorConfig
	WindowSize    int
	WeightConfig  ChunkWeightConfig

	ParserVersion string
}

// PipelineResult carries extracted/confirmed values back to the caller.
// The caller is responsible for persisting these into Document and File.
type PipelineResult struct {
	// Work-level (go into Document)
	Subtype  string
	Number   string
	Author   string
	Date     string
	Country  string
	Name     string
	NPALevel int

	// Expression-level (go into File)
	VersionDate   string
	VersionNumber string
	VersionLabel  string
	Language      string
	PubName       string
	PubDate       string
	PubNumber     string

	// Enrichment extracted by LLM (stored in parsed.json only, not DB columns)
	Lifecycle            []LifecycleEvent
	PassiveModifications []PassiveModification
	References           []TLCReference
	Keywords             []string

	// ParsedFilePath is the path to the written application/lessed artifact.
	ParsedFilePath string
	PlainTextPath  string
}

// Run executes the full parse+extract pipeline and returns PipelineResult.
// Returns error only for hard failures (parse error, missing required fields).
func Run(ctx context.Context, cfg PipelineConfig, onProgress ProgressFunc) (*PipelineResult, error) {
	emit := func(stage, msg string, mods ...func(*ParseProgress)) {
		p := ParseProgress{Stage: stage, Message: msg}
		for _, fn := range mods {
			fn(&p)
		}
		onProgress(p)
	}
	fail := func(msg string, err error, missing ...[]string) error {
		p := ParseProgress{Stage: StageFailed, Message: msg, Error: err.Error()}
		if len(missing) > 0 {
			p.MissingFields = missing[0]
		}
		onProgress(p)
		return err
	}

	res := &PipelineResult{
		// Start with whatever was already known.
		Subtype:       cfg.KnownSubtype,
		Number:        cfg.KnownNumber,
		Author:        cfg.KnownAuthor,
		Date:          cfg.KnownDate,
		Country:       cfg.KnownCountry,
		Name:          cfg.KnownName,
		VersionDate:   cfg.KnownVersionDate,
		VersionNumber: cfg.KnownVersionNumber,
		VersionLabel:  cfg.KnownVersionLabel,
		Language:      cfg.KnownLanguage,
		PubName:       cfg.KnownPubName,
		PubDate:       cfg.KnownPubDate,
		PubNumber:     cfg.KnownPubNumber,
	}

	// --- 1. Parse document structure ---
	emit(StageParsing, "parsing document structure")

	raw, err := ParseFile(cfg.Path, cfg.MimeType)
	if err != nil {
		return nil, fail("document parsing failed", err)
	}
	total := countSections(raw.Sections)
	emit(StageParsing, fmt.Sprintf("parsed %d sections", total), func(p *ParseProgress) {
		p.SectionsFound = total
	})

	// --- 2. Decide whether LLM is needed ---
	needLLM := res.Subtype == "" || res.Number == "" || res.Author == "" || res.Date == "" ||
		res.VersionDate == "" || res.Language == ""

	var metaCh chan metaResult
	if !needLLM {
		emit(StageLLMSkipped, "all required metadata provided explicitly")
	} else {
		// --- 3. Start metadata extraction once first N chars are available ---
		size := cfg.WindowSize
		if size <= 0 {
			size = 3000
		}
		startW := firstNRunes(raw.PlainText, size)
		endW := startW
		emit(StageLLMRequested, fmt.Sprintf("extracting AKN metadata (chars=%d)", len(startW)))
		metaCh = make(chan metaResult, 1)
		go func() {
			meta, ok := ExtractMeta(ctx, cfg.MetaExtractor, startW, endW)
			metaCh <- metaResult{meta, ok}
		}()
	}

	// LLM may still be running while full parse is already completed.
	if needLLM && metaCh != nil {
		mr := <-metaCh
		score := mr.meta.Score()
		okVal := mr.ok
		emit(StageLLMDone,
			fmt.Sprintf("metadata extracted: %d fields", score),
			func(p *ParseProgress) {
				p.MetaScore = score
				p.MetaOK = &okVal
			},
		)
		m := mr.meta
		if res.Subtype == "" {
			res.Subtype = m.Subtype
		}
		if res.Number == "" {
			res.Number = m.Number
		}
		if res.Author == "" {
			res.Author = m.Author
		}
		if res.Date == "" {
			res.Date = m.Date
		}
		if res.Country == "" {
			res.Country = m.Country
		}
		if res.Name == "" {
			res.Name = m.Name
		}
		if res.VersionDate == "" {
			res.VersionDate = m.VersionDate
		}
		if res.VersionNumber == "" {
			res.VersionNumber = m.VersionNumber
		}
		if res.VersionLabel == "" {
			res.VersionLabel = m.VersionLabel
		}
		if res.Language == "" {
			res.Language = m.Language
		}
		if res.PubName == "" {
			res.PubName = m.PubName
		}
		if res.PubDate == "" {
			res.PubDate = m.PubDate
		}
		if res.PubNumber == "" {
			res.PubNumber = m.PubNumber
		}
		res.Lifecycle = m.Lifecycle
		res.PassiveModifications = m.PassiveModifications
		res.References = m.References
		res.Keywords = m.Keywords
	}

	// --- 6. Validate required Work fields ---
	var missing []string
	if res.Subtype == "" {
		missing = append(missing, "subtype")
	}
	if res.Number == "" {
		missing = append(missing, "number")
	}
	if res.Author == "" {
		missing = append(missing, "author")
	}
	if res.Date == "" {
		missing = append(missing, "date")
	}

	if len(missing) > 0 {
		return nil, fail(
			fmt.Sprintf("required fields missing: %v", missing),
			fmt.Errorf("required document fields missing: %v", missing),
			missing,
		)
	}

	// --- 7. Derive NPALevel deterministically ---
	if res.Country == "" {
		res.Country = "by"
	}
	if res.Language == "" {
		res.Language = "rus"
	}
	res.NPALevel = DeriveNPALevel(res.Subtype, res.Author)

	// --- 8. Assemble and write artifacts ---
	emit(StageSaving, "writing plain and lessed artifacts")
	assignChunkWeights(raw, cfg.WeightConfig)

	pf := &ParsedFile{
		FileID:     cfg.FileID,
		DocumentID: cfg.DocumentID,
		Meta: ParsedMeta{
			Subtype:              res.Subtype,
			Number:               res.Number,
			Author:               res.Author,
			Date:                 res.Date,
			Country:              res.Country,
			Name:                 res.Name,
			NPALevel:             res.NPALevel,
			VersionDate:          res.VersionDate,
			VersionNumber:        res.VersionNumber,
			VersionLabel:         res.VersionLabel,
			Language:             res.Language,
			PubName:              res.PubName,
			PubDate:              res.PubDate,
			PubNumber:            res.PubNumber,
			Lifecycle:            res.Lifecycle,
			PassiveModifications: res.PassiveModifications,
			References:           res.References,
			Keywords:             res.Keywords,
		},
		Sections:      raw.Sections,
		ChunkContent:  raw.ChunkContent,
		PlainTextPath: filepath.ToSlash(filepath.Join("plain", cfg.FileID)),
		PlainTextLen:  len([]rune(raw.PlainText)),
		ParsedAt:      time.Now().UTC(),
		ParserVersion: cfg.ParserVersion,
	}

	outPath, plainPath, err := writeArtifacts(cfg, pf, raw.PlainText)
	if err != nil {
		return nil, fail("failed to write artifacts", err)
	}
	res.ParsedFilePath = outPath
	res.PlainTextPath = plainPath

	emit(StageDone, "document ready", func(p *ParseProgress) {
		p.SectionsFound = total
	})
	return res, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func countSections(ss []Section) int {
	n := len(ss)
	for _, s := range ss {
		n += countSections(s.Children)
	}
	return n
}

func writeArtifacts(cfg PipelineConfig, parsed any, plain string) (parsedPath, plainPath string, err error) {
	plainPath = filepath.Join(cfg.DataPath, "plain", cfg.FileID)
	if err = os.MkdirAll(filepath.Dir(plainPath), 0755); err != nil {
		return "", "", fmt.Errorf("mkdir plain: %w", err)
	}
	if err = os.WriteFile(plainPath, []byte(plain), 0644); err != nil {
		return "", "", fmt.Errorf("write plain: %w", err)
	}

	parsedPath = filepath.Join(cfg.DataPath, "lessed", cfg.FileID)
	if err = os.MkdirAll(filepath.Dir(parsedPath), 0755); err != nil {
		return "", "", fmt.Errorf("mkdir lessed: %w", err)
	}
	data, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal: %w", err)
	}
	if err = os.WriteFile(parsedPath, data, 0644); err != nil {
		return "", "", fmt.Errorf("write: %w", err)
	}
	return parsedPath, plainPath, nil
}
