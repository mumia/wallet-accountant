# 18: Notification read state per User over HTTP

**What to build:** an HTTP test showing that read state is per User: one Household member marks a Notification read through `POST /households/{household_id}/notifications/{notification_id}/read`, and `GET /households/{household_id}/notifications` still shows it unread to another member. Spec: `../spec.md` story 40f, "Households: Notification".

**Blocked by:** a second User can join a Household (not in this slice: only one User can join a Household here)

**Status:** needs-triage

- [ ] HTTP test with two members of one Household: after the first marks the Notification read, it is `read: true` for the first and `read: false` for the second
- [ ] Marking read as the second member then shows it read for both

## Comments

Left untested when the slice was implemented (see ticket 13's comments). The per-User read set is covered only by the Decider test in `households/domain`; the HTTP tests in `internal/ingestion/adapters/http/notifications_integration_test.go` use a single member. Every other gap noted at the end of the slice is now covered by `make smoke` (a real Zitadel token signs in) and `SMOKE_UP=up-observability make smoke` (traces, metrics and logs reach Grafana).
