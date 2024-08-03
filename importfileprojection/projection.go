package importfileprojection

import (
	"context"
	"github.com/looplab/eventhorizon"
	"go.uber.org/zap"
	"walletaccountant/definitions"
	"walletaccountant/importfile"
	"walletaccountant/importfilereadmodel"
	"walletaccountant/websocket"
)

var _ eventhorizon.EventHandler = &Projection{}
var _ websocket.ModelUpdateNotifier = &Projection{}
var _ ReadModelProjection = &Projection{}

type ReadModelProjection interface {
	eventhorizon.EventHandler
}

type Projection struct {
	repository    importfilereadmodel.ReadModeler
	updateChannel chan websocket.ModelUpdated
	fileToParse   *FileToParseNotifier
	log           *zap.Logger
}

func NewProjection(
	repository importfilereadmodel.ReadModeler,
	fileToParse *FileToParseNotifier,
	log *zap.Logger,
) *Projection {
	return &Projection{
		repository:    repository,
		updateChannel: make(chan websocket.ModelUpdated),
		fileToParse:   fileToParse,
		log:           log,
	}
}

func (projection *Projection) HandlerType() eventhorizon.EventHandlerType {
	return eventhorizon.EventHandlerType(importfile.AggregateType.String())
}

func (projection *Projection) HandleEvent(ctx context.Context, event eventhorizon.Event) error {
	var err error
	switch event.EventType() {
	case importfile.NewImportFileRegistered:
		err = projection.handleNewImportFileRegistered(ctx, event)

	case importfile.FileParseStarted:
		err = projection.handleFileParseStarted(ctx, event)

	case importfile.FileParseRestarted:
		err = projection.handleFileParseRestarted(ctx, event)

	case importfile.FileParseEnded:
		err = projection.handleFileParseEnded(ctx, event)

	case importfile.FileParseFailed:
		err = projection.handleFileParseFailed(ctx, event)

	case importfile.FileDataRowAdded:
		err = projection.handleFileDataRowAdded(ctx, event)

	case importfile.FileDataRowMarkedAsVerified:
		err = projection.handleFileDataRowMarkedAsVerified(ctx, event)

	case importfile.FileDataRowMarkedAsInvalid:
		err = projection.handleFileDataRowMarkedAsInvalid(ctx, event)

	}

	if err == nil {
		projection.updateChannel <- websocket.ModelUpdated{Event: event.EventType()}
	}

	return err
}

func (projection *Projection) UpdatedAggregate() eventhorizon.AggregateType {
	return importfile.AggregateType
}

func (projection *Projection) UpdateChannel() chan websocket.ModelUpdated {
	return projection.updateChannel
}

func (projection *Projection) handleNewImportFileRegistered(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.NewImportFileRegisteredData)
	if !ok {
		return definitions.EventDataTypeError(importfile.NewImportFileRegistered, event.EventType())
	}

	err := projection.repository.Register(
		ctx,
		importfilereadmodel.Entity{
			ImportFileId: eventData.ImportFileId,
			AccountId:    eventData.AccountId,
			Filename:     eventData.Filename,
			FileType:     eventData.FileType,
			ImportDate:   event.Timestamp(),
			RowCount:     0,
		},
	)
	if err != nil {
		return err
	}

	projection.fileToParse.Channel() <- eventData.ImportFileId

	return nil
}

func (projection *Projection) handleFileParseStarted(ctx context.Context, event eventhorizon.Event) error {
	return projection.repository.StartParse(ctx, importfile.IdFromUUID(event.AggregateID()), event.Timestamp())
}

func (projection *Projection) handleFileParseRestarted(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.FileParseRestartedData)
	if !ok {
		return definitions.EventDataTypeError(importfile.FileParseRestarted, event.EventType())
	}

	id := importfile.IdFromUUID(event.AggregateID())

	err := projection.repository.RestartParse(ctx, id, event.Timestamp())
	if err != nil {
		return err
	}

	projection.fileToParse.Channel() <- eventData.ImportFileId

	return nil
}

func (projection *Projection) handleFileParseEnded(ctx context.Context, event eventhorizon.Event) error {
	return projection.repository.EndParse(ctx, importfile.IdFromUUID(event.AggregateID()), event.Timestamp())
}

func (projection *Projection) handleFileParseFailed(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.FileParseFailedData)
	if !ok {
		return definitions.EventDataTypeError(importfile.FileParseFailed, event.EventType())
	}

	id := importfile.IdFromUUID(event.AggregateID())

	return projection.repository.FailParse(ctx, id, event.Timestamp(), eventData.Code, eventData.Reason)
}

func (projection *Projection) handleFileDataRowAdded(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.FileDataRowAddedData)
	if !ok {
		return definitions.EventDataTypeError(importfile.FileDataRowAdded, event.EventType())
	}

	id := importfile.IdFromUUID(event.AggregateID())

	return projection.repository.AddFileDataRow(
		ctx,
		id,
		importfilereadmodel.FileRowEntity{
			FileDataRowId: eventData.FileDataRowId,
			Date:          eventData.Date,
			Description:   eventData.Description,
			Amount:        eventData.Amount,
			RawData:       eventData.RawData,
			State:         importfile.Unverified,
		},
	)
}

func (projection *Projection) handleFileDataRowMarkedAsVerified(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.FileDataRowMarkedAsVerifiedData)
	if !ok {
		return definitions.EventDataTypeError(importfile.FileDataRowMarkedAsVerified, event.EventType())
	}

	return projection.repository.VerifyDataRow(ctx, eventData.FileDataRowId)
}

func (projection *Projection) handleFileDataRowMarkedAsInvalid(ctx context.Context, event eventhorizon.Event) error {
	eventData, ok := event.Data().(*importfile.FileDataRowMarkedAsInvalidData)
	if !ok {
		return definitions.EventDataTypeError(importfile.FileDataRowMarkedAsInvalid, event.EventType())
	}

	return projection.repository.InvalidateDataRow(ctx, eventData.FileDataRowId)
}
