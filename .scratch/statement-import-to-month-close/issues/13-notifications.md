# 13: Notifications

**What to build:** when a Statement import produces Suspected duplicates, every Household member sees a Notification linking to it, and each can mark it read. This is the slice's proof of the end-to-end deferred event path. Spec: `../spec.md` stories 40a, 40f, "Households: Notification"; ADR-0013.

**Blocked by:** 12 (Suspected duplicates, Keep, and the waiting Balance check)

**Status:** claimed

- [ ] A deferred Watermill handler on the Statement import's event creates a "suspected duplicates found" Notification with a link to the import
- [ ] `GET /households/{household_id}/notifications` and `POST …/notifications/{notification_id}/read`; read state is per User
- [ ] The handler continues the request's trace and is deduplicated through the processed-events table
- [ ] HTTP tests: the Notification appears only after events are drained with the test helper; no Notification for an import without Suspected duplicates; mark as read; another Household gets 404
- [ ] Row-level security test for every table this ticket adds; Notification fold registered for rebuild
- [ ] Consistency map row for Upload Statement import (deferred column) matches the code

## Comments

Implementation notes (implementer agent):

- **Contract.** `GET /households/{h}/notifications` (`listNotifications`, `NotificationList`, newest first, no paging) and `POST /households/{h}/notifications/{notification_id}/read` (`markNotificationRead`, 200 `Notification`, Idempotency-Key). `Notification`: `notification_id`, `kind` (`SUSPECTED_DUPLICATES_FOUND`), `statement_import_id`, `connection_id`, `account_id` (the link to the import: Statement imports are listed under the Connection), `suspected_duplicates`, `created_at`, `read` (for the caller). Another Household's or an unknown Notification → 404. Marking read again records nothing and answers 200.
- **Decider.** `households/domain.Notification`: `RaiseSuspectedDuplicatesFound` → `…households.notification.raised.v1` (refused when already raised or with no Suspected duplicates); `MarkNotificationRead{UserID}` → `…households.notification.read.v1` (refused unless raised; nothing when that User already read it). Read state is a per-User set in the fold, so one member reading leaves it unread for the others (Decider test; HTTP can't show a second member yet, since only one User can join a Household in this slice). Fold registered for rebuild; the HTTP test rebuilds and reads it back.
- **Handler, without Households importing Ingestion.** Ingestion publishes `ingestionapp.SuspectedDuplicatesFoundIn(CloudEvent)` (`internal/ingestion/app/published.go`), which reads its own `statementimport.recorded.v1` and reports false unless `suspectedduplicates` > 0. Households exposes `householdsapp.NotificationRaiser`. The forwarding handler (`internal/wiring/notifications.go`, consumer group `households.suspected-duplicates-notifier`) translates between them, like `wiring/ledgerport.go` does for immediate ports. It is the first registered handler, so `app.Drain(t)` now waits on it.
- **Platform additions.** `eventstore.HandlerTx(pgx.Tx, cause CloudEvent)` lets a deferred handler run commands in the transaction forwarding gave it: events get the cause's Household, `causationid` = the cause, its `correlationid` (or the cause's ID), the handler's trace (the middleware already continued the request's trace; the HTTP test checks the `handle …statementimport.recorded.v1` span is in the upload's trace), and no `actorid`. Dedup is the existing processed-events middleware.
- **Forwarding fix (exposed by registering the first handler).** A worker stopped while still subscribing (every short `apitest.Start` test) returned `cannot subscribe topic events: … context canceled`, and after a first attempt at a fix, `router close timeout`: Watermill leaks its handler wait group when `Subscribe` fails, so `router.Close` waits the whole `CloseTimeout`. `forwarding.Run` now returns nil when the router never got running. `TestAForwarderStoppedWhileStartingStopsCleanly` fails without it.
- **Migration** `000014_notifications`: `notifications` and `notification_reads`, both Household-scoped with forced RLS; `AssertHouseholdIsolation` tests in `internal/households/adapters/postgres/rls_integration_test.go`.
- **HTTP tests** live in `internal/ingestion/adapters/http/notifications_integration_test.go` (they reuse the Statement import fixtures): the Notification after draining (fields, trace, rebuild), none for an import without Suspected duplicates, mark read (+ replay and repeat), another Household 404 on both paths and an unknown ID 404. A test can't show the Notification is absent *before* draining without racing the in-process worker, so "appears only after draining" is asserted by draining first and never sleeping.
- **Consistency map.** The Upload Statement import deferred column now names the handler and its dedup; added a "Mark Notification read" row.
