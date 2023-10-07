package definitions

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
)

type EventMatcherHandleProvider interface {
	Matcher() eventhorizon.MatchEvents
	Handler() eventhorizon.EventHandler
}

func AsEventMatcherHandleProvider(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(EventMatcherHandleProvider)),
		fx.ResultTags(`group:"eventMatcherHandleProviders"`),
	)
}
