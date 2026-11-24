# 05: Revoke an Invitation

**What to build:** any member can revoke an open Invitation to their Household, including one another member issued. A revoked code then gets the same generic error as any invalid one. Spec: `../spec.md`, "Households: Invitation in Household scope", story 5.

**Blocked by:** 04

**Status:** claimed

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
