package apitest

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"slices"
	"testing"
)

// Form is a multipart/form-data body: its text fields and one file.
type Form struct {
	Fields   map[string]string
	FileName string
	File     []byte
}

// PostForm sends the form as multipart/form-data in a POST request, with
// the Idempotency-Key header set to key unless it is empty. Every call
// builds the form afresh, with a new random boundary, as a client
// rebuilding it to retry would.
func (a *App) PostForm(t testing.TB, path, token, key string, form Form) *Response {
	t.Helper()

	var body bytes.Buffer

	w := multipart.NewWriter(&body)

	names := make([]string, 0, len(form.Fields))
	for name := range form.Fields {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		if err := w.WriteField(name, form.Fields[name]); err != nil {
			t.Fatal(err)
		}
	}

	if form.File != nil {
		f, err := w.CreateFormFile("file", form.FileName)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := f.Write(form.File); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	header := http.Header{"Content-Type": {w.FormDataContentType()}}
	if key != "" {
		header.Set("Idempotency-Key", key)
	}

	return a.DoWithHeader(t, http.MethodPost, path, token, header, &body)
}
