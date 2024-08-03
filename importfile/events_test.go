package importfile_test

import (
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"walletaccountant/importfile"
)

func setupEventRegister() (*importfile.EventRegister, map[eventhorizon.EventType]eventhorizon.EventData) {
	return importfile.NewEventRegister(),
		map[eventhorizon.EventType]eventhorizon.EventData{
			importfile.NewImportFileRegistered:                           &importfile.NewImportFileRegisteredData{},
			importfile.FileParseStarted:                                  &importfile.FileParseStartedData{},
			importfile.FileParseRestarted:                                &importfile.FileParseRestartedData{},
			importfile.FileParseEnded:                                    &importfile.FileParseEndedData{},
			importfile.FileParseFailed:                                   &importfile.FileParseFailedData{},
			importfile.FileDataRowAdded:                                  &importfile.FileDataRowAddedData{},
			importfile.FileDataRowMarkedAsVerified:                       &importfile.FileDataRowMarkedAsVerifiedData{},
			importfile.FileDataRowMarkedAsInvalid:                        &importfile.FileDataRowMarkedAsInvalidData{},
			importfile.AccountMovementIdForVerifiedFileDataRowRegistered: &importfile.AccountMovementIdForVerifiedFileDataRowRegisteredData{},
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
