# 04: Issue and list Invitations to a Household

**What to build:** any member can issue an Invitation to their Household and list the open ones. The Invitation lives in Household scope, and only its code lookup row stays in instance scope. Spec: `../spec.md`, "Households: Invitation in Household scope", stories 1–4.

**Blocked by:** 01

**Status:** claimed

- [ ] Migration:
  - a Household-scoped Invitation fold table with `household_id`, `FORCE ROW LEVEL SECURITY` and the usual policy, holding issued by, issued at, expires at, redeemed and revoked;
  - a nullable `household_id` on the instance-scope code lookup row, which stays null for the bootstrap Invitation
- [ ] `invitation.issued` for a Household:
  - it carries the `householdid` envelope and the issuing User as `actorid`, with no `issuedby`;
  - its role is always `user`, and it expires after `CodeLifetime`;
  - each issue is a new stream, never a reissue
- [ ] The fold store writes the Household row, then switches to instance scope for the lookup row (the `inInstanceScope` pattern). `rebuild` regenerates both
- [ ] `createInvitation` answers 201 with `{ invitation_id, code, expires_at }` and takes an Idempotency-Key. A replay returns the same code
- [ ] `listInvitations` returns open Invitations only (not redeemed, not revoked, `expires_at` later than the clock), oldest first. It never returns a code
- [ ] A caller who isn't a member gets 404 on both. Delete both stubs
- [ ] Add an "Issue Invitation to a Household" row to `docs/architecture/consistency.md`
- [ ] Decider tests, plus HTTP tests:
  - issue then list;
  - an expired Invitation drops out of the list after moving the clock past 24 hours;
  - another Household gets 404;
  - Idempotency-Key replay
- [ ] Adapter-level row-level security test: Invitation fold rows are invisible from another Household and from instance scope, and the lookup row is invisible from Household scope
