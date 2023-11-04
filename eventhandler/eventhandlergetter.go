package eventhandler

import "github.com/looplab/eventhorizon"

type HandlerGetter interface {
	GetHandler(
		handlerType eventhorizon.EventHandlerType,
	) (eventhorizon.EventMatcher, eventhorizon.EventHandler, error)
}
