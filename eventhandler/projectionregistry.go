package eventhandler

import (
	"fmt"
	"github.com/looplab/eventhorizon"
	"sync"
	"walletaccountant/definitions"
)

var _ HandlerGetter = &ProjectionRegistry{}

type ProjectionRegistry struct {
	handlersMutex  *sync.RWMutex
	handlersByType map[eventhorizon.EventHandlerType]*matcherHandler
}

type matcherHandler struct {
	eventhorizon.EventMatcher
	eventhorizon.EventHandler
}

func NewProjectionRegistry(providers []definitions.ProjectionProvider) (*ProjectionRegistry, error) {
	registry := &ProjectionRegistry{
		handlersMutex:  &sync.RWMutex{},
		handlersByType: make(map[eventhorizon.EventHandlerType]*matcherHandler),
	}

	for _, provider := range providers {
		if err := registry.AddHandler(provider.Matcher(), provider.Handler()); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func (registry *ProjectionRegistry) AddHandler(
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

func (registry *ProjectionRegistry) GetHandler(
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

func (registry *ProjectionRegistry) GetHandlers() map[eventhorizon.EventHandlerType]*matcherHandler {
	return registry.handlersByType
}
