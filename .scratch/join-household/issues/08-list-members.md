# 08: List a Household's members

**What to build:** `GET /households/{household_id}/members` lists every member with their display name and when they joined. Spec: `../spec.md`, "Households: members", story 14.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Migration: `household_members.joined_at`, backfilled from each member's `household.memberadded` event time. The fold store writes it from the event's time from now on
- [ ] Read the member IDs and `joined_at` in Household scope, then switch to instance scope (`inInstanceScope`) to read display names. Nothing is copied into Household scope
- [ ] Items are `{ user_id, display_name, joined_at }`, oldest first. A legacy User has no `display_name`
- [ ] A caller who isn't a member gets 404. Delete the stub
- [ ] HTTP tests:
  - the creator alone;
  - two members after a join (use ticket 06's helper when it has landed, otherwise seed through a test helper);
  - a rename reflected in the list;
  - no name for a legacy User;
  - another Household gets 404
