package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
)

// TestCsvMapping implements testCsvMapping: it reads a sample file with an
// unsaved mapping and records nothing.
func (h *ConnectionHandler) TestCsvMapping(ctx context.Context, req *oas.CsvMappingTestUploadMultipart,
	params oas.TestCsvMappingParams,
) (oas.TestCsvMappingRes, error) {
	invalid := func(problemType, title, detail string) (oas.TestCsvMappingRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, problemType, title, detail)

		return new(oas.TestCsvMappingUnprocessableEntity(p)), nil
	}

	file, refused := statementFile(req.File)
	if refused != nil {
		return invalid(refused.Type, refused.Title, refused.Detail.Value)
	}

	sample, err := h.connections.TestMapping(ctx, params.HouseholdID, params.AccountID, mappingIn(req.CsvMapping), file)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.TestCsvMappingNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrInvalidCSVMapping):
		return invalid(InvalidCSVMappingType, "Invalid CSV mapping", mappingDetail(err))
	case errors.Is(err, domain.ErrEmptyStatementFile):
		return invalid(EmptyStatementFileType, "Empty statement file", "The statement file has no rows.")
	case err != nil:
		return nil, fmt.Errorf("test CSV mapping: %w", err)
	}

	out := &oas.CsvMappingTest{
		Rows:             make([]oas.ParsedStatementRow, 0, len(sample.Rows)),
		ReadableRowCount: int32(sample.Readable), //nolint:gosec // a statement file is far smaller
		UnreadableRows:   make([]oas.RowError, 0, len(sample.Unreadable)),
	}

	for _, r := range sample.Rows {
		amount, err := money(r.Amount, sample.Currency)
		if err != nil {
			return nil, err
		}

		row := oas.ParsedStatementRow{
			Line:            int32(r.Line), //nolint:gosec // a statement file is far smaller
			BookingDate:     r.BookingDate,
			TransactionDate: r.TransactionDate,
			Description:     r.Description,
			Amount:          amount,
		}

		if !r.ValueDate.IsZero() {
			row.ValueDate = oas.NewOptDate(r.ValueDate)
		}

		if r.Payee != "" {
			row.Payee = oas.NewOptString(r.Payee)
		}

		if r.BankTransactionID != "" {
			row.BankTransactionID = oas.NewOptString(r.BankTransactionID)
		}

		out.Rows = append(out.Rows, row)
	}

	for _, r := range sample.Unreadable {
		out.UnreadableRows = append(out.UnreadableRows, oas.RowError{Line: int32(r.Line), Reason: r.Reason}) //nolint:gosec // a statement file is far smaller
	}

	return out, nil
}
