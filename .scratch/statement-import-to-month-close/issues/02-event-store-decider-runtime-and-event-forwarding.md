# 02: Event store, Decider runtime and event forwarding

**What to build:** the platform module every aggregate uses. A command loads a fold, decides, appends events and saves the fold in one database transaction. Events are forwarded to deferred handlers in-process, exactly once in effect, with Household isolation applied. This is enabling work, so it is verified by platform integration tests, not by a user-facing endpoint. Spec: `../spec.md` ("Platform module", "Migrations"); ADR-0006, ADR-0011, ADR-0013, ADR-0014, ADR-0015; the spike branch `prototype/spike-eventstore-ogen` is the reference.

**Blocked by:** 01 (Project scaffolding and tooling)

**Status:** ready-for-agent

- [ ] The `migrate` run mode runs golang-migrate forward-only as `wa_owner`; `make migrate` runs it against the dev database
- [ ] The first migration creates the three roles (ADR-0014), the event store table, Watermill's offsets table and the processed-events table. The event store has `FORCE ROW LEVEL SECURITY` with the `app.household_id` policy and admits rows with a NULL Household only in instance scope
- [ ] The CloudEvents codec (ADR-0015) encodes and decodes events, carrying `householdid` and `traceparent`
- [ ] The Decider runtime: load fold and version, decide, stamp, encode/decode round-trip, evolve, append, and save if the version is unchanged; it retries on a version conflict
- [ ] A custom Watermill SchemaAdapter forwards events from the event store in-process
- [ ] Handler middleware: a SAVEPOINT per attempt, rolled back before a Nack; processed-events dedup by `(consumer_group, event_id)`; `SET LOCAL ROLE wa_app` plus `set_config` from `householdid` (empty for instance-level events); `RESET ROLE` before ack; the handler continues the event's trace
- [ ] The `worker` and `all` run modes start the forwarder; `make run-worker` works
- [ ] The `rebuild` run mode rebuilds every registered fold from the event store; `make rebuild` works. Later tickets register each new fold
- [ ] A test helper drains every pending event on demand, so deferred effects can be asserted without sleeping
- [ ] Platform integration tests against real PostgreSQL cover the spike's scenarios: gap safety, replay, Nack and shutdown redelivery, and a handler trying to write into another Household
