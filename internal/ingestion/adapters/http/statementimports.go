package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ht "github.com/ogen-go/ogen/http"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/amount"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Problem types of the Statement import operations.
const (
	DuplicateStatementImportType      = "/problems/duplicate-statement-import"
	OpeningBalanceAlreadyRecordedType = "/problems/opening-balance-already-recorded"
	ExportBalanceMismatchType         = "/problems/export-balance-mismatch"
	InvalidExportBalanceType          = "/problems/invalid-export-balance"
	UnreadableStatementFileType       = "/problems/unreadable-statement-file"
	EmptyStatementFileType            = "/problems/empty-statement-file"
	StatementFileTooLargeType         = "/problems/statement-file-too-large"
	CSVMappingRequiredType            = "/problems/csv-mapping-required"
	PreviewRequiredType               = "/problems/preview-required"
	PreviewMismatchType               = "/problems/preview-mismatch"
	BalanceCheckFailedType            = "/problems/balance-check-failed"
	InvalidCursorType                 = "/problems/invalid-cursor"

	NotTheLatestStatementImportType    = "/problems/not-the-latest-statement-import"
	StatementImportHasAcceptedRowsType = "/problems/statement-import-has-accepted-rows"
	StatementImportAlreadyVoidedType   = "/problems/statement-import-already-voided"

	StatementImportTouchesClosedMonthType = "/problems/statement-import-touches-closed-month"
)

// MaxStatementFile is the largest statement file accepted, far above any
// bank's monthly export.
const MaxStatementFile = 16 << 20

// DefaultPageSize is how many items a page holds when the request doesn't
// say.
const DefaultPageSize = 50

// StatementImportHandler implements the Statement import and Imported
// transaction operations of oas.Handler.
type StatementImportHandler struct {
	imports  *app.StatementImports
	imported *app.ImportedTransactions
}

// NewStatementImportHandler returns the Statement import operations of the
// Ingestion HTTP adapter.
func NewStatementImportHandler(imports *app.StatementImports, imported *app.ImportedTransactions) *StatementImportHandler {
	return &StatementImportHandler{imports: imports, imported: imported}
}

// outcome is how a refused preview or upload is answered.
type outcome struct {
	notFound bool
	conflict *oas.StatementImportConflictProblem
	refused  *oas.StatementFileProblem
}

// PreviewStatementImport implements previewStatementImport.
func (h *StatementImportHandler) PreviewStatementImport(ctx context.Context, req *oas.StatementImportPreviewUploadMultipart,
	params oas.PreviewStatementImportParams,
) (oas.PreviewStatementImportRes, error) {
	up, refused := upload(req.File, req.ExportBalance)
	if refused == nil {
		up.Confirmation, refused = exportBalance(req.ExportBalanceConfirmation)
	}

	if refused != nil {
		return refused, nil
	}

	p, err := h.imports.Preview(ctx, params.HouseholdID, params.ConnectionID, up)
	if o, ok := refusal(err); ok {
		switch {
		case o.notFound:
			return new(oas.PreviewStatementImportNotFound(httpapi.NotFoundProblem())), nil
		case o.conflict != nil:
			return o.conflict, nil
		default:
			return o.refused, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("preview statement import: %w", err)
	}

	out := &oas.StatementImportPreview{
		PreviewID:                  p.ID,
		StartDate:                  p.StartDate,
		RowCount:                   int32(p.Rows),    //nolint:gosec // a statement file is far smaller
		RowsIgnoredBeforeStartDate: int32(p.Ignored), //nolint:gosec // a statement file is far smaller
	}

	for dst, v := range map[*oas.Money]sharedkernel.Decimal{
		&out.OpeningBalance: p.OpeningBalance, &out.ExportBalance: p.ExportBalance, &out.RowsTotal: p.Total,
	} {
		if *dst, err = money(v, p.Currency); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// UploadStatementImport implements uploadStatementImport.
func (h *StatementImportHandler) UploadStatementImport(ctx context.Context, req *oas.StatementImportUploadMultipart,
	params oas.UploadStatementImportParams,
) (oas.UploadStatementImportRes, error) {
	up, refused := upload(req.File, req.ExportBalance)
	if refused != nil {
		return refused, nil
	}

	up.PreviewID = req.PreviewID.Or("")

	recorded, err := h.imports.Upload(ctx, params.HouseholdID, params.ConnectionID, up)
	if o, ok := refusal(err); ok {
		switch {
		case o.notFound:
			return new(oas.UploadStatementImportNotFound(httpapi.NotFoundProblem())), nil
		case o.conflict != nil:
			return o.conflict, nil
		default:
			return o.refused, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("upload statement import: %w", err)
	}

	return statementImportResponse(recorded)
}

// VoidStatementImport implements voidStatementImport.
func (h *StatementImportHandler) VoidStatementImport(ctx context.Context, params oas.VoidStatementImportParams,
) (oas.VoidStatementImportRes, error) {
	voided, err := h.imports.Void(ctx, params.HouseholdID, params.ConnectionID, params.StatementImportID)

	var touches *domain.TouchesClosedMonthError

	conflict := func(typ, title, detail string) (oas.VoidStatementImportRes, error) {
		return httpapi.MonthConflict(typ, title, detail, "", ""), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrStatementImportNotFound):
		return new(oas.VoidStatementImportNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrStatementImportAlreadyVoided):
		return conflict(StatementImportAlreadyVoidedType, "Statement import already voided",
			"The Statement import is already voided.")
	case errors.Is(err, app.ErrNotTheLatestStatementImport):
		return conflict(NotTheLatestStatementImportType, "Not the latest Statement import",
			"Only the Account's latest Statement import can be voided: void every later one first.")
	case errors.Is(err, app.ErrStatementImportHasAcceptedRows), errors.Is(err, app.ErrImportedTransactionAlreadyAccepted):
		return conflict(StatementImportHasAcceptedRowsType, "Statement import has Accepted rows",
			"Some of the Statement import's Imported transactions are Accepted: reverse their Transactions first.")
	case errors.As(err, &touches):
		month := httpapi.MonthWords(touches.Month)

		return httpapi.MonthConflict(StatementImportTouchesClosedMonthType, "Statement import touches a closed month",
			fmt.Sprintf("This import has rows in %s, which is closed. Reopen %s first.", month, month),
			touches.Month, touches.OpenMonth), nil
	case err != nil:
		return nil, fmt.Errorf("void statement import: %w", err)
	}

	return statementImportResponse(voided)
}

// ListStatementImports implements listStatementImports.
func (h *StatementImportHandler) ListStatementImports(ctx context.Context, params oas.ListStatementImportsParams,
) (oas.ListStatementImportsRes, error) {
	list, err := h.imports.List(ctx, params.HouseholdID, params.ConnectionID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrConnectionNotFound):
		return new(oas.ListStatementImportsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list statement imports: %w", err)
	}

	out := &oas.StatementImportList{Items: make([]oas.StatementImport, 0, len(list))}

	for _, i := range list {
		res, err := statementImportResponse(i)
		if err != nil {
			return nil, err
		}

		out.Items = append(out.Items, *res)
	}

	return out, nil
}

// ListImportedTransactions implements listImportedTransactions.
func (h *StatementImportHandler) ListImportedTransactions(ctx context.Context, params oas.ListImportedTransactionsParams,
) (oas.ListImportedTransactionsRes, error) {
	badRequest := func(typ, title, detail string) (oas.ListImportedTransactionsRes, error) {
		return new(oas.ListImportedTransactionsBadRequest(httpapi.Problem(http.StatusBadRequest, typ, title, detail))), nil
	}

	filter := app.ImportedFilter{Limit: int(params.Limit.Or(DefaultPageSize))}

	if s, ok := params.State.Get(); ok {
		state, known := importedStatesIn[s]
		if !known {
			return badRequest("", "", "The state must be PENDING, SUSPECTED_DUPLICATE, ACCEPTED or DISCARDED.")
		}

		filter.State = state
	}

	if m, ok := params.BookingMonth.Get(); ok {
		month, err := time.Parse("2006-01", m)
		if err != nil {
			return badRequest("", "", "The booking_month must be written YYYY-MM.")
		}

		filter.Month = month
	}

	if c, ok := params.Cursor.Get(); ok {
		cursor, err := app.ParseImportedCursor(c)
		if err != nil {
			return badRequest(InvalidCursorType, "Invalid cursor", "The cursor is not one this API handed out.")
		}

		filter.After = &cursor
	}

	page, err := h.imported.List(ctx, params.HouseholdID, params.AccountID, filter)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.ListImportedTransactionsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list imported transactions: %w", err)
	}

	out := &oas.ImportedTransactionPage{Items: make([]oas.ImportedTransaction, 0, len(page.Items))}

	for _, i := range page.Items {
		item, err := importedTransactionOut(i)
		if err != nil {
			return nil, err
		}

		out.Items = append(out.Items, item)
	}

	if page.Next != nil {
		out.NextCursor = oas.NewOptString(page.Next.String())
	}

	return out, nil
}

// upload reads the file and the Export balance of a request.
func upload(file ht.MultipartFile, balance oas.DecimalText) (app.Upload, *oas.StatementFileProblem) {
	raw, err := io.ReadAll(io.LimitReader(file.File, MaxStatementFile+1))
	if err != nil {
		return app.Upload{}, fileProblem(StatementFileTooLargeType, "Unreadable upload", "The file could not be read.")
	}

	if len(raw) > MaxStatementFile {
		return app.Upload{}, fileProblem(StatementFileTooLargeType, "Statement file too large",
			fmt.Sprintf("A statement file may be at most %d MiB.", MaxStatementFile>>20))
	}

	export, problem := exportBalance(balance)

	return app.Upload{File: raw, ExportBalance: export}, problem
}

func exportBalance(v oas.DecimalText) (sharedkernel.Decimal, *oas.StatementFileProblem) {
	d, err := sharedkernel.ParseDecimal(string(v))
	if err != nil {
		return sharedkernel.Decimal{}, fileProblem(InvalidExportBalanceType, "Invalid Export balance",
			"The Export balance must be a decimal number.")
	}

	return d, nil
}

// refusal maps a refused preview or upload to its answer.
func refusal(err error) (outcome, bool) {
	refused := func(typ, title, detail string) (outcome, bool) {
		return outcome{refused: fileProblem(typ, title, detail)}, true
	}

	conflict := func(typ, title, detail string) *oas.StatementImportConflictProblem {
		return &oas.StatementImportConflictProblem{
			Type: typ, Title: title, Status: http.StatusConflict, Detail: oas.NewOptString(detail),
		}
	}

	var (
		duplicate *app.DuplicateImportError
		rows      *domain.RowsError
		failed    *domain.BalanceCheckFailedError
	)

	switch {
	case err == nil:
		return outcome{}, false
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrConnectionNotFound),
		errors.Is(err, app.ErrAccountNotFound):
		return outcome{notFound: true}, true
	case errors.As(err, &duplicate):
		c := conflict(DuplicateStatementImportType, "Statement file already imported",
			"This file was already imported through the Connection as Statement import "+duplicate.Earlier.String()+".")
		c.StatementImportID = oas.NewOptUUID(duplicate.Earlier)

		return outcome{conflict: c}, true
	case errors.Is(err, app.ErrOpeningBalanceAlreadyRecorded):
		return outcome{conflict: conflict(OpeningBalanceAlreadyRecordedType, "Opening balance already recorded",
			"The Account already has its Opening balance from its first Statement import.")}, true
	case errors.As(err, &rows):
		p := fileProblem(UnreadableStatementFileType, "Unreadable statement file",
			"The statement file was not imported: every line listed in rows can't be read.")
		for _, r := range rows.Rows {
			p.Rows = append(p.Rows, oas.RowError{Line: int32(r.Line), Reason: r.Reason}) //nolint:gosec // a statement file is far smaller
		}

		return outcome{refused: p}, true
	case errors.Is(err, domain.ErrEmptyStatementFile):
		return refused(EmptyStatementFileType, "Empty statement file", "The statement file has no rows.")
	case errors.Is(err, domain.ErrInvalidExportBalance):
		return refused(InvalidExportBalanceType, "Invalid Export balance",
			"The Export balance has more decimal places than the Account currency.")
	case errors.Is(err, app.ErrExportBalanceMismatch):
		return refused(ExportBalanceMismatchType, "Export balance entries differ",
			"The Export balance and its confirmation must be the same amount.")
	case errors.Is(err, app.ErrNoCSVMapping):
		return refused(CSVMappingRequiredType, "CSV mapping required",
			"The Connection needs a CSV mapping before a statement file can be imported.")
	case errors.Is(err, app.ErrPreviewRequired):
		return refused(PreviewRequiredType, "Preview required",
			"The Account's first Statement import needs the preview_id of its preview.")
	case errors.Is(err, app.ErrPreviewMismatch):
		return refused(PreviewMismatchType, "Upload doesn't match its preview",
			"The file, the Export balance or the Connection's CSV mapping differs from the preview; preview it again.")
	case errors.As(err, &failed):
		return balanceCheckFailed(failed)
	default:
		return outcome{}, false
	}
}

// balanceCheckFailed answers a later import refused by its Balance check
// with the expected balance, the Export balance and the difference. When
// rows of the file in closed months aren't in the books, it counts them
// and names each month to Reopen (spec story 13).
func balanceCheckFailed(e *domain.BalanceCheckFailedError) (outcome, bool) {
	detail := fmt.Sprintf(
		"The Account's Opening balance and Imported transactions, with this file's new rows, come to %s %s, "+
			"but the Export balance is %s %s (difference %s). Nothing was imported: check the file, its mapping "+
			"and the Export balance.", e.Expected, e.Currency, e.ExportBalance, e.Currency, e.Difference)

	if n := e.RowsInClosedMonthsNotInBooks(); n > 0 {
		detail += " " + closedMonthsNotInBooks(e.ClosedMonthsNotInBooks)
	}

	p := fileProblem(BalanceCheckFailedType, "Balance check failed", detail)

	if n := e.RowsInClosedMonthsNotInBooks(); n > 0 {
		p.RowsInClosedMonthsNotInBooks = oas.NewOptInt32(int32(n)) //nolint:gosec // a statement file is far smaller
	}

	for dst, v := range map[*oas.OptMoney]sharedkernel.Decimal{
		&p.ExpectedBalance: e.Expected, &p.ExportBalance: e.ExportBalance, &p.Difference: e.Difference,
	} {
		m, err := money(v, e.Currency)
		if err != nil {
			// The Decider checked the currency; the amounts are the
			// currency's own sums, so this can't happen.
			return outcome{}, false
		}

		*dst = oas.NewOptMoney(m)
	}

	return outcome{refused: p}, true
}

// closedMonthsNotInBooks says how many of the file's rows fall in closed
// months without being in the books, and which months to Reopen, latest
// first, since only the latest closed month reopens:
// "2 rows of this file fall in closed January 2026 and aren't in the books.
// Reopen January 2026, then import the file again." Months come in order.
func closedMonthsNotInBooks(months []domain.MonthRows) string {
	if len(months) == 1 {
		m, name := months[0], httpapi.MonthWords(months[0].Month)
		if m.Rows == 1 {
			return fmt.Sprintf("1 row of this file falls in closed %s and isn't in the books. "+
				"Reopen %s, then import the file again.", name, name)
		}

		return fmt.Sprintf("%d rows of this file fall in closed %s and aren't in the books. "+
			"Reopen %s, then import the file again.", m.Rows, name, name)
	}

	total := 0
	counts := make([]string, 0, len(months))
	reopen := make([]string, 0, len(months))

	for _, m := range months {
		total += m.Rows
		counts = append(counts, fmt.Sprintf("%d in %s", m.Rows, httpapi.MonthWords(m.Month)))
		reopen = append([]string{httpapi.MonthWords(m.Month)}, reopen...)
	}

	return fmt.Sprintf("%d rows of this file fall in closed months and aren't in the books: %s. "+
		"Reopen %s, then import the file again.", total, listWords(counts), strings.Join(reopen, ", then "))
}

// listWords joins words as a sentence lists them: "a", "a and b",
// "a, b and c".
func listWords(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}

	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

func fileProblem(typ, title, detail string) *oas.StatementFileProblem {
	return &oas.StatementFileProblem{
		Type: typ, Title: title, Status: http.StatusUnprocessableEntity, Detail: oas.NewOptString(detail),
	}
}

var balanceChecksOut = map[domain.BalanceCheck]string{
	domain.BalanceCheckPassed:  "PASSED",
	domain.BalanceCheckWaiting: "WAITING",
	domain.BalanceCheckFailed:  "FAILED",
}

var voidBlockNames = map[domain.VoidBlock]string{
	domain.VoidBlockedNotLatest:          "NOT_LATEST",
	domain.VoidBlockedHasAcceptedRows:    "HAS_ACCEPTED_ROWS",
	domain.VoidBlockedTouchesClosedMonth: "TOUCHES_CLOSED_MONTH",
}

// voidBlocksOut writes the reasons a Void would be refused as the API names
// them; an empty list when it is allowed.
func voidBlocksOut(blocks []domain.VoidBlock) []string {
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, voidBlockNames[b])
	}

	return out
}

func statementImportResponse(i app.StatementImport) (*oas.StatementImport, error) {
	s := i.State
	out := &oas.StatementImport{
		VoidBlockedBy:               voidBlocksOut(i.VoidBlockedBy),
		StatementImportID:           i.ID,
		ConnectionID:                s.ConnectionID,
		AccountID:                   s.AccountID,
		RowsImported:                int32(s.RowsImported),              //nolint:gosec // a statement file is far smaller
		RowsSkippedAsCertainRepeats: int32(s.RowsSkipped),               //nolint:gosec // a statement file is far smaller
		SuspectedDuplicates:         int32(s.SuspectedDuplicates),       //nolint:gosec // a statement file is far smaller
		RowsIgnoredBeforeStartDate:  int32(s.RowsIgnored),               //nolint:gosec // a statement file is far smaller
		RowsIgnoredInClosedMonths:   int32(s.RowsIgnoredInClosedMonths), //nolint:gosec // a statement file is far smaller
		BalanceCheck:                balanceChecksOut[s.BalanceCheck],
		ImportedAt:                  s.RecordedAt,
	}

	var err error

	if out.ExportBalance, err = money(s.ExportBalance, s.Currency); err != nil {
		return nil, err
	}

	if out.RowsTotal, err = money(s.RowsTotal, s.Currency); err != nil {
		return nil, err
	}

	if s.Voided() {
		out.VoidedAt = oas.NewOptDateTime(s.VoidedAt)
	}

	if s.OpeningBalance != nil {
		opening, err := money(*s.OpeningBalance, s.Currency)
		if err != nil {
			return nil, err
		}

		out.OpeningBalance = oas.NewOptMoney(opening)
	}

	return out, nil
}

// money is an amount in the currency as the API writes it, with the
// currency's decimal places (ADR-0007).
func money(value sharedkernel.Decimal, currency string) (oas.Money, error) {
	a, err := amount.Of(value, currency)
	if err != nil {
		return oas.Money{}, fmt.Errorf("write %s %s: %w", value, currency, err)
	}

	return oas.Money{Amount: a, Currency: oas.CurrencyCode(currency)}, nil
}
