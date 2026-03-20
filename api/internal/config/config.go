package config

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"

	cenv "github.com/caarlos0/env/v11"
)

//go:embed metadata_prompt_default.txt
var embeddedMetadataLLMPrompt string

//go:embed qdrant_rag_prompt_prefix_default.txt
var embeddedQdrantRagPromptPrefix string

//go:embed rag_diff_smart_prompt_default.txt
var embeddedRagDiffSmartPrompt string

type Config struct {
	Addr       string
	DataPath   string
	JWTSecret  string
	PublicHost string
	BasePath   string
	Dev        bool

	QdrantHost     string
	QdrantGRPCPort string
	QdrantCollection string

	OllamaBaseURL string
	EmbedModel    string
	MetadataModel string
	AnalysisModel string

	// OpenAI-compatible provider (optional; used when LLM*_PROVIDER != "ollama").
	OpenAIBaseURL string
	OpenAIAPIKey  string

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

	// RAG-diff prompts.
	QdrantRagPromptPrefix string // loaded from QDRANT_RAG_PROMPT_PREFIX_FILE or embedded default
	RagDiffSmartPrompt    string // loaded from RAG_DIFF_SMART_PROMPT_FILE or embedded default

	// Embedding (Ollama /api/embed) — batch size, SSE throttle, HTTP timeout
	EmbedBatchSize                int
	EmbedShortChunkPrefixMaxChars int
	EmbedUseWeightPrefix          bool
	WeightRegexFile               string
	WeightCritical                float64
	WeightMain                    float64
	WeightStandard                float64
	WeightTechnical               float64
	WeightMaxCap                  float64
	EmbeddingContextHash          string // deprecated: embedder computes the real formula hash now
	EmbedProgressIntervalMS       int
	EmbedHTTPTimeoutMS            int

	// Diff matching (chunk matching)
	DiffMatchLowThreshold        float64
	DiffMatchHighThreshold       float64
	DiffMatchRegexFile          string
	DiffMatchProgressIntervalMS  int
}

func Load() *Config {
	var e struct {
		Addr       string `env:"ADDR"`
		Port       string `env:"PORT" envDefault:"8080"`
		DataPath   string `env:"DATA_PATH" envDefault:"../data"`
		JWTSecret  string `env:"JWT_SECRET" envDefault:"change-me-in-prod"`
		PublicHost string `env:"PUBLIC_HOST"`
		BasePath   string `env:"BASE_PATH"`
		Env        string `env:"ENV" envDefault:"dev"`

		QdrantHost     string `env:"QDRANT_HOST" envDefault:"127.0.0.1"`
		QdrantGRPCPort string `env:"QDRANT_GRPC_PORT" envDefault:"6334"`
		QdrantCollection string `env:"QDRANT_COLLECTION" envDefault:"legist_rag_chunks"`

		OllamaBaseURL string `env:"OLLAMA_BASE_URL" envDefault:"http://127.0.0.1:11434"`
		EmbedModel    string `env:"EMBED_MODEL" envDefault:"nomic-embed-text"`
		MetadataModel string `env:"METADATA_MODEL" envDefault:"qwen2.5:3b"`
		AnalysisModel string `env:"ANALYSIS_MODEL" envDefault:"qwen2.5:7b"`

		OpenAIBaseURL string `env:"OPENAI_BASE_URL"`
		OpenAIAPIKey  string `env:"OPENAI_API_KEY"`

		LLMMetadataProvider string `env:"LLM_METADATA" envDefault:"ollama"`
		LLMAnalysisProvider string `env:"LLM_ANALYSIS" envDefault:"ollama"`
		AnthropicAPIKey     string `env:"ANTHROPIC_API_KEY"`
		DeepseekAPIKey      string `env:"DEEPSEEK_API_KEY"`

		MetadataWindowSize   int `env:"LLM_METADATA_WINDOW" envDefault:"3000"`
		MetadataMaxRetries   int `env:"LLM_METADATA_RETRIES" envDefault:"3"`
		MetadataHTTPTimeoutMS int `env:"METADATA_LLM_HTTP_TIMEOUT_MS" envDefault:"60000"`

		EmbedBatchSize                int     `env:"EMBED_BATCH_SIZE" envDefault:"32"`
		EmbedShortChunkPrefixMaxChars int     `env:"EMBED_SHORT_CHUNK_PREFIX_MAX_CHARS" envDefault:"200"`
		EmbedUseWeightPrefix          bool    `env:"EMBED_USE_WEIGHT_PREFIX" envDefault:"true"`
		WeightRegexFile               string  `env:"WEIGHT_REGEX_FILE"`
		WeightCritical                float64 `env:"WEIGHT_CRITICAL" envDefault:"3.0"`
		WeightMain                    float64 `env:"WEIGHT_MAIN" envDefault:"2.0"`
		WeightStandard                float64 `env:"WEIGHT_STANDARD" envDefault:"1.0"`
		WeightTechnical               float64 `env:"WEIGHT_TECHNICAL" envDefault:"0.5"`
		WeightMaxCap                  float64 `env:"WEIGHT_MAX_CAP" envDefault:"3.0"`
		EmbedProgressIntervalMS       int     `env:"EMBED_PROGRESS_INTERVAL_MS" envDefault:"500"`
		EmbedHTTPTimeoutMS            int     `env:"EMBED_HTTP_TIMEOUT_MS" envDefault:"120000"`

		DiffMatchLowThreshold       float64 `env:"DIFF_MATCH_THRESHOLD_LOW" envDefault:"0.4"`
		DiffMatchHighThreshold      float64 `env:"DIFF_MATCH_THRESHOLD_HIGH" envDefault:"0.85"`
		DiffMatchRegexFile          string  `env:"DIFF_MATCH_REGEX_FILE"`
		DiffMatchProgressIntervalMS int     `env:"DIFF_MATCH_PROGRESS_INTERVAL_MS" envDefault:"500"`
	}
	_ = cenv.Parse(&e)

	addr := e.Addr
	if addr == "" {
		addr = fmt.Sprintf("0.0.0.0:%s", e.Port)
	}

	cfg := &Config{
		Addr:       addr,
		DataPath:   e.DataPath,
		JWTSecret:  e.JWTSecret,
		PublicHost: e.PublicHost,
		BasePath:   e.BasePath,
		Dev:        e.Env == "dev",

		QdrantHost:     e.QdrantHost,
		QdrantGRPCPort: e.QdrantGRPCPort,
		QdrantCollection: e.QdrantCollection,

		OllamaBaseURL: e.OllamaBaseURL,
		EmbedModel:    e.EmbedModel,
		MetadataModel: e.MetadataModel,
		AnalysisModel: e.AnalysisModel,

		OpenAIBaseURL: e.OpenAIBaseURL,
		OpenAIAPIKey:  e.OpenAIAPIKey,

		LLMMetadataProvider: e.LLMMetadataProvider,
		LLMAnalysisProvider: e.LLMAnalysisProvider,
		AnthropicAPIKey:     e.AnthropicAPIKey,
		DeepseekAPIKey:      e.DeepseekAPIKey,

		MetadataWindowSize:   e.MetadataWindowSize,
		MetadataMaxRetries:   e.MetadataMaxRetries,
		MetadataHTTPTimeoutMS: e.MetadataHTTPTimeoutMS,

		EmbedBatchSize:                e.EmbedBatchSize,
		EmbedShortChunkPrefixMaxChars: e.EmbedShortChunkPrefixMaxChars,
		EmbedUseWeightPrefix:          e.EmbedUseWeightPrefix,
		WeightRegexFile:               e.WeightRegexFile,
		WeightCritical:                e.WeightCritical,
		WeightMain:                    e.WeightMain,
		WeightStandard:                e.WeightStandard,
		WeightTechnical:               e.WeightTechnical,
		WeightMaxCap:                  e.WeightMaxCap,
		EmbedProgressIntervalMS:       e.EmbedProgressIntervalMS,
		EmbedHTTPTimeoutMS:            e.EmbedHTTPTimeoutMS,

		DiffMatchLowThreshold:       e.DiffMatchLowThreshold,
		DiffMatchHighThreshold:      e.DiffMatchHighThreshold,
		DiffMatchRegexFile:          e.DiffMatchRegexFile,
		DiffMatchProgressIntervalMS: e.DiffMatchProgressIntervalMS,
	}
	if cfg.EmbedBatchSize < 1 {
		cfg.EmbedBatchSize = 32
	}
	if cfg.EmbedShortChunkPrefixMaxChars < 0 {
		cfg.EmbedShortChunkPrefixMaxChars = 200
	}
	if cfg.WeightCritical <= 0 {
		cfg.WeightCritical = 3.0
	}
	if cfg.WeightMain <= 0 {
		cfg.WeightMain = 2.0
	}
	if cfg.WeightStandard <= 0 {
		cfg.WeightStandard = 1.0
	}
	if cfg.WeightTechnical <= 0 {
		cfg.WeightTechnical = 0.5
	}
	if cfg.WeightMaxCap <= 0 {
		cfg.WeightMaxCap = 3.0
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
	if cfg.DiffMatchLowThreshold < 0 {
		cfg.DiffMatchLowThreshold = 0
	}
	if cfg.DiffMatchLowThreshold > 1 {
		cfg.DiffMatchLowThreshold = 1
	}

	if cfg.DiffMatchHighThreshold < 0 {
		cfg.DiffMatchHighThreshold = 0
	}
	if cfg.DiffMatchHighThreshold > 1 {
		cfg.DiffMatchHighThreshold = 1
	}

	// Ensure low <= high, otherwise swap to preserve semantics.
	if cfg.DiffMatchLowThreshold > cfg.DiffMatchHighThreshold {
		cfg.DiffMatchLowThreshold, cfg.DiffMatchHighThreshold = cfg.DiffMatchHighThreshold, cfg.DiffMatchLowThreshold
	}
	if cfg.DiffMatchProgressIntervalMS < 0 {
		cfg.DiffMatchProgressIntervalMS = 500
	}
	// Keep for backward compatibility; embedder uses a new formula hash.
	cfg.EmbeddingContextHash = ""
	cfg.MetadataLLMPrompt = loadMetadataLLMPrompt(getEnv("METADATA_LLM_PROMPT_FILE", ""))
	cfg.QdrantRagPromptPrefix = loadPromptText(getEnv("QDRANT_RAG_PROMPT_PREFIX_FILE", ""), embeddedQdrantRagPromptPrefix)
	cfg.RagDiffSmartPrompt = loadPromptText(getEnv("RAG_DIFF_SMART_PROMPT_FILE", ""), embeddedRagDiffSmartPrompt)
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

func loadPromptText(file string, embedded string) string {
	file = strings.TrimSpace(file)
	if file != "" {
		if b, err := os.ReadFile(file); err == nil && len(b) > 0 {
			return string(b)
		}
	}
	return embedded
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

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
