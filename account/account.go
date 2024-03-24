package account

import (
	"context"
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"time"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ events.VersionedAggregate = &Account{}

const AggregateType eventhorizon.AggregateType = "account"

type BankName string

const (
	DB  BankName = "Deutsche Bank"
	N26          = "N26"
	BCP          = "Millennium bcp"
)

type Currency string

const (
	EUR Currency = "EUR"
	USD          = "USD"
	CHF          = "CHF"
)

type ActiveMonth struct {
	month time.Month
	year  uint
}

func NewActiveMonth(month time.Month, year uint) ActiveMonth {
	return ActiveMonth{
		month: month,
		year:  year,
	}
}

type Account struct {
	*events.AggregateBase
	clock *clock.Clock

	activeMonth ActiveMonth
}

func (account *Account) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *RegisterNewAccount:
		if account.AggregateVersion() != 0 {
			return errors.New("account: is already registered")
		}
	default:
		if account.AggregateVersion() <= 0 {
			return errors.New("account: needs to be registered first")
		}
	}

	switch command := command.(type) {
	case *RegisterNewAccount:
		account.AppendEvent(
			NewAccountRegistered,
			&NewAccountRegisteredData{
				AccountId:           &command.AccountId,
				BankName:            command.BankName,
				Name:                command.Name,
				AccountType:         command.AccountType,
				StartingBalance:     command.StartingBalance,
				StartingBalanceDate: command.StartingBalanceDate,
				Currency:            command.Currency,
				Notes:               command.Notes,
				ActiveMonth:         command.StartingBalanceDate.Month(),
				ActiveYear:          uint(command.StartingBalanceDate.Year()),
			},
			account.clock.Now(),
		)

	case *StartNextMonth:
		nextMonth := account.activeMonth.month
		nextYear := account.activeMonth.year
		if nextMonth == time.December {
			nextMonth = time.January
			nextYear++
		} else {
			nextMonth++
		}

		account.AppendEvent(
			NextMonthStarted,
			&NextMonthStartedData{
				AccountId: &command.AccountId,
				Balance:   command.Balance,
				NextMonth: nextMonth,
				NextYear:  nextYear,
			},
			account.clock.Now(),
		)

	default:
		return fmt.Errorf("no command matched. CommandType: %s", command.CommandType().String())
	}

	return nil
}

func (account *Account) ApplyEvent(ctx context.Context, event eventhorizon.Event) error {
	switch event.EventType() {
	case NewAccountRegistered:
		eventData, ok := event.Data().(*NewAccountRegisteredData)
		if !ok {
			return definitions.EventDataTypeError(NewAccountRegistered, event.EventType())
		}

		account.activeMonth = ActiveMonth{
			month: eventData.ActiveMonth,
			year:  eventData.ActiveYear,
		}

	case NextMonthStarted:
		eventData, ok := event.Data().(*NextMonthStartedData)
		if !ok {
			return definitions.EventDataTypeError(NextMonthStarted, event.EventType())
		}

		account.activeMonth = ActiveMonth{
			month: eventData.NextMonth,
			year:  eventData.NextYear,
		}
	}

	return nil
}

func (account *Account) AccountId() *Id {
	return IdFromUUID(account.EntityID())
}

func (account *Account) ActiveMonth() ActiveMonth {
	return account.activeMonth
}

func (activeMonth ActiveMonth) Month() time.Month {
	return activeMonth.month
}

func (activeMonth ActiveMonth) Year() uint {
	return activeMonth.year
}
