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

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	embedder "github.com/nadevko/legist/internal/embed"
	"github.com/nadevko/legist/internal/parser"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/upload"
	"github.com/nadevko/legist/internal/webhook"
)

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

// runDiffPendingFiles runs the file pipeline for each pending file in order, then runDiffComputation.
// preamble runs once before the first file (e.g. document_* SSE for two-file upload).
func (s *Server) runDiffPendingFiles(diffID string, doc *store.Document, pch *processFileChannel, preamble func(), files []*store.File, threshold float64) {
	if preamble != nil {
		preamble()
	}
	for i, f := range files {
		s.processFileWithChannel(f, doc, pch)
		if !s.fileParseSucceeded(f.ID) {
			msg := "file processing failed"
			if len(files) == 2 && i == 0 {
				msg = "left file processing failed"
			}
			if len(files) == 2 && i == 1 {
				msg = "right file processing failed"
			}
			s.markDiffFailed(diffID, msg)
			return
		}
	}
	s.runDiffComputation(diffID, threshold)
}

func (s *Server) respondDiffCreated(c echo.Context, d *store.Diff) error {
	if c.Request().Header.Get("Accept") == "text/event-stream" {
		return sse.Stream(c, s.broker, d.ID, "diff_done", "diff_failed")
	}
	return c.JSON(http.StatusCreated, toDiffResponse(*d))
}

// runDiffComputation performs structural diff (placeholder) and marks the diff done or failed.
func (s *Server) runDiffComputation(diffID string, threshold float64) {
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

	// Clamp to sane range.
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}

	leftPath := filepath.Join(s.cfg.DataPath, "lessed", d.LeftFileID)
	rightPath := filepath.Join(s.cfg.DataPath, "lessed", d.RightFileID)

	embedCfg := embedder.Config{
		OllamaBaseURL:    s.cfg.OllamaBaseURL,
		Model:            s.cfg.EmbedModel,
		BatchSize:        s.cfg.EmbedBatchSize,
		ProgressInterval: time.Duration(s.cfg.EmbedProgressIntervalMS) * time.Millisecond,
		HTTPTimeout:      time.Duration(s.cfg.EmbedHTTPTimeoutMS) * time.Millisecond,
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

	NLeft := len(leftEmb)
	NRight := len(rightEmb)

	// Validate embeddings arrays are non-empty/consistent.
	if NLeft == 0 || NRight == 0 {
		sim := 0.0
		zeroLeft := make([]*int, NLeft)  // empty slices
		zeroRight := make([]*int, NRight) // empty slices
		payload := struct {
			Params struct {
				Threshold float64 `json:"threshold"`
			} `json:"params"`
			MatchedPairsCount int  `json:"matched_pairs_count"`
			LeftMatches        []*int `json:"left_matches"`
			RightMatches       []*int `json:"right_matches"`
			SimilarityMatrix   any   `json:"similarity_matrix"`
		}{}
		payload.Params.Threshold = threshold
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
		var sum float64
		for _, x := range leftEmb[i] {
			sum += x * x
		}
		n := math.Sqrt(sum)
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
		var sum float64
		for _, x := range rightEmb[j] {
			sum += x * x
		}
		n := math.Sqrt(sum)
		if n == 0 {
			fail("right embedding norm is zero", nil)
			return
		}
		normsR[j] = n
	}

	candidates := make([]edge, 0)
	interval := time.Duration(s.cfg.DiffMatchProgressIntervalMS) * time.Millisecond
	lastEmit := time.Time{}
	lastSentPct := -1

	// Compute similarities and build candidate edges.
	for i := 0; i < NLeft; i++ {
		li := leftEmb[i]
		ni := normsL[i]
		for j := 0; j < NRight; j++ {
			rj := rightEmb[j]
			dot := 0.0
			for k := 0; k < lDim; k++ {
				dot += li[k] * rj[k]
			}
			sim := dot / (ni * normsR[j])
			if sim >= threshold {
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
	matchedPairs := 0

	for _, e := range candidates {
		if leftUsed[e.i] || rightUsed[e.j] {
			continue
		}
		if e.sim < threshold {
			continue
		}
		leftUsed[e.i] = true
		rightUsed[e.j] = true
		jv := e.j
		iv := e.i
		leftMatches[e.i] = &jv
		rightMatches[e.j] = &iv
		matchedPairs++
	}

	denom := NLeft
	if NRight > denom {
		denom = NRight
	}
	simPercent := 0.0
	if denom > 0 {
		simPercent = float64(matchedPairs) * 100.0 / float64(denom)
	}

	type diffMatchingData struct {
		Params struct {
			Threshold float64 `json:"threshold"`
		} `json:"params"`
		MatchedPairsCount int    `json:"matched_pairs_count"`
		LeftMatches        []*int `json:"left_matches"`
		RightMatches       []*int `json:"right_matches"`
		SimilarityMatrix   any    `json:"similarity_matrix"`
	}

	var payload diffMatchingData
	payload.Params.Threshold = threshold
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
	// @Param       match_threshold formData number false "Greedy chunk matching threshold for diff (admin only; default from DIFF_MATCH_THRESHOLD)"
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

	threshold := s.cfg.DiffMatchThreshold
	if req.MatchThreshold != nil {
		if !auth.IsAdmin(c) {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				"only admins may set match_threshold", "match_threshold")
		}
		threshold = *req.MatchThreshold
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
		return s.createDiffFromTwoFiles(c, req, threshold)
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
		return s.createDiffFromFileAndID(c, req, leftID, rightID, threshold)
	}
	if leftID != "" && rightID != "" {
		return s.createDiffFromIDs(c, req, leftID, rightID, threshold)
	}
	return errorf(http.StatusBadRequest, "invalid_request",
		"send left_file_id+right_file_id, or one id + file, or file_left+file_right")
}

func (s *Server) createDiffFromIDs(c echo.Context, req upload.FormData, leftID, rightID string, threshold float64) error {
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
	if left.Status != "done" || right.Status != "done" {
		return errorf(http.StatusConflict, "not_ready",
			"both files must have status done before diffing")
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

	go s.runDiffComputation(d.ID, threshold)

	return s.respondDiffCreated(c, d)
}

func (s *Server) createDiffFromFileAndID(c echo.Context, req upload.FormData, leftID, rightID string, threshold float64) error {
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
	if existing.Status != "done" {
		return errorf(http.StatusConflict, "not_ready",
			"existing file must have status done")
	}
	if existing.DocumentID == nil {
		return errorf(http.StatusBadRequest, "invalid_request", "file has no document", "left_file_id")
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
	if newIsRight {
		leftFileID = existingID
		rightFileID = newF.ID
	} else {
		leftFileID = newF.ID
		rightFileID = existingID
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
	go func(newFile *store.File, doc *store.Document, diffID string) {
		s.runDiffPendingFiles(diffID, doc, pch, nil, []*store.File{newFile}, threshold)
	}(newF, doc, d.ID)

	return s.respondDiffCreated(c, d)
}

func (s *Server) createDiffFromTwoFiles(c echo.Context, req upload.FormData, threshold float64) error {
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
	go s.runDiffPendingFiles(diffID, doc, pch, preamble, []*store.File{fLeft, fRight}, threshold)

	return s.respondDiffCreated(c, d)
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
