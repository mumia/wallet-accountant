package eventstore

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// SpecVersion is the CloudEvents specification version events follow.
const SpecVersion = "1.0"

// ContentTypeJSON is every event's data content type (ADR-0015).
const ContentTypeJSON = "application/json"

// CloudEvent is a recorded event described with CloudEvents attributes
// (ADR-0015). In the event store each attribute is a column; outside the
// process it is a CloudEvent in structured JSON mode.
type CloudEvent struct {
	ID              uuid.UUID
	Source          string
	Type            string
	Subject         uuid.UUID // the stream ID
	Time            time.Time
	DataContentType string
	DataSchema      string

	// Extensions. A uuid.Nil ID is absent: an instance-level event has no
	// HouseholdID, an operator run mode's event no ActorID.
	HouseholdID   uuid.UUID
	StreamType    string
	StreamVersion int64
	ActorID       uuid.UUID
	CorrelationID uuid.UUID
	CausationID   uuid.UUID

	// W3C trace context of the command that recorded the event.
	TraceParent string
	TraceState  string

	Data json.RawMessage
}

// structured is the CloudEvents JSON format. Extension values are strings,
// as CloudEvents requires for transport.
type structured struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	DataSchema      string          `json:"dataschema,omitempty"`
	HouseholdID     string          `json:"householdid,omitempty"`
	StreamType      string          `json:"streamtype"`
	StreamVersion   string          `json:"streamversion"`
	ActorID         string          `json:"actorid,omitempty"`
	CorrelationID   string          `json:"correlationid,omitempty"`
	CausationID     string          `json:"causationid,omitempty"`
	TraceParent     string          `json:"traceparent,omitempty"`
	TraceState      string          `json:"tracestate,omitempty"`
	Data            json.RawMessage `json:"data"`
}

// MarshalJSON encodes the event in CloudEvents structured JSON mode.
func (e CloudEvent) MarshalJSON() ([]byte, error) {
	data := e.Data
	if data == nil {
		data = json.RawMessage("null")
	}

	return json.Marshal(structured{ //nolint:wrapcheck // plain JSON encoding
		SpecVersion:     SpecVersion,
		ID:              e.ID.String(),
		Source:          e.Source,
		Type:            e.Type,
		Subject:         e.Subject.String(),
		Time:            e.Time.UTC().Format(time.RFC3339Nano),
		DataContentType: e.DataContentType,
		DataSchema:      e.DataSchema,
		HouseholdID:     optionalID(e.HouseholdID),
		StreamType:      e.StreamType,
		StreamVersion:   strconv.FormatInt(e.StreamVersion, 10),
		ActorID:         optionalID(e.ActorID),
		CorrelationID:   optionalID(e.CorrelationID),
		CausationID:     optionalID(e.CausationID),
		TraceParent:     e.TraceParent,
		TraceState:      e.TraceState,
		Data:            data,
	})
}

// UnmarshalJSON decodes an event in CloudEvents structured JSON mode.
func (e *CloudEvent) UnmarshalJSON(raw []byte) error {
	var s structured
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("cloudevent: %w", err)
	}

	if s.SpecVersion != SpecVersion {
		return fmt.Errorf("cloudevent: unsupported specversion %q", s.SpecVersion)
	}

	var (
		ev = CloudEvent{
			Source: s.Source, Type: s.Type, DataContentType: s.DataContentType, DataSchema: s.DataSchema,
			StreamType: s.StreamType, TraceParent: s.TraceParent, TraceState: s.TraceState, Data: s.Data,
		}
		err error
	)

	for _, f := range []struct {
		dst      *uuid.UUID
		raw      string
		optional bool
	}{
		{&ev.ID, s.ID, false},
		{&ev.Subject, s.Subject, false},
		{&ev.HouseholdID, s.HouseholdID, true},
		{&ev.ActorID, s.ActorID, true},
		{&ev.CorrelationID, s.CorrelationID, true},
		{&ev.CausationID, s.CausationID, true},
	} {
		if f.optional && f.raw == "" {
			continue
		}

		if *f.dst, err = uuid.Parse(f.raw); err != nil {
			return fmt.Errorf("cloudevent: %w", err)
		}
	}

	if ev.Time, err = time.Parse(time.RFC3339Nano, s.Time); err != nil {
		return fmt.Errorf("cloudevent: time: %w", err)
	}

	if ev.StreamVersion, err = strconv.ParseInt(s.StreamVersion, 10, 64); err != nil {
		return fmt.Errorf("cloudevent: streamversion: %w", err)
	}

	*e = ev

	return nil
}

func optionalID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}
