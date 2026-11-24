package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
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
	// InClosedMonths are the rows WithoutClosedMonths split off: booked in
	// a month before the Account's Open month, they are ignored (ADR-0024).
	InClosedMonths []StatementRow
	// Skipped counts the certain repeats WithoutCertainRepeats left out.
	Skipped int
	// Suspected counts the rows WithSuspectedDuplicates marked.
	Suspected int
	Total     sharedkernel.Decimal

	// The file's balance, worked out from every readable row before any is
	// ignored, when the mapping has a balance column: hasFileBalance, then
	// fileBalance or why it can't be worked out. currency is the Account
	// currency's code.
	hasFileBalance bool
	fileBalance    FileBalance
	fileBalanceErr error
	currency       string
}

// HasFileBalance reports whether the file was read with a balance column,
// so its Export balance is checked against the file's balance.
func (s Statement) HasFileBalance() bool { return s.hasFileBalance }

// FileBalance is the file's balance (FileBalanceOf), counting every
// readable row, those the import ignores included: the check is about the
// file. It is a *FileBalanceUnknownError when it can't be worked out or the
// mapping has no balance column.
func (s Statement) FileBalance() (FileBalance, error) {
	if !s.hasFileBalance {
		return FileBalance{}, &FileBalanceUnknownError{Reason: "the CSV mapping has no balance column"}
	}

	return s.fileBalance, s.fileBalanceErr
}

// CheckExportBalanceAgainstFile refuses, for a file read with a balance
// column, an Export balance that isn't the file's balance (an
// *ExportBalanceDiffersFromFileError) and a file whose balance can't be
// worked out (a *FileBalanceUnknownError). Without a balance column it
// checks nothing.
func (s Statement) CheckExportBalanceAgainstFile(exportBalance sharedkernel.Decimal) error {
	if !s.hasFileBalance {
		return nil
	}

	if s.fileBalanceErr != nil {
		return s.fileBalanceErr
	}

	if !exportBalance.Equal(s.fileBalance.Amount) {
		return &ExportBalanceDiffersFromFileError{ExportBalance: exportBalance, File: s.fileBalance, Currency: s.currency}
	}

	return nil
}

// WithoutClosedMonths splits off every row booked in a month before the
// Account's Open month (YYYY-MM) into InClosedMonths, and works the total
// out again. It runs before certain repeats and Suspected duplicates are
// told, so a closed month's rows never become either (ADR-0024).
func (s Statement) WithoutClosedMonths(openMonth string) Statement {
	out := s
	out.Rows, out.InClosedMonths, out.Total = []StatementRow{}, slices.Clone(s.InClosedMonths), sharedkernel.Decimal{}

	for _, r := range s.Rows {
		if r.BookingDate.Format(MonthLayout) < openMonth {
			out.InClosedMonths = append(out.InClosedMonths, r)

			continue
		}

		out.Rows = append(out.Rows, r)
		out.Total = out.Total.Add(r.Amount)
	}

	return out
}

// EarliestBookingMonth is the earliest Booking month (YYYY-MM) of the
// rows; empty when there are none.
func (s Statement) EarliestBookingMonth() string {
	earliest := ""

	for _, r := range s.Rows {
		if m := r.BookingDate.Format(MonthLayout); earliest == "" || m < earliest {
			earliest = m
		}
	}

	return earliest
}

// MonthRows counts a statement file's rows in one Booking month (YYYY-MM).
type MonthRows struct {
	Month string
	Rows  int
}

// ClosedMonthsNotInBooks counts, per closed month, the rows split off by
// WithoutClosedMonths that are neither a certain repeat nor a lookalike of
// an earlier Imported transaction of the Account: the bank has them, the
// books don't, and only reopening the month lets them in (spec story 13).
// Months come in order.
func (s Statement) ClosedMonthsNotInBooks(earlier []ImportedKey) []MonthRows {
	idx := indexEarlier(earlier)
	counts := map[string]int{}

	for _, r := range s.InClosedMonths {
		if _, known := idx.resembled(r); !known {
			counts[r.BookingDate.Format(MonthLayout)]++
		}
	}

	out := make([]MonthRows, 0, len(counts))
	for _, m := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, MonthRows{Month: m, Rows: counts[m]})
	}

	return out
}

// MonthLayout is how a Booking month is written.
const MonthLayout = "2006-01"

// earlierIndex is the Account's earlier Imported transactions indexed once,
// keeping the first of each key, so each row is told in constant time.
type earlierIndex struct {
	byBankID map[string]sharedkernel.ID
	byLook   map[lookKey]sharedkernel.ID
}

func indexEarlier(earlier []ImportedKey) earlierIndex {
	idx := earlierIndex{
		byBankID: make(map[string]sharedkernel.ID, len(earlier)),
		byLook:   make(map[lookKey]sharedkernel.ID, len(earlier)),
	}

	for _, k := range earlier {
		if k.BankTransactionID != "" {
			if _, ok := idx.byBankID[k.BankTransactionID]; !ok {
				idx.byBankID[k.BankTransactionID] = k.ID
			}
		}

		look := lookKeyOf(k.BookingDate, k.Amount, k.Description)
		if _, ok := idx.byLook[look]; !ok {
			idx.byLook[look] = k.ID
		}
	}

	return idx
}

// resembled is the earlier Imported transaction the row is a certain repeat
// or a lookalike of: the same bank transaction ID or, for a row without
// one, the same Booking date, amount and normalised description.
func (idx earlierIndex) resembled(r StatementRow) (sharedkernel.ID, bool) {
	if r.BankTransactionID != "" {
		id, ok := idx.byBankID[r.BankTransactionID]

		return id, ok
	}

	id, ok := idx.byLook[lookKeyOf(r.BookingDate, r.Amount, r.Description)]

	return id, ok
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

	idx := indexEarlier(earlier)

	for _, r := range s.Rows {
		if id, found := idx.resembled(r); found {
			r.Resembles = id
			out.Suspected++
		}

		out.Rows = append(out.Rows, r)
	}

	return out
}

// dayKey is a Booking date as a map key: two dates are the same key exactly
// when they are Equal.
type dayKey struct {
	sec  int64
	nsec int
}

func dayKeyOf(d time.Time) dayKey { return dayKey{sec: d.Unix(), nsec: d.Nanosecond()} }

// lookKey is what a row without a bank transaction ID looks like: its
// Booking date, amount (by value, so 10.5 and 10.50 agree) and normalised
// description.
type lookKey struct {
	day         dayKey
	amount      string
	description string
}

func lookKeyOf(d time.Time, amount sharedkernel.Decimal, description string) lookKey {
	return lookKey{day: dayKeyOf(d), amount: amount.String(), description: NormaliseDescription(description)}
}

// repeatKey is what a certain repeat shares with an earlier Imported
// transaction: its bank transaction ID, Booking date and amount (by value).
type repeatKey struct {
	bankTransactionID string
	day               dayKey
	amount            string
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
	out := s
	out.Rows, out.Total = []StatementRow{}, sharedkernel.Decimal{}

	repeats := make(map[repeatKey]bool, len(earlier))
	for _, k := range earlier {
		if k.BankTransactionID != "" {
			repeats[repeatKey{k.BankTransactionID, dayKeyOf(k.BookingDate), k.Amount.String()}] = true
		}
	}

	for _, r := range s.Rows {
		if r.BankTransactionID != "" && repeats[repeatKey{r.BankTransactionID, dayKeyOf(r.BookingDate), r.Amount.String()}] {
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
// booked before the start date are ignored and counted (spec story 41), but
// still count for the file's balance when the mapping has a balance column.
// A file with no rows at all is ErrEmptyStatementFile.
func ReadStatement(m CSVMapping, file []byte, currency sharedkernel.Currency, startDate time.Time) (Statement, error) {
	rows, err := parseStatementFile(m, file, int32(currency.Decimals())) //nolint:gosec // at most 18 places
	if err != nil {
		return Statement{}, err
	}

	if len(rows) == 0 {
		return Statement{}, ErrEmptyStatementFile
	}

	s := Statement{Rows: []StatementRow{}, currency: currency.Code()}

	if m.HasBalance() {
		s.hasFileBalance = true
		s.fileBalance, s.fileBalanceErr = FileBalanceOf(rows)
	}

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
