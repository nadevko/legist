package main

import (
	"log"

	_ "github.com/nadevko/legist/docs"
	"github.com/nadevko/legist/internal/api"
	"github.com/nadevko/legist/internal/config"
	"github.com/nadevko/legist/internal/store"
)

func main() {
	cfg := config.Load()

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	log.Printf("database: %s", cfg.DBPath)

	srv := api.NewServer(cfg, db)

	log.Printf("starting server on %s (dev=%v)", cfg.Addr, cfg.Dev)
	log.Printf("base path: %s", cfg.BasePath)
	log.Printf("public host: %s", cfg.PublicHost)
	log.Printf("swagger: %s%s%s/swagger/index.html", cfg.BasePath, cfg.Addr, cfg.BasePath)
	log.Printf("llm: metadata=%s/%s analysis=%s/%s",
		cfg.LLMMetadataProvider, cfg.MetadataModel,
		cfg.LLMAnalysisProvider, cfg.AnalysisModel,
	)
	log.Printf("qdrant: %s:%s", cfg.QdrantHost, cfg.QdrantGRPCPort)
	log.Printf("ollama: %s", cfg.OllamaBaseURL)

	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
