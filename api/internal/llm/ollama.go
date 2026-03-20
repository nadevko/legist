package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaProvider struct {
	BaseURL    string
	HTTPClient    *http.Client
	PullHTTPClient *http.Client
}

type ollamaGenerateRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Stream bool     `json:"stream"`
	Options any     `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

// pullProgressEvent mirrors the streaming JSON objects from Ollama /api/pull.
// Ollama has a few different shapes across versions; keep decoding lenient.
type pullProgressEvent struct {
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

func NewOllamaProvider(baseURL string, httpClient *http.Client) *OllamaProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &OllamaProvider{
		BaseURL:         baseURL,
		HTTPClient:     httpClient,
		PullHTTPClient: &http.Client{Timeout: 0},
	}
}

func (p *OllamaProvider) Generate(ctx context.Context, model string, prompt string) (string, error) {
	base := strings.TrimRight(p.BaseURL, "/")
	genURL := base + "/api/generate"

	payload := ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ollama generate marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, genURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("ollama generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama generate read: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var or ollamaGenerateResponse
		if err := json.Unmarshal(raw, &or); err != nil {
			return "", fmt.Errorf("ollama generate decode: %w", err)
		}
		return or.Response, nil
	}

	underlyingErr := fmt.Errorf("ollama generate http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	if isOllamaModelMissing(raw, model) {
		// async_error policy: start pull and return an error with the latest progress snapshot.
		snap := p.startPullAndSnapshot(model, underlyingErr)
		return "", snap
	}

	return "", underlyingErr
}

func isOllamaModelMissing(raw []byte, model string) bool {
	s := strings.ToLower(string(raw))
	if strings.Contains(s, "model") {
		if strings.Contains(s, "not found") || strings.Contains(s, "not installed") || strings.Contains(s, "does not exist") {
			return true
		}
	}
	// Some Ollama responses only say "not found" without the "model" word.
	if strings.Contains(s, "not found") || strings.Contains(s, "does not exist") {
		return true
	}
	// Best-effort fallback: if the model name appears with "error".
	if model != "" && strings.Contains(s, strings.ToLower(model)) && (strings.Contains(s, "error") || strings.Contains(s, "missing")) {
		return true
	}
	return false
}

func (p *OllamaProvider) startPullAndSnapshot(model string, underlyingErr error) error {
	firstWait := 2 * time.Second

	progressCh := make(chan DownloadProgress, 1)
	// Detached pull so the user gets immediate feedback, and future requests can succeed once it finishes.
	go func() {
		_ = p.pullModelStream(model, func(dp DownloadProgress) {
			select {
			case progressCh <- dp:
			default:
			}
		})
	}()

	var snap DownloadProgress
	select {
	case snap = <-progressCh:
		// good enough
	case <-time.After(firstWait):
		// no progress yet; keep status empty
	}

	// Ensure we show "pull started" even if progress is empty.
	if snap.Status == "" {
		snap.Status = "pull started"
	}

	return &ErrModelMissing{
		Model:       model,
		PullStarted: true,
		Progress:    snap,
		UnderlyingErr: underlyingErr,
	}
}

func (p *OllamaProvider) pullModelStream(model string, onProgress func(DownloadProgress)) error {
	base := strings.TrimRight(p.BaseURL, "/")
	pullURL := base + "/api/pull"

	payload := map[string]any{
		"name":   model,
		"stream": true,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ollama pull marshal: %w", err)
	}

	// Use a detached context; we don't want cancellation of Generate to kill the pull.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, pullURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("ollama pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := p.PullHTTPClient
	if client == nil {
		client = p.HTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama pull http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama pull http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	sc := bufio.NewScanner(resp.Body)
	// allow larger json lines
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var last DownloadProgress
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev pullProgressEvent
		// Ollama uses newline-delimited JSON objects.
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// ignore unknown chunks
			continue
		}
		if ev.Error != "" {
			return errors.New(ev.Error)
		}

		last = DownloadProgress{
			Status:         ev.Status,
			CompletedBytes: ev.Completed,
			TotalBytes:     ev.Total,
		}
		if onProgress != nil && ev.Status != "" {
			onProgress(last)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("ollama pull scan: %w", err)
	}

	return nil
}

