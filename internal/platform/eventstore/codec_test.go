package eventstore_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

type (
	thingEvent    interface{ isThingEvent() }
	thingHappened struct {
		Amount string `json:"amount"`
	}
	thingRenamed struct {
		Name string `json:"name"`
	}
)

func (thingHappened) isThingEvent() {}
func (thingRenamed) isThingEvent()  {}

const (
	happenedV1  = "io.walletaccountant.test.thing.happened.v1"
	happenedV2  = "io.walletaccountant.test.thing.happened.v2"
	renamedV1   = "io.walletaccountant.test.thing.renamed.v1"
	traceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
)

func newThingCodec() *eventstore.Codec[thingEvent] {
	c := eventstore.NewCodec[thingEvent]("/test")
	eventstore.Register[thingEvent, thingHappened](c, happenedV2)
	eventstore.Register[thingEvent, thingRenamed](c, renamedV1)

	return c
}

func TestCodecRoundTripsAnEventAsACloudEventCarryingHouseholdAndTrace(t *testing.T) {
	t.Parallel()
	c := newThingCodec()

	attrs := eventstore.CloudEvent{
		ID:            uuid.Must(uuid.NewV7()),
		Subject:       uuid.Must(uuid.NewV7()),
		Time:          time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		HouseholdID:   uuid.New(),
		StreamType:    "test.thing",
		StreamVersion: 3,
		ActorID:       uuid.New(),
		CorrelationID: uuid.New(),
		CausationID:   uuid.New(),
		TraceParent:   traceParent,
		TraceState:    "vendor=1",
	}

	ce, err := c.Encode(attrs, thingHappened{Amount: "100.10"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	raw, err := json.Marshal(ce)
	if err != nil {
		t.Fatal(err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}

	for attr, want := range map[string]any{
		"specversion":     "1.0",
		"type":            happenedV2,
		"source":          "/test",
		"subject":         attrs.Subject.String(),
		"time":            "2026-11-16T10:00:00Z",
		"datacontenttype": "application/json",
		"householdid":     attrs.HouseholdID.String(),
		"streamtype":      "test.thing",
		"streamversion":   "3",
		"actorid":         attrs.ActorID.String(),
		"traceparent":     traceParent,
		"tracestate":      "vendor=1",
	} {
		if wire[attr] != want {
			t.Errorf("CloudEvent attribute %s = %v, want %v", attr, wire[attr], want)
		}
	}

	if data, _ := wire["data"].(map[string]any); data["amount"] != "100.10" {
		t.Errorf("data = %v, want the amount as a decimal string", wire["data"])
	}

	var back eventstore.CloudEvent
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if back.TraceParent != traceParent || back.HouseholdID != attrs.HouseholdID || back.CorrelationID != attrs.CorrelationID {
		t.Errorf("attributes lost in the round trip: %+v", back)
	}

	env, err := c.Decode(back)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := decider.Envelope[thingEvent]{
		ID:            attrs.ID,
		StreamID:      attrs.Subject,
		StreamVersion: 3,
		Time:          attrs.Time,
		HouseholdID:   attrs.HouseholdID,
		ActorID:       attrs.ActorID,
		Event:         thingHappened{Amount: "100.10"},
	}
	if !env.Time.Equal(want.Time) {
		t.Errorf("time = %v, want %v", env.Time, want.Time)
	}

	env.Time = want.Time
	if env != want {
		t.Errorf("decoded:\n  got  %+v\n  want %+v", env, want)
	}
}

func TestInstanceLevelEventsOmitTheHousehold(t *testing.T) {
	t.Parallel()
	c := newThingCodec()

	ce, err := c.Encode(eventstore.CloudEvent{ID: uuid.New(), Subject: uuid.New(), StreamType: "test.thing", StreamVersion: 1}, thingRenamed{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(ce)
	if err != nil {
		t.Fatal(err)
	}

	for _, attr := range []string{`"householdid"`, `"actorid"`} {
		if strings.Contains(string(raw), attr) {
			t.Errorf("instance-level event carries %s: %s", attr, raw)
		}
	}
}

func TestCodecUpcastsOldSchemaVersions(t *testing.T) {
	t.Parallel()
	c := newThingCodec()
	c.Upcast(happenedV1, func(data json.RawMessage) (string, json.RawMessage, error) {
		var v1 struct {
			Cents int `json:"cents"`
		}
		if err := json.Unmarshal(data, &v1); err != nil {
			return "", nil, err
		}

		return happenedV2, json.RawMessage(`{"amount":"` + centsToDecimal(v1.Cents) + `"}`), nil
	})

	env, err := c.Decode(eventstore.CloudEvent{ID: uuid.New(), Type: happenedV1, Data: json.RawMessage(`{"cents":1050}`)})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if env.Event != (thingHappened{Amount: "10.50"}) {
		t.Errorf("upcast event = %+v, want amount 10.50", env.Event)
	}
}

func TestCodecRejectsUnknownTypes(t *testing.T) {
	t.Parallel()
	c := newThingCodec()

	if _, err := c.Decode(eventstore.CloudEvent{Type: "io.walletaccountant.test.thing.vanished.v1", Data: json.RawMessage(`{}`)}); !errors.Is(err, eventstore.ErrUnknownEventType) {
		t.Errorf("decode unknown type: err = %v, want ErrUnknownEventType", err)
	}

	type unregistered struct{ thingHappened }

	if _, err := c.Encode(eventstore.CloudEvent{}, unregistered{}); !errors.Is(err, eventstore.ErrUnknownEventType) {
		t.Errorf("encode unregistered event: err = %v, want ErrUnknownEventType", err)
	}
}

func centsToDecimal(cents int) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }
