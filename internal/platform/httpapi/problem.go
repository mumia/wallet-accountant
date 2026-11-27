package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// ProblemContentType is the media type of every error response (RFC 9457).
const ProblemContentType = "application/problem+json"

// NotImplementedType is the problem type of an operation the contract
// already has but the server doesn't implement yet.
const NotImplementedType = "/problems/not-implemented"

// ErrNotImplemented is returned by a handler for an operation the contract
// already has but the server doesn't implement yet. It leaves as 501.
var ErrNotImplemented = errors.New("not implemented yet")

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

// MonthConflict returns a 409 as a MonthProblem: a conflict with the current
// state that may name the Booking month it is about and the Account's open
// month (ADR-0024). Either month is left out when empty.
func MonthConflict(typ, title, detail, bookingMonth, openMonth string) *oas.MonthProblem {
	p := Problem(http.StatusConflict, typ, title, detail)
	out := &oas.MonthProblem{Type: p.Type, Title: p.Title, Status: p.Status, Detail: p.Detail}

	if bookingMonth != "" {
		out.BookingMonth = oas.NewOptString(bookingMonth)
	}

	if openMonth != "" {
		out.OpenMonth = oas.NewOptString(openMonth)
	}

	return out
}

// Unprocessable returns a 422 as a TransferProblem, the schema of every 422
// of an operation that can refuse a Transfer. It has no reason; a refused
// Transfer sets one.
func Unprocessable(typ, title, detail string) *oas.TransferProblem {
	p := Problem(http.StatusUnprocessableEntity, typ, title, detail)

	return &oas.TransferProblem{Type: p.Type, Title: p.Title, Status: p.Status, Detail: p.Detail}
}

// CategoryDirectionType is the problem type of a Category whose kind
// doesn't fit the direction of the amount.
const CategoryDirectionType = "/problems/category-direction"

// CategoryDirection is the 422 of a Category refused because its kind,
// INCOME or EXPENSE, doesn't fit the amount's direction, IN or OUT.
func CategoryDirection(category uuid.UUID, kind, direction string) *oas.TransferProblem {
	detail := "An Expense Category takes only money out, and this amount is money in. " +
		"Choose an Income Category, or Uncategorised."
	if kind == "INCOME" {
		detail = "An Income Category takes only money in, and this amount is money out. " +
			"Choose an Expense Category, or Uncategorised."
	}

	p := Unprocessable(CategoryDirectionType, "Category doesn't fit the amount", detail)
	p.CategoryID = oas.NewOptUUID(category)
	p.CategoryKind = oas.NewOptString(kind)
	p.Direction = oas.NewOptString(direction)

	return p
}

// Problem types about a month and the Account's Open month (ADR-0024).
const (
	MonthClosedType     = "/problems/month-closed"
	MonthNotOpenYetType = "/problems/month-not-open-yet"
)

// MonthClosed is the 409 of a change to a closed Booking month (YYYY-MM).
func MonthClosed(month, openMonth string) *oas.MonthProblem {
	return MonthConflict(MonthClosedType, "Month closed",
		MonthWords(month)+" is closed. Reopen it to change its entries.", month, openMonth)
}

// MonthNotOpenYet is the 409 of a Booking month (YYYY-MM) after the
// Account's Open month.
func MonthNotOpenYet(month, openMonth string) *oas.MonthProblem {
	return MonthConflict(MonthNotOpenYetType, "Month not open yet",
		fmt.Sprintf("%s isn't open yet. Close %s first.", MonthWords(month), MonthWords(openMonth)), month, openMonth)
}

// MonthWords writes a Booking month (YYYY-MM) as problem copy writes it:
// "January 2026". A month that doesn't parse is written as given.
func MonthWords(month string) string {
	m, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}

	return m.Format("January 2006")
}

// DateWords writes a date as problem copy writes it: "1 December 2026".
func DateWords(d time.Time) string { return d.Format("2 January 2006") }

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

	if errors.Is(err, ErrNotImplemented) {
		return &oas.ProblemStatusCode{StatusCode: http.StatusNotImplemented, Response: Problem(http.StatusNotImplemented,
			NotImplementedType, "Not implemented yet", "The server doesn't implement this operation yet.")}
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
