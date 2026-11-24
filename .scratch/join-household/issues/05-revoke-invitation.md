# 05: Revoke an Invitation

**What to build:** any member can revoke an open Invitation to their Household, including one another member issued. A revoked code then gets the same generic error as any invalid one. Spec: `../spec.md`, "Households: Invitation in Household scope", story 5.

**Blocked by:** 04

**Status:** resolved

- [ ] `RevokeInvitation` → `invitation.revoked`, registered on the codec:
  - refused once the Invitation is redeemed or already revoked;
  - allowed when it has expired but was never redeemed;
  - redeeming a revoked Invitation is refused through the same error path as a wrong code
- [ ] `revokeInvitation` answers 204. An `invitation_id` that isn't an open Invitation of this Household, or a caller who isn't a member, gets 404. Delete the stub
- [ ] A revoked Invitation drops out of `listInvitations`
- [ ] Add a "Revoke Invitation" row to `docs/architecture/consistency.md`
- [ ] Decider tests, plus HTTP tests:
  - revoke, then it's gone from the list;
  - revoking twice gets 404;
  - another member's Invitation can be revoked;
  - another Household's Invitation gets 404

## Comments

2026-11-24, implementation notes:

- **No migration:** ticket 04's `household_invitations.revoked_at` already exists. The Household Invitation fold store now reads and writes it. The fold schema version is unchanged, since no stored fold can be revoked yet.
- **Decider:** `RevokeInvitation{}` records `InvitationRevoked{}` (no data; the revoking member is the envelope's actor and `RevokedAt` comes from its time). It's refused with `ErrInvitationNotIssued`, `ErrInvitationAlreadyRedeemed` or the new `ErrInvitationRevoked`. It judges no expiry, so an expired but unredeemed Invitation can be revoked. Registered on the codec as `io.walletaccountant.households.invitation.revoked.v1`.
- **Redeem:** one new case, `case s.Revoked(): return nil, ErrWrongInvitationCode`, right after the code check. It returns the existing wrong-code error, so the app's mapping to `invalid-invitation` needs no change. This is the only line in Redeem that ticket 06 will need to merge around.
- **Expired at the HTTP level:** the contract's description of `revokeInvitation` says an expired Invitation gets 404, like a redeemed or revoked one. So `app.Invitations.Revoke` first checks that the Invitation is open with `InvitationReader.IsOpen` (same rule as the list, by the transaction's clock), and answers `ErrInvitationNotFound` (404) otherwise. The Decider's "expired may be revoked" rule is still what the domain allows; the API just doesn't expose it. Unknown IDs and another Household's Invitations also get 404, because the fold isn't found in this Household's scope.
- **Test seam:** joining (ticket 06) isn't there yet, so the "another member's Invitation" HTTP test uses a new `apitest.App.SeedMember`. It writes `household_members` and `user_households` rows as wa_owner, with no event, so a rebuild would drop it. Once 06 lands, that test can join through `/memberships` and `SeedMember` can go.
- **Tests:** Decider tests for revoking, the three refusals and redeeming a revoked one. HTTP tests for revoke then list (also after a rebuild), revoking twice getting 404, another member revoking, another Household (as a non-member and through the caller's own Household) getting 404, and an unknown or expired Invitation getting 404.
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
