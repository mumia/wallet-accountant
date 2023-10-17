package tagcategory_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/tagcategory"
)

func setupEventRegister() (*tagcategory.EventRegister, map[eventhorizon.EventType]eventhorizon.EventData) {
	return tagcategory.NewEventRegister(),
		map[eventhorizon.EventType]eventhorizon.EventData{
			tagcategory.NewTagAddedToNewCategory:      &tagcategory.NewTagAddedToNewCategoryData{},
			tagcategory.NewTagAddedToExistingCategory: &tagcategory.NewTagAddedToExistingCategoryData{},
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
