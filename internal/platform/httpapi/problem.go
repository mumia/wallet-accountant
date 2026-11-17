package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// ProblemContentType is the media type of every error response (RFC 9457).
const ProblemContentType = "application/problem+json"

// BlankType is the problem type used when the status code says it all.
const BlankType = "about:blank"

// Problem returns problem details for status. The title is the status text;
// typ is BlankType unless the problem has its own type.
func Problem(status int, typ, title, detail string) oas.Problem {
	if typ == "" {
		typ = BlankType
	}

	if title == "" {
		title = http.StatusText(status)
	}

	p := oas.Problem{Type: typ, Title: title, Status: int32(status)} //nolint:gosec // HTTP status codes fit in int32
	if detail != "" {
		p.Detail = oas.NewOptString(detail)
	}

	return p
}

// NotFoundProblem is the 404 of a resource the caller may not know exists.
// It says nothing more, so it never confirms that the resource exists.
func NotFoundProblem() oas.Problem {
	return Problem(http.StatusNotFound, "", "", "")
}

// Errors implements the error hooks of the ogen server so that every error
// leaves as application/problem+json with its status set. It is embedded in
// the application's oas.Handler to provide NewError.
type Errors struct {
	Logger Logger
}

// Logger is the subset of *slog.Logger the error hooks need.
type Logger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// NewError handles authentication failures and unexpected handler errors.
// ogen would send 200 if the status were left at 0, so it is always set.
func (e Errors) NewError(ctx context.Context, err error) *oas.ProblemStatusCode {
	if _, ok := errors.AsType[*ogenerrors.SecurityError](err); ok {
		return problemStatus(http.StatusUnauthorized, "A valid bearer token is required.")
	}

	if e.Logger != nil {
		e.Logger.ErrorContext(ctx, "unexpected handler error", "error", err)
	}

	return problemStatus(http.StatusInternalServerError, "")
}

// HandleError handles errors raised before a handler runs: decoding and
// validating parameters and bodies. The details never repeat ogen's message,
// which names internal operations and decode paths.
func (e Errors) HandleError(ctx context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	if _, ok := errors.AsType[*ogenerrors.SecurityError](err); ok {
		WriteProblem(w, Problem(http.StatusUnauthorized, "", "", "A valid bearer token is required."))

		return
	}

	status := ogenerrors.ErrorCode(err)
	if status >= http.StatusInternalServerError {
		if e.Logger != nil {
			e.Logger.ErrorContext(ctx, "unexpected request error", "error", err)
		}

		WriteProblem(w, Problem(status, "", "", ""))

		return
	}

	WriteProblem(w, Problem(status, "", "", requestErrorDetail(err)))
}

// NotFound answers a request for an unknown route.
func (Errors) NotFound(w http.ResponseWriter, _ *http.Request) {
	WriteProblem(w, Problem(http.StatusNotFound, "", "", ""))
}

// MethodNotAllowed answers a request with a method the route doesn't support.
func (Errors) MethodNotAllowed(w http.ResponseWriter, _ *http.Request, allowed string) {
	w.Header().Set("Allow", allowed)
	WriteProblem(w, Problem(http.StatusMethodNotAllowed, "", "", "Allowed methods: "+allowed+"."))
}

// WriteProblem writes p as application/problem+json with p.Status.
func WriteProblem(w http.ResponseWriter, p oas.Problem) {
	body := map[string]any{"type": p.Type, "title": p.Title, "status": p.Status}
	if d, ok := p.Detail.Get(); ok {
		body["detail"] = d
	}

	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(int(p.Status))
	_ = json.NewEncoder(w).Encode(body) //nolint:errchkjson // the response is already committed
}

func problemStatus(status int, detail string) *oas.ProblemStatusCode {
	return &oas.ProblemStatusCode{StatusCode: status, Response: Problem(status, "", "", detail)}
}

func requestErrorDetail(err error) string {
	if v, ok := errors.AsType[*validate.Error](err); ok {
		names := make([]string, 0, len(v.Fields))
		for _, f := range v.Fields {
			names = append(names, f.Name)
		}

		return fmt.Sprintf("Invalid value for: %s.", strings.Join(names, ", "))
	}

	if p, ok := errors.AsType[*ogenerrors.DecodeParamError](err); ok {
		return fmt.Sprintf("Invalid %s parameter %q.", p.In, p.Name)
	}

	if _, ok := errors.AsType[*validate.InvalidContentTypeError](err); ok {
		return "Unsupported content type."
	}

	if _, ok := errors.AsType[*ogenerrors.DecodeParamsError](err); ok {
		return "The request parameters are not valid."
	}

	return "The request body is not valid."
}
