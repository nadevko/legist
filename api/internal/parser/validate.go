package parser

import (
	"fmt"
	"io"
)

const maxFileSize = 50 * 1024 * 1024 // 50MB

var magicBytes = map[string][]byte{
	"application/pdf": []byte("%PDF"),
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": []byte("PK\x03\x04"),
}

// ValidateReader проверяет файл из io.Reader (для multipart upload).
// Читает только первые байты — не тратит время на весь файл.
func ValidateReader(r io.Reader, size int64, mimeType string) error {
	if size == 0 {
		return fmt.Errorf("file is empty")
	}
	if size > maxFileSize {
		return fmt.Errorf("file too large: %d bytes, max %d", size, maxFileSize)
	}

	magic, ok := magicBytes[mimeType]
	if !ok {
		return fmt.Errorf("unsupported mime type: %s", mimeType)
	}

	buf := make([]byte, len(magic))
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("read file header: %w", err)
	}

	for i, b := range magic {
		if buf[i] != b {
			return fmt.Errorf("file signature does not match mime type %s", mimeType)
		}
	}

	return nil
}

// Validate проверяет файл из io.ReaderAt (для уже сохранённого файла).
// - размер не превышает лимит
// - magic bytes совпадают с заявленным MIME типом
func Validate(r io.ReaderAt, size int64, mimeType string) error {
	if size == 0 {
		return fmt.Errorf("file is empty")
	}
	if size > maxFileSize {
		return fmt.Errorf("file too large: %d bytes, max %d", size, maxFileSize)
	}

	magic, ok := magicBytes[mimeType]
	if !ok {
		return fmt.Errorf("unsupported mime type: %s", mimeType)
	}

	buf := make([]byte, len(magic))
	if _, err := r.ReadAt(buf, 0); err != nil {
		return fmt.Errorf("read file header: %w", err)
	}

	for i, b := range magic {
		if buf[i] != b {
			return fmt.Errorf("file signature does not match mime type %s", mimeType)
		}
	}

	return nil
}
