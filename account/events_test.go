package account_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/account"
)

func setupEventRegister() (*account.EventRegister, map[eventhorizon.EventType]eventhorizon.EventData) {
	return account.NewEventRegister(),
		map[eventhorizon.EventType]eventhorizon.EventData{
			account.NewAccountRegistered: &account.NewAccountRegisteredData{},
			account.NextMonthStarted:     &account.NextMonthStartedData{},
		}
}

func TestEventRegister_Registers(t *testing.T) {
	eventRegister, expectedEvents := setupEventRegister()

	eventRegisters := eventRegister.Registers()

	asserts := assert.New(t)

	t.Run("successfully returns registers for all available events", func(t *testing.T) {
		asserts.Len(eventRegisters, len(expectedEvents))

		for _, eventRegister := range eventRegisters {
			asserts.Contains(expectedEvents, eventRegister.EventType)
			asserts.Equal(expectedEvents[eventRegister.EventType], eventRegister.EventData())
		}
	})
}
