package definitions

import (
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"go.uber.org/fx"
)

type SagaProvider interface {
	saga.Saga
	Matcher() eventhorizon.MatchEvents
}

func AsSagaProvider(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(SagaProvider)),
		fx.ResultTags(`group:"sagaProviders"`),
	)
}
