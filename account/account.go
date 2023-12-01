package account

import (
	"context"
	"errors"
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"time"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ events.VersionedAggregate = &Account{}

const AggregateType eventhorizon.AggregateType = "account"

type Id = uuid.UUID
type Type int

const (
	Checking Type = iota + 1
	Savings
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

type Account struct {
	*events.AggregateBase
	clock *clock.Clock

	bankName            string
	name                string
	accountType         Type
	startingBalance     float64
	startingBalanceDate time.Time
	currency            Currency
	activeMonth         ActiveMonth
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
			NewAccountRegisteredData{
				AccountID:           &command.AccountId,
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
			NextMonthStartedData{
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
		eventData, ok := event.Data().(NewAccountRegisteredData)
		if !ok {
			return definitions.EventDataTypeError(NewAccountRegistered, event.EventType())
		}

		account.bankName = eventData.BankName
		account.name = eventData.Name
		account.accountType = eventData.AccountType
		account.startingBalance = eventData.StartingBalance
		account.startingBalanceDate = eventData.StartingBalanceDate
		account.currency = eventData.Currency
		account.activeMonth = ActiveMonth{
			month: eventData.ActiveMonth,
			year:  eventData.ActiveYear,
		}

	case NextMonthStarted:
		eventData, ok := event.Data().(NextMonthStartedData)
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

func (account *Account) BankName() string {
	return account.bankName
}

func (account *Account) Name() string {
	return account.name
}

func (account *Account) AccountType() Type {
	return account.accountType
}

func (account *Account) StartingBalance() float64 {
	return account.startingBalance
}

func (account *Account) StartingBalanceDate() time.Time {
	return account.startingBalanceDate
}

func (account *Account) Currency() Currency {
	return account.currency
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
