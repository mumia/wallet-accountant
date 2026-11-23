package apitest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// Put sends body as JSON in a PUT request.
func (a *App) Put(t testing.TB, path, token string, body any) *Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return a.Do(t, http.MethodPut, path, token, bytes.NewReader(raw))
}

// Patch sends body as JSON in a PATCH request.
func (a *App) Patch(t testing.TB, path, token string, body any) *Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return a.Do(t, http.MethodPatch, path, token, bytes.NewReader(raw))
}
