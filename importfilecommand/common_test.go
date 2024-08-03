package importfilecommand_test

import (
	"github.com/looplab/eventhorizon/uuid"
	"io"
)

var fileUploadPath = "/testfiles"

var importFileUUID1 = uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
var fileDataRowUUID1 = uuid.MustParse("73874369-e9b2-4deb-aac7-2b020d2b84f7")
var accountUUID1 = uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772")
var tagUUID1 = uuid.MustParse("58430fe8-5f18-438e-8729-d8d55e2563cd")
var movementTypeUUiDString1 = "6b46349f-5681-445f-8f8d-89aa34b0ffdf"
var sourceAccountUUiDString1 = "d64755b6-58e0-40fe-b343-a1518f18f8bb"

var _ io.ReadCloser = &testFile{}

type testFile struct {
	reader      io.Reader
	readCalled  int
	closeCalled int
}

func newTestFile(content io.Reader) *testFile {
	return &testFile{
		reader:      content,
		readCalled:  0,
		closeCalled: 0,
	}
}

func (t *testFile) Read(p []byte) (n int, err error) {
	t.readCalled++

	return t.reader.Read(p)
}

func (t *testFile) Close() error {
	t.closeCalled++

	return nil
}
