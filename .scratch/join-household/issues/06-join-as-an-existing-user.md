# 06: Join a Household as an existing User

**What to build:** `POST /memberships` lets a User redeem an Invitation to a Household and become its member. This ticket builds the shared Join Household command that ticket 07 reuses for Unregistered callers. Spec: `../spec.md`, "Households: redeeming into a Household", stories 7–10.

**Blocked by:** 04

**Status:** ready-for-agent

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
