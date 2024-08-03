package common

import (
	"bytes"
	"io"
)

var _ Filer = &FileHandlerMock{}

type FileHandlerMock struct {
	OpenFn func(filePath string) (io.ReadCloser, error)
	SaveFn func(filePath string, content *bytes.Buffer) error
}

func (mock *FileHandlerMock) Open(filePath string) (io.ReadCloser, error) {
	if mock != nil && mock.OpenFn != nil {
		return mock.OpenFn(filePath)
	}

	return nil, nil
}

func (mock *FileHandlerMock) Save(filePath string, content *bytes.Buffer) error {
	if mock != nil && mock.SaveFn != nil {
		return mock.SaveFn(filePath, content)
	}

	return nil
}
