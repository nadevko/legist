package embedder

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

// OllamaClient calls Ollama's /api/embed with batched string input.
type OllamaClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Embedding  []float64   `json:"embedding"` // single-input fallback
}

// EmbedBatch returns one vector per input string. Fails if the response size does not match.
func (c *OllamaClient) EmbedBatch(ctx context.Context, model string, inputs []string) ([][]float64, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	base := strings.TrimRight(c.BaseURL, "/")
	u := base + "/api/embed"

	body, err := json.Marshal(ollamaEmbedRequest{Model: model, Input: inputs})
	if err != nil {
		return nil, fmt.Errorf("embed request marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embed read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed http %d: %s", resp.StatusCode, bytes.TrimSpace(raw))
	}

	var out ollamaEmbedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("embed json: %w", err)
	}

	if len(out.Embeddings) == len(inputs) {
		return out.Embeddings, nil
	}
	// Single vector for a single string (older or alternate Ollama shape)
	if len(inputs) == 1 && len(out.Embedding) > 0 && len(out.Embeddings) == 0 {
		return [][]float64{out.Embedding}, nil
	}
	return nil, fmt.Errorf("embed response: got %d embeddings for %d inputs", len(out.Embeddings), len(inputs))
}

// NewHTTPClient returns a client with the given per-request timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
