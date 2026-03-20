package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"gonum.org/v1/gonum/floats"

	"github.com/nadevko/legist/internal/auth"
	embedder "github.com/nadevko/legist/internal/embed"
	"github.com/nadevko/legist/internal/parser"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/upload"
	"github.com/nadevko/legist/internal/webhook"
)

var requestValidator = validator.New()

func toDiffResponse(d store.Diff) diffResponse {
	return diffResponse{
		ID:                d.ID,
		Object:            "diff",
		DocumentID:        d.DocumentID,
		LeftFileID:        d.LeftFileID,
		RightFileID:       d.RightFileID,
		Status:            d.Status,
		SimilarityPercent: d.SimilarityPercent,
		Created:           toUnix(d.CreatedAt),
	}
}

// resolveDiff loads the diff and enforces ownership (404 if missing or other user).
func (s *Server) resolveDiff(c echo.Context, id string) (*store.Diff, error) {
	d, err := s.diffs.GetByID(id)
	if store.IsNotFound(err) {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such diff: "+id)
	}
	if err != nil {
		return nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if d.UserID == nil || *d.UserID != auth.UserID(c) {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such diff: "+id)
	}
	return d, nil
}

func (s *Server) publishDiffEvent(diffID, typ string, data any) {
	s.broker.Publish(diffID, sse.Event{Type: typ, Data: data})
}

func diffProcessingChannel(diffID string) *processFileChannel {
	return &processFileChannel{
		Key:         diffID,
		DoneEvent:   "file_done",
		FailedEvent: "file_failed",
	}
}

func (s *Server) markDiffFailed(diffID, errMsg string) {
	_ = s.diffs.UpdateStatus(diffID, "failed")
	s.publishDiffEvent(diffID, "diff_failed", map[string]any{"diff_id": diffID, "error": errMsg})
	s.dispatcher.Dispatch(webhook.EventDiffFailed, map[string]any{"id": diffID, "object": "diff"})
}

func (s *Server) fileParseSucceeded(fileID string) bool {
	f, err := s.files.GetByID(fileID)
	return err == nil && f.Status == "done"
}

func (s *Server) ensureFileReadyForDiff(f *store.File, doc *store.Document, pch *processFileChannel) error {
	// Poll file status; if it is pending, claim it and run parsing synchronously
	// so diff SSE can stream progress (when pch points to diffID).
	const pollInterval = 300 * time.Millisecond

	publishFileDone := func() {
		if pch == nil || pch.Key == "" {
			return
		}
		doneEvent := pch.DoneEvent
		if doneEvent == "" {
			doneEvent = "file_done"
		}
		s.broker.Publish(pch.Key, sse.Event{
			Type: doneEvent,
			Data: map[string]any{"file_id": f.ID, "status": "done"},
		})
	}

	publishFileFailed := func(errMsg string) {
		if pch == nil || pch.Key == "" {
			return
		}
		failedEvent := pch.FailedEvent
		if failedEvent == "" {
			failedEvent = "file_failed"
		}
		s.broker.Publish(pch.Key, sse.Event{
			Type: failedEvent,
			Data: map[string]any{"file_id": f.ID, "error": errMsg},
		})
	}

	ranParse := false

	for {
		cur, err := s.files.GetByID(f.ID)
		if err != nil {
			return fmt.Errorf("load file for diff: %w", err)
		}

		if cur.DocumentID == nil || *cur.DocumentID != doc.ID {
			return fmt.Errorf("file %s belongs to a different document", cur.ID)
		}

		switch cur.Status {
		case "done":
			if !ranParse {
				publishFileDone()
			}
			return nil
		case "failed":
			if !ranParse {
				publishFileFailed("file parsing failed")
			}
			return fmt.Errorf("file %s parsing failed", cur.ID)
		case "processing":
			time.Sleep(pollInterval)
			continue
		case "pending":
			claimed, err := s.files.UpdateStatusIf(cur.ID, "pending", "processing")
			if err != nil {
				return err
			}
			if !claimed {
				// Another worker claimed it; re-check state.
				time.Sleep(pollInterval)
				continue
			}
			// Run parse+embed in the current goroutine so matching starts only after readiness.
			ranParse = true
			s.processFileWithChannel(cur, doc, pch)
			// Loop will re-check DB status (done/failed).
			continue
		default:
			// Treat unknown statuses conservatively.
			time.Sleep(pollInterval)
		}
	}
}

// runDiffPendingFiles runs the file pipeline for each pending file in order, then runDiffComputation.
// preamble runs once before the first file (e.g. document_* SSE for two-file upload).
func (s *Server) runDiffPendingFiles(diffID string, doc *store.Document, pch *processFileChannel, preamble func(), files []*store.File, lowThreshold float64, highThreshold float64) {
	if preamble != nil {
		preamble()
	}
	for i, f := range files {
		if err := s.ensureFileReadyForDiff(f, doc, pch); err != nil {
			msg := "file processing failed"
			if len(files) == 2 && i == 0 {
				msg = "left file processing failed"
			}
			if len(files) == 2 && i == 1 {
				msg = "right file processing failed"
			}
			_ = err // keep original message in SSE payload below
			s.markDiffFailed(diffID, msg+": "+err.Error())
			return
		}
	}
	s.runDiffComputation(diffID, lowThreshold, highThreshold)
}

func (s *Server) respondDiffCreated(c echo.Context, d *store.Diff) error {
	if c.Request().Header.Get("Accept") == "text/event-stream" {
		return sse.Stream(c, s.broker, d.ID, "diff_done", "diff_failed")
	}
	return c.JSON(http.StatusCreated, toDiffResponse(*d))
}

func (s *Server) respondDiffCreatedWithStart(c echo.Context, d *store.Diff, startFn func()) error {
	if c.Request().Header.Get("Accept") == "text/event-stream" {
		// Subscribe first to avoid losing early events (diff_started/file_done/...).
		return sse.StreamWithInitialEvent(
			c,
			s.broker,
			d.ID,
			"",
			nil,
			startFn,
			"diff_done",
			"diff_failed",
		)
	}
	startFn()
	return c.JSON(http.StatusCreated, toDiffResponse(*d))
}

// runDiffComputation performs structural diff (placeholder) and marks the diff done or failed.
func (s *Server) runDiffComputation(diffID string, lowThreshold float64, highThreshold float64) {
	if err := s.diffs.UpdateStatus(diffID, "processing"); err != nil {
		return
	}
	s.publishDiffEvent(diffID, "diff_started", map[string]any{"diff_id": diffID})

	ctx := context.Background()

	publishProgress := func(p parser.ParseProgress) {
		// Keep the same shape as file parsing progress events:
		// {file_id: <null>, progress: <ParseProgress>}
		s.broker.Publish(diffID, sse.Event{
			Type: "progress",
			Data: map[string]any{"file_id": nil, "progress": p},
		})
	}

	fail := func(errMsg string, err error) {
		_ = errMsg
		_ = err
		_ = s.diffs.UpdateStatus(diffID, "failed")
		s.publishDiffEvent(diffID, "diff_failed", map[string]any{"diff_id": diffID, "error": errMsg})
		s.dispatcher.Dispatch(webhook.EventDiffFailed, map[string]any{"id": diffID, "object": "diff", "error": errMsg})
	}

	d, err := s.diffs.GetByID(diffID)
	if err != nil {
		fail("failed to load diff", err)
		return
	}

	doc, _ := s.documents.GetByID(d.DocumentID)

	// Clamp to sane range.
	if lowThreshold < 0 {
		lowThreshold = 0
	}
	if lowThreshold > 1 {
		lowThreshold = 1
	}
	if highThreshold < 0 {
		highThreshold = 0
	}
	if highThreshold > 1 {
		highThreshold = 1
	}
	// Ensure low <= high, otherwise swap to preserve semantics.
	if lowThreshold > highThreshold {
		lowThreshold, highThreshold = highThreshold, lowThreshold
	}

	leftPath := filepath.Join(s.cfg.DataPath, "lessed", d.LeftFileID)
	rightPath := filepath.Join(s.cfg.DataPath, "lessed", d.RightFileID)

	weightRules, err := s.regexRules.ListWeightRules()
	if err != nil {
		fail("failed to load weight regex rules", err)
		return
	}
	omitRules, err := s.regexRules.ListOmitRules()
	if err != nil {
		fail("failed to load omit regex rules", err)
		return
	}

	embedCfg := embedder.Config{
		OllamaBaseURL:            s.cfg.OllamaBaseURL,
		Model:                    s.cfg.EmbedModel,
		BatchSize:                s.cfg.EmbedBatchSize,
		ShortChunkPrefixMaxChars: s.cfg.EmbedShortChunkPrefixMaxChars,
		UseWeightPrefix:          s.cfg.EmbedUseWeightPrefix,
		WeightRules:             weightRules,
		OmitRules:               omitRules,
		WeightCritical:           s.cfg.WeightCritical,
		WeightMain:               s.cfg.WeightMain,
		WeightStandard:           s.cfg.WeightStandard,
		WeightTechnical:          s.cfg.WeightTechnical,
		WeightMaxCap:             s.cfg.WeightMaxCap,
		ProgressInterval:         time.Duration(s.cfg.EmbedProgressIntervalMS) * time.Millisecond,
		HTTPTimeout:              time.Duration(s.cfg.EmbedHTTPTimeoutMS) * time.Millisecond,
	}

	// Ensure embeddings are present (skip when already up-to-date).
	if err := embedder.LegistEmbedIfNeeded(ctx, leftPath, embedCfg, publishProgress); err != nil {
		fail("left embedding failed", err)
		return
	}
	if err := embedder.LegistEmbedIfNeeded(ctx, rightPath, embedCfg, publishProgress); err != nil {
		fail("right embedding failed", err)
		return
	}

	leftRaw, err := os.ReadFile(leftPath)
	if err != nil {
		fail("failed to read left legist JSON", err)
		return
	}
	rightRaw, err := os.ReadFile(rightPath)
	if err != nil {
		fail("failed to read right legist JSON", err)
		return
	}

	var leftPF, rightPF parser.ParsedFile
	if err := json.Unmarshal(leftRaw, &leftPF); err != nil {
		fail("failed to parse left legist JSON", err)
		return
	}
	if err := json.Unmarshal(rightRaw, &rightPF); err != nil {
		fail("failed to parse right legist JSON", err)
		return
	}

	leftEmb := leftPF.ChunkEmbeddings
	rightEmb := rightPF.ChunkEmbeddings
	leftContents := leftPF.ChunkContent
	rightContents := rightPF.ChunkContent
	leftWeights := parser.FlattenChunkWeights(leftPF.Sections)
	rightWeights := parser.FlattenChunkWeights(rightPF.Sections)

	NLeft := len(leftEmb)
	NRight := len(rightEmb)
	if len(leftContents) != NLeft || len(rightContents) != NRight {
		fail("chunk content/embedding length mismatch", fmt.Errorf("left=%d/%d right=%d/%d", len(leftContents), NLeft, len(rightContents), NRight))
		return
	}
	if len(leftWeights) != NLeft || len(rightWeights) != NRight {
		fail("chunk weight/embedding length mismatch", fmt.Errorf("left=%d/%d right=%d/%d", len(leftWeights), NLeft, len(rightWeights), NRight))
		return
	}

	// Validate embeddings arrays are non-empty/consistent.
	if NLeft == 0 || NRight == 0 {
		sim := 0.0
		zeroLeft := make([]*int, NLeft)  // empty slices
		zeroRight := make([]*int, NRight) // empty slices
		payload := struct {
			Params struct {
				LowThreshold  float64 `json:"low_threshold"`
				HighThreshold float64 `json:"high_threshold"`
			} `json:"params"`
			MatchedPairsCount int  `json:"matched_pairs_count"`
			LeftMatches        []*int `json:"left_matches"`
			RightMatches       []*int `json:"right_matches"`
			SimilarityMatrix   any   `json:"similarity_matrix"`
		}{}
		payload.Params.LowThreshold = lowThreshold
		payload.Params.HighThreshold = highThreshold
		payload.LeftMatches = zeroLeft
		payload.RightMatches = zeroRight
		payload.SimilarityMatrix = nil

		data, _ := json.Marshal(payload)
		diffData := store.DiffData(data)
		if err := s.diffs.UpdateResult(diffID, &sim, diffData, "done"); err != nil {
			fail("failed to persist diff result", err)
			return
		}
		d2, err := s.diffs.GetByID(diffID)
		if err != nil {
			return
		}
		s.dispatcher.Dispatch(webhook.EventDiffDone, toDiffResponse(*d2))
		s.publishDiffEvent(diffID, "diff_done", map[string]any{
			"diff_id":              diffID,
			"status":               "done",
			"similarity_percent":  sim,
			"left_matches":        zeroLeft,
			"right_matches":       zeroRight,
		})
		return
	}

	// Cosine similarity helpers.
	type edge struct {
		i   int
		j   int
		sim float64
	}

	lDim := len(leftEmb[0])
	rDim := len(rightEmb[0])
	if lDim == 0 || rDim == 0 || lDim != rDim {
		fail("invalid embeddings dimension", fmt.Errorf("left_dim=%d right_dim=%d", lDim, rDim))
		return
	}

	normsL := make([]float64, NLeft)
	for i := 0; i < NLeft; i++ {
		if len(leftEmb[i]) != lDim {
			fail("left embeddings dimension mismatch", fmt.Errorf("idx=%d expected=%d got=%d", i, lDim, len(leftEmb[i])))
			return
		}
		n := floats.Norm(leftEmb[i], 2)
		if n == 0 {
			fail("left embedding norm is zero", nil)
			return
		}
		normsL[i] = n
	}
	normsR := make([]float64, NRight)
	for j := 0; j < NRight; j++ {
		if len(rightEmb[j]) != rDim {
			fail("right embeddings dimension mismatch", fmt.Errorf("idx=%d expected=%d got=%d", j, rDim, len(rightEmb[j])))
			return
		}
		n := floats.Norm(rightEmb[j], 2)
		if n == 0 {
			fail("right embedding norm is zero", nil)
			return
		}
		normsR[j] = n
	}

	// `regex/omits` is pre-processing before embedding/matching:
	// - a chunk is "skipped" when its cleaned text becomes empty
	// - skipped chunks do not participate in matching and do not affect similarity_percent denominator
	omitPatterns := make([]string, 0, len(omitRules))
	for _, r := range omitRules {
		if !r.Enabled {
			continue
		}
		omitPatterns = append(omitPatterns, r.Regex)
	}
	compiledOmitRules, err := parser.CompileOmitRules(omitPatterns)
	if err != nil {
		fail("invalid omit regex in DB", err)
		return
	}

	skippedLeft := make([]bool, NLeft)
	for i := 0; i < NLeft; i++ {
		skippedLeft[i] = parser.CleanText(leftContents[i], compiledOmitRules) == ""
	}
	skippedRight := make([]bool, NRight)
	for j := 0; j < NRight; j++ {
		skippedRight[j] = parser.CleanText(rightContents[j], compiledOmitRules) == ""
	}

	candidates := make([]edge, 0)
	interval := time.Duration(s.cfg.DiffMatchProgressIntervalMS) * time.Millisecond
	lastEmit := time.Time{}
	lastSentPct := -1

	// Compute similarities and build candidate edges.
	for i := 0; i < NLeft; i++ {
		if skippedLeft[i] {
			continue
		}
		li := leftEmb[i]
		ni := normsL[i]
		for j := 0; j < NRight; j++ {
			if skippedRight[j] {
				continue
			}
			rj := rightEmb[j]
			dot := floats.Dot(li, rj)
			sim := dot / (ni * normsR[j])
			if sim >= lowThreshold {
				candidates = append(candidates, edge{i: i, j: j, sim: sim})
			}
		}

		// SSE progress for "matching": based on outer loop i.
		pct := ((i + 1) * 100) / NLeft
		now := time.Now()
		if pct != lastSentPct && (lastEmit.IsZero() || now.Sub(lastEmit) >= interval) {
			lastEmit = now
			lastSentPct = pct
			publishProgress(parser.ParseProgress{
				Stage:            parser.StageMatching,
				Message:          fmt.Sprintf("matching %d / %d chunks", i+1, NLeft),
				EmbeddingPercent: pct,
				ChunksEmbedded:   i + 1,
				ChunksTotal:      NLeft,
			})
		}
	}

	// Greedy one-to-one matching.
	sort.Slice(candidates, func(a, b int) bool { return candidates[a].sim > candidates[b].sim })
	leftUsed := make([]bool, NLeft)
	rightUsed := make([]bool, NRight)
	leftMatches := make([]*int, NLeft)
	rightMatches := make([]*int, NRight)
	leftMatchSims := make([]float64, NLeft)
	matchedPairs := 0

	for _, e := range candidates {
		if leftUsed[e.i] || rightUsed[e.j] {
			continue
		}
		if e.sim < lowThreshold {
			continue
		}
		leftUsed[e.i] = true
		rightUsed[e.j] = true
		jv := e.j
		iv := e.i
		leftMatches[e.i] = &jv
		rightMatches[e.j] = &iv
		leftMatchSims[e.i] = e.sim
		matchedPairs++
	}

	// Omit pre-processing happens before embedding; skipped chunks do not participate in matching.
	// So there is no separate risk-zone filtering step here.
	matchedPairs = 0
	for i := 0; i < NLeft; i++ {
		if leftMatches[i] != nil {
			matchedPairs++
		}
	}

	sumWeightsLeft := 0.0
	for i, w := range leftWeights {
		if skippedLeft[i] {
			continue
		}
		if w <= 0 {
			w = 1.0
		}
		sumWeightsLeft += w
	}
	sumWeightsRight := 0.0
	for j, w := range rightWeights {
		if skippedRight[j] {
			continue
		}
		if w <= 0 {
			w = 1.0
		}
		sumWeightsRight += w
	}
	denom := math.Max(sumWeightsLeft, sumWeightsRight)
	weightedNumerator := 0.0
	for i := 0; i < NLeft; i++ {
		if leftMatches[i] == nil || skippedLeft[i] {
			continue
		}
		w := leftWeights[i]
		if w <= 0 {
			w = 1.0
		}
		weightedNumerator += leftMatchSims[i] * w
	}
	simPercent := 0.0
	if denom > 0 {
		simPercent = (weightedNumerator / denom) * 100.0
	}

	// rag-diff stage (best-effort): build a JSON report for on-demand DOCX rendering.
	s.runRagDiffStage(ctx, diffID, d, doc, leftPF, rightPF,
		leftContents, rightContents,
		leftWeights, rightWeights,
		leftMatches, leftMatchSims,
	)

	type diffMatchingData struct {
		Params struct {
			LowThreshold  float64 `json:"low_threshold"`
			HighThreshold float64 `json:"high_threshold"`
		} `json:"params"`
		MatchedPairsCount int    `json:"matched_pairs_count"`
		LeftMatches        []*int `json:"left_matches"`
		RightMatches       []*int `json:"right_matches"`
		SimilarityMatrix   any    `json:"similarity_matrix"`
	}

	var payload diffMatchingData
	payload.Params.LowThreshold = lowThreshold
	payload.Params.HighThreshold = highThreshold
	payload.MatchedPairsCount = matchedPairs
	payload.LeftMatches = leftMatches
	payload.RightMatches = rightMatches
	payload.SimilarityMatrix = nil

	data, err := json.Marshal(payload)
	if err != nil {
		fail("failed to marshal diff_data", err)
		return
	}
	diffData := store.DiffData(data)
	simPtr := simPercent
	if err := s.diffs.UpdateResult(diffID, &simPtr, diffData, "done"); err != nil {
		fail("failed to persist diff result", err)
		return
	}

	d2, err := s.diffs.GetByID(diffID)
	if err != nil {
		return
	}

	s.dispatcher.Dispatch(webhook.EventDiffDone, toDiffResponse(*d2))
	s.publishDiffEvent(diffID, "diff_done", map[string]any{
		"diff_id":             diffID,
		"status":              "done",
		"similarity_percent": simPercent,
		"left_matches":       leftMatches,
		"right_matches":      rightMatches,
	})
}

// handleCreateDiff godoc
// @Summary     Create a diff (multipart: two file IDs, ID + file, or two files)
// @Tags        Diffs
// @Security    BearerAuth
// @Accept      multipart/form-data
// @Produce     json
// @Param       left_file_id    formData string false "With right_file_id: compare two existing files"
// @Param       right_file_id   formData string false "With left_file_id: compare two existing files"
// @Param       file            formData file   false "With left_file_id or right_file_id: new version to compare"
// @Param       file_left       formData file   false "With file_right: create document and compare"
// @Param       file_right      formData file   false "With file_left: create document and compare"
// @Param       subtype         formData string false "Work metadata when creating document (two files)"
// @Param       number          formData string false "Work metadata"
// @Param       author          formData string false "Work metadata"
// @Param       date            formData string false "Work metadata"
// @Param       country         formData string false "Work metadata"
// @Param       name            formData string false "Work metadata"
	// @Param       match_threshold_low formData number false "Hard threshold for diff chunk matching (admin only; default from DIFF_MATCH_THRESHOLD_LOW)"
	// @Param       match_threshold_high formData number false "Soft threshold for diff chunk matching (admin only; default from DIFF_MATCH_THRESHOLD_HIGH)"
// @Param       Accept          header   string false "application/json | text/event-stream"
// @Param       Idempotency-Key header   string true  "Idempotency key"
// @Success     201 {object} diffResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /diffs [post]
func (s *Server) handleCreateDiff(c echo.Context) error {
	mf, err := upload.OpenMultipart(c)
	if err != nil {
		return badUpload(err)
	}
	req, err := upload.BindFormData(c)
	if err != nil {
		return badUpload(err)
	}

	lowThreshold := s.cfg.DiffMatchLowThreshold
	highThreshold := s.cfg.DiffMatchHighThreshold
	if req.MatchThresholdLow != nil || req.MatchThresholdHigh != nil {
		if !auth.IsAdmin(c) {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				"only admins may set match_threshold_low/match_threshold_high", "")
		}
		if req.MatchThresholdLow != nil {
			lowThreshold = *req.MatchThresholdLow
		}
		if req.MatchThresholdHigh != nil {
			highThreshold = *req.MatchThresholdHigh
		}
	}
	if err := requestValidator.Var(lowThreshold, "gte=0,lte=1"); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "match_threshold_low must be within [0,1]", "match_threshold_low")
	}
	if err := requestValidator.Var(highThreshold, "gte=0,lte=1"); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "match_threshold_high must be within [0,1]", "match_threshold_high")
	}

	hasFL := upload.HasFile(mf, "file_left")
	hasFR := upload.HasFile(mf, "file_right")
	if hasFL != hasFR {
		return errorf(http.StatusBadRequest, "invalid_request",
			"both file_left and file_right are required together", "file_left")
	}

	hasFile := upload.HasFile(mf, "file")

	leftID := strings.TrimSpace(c.FormValue("left_file_id"))
	rightID := strings.TrimSpace(c.FormValue("right_file_id"))

	if hasFL && hasFR {
		return s.createDiffFromTwoFiles(c, req, lowThreshold, highThreshold)
	}
	if hasFile {
		if leftID != "" && rightID != "" {
			return errorf(http.StatusBadRequest, "invalid_request",
				"provide only one of left_file_id or right_file_id with file", "left_file_id")
		}
		if leftID == "" && rightID == "" {
			return errorf(http.StatusBadRequest, "parameter_missing",
				"left_file_id or right_file_id is required with file", "left_file_id")
		}
		return s.createDiffFromFileAndID(c, req, leftID, rightID, lowThreshold, highThreshold)
	}
	if leftID != "" && rightID != "" {
		return s.createDiffFromIDs(c, req, leftID, rightID, lowThreshold, highThreshold)
	}
	return errorf(http.StatusBadRequest, "invalid_request",
		"send left_file_id+right_file_id, or one id + file, or file_left+file_right")
}

func (s *Server) createDiffFromIDs(c echo.Context, req upload.FormData, leftID, rightID string, lowThreshold float64, highThreshold float64) error {
	if leftID == rightID {
		return errorf(http.StatusBadRequest, "invalid_request",
			"left_file_id and right_file_id must differ", "right_file_id")
	}

	left, err := s.files.GetByID(leftID)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such file: "+leftID)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	right, err := s.files.GetByID(rightID)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such file: "+rightID)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = s.ensureFileReadable(c, left); err != nil {
		return err
	}
	if err = s.ensureFileReadable(c, right); err != nil {
		return err
	}
	if left.DocumentID == nil || right.DocumentID == nil || *left.DocumentID != *right.DocumentID {
		return errorf(http.StatusBadRequest, "invalid_request",
			"both files must belong to the same document", "right_file_id")
	}

	// Fail fast if either side already failed.
	if left.Status == "failed" || right.Status == "failed" {
		return errorf(http.StatusConflict, "not_ready",
			"one of the files already failed; cannot diff")
	}

	doc, err := s.documents.GetByID(*left.DocumentID)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "document not found")
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	uid := auth.UserID(c)
	userPtr := &uid
	d := &store.Diff{
		ID:          newID("diff"),
		UserID:      userPtr,
		DocumentID:  *left.DocumentID,
		LeftFileID:  leftID,
		RightFileID: rightID,
		Status:      "pending",
	}
	if err = s.diffs.Create(d); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	s.dispatcher.Dispatch(webhook.EventDiffCreated, toDiffResponse(*d))

	pch := diffProcessingChannel(d.ID)
	startFn := func() {
		s.runDiffPendingFiles(d.ID, doc, pch, nil, []*store.File{left, right}, lowThreshold, highThreshold)
	}

	return s.respondDiffCreatedWithStart(c, d, startFn)
}

func (s *Server) createDiffFromFileAndID(c echo.Context, req upload.FormData, leftID, rightID string, lowThreshold float64, highThreshold float64) error {
	var existingID string
	var newIsRight bool
	switch {
	case leftID != "" && rightID == "":
		existingID = leftID
		newIsRight = true
	case leftID == "" && rightID != "":
		existingID = rightID
		newIsRight = false
	default:
		return errorf(http.StatusBadRequest, "invalid_request",
			"provide exactly one of left_file_id or right_file_id", "left_file_id")
	}

	existing, err := s.files.GetByID(existingID)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such file: "+existingID)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = s.ensureFileReadable(c, existing); err != nil {
		return err
	}
	if existing.DocumentID == nil {
		return errorf(http.StatusBadRequest, "invalid_request", "file has no document", "left_file_id")
	}
	if existing.Status == "failed" {
		return errorf(http.StatusConflict, "not_ready",
			"existing file parsing failed; cannot diff")
	}

	doc, err := s.documents.GetByID(*existing.DocumentID)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if doc.OwnerID() != "" && doc.OwnerID() != auth.UserID(c) {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+existingID)
	}

	docID := doc.ID
	newF, _, err := s.saveFormFile(c, "file", &docID, req)
	if err != nil {
		return err
	}

	var leftFileID, rightFileID string
	var leftFile, rightFile *store.File
	if newIsRight {
		leftFileID = existingID
		rightFileID = newF.ID
		leftFile = existing
		rightFile = newF
	} else {
		leftFileID = newF.ID
		rightFileID = existingID
		leftFile = newF
		rightFile = existing
	}

	uid := auth.UserID(c)
	userPtr := &uid
	d := &store.Diff{
		ID:          newID("diff"),
		UserID:      userPtr,
		DocumentID:  docID,
		LeftFileID:  leftFileID,
		RightFileID: rightFileID,
		Status:      "pending",
	}
	if err = s.diffs.Create(d); err != nil {
		_ = newF // file on disk
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	s.dispatcher.Dispatch(webhook.EventFileCreated, toFileResponse(*newF))
	s.dispatcher.Dispatch(webhook.EventDiffCreated, toDiffResponse(*d))

	pch := diffProcessingChannel(d.ID)
	startFn := func() {
		s.runDiffPendingFiles(d.ID, doc, pch, nil, []*store.File{leftFile, rightFile}, lowThreshold, highThreshold)
	}

	return s.respondDiffCreatedWithStart(c, d, startFn)
}

func (s *Server) createDiffFromTwoFiles(c echo.Context, req upload.FormData, lowThreshold float64, highThreshold float64) error {
	uid := auth.UserID(c)

	fLeft, doc, err := s.saveFormFile(c, "file_left", nil, req)
	if err != nil {
		return err
	}
	docID := doc.ID
	fRight, doc2, err := s.saveFormFile(c, "file_right", &docID, req)
	if err != nil {
		return err
	}
	if doc2.ID != doc.ID {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	diffID := newID("diff")
	userPtr := &uid
	d := &store.Diff{
		ID:          diffID,
		UserID:      userPtr,
		DocumentID:  doc.ID,
		LeftFileID:  fLeft.ID,
		RightFileID: fRight.ID,
		Status:      "pending",
	}
	if err = s.diffs.Create(d); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	s.dispatcher.Dispatch(webhook.EventFileCreated, toFileResponse(*fLeft))
	s.dispatcher.Dispatch(webhook.EventFileCreated, toFileResponse(*fRight))
	s.dispatcher.Dispatch(webhook.EventDiffCreated, toDiffResponse(*d))

	pch := diffProcessingChannel(diffID)
	preamble := func() {
		s.publishDiffEvent(diffID, "document_will_be_created", map[string]any{"diff_id": diffID})
		s.publishDiffEvent(diffID, "document_created", map[string]any{"diff_id": diffID, "document_id": doc.ID})
	}
	startFn := func() {
		s.runDiffPendingFiles(diffID, doc, pch, preamble, []*store.File{fLeft, fRight}, lowThreshold, highThreshold)
	}

	return s.respondDiffCreatedWithStart(c, d, startFn)
}

// ensureFileReadable returns 404 if the file is not readable by the current user.
// Public files (user_id NULL) are readable by any authenticated user.
func (s *Server) ensureFileReadable(c echo.Context, f *store.File) error {
	uid := auth.UserID(c)
	if f.UserID != nil && *f.UserID != uid {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+f.ID)
	}
	return nil
}

// handleListDiffs godoc
// @Summary     List diffs for the current user
// @Tags        Diffs
// @Security    BearerAuth
// @Produce     json
// @Param       document_id    query  string   false "Filter by document"
// @Param       file_id        query  string   false "Filter by left or right file"
// @Param       status         query  string   false "pending|processing|done|failed"
// @Param       limit          query  int      false "Limit"
// @Param       starting_after query  string   false "Cursor"
// @Param       ending_before  query  string   false "Cursor"
// @Param       expand[]       query  []string false "Expand: document, left_file, right_file"
// @Success     200 {object} listResponse[diffResponse]
// @Failure     401 {object} apiErrorResponse
// @Router      /diffs [get]
func (s *Server) handleListDiffs(c echo.Context) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	uid := auth.UserID(c)
	filter := store.DiffFilter{
		UserID: &uid,
		Status: c.QueryParam("status"),
	}
	if docID := c.QueryParam("document_id"); docID != "" {
		filter.DocumentID = &docID
	}
	if fid := c.QueryParam("file_id"); fid != "" {
		filter.FileID = fid
	}
	items, err := s.diffs.List(filter, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(items, p.Limit,
		toDiffResponse, func(d store.Diff) string { return d.ID }))
}

// handleGetDiff godoc
// @Summary     Get diff by ID or stream pipeline (SSE)
// @Tags        Diffs
// @Security    BearerAuth
// @Param       id       path   string true  "Diff ID"
// @Param       expand[] query  []string false "Expand: document, left_file, right_file"
// @Param       Accept   header string   false "application/json | text/event-stream"
// @Produce     json
// @Success     200 {object} diffResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /diffs/{id} [get]
func (s *Server) handleGetDiff(c echo.Context) error {
	id := c.Param("id")
	d, err := s.resolveDiff(c, id)
	if err != nil {
		return err
	}
	if c.Request().Header.Get("Accept") == "text/event-stream" {
		return sse.Stream(c, s.broker, d.ID, "diff_done", "diff_failed")
	}
	return c.JSON(http.StatusOK, toDiffResponse(*d))
}
