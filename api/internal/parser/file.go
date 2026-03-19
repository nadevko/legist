package parser

import (
	"fmt"
	"os"
)

// readerAtFile оборачивает *os.File и добавляет size.
type readerAtFile struct {
	*os.File
	size int64
}

func openReaderAt(path string) (*readerAtFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}
	return &readerAtFile{File: f, size: info.Size()}, nil
}
