package definitions

import (
	"fmt"
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
)

type EventDataRegisters interface {
	Registers() []EventDataRegister
}

type EventDataRegister struct {
	EventType eventhorizon.EventType
	EventData func() eventhorizon.EventData
}

func AsEventDataRegister(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(EventDataRegisters)),
		fx.ResultTags(`group:"eventDataRegisters"`),
	)
}

func EventDataTypeError(
	expectedEventType eventhorizon.EventType,
	foundEventType eventhorizon.EventType,
) error {
	return fmt.Errorf("invalid event type. Expected: %s Found: %s", expectedEventType, foundEventType)
}
