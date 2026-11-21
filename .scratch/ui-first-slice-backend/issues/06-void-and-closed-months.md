# 06: Void refused when an import touches a closed month; `void_blocked_by`

**What to build:** Void is refused with 409 `statement-import-touches-closed-month` when the import touches a closed month, and every Statement import lists `void_blocked_by`. Spec: `../spec.md`, "Ingestion around the Open month" (Void, `void_blocked_by`), stories 14, 15.

**Blocked by:** 05

**Status:** ready-for-agent

- [ ] The touches-closed-month rule: the earliest Booking month of its rows in any state, or the first import's start month
- [ ] 409 with `booking_month` (earliest closed month touched), `open_month` and the spec's copy, after the existing refusals
- [ ] `void_blocked_by` on list, upload and void responses, in the order `NOT_LATEST`, `HAS_ACCEPTED_ROWS`, `TOUCHES_CLOSED_MONTH`; empty for a voided import
- [ ] HTTP tests: a later import with rows in a closed month; the first import with a closed start month; rows that were ignored don't block; `void_blocked_by` with several reasons at once
- [ ] Consistency map: Void row

## Comments
