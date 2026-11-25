# Invitations to a Household: any member admits people to the instance and to their Household

Any member of a Household can issue an Invitation to that Household. Whoever redeems it becomes a Member of it: someone Unregistered becomes a User and a Member in one step, and an existing User joins one more Household. This amends ADR-0017, which kept admission with the operator and the Instance administrator and left this lifecycle for later. An Invitation to a Household therefore also **admits a new person to the instance**, so admission is delegated to the members of every Household. We accept that for a self-hosted family instance (ADR-0005), where the people sharing books are the people the operator would admit anyway. A hosted service must revisit it.

## One Invitation concept, in Household scope

An Invitation optionally names a Household. The bootstrap Invitation names none, grants Instance administrator and stays in instance scope, unchanged. An Invitation to a Household grants role `user` and lives in **Household scope** (ADR-0014): its events carry the `householdid`, its fold sits under the usual row-level security, and issuing, listing and revoking it are ordinary Household commands. `invitation.issued` records the issuing member as `actorid`.

Only the **code lookup row** (code hash → Invitation, now with a nullable Household) stays in instance scope, because the person redeeming a code doesn't know its Household yet. **Join Household** is the command that spans both scopes, in the order ADR-0017 foresaw, in one database transaction: look up the code and resolve or register the User in instance scope, then set the Household, add the Member, and mark the Invitation redeemed. Adding the Member runs before the redemption, so redeeming into a Household you already belong to is refused and leaves the code unused.

## Lifetime, bearer codes and revocation

- **24 hours, no reissue.** An Invitation to a Household expires 24 hours after it is issued, like the bootstrap code. It is never reissued: each issue is a new Invitation with its own code. The code is shown once, to the member who issued it, and never appears in a listing.
- **Bearer codes.** Whoever holds the code can redeem it. We kept ADR-0017's reasons: nobody knows an invitee's OIDC subject before they sign in, and an email claim is only trustworthy when `email_verified` is set, which a replaceable provider (ADR-0010) may not give. A short lifetime and revocation limit what a leaked code can do.
- **Revocation.** Any member can revoke any open Invitation of their Household, including one another member issued. A revoked code gets the same error as an unknown, expired or redeemed one.
- **Issuers who leave.** An Invitation stays valid when the member who issued it leaves or is removed, until it expires or is revoked. It was issued on the Household's behalf, and every remaining member can see and revoke it.

## Members are equal

Every Member is equal (ADR-0003): there are no owners or roles inside a Household. Any member can remove any other member, and leaving is removing yourself. `household.memberremoved` records who removed whom through `actorid`. A Household always has at least one Member, so the last member can't leave: otherwise nobody could ever reach its books again. Joining, leaving and removal each raise a Notification every member sees, so removal never happens silently. A removed User stays a User, can create a new Household and can rejoin through a new Invitation.

## Display names are personal data

Every User now has a display name, typed by them when they register and changeable later. `user.registered` and `user.renamed` put it in the permanent event store, next to the OIDC issuer and subject. ADR-0017's caveat applies to it too: that is acceptable while instances are self-hosted, and erasure must be settled before a hosted service.

## Considered Options

- **Two codes**, one admitting to the instance and one to a Household. Rejected: an invitee who is new to the instance would need both, from two different people, to join the books they were invited to share.
- **Only Instance administrators issue Invitations.** Rejected: on a family instance it makes the administrator a bottleneck for every Household, and it would still need a separate step to bring the new User into the Household.
- **A separate Household invitation concept**, beside the Invitation that admits to the instance. Rejected: it would duplicate the code, hashing, expiry, single use and uniform error that the Invitation already has, and it would still have to register someone Unregistered, which is the Invitation's job.

## Consequences

- Any member can let a stranger into the instance and can remove any other member. Both are trust choices a future hosted service (ADR-0005) should revisit.
- An Invitation's events are split across scopes: the bootstrap Invitation's are instance-level, and every other Invitation's are Household events.
- Requiring a display name on registration broke the UI's `/register` form until the UI added the field.
