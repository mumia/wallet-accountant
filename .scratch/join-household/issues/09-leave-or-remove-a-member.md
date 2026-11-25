# 09: Leave a Household or remove a member

**What to build:** `DELETE /households/{household_id}/members/{user_id}` removes a member, or makes the caller leave when the ID is their own. The last member can't leave. Spec: `../spec.md`, "Households: members", stories 15–19.

**Blocked by:** 06

**Status:** claimed

- [ ] `RemoveMember { UserID }` → `household.memberremoved { userid }`, with whoever removed them as `actorid`:
  - a User who isn't a member gets 404;
  - the only member gets 409 `last-member`
- [ ] Saving the fold deletes both the `household_members` row and the `user_households` row. Access ends with the next request
- [ ] Nothing else changes: the removed User stays a User, their earlier events keep naming them, Invitations they issued stay valid, and they can rejoin through a new Invitation
- [ ] It answers 204. Delete the stub
- [ ] Add a "Remove member / leave" row to `docs/architecture/consistency.md`, leaving its deferred column for ticket 10
- [ ] Decider tests: remove, leave, last member refused, non-member refused, rejoin after removal. HTTP tests:
  - remove another member, who then gets 404 on the Household and lands with zero Households;
  - leave;
  - the last member gets 409;
  - an Invitation from a removed member still works;
  - rejoin;
  - another Household gets 404
