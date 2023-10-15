package tagcategory

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/clock"
	"walletaccountant/definitions"
)

var _ definitions.AggregateFactory = &Factory{}

type Factory struct {
	clock *clock.Clock
}

func NewFactory() *Factory {
	return &Factory{}
}

func (factory *Factory) Factory() func(id uuid.UUID) eventhorizon.Aggregate {
	return func(id uuid.UUID) eventhorizon.Aggregate {
		return &TagCategory{
			AggregateBase: events.NewAggregateBase(AggregateType, id),
			clock:         factory.clock,
		}
	}
}
