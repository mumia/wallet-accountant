package importfilesaga

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/importfile"
	"walletaccountant/importfilereadmodel"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
)

var _ saga.Saga = &ImportFileDataRowVerifiedSaga{}
var _ definitions.SagaProvider = &ImportFileDataRowVerifiedSaga{}

const ImportFileDataRowVerifiedSagaType saga.Type = "ImportFileDataRowVerifiedSaga"

type ImportFileDataRowVerifiedSaga struct {
	importFileRepository   importfilereadmodel.ReadModeler
	accountRepository      accountreadmodel.ReadModeler
	accountMonthRepository ledgerreadmodel.ReadModeler
	idCreator              eventstoredb.IdGenerator
}

func NewImportFileDataRowVerifiedSaga(
	importFileRepository importfilereadmodel.ReadModeler,
	accountRepository accountreadmodel.ReadModeler,
	accountMonthRepository ledgerreadmodel.ReadModeler,
	idCreator eventstoredb.IdGenerator,
) *ImportFileDataRowVerifiedSaga {
	return &ImportFileDataRowVerifiedSaga{
		importFileRepository:   importFileRepository,
		accountRepository:      accountRepository,
		accountMonthRepository: accountMonthRepository,
		idCreator:              idCreator,
	}
}

func (saga *ImportFileDataRowVerifiedSaga) Matcher() eventhorizon.MatchEvents {
	return eventhorizon.MatchEvents{
		importfile.FileDataRowMarkedAsVerified,
	}
}

func (saga *ImportFileDataRowVerifiedSaga) SagaType() saga.Type {
	return ImportFileDataRowVerifiedSagaType
}

func (saga *ImportFileDataRowVerifiedSaga) RunSaga(
	ctx context.Context,
	event eventhorizon.Event,
	handler eventhorizon.CommandHandler,
) error {
	switch event.EventType() {
	case importfile.FileDataRowMarkedAsVerified:
		eventData, ok := event.Data().(*importfile.FileDataRowMarkedAsVerifiedData)
		if !ok {
			return definitions.EventDataTypeError(importfile.FileDataRowMarkedAsVerified, event.EventType())
		}

		return saga.handleFileDataRowMarkedAsVerified(ctx, handler, eventData)
	}

	return nil
}

func (saga *ImportFileDataRowVerifiedSaga) handleFileDataRowMarkedAsVerified(
	ctx context.Context,
	handler eventhorizon.CommandHandler,
	eventData *importfile.FileDataRowMarkedAsVerifiedData,
) error {
	fileEntity, err := saga.importFileRepository.GetById(ctx, eventData.ImportFileId)
	if err != nil {
		return fmt.Errorf("failed to get imported file. Id: %s Error: %w", eventData.ImportFileId, err)
	}

	fileDataRowEntity, err := saga.importFileRepository.GetFileRowByRowId(
		ctx,
		eventData.FileDataRowId,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get imported file data row. Id: %s DataRowId: %s Error: %w",
			fileEntity.ImportFileId,
			eventData.FileDataRowId,
			err,
		)
	}

	accountEntity, err := saga.accountRepository.GetByAccountId(ctx, fileEntity.AccountId)
	if err != nil {
		return fmt.Errorf(
			"failed to get account for imported file. ImportFileId: %s DataRowId: %s AccountId: %s Error: %w",
			fileEntity.ImportFileId,
			fileDataRowEntity.FileDataRowId,
			fileEntity.AccountId,
			err,
		)
	}

	accountMonthEntity, err := saga.accountMonthRepository.GetByAccountActiveMonth(ctx, accountEntity)
	if err != nil {
		return fmt.Errorf(
			"failed to get account month for imported file. ImportFileId: %s DataRowId: %s AccountId: %s Month: %d Year: %d Error: %w",
			fileEntity.ImportFileId,
			fileDataRowEntity.FileDataRowId,
			fileEntity.AccountId,
			accountEntity.ActiveMonth.Month,
			accountEntity.ActiveMonth.Year,
			err,
		)
	}

	action := common.Credit
	if fileDataRowEntity.Amount < 0 {
		action = common.Debit

		fileDataRowEntity.Amount = fileDataRowEntity.Amount * -1
	}

	accountMovementId := ledger.AccountMovementIdFromUUID(saga.idCreator.New())

	err = handler.HandleCommand(
		ctx,
		&ledger.RegisterNewAccountMovement{
			AccountMonthId:    *accountMonthEntity.AccountMonthId,
			AccountMovementId: *accountMovementId,
			MovementTypeId:    eventData.MovementTypeId,
			Action:            action,
			Amount:            fileDataRowEntity.Amount,
			Date:              fileDataRowEntity.Date,
			SourceAccountId:   eventData.SourceAccountId,
			Description:       eventData.Description,
			Notes:             nil,
			TagIds:            eventData.TagIds,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to handle register new account movement command for imported file."+
				"  ImportFileId: %s DataRowId: %s AccountId: %s AccountMonthId: %s Error: %w",
			fileEntity.ImportFileId,
			fileDataRowEntity.FileDataRowId,
			fileEntity.AccountId,
			accountMonthEntity.AccountMonthId,
			err,
		)
	}

	err = handler.HandleCommand(
		ctx,
		&importfile.RegisterAccountMovementIdForVerifiedFileDataRow{
			ImportFileId:      *fileEntity.ImportFileId,
			FileDataRowId:     *fileDataRowEntity.FileDataRowId,
			AccountMovementId: *accountMovementId,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to handle register account movement id for verified file data row command for imported file."+
				"  ImportFileId: %s DataRowId: %s AccountId: %s AccountMonthId: %s Error: %w",
			fileEntity.ImportFileId,
			fileDataRowEntity.FileDataRowId,
			fileEntity.AccountId,
			accountMonthEntity.AccountMonthId,
			err,
		)
	}

	return nil
}
