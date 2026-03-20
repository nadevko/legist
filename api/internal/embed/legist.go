package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nadevko/legist/internal/parser"
)

// Config drives legist artifact embedding updates.
type Config struct {
	OllamaBaseURL string
	Model         string
	BatchSize     int
	// If >0 and chunk length is below this limit, prefix text as "{section_id}:{content}".
	ShortChunkPrefixMaxChars int
	UseWeightPrefix          bool
	WeightCritical           float64
	WeightMain               float64
	WeightStandard           float64
	WeightTechnical          float64
	WeightMaxCap             float64
	EmbeddingContextHash     string
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

	contextHash := cfg.EmbeddingContextHash
	if contextHash == "" {
		contextHash = computeEmbeddingContextHash(cfg)
	}
	if pf.EmbeddingsCurrent(cfg.Model, cfg.ShortChunkPrefixMaxChars, contextHash) {
		return nil
	}

	texts := make([]string, len(pf.ChunkContent))
	copy(texts, pf.ChunkContent)
	if len(texts) == 0 {
		return nil
	}
	weights := parser.FlattenChunkWeights(pf.Sections)
	if len(weights) != len(texts) {
		return fmt.Errorf("embed: chunk_content and chunk weight length mismatch (%d != %d)", len(texts), len(weights))
	}
	if cfg.UseWeightPrefix {
		for i := range texts {
			texts[i] = weightPrefix(weights[i], cfg) + " " + texts[i]
		}
	}

	total := len(texts)
	onProgress(parser.ParseProgress{
		Stage:       parser.StageEmbeddingStarted,
		Message:     fmt.Sprintf("embedding %d chunks", total),
		ChunksTotal: total,
	})

	client := &OllamaClient{
		BaseURL:    cfg.OllamaBaseURL,
		HTTPClient: NewHTTPClient(cfg.HTTPTimeout),
	}

	out := make([][]float64, 0, total)
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
		batch := texts[start:end]
		vecs, err := client.EmbedBatch(ctx, cfg.Model, batch)
		if err != nil {
			return fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
		}
		out = append(out, vecs...)
		done = len(out)
		emitProgress()
	}

	if len(out) != total {
		return fmt.Errorf("embed: expected %d vectors, got %d", total, len(out))
	}

	pf.ChunkEmbeddings = out
	pf.EmbeddingModel = cfg.Model
	pf.EmbeddingShortChunkPrefixMaxChars = cfg.ShortChunkPrefixMaxChars
	pf.EmbeddingContextHash = contextHash

	serialized, err := json.MarshalIndent(&pf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal legist json: %w", err)
	}
	if err := os.WriteFile(path, serialized, 0644); err != nil {
		return fmt.Errorf("write legist artifact: %w", err)
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

func computeEmbeddingContextHash(cfg Config) string {
	payload := fmt.Sprintf(
		"model=%s|weight_prefix=%t|short_prefix_max=%d|w_crit=%.6f|w_main=%.6f|w_std=%.6f|w_tech=%.6f|w_cap=%.6f",
		cfg.Model,
		cfg.UseWeightPrefix,
		cfg.ShortChunkPrefixMaxChars,
		cfg.WeightCritical,
		cfg.WeightMain,
		cfg.WeightStandard,
		cfg.WeightTechnical,
		cfg.WeightMaxCap,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
