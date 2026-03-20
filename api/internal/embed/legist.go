package embedder

import (
	"context"
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

	if pf.EmbeddingsCurrent(cfg.Model) {
		return nil
	}

	texts := parser.FlattenChunkContents(pf.Sections)
	if len(texts) == 0 {
		return nil
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
