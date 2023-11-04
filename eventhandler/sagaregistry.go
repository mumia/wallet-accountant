package eventhandler

import (
	"fmt"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"sync"
	"walletaccountant/definitions"
)

var _ HandlerGetter = &SagaRegistry{}

type SagaRegistry struct {
	handlersMutex  *sync.RWMutex
	handlersByType map[eventhorizon.EventHandlerType]*matcherHandler
}

func NewSagaRegistry(
	sagas []definitions.SagaProvider,
	commandHandler eventhorizon.CommandHandler,
) (*SagaRegistry, error) {
	registry := &SagaRegistry{
		handlersMutex:  &sync.RWMutex{},
		handlersByType: make(map[eventhorizon.EventHandlerType]*matcherHandler),
	}

	for _, aSaga := range sagas {
		sagaEventHandler := saga.NewEventHandler(aSaga, commandHandler)

		if err := registry.AddHandler(aSaga.Matcher(), sagaEventHandler); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func (registry *SagaRegistry) AddHandler(
	matcher eventhorizon.EventMatcher,
	handler eventhorizon.EventHandler,
) error {
	if matcher == nil {
		return eventhorizon.ErrMissingMatcher
	}

	if handler == nil {
		return eventhorizon.ErrMissingHandler
	}

	registry.handlersMutex.Lock()
	defer registry.handlersMutex.Unlock()

	if _, ok := registry.handlersByType[handler.HandlerType()]; ok {
		return eventhorizon.ErrHandlerAlreadyAdded
	}

	matcherHandler := &matcherHandler{matcher, handler}
	registry.handlersByType[handler.HandlerType()] = matcherHandler

	return nil
}

func (registry *SagaRegistry) GetHandler(
	handlerType eventhorizon.EventHandlerType,
) (eventhorizon.EventMatcher, eventhorizon.EventHandler, error) {
	registry.handlersMutex.Lock()
	defer registry.handlersMutex.Unlock()

	matcherHandler, ok := registry.handlersByType[handlerType]
	if !ok {
		return nil, nil, fmt.Errorf("no event matcher found. Event handler type: %s", handlerType)
	}

	return matcherHandler.EventMatcher, matcherHandler.EventHandler, nil
}
