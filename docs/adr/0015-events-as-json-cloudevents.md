# Domain events are JSON, described with CloudEvents attributes

Every domain event is encoded as JSON and described with the attributes of the [CloudEvents](https://cloudevents.io/) specification (`id`, `source`, `type`, `subject`, `time`, `datacontenttype`, `dataschema` and extensions). In the event store, the attributes are columns and the event data is `jsonb`. When an event leaves the process (forwarded to handlers or, later, a broker) it is a CloudEvents message. We chose JSON over Protobuf because event volumes are small, and being able to read and query history with SQL matters more than compactness. We chose CloudEvents so that events are self-describing and any broker or external consumer can understand them without a custom envelope. Money amounts inside event data are decimal strings, never JSON numbers (ADR-0007). Encoding goes through the `Codec` port (ADR-0011), so it can be changed later.

## Attribute mapping

| Attribute | Value | Example |
|---|---|---|
| `specversion` | `1.0` | |
| `id` | event ID (UUIDv7) | |
| `source` | `/<context>` | `/ledger` |
| `type` | `io.walletaccountant.<context>.<aggregate>.<past-tense event>.v<schema version>` | `io.walletaccountant.ledger.transaction.recorded.v1` |
| `subject` | stream ID (the aggregate's ID) | |
| `time` | when the event occurred | |
| `datacontenttype` | `application/json` | |
| `dataschema` | optional URI of the event's JSON Schema | |
| extensions | `householdid`, `streamtype`, `streamversion`, `actorid`, `correlationid`, `causationid` | |
| distributed tracing extension | `traceparent`, `tracestate` | W3C trace context of the command that recorded the event |

The schema version is part of `type`, so consumers and `upcast` can tell versions apart without reading `data`. The distributed tracing extension lets a Watermill handler continue the trace of the request that caused the event, so one trace shows the request and every deferred effect. Extension names are lowercase letters and digits only, as CloudEvents requires. Instance-level events (Users and Invitations, ADR-0017) omit `householdid`. Events recorded by an operator run mode, which has no User, omit `actorid`.
