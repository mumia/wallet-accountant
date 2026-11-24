package domain

import (
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrFileBalanceUnknown is returned, as a *FileBalanceUnknownError, for
	// a statement file whose balance column doesn't tell its newest row.
	ErrFileBalanceUnknown = errors.New("the file's balance can't be worked out")
	// ErrExportBalanceDiffersFromFile is returned, as an
	// *ExportBalanceDiffersFromFileError, for an Export balance that isn't
	// the file's balance.
	ErrExportBalanceDiffersFromFile = errors.New("export balance differs from the file's balance")
)

// FileBalance is a statement file's balance: the balance on its newest row,
// and that row's line.
type FileBalance struct {
	Amount sharedkernel.Decimal
	Line   int
}

// FileBalanceUnknownError is why a statement file's balance can't be worked
// out. It is ErrFileBalanceUnknown, and its message is the reason alone.
type FileBalanceUnknownError struct {
	Reason string
}

func (e *FileBalanceUnknownError) Error() string { return e.Reason }

// Is reports whether target is ErrFileBalanceUnknown.
func (*FileBalanceUnknownError) Is(target error) bool { return target == ErrFileBalanceUnknown }

// ExportBalanceDiffersFromFileError is an Export balance, in the Account
// currency, that isn't the file's balance.
type ExportBalanceDiffersFromFileError struct {
	ExportBalance sharedkernel.Decimal
	File          FileBalance
	Currency      string
}

func (e *ExportBalanceDiffersFromFileError) Error() string {
	return fmt.Sprintf("the Export balance %s %s isn't the file's balance %s %s (line %d)",
		e.ExportBalance, e.Currency, e.File.Amount, e.Currency, e.File.Line)
}

// Is reports whether target is ErrExportBalanceDiffersFromFile.
func (*ExportBalanceDiffersFromFileError) Is(target error) bool {
	return target == ErrExportBalanceDiffersFromFile
}

// FileBalanceOf is the balance on the newest of a statement file's rows,
// which are in file order and all carry a balance. The newest row is told
// by the balances alone, not by dates: the rows are oldest first when the
// last row's balance is the first row's plus every later row's amount, and
// newest first when the first row's balance is the last row's plus every
// earlier row's amount. A file of one row has that row's balance. A file
// whose balances add up neither way, or both ways to different balances,
// is a *FileBalanceUnknownError.
func FileBalanceOf(rows []StatementRow) (FileBalance, error) {
	if len(rows) == 0 {
		return FileBalance{}, &FileBalanceUnknownError{Reason: "the file has no rows"}
	}

	first, last := rows[0], rows[len(rows)-1]
	if len(rows) == 1 {
		return FileBalance{Amount: first.Balance, Line: first.Line}, nil
	}

	var afterFirst, beforeLast sharedkernel.Decimal

	for i, r := range rows {
		if i > 0 {
			afterFirst = afterFirst.Add(r.Amount)
		}

		if i < len(rows)-1 {
			beforeLast = beforeLast.Add(r.Amount)
		}
	}

	oldestFirst := last.Balance.Equal(first.Balance.Add(afterFirst))
	newestFirst := first.Balance.Equal(last.Balance.Add(beforeLast))

	switch {
	case oldestFirst && newestFirst && !first.Balance.Equal(last.Balance):
		return FileBalance{}, &FileBalanceUnknownError{
			Reason: "the balance column adds up from both ends of the file, so its newest row can't be told",
		}
	case oldestFirst:
		return FileBalance{Amount: last.Balance, Line: last.Line}, nil
	case newestFirst:
		return FileBalance{Amount: first.Balance, Line: first.Line}, nil
	default:
		return FileBalance{}, &FileBalanceUnknownError{
			Reason: "the balance column doesn't add up from one end of the file to the other",
		}
	}
}
