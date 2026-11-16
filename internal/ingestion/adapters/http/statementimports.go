package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	LaterImportsNotSupportedType      = "/problems/later-statement-imports-not-supported"
	InvalidCursorType                 = "/problems/invalid-cursor"
)

// MaxStatementFile is the largest statement file accepted, far above any
// bank's monthly export.
const MaxStatementFile = 16 << 20

// DefaultPageSize is how many items a page holds when the request doesn't
// say.
const DefaultPageSize = 50

var importedStatesOut = map[domain.ImportedState]string{domain.ImportedPending: "PENDING"}

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
		switch s {
		case "PENDING":
			filter.State = domain.ImportedPending
		default:
			return badRequest("", "", "The state must be PENDING.")
		}
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
		s := i.State

		a, err := money(s.Amount, s.Currency)
		if err != nil {
			return nil, err
		}

		item := oas.ImportedTransaction{
			ImportedTransactionID: i.ID,
			StatementImportID:     s.StatementImportID,
			State:                 importedStatesOut[s.State],
			Line:                  int32(s.Line), //nolint:gosec // a statement file is far smaller
			BookingDate:           s.BookingDate,
			TransactionDate:       s.TransactionDate,
			Amount:                a,
			Description:           s.Description,
		}

		if !s.ValueDate.IsZero() {
			item.ValueDate = oas.NewOptDate(s.ValueDate)
		}

		if s.Payee != "" {
			item.Payee = oas.NewOptString(s.Payee)
		}

		if s.BankTransactionID != "" {
			item.BankTransactionID = oas.NewOptString(s.BankTransactionID)
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
	case errors.Is(err, app.ErrLaterImportsNotSupported):
		return refused(LaterImportsNotSupportedType, "Later Statement imports not supported",
			"Only an Account's first Statement import is supported yet.")
	default:
		return outcome{}, false
	}
}

func fileProblem(typ, title, detail string) *oas.StatementFileProblem {
	return &oas.StatementFileProblem{
		Type: typ, Title: title, Status: http.StatusUnprocessableEntity, Detail: oas.NewOptString(detail),
	}
}

func statementImportResponse(i app.StatementImport) (*oas.StatementImport, error) {
	s := i.State
	out := &oas.StatementImport{
		StatementImportID:          i.ID,
		ConnectionID:               s.ConnectionID,
		AccountID:                  s.AccountID,
		RowsImported:               int32(s.RowsImported), //nolint:gosec // a statement file is far smaller
		RowsIgnoredBeforeStartDate: int32(s.RowsIgnored),  //nolint:gosec // a statement file is far smaller
		BalanceCheck:               "PASSED",
		ImportedAt:                 s.RecordedAt,
	}

	var err error

	if out.ExportBalance, err = money(s.ExportBalance, s.Currency); err != nil {
		return nil, err
	}

	if out.RowsTotal, err = money(s.RowsTotal, s.Currency); err != nil {
		return nil, err
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
