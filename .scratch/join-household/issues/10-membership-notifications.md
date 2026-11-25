# 10: Notifications for joining, leaving and removal

**What to build:** every member sees a Notification when someone joins, leaves or is removed. Spec: `../spec.md`, "Households: Notifications", stories 20–21.

**Blocked by:** 06, 09

**Status:** resolved

- [ ] New Notification kinds `member_joined`, `member_left` and `member_removed`, each storing the member's User ID and, for removal, the remover's. They are raised by deferred Households handlers and deduplicated through the processed-events table
- [ ] `household.memberadded` raises `member_joined` only when it carries an `invitationid`, so creating a Household raises nothing
- [ ] `household.memberremoved` raises `member_left` when `actorid` equals `userid`, and `member_removed` otherwise
- [ ] The Notification list returns `member` and `removed_by` as `{ user_id, display_name }`, reading names in instance scope (`inInstanceScope`) as the members list does. This covers removed members, who are no longer in the members list
- [ ] Read state stays per User, so a new member sees earlier Notifications as unread
- [ ] Fill in the deferred column of the "Join Household" and "Remove member / leave" rows in `docs/architecture/consistency.md`
- [ ] Handler tests, plus HTTP tests after draining events: joined, left, removed, and nothing for creating a Household

## Comments

Implementation notes (implementer agent):

- **Decider.** `RaiseMemberJoined{MemberID}`, `RaiseMemberLeft{MemberID}`, `RaiseMemberRemoved{MemberID, RemovedBy}` record the existing `notification.raised.v1` with new `memberid` and `removedby` fields; all refused once raised. Every non-`kind` field of `NotificationRaised` is now `omitzero`, so a member Notification carries no zero Statement import IDs (older events still decode the same). Fold schema version unchanged: existing folds have nothing to add.
- **Handler.** `householdsapp.MembershipNotificationIn(CloudEvent)` is the pure translation (memberadded with `invitationid` → joined, without → nothing; memberremoved with `actorid == userid` → left, otherwise removed by the actor; anything else → nothing), unit-tested in `internal/households/app/membership_notifications_test.go`. `NotificationRaiser.RaiseMembershipNotification` raises it. The forwarding handler is `membershipNotifier()` in `internal/wiring/notifications.go`, consumer group `households.membership-notifier`, deduplicated by the processed-events middleware; it is in `handlers()`, so `app.Drain` waits for it.
- **Migration `000024_membership_notifications`:** the Statement import columns of `notifications` become nullable (NULL for other kinds) and `member_id`, `removed_by` are added. No names are copied.
- **Reading.** The list (and the `markNotificationRead` response, which now reads the Notification back through the new `NotificationReader.NotificationFor` instead of building it from the fold) fills in display names with ticket 04's `displayNames` in `inInstanceScope`, so removed members keep their name and renames show. The response leaves out every field of other kinds (previously the Statement import fields were always set).
- **HTTP tests** (`internal/households/adapters/http/membership_notifications_integration_test.go`): nothing for creating a Household; joined through `/memberships` (both members see it) and through `/registrations`; left; removed with `removed_by`, then a rename and a rebuild; a new member sees an earlier Notification unread while the member who read it sees it read, and the mark-read response names the member.
- **Consistency map:** deferred column filled in for "Join Household" and "Remove member / leave". The "Redeem Invitation" row with a Household code goes through the Join Household steps, so it is covered there.
