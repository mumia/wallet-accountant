package projector

import (
	"context"
	"fmt"
	"github.com/looplab/eventhorizon"
	"sync"
	"walletaccountant/definitions"
)

type EventMatcherHandlerRegistry struct {
	handlersMutex  *sync.RWMutex
	handlersByType map[eventhorizon.EventHandlerType]*matcherHandler
}

type matcherHandler struct {
	eventhorizon.EventMatcher
	eventhorizon.EventHandler
}

func NewEventMatcherHandlerRegistry(
	providers []definitions.EventMatcherHandleProvider,
) (*EventMatcherHandlerRegistry, error) {
	registry := &EventMatcherHandlerRegistry{
		handlersMutex:  &sync.RWMutex{},
		handlersByType: make(map[eventhorizon.EventHandlerType]*matcherHandler),
	}

	for _, provider := range providers {
		if err := registry.AddHandler(context.TODO(), provider.Matcher(), provider.Handler()); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func (projector EventMatcherHandlerRegistry) AddHandler(
	ctx context.Context,
	matcher eventhorizon.EventMatcher,
	handler eventhorizon.EventHandler,
) error {
	if matcher == nil {
		return eventhorizon.ErrMissingMatcher
	}

	if handler == nil {
		return eventhorizon.ErrMissingHandler
	}

	projector.handlersMutex.Lock()
	defer projector.handlersMutex.Unlock()

	if _, ok := projector.handlersByType[handler.HandlerType()]; ok {
		return eventhorizon.ErrHandlerAlreadyAdded
	}

	matcherHandler := &matcherHandler{matcher, handler}
	projector.handlersByType[handler.HandlerType()] = matcherHandler

	return nil
}

func (projector EventMatcherHandlerRegistry) GetHandler(
	handlerType eventhorizon.EventHandlerType,
) (eventhorizon.EventMatcher, eventhorizon.EventHandler, error) {
	projector.handlersMutex.Lock()
	defer projector.handlersMutex.Unlock()

	matcherHandler, ok := projector.handlersByType[handlerType]
	if !ok {
		return nil, nil, fmt.Errorf("no event matcher found. Event handler type: %s", handlerType)
	}

	return matcherHandler.EventMatcher, matcherHandler.EventHandler, nil
}
