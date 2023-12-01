package movementtype

import (
	"context"
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/account"
	"walletaccountant/clock"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ events.VersionedAggregate = &MovementType{}

const AggregateType eventhorizon.AggregateType = "movementType"

const (
	Debit  Type = "debit"
	Credit Type = "credit"
)

type Id = uuid.UUID

type Type string

type MovementType struct {
	*events.AggregateBase
	clock *clock.Clock

	movementType    Type
	accountId       *account.Id
	sourceAccountId *account.Id
	description     string
	notes           *string
	tagIds          []*tagcategory.TagId
}

func (movementType *MovementType) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *RegisterNewMovementType:
		if movementType.AggregateVersion() != 0 {
			return errors.New("movement type: is already registered")
		}
	default:
		if movementType.AggregateVersion() <= 0 {
			return errors.New("movement type: needs to be registered first")
		}
	}

	switch command := command.(type) {
	case *RegisterNewMovementType:
		var tagIds []*tagcategory.TagId
		for _, tagId := range command.TagIds {
			// TODO remove when new Go vesion is out
			copyTagId := tagId

			tagIds = append(tagIds, copyTagId)
		}

		movementType.AppendEvent(
			NewMovementTypeRegistered,
			&NewMovementTypeRegisteredData{
				MovementTypeId:  &command.MovementTypeId,
				Type:            command.Type,
				AccountId:       &command.AccountId,
				SourceAccountId: command.SourceAccountId,
				Description:     command.Description,
				Notes:           command.Notes,
				TagIds:          tagIds,
			},
			movementType.clock.Now(),
		)

	default:
		return fmt.Errorf("no command matched. CommandType: %s", command.CommandType().String())
	}

	return nil
}

func (movementType *MovementType) ApplyEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case NewMovementTypeRegistered:
		eventData, ok := event.Data().(*NewMovementTypeRegisteredData)
		if !ok {
			return definitions.EventDataTypeError(NewMovementTypeRegistered, event.EventType())
		}

		movementType.movementType = eventData.Type
		movementType.accountId = eventData.AccountId
		movementType.sourceAccountId = eventData.SourceAccountId
		movementType.description = eventData.Description
		movementType.notes = eventData.Notes
		movementType.tagIds = eventData.TagIds
	}

	return nil
}

func (movementType *MovementType) MovementTypeId() *Id {
	movementTypeId := Id(movementType.EntityID())

	return &movementTypeId
}

func (movementType *MovementType) Type() Type {
	return movementType.movementType
}

func (movementType *MovementType) AccountId() *account.Id {
	return movementType.accountId
}

func (movementType *MovementType) SourceAccountId() *account.Id {
	return movementType.sourceAccountId
}

func (movementType *MovementType) Description() string {
	return movementType.description
}

func (movementType *MovementType) Notes() *string {
	return movementType.notes
}

func (movementType *MovementType) TagIds() []*tagcategory.TagId {
	return movementType.tagIds
}
