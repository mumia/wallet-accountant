package eventstoredb

import (
	"github.com/looplab/eventhorizon"
	"walletaccountant/definitions"
)

func RegisterEvents(eventDataRegisters []definitions.EventDataRegisters) {
	for _, eventDataRegister := range eventDataRegisters {
		for _, register := range eventDataRegister.Registers() {
			eventhorizon.RegisterEventData(register.EventType, register.EventData)
		}
	}
}
