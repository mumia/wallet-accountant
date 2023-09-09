package account

import (
	"context"
	"errors"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"time"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ eventhorizon.Aggregate = &Account{}

const AggregateType eventhorizon.AggregateType = "Account"

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

type Month struct {
	Month time.Month
	Year  uint
}

type Account struct {
	*events.AggregateBase
	clock clock.Clock

	bankName            string
	name                string
	accountType         Type
	startingBalance     float64
	startingBalanceDate time.Time
	currency            Currency
	notes               string
	currentMonth        Month
}

func (account *Account) HandleCommand(ctx context.Context, command eventhorizon.Command) error {
	switch command.(type) {
	case *RegisterNewAccount:
		if account.name != "" {
			return errors.New("account: is already registered")
		}
	}

	switch command := command.(type) {
	case *RegisterNewAccount:
		account.AppendEvent(
			NewAccountRegistered,
			NewAccountRegisteredData{
				AccountID:           command.AccountId,
				BankName:            command.BankName,
				Name:                command.Name,
				AccountType:         command.AccountType,
				StartingBalance:     command.StartingBalance,
				StartingBalanceDate: command.StartingBalanceDate,
				Currency:            command.Currency,
				Notes:               command.Notes,
			},
			account.clock.Now(),
		)

	case *StartNextMonth:
		account.AppendEvent(NextMonthStarted, nil, account.clock.Now())
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
		account.notes = eventData.Notes
		account.currentMonth = Month{
			Month: eventData.StartingBalanceDate.Month(),
			Year:  uint(eventData.StartingBalanceDate.Year()),
		}

	case NextMonthStarted:
		nextMonth := account.currentMonth
		if nextMonth.Month == time.December {
			nextMonth.Month = time.January
			nextMonth.Year++
		} else {
			nextMonth.Month++
		}

		account.currentMonth = nextMonth
	}

	return nil
}
