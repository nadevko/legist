package llm

import (
	"errors"
	"fmt"
)

// DownloadProgress describes pull/download progress for model provisioning.
// Ollama emits streaming progress events; we surface the latest snapshot here.
type DownloadProgress struct {
	// Status is a human-readable stage, e.g. "pulling", "downloading", "verifying".
	Status string `json:"status"`
	// CompletedBytes is how many bytes are known to be downloaded so far.
	CompletedBytes int64 `json:"completed_bytes"`
	// TotalBytes is total size if known (0 when unknown).
	TotalBytes int64 `json:"total_bytes"`
}

// ErrModelMissing is returned when the provider cannot run generation because
// the requested model is not installed.
// Implementations should include a best-effort DownloadProgress snapshot.
type ErrModelMissing struct {
	Model          string
	PullStarted    bool
	Progress       DownloadProgress
	UnderlyingErr  error
}

func (e *ErrModelMissing) Error() string {
	if e == nil {
		return "model missing"
	}
	if e.Progress.Status == "" && e.Progress.CompletedBytes == 0 && e.Progress.TotalBytes == 0 {
		if e.PullStarted {
			return fmt.Sprintf("model %q is not installed; pull started but no progress yet", e.Model)
		}
		return fmt.Sprintf("model %q is not installed", e.Model)
	}
	p := e.Progress
	pct := ""
	if p.TotalBytes > 0 {
		pct = fmt.Sprintf(" (%.1f%%)", float64(p.CompletedBytes)*100/float64(p.TotalBytes))
	}
	if e.PullStarted {
		return fmt.Sprintf("model %q is not installed; pull started: %s %d/%d bytes%s", e.Model, p.Status, p.CompletedBytes, p.TotalBytes, pct)
	}
	return fmt.Sprintf("model %q is not installed: %s %d/%d bytes%s", e.Model, p.Status, p.CompletedBytes, p.TotalBytes, pct)
}

func (e *ErrModelMissing) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.UnderlyingErr
}

func IsModelMissing(err error) bool {
	var target *ErrModelMissing
	return errors.As(err, &target)
}

