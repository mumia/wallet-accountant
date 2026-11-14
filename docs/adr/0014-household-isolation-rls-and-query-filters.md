# Household isolation by both row-level security and query filters

Every table and every event carries a `household_id`. Isolation between Households is enforced twice:

- **In the application.** Every repository query filters by Household. Tests check that a Household can never read or change another Household's rows.
- **In PostgreSQL.** Row-level security policies on every Household-scoped table compare `household_id` with a transaction-local setting, `app.household_id`. Each request or event handler sets it with `set_config('app.household_id', …, true)` at the start of its database transaction. Policies read it as `NULLIF(current_setting('app.household_id', true), '')::uuid`: on a pooled connection that has set it before, the setting reads as `''` (not NULL) once the transaction ends, and a plain `::uuid` cast would throw. Tables use `FORCE ROW LEVEL SECURITY`, so the policies also apply to the table owner.

We adopted row-level security even while self-hosted (ADR-0005), so that a single missing filter can never leak data. Doing it from the start is much cheaper than retrofitting it before going SaaS. The cost falls on work that legitimately spans Households, such as event forwarding, projection replay and `rebuild`. ## Database roles

| Role | Used by | Row-level security |
|---|---|---|
| `wa_owner` | the `migrate` run mode only; owns the schema | n/a |
| `wa_app` | the `api` run mode and every command; event handlers switch to it | enforced |
| `wa_event_store_reader` | the Watermill subscriber's connection and `rebuild`'s stream reads | `BYPASSRLS`, but only `SELECT` on the event store plus access to Watermill's offsets tables. It is a member of `wa_app` `WITH INHERIT FALSE, SET TRUE`: it can switch to `wa_app` but has none of its privileges otherwise |

Only reading the event log ever bypasses the policies. Every write anywhere goes through `wa_app` under row-level security. Event handlers run inside the subscriber's transaction, which belongs to `wa_event_store_reader` (ADR-0011). Before writing, a handler runs `SET LOCAL ROLE wa_app` and sets `app.household_id` from the event's `householdid` attribute, and runs `RESET ROLE` before acking. The spike confirmed that a handler acting this way cannot see or write another Household's rows, and that the reader role alone cannot write to the event store or to projections. `rebuild` reads streams with `wa_event_store_reader` and writes folds with `wa_app`, switching Household per stream. We rejected leaving the event store outside row-level security, and we rejected a wildcard Household setting for workers, because a single bug in a request path could then expose every Household.
