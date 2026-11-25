# 10: Notifications for joining, leaving and removal

**What to build:** every member sees a Notification when someone joins, leaves or is removed. Spec: `../spec.md`, "Households: Notifications", stories 20–21.

**Blocked by:** 06, 09

**Status:** claimed

- [ ] New Notification kinds `member_joined`, `member_left` and `member_removed`, each storing the member's User ID and, for removal, the remover's. They are raised by deferred Households handlers and deduplicated through the processed-events table
- [ ] `household.memberadded` raises `member_joined` only when it carries an `invitationid`, so creating a Household raises nothing
- [ ] `household.memberremoved` raises `member_left` when `actorid` equals `userid`, and `member_removed` otherwise
- [ ] The Notification list returns `member` and `removed_by` as `{ user_id, display_name }`, reading names in instance scope (`inInstanceScope`) as the members list does. This covers removed members, who are no longer in the members list
- [ ] Read state stays per User, so a new member sees earlier Notifications as unread
- [ ] Fill in the deferred column of the "Join Household" and "Remove member / leave" rows in `docs/architecture/consistency.md`
- [ ] Handler tests, plus HTTP tests after draining events: joined, left, removed, and nothing for creating a Household
