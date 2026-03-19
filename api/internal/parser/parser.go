package parser

import (
	"fmt"
	"io"
)

// Parser парсит документ из потока и возвращает структуру Document.
type Parser interface {
	Parse(r io.ReaderAt, size int64) (*Document, error)
}

// New возвращает парсер для указанного MIME типа.
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

// ParseFile парсит файл по пути.
func ParseFile(path, mimeType string) (*Document, error) {
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
