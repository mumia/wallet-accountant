// Package eventstore is the platform's event sourcing (ADR-0011): the event
// store table, the CloudEvents codec (ADR-0015), and the Decider runtime that
// loads a fold, decides, stamps, appends and saves the fold in one database
// transaction, retrying on a version conflict.
package eventstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// ErrUnknownEventType is returned for an event whose type is not registered.
var ErrUnknownEventType = errors.New("unknown event type")

// Codec encodes an aggregate's events, of the sum type E, as CloudEvents with
// JSON data and decodes them back. Each concrete event type is registered
// under its CloudEvents type, whose suffix is its schema version; older
// schema versions are upcast when read, never rewritten.
type Codec[E any] struct {
	source   string
	names    map[reflect.Type]string
	decoders map[string]func(json.RawMessage) (E, error)
	upcasts  map[string]Upcaster
}

// Upcaster turns the data of an old schema version into the next one,
// returning the type it now has.
type Upcaster func(data json.RawMessage) (eventType string, upcast json.RawMessage, err error)

// NewCodec returns a codec for events recorded by the context at source
// ("/<context>").
func NewCodec[E any](source string) *Codec[E] {
	return &Codec[E]{
		source:   source,
		names:    map[reflect.Type]string{},
		decoders: map[string]func(json.RawMessage) (E, error){},
		upcasts:  map[string]Upcaster{},
	}
}

// Register adds the event T, one of E's cases, under eventType
// (io.walletaccountant.<context>.<aggregate>.<past-tense>.v<N>).
func Register[E, T any](c *Codec[E], eventType string) {
	var zero T
	if _, ok := any(zero).(E); !ok {
		panic(fmt.Sprintf("eventstore: %T is not a case of %v", zero, reflect.TypeFor[E]()))
	}

	c.names[reflect.TypeFor[T]()] = eventType
	c.decoders[eventType] = func(data json.RawMessage) (E, error) {
		var v T
		if err := json.Unmarshal(data, &v); err != nil {
			var none E

			return none, fmt.Errorf("decode %s: %w", eventType, err)
		}

		return any(v).(E), nil //nolint:forcetypeassert // checked when registered
	}
}

// Upcast registers how to read the retired type fromType.
func (c *Codec[E]) Upcast(fromType string, up Upcaster) {
	c.upcasts[fromType] = up
}

// Encode describes event with attrs, setting its type, source, content type
// and data.
func (c *Codec[E]) Encode(attrs CloudEvent, event E) (CloudEvent, error) {
	name, ok := c.names[reflect.TypeOf(event)]
	if !ok {
		return CloudEvent{}, fmt.Errorf("%w: %T", ErrUnknownEventType, event)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return CloudEvent{}, fmt.Errorf("encode %s: %w", name, err)
	}

	attrs.Type = name
	attrs.Source = c.source
	attrs.DataContentType = ContentTypeJSON
	attrs.Data = data

	return attrs, nil
}

// Decode reads a recorded event, upcasting an old schema version first.
func (c *Codec[E]) Decode(ce CloudEvent) (decider.Envelope[E], error) {
	eventType, data := ce.Type, ce.Data

	for {
		up, ok := c.upcasts[eventType]
		if !ok {
			break
		}

		var err error
		if eventType, data, err = up(data); err != nil {
			return decider.Envelope[E]{}, fmt.Errorf("upcast %s: %w", ce.Type, err)
		}
	}

	decode, ok := c.decoders[eventType]
	if !ok {
		return decider.Envelope[E]{}, fmt.Errorf("%w: %s", ErrUnknownEventType, eventType)
	}

	event, err := decode(data)
	if err != nil {
		return decider.Envelope[E]{}, err
	}

	return decider.Envelope[E]{
		ID:            ce.ID,
		StreamID:      ce.Subject,
		StreamVersion: ce.StreamVersion,
		Time:          ce.Time,
		HouseholdID:   ce.HouseholdID,
		ActorID:       ce.ActorID,
		Event:         event,
	}, nil
}
