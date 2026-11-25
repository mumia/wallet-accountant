# 07: Register and join a Household with one code

**What to build:** someone Unregistered redeems an Invitation to a Household at `POST /registrations` and becomes a User and a member in one transaction. The bootstrap code keeps working as before. Spec: `../spec.md`, "Households: redeeming into a Household" (step 2), story 6.

**Blocked by:** 06

**Status:** claimed

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
