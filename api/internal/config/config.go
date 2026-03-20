package config

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed metadata_prompt_default.txt
var embeddedMetadataLLMPrompt string

type Config struct {
	Addr       string
	DBPath     string
	DataPath   string
	JWTSecret  string
	PublicHost string
	BasePath   string
	Dev        bool

	QdrantHost     string
	QdrantGRPCPort string

	OllamaBaseURL string
	EmbedModel    string
	MetadataModel string
	AnalysisModel string

	LLMMetadataProvider string
	LLMAnalysisProvider string
	AnthropicAPIKey     string
	DeepseekAPIKey      string

	// LLM metadata extraction settings
	MetadataWindowSize int // chars of document text sent to metadata LLM (LLM_METADATA_WINDOW)
	MetadataMaxRetries int // max LLM attempts per file (LLM_METADATA_RETRIES)
	// MetadataLLMPrompt is loaded from METADATA_LLM_PROMPT_FILE or bundled default (see metadata_prompt_default.txt).
	MetadataLLMPrompt     string
	MetadataHTTPTimeoutMS int // per-request timeout for metadata Ollama calls (METADATA_LLM_HTTP_TIMEOUT_MS)

	// Embedding (Ollama /api/embed) — batch size, SSE throttle, HTTP timeout
	EmbedBatchSize           int
	EmbedProgressIntervalMS  int
	EmbedHTTPTimeoutMS       int
}

func Load() *Config {
	addr := getEnv("ADDR", "")
	if addr == "" {
		port := getEnv("PORT", "8080")
		addr = fmt.Sprintf("0.0.0.0:%s", port)
	}

	cfg := &Config{
		Addr:       addr,
		DBPath:     getEnv("DB_PATH", "legist.db"),
		DataPath:   getEnv("DATA_PATH", "../data"),
		JWTSecret:  getEnv("JWT_SECRET", "change-me-in-prod"),
		PublicHost: getEnv("PUBLIC_HOST", ""),
		BasePath:   getEnv("BASE_PATH", ""),
		Dev:        getEnv("ENV", "dev") == "dev",

		QdrantHost:     getEnv("QDRANT_HOST", "127.0.0.1"),
		QdrantGRPCPort: getEnv("QDRANT_GRPC_PORT", "6334"),

		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		EmbedModel:    getEnv("EMBED_MODEL", "nomic-embed-text"),
		MetadataModel: getEnv("METADATA_MODEL", "qwen2.5:3b"),
		AnalysisModel: getEnv("ANALYSIS_MODEL", "qwen2.5:7b"),

		LLMMetadataProvider: getEnv("LLM_METADATA", "ollama"),
		LLMAnalysisProvider: getEnv("LLM_ANALYSIS", "ollama"),
		AnthropicAPIKey:     getEnv("ANTHROPIC_API_KEY", ""),
		DeepseekAPIKey:      getEnv("DEEPSEEK_API_KEY", ""),

		MetadataWindowSize: getEnvInt("LLM_METADATA_WINDOW", 3000),
		MetadataMaxRetries: getEnvInt("LLM_METADATA_RETRIES", 3),

		MetadataHTTPTimeoutMS: getEnvInt("METADATA_LLM_HTTP_TIMEOUT_MS", 60000),

		EmbedBatchSize:          getEnvInt("EMBED_BATCH_SIZE", 32),
		EmbedProgressIntervalMS: getEnvInt("EMBED_PROGRESS_INTERVAL_MS", 500),
		EmbedHTTPTimeoutMS:      getEnvInt("EMBED_HTTP_TIMEOUT_MS", 120000),
	}
	if cfg.EmbedBatchSize < 1 {
		cfg.EmbedBatchSize = 32
	}
	if cfg.EmbedProgressIntervalMS < 0 {
		cfg.EmbedProgressIntervalMS = 500
	}
	if cfg.EmbedHTTPTimeoutMS < 1 {
		cfg.EmbedHTTPTimeoutMS = 120000
	}
	if cfg.MetadataHTTPTimeoutMS < 1 {
		cfg.MetadataHTTPTimeoutMS = 60000
	}
	cfg.MetadataLLMPrompt = loadMetadataLLMPrompt(getEnv("METADATA_LLM_PROMPT_FILE", ""))
	return cfg
}

func loadMetadataLLMPrompt(file string) string {
	file = strings.TrimSpace(file)
	if file != "" {
		if b, err := os.ReadFile(file); err == nil && len(b) > 0 {
			return string(b)
		}
	}
	return embeddedMetadataLLMPrompt
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
