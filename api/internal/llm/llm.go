package llm

import "context"

// Provider abstracts an LLM backend (Ollama, OpenAI-compatible, ...).
// Prompt is passed as a single string for the current generation use-cases.
type Provider interface {
	Generate(ctx context.Context, model string, prompt string) (string, error)
}

