# 06: Join a Household as an existing User

**What to build:** `POST /memberships` lets a User redeem an Invitation to a Household and become its member. This ticket builds the shared Join Household command that ticket 07 reuses for Unregistered callers. Spec: `../spec.md`, "Households: redeeming into a Household", stories 7–10.

**Blocked by:** 04

**Status:** resolved

- [ ] Join Household runs in one database transaction:
  1. In instance scope, resolve the caller and look up the code hash to find the Invitation and its Household. Nothing found, or the bootstrap Invitation, gets 422 `invalid-invitation`.
  2. Set the Household.
  3. The Household Decider's `AddMember { UserID, InvitationID }` records `household.memberadded`, which gains an optional `invitationid`. An existing member is refused with 409 `already-a-member`, which rolls back the transaction and leaves the code unused.
  4. The Invitation Decider redeems it.
  5. Save the folds, including the `user_households` index.
- [ ] Every invalid code gets the same 422: unknown, expired, revoked, redeemed, and the bootstrap code
- [ ] An Unregistered caller gets 403 `registration-required`
- [ ] It answers 201 with the joined `Household`. Delete the `createMembership` stub
- [ ] Add a "Join Household" row to `docs/architecture/consistency.md`, leaving its deferred column for ticket 10
- [ ] Decider tests: add a member; an existing member refused. HTTP tests:
  - a User with their own Household joins a second one, `GET /households` lists both, and the new member sees the existing Accounts;
  - an existing member gets 409, and the code then works for another User;
  - every invalid-code case gets the same 422;
  - two concurrent redemptions of one code succeed exactly once;
  - an Unregistered caller gets 403

## Comments

2026-11-24, implementation notes:

- **Decider:** new command `AddMember { UserID, InvitationID }`. It records `HouseholdMemberAdded`, which gains `invitationid` (`omitzero`, so it is absent for a creator and on old events). It refuses `ErrAlreadyAMember` for an existing member and `ErrHouseholdNotCreated` for a Household that was never created. The fold schema version is unchanged, because the fold doesn't store the Invitation.
- **Lookup:** `Lookups.InvitationByCode` now returns `app.InvitationCode { InvitationID, HouseholdID }`. `HouseholdID` is `uuid.Nil` for the bootstrap Invitation or an operator's. `Register` now refuses a code that names a Household with `ErrInvalidInvitation` up front; before this it failed the same way, through the instance-scope load. **Ticket 07** replaces that refusal with the call to `join`.
- **Join Household:** `Users.Join` (in `app/memberships.go`) resolves the User and the code in instance scope, sets the actor, then calls `Users.join`. `join` sets the transaction to the Invitation's Household, runs `AddMember` and then `RedeemInvitation` at `tx.Now()`, and returns the Household from the `AddMember` result. **Ticket 07** can call `join` from `Register` after the User Decider. `Users` now takes a `HouseholdAggregate`. A refusal maps to `app.ErrAlreadyAMember` (409), `ErrInvalidInvitation` (422) or `ErrRegistrationRequired` (403).
- **Clock:** `wiring.NewUsers(pool, now)` now takes the clock, and its store uses `eventstore.WithClock(now)`. `NewContexts` passes the injected clock. Bootstrap and `apitest`'s seeding service pass `time.Now`.
- **HTTP:** `CreateMembership` is in `adapters/http/memberships.go`, on `Handler`. The stub is deleted. It calls `httpapi.ResponseBelongsTo` with the joined Household, so an Idempotency-Key replay is stored in that Household's scope, as for `createHousehold`. `createRegistration` and `createMembership` now share one `invalidInvitation()` problem, whose detail mentions "revoked".
- **Consistency map:** added a "Join Household (`createMembership`)" row with its deferred column left as "—" for ticket 10. Ticket 07 should extend the row to `createRegistration` with a Household code.
- **Tests:** Decider tests for adding a member, an existing member refused and a Household never created refused. HTTP tests in `memberships_integration_test.go`:
  - joining a second Household, after which both are listed, the existing Account is visible and the Invitation is no longer open;
  - 409 for an existing member, with the code still open and then working for another User;
  - the same 422 body for unknown, expired (clock moved 24 hours), redeemed and bootstrap codes;
  - four concurrent redemptions with exactly one 201;
  - 403 for an Unregistered caller, with the code left open.

  The adapter test for the lookup row now checks the Household through `InvitationByCode` instead of raw SQL.
- **Left for the merge with ticket 05:** add a revoked code to `TestEveryInvalidCodeGetsTheSameProblem`. A comment marks the spot. `asInvalidInvitation` must also map ticket 05's revoked refusal, if 05 adds a new domain error.
- **Merge with ticket 08:** expect conflicts in `HouseholdMemberAdded`/`evolveHousehold`, where 08 adds `JoinedAt`, and in `HouseholdFolds.Save`, where 08 adds `joined_at`.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.

2026-11-25, review fix (an existing member's unusable code):

- **Bug:** `join` ran `AddMember` before the Invitation's Redeem checked the code, so a member of the code's Household got 409 `already-a-member` ("The Invitation code is still unused") for a revoked, expired or redeemed code. That revealed a revoked code (ADR-0025, spec story 9), and the 409 text was false.
- **Fix:** in `app/memberships.go`, `join` no longer returns on `domain.ErrAlreadyAMember`. It still runs Redeem after `AddMember`, so the event order is unchanged (memberadded, then invitation.redeemed). A code that can't be redeemed gets `ErrInvalidInvitation` (422) whoever sends it. Only when Redeem succeeds for a member does `join` return `ErrAlreadyAMember` (409). The redeemed event is then rolled back with the transaction, so the code stays unused, as `TestJoiningAHouseholdYouBelongToLeavesTheCodeUnused` still shows.
- **Test:** `TestAMemberWithAnInvalidCodeForTheirHouseholdGetsTheSameProblem` checks that a member gets the same 422 body as for an unknown code when the code is revoked, expired, redeemed by another User, or redeemed by themselves.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
