# 07: Register and join a Household with one code

**What to build:** someone Unregistered redeems an Invitation to a Household at `POST /registrations` and becomes a User and a member in one transaction. The bootstrap code keeps working as before. Spec: `../spec.md`, "Households: redeeming into a Household" (step 2), story 6.

**Blocked by:** 06

**Status:** resolved

- [ ] `createRegistration` with a Household code:
  - looks the code up in instance scope;
  - registers the User with role `user` and the given display name, and saves the identity lookup row;
  - then runs ticket 06's Join Household steps
- [ ] Instance-level events are written before Household events, in one transaction (ADR-0017). Any refusal rolls back the registration too
- [ ] It answers 201 with `Me`, as today. An identity that is already a User still gets 409 `already-registered`
- [ ] Update the "Redeem Invitation" row in `docs/architecture/consistency.md` to cover a Household code
- [ ] HTTP tests:
  - an Unregistered person redeems a Household code, then `GET /households` shows the Household and its Accounts;
  - an expired or revoked Household code gets 422 and no User is created;
  - two identities racing for one code: exactly one becomes a User and member;
  - bootstrap redemption is unchanged apart from the display name

## Comments

2026-11-25, implementation notes:

- **App:** `Users.Register` no longer refuses a Household code up front. For a code naming a Household it runs `RegisterUser` with role `user` (Household Invitations are always issued with that role, and their stream is in Household scope, so it isn't read before the transaction moves there), then calls `Users.join` with the same transaction. The User's instance-level events are written before `join` sets the transaction to the Household (ADR-0017), and any refusal from `join` rolls the registration back. A bootstrap or operator code takes the old path unchanged. `ErrAlreadyAMember` can't happen for a brand-new User, so the handler needs no new mapping.
- **HTTP:** unchanged. A registration still answers `Me`, so its Idempotency-Key replay stays in the caller's scope rather than the Household's.
- **Consistency map:** the "Redeem Invitation" row now names `createRegistration`, the display name, and both code kinds; the "Join Household" row names `createRegistration` with a Household code too.
- **Tests** (`registration_integration_test.go`): registering with a Household code then seeing the Household and its Account, the Invitation no longer open, and 409 `already-registered` on a second code; unknown, expired (clock moved 24 hours) and revoked codes all get the same 422 with no User and no new member; four identities racing for one code, with exactly one registered and a member. The bootstrap test in `internal/wiring` now also checks the display name and that the registrant has no Households.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
