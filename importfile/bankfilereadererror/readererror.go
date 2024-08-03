package bankfilereadererror

import (
	"fmt"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

type Code string

const (
	FileAccountNotFound  Code = "file_account_not_found"
	FileOpenFailed            = "file_open_failed"
	FileDataRowAddFailed      = "file_data_row_add_failed"
	ParserFailed              = "parser_failed"
)

var _ error = &ReaderError{}

type ReaderError struct {
	Code   Code
	Reason error
}

func FileAccountNotFoundError(importFileId *importfile.Id, accountId *account.Id, err error) *ReaderError {
	return &ReaderError{
		Code: FileAccountNotFound,
		Reason: fmt.Errorf(
			"file account was not found. ImportFileId: %s AccountId: %s Error: %w",
			importFileId,
			accountId,
			err,
		),
	}
}

func FileOpenError(importFileId *importfile.Id, accountId *account.Id, err error) *ReaderError {
	return &ReaderError{
		Code: FileOpenFailed,
		Reason: fmt.Errorf(
			"failed to open file. ImportFileId: %s AccountId: %s Error: %w",
			importFileId,
			accountId,
			err,
		),
	}
}

func FileDataRowAddFailedError(
	importFileId *importfile.Id,
	bankFileDataRow any,
	err error,
) *ReaderError {
	return &ReaderError{
		Code: FileDataRowAddFailed,
		Reason: fmt.Errorf(
			"failed to add file data row. ImportFileId: %s Row: %+v Error: %w",
			importFileId,
			bankFileDataRow,
			err,
		),
	}
}

func ParserFailedError(
	importFileId *importfile.Id,
	accountId *account.Id,
	err error,
) *ReaderError {
	return &ReaderError{
		Code: ParserFailed,
		Reason: fmt.Errorf(
			"parser failed. ImportFileId: %s AccountId: %s Error: %w",
			importFileId,
			accountId,
			err,
		),
	}
}

func (r ReaderError) Error() string {
	return fmt.Sprintf("%s - %s", r.Code, r.Reason.Error())
}
