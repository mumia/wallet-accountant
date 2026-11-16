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
