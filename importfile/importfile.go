package importfile

import (
	"context"
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ events.VersionedAggregate = &ImportFile{}

const AggregateType eventhorizon.AggregateType = "importFile"

type State string

const (
	Imported         State = "imported"
	ParsingStarted   State = "parsingStarted"
	ParsingRestarted State = "parsingRestarted"
	ParsingEnded     State = "parsingEnded"
	ParsingFailed    State = "parsingFailed"
)

type DataRowState string

const (
	Unverified DataRowState = "unverified"
	Verified   DataRowState = "verified"
	Invalid    DataRowState = "invalid"
)

type ImportFile struct {
	*events.AggregateBase
	clock *clock.Clock

	state             State
	importedRowStates map[DataRowId]DataRowState
}

func (importFile *ImportFile) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *RegisterNewImportFile:
		if importFile.AggregateVersion() != 0 {
			return errors.New("importfile: is already registered")
		}
	default:
		if importFile.AggregateVersion() <= 0 {
			return errors.New("importfile: needs to be registered first")
		}
	}

	switch command := command.(type) {
	case *RegisterNewImportFile:
		importFile.AppendEvent(
			NewImportFileRegistered,
			&NewImportFileRegisteredData{
				ImportFileId: &command.ImportFileId,
				AccountId:    &command.AccountId,
				Filename:     command.Filename,
				FileType:     command.FileType,
			},
			importFile.clock.Now(),
		)

	case *StartFileParse:
		if importFile.state != Imported {
			return fmt.Errorf("importfile: invalida state for StartFileParse. State: %s", importFile.state)
		}

		importFile.state = ParsingStarted

		importFile.AppendEvent(
			FileParseStarted,
			&FileParseStartedData{
				ImportFileId: &command.ImportFileId,
			},
			importFile.clock.Now(),
		)

	case *RestartFileParse:
		if importFile.state != ParsingFailed {
			return fmt.Errorf("importfile: invalida state for RestartFileParse. State: %s", importFile.state)
		}

		importFile.AppendEvent(
			FileParseRestarted,
			&FileParseRestartedData{
				ImportFileId: &command.ImportFileId,
			},
			importFile.clock.Now(),
		)

	case *EndFileParse:
		if importFile.state != ParsingStarted && importFile.state != ParsingRestarted {
			return fmt.Errorf("importfile: invalida state for EndFileParse. State: %s", importFile.state)
		}

		importFile.AppendEvent(
			FileParseEnded,
			&FileParseEndedData{
				ImportFileId: &command.ImportFileId,
			},
			importFile.clock.Now(),
		)

	case *FailFileParse:
		if importFile.state != ParsingStarted && importFile.state != ParsingRestarted {
			return fmt.Errorf("importfile: invalida state for FailFileParse. State: %s", importFile.state)
		}

		importFile.AppendEvent(
			FileParseFailed,
			&FileParseFailedData{
				ImportFileId: &command.ImportFileId,
				Code:         command.Code,
				Reason:       command.Reason,
			},
			importFile.clock.Now(),
		)

	case *AddFileDataRow:
		if importFile.state != ParsingEnded {
			return fmt.Errorf("importfile: invalida state for AddFileDataRow. State: %s", importFile.state)
		}

		if _, ok := importFile.importedRowStates[command.FileDataRowId]; ok {
			return fmt.Errorf(
				"importFile: data row has already been added. DataRowId: %s",
				command.FileDataRowId.String(),
			)
		}

		importFile.AppendEvent(
			FileDataRowAdded,
			&FileDataRowAddedData{
				ImportFileId:  &command.ImportFileId,
				FileDataRowId: &command.FileDataRowId,
				Date:          command.Date,
				Description:   command.Description,
				Amount:        command.Amount,
				RawData:       command.RawData,
			},
			importFile.clock.Now(),
		)

	case *VerifyFileDataRow:
		if importFile.state != ParsingEnded {
			return fmt.Errorf("importfile: invalida state for VerifyFileDataRow. State: %s", importFile.state)
		}

		if state, ok := importFile.importedRowStates[command.FileDataRowId]; !ok || state != Unverified {
			return fmt.Errorf(
				"importFile: data row was not imported or is in an invalid state for verification. DataRowId: %s, Exists: %t, State: %s",
				command.FileDataRowId.String(),
				ok,
				state,
			)
		}

		importFile.AppendEvent(
			FileDataRowMarkedAsVerified,
			&FileDataRowMarkedAsVerifiedData{
				ImportFileId:    &command.ImportFileId,
				FileDataRowId:   &command.FileDataRowId,
				Description:     command.Description,
				MovementTypeId:  command.MovementTypeId,
				SourceAccountId: command.SourceAccountId,
				TagIds:          command.TagIds,
			},
			importFile.clock.Now(),
		)

	case *InvalidateFileDataRow:
		if importFile.state != ParsingEnded {
			return fmt.Errorf("importfile: invalida state for InvalidateFileDataRow. State: %s", importFile.state)
		}

		if state, ok := importFile.importedRowStates[command.FileDataRowId]; !ok || state != Unverified {
			return fmt.Errorf(
				"importFile: data row was not imported or is in an invalid state for invalidation. DataRowId: %s, Exists: %t, State: %s",
				command.FileDataRowId.String(),
				ok,
				state,
			)
		}

		importFile.AppendEvent(
			FileDataRowMarkedAsInvalid,
			&FileDataRowMarkedAsInvalidData{
				ImportFileId:  &command.ImportFileId,
				FileDataRowId: &command.FileDataRowId,
				Reason:        command.Reason,
			},
			importFile.clock.Now(),
		)

	case *RegisterAccountMovementIdForVerifiedFileDataRow:
		if importFile.state != ParsingEnded {
			return fmt.Errorf(
				"importfile: invalida state for RegisterAccountMovementIdForVerifiedFileDataRow. State: %s",
				importFile.state,
			)
		}

		if state, ok := importFile.importedRowStates[command.FileDataRowId]; !ok || state != Verified {
			return fmt.Errorf(
				"importFile: data row was not imported or is in an invalid state for verification account movement. DataRowId: %s, Exists: %t, State: %s",
				command.FileDataRowId.String(),
				ok,
				state,
			)
		}

		importFile.AppendEvent(
			AccountMovementIdForVerifiedFileDataRowRegistered,
			&AccountMovementIdForVerifiedFileDataRowRegisteredData{
				ImportFileId:      &command.ImportFileId,
				FileDataRowId:     &command.FileDataRowId,
				AccountMovementId: &command.AccountMovementId,
			},
			importFile.clock.Now(),
		)

	default:
		return fmt.Errorf("no command matched. CommandType: %s", command.CommandType().String())
	}

	return nil
}

func (importFile *ImportFile) ApplyEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case NewImportFileRegistered:
		importFile.state = Imported
		importFile.importedRowStates = make(map[DataRowId]DataRowState)

	case FileParseStarted:
		importFile.state = ParsingStarted

	case FileParseRestarted:
		importFile.state = ParsingRestarted

	case FileParseEnded:
		importFile.state = ParsingEnded

	case FileParseFailed:
		importFile.state = ParsingFailed

	case FileDataRowAdded:
		eventData, ok := event.Data().(*FileDataRowAddedData)
		if !ok {
			return definitions.EventDataTypeError(FileDataRowAdded, event.EventType())
		}

		importFile.importedRowStates[*eventData.FileDataRowId] = Unverified

	case FileDataRowMarkedAsVerified:
		eventData, ok := event.Data().(*FileDataRowMarkedAsVerifiedData)
		if !ok {
			return definitions.EventDataTypeError(FileDataRowMarkedAsVerified, event.EventType())
		}

		importFile.importedRowStates[*eventData.FileDataRowId] = Verified

	case FileDataRowMarkedAsInvalid:
		eventData, ok := event.Data().(*FileDataRowMarkedAsInvalidData)
		if !ok {
			return definitions.EventDataTypeError(FileDataRowMarkedAsInvalid, event.EventType())
		}

		importFile.importedRowStates[*eventData.FileDataRowId] = Invalid
	}

	return nil
}

func (importFile *ImportFile) ImportFileId() *Id {
	importFileId := Id(importFile.EntityID())

	return &importFileId
}
