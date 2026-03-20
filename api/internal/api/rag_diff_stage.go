package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	embedder "github.com/nadevko/legist/internal/embed"
	"github.com/nadevko/legist/internal/qdrant"
	"github.com/nadevko/legist/internal/parser"
	"github.com/nadevko/legist/internal/store"
)

type ragLink struct {
	Source     *string `json:"source"`
	Law        *string `json:"law"`
	Article    *string `json:"article"`
	Section    *string `json:"section"`
	URL        *string `json:"url"`
	Text       *string `json:"text"`
}

type ragDiffSmartResult struct {
	Message    string      `json:"message"`
	Assessment string      `json:"assessment"`
	Links      []ragLink  `json:"links"`
}

type ragDiffReport struct {
	DiffID    string                   `json:"diff_id"`
	CreatedAt string                   `json:"created_at"`
	Changes   []ragDiffChangeReport   `json:"changes"`
}

type ragDiffChangeReport struct {
	LeftChunkIndex  int     `json:"left_chunk_index"`
	RightChunkIndex int     `json:"right_chunk_index"`
	Similarity       float64 `json:"similarity"`
	LeftWeight       float64 `json:"left_weight"`
	Was              string  `json:"was"`
	Is               string  `json:"is"`
	Smart            *ragDiffSmartResult `json:"smart,omitempty"`
	QdrantHits       []qdrantHitSlim     `json:"qdrant_hits,omitempty"`
}

type qdrantHitSlim struct {
	Score   float32            `json:"score"`
	Payload map[string]any    `json:"payload,omitempty"`
}

func (s *Server) runRagDiffStage(ctx context.Context, diffID string, d *store.Diff, doc *store.Document,
	leftPF, rightPF parser.ParsedFile,
	leftContents, rightContents []string,
	leftWeights, rightWeights []float64,
	leftMatches []*int,
	leftMatchSims []float64,
) {
	// This stage is best-effort: if anything fails, we still proceed with diff_done.
	// The report file is written only when at least smart results exist.

	defer func() {
		_ = recover() // avoid crashing diff SSE path
	}()

	leftSectionIDs := parser.FlattenChunkSectionIDs(leftPF.Sections)
	rightSectionIDs := parser.FlattenChunkSectionIDs(rightPF.Sections)

	type candidate struct {
		leftI int
		rightJ int
		sim float64
		weight float64
		wasText string
		isText string
		leftSectionID string
		rightSectionID string
	}

	const weightThresholdForSmart = 2.0
	const simThresholdForSmart = 0.8

	cands := make([]candidate, 0)
	for i, jPtr := range leftMatches {
		if jPtr == nil {
			continue
		}
		j := *jPtr
		sim := leftMatchSims[i]

		w := leftWeights[i]
		if w <= 0 {
			w = 1.0
		}

		if w < weightThresholdForSmart && sim >= simThresholdForSmart {
			continue
		}

		c := candidate{
			leftI: i,
			rightJ: j,
			sim:   sim,
			weight: w,
			wasText: strings.TrimSpace(leftContents[i]),
			isText: strings.TrimSpace(rightContents[j]),
		}
		if i < len(leftSectionIDs) {
			c.leftSectionID = leftSectionIDs[i]
		}
		if j < len(rightSectionIDs) {
			c.rightSectionID = rightSectionIDs[j]
		}
		cands = append(cands, c)
	}

	if len(cands) == 0 {
		return
	}

	// Qdrant search settings.
	const topK uint64 = 8
	const maxPayloadContextHits = 6
	withPayloadKeys := []string{
		// payload schema (planned/expected): text + source/article + any other link-ish fields
		"text", "source", "article", "level",
		"law", "section", "url",
		"doc_id", "document_id", "file_id", "contract_type", "jurisdiction",
	}

	qClient, err := qdrant.NewClient(s.cfg.QdrantHost, s.cfg.QdrantGRPCPort)
	if err != nil {
		return
	}
	_ = qClient.Close()

	ollamaClient := &embedder.OllamaClient{
		BaseURL:    s.cfg.OllamaBaseURL,
		HTTPClient: embedder.NewHTTPClient(time.Duration(s.cfg.EmbedHTTPTimeoutMS) * time.Millisecond),
	}

	now := time.Now().UTC().Format(time.RFC3339)
	report := ragDiffReport{
		DiffID:    diffID,
		CreatedAt: now,
		Changes:   make([]ragDiffChangeReport, 0, len(cands)),
	}

	type candidateWork struct {
		c         candidate
		queries   string
		embeds    []float64
		hits      []qdrantHitSlim
		smart     *ragDiffSmartResult
	}

	maxPromptChars := s.cfg.MetadataWindowSize * 2
	if maxPromptChars < 4000 {
		maxPromptChars = 8000
	}

	for start := 0; start < len(cands); start += s.cfg.EmbedBatchSize {
		end := start + s.cfg.EmbedBatchSize
		if end > len(cands) {
			end = len(cands)
		}
		batch := cands[start:end]

		// 1) Embed Qdrant query vectors for this batch.
		inputs := make([]string, 0, len(batch))
		for _, c := range batch {
			q := strings.TrimSpace(s.cfg.QdrantRagPromptPrefix)
			q = strings.TrimSpace(q + "\n\nWAS:\n" + c.wasText + "\n\nIS:\n" + c.isText)
			inputs = append(inputs, q)
		}

		vecs, err := ollamaClient.EmbedBatch(ctx, s.cfg.EmbedModel, inputs)
		if err != nil {
			continue
		}

		// 2) Qdrant batch search.
		hitSets, err := qClient.SearchBatch(ctx, s.cfg.QdrantCollection, vecs, topK, withPayloadKeys)
		if err != nil {
			continue
		}

		// 3) For each change: stream qdrant hits, call smart LLM, stream result.
		for i, c := range batch {
			_ = i
			hits := []qdrantHitSlim{}
			if i < len(hitSets) {
				for _, h := range hitSets[i] {
					hits = append(hits, qdrantHitSlim{Score: h.Score, Payload: h.Payload})
					if len(hits) >= maxPayloadContextHits {
						break
					}
				}
			}

			// SSE: stream hits (only lightweight part).
			for _, h := range hits {
				s.publishDiffEvent(diffID, "rag_diff_qdrant_hit", map[string]any{
					"diff_id": diffID,
					"left_chunk_index": c.leftI,
					"right_chunk_index": c.rightJ,
					"hit_score": h.Score,
					// payload is included but may be large; keep it as-is for now.
					"payload": h.Payload,
				})
			}

			ctxString := buildQdrantContextString(hits)
			prompt := s.buildRagDiffSmartPrompt(c.wasText, c.isText, ctxString, doc)
			smartOut, err := callOllamaGenerateJSON(ctx, s.cfg.OllamaBaseURL, s.cfg.AnalysisModel, prompt)
			if err != nil {
				// Still store qdrant hits; smart result stays nil.
				report.Changes = append(report.Changes, ragDiffChangeReport{
					LeftChunkIndex:  c.leftI,
					RightChunkIndex: c.rightJ,
					Similarity:       c.sim,
					LeftWeight:       c.weight,
					Was:              c.wasText,
					Is:               c.isText,
					QdrantHits:       hits,
				})
				continue
			}

			report.Changes = append(report.Changes, ragDiffChangeReport{
				LeftChunkIndex:  c.leftI,
				RightChunkIndex: c.rightJ,
				Similarity:       c.sim,
				LeftWeight:       c.weight,
				Was:              c.wasText,
				Is:               c.isText,
				Smart:            smartOut,
				QdrantHits:       hits,
			})

			s.publishDiffEvent(diffID, "rag_diff_result", map[string]any{
				"diff_id": diffID,
				"left_chunk_index": c.leftI,
				"right_chunk_index": c.rightJ,
				"smart": smartOut,
			})
		}
	}

	// 4) Persist rag-diff report to disk.
	// Storage: {data_path}/diff/{diff_id}
	outPath := filepath.Join(s.cfg.DataPath, "diff", diffID)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(outPath, b, 0644)
}

func buildQdrantContextString(hits []qdrantHitSlim) string {
	// Keep context compact: score + payload (already requested with withPayloadKeys).
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		payloadRaw := "{}"
		if h.Payload != nil {
			if b, err := json.Marshal(h.Payload); err == nil {
				payloadRaw = string(b)
			}
		}
		parts = append(parts, fmt.Sprintf("score=%.4f payload=%s", h.Score, payloadRaw))
	}
	return strings.Join(parts, "\n\n")
}

func (s *Server) buildRagDiffSmartPrompt(was, is, context string, doc *store.Document) string {
	tpl := s.cfg.RagDiffSmartPrompt
	was = strings.TrimSpace(was)
	is = strings.TrimSpace(is)
	context = strings.TrimSpace(context)
	if len(context) > 0 {
		if len(context) > s.cfg.MetadataWindowSize*2 {
			context = context[:s.cfg.MetadataWindowSize*2]
		}
	}

	// Optional enrichment from document metadata: helps grounding.
	docInfo := map[string]any{}
	if doc != nil {
		docInfo = map[string]any{
			"subtype":         doc.Subtype,
			"country":         doc.Country,
			"rag_tags":        doc.RagTags,
			"rag_categories":  doc.RagCategories,
			"rag_keywords":   doc.RagKeywords,
			"jurisdiction":   doc.Jurisdiction,
			"contract_type":  doc.ContractType,
		}
	}
	docInfoJSON, _ := json.Marshal(docInfo)

	out := tpl
	out = strings.ReplaceAll(out, "{{WAS}}", was)
	out = strings.ReplaceAll(out, "{{IS}}", is)
	out = strings.ReplaceAll(out, "{{CONTEXT}}", context)
	out = strings.ReplaceAll(out, "{{DOC_INFO}}", string(docInfoJSON))

	// Final truncation to keep request size bounded.
	maxChars := s.cfg.MetadataWindowSize * 2
	if maxChars < 4000 {
		maxChars = 8000
	}
	if len(out) > maxChars {
		out = out[:maxChars]
	}
	return out
}

func callOllamaGenerateJSON(ctx context.Context, baseURL, model, prompt string) (*ragDiffSmartResult, error) {
	type ollamaGenerateRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}
	type ollamaGenerateResponse struct {
		Response string `json:"response"`
	}

	reqBody, err := json.Marshal(ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return nil, fmt.Errorf("ollama generate request marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ollama generate new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	var raw ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ollama generate decode: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama generate http %d", resp.StatusCode)
	}

	parsed, err := parseRagDiffSmartJSON(raw.Response)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseRagDiffSmartJSON(s string) (*ragDiffSmartResult, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var out ragDiffSmartResult
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("parse smart llm json: %w", err)
	}

	out.Assessment = strings.TrimSpace(out.Assessment)
	switch out.Assessment {
	case "contradiction", "risk", "safe":
	default:
		out.Assessment = "safe"
	}

	return &out, nil
}

