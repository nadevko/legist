package main

import (
	"log"
	"os"
	"path/filepath"

	_ "github.com/nadevko/legist/docs"
	"github.com/nadevko/legist/internal/api"
	"github.com/nadevko/legist/internal/config"
	"github.com/nadevko/legist/internal/store"
)

func main() {
	cfg := config.Load()

	dbPath := filepath.Join(cfg.DataPath, "db.sqlite")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	log.Printf("database: %s", dbPath)

	// Seed global regex rules from template files when tables are empty.
	// Rules become admin-editable at runtime via CRUD endpoints (added in later steps).
	if err := store.NewRegexRulesStore(db).
		SeedFromTemplatesIfEmpty(cfg.WeightRegexFile, cfg.DiffMatchRegexFile); err != nil {
		log.Fatalf("seed regex rules: %v", err)
	}

	srv := api.NewServer(cfg, db)

	log.Printf("starting server on %s (dev=%v)", cfg.Addr, cfg.Dev)
	log.Printf("base path: %q", cfg.BasePath)
	log.Printf("public host: %q", cfg.PublicHost)
	log.Printf("llm: metadata=%s/%s analysis=%s/%s",
		cfg.LLMMetadataProvider, cfg.MetadataModel,
		cfg.LLMAnalysisProvider, cfg.AnalysisModel,
	)
	log.Printf("llm: window=%d chars, retries=%d",
		cfg.MetadataWindowSize, cfg.MetadataMaxRetries,
	)
	log.Printf("qdrant: %s:%s", cfg.QdrantHost, cfg.QdrantGRPCPort)
	log.Printf("ollama: %s", cfg.OllamaBaseURL)
	log.Printf("embed: model=%s batch=%d progress_interval_ms=%d http_timeout_ms=%d",
		cfg.EmbedModel, cfg.EmbedBatchSize, cfg.EmbedProgressIntervalMS, cfg.EmbedHTTPTimeoutMS)
	metaPrompt := "embedded default (metadata_prompt_default.txt)"
	if p := os.Getenv("METADATA_LLM_PROMPT_FILE"); p != "" {
		metaPrompt = p
	}
	log.Printf("metadata llm: prompt_file=%q http_timeout_ms=%d", metaPrompt, cfg.MetadataHTTPTimeoutMS)

	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
