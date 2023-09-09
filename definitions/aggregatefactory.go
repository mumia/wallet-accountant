package definitions

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/fx"
)

type AggregateFactory interface {
	Factory() func(id uuid.UUID) eventhorizon.Aggregate
}

func AsAggregateFactory(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(AggregateFactory)),
		fx.ResultTags(`group:"aggregateFactories"`),
	)
}
