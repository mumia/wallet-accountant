package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	// Skipped counts the certain repeats WithoutCertainRepeats left out.
	Skipped int
	// Suspected counts the rows WithSuspectedDuplicates marked.
	Suspected int
	Total     sharedkernel.Decimal
}

// ImportedKey is an earlier Imported transaction of the Account as
// certain repeats and Suspected duplicates are told by: its ID, bank
// transaction ID, Booking date, amount and description.
type ImportedKey struct {
	ID                sharedkernel.ID
	BankTransactionID string
	BookingDate       time.Time
	Amount            sharedkernel.Decimal
	Description       string
}

// WithSuspectedDuplicates marks, and counts, every row that only looks
// like an earlier Imported transaction of the Account (spec story 39): one
// with the same bank transaction ID but another Booking date or amount;
// or, for a row without a bank transaction ID, one with the same Booking
// date, amount and normalised description. Each row records the first
// such earlier Imported transaction. Certain repeats must already be left
// out (WithoutCertainRepeats), and rows are compared only with earlier
// imports, never with each other (spec story 40e).
func (s Statement) WithSuspectedDuplicates(earlier []ImportedKey) Statement {
	out := s
	out.Rows, out.Suspected = make([]StatementRow, 0, len(s.Rows)), s.Suspected

	for _, r := range s.Rows {
		i := slices.IndexFunc(earlier, func(k ImportedKey) bool { return resembles(r, k) })
		if i >= 0 {
			r.Resembles = earlier[i].ID
			out.Suspected++
		}

		out.Rows = append(out.Rows, r)
	}

	return out
}

func resembles(r StatementRow, k ImportedKey) bool {
	if r.BankTransactionID != "" {
		return k.BankTransactionID == r.BankTransactionID
	}

	return k.BookingDate.Equal(r.BookingDate) && k.Amount.Equal(r.Amount) &&
		NormaliseDescription(k.Description) == NormaliseDescription(r.Description)
}

// NormaliseDescription is a bank description as Suspected duplicates
// compare it: lower case, with runs of white space made one space and none
// at either end.
func NormaliseDescription(d string) string {
	return strings.Join(strings.Fields(strings.ToLower(d)), " ")
}

// WithoutCertainRepeats leaves out, and counts, every row whose bank
// transaction ID, Booking date and amount all equal those of an earlier
// Imported transaction of the Account (spec story 38), and works the total
// out again. A row without a bank transaction ID is never a certain
// repeat, and rows are compared only with earlier imports, never with each
// other.
func (s Statement) WithoutCertainRepeats(earlier []ImportedKey) Statement {
	out := Statement{Rows: []StatementRow{}, Ignored: s.Ignored, Skipped: s.Skipped}

	for _, r := range s.Rows {
		if r.BankTransactionID != "" && slices.ContainsFunc(earlier, func(k ImportedKey) bool {
			return k.BankTransactionID == r.BankTransactionID && k.BookingDate.Equal(r.BookingDate) && k.Amount.Equal(r.Amount)
		}) {
			out.Skipped++

			continue
		}

		out.Rows = append(out.Rows, r)
		out.Total = out.Total.Add(r.Amount)
	}

	return out
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
