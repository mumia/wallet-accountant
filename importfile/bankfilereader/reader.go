package bankfilereader

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"time"
	"walletaccountant/accountreadmodel"
	"walletaccountant/importfile"
	"walletaccountant/importfile/bankfileparser"
	"walletaccountant/importfile/bankfilereadererror"
	"walletaccountant/importfilecommand"
	"walletaccountant/importfileprojection"
	"walletaccountant/importfilereadmodel"
)

type BankFileReader struct {
	parsers           map[string]bankfileparser.BankFileParser
	accountRepository accountreadmodel.ReadModeler
	mediator          importfilecommand.CommandMediatorer
	readModel         importfilereadmodel.ReadModeler
	fileToParse       *importfileprojection.FileToParseNotifier
	logger            *zap.Logger
	fileUploadPath    string
	noFilesFound      bool
}

func StartBankFileReader(
	parsers []bankfileparser.BankFileParser,
	accountRepository accountreadmodel.ReadModeler,
	mediator importfilecommand.CommandMediatorer,
	readModel importfilereadmodel.ReadModeler,
	fileToParse *importfileprojection.FileToParseNotifier,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) {
	fileUploadPath := os.Getenv("FILE_UPLOAD_PATH")

	var bankParser = make(map[string]bankfileparser.BankFileParser)
	for _, bankFileParser := range parsers {
		bankParser[bankFileParser.Id()] = bankFileParser
	}

	logger = logger.With(zap.String("tag", "bfr"))

	bankFileReader := BankFileReader{
		parsers:           bankParser,
		accountRepository: accountRepository,
		mediator:          mediator,
		readModel:         readModel,
		fileToParse:       fileToParse,
		logger:            logger,
		fileUploadPath:    fileUploadPath,
		noFilesFound:      false,
	}

	var lifecycleCtx context.Context
	var lifecycleCtxCancel context.CancelFunc
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				lifecycleCtx, lifecycleCtxCancel = context.WithCancel(context.Background())

				go bankFileReader.fileReadScheduler(lifecycleCtx)

				return nil
			},
			OnStop: func(ctx context.Context) error {
				lifecycleCtxCancel()

				return nil
			},
		},
	)
}

func (reader *BankFileReader) fileReadScheduler(ctx context.Context) {
	reader.logger.Debug(
		"bank file reader started",
		zap.String("fileUploadPath", reader.fileUploadPath),
	)

	executeFileRead := func() {
		err := reader.findAndReadFiles(ctx)
		if err != nil {
			reader.logger.Error("bank file read error", zap.Error(err))
		}
	}

	executeFileRead()

	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			keepRunning = false
		case <-reader.fileToParse.Channel():
			executeFileRead()
		case <-time.After(1 * time.Hour):
			executeFileRead()
		}
	}

	reader.logger.Debug("bank file reader stopped")
}

func (reader *BankFileReader) findAndReadFiles(ctx context.Context) error {
	for {
		fileEntity, err := reader.readModel.GetAndLockNextFileToParse(ctx)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				if !reader.noFilesFound {
					reader.logger.Debug("no bank files found")
				}
				reader.noFilesFound = true

				break
			}

			return err
		}

		// Continue parsing file until no file is available
		if fileEntity == nil {
			break
		}

		reader.noFilesFound = false

		reader.logger.Debug(
			"found file",
			zap.String("fileImportId", fileEntity.ImportFileId.String()),
		)

		if fileEntity.StartParseDate == nil {
			reader.logger.Debug(
				"file parse start",
				zap.String("fileImportId", fileEntity.ImportFileId.String()),
			)

			waErr := reader.mediator.StartFileParse(ctx, fileEntity.ImportFileId)
			if waErr != nil {
				return waErr
			}
		}

		readerError := reader.readFile(ctx, fileEntity)
		if readerError != nil {
			return reader.handleFailureOnReaderError(ctx, fileEntity.ImportFileId, readerError)
		}

		reader.logger.Debug(
			"file parse end",
			zap.String("fileImportId", fileEntity.ImportFileId.String()),
		)
		waErr := reader.mediator.EndFileParse(ctx, fileEntity.ImportFileId)
		if waErr != nil {
			return waErr
		}
	}

	return nil
}

func (reader *BankFileReader) handleFailureOnReaderError(
	ctx context.Context,
	importFileId *importfile.Id,
	readerError *bankfilereadererror.ReaderError,
) error {
	reader.logger.Debug(
		"file parse failed",
		zap.String("fileImportId", importFileId.String()),
		zap.String("code", string(readerError.Code)),
		zap.String("reason", readerError.Reason.Error()),
	)

	err := reader.mediator.FailFileParse(
		ctx,
		importFileId,
		string(readerError.Code),
		readerError.Reason.Error(),
	)
	if err != nil {
		return fmt.Errorf(
			"fail to register parse failure for %s - %s Error: %w",
			readerError.Code,
			readerError.Reason,
			err,
		)
	}

	return readerError
}

func (reader *BankFileReader) readFile(
	ctx context.Context,
	fileEntity *importfilereadmodel.Entity,
) *bankfilereadererror.ReaderError {
	accountEntity, err := reader.accountRepository.GetByAccountId(ctx, fileEntity.AccountId)
	if err != nil {
		return bankfilereadererror.FileAccountNotFoundError(fileEntity.ImportFileId, fileEntity.AccountId, err)
	}

	fullUploadPath := filepath.Join(reader.fileUploadPath, fileEntity.Filename)
	file, err := os.Open(fullUploadPath)
	if err != nil {
		return bankfilereadererror.FileOpenError(fileEntity.ImportFileId, fileEntity.AccountId, err)
	}

	parser := reader.parsers[bankfileparser.ParserId(accountEntity.BankName, fileEntity.FileType)]

	err = parser.Parse(
		ctx,
		file,
		fileEntity.ImportFileId,
		fileEntity.AccountId,
		reader.registerFileDataRow(accountEntity),
	)
	if err != nil {
		return bankfilereadererror.ParserFailedError(fileEntity.ImportFileId, fileEntity.AccountId, err)
	}

	return nil
}

func (reader *BankFileReader) registerFileDataRow(
	accountEntity *accountreadmodel.Entity,
) bankfileparser.BankFileDataRowHandler {
	return func(
		ctx context.Context,
		importFileId *importfile.Id,
		row *bankfileparser.BankFileDataRow,
	) *bankfilereadererror.ReaderError {
		// Ignore row if date is not as same as current account month
		if accountEntity.ActiveMonth.Month != row.Date.Month() ||
			accountEntity.ActiveMonth.Year != uint(row.Date.Year()) {
			return nil
		}

		waErr := reader.mediator.AddFileDataRow(ctx, importFileId, row)
		if waErr != nil {
			reader.logger.Error("Add file row", zap.Error(waErr))

			return bankfilereadererror.FileDataRowAddFailedError(importFileId, row, waErr)
		}

		return nil
	}
}
