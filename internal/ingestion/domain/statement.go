package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrEmptyStatementFile is returned for a statement file with no rows at
	// all. There is nothing to import, and it is far more likely a wrong
	// export than a real statement, so it is refused rather than recorded
	// (and its hash never blocks the right file).
	ErrEmptyStatementFile = errors.New("statement file has no rows")
	// ErrInvalidExportBalance is returned for an Export balance with more
	// decimal places than the Account currency has.
	ErrInvalidExportBalance = errors.New("invalid export balance")
)

// Statement is a statement file read for an Account: the rows booked on or
// after its start date, how many rows before it were ignored, and the total
// of the rows kept.
type Statement struct {
	Rows    []StatementRow
	Ignored int
	Total   sharedkernel.Decimal
}

// ReadStatement reads a statement file with the mapping for an Account in
// the currency whose history starts on startDate. Every row must be
// readable, with no more decimal places than the currency has; otherwise
// the whole file is refused with a *RowsError listing every bad line. Rows
// booked before the start date are ignored and counted (spec story 41). A
// file with no rows at all is ErrEmptyStatementFile.
func ReadStatement(m CSVMapping, file []byte, currency sharedkernel.Currency, startDate time.Time) (Statement, error) {
	rows, err := parseStatementFile(m, file, int32(currency.Decimals())) //nolint:gosec // at most 18 places
	if err != nil {
		return Statement{}, err
	}

	if len(rows) == 0 {
		return Statement{}, ErrEmptyStatementFile
	}

	s := Statement{Rows: []StatementRow{}}

	for _, r := range rows {
		if r.BookingDate.Before(startDate) {
			s.Ignored++

			continue
		}

		s.Rows = append(s.Rows, r)
		s.Total = s.Total.Add(r.Amount)
	}

	return s, nil
}

// OpeningBalance is the Opening balance an Account's first Statement import
// records: the Export balance minus the total of the rows on or after the
// start date (ADR-0018).
func (s Statement) OpeningBalance(exportBalance sharedkernel.Decimal) sharedkernel.Decimal {
	return exportBalance.Sub(s.Total)
}

// CheckExportBalance refuses an Export balance with more decimal places
// than the Account currency has.
func CheckExportBalance(exportBalance sharedkernel.Decimal, currency sharedkernel.Currency) error {
	if !exportBalance.Equal(exportBalance.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return fmt.Errorf("%w: %s has more than the %d decimal places of %s",
			ErrInvalidExportBalance, exportBalance, currency.Decimals(), currency.Code())
	}

	return nil
}

// FileHash identifies a statement file by its content: a Statement import is
// deduplicated by it within its Connection.
func FileHash(file []byte) string {
	sum := sha256.Sum256(file)

	return hex.EncodeToString(sum[:])
}

// PreviewID ties the confirm of an Account's first Statement import to its
// preview without the server keeping any state: a hash of the Account, the
// Connection, the file, the Export balance (by value, so 100.5 and 100.50
// agree) and the Connection's mapping version. A confirm whose file, Export
// balance or mapping differs from the preview gets another ID.
func PreviewID(account, connection sharedkernel.ID, fileHash string, exportBalance sharedkernel.Decimal, mappingVersion int) string {
	h := sha256.New()

	for _, part := range []string{
		"statement-import-preview/v1", account.String(), connection.String(), fileHash,
		exportBalance.String(), strconv.Itoa(mappingVersion),
	} {
		_, _ = fmt.Fprintf(h, "%d:%s", len(part), part)
	}

	return hex.EncodeToString(h.Sum(nil))
}
