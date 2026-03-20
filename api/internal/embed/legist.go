package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/nadevko/legist/internal/parser"
	"github.com/nadevko/legist/internal/store"
)

// Config drives legist artifact embedding updates.
type Config struct {
	OllamaBaseURL string
	Model         string
	BatchSize     int
	ShortChunkPrefixMaxChars int
	UseWeightPrefix          bool
	// Weight rules are used for both:
	// - computing per-chunk weights (and [W*] prefix)
	// - building per-chunk input hashes for lazy re-embed.
	WeightRules []store.RegexWeightRule
	// Omit rules affect Stage3 risk-zone matching only, but they are included
	// in embedding_context_hash for a consistent "formula of meaning".
	OmitRules []store.RegexOmitRule
	WeightCritical           float64
	WeightMain               float64
	WeightStandard           float64
	WeightTechnical          float64
	WeightMaxCap             float64
	// Min time between throttled progress callbacks after the first emit (0 = only gate on percent change).
	ProgressInterval time.Duration
	HTTPTimeout      time.Duration
}

// LegistEmbedIfNeeded loads application/lessed JSON at path, embeds each chunk's content when needed,
// writes the file back, and reports progress via onProgress.
func LegistEmbedIfNeeded(ctx context.Context, path string, cfg Config, onProgress parser.ProgressFunc) error {
	if cfg.BatchSize <= 0 {
		return fmt.Errorf("embed batch size must be positive")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read legist artifact: %w", err)
	}

	var pf parser.ParsedFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return fmt.Errorf("parse legist json: %w", err)
	}

	// Build stable rule "content" for embedding_context_hash.
	// Only rule fields that affect behavior are included (regex, enabled, weight).
	weightHashItems := make([]struct {
		Regex   string  `json:"regex"`
		Enabled bool    `json:"enabled"`
		Weight  float64 `json:"weight"`
	}, 0, len(cfg.WeightRules))
	for _, r := range cfg.WeightRules {
		weightHashItems = append(weightHashItems, struct {
			Regex   string  `json:"regex"`
			Enabled bool    `json:"enabled"`
			Weight  float64 `json:"weight"`
		}{
			Regex:   r.Regex,
			Enabled: r.Enabled,
			Weight:  r.Weight,
		})
	}
	weightRulesContentBytes, err := json.Marshal(weightHashItems)
	if err != nil {
		return fmt.Errorf("marshal weight rules hash items: %w", err)
	}
	weightRulesContent := string(weightRulesContentBytes)

	omitHashItems := make([]struct {
		Regex   string `json:"regex"`
		Enabled bool   `json:"enabled"`
	}, 0, len(cfg.OmitRules))
	for _, r := range cfg.OmitRules {
		omitHashItems = append(omitHashItems, struct {
			Regex   string `json:"regex"`
			Enabled bool   `json:"enabled"`
		}{
			Regex:   r.Regex,
			Enabled: r.Enabled,
		})
	}
	omitRulesContentBytes, err := json.Marshal(omitHashItems)
	if err != nil {
		return fmt.Errorf("marshal omit rules hash items: %w", err)
	}
	omitRulesContent := string(omitRulesContentBytes)

	compiledWeightRules := make([]compiledWeightRule, 0, len(cfg.WeightRules))
	for _, r := range cfg.WeightRules {
		if !r.Enabled {
			continue
		}
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			return fmt.Errorf("compile enabled weight rule regex: %w", err)
		}
		compiledWeightRules = append(compiledWeightRules, compiledWeightRule{re: re, weight: r.Weight})
	}

	omitPatterns := make([]string, 0, len(cfg.OmitRules))
	for _, r := range cfg.OmitRules {
		if !r.Enabled {
			continue
		}
		omitPatterns = append(omitPatterns, r.Regex)
	}
	compiledOmitRules, err := parser.CompileOmitRules(omitPatterns)
	if err != nil {
		return fmt.Errorf("compile enabled omit rule regex: %w", err)
	}

	currentFormulaHash := computeEmbeddingFormulaHash(cfg, weightRulesContent, omitRulesContent)

	if pf.EmbeddingsCurrent(cfg.Model, cfg.ShortChunkPrefixMaxChars, currentFormulaHash) {
		return nil
	}

	texts := pf.ChunkContent
	n := len(texts)
	if n == 0 {
		return nil
	}

	// Ensure embeddings slice exists and has stable size.
	if len(pf.ChunkEmbeddings) != n {
		pf.ChunkEmbeddings = make([][]float64, n)
	}
	if len(pf.ChunkEmbeddingInputHashes) != n {
		pf.ChunkEmbeddingInputHashes = make([]string, n)
	}

	weights := make([]float64, n)
	inputHashes := make([]string, n)
	inputTexts := make([]string, n) // input string passed to embedder (based on cleaned text) for hash and selective embedding

	for i := 0; i < n; i++ {
		clean := parser.CleanText(texts[i], compiledOmitRules)

		// Weight classification and embedding input must both use cleaned text.
		w := classifyWeight(clean, compiledWeightRules, cfg)
		weights[i] = w

		// If cleaned chunk is empty, embed empty content (no weight prefix),
		// since stage3 will skip such chunks anyway.
		var input string
		switch {
		case cfg.UseWeightPrefix && clean != "":
			input = weightPrefix(w, cfg) + " " + clean
		case cfg.UseWeightPrefix && clean == "":
			input = ""
		default:
			input = clean
		}

		inputTexts[i] = input
		sum := sha256.Sum256([]byte(input))
		inputHashes[i] = hex.EncodeToString(sum[:])
	}

	// Update chunk weights in the parsed artifact so Stage3 weighted similarity is correct.
	applyWeightsToSections(&pf.Sections, weights)

	toEmbed := make([]int, 0, n)
	for i := 0; i < n; i++ {
		// len(nil) is 0, so the nil check is redundant here.
		needVec := len(pf.ChunkEmbeddings[i]) == 0
		needHash := pf.ChunkEmbeddingInputHashes[i] != inputHashes[i]
		if needVec || needHash {
			toEmbed = append(toEmbed, i)
		}
	}

	pf.EmbeddingModel = cfg.Model
	pf.EmbeddingShortChunkPrefixMaxChars = cfg.ShortChunkPrefixMaxChars
	pf.EmbeddingContextHash = currentFormulaHash
	pf.ChunkEmbeddingInputHashes = inputHashes

	// Nothing to embed, but we still need to persist weight + hash updates.
	if len(toEmbed) == 0 {
		onProgress(parser.ParseProgress{
			Stage:            parser.StageEmbeddingDone,
			Message:          "embeddings already up-to-date",
			EmbeddingPercent: 100,
			ChunksEmbedded:   0,
			ChunksTotal:      0,
		})
		return writeParsedFile(path, &pf)
	}

	total := len(toEmbed)
	onProgress(parser.ParseProgress{
		Stage:       parser.StageEmbeddingStarted,
		Message:     fmt.Sprintf("embedding %d chunks", total),
		ChunksTotal: total,
	})

	client := &OllamaClient{
		BaseURL:    cfg.OllamaBaseURL,
		HTTPClient: NewHTTPClient(cfg.HTTPTimeout),
	}

	done := 0
	var lastEmit time.Time
	lastSentPercent := -1
	interval := cfg.ProgressInterval
	if interval < 0 {
		interval = 0
	}

	emitProgress := func() {
		pct := 0
		if total > 0 {
			pct = (done * 100) / total
		}
		if pct == lastSentPercent {
			return
		}
		now := time.Now()
		if !lastEmit.IsZero() && now.Sub(lastEmit) < interval {
			return
		}
		lastEmit = now
		lastSentPercent = pct
		onProgress(parser.ParseProgress{
			Stage:            parser.StageEmbedding,
			Message:          fmt.Sprintf("embedded %d / %d chunks", done, total),
			EmbeddingPercent: pct,
			ChunksEmbedded:   done,
			ChunksTotal:      total,
		})
	}

	for start := 0; start < total; start += cfg.BatchSize {
		end := start + cfg.BatchSize
		if end > total {
			end = total
		}
		batchIdx := toEmbed[start:end]
		batchTexts := make([]string, 0, len(batchIdx))
		for _, idx := range batchIdx {
			batchTexts = append(batchTexts, inputTexts[idx])
		}
		vecs, err := client.EmbedBatch(ctx, cfg.Model, batchTexts)
		if err != nil {
			return fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
		}

		for j := range vecs {
			idx := batchIdx[j]
			pf.ChunkEmbeddings[idx] = vecs[j]
		}
		done += len(vecs)
		emitProgress()
	}

	if err := writeParsedFile(path, &pf); err != nil {
		return err
	}

	onProgress(parser.ParseProgress{
		Stage:            parser.StageEmbeddingDone,
		Message:          "embeddings written",
		EmbeddingPercent: 100,
		ChunksEmbedded:   total,
		ChunksTotal:      total,
	})
	return nil
}

func weightPrefix(w float64, cfg Config) string {
	crit := cfg.WeightCritical
	main := cfg.WeightMain
	tech := cfg.WeightTechnical
	if crit <= 0 {
		crit = 3.0
	}
	if main <= 0 {
		main = 2.0
	}
	if tech <= 0 {
		tech = 0.5
	}
	switch {
	case w >= crit:
		return "[W3]"
	case w >= main:
		return "[W2]"
	case w <= tech:
		return "[W05]"
	default:
		return "[W1]"
	}
}


func computeEmbeddingFormulaHash(cfg Config, weightRulesContent, omitRulesContent string) string {
	// Formula of Meaning: embedding input depends on model, prefix mode and
	// weight/diff regex rules (even if some of them don't influence the actual vectors).
	payload := fmt.Sprintf(
		"model=%s|use_weight_prefix=%t|short_prefix_max=%d|w_crit=%.6f|w_main=%.6f|w_std=%.6f|w_tech=%.6f|w_cap=%.6f|prefix_scheme=v1|weight_rules=%s|omit_rules=%s",
		cfg.Model,
		cfg.UseWeightPrefix,
		cfg.ShortChunkPrefixMaxChars,
		cfg.WeightCritical,
		cfg.WeightMain,
		cfg.WeightStandard,
		cfg.WeightTechnical,
		cfg.WeightMaxCap,
		weightRulesContent,
		omitRulesContent,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

type compiledWeightRule struct {
	re     *regexp.Regexp
	weight float64
}

func classifyWeight(chunkText string, rules []compiledWeightRule, cfg Config) float64 {
	best := cfg.WeightStandard
	for _, r := range rules {
		if !r.re.MatchString(chunkText) {
			continue
		}
		if r.weight > best {
			best = r.weight
		}
	}

	// Apply safety bounds.
	if cfg.WeightMaxCap > 0 && best > cfg.WeightMaxCap {
		best = cfg.WeightMaxCap
	}
	if best <= 0 {
		best = cfg.WeightStandard
	}
	return best
}

func applyWeightsToSections(sections *[]parser.Section, weights []float64) {
	// DFS chunk_content order must match DFS order in parser.Section[].Chunks[].
	i := 0

	var walk func(sec *parser.Section)
	walk = func(sec *parser.Section) {
		if sec == nil {
			return
		}
		for ci := range sec.Chunks {
			if i >= len(weights) {
				return
			}
			sec.Chunks[ci].Weight = weights[i]
			i++
		}
		for ci := range sec.Children {
			walk(&sec.Children[ci])
		}
	}

	for si := range *sections {
		walk(&(*sections)[si])
	}
}

func writeParsedFile(path string, pf *parser.ParsedFile) error {
	serialized, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal legist json: %w", err)
	}
	if err := os.WriteFile(path, serialized, 0644); err != nil {
		return fmt.Errorf("write legist artifact: %w", err)
	}
	return nil
}
