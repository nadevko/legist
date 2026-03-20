package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fumiama/go-docx"
	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/store"
)

// handleGetReport godoc
// @Summary     Get optional DOCX report for an annotated diff
// @Tags        Reports
// @Security    BearerAuth
// @Produce     application/vnd...docx
// @Param       diff_id  path   string  true  "Diff ID"
// @Param       lazy     query  boolean false "If true (default) and report missing -> 404; if false -> best-effort generate from rag-diff JSON"
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /reports/:diff_id [get]
func (s *Server) handleGetReport(c echo.Context) error {
	diffID := strings.TrimSpace(c.Param("diff_id"))
	if diffID == "" {
		return errorf(http.StatusBadRequest, "invalid_request", "diff_id is required", "diff_id")
	}

	lazyStr := strings.TrimSpace(c.QueryParam("lazy"))
	lazy := true
	if lazyStr != "" {
		switch strings.ToLower(lazyStr) {
		case "0", "false", "no":
			lazy = false
		case "1", "true", "yes":
			lazy = true
		default:
			return errorf(http.StatusBadRequest, "invalid_request", "lazy must be boolean", "lazy")
		}
	}

	outPath := filepath.Join(s.cfg.DataPath, "reports", diffID)
	if _, err := os.Stat(outPath); err == nil {
		return c.File(outPath)
	}

	// If report is requested explicitly, but rag-diff artefacts are not present:
	// - lazy=true: error
	// - lazy=false: best-effort generation from existing rag-diff JSON
	if lazy {
		return errorf(http.StatusNotFound, "resource_missing", "report not found", diffID)
	}

	ragDiffPath := filepath.Join(s.cfg.DataPath, "diff", diffID)
	if _, err := os.Stat(ragDiffPath); err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "rag-diff artefacts not found", diffID)
	}

	if err := s.generateDocxReport(diffID, ragDiffPath, outPath); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "failed to generate report: "+err.Error())
	}

	return c.File(outPath)
}

func (s *Server) generateDocxReport(diffID, ragDiffPath, outPath string) error {
	raw, err := os.ReadFile(ragDiffPath)
	if err != nil {
		return fmt.Errorf("read rag-diff report: %w", err)
	}

	var report ragDiffReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("unmarshal rag-diff report: %w", err)
	}

	// Load metadata for nicer document header (best-effort).
	var d *store.Diff
	var doc *store.Document
	var leftF, rightF *store.File
	d, _ = s.diffs.GetByID(diffID)
	if d != nil {
		doc, _ = s.documents.GetByID(d.DocumentID)
		leftF, _ = s.files.GetByID(d.LeftFileID)
		rightF, _ = s.files.GetByID(d.RightFileID)
	}

	w := docx.New().WithDefaultTheme().WithA4Page()

	// Title
	w.AddParagraph().Style("Heading 1").AddText("Legist Diff Report")

	// Document header
	if doc != nil {
		p := w.AddParagraph()
		p.AddText(strings.TrimSpace(doc.Subtype))
		p.AddText(" — ")
		p.AddText(strings.TrimSpace(doc.Number))
		p.AddText("\n")
		p.AddText("Country: " + strings.TrimSpace(doc.Country))
	}

	if doc != nil && doc.Name != "" {
		w.AddParagraph().AddText("Name: " + doc.Name)
	}

	// Versions (best-effort)
	if leftF != nil || rightF != nil {
		w.AddParagraph().AddText("Files:")
		if leftF != nil {
			w.AddParagraph().AddText(fmt.Sprintf("  Left: %s (language=%s)", leftF.MimeType, strPtr(leftF.Language)))
		}
		if rightF != nil {
			w.AddParagraph().AddText(fmt.Sprintf("  Right: %s (language=%s)", rightF.MimeType, strPtr(rightF.Language)))
		}
	}

	// Changes
	if len(report.Changes) == 0 {
		w.AddParagraph().AddText("No change candidates for RAG analysis.")
	}
	for i, ch := range report.Changes {
		w.AddParagraph().Style("Heading 2").AddText(fmt.Sprintf("Change %d", i+1))
		w.AddParagraph().AddText("Similarity: " + fmt.Sprintf("%.4f", ch.Similarity) + " | Weight: " + fmt.Sprintf("%.2f", ch.LeftWeight))
		w.AddParagraph().AddText("Was:\n" + truncateForDocx(ch.Was))
		w.AddParagraph().AddText("Is:\n" + truncateForDocx(ch.Is))

		if ch.Smart != nil {
			w.AddParagraph().AddText("Assessment: " + ch.Smart.Assessment)
			if strings.TrimSpace(ch.Smart.Message) != "" {
				w.AddParagraph().AddText("Message:\n" + truncateForDocx(ch.Smart.Message))
			}

			if len(ch.Smart.Links) > 0 {
				w.AddParagraph().Style("Heading 3").AddText("Links")
				for li, lk := range ch.Smart.Links {
					_ = li
					w.AddParagraph().AddText(linkToLine(lk))
				}
			}
		} else {
			w.AddParagraph().AddText("Smart analysis: not available")
		}
	}

	// Ensure output directory exists.
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("mkdir report dir: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create docx: %w", err)
	}
	defer f.Close()

	if _, err := w.WriteTo(f); err != nil {
		return fmt.Errorf("write docx: %w", err)
	}
	return nil
}

func truncateForDocx(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 3000 {
		return s
	}
	return s[:3000] + "...(truncated)"
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func linkToLine(l ragLink) string {
	parts := []string{}
	if l.Law != nil && strings.TrimSpace(*l.Law) != "" {
		parts = append(parts, *l.Law)
	}
	if l.Article != nil && strings.TrimSpace(*l.Article) != "" {
		parts = append(parts, "art. "+*l.Article)
	}
	if l.Section != nil && strings.TrimSpace(*l.Section) != "" {
		parts = append(parts, "sec. "+*l.Section)
	}
	if l.URL != nil && strings.TrimSpace(*l.URL) != "" {
		parts = append(parts, "url="+*l.URL)
	}
	if l.Text != nil && strings.TrimSpace(*l.Text) != "" {
		parts = append(parts, truncateForDocx(*l.Text))
	}
	if len(parts) == 0 {
		return "(link unavailable)"
	}
	return strings.Join(parts, " | ")
}

// Ensure rag_diff_stage.go types are reachable from this file.
var _ = ragDiffSmartResult{}

