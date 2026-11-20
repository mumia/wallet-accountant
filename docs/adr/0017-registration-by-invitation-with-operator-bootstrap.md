# Registration by Invitation, with the first Invitation issued by the operator

Signing in through the OIDC provider (ADR-0010) proves who someone is. It does not admit them to the instance. A verified identity with no User is **unregistered**: it can read its own registration status and redeem an Invitation, and nothing else. A User is created only by **redeeming an Invitation**. We rejected auto-registering every authenticated subject because ADR-0010 lets any OIDC provider replace Zitadel. Behind a shared or public provider such as Google, "authenticated" would then mean "anyone". Turning off self-registration in Zitadel is still recommended as a second layer, but the app never relies on it.

## The first User

When an instance has no Instance administrator, nobody can issue Invitations. The operator solves this with the `bootstrap` run mode (ADR-0009), for example `make local-bootstrap` or `make prod-bootstrap`, which run it as a one-off pod (ADR-0019). It issues the **bootstrap Invitation**, which grants Instance administrator, and prints its code to stdout. Access to the deployment is the proof of authority. The first User then registers the same way as every later User, so there is no first-login special case. We rejected:

- **first come, first served**, because anyone who reaches a freshly started instance before its owner could claim it;
- **a setup code written to the logs**, because logs are shipped to Loki (ADR-0016), so the secret would end up in telemetry;
- **an allowlisted subject or email in an environment variable**, because nobody knows their OIDC subject before signing in, and an email claim is only trustworthy when `email_verified` is set. It remains a possible later addition for automated deploys.

The bootstrap Invitation is a single stream with a fixed, well-known ID. Running `bootstrap` again before it is redeemed reissues it with a new code and expiry, so only one bootstrap code is ever valid. Once it is redeemed, `bootstrap` is refused. Both rules are decided by that one aggregate under its version check, so concurrent runs or redemptions can't break them. The code is printed with `fmt`, never through slog or OpenTelemetry, and expires 24 hours after it is issued.

## Invitations

An Invitation is a single-use bearer code with an expiry. The code has 256 bits from `crypto/rand` and is stored only as its SHA-256 hash, which is enough for a random secret of that size. An unknown, expired, superseded or already redeemed code gets the same error, so the endpoint never reveals which codes exist. The Invitation carries the role it grants. In the first slice only the bootstrap Invitation exists.

**Instance administrator** is an instance-level role that controls who is admitted. It grants no access to any Household's data: ADR-0003 still holds, and an Instance administrator sees a Household only by being a member of it.

Every later User will also join through an Invitation, issued from inside the app. That lifecycle comes later: who may issue them, whether an Invitation can also make the invitee a member of a Household, listing, revoking and expiry settings. It will extend this ADR with new Invitation and User events.

## Event-sourced, like every other aggregate

User and Invitation are Deciders with stored folds (ADR-0011). Every change to them is an event in the event store, and `rebuild` regenerates their folds from those events.

| Aggregate | Events (`io.walletaccountant.households.<aggregate>.<event>.v1`) | Fold and lookup rows |
|---|---|---|
| Invitation | `invitation.issued` (role, code hash, expiry), `invitation.reissued` (new code hash, new expiry), `invitation.redeemed` (User ID) | State, role, expiry, redeemed by and at. One lookup row: code hash → Invitation, replaced on reissue |
| User | `user.registered` (OIDC issuer and subject, the Invitation redeemed, the role granted) | Identity and Instance administrator flag. One lookup row: issuer and subject → User, with a unique constraint |

- **Redeem Invitation** runs both Deciders in one database transaction, each with its own version check. The Invitation decides whether the code is valid, using the command's timestamp for expiry, so no expiry event is needed. The User decides the registration. If two people redeem one code concurrently, the version conflict makes one retry, and the retry finds the Invitation redeemed. If one identity redeems two codes, the unique identity lookup row rejects the second one. That constraint is the backstop for a uniqueness rule that no single aggregate can enforce.
- **Actors.** `user.registered` and `invitation.redeemed` carry the new User's ID as `actorid`. Events recorded by the `bootstrap` run mode have no User to point to: they omit `actorid` and record `issuedby: operator` in their data.
- **Personal data.** `user.registered` puts the OIDC issuer and subject in the permanent event store. That is acceptable while instances are self-hosted (ADR-0005). Erasure, for example by crypto-shredding, must be settled before a hosted service.

## Instance scope

Users and Invitations belong to the instance, not to a Household. Their events have no `householdid` (ADR-0015) and their fold tables have no `household_id`. They still sit under row-level security (ADR-0014), in **instance scope**, meaning a transaction with no Household set:

- Instance-level rows (events with a NULL Household, and the Users and Invitations fold and lookup tables) are visible and writable only while `app.household_id` is empty.
- Household rows are visible and writable only while it is set, as before.

A Household request therefore can't read or write identities, and an instance-level command can't touch Household data. A request resolves its caller's User in instance scope before it sets `app.household_id`. The handler middleware and `rebuild` treat an event without `householdid` as instance scope and leave the setting empty. A later command that must span both scopes, such as redeeming a Household-bound Invitation, writes its instance-level events first and then sets the Household, within one transaction. We rejected leaving these tables outside row-level security, because then any Household request with a missing filter could read every User's identity.
