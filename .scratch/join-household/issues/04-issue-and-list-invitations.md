# 04: Issue and list Invitations to a Household

**What to build:** any member can issue an Invitation to their Household and list the open ones. The Invitation lives in Household scope, and only its code lookup row stays in instance scope. Spec: `../spec.md`, "Households: Invitation in Household scope", stories 1–4.

**Blocked by:** 01

**Status:** resolved

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

## Comments

2026-11-24, implementation notes:

- **Migration 000022** (`household_invitations`). First numbered 000023 to leave 000022 for ticket 08's `joined_at` backfill; renumbered to 000022 on merge into `join-household` so there's no gap. Ticket 08 takes the next free number after it.
  - `household_invitations`: `household_id`, `code_hash`, `issued_by`, `issued_at`, `expires_at`, `redeemed_by`/`redeemed_at`, `revoked_at` (written as NULL until ticket 05 adds the revoke), with forced row-level security and the usual policy. wa_app gets no DELETE.
  - `invitation_codes` gains a nullable `household_id`, and loses its foreign key to `invitations`, since its row may now point at either table.
- **Decider:** new command `IssueHouseholdInvitation { CodeHash, At }`, which records the existing `InvitationIssued` with role `user`, `expiresat = At + CodeLifetime` and no `issuedby`. A stream that is already issued refuses it with `ErrInvitationAlreadyIssued`, so there is never a reissue. Evolve takes `HouseholdID`, `IssuedBy` (actorid) and `IssuedAt` from the envelope, only for an Invitation to a Household. The fold schema version is unchanged: the bootstrap fold doesn't store these fields.
- **Fold store:** `InvitationFolds` picks the table by the transaction's scope. Set to a Household, it uses `household_invitations` and then writes the lookup row through `inInstanceScope`. In instance scope it uses `invitations` as before. So rebuild, which sets each stream's Household, regenerates both. Ticket 06 must set the transaction to the lookup row's Household before it loads or redeems an Invitation to a Household. `Lookups.InvitationByCode` doesn't return `household_id` yet; 06 needs it.
- **App/HTTP:** new `app.Invitations` service (`Issue`, `ListOpen`), with an `InvitationReader` port and an `app.UserRef` type, plus a separate `InvitationHandler` embedded in `apiHandler`, so `NewHandler` stays untouched for 03 and 08. The issuer's display name is read in instance scope by `displayNames` in `adapters/postgres/invitations.go`. Ticket 08 may want to share it. Both stubs are deleted.
- **Tests:** Decider tests for issuing, never reissuing and expiry at exactly 24 hours. HTTP tests for issue then list (codes never listed, oldest first, `issued_by` with display name), expiry by moving the clock, another Household getting 404 on both, Idempotency-Key replay returning the same code, and surviving a rebuild. Adapter tests for row-level security on `household_invitations` and on the lookup row with a Household, plus one that issues through the fold store in Household scope. It checks that the fold is invisible from another Household and from instance scope, that the lookup row is visible only from instance scope and names the Household, and that a rebuild restores a lost lookup row.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
