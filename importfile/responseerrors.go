package importfile

import (
	"github.com/Clever/csvlint"
	"strconv"
	"walletaccountant/definitions"
)

const (
	NonExistentImportFileCode definitions.ErrorCode = iota + 100
	UnsupportedFileTypeCode
	InvalidCsvFileCode
	CannotRestartFileParseCode
)

const (
	NonExistentImportFile  definitions.ErrorReason = "Import file does not exist"
	UnsupportedFileType    definitions.ErrorReason = "Unsupported file type"
	InvalidCsvFile         definitions.ErrorReason = "Invalid CSV file"
	CannotRestartFileParse definitions.ErrorReason = "Cannot restart file parse, state must be failed"
)

func NonExistentImportFileError(importFileId string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    NonExistentImportFileCode,
		Reason:  NonExistentImportFile,
		Context: definitions.ErrorContext{"ImportFileId": importFileId},
	}
}

func UnsupportedFileTypeError(fileType string) *definitions.WalletAccountantError {
	return &definitions.WalletAccountantError{
		Code:    UnsupportedFileTypeCode,
		Reason:  UnsupportedFileType,
		Context: definitions.ErrorContext{"FileType": fileType},
	}
}

func InvalidCsvFileError(
	csvErrors []csvlint.CSVError,
	context definitions.ErrorContext,
) *definitions.WalletAccountantError {
	if context == nil {
		context = definitions.ErrorContext{}
	}

	for _, csvError := range csvErrors {
		context[strconv.Itoa(csvError.Num)] = csvError.Error()
	}

	return &definitions.WalletAccountantError{
		Code:    InvalidCsvFileCode,
		Reason:  InvalidCsvFile,
		Context: context,
	}
}

func CannotRestartFileParseError(
	importFileId *Id,
	currentState State,
) *definitions.WalletAccountantError {
	context := definitions.ErrorContext{}
	context["importFileId"] = importFileId.String()
	context["currentParseState"] = currentState

	return &definitions.WalletAccountantError{
		Code:    CannotRestartFileParseCode,
		Reason:  CannotRestartFileParse,
		Context: context,
	}
}
