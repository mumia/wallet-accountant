package definitions

import (
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
)

type ProjectionProvider interface {
	Matcher() eventhorizon.MatchEvents
	Handler() eventhorizon.EventHandler
}

func AsProjectionProvider(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(ProjectionProvider)),
		fx.ResultTags(`group:"projectionProviders"`),
	)
}
