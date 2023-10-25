package movementtype_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/movementtype"
)

func setupEventRegister() (*movementtype.EventRegister, map[eventhorizon.EventType]eventhorizon.EventData) {
	return movementtype.NewEventRegister(),
		map[eventhorizon.EventType]eventhorizon.EventData{
			movementtype.NewMovementTypeRegistered: &movementtype.NewMovementTypeRegisteredData{},
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
