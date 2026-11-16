package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/openapi"
	"github.com/ogen-go/ogen/validate"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
)

var operation = ogenerrors.OperationContext{Name: "CreditAccount", ID: "creditAccount"}

func TestRequestErrorsAreCleanProblemDetails(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err        error
		wantStatus int
		wantDetail string
	}{
		"body that is not JSON": {
			err: &ogenerrors.DecodeRequestError{
				OperationContext: operation,
				Err:              errors.New(`decode application/json: invalid: unexpected byte 123 '{' at 0`),
			},
			wantStatus: http.StatusBadRequest,
			wantDetail: "The request body is not valid.",
		},
		"body that fails validation": {
			err: &ogenerrors.DecodeRequestError{
				OperationContext: operation,
				Err: &validate.Error{Fields: []validate.FieldError{
					{Name: "currency", Error: errors.New(`string: no regex match: ^[A-Z]{3}$`)},
				}},
			},
			wantStatus: http.StatusBadRequest,
			wantDetail: "Invalid value for: currency.",
		},
		"parameter that is not a UUID": {
			err: &ogenerrors.DecodeParamsError{
				OperationContext: operation,
				Err: &ogenerrors.DecodeParamError{
					Name: "account_id",
					In:   openapi.LocationPath,
					Err:  errors.New("parse uuid: invalid UUID length: 3"),
				},
			},
			wantStatus: http.StatusBadRequest,
			wantDetail: `Invalid path parameter "account_id".`,
		},
		"unsupported content type": {
			err: &ogenerrors.DecodeRequestError{
				OperationContext: operation,
				Err:              &validate.InvalidContentTypeError{ContentType: "text/plain"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantDetail: "Unsupported content type.",
		},
		"unexpected failure": {
			err:        errors.New("pq: connection refused at 10.0.0.3"),
			wantStatus: http.StatusInternalServerError,
			wantDetail: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			httpapi.Errors{}.HandleError(t.Context(), w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), tc.err)

			p := decodeProblem(t, w, tc.wantStatus)
			if p.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", p.Detail, tc.wantDetail)
			}

			for _, leak := range []string{"CreditAccount", "creditAccount", "decode", "10.0.0.3", "regex"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("body leaks %q: %s", leak, w.Body)
				}
			}
		})
	}
}

func TestNewErrorAlwaysSetsAStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err  error
		want int
	}{
		"authentication failed": {
			err:  &ogenerrors.SecurityError{OperationContext: operation, Security: "bearer_auth", Err: errors.New("bad token")},
			want: http.StatusUnauthorized,
		},
		"unexpected handler error": {err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := httpapi.Errors{}.NewError(t.Context(), tc.err)
			if got.StatusCode != tc.want || int(got.Response.Status) != tc.want {
				t.Fatalf("status = %d (body %d), want %d", got.StatusCode, got.Response.Status, tc.want)
			}

			if strings.Contains(got.Response.Detail.Or(""), "boom") || strings.Contains(got.Response.Detail.Or(""), "bad token") {
				t.Errorf("detail leaks the internal error: %q", got.Response.Detail.Or(""))
			}
		})
	}
}

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func decodeProblem(t *testing.T, w *httptest.ResponseRecorder, status int) problem {
	t.Helper()

	if w.Code != status {
		t.Fatalf("status = %d, want %d", w.Code, status)
	}

	if ct := w.Header().Get("Content-Type"); ct != httpapi.ProblemContentType {
		t.Fatalf("Content-Type = %q", ct)
	}

	var p problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}

	if p.Status != status || p.Type != httpapi.BlankType || p.Title == "" {
		t.Fatalf("problem = %+v", p)
	}

	return p
}
