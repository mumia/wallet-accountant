package importfilecommand

import (
	"context"
	"errors"
	"fmt"
	"github.com/Clever/csvlint"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/importfile"
	"walletaccountant/importfile/bankfileparser"
	"walletaccountant/importfilereadmodel"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorer interface {
	RegisterNewImportFile(
		ctx *gin.Context,
		transferObject RegisterNewImportFileTransferObject,
	) (*importfile.Id, *definitions.WalletAccountantError)
	StartFileParse(ctx context.Context, importFileId *importfile.Id) *definitions.WalletAccountantError
	RestartFileParse(ctx context.Context, importFileId *importfile.Id) *definitions.WalletAccountantError
	EndFileParse(ctx context.Context, importFileId *importfile.Id) *definitions.WalletAccountantError
	FailFileParse(
		ctx context.Context,
		importFileId *importfile.Id,
		code string,
		reason string,
	) *definitions.WalletAccountantError
	AddFileDataRow(
		ctx context.Context,
		importFileId *importfile.Id,
		dataRow *bankfileparser.BankFileDataRow,
	) *definitions.WalletAccountantError
	VerifyFileDataRow(
		ctx context.Context,
		transferObject VerifyFileDataRowTransferObject,
	) *definitions.WalletAccountantError
	InvalidateFileDataRow(
		ctx context.Context,
		transferObject InvalidateFileDataRowTransferObject,
	) *definitions.WalletAccountantError
}

type CommandMediator struct {
	commandHandler    eventhorizon.CommandHandler
	repository        importfilereadmodel.ReadModeler
	accountRepository accountreadmodel.ReadModeler
	idCreator         eventstoredb.IdGenerator
	fileHandler       common.Filer
	fileUploadPath    string
	parsers           map[string]bankfileparser.BankFileParser
	logger            *zap.Logger
}

func NewCommandMediator(
	parsers []bankfileparser.BankFileParser,
	commandHandler eventhorizon.CommandHandler,
	repository importfilereadmodel.ReadModeler,
	accountRepository accountreadmodel.ReadModeler,
	idCreator eventstoredb.IdGenerator,
	fileHandler common.Filer,
	logger *zap.Logger,
) *CommandMediator {
	fileUploadPath := os.Getenv("FILE_UPLOAD_PATH")

	var bankParsers = make(map[string]bankfileparser.BankFileParser)
	for _, bankFileParser := range parsers {
		bankParsers[bankFileParser.Id()] = bankFileParser
	}

	return &CommandMediator{
		commandHandler:    commandHandler,
		repository:        repository,
		accountRepository: accountRepository,
		idCreator:         idCreator,
		fileHandler:       fileHandler,
		fileUploadPath:    fileUploadPath,
		parsers:           bankParsers,
		logger:            logger,
	}
}

func (mediator CommandMediator) RegisterNewImportFile(
	ctx *gin.Context,
	transferObject RegisterNewImportFileTransferObject,
) (*importfile.Id, *definitions.WalletAccountantError) {
	accountId := account.IdFromUUIDString(transferObject.AccountId)

	existingAccount, err := mediator.accountRepository.GetByAccountId(ctx, accountId)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, definitions.GenericError(err, nil)
	}

	if existingAccount == nil {
		return nil, account.NonExistentAccountError(accountId.String())
	}

	command, err := eventhorizon.CreateCommand(importfile.RegisterNewImportFileCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	registerNewImportFileCommand, ok := command.(*importfile.RegisterNewImportFile)
	if !ok {
		return nil, definitions.InvalidCommandError(importfile.RegisterNewImportFileCommand, command.CommandType())
	}

	fullUploadPath := filepath.Join(mediator.fileUploadPath, transferObject.Filename)
	fileType, err := mediator.determineFileTypeCleanAndValidate(existingAccount.BankName, fullUploadPath)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	registerNewImportFileCommand.ImportFileId = *importfile.IdFromUUID(mediator.idCreator.New())
	registerNewImportFileCommand.AccountId = *accountId
	registerNewImportFileCommand.Filename = transferObject.Filename
	registerNewImportFileCommand.FileType = fileType

	err = mediator.commandHandler.HandleCommand(ctx, registerNewImportFileCommand)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return &registerNewImportFileCommand.ImportFileId, nil
}

func (mediator CommandMediator) StartFileParse(
	ctx context.Context,
	importFileId *importfile.Id,
) *definitions.WalletAccountantError {
	existingImportFile, err := mediator.repository.GetById(ctx, importFileId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return definitions.GenericError(err, nil)
	}

	if existingImportFile == nil {
		return importfile.NonExistentImportFileError(importFileId.String())
	}

	command, err := eventhorizon.CreateCommand(importfile.StartFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	startFileParseCommand, ok := command.(*importfile.StartFileParse)
	if !ok {
		return definitions.InvalidCommandError(importfile.StartFileParseCommand, command.CommandType())
	}

	startFileParseCommand.ImportFileId = *importFileId

	err = mediator.commandHandler.HandleCommand(ctx, startFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	return nil
}

func (mediator CommandMediator) RestartFileParse(
	ctx context.Context,
	importFileId *importfile.Id,
) *definitions.WalletAccountantError {
	existingImportFile, err := mediator.repository.GetById(ctx, importFileId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return definitions.GenericError(err, nil)
	}

	if existingImportFile == nil {
		return importfile.NonExistentImportFileError(importFileId.String())
	}

	if existingImportFile.State != importfile.ParsingFailed {
		return importfile.CannotRestartFileParseError(importFileId, existingImportFile.State)
	}

	command, err := eventhorizon.CreateCommand(importfile.RestartFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	restartFileParseCommand, ok := command.(*importfile.RestartFileParse)
	if !ok {
		return definitions.InvalidCommandError(importfile.RestartFileParseCommand, command.CommandType())
	}

	restartFileParseCommand.ImportFileId = *importFileId

	err = mediator.commandHandler.HandleCommand(ctx, restartFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	return nil
}

func (mediator CommandMediator) EndFileParse(
	ctx context.Context,
	importFileId *importfile.Id,
) *definitions.WalletAccountantError {
	existingImportFile, err := mediator.repository.GetById(ctx, importFileId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return definitions.GenericError(err, nil)
	}

	if existingImportFile == nil {
		return importfile.NonExistentImportFileError(importFileId.String())
	}

	command, err := eventhorizon.CreateCommand(importfile.EndFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	endFileParseCommand, ok := command.(*importfile.EndFileParse)
	if !ok {
		return definitions.InvalidCommandError(importfile.EndFileParseCommand, command.CommandType())
	}

	endFileParseCommand.ImportFileId = *importFileId

	err = mediator.commandHandler.HandleCommand(ctx, endFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	return nil
}

func (mediator CommandMediator) FailFileParse(
	ctx context.Context,
	importFileId *importfile.Id,
	code string,
	reason string,
) *definitions.WalletAccountantError {
	existingImportFile, err := mediator.repository.GetById(ctx, importFileId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return definitions.GenericError(err, nil)
	}

	if existingImportFile == nil {
		return importfile.NonExistentImportFileError(importFileId.String())
	}

	command, err := eventhorizon.CreateCommand(importfile.FailFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	failFileParseCommand, ok := command.(*importfile.FailFileParse)
	if !ok {
		return definitions.InvalidCommandError(importfile.FailFileParseCommand, command.CommandType())
	}

	failFileParseCommand.ImportFileId = *importFileId
	failFileParseCommand.Code = code
	failFileParseCommand.Reason = reason

	err = mediator.commandHandler.HandleCommand(ctx, failFileParseCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": existingImportFile.ImportFileId.String(),
				"accountId":    existingImportFile.AccountId.String(),
			},
		)
	}

	return nil
}

func (mediator CommandMediator) AddFileDataRow(
	ctx context.Context,
	importFileId *importfile.Id,
	dataRow *bankfileparser.BankFileDataRow,
) *definitions.WalletAccountantError {
	command, err := eventhorizon.CreateCommand(importfile.AddFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": importFileId.String(),
				"accountId":    dataRow.AccountId.String(),
			},
		)
	}

	addFileDataRowCommand, ok := command.(*importfile.AddFileDataRow)
	if !ok {
		return definitions.InvalidCommandError(importfile.AddFileDataRowCommand, command.CommandType())
	}

	addFileDataRowCommand.ImportFileId = *importFileId
	addFileDataRowCommand.FileDataRowId = *dataRow.DataRowId
	addFileDataRowCommand.Date = dataRow.Date
	addFileDataRowCommand.Description = dataRow.Description
	addFileDataRowCommand.Amount = dataRow.Amount
	addFileDataRowCommand.RawData = dataRow.RawData

	err = mediator.commandHandler.HandleCommand(ctx, addFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId": importFileId.String(),
				"accountId":    dataRow.AccountId.String(),
			},
		)
	}

	return nil
}

func (mediator CommandMediator) VerifyFileDataRow(
	ctx context.Context,
	transferObject VerifyFileDataRowTransferObject,
) *definitions.WalletAccountantError {
	command, err := eventhorizon.CreateCommand(importfile.VerifyFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId":  transferObject.ImportFileId,
				"fileDataRowId": transferObject.FileDataRowId,
			},
		)
	}

	verifyFileDataRowCommand, ok := command.(*importfile.VerifyFileDataRow)
	if !ok {
		return definitions.InvalidCommandError(importfile.VerifyFileDataRowCommand, command.CommandType())
	}

	importFileId := importfile.IdFromUUIDString(transferObject.ImportFileId)
	fileDataRowId := importfile.DataRowIdFromUUIDString(transferObject.FileDataRowId)

	verifyFileDataRowCommand.ImportFileId = *importFileId
	verifyFileDataRowCommand.FileDataRowId = *fileDataRowId
	if transferObject.MovementTypeId != nil {
		verifyFileDataRowCommand.MovementTypeId = movementtype.IdFromUUIDString(*transferObject.MovementTypeId)
	}
	if transferObject.SourceAccountId != nil {
		verifyFileDataRowCommand.SourceAccountId = account.IdFromUUIDString(*transferObject.SourceAccountId)
	}
	verifyFileDataRowCommand.Description = transferObject.Description
	verifyFileDataRowCommand.TagIds = tagcategory.TagIdsFromUUIDStrings(transferObject.TagIds)

	err = mediator.commandHandler.HandleCommand(ctx, verifyFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId":  transferObject.ImportFileId,
				"fileDataRowId": transferObject.FileDataRowId,
			},
		)
	}

	return nil
}

func (mediator CommandMediator) InvalidateFileDataRow(
	ctx context.Context,
	transferObject InvalidateFileDataRowTransferObject,
) *definitions.WalletAccountantError {
	command, err := eventhorizon.CreateCommand(importfile.InvalidateFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId":  transferObject.ImportFileId,
				"fileDataRowId": transferObject.FileDataRowId,
			},
		)
	}

	invalidateFileDataRowCommand, ok := command.(*importfile.InvalidateFileDataRow)
	if !ok {
		return definitions.InvalidCommandError(importfile.InvalidateFileDataRowCommand, command.CommandType())
	}

	importFileId := importfile.IdFromUUIDString(transferObject.ImportFileId)
	fileDataRowId := importfile.DataRowIdFromUUIDString(transferObject.FileDataRowId)

	invalidateFileDataRowCommand.ImportFileId = *importFileId
	invalidateFileDataRowCommand.FileDataRowId = *fileDataRowId
	invalidateFileDataRowCommand.Reason = transferObject.Reason

	err = mediator.commandHandler.HandleCommand(ctx, invalidateFileDataRowCommand)
	if err != nil {
		return definitions.GenericError(
			err,
			definitions.ErrorContext{
				"importFileId":  transferObject.ImportFileId,
				"fileDataRowId": transferObject.FileDataRowId,
			},
		)
	}

	return nil
}

func (mediator CommandMediator) determineFileTypeCleanAndValidate(
	bankName account.BankName,
	fullUploadPath string,
) (importfile.FileType, error) {
	file, err := mediator.fileHandler.Open(fullUploadPath)
	if err != nil {
		return "", mediator.handleGenericFileError(fullUploadPath, err)
	}

	defer func(fileReader io.ReadCloser) {
		err := fileReader.Close()
		if err != nil {
			mediator.logger.Warn(
				"failed to close file",
				zap.String("filepath", fullUploadPath),
				zap.Error(err),
			)
		}
	}(file)

	//mimeFileType, err := mimetype.DetectReader(file)
	//if err != nil {
	//	return "", mediator.handleGenericFileError(fullUploadPath, err)
	//}

	//_, err = file.Seek(0, io.SeekStart)
	//if err != nil {
	//	return "", err
	//}

	//var fileType importfile.FileType
	//switch mimeFileType.String() {
	//case "text/csv":
	//	fileType = importfile.CSV
	//default:
	//	return "", importfile.UnsupportedFileTypeError(mimeFileType.String())
	//}

	// hard coding CSV type for now, will change if we get different types
	fileType := importfile.CSV

	parser := mediator.parsers[bankfileparser.ParserId(bankName, fileType)]

	content, typeErrors, err := parser.CleanAndValidate(file)
	if err != nil {
		return "", mediator.handleGenericFileError(fullUploadPath, err)
	}

	if typeErrors != nil {
		return "", mediator.handleTypeErrors(fullUploadPath, fileType, typeErrors)
	}

	// save file after cleanup
	err = mediator.fileHandler.Save(fullUploadPath, content)
	if err != nil {
		return "", mediator.handleGenericFileError(fullUploadPath, err)
	}

	return fileType, nil
}

func (mediator CommandMediator) removeFile(fullUploadPath string) definitions.ErrorContext {
	err := os.Remove(fullUploadPath)
	var ctx definitions.ErrorContext = nil
	if err != nil {
		ctx = definitions.ErrorContext{"fileRemovalError": err.Error()}
	}

	return ctx
}

func (mediator CommandMediator) handleGenericFileError(
	fullUploadPath string,
	err error,
) *definitions.WalletAccountantError {
	ctx := mediator.removeFile(fullUploadPath)

	return definitions.GenericError(err, ctx)
}

func (mediator CommandMediator) handleTypeErrors(
	fullUploadPath string,
	fileType importfile.FileType,
	typeErrors interface{},
) *definitions.WalletAccountantError {
	ctx := mediator.removeFile(fullUploadPath)

	switch fileType {
	case importfile.CSV:
		csvErrors, ok := typeErrors.([]csvlint.CSVError)
		if !ok {
			return mediator.errorForFileType(
				"CSV error type failed conversion",
				fileType,
				fmt.Sprintf("%T", typeErrors),
			)

		}

		return importfile.InvalidCsvFileError(csvErrors, ctx)

	default:
		return mediator.errorForFileType("error for unknown file type", fileType, typeErrors)
	}
}

func (mediator CommandMediator) errorForFileType(
	message string,
	fileType importfile.FileType,
	typeErrors any,
) *definitions.WalletAccountantError {
	return definitions.GenericError(
		errors.New(message),
		definitions.ErrorContext{
			"fileType":  fileType,
			"errorType": fmt.Sprintf("%T", typeErrors),
		},
	)
}
