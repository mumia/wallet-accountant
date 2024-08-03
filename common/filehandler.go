package common

import (
	"bytes"
	"io"
	"os"
)

var _ Filer = &FileHandler{}

type Filer interface {
	Open(filePath string) (io.ReadCloser, error)
	Save(filePath string, content *bytes.Buffer) error
}

type FileHandler struct {
}

func NewFileHandler() *FileHandler {
	return &FileHandler{}
}

func (fileHandler *FileHandler) Open(filePath string) (io.ReadCloser, error) {
	return os.OpenFile(filePath, os.O_RDONLY, 0660)
}

func (fileHandler *FileHandler) Save(filePath string, content *bytes.Buffer) error {
	return os.WriteFile(filePath, content.Bytes(), 0660)
}
