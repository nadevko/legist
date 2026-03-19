package parser

import (
	"fmt"
	"io"
)

// Parser parses a document from a stream into a section tree.
type Parser interface {
	Parse(r io.ReaderAt, size int64) (*Document, error)
}

// New returns the appropriate parser for the given MIME type.
func New(mimeType string) (Parser, error) {
	switch mimeType {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return &docxParser{}, nil
	case "application/pdf":
		return &pdfParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported mime type: %s", mimeType)
	}
}

// ParseFile parses a file by path and returns the raw section tree.
// For PDF, passes the path directly to pdftotext (more reliable than stdin).
func ParseFile(path, mimeType string) (*Document, error) {
	if mimeType == "application/pdf" {
		doc, err := pdfParseByPath(path)
		if err != nil {
			return nil, fmt.Errorf("parse pdf: %w", err)
		}
		return doc, nil
	}

	p, err := New(mimeType)
	if err != nil {
		return nil, fmt.Errorf("create parser: %w", err)
	}
	f, err := openReaderAt(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	doc, err := p.Parse(f, f.size)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", mimeType, err)
	}
	return doc, nil
}
