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

## Comments

2026-11-25, implementation notes:

- **Decider:** `RemoveMember { UserID }` records `household.memberremoved { userid }` (`HouseholdMemberRemovedType`, registered in `NewHouseholdAggregate`). The spec's `RemovedBy` isn't in the command: the remover is the event's `actorid`, which `InHousehold` sets to the caller, and the Decider needs nothing else. Refusals: `ErrNotAMember` (also for a Household never created) and `ErrLastMember`. Evolve drops the User from `Members` and from `JoinedAt`, so a rejoin records its new time. The fold schema version is unchanged: the fold's shape is the same.
- **Fold store:** unchanged. `HouseholdFolds.Save` already deletes the `household_members` and `user_households` rows of anyone no longer in `Members`.
- **App:** `Households.RemoveMember(ctx, household, user)` runs in `InHousehold`, so a non-member caller or an unknown Household gets `ErrHouseholdNotFound`; a target who isn't a member maps to `app.ErrMemberNotFound` (404), the only member to `app.ErrLastMember` (409).
- **HTTP:** `RemoveMember` is in `adapters/http/members.go`, with `LastMemberType`. `pending.go` is deleted, since this was its last stub. A 404 here is the shared `about:blank` not-found problem, like every other Household route.
- **Test helper:** `apitest.App.EventActors(t, eventType)` reads each event's `actorid`, so the tests check who removed whom.
- **Consistency map:** added a "Remove member / leave (`removeMember`)" row, deferred column left as "—" for ticket 10.
- **Tests:** Decider tests for remove, leave, rejoin after removal, a rejoin's new joining time, and refusals (last member, last member after the other left, non-member, already removed, Household never created). HTTP tests in `remove_member_integration_test.go`: removing another member (actor is the remover; the removed User gets 404 on the Household and its members, has zero Households, is still a User and can create a Household); leaving; last-member 409 recording nothing, also after the other member left; non-member target 404; another Household's members 404; an Invitation issued by a removed member still works; rejoin with the new `joined_at`, also after a rebuild. Members are added through `POST /memberships`.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
