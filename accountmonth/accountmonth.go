package accountmonth

import (
	"context"
	"errors"
	"fmt"
	uuid2 "github.com/google/uuid"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/zeebo/xxh3"
	"time"
	"walletaccountant/account"
	"walletaccountant/clock"
	"walletaccountant/common"
	"walletaccountant/definitions"
)

var _ events.VersionedAggregate = &AccountMonth{}

const AggregateType eventhorizon.AggregateType = "accountMonth"
const idFormat string = "%s-%d-%d"

type Id = uuid.UUID
type AccountMovementId = uuid.UUID

type ActiveMonth struct {
	month time.Month
	year  uint
}

type AccountMonth struct {
	*events.AggregateBase
	clock *clock.Clock

	activeMonth *ActiveMonth
	balance     float32
}

func (accountMonth *AccountMonth) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *StartAccountMonth:
		if accountMonth.AggregateVersion() != 0 {
			return errors.New("account month: is already registered")
		}
	default:
		if accountMonth.AggregateVersion() <= 0 {
			return errors.New("account month: needs to be registered first")
		}
	}

	switch command := command.(type) {
	case *StartAccountMonth:
		accountMonth.AppendEvent(
			MonthStarted,
			&MonthStartedData{
				AccountMonthId: &command.AccountMonthId,
				AccountId:      &command.AccountId,
				StartBalance:   command.StartBalance,
				Month:          command.Month,
				Year:           command.Year,
			},
			accountMonth.clock.Now(),
		)

	case *RegisterNewAccountMovement:
		if command.Date.Month() != accountMonth.activeMonth.month ||
			command.Date.Year() != int(accountMonth.activeMonth.year) {
			return fmt.Errorf(
				"account month: mismatched active month. Account: %d/%d NewMovement: %d/%d",
				accountMonth.activeMonth.year,
				accountMonth.activeMonth.month,
				command.Date.Year(),
				command.Date.Month(),
			)
		}

		accountMonth.AppendEvent(
			NewAccountMovementRegistered,
			&NewAccountMovementRegisteredData{
				AccountMonthId:    &command.AccountMonthId,
				AccountMovementId: &command.AccountMovementId,
				MovementTypeId:    command.MovementTypeId,
				Action:            command.Action,
				Amount:            command.Amount,
				Date:              command.Date,
				SourceAccountId:   command.SourceAccountId,
				Description:       command.Description,
				Notes:             command.Notes,
				TagIds:            command.TagIds,
			},
			accountMonth.clock.Now(),
		)

	case *EndAccountMonth:
		if command.EndBalance != accountMonth.balance {
			return fmt.Errorf(
				"account month: end of month balance is different. Account: %.2f EndOfMonth: %.2f",
				accountMonth.balance,
				command.EndBalance,
			)
		}

		accountMonth.AppendEvent(
			MonthEnded,
			&MonthEndedData{
				AccountMonthId: &command.AccountMonthId,
				AccountId:      &command.AccountId,
				EndBalance:     command.EndBalance,
				Month:          command.Month,
				Year:           command.Year,
			},
			accountMonth.clock.Now(),
		)

	default:
		return fmt.Errorf("no command matched. CommandType: %s", command.CommandType().String())
	}

	return nil
}

func (accountMonth *AccountMonth) ApplyEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case MonthStarted:
		eventData, ok := event.Data().(*MonthStartedData)
		if !ok {
			return definitions.EventDataTypeError(MonthStarted, event.EventType())
		}

		activeMonth := ActiveMonth{
			month: eventData.Month,
			year:  eventData.Year,
		}

		accountMonth.activeMonth = &activeMonth
		accountMonth.balance = eventData.StartBalance

	case NewAccountMovementRegistered:
		eventData, ok := event.Data().(*NewAccountMovementRegisteredData)
		if !ok {
			return definitions.EventDataTypeError(NewAccountMovementRegistered, event.EventType())
		}

		switch eventData.Action {
		case common.Credit:
			accountMonth.balance = accountMonth.balance + eventData.Amount

		case common.Debit:
			accountMonth.balance = accountMonth.balance - eventData.Amount
		}
	}

	return nil
}

func GenerateAccountMonthId(accountId *account.Id, month time.Month, year uint) (*Id, error) {
	idData := fmt.Sprintf(idFormat, accountId.String(), month, year)
	hash := xxh3.HashString128(idData).Bytes()
	uuidAux, err := uuid2.FromBytes(hash[:])
	if err != nil {
		return nil, err
	}

	id := Id(uuidAux)

	return &id, nil
}
