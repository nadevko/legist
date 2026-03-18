package api

// errorResponse is used in swagger annotations for error responses.
//
//nolint:unused
type errorResponse struct {
	Message string `json:"message"`
}

// fileResponse is the public representation of a file.
type fileResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// jobFileResponse is the public representation of a job file entry.
type jobFileResponse struct {
	FileID string  `json:"file_id"`
	Status string  `json:"status"`
	Error  *string `json:"error,omitempty"`
}

// jobResponse is the public representation of a job.
type jobResponse struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Status    string            `json:"status"`
	CreatedAt string            `json:"created_at"`
	ExpiresAt string            `json:"expires_at"`
	Files     []jobFileResponse `json:"files"`
}

var allowedMIME = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}
