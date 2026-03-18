package config

import "os"

type Config struct {
	Addr      string
	DBPath    string
	DataPath  string
	JWTSecret string
	Dev       bool

	QdrantHost     string
	QdrantGRPCPort string

	OllamaBaseURL string
	EmbedModel    string
	MetadataModel string

	LLMMetadataProvider string
	LLMAnalysisProvider string
	AnalysisModel       string
	AnthropicAPIKey     string
	DeepseekAPIKey      string
}

func Load() *Config {
	return &Config{
		Addr:      getEnv("ADDR", ":8080"),
		DBPath:    getEnv("DB_PATH", "legist.db"),
		DataPath:  getEnv("DATA_PATH", "../data"),
		JWTSecret: getEnv("JWT_SECRET", "change-me-in-prod"),
		Dev:       getEnv("ENV", "dev") == "dev",

		QdrantHost:     getEnv("QDRANT_HOST", "127.0.0.1"),
		QdrantGRPCPort: getEnv("QDRANT_GRPC_PORT", "6334"),

		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		EmbedModel:    getEnv("EMBED_MODEL", "nomic-embed-text"),
		MetadataModel: getEnv("METADATA_MODEL", "qwen2.5:3b"),

		LLMMetadataProvider: getEnv("LLM_METADATA", "ollama"),
		LLMAnalysisProvider: getEnv("LLM_ANALYSIS", "ollama"),
		AnalysisModel:       getEnv("ANALYSIS_MODEL", "qwen2.5:7b"),
		AnthropicAPIKey:     getEnv("ANTHROPIC_API_KEY", ""),
		DeepseekAPIKey:      getEnv("DEEPSEEK_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
