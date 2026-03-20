package llm

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

type OpenAICompatibleProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type openaiChatRequest struct {
	Model     string `json:"model"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream    bool    `json:"stream"`
	Temperature float64 `json:"temperature,omitempty"`
}

type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewOpenAICompatibleProvider(baseURL, apiKey string, httpClient *http.Client) *OpenAICompatibleProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &OpenAICompatibleProvider{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}
}

func (p *OpenAICompatibleProvider) Generate(ctx context.Context, model string, prompt string) (string, error) {
	base := strings.TrimRight(p.BaseURL, "/")
	url := base + "/v1/chat/completions"

	reqPayload := openaiChatRequest{
		Model:  model,
		Stream: false,
		Temperature: 0,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: prompt},
		},
	}
	b, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("openai chat marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("openai chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(p.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai chat http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai chat read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai chat http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var or openaiChatResponse
	if err := json.Unmarshal(raw, &or); err != nil {
		return "", fmt.Errorf("openai chat decode: %w", err)
	}
	if len(or.Choices) == 0 {
		return "", fmt.Errorf("openai chat: empty choices")
	}
	content := or.Choices[0].Message.Content
	return strings.TrimSpace(content), nil
}

// ensure compile-time reference to time import when HTTPClient is nil.
var _ = time.Second

