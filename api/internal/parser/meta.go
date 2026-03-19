package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// LLM prompt
// ---------------------------------------------------------------------------

const metaPromptBase = `You are a legal metadata extractor for Belarusian normative legal acts (НПА).
Extract metadata from the document fragment and return ONLY a valid JSON object.
No markdown, no explanation, no code fences — raw JSON only.

JSON schema (null for unknown fields):
{
  "FRBRWork": {
    "FRBRdate":    string | null,  // REQUIRED adoption date "YYYY-MM-DD"
    "FRBRauthor":  string | null,  // issuing body: "Парламент"|"Президент"|"Совет Министров"|"Министерство ..."|...
    "FRBRcountry": string | null,  // always "by"
    "FRBRsubtype": string | null,  // REQUIRED: "закон"|"кодекс"|"декрет"|"указ"|"постановление"|"приказ"|"решение"|"конституция"|"иное"
    "FRBRnumber":  string | null,  // REQUIRED: "296-З", "15", "1234"
    "FRBRname":    string | null   // short title
  },
  "FRBRExpression": {
    "FRBRdate":          string | null,  // amendment date "YYYY-MM-DD" (if amended version)
    "FRBRlanguage":      string | null,  // "rus" or "bel"
    "FRBRversionNumber": string | null,  // "3" or "2024-01-15"
    "versionLabel":      string | null   // "ред. от 15.01.2024"
  },
  "publication": {
    "name":   string | null,
    "date":   string | null,  // "YYYY-MM-DD"
    "number": string | null,
    "showAs": string | null
  } | null,
  "lifecycle": [
    {
      "type":   "generation"|"amendment"|"repeal"|"suspension",
      "date":   string,
      "source": string | null
    }
  ],
  "passiveModifications": [
    {
      "type":    "substitution"|"insertion"|"deletion"|"split"|"merge",
      "source":  string,
      "section": string | null,
      "date":    string | null
    }
  ],
  "references": [
    {
      "eId":     string,
      "href":    string,
      "showAs":  string,
      "type":    "TLCOrganization"|"TLCPerson"|"TLCConcept"|"TLCEvent"|"TLCTerm",
      "article": string | null
    }
  ],
  "classification": {
    "keywords":   string[] | null,
    "dictionary": string | null
  } | null
}

Document fragment:
`

// ---------------------------------------------------------------------------
// npaLevel derivation — deterministic, never ask LLM
// ---------------------------------------------------------------------------

var subtypeToLevel = map[string]int{
	"конституция":   0,
	"закон":         2,
	"кодекс":        2,
	"декрет":        3,
	"указ":          3,
	"постановление": 4,
	"приказ":        6,
	"решение":       7,
	"иное":          8,
}

// DeriveNPALevel returns the НПА hierarchy level (0–9) from subtype and author.
// Author is used to disambiguate "постановление" across levels 4–7.
func DeriveNPALevel(subtype, author string) int {
	sub := strings.ToLower(strings.TrimSpace(subtype))
	auth := strings.ToLower(strings.TrimSpace(author))

	if sub == "конституция" {
		return 0
	}
	if sub == "постановление" {
		switch {
		case strings.Contains(auth, "совет министров") || strings.Contains(auth, "совмин"):
			return 4
		case strings.Contains(auth, "парламент") ||
			strings.Contains(auth, "национальное собрание") ||
			strings.Contains(auth, "верховный суд") ||
			strings.Contains(auth, "генеральная прокуратура"):
			return 5
		case strings.Contains(auth, "министерств") || strings.Contains(auth, "комитет"):
			return 6
		case strings.Contains(auth, "местн") || strings.Contains(auth, "исполнительный комитет"):
			return 7
		default:
			return 4
		}
	}
	if lvl, ok := subtypeToLevel[sub]; ok {
		return lvl
	}
	return 8
}

// ---------------------------------------------------------------------------
// LLMMeta — raw output from the LLM after validation
// ---------------------------------------------------------------------------

// LLMMeta holds the validated metadata extracted by the LLM.
// Fields are plain strings (empty = not extracted). npaLevel is always
// derived via DeriveNPALevel, never taken from LLM output.
type LLMMeta struct {
	// Work-level
	Subtype string
	Number  string
	Author  string
	Date    string // YYYY-MM-DD
	Country string
	Name    string

	// Expression-level
	VersionDate   string
	VersionNumber string
	VersionLabel  string
	Language      string

	// Publication
	PubName   string
	PubDate   string
	PubNumber string

	// Enrichment (stored in parsed.json only)
	Lifecycle            []LifecycleEvent
	PassiveModifications []PassiveModification
	References           []TLCReference
	Keywords             []string
}

// Score returns the number of non-empty critical fields.
func (m LLMMeta) Score() int {
	n := 0
	if m.Subtype != "" {
		n++
	}
	if m.Number != "" {
		n++
	}
	if m.Author != "" {
		n++
	}
	if m.Date != "" {
		n++
	}
	if m.VersionDate != "" {
		n++
	}
	if m.VersionLabel != "" {
		n++
	}
	if m.EffectiveDate() != "" {
		n++
	}
	if len(m.Keywords) > 0 {
		n++
	}
	if len(m.References) > 0 {
		n++
	}
	return n
}

// EffectiveDate is a convenience accessor — pulled from lifecycle generation event if present.
func (m LLMMeta) EffectiveDate() string {
	for _, e := range m.Lifecycle {
		if e.Type == "generation" {
			return e.Date
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Extractor
// ---------------------------------------------------------------------------

// MetaExtractorConfig holds extraction dependencies.
type MetaExtractorConfig struct {
	OllamaBaseURL string
	MetadataModel string
	MaxRetries    int
	HTTPTimeout   time.Duration
}

// ExtractMeta sends the combined document window to the metadata LLM.
// startWindow covers the document header (identity fields).
// endWindow covers the document tail (lifecycle, effective date, references).
// Returns the best merged result across all retries.
// ok=false means required Work fields (subtype, number, date) are still missing.
func ExtractMeta(ctx context.Context, cfg MetaExtractorConfig, startWindow, endWindow string) (LLMMeta, bool) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 60 * time.Second
	}

	client := &http.Client{Timeout: cfg.HTTPTimeout}

	combined := startWindow
	if endWindow != "" && endWindow != startWindow {
		combined = startWindow + "\n---\n" + endWindow
	}
	prompt := metaPromptBase + combined

	var best LLMMeta
	bestScore := -1

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		raw, err := callOllama(ctx, client, cfg.OllamaBaseURL, cfg.MetadataModel, prompt)
		if err != nil {
			continue
		}
		meta, score, err := parseAndValidateMeta(raw)
		if err != nil {
			continue
		}
		best = mergeLLMMeta(best, meta)
		if score > bestScore {
			bestScore = score
		}
		if hasRequiredFields(best) {
			break
		}
	}

	// Defaults for always-known fields.
	if best.Country == "" {
		best.Country = "by"
	}
	if best.Language == "" {
		best.Language = "rus"
	}

	return best, hasRequiredFields(best)
}

func hasRequiredFields(m LLMMeta) bool {
	return m.Subtype != "" && m.Number != "" && m.Date != ""
}

// mergeLLMMeta combines two LLMMeta values, preferring non-empty fields from b.
func mergeLLMMeta(a, b LLMMeta) LLMMeta {
	pick := func(av, bv string) string {
		if bv != "" {
			return bv
		}
		return av
	}
	a.Subtype = pick(a.Subtype, b.Subtype)
	a.Number = pick(a.Number, b.Number)
	a.Author = pick(a.Author, b.Author)
	a.Date = pick(a.Date, b.Date)
	a.Country = pick(a.Country, b.Country)
	a.Name = pick(a.Name, b.Name)
	a.VersionDate = pick(a.VersionDate, b.VersionDate)
	a.VersionNumber = pick(a.VersionNumber, b.VersionNumber)
	a.VersionLabel = pick(a.VersionLabel, b.VersionLabel)
	a.Language = pick(a.Language, b.Language)
	a.PubName = pick(a.PubName, b.PubName)
	a.PubDate = pick(a.PubDate, b.PubDate)
	a.PubNumber = pick(a.PubNumber, b.PubNumber)
	if len(b.Keywords) > len(a.Keywords) {
		a.Keywords = b.Keywords
	}
	if len(b.References) > len(a.References) {
		a.References = b.References
	}
	if len(b.Lifecycle) > len(a.Lifecycle) {
		a.Lifecycle = b.Lifecycle
	}
	if len(b.PassiveModifications) > len(a.PassiveModifications) {
		a.PassiveModifications = b.PassiveModifications
	}
	return a
}

// ---------------------------------------------------------------------------
// Parsing and validation
// ---------------------------------------------------------------------------

// rawLLMResponse mirrors the JSON schema in the prompt for lenient decoding.
type rawLLMResponse struct {
	FRBRWork struct {
		FRBRdate    *string `json:"FRBRdate"`
		FRBRauthor  *string `json:"FRBRauthor"`
		FRBRcountry *string `json:"FRBRcountry"`
		FRBRsubtype *string `json:"FRBRsubtype"`
		FRBRnumber  *string `json:"FRBRnumber"`
		FRBRname    *string `json:"FRBRname"`
	} `json:"FRBRWork"`
	FRBRExpression struct {
		FRBRdate          *string `json:"FRBRdate"`
		FRBRlanguage      *string `json:"FRBRlanguage"`
		FRBRversionNumber *string `json:"FRBRversionNumber"`
		VersionLabel      *string `json:"versionLabel"`
	} `json:"FRBRExpression"`
	Publication *struct {
		Name   *string `json:"name"`
		Date   *string `json:"date"`
		Number *string `json:"number"`
	} `json:"publication"`
	Lifecycle []struct {
		Type   string  `json:"type"`
		Date   string  `json:"date"`
		Source *string `json:"source"`
	} `json:"lifecycle"`
	PassiveModifications []struct {
		Type    string  `json:"type"`
		Source  string  `json:"source"`
		Section *string `json:"section"`
		Date    *string `json:"date"`
	} `json:"passiveModifications"`
	References []struct {
		EID     string  `json:"eId"`
		Href    string  `json:"href"`
		ShowAs  string  `json:"showAs"`
		Type    string  `json:"type"`
		Article *string `json:"article"`
	} `json:"references"`
	Classification *struct {
		Keywords   []string `json:"keywords"`
		Dictionary *string  `json:"dictionary"`
	} `json:"classification"`
}

func parseAndValidateMeta(raw string) (LLMMeta, int, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var r rawLLMResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return LLMMeta{}, 0, fmt.Errorf("json: %w", err)
	}

	var m LLMMeta
	score := 0

	str := func(p *string, maxLen int) string {
		if p == nil || *p == "" || len(*p) > maxLen {
			return ""
		}
		return strings.TrimSpace(*p)
	}

	m.Country = str(r.FRBRWork.FRBRcountry, 10)
	m.Subtype = str(r.FRBRWork.FRBRsubtype, 64)
	if m.Subtype != "" {
		score++
	}
	m.Number = str(r.FRBRWork.FRBRnumber, 32)
	if m.Number != "" {
		score++
	}
	m.Author = str(r.FRBRWork.FRBRauthor, 128)
	m.Date = validateDate(str(r.FRBRWork.FRBRdate, 10))
	if m.Date != "" {
		score++
	}
	m.Name = str(r.FRBRWork.FRBRname, 256)

	m.Language = str(r.FRBRExpression.FRBRlanguage, 8)
	m.VersionDate = validateDate(str(r.FRBRExpression.FRBRdate, 10))
	m.VersionNumber = str(r.FRBRExpression.FRBRversionNumber, 32)
	m.VersionLabel = str(r.FRBRExpression.VersionLabel, 64)

	if r.Publication != nil {
		m.PubName = str(r.Publication.Name, 256)
		m.PubDate = validateDate(str(r.Publication.Date, 10))
		m.PubNumber = str(r.Publication.Number, 64)
	}

	validLifecycleTypes := map[string]bool{
		"generation": true, "amendment": true, "repeal": true, "suspension": true,
	}
	for _, e := range r.Lifecycle {
		if !validLifecycleTypes[e.Type] {
			continue
		}
		d := validateDate(e.Date)
		if d == "" {
			continue
		}
		src := ""
		if e.Source != nil {
			src = *e.Source
		}
		m.Lifecycle = append(m.Lifecycle, LifecycleEvent{
			Type: e.Type, Date: d, Source: src,
		})
	}

	validModTypes := map[string]bool{
		"substitution": true, "insertion": true, "deletion": true, "split": true, "merge": true,
	}
	for _, pm := range r.PassiveModifications {
		if !validModTypes[pm.Type] || pm.Source == "" {
			continue
		}
		sec := ""
		if pm.Section != nil {
			sec = *pm.Section
		}
		d := ""
		if pm.Date != nil {
			d = validateDate(*pm.Date)
		}
		m.PassiveModifications = append(m.PassiveModifications, PassiveModification{
			Type: pm.Type, Source: pm.Source, Section: sec, Date: d,
		})
	}

	validTLCTypes := map[string]bool{
		"TLCOrganization": true, "TLCPerson": true,
		"TLCConcept": true, "TLCEvent": true, "TLCTerm": true,
	}
	for _, ref := range r.References {
		if !validTLCTypes[ref.Type] || ref.ShowAs == "" || len(ref.ShowAs) > 256 {
			continue
		}
		art := ""
		if ref.Article != nil {
			art = *ref.Article
		}
		m.References = append(m.References, TLCReference{
			EID: ref.EID, Href: ref.Href, ShowAs: ref.ShowAs, Type: ref.Type, Article: art,
		})
	}

	if r.Classification != nil {
		for _, kw := range r.Classification.Keywords {
			if kw != "" && len(kw) <= 64 {
				m.Keywords = append(m.Keywords, kw)
			}
			if len(m.Keywords) >= 8 {
				break
			}
		}
	}

	return m, score, nil
}

func validateDate(s string) string {
	if len(s) != 10 {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return ""
	}
	return s
}

// ---------------------------------------------------------------------------
// Ollama HTTP client
// ---------------------------------------------------------------------------

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func callOllama(ctx context.Context, client *http.Client, baseURL, model, prompt string) (string, error) {
	body, _ := json.Marshal(ollamaRequest{Model: model, Prompt: prompt, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/generate",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	var or ollamaResponse
	if err = json.Unmarshal(raw, &or); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return or.Response, nil
}
