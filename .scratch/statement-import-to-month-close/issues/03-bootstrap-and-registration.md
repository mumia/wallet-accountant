# 03: Bootstrap and registration

**What to build:** the first person on a new instance becomes a User and Instance administrator. The operator runs `make dev-bootstrap` (or `make bootstrap`) to get a one-time Invitation code. The person signs in and redeems the code with `POST /registrations`. `GET /me` then returns the User and that they are Instance administrator. Spec: `../spec.md` stories 1–2d, 66a–66c, "Registration (bootstrap only)"; ADR-0017.

**Blocked by:** 02 (Event store, Decider runtime and event forwarding)

**Status:** ready-for-agent

- [ ] Invitation Decider (instance scope): `issued`, `reissued`, `redeemed`. Redeeming is refused for a wrong code, an expired code (judged by the command's timestamp) or one already redeemed. The fold keeps a lookup row from code hash to Invitation
- [ ] User Decider (instance scope): `registered`, with OIDC issuer and subject, the redeemed Invitation and the granted role. The fold keeps the Instance administrator flag and a lookup row from issuer and subject to User, whose unique constraint stops one identity from registering twice
- [ ] Users and Invitations fold and lookup tables are admitted only in instance scope
- [ ] The `bootstrap` run mode issues the fixed-ID bootstrap Invitation: 256 random bits, stored as a SHA-256 hash, valid 24 hours, printed with `fmt` and never logged. A rerun before redemption reissues it with a new code; once redeemed, `bootstrap` is refused
- [ ] Redeeming runs the Invitation and User Deciders in one database transaction, so two concurrent redemptions can't both succeed
- [ ] `POST /registrations` answers 201 with the User, 422 with one generic error for an unknown, expired, superseded or already-used code, and 409 if the caller is already registered
- [ ] `GET /me` returns the User and whether they are Instance administrator; an unregistered caller still gets 403 `registration-required`
- [ ] A test helper seeds Invitations so tests can register several identities
- [ ] Decider tests: issue, reissue, and every redemption refusal
- [ ] HTTP tests: every invalid-code case, double redemption, concurrent redemption of one code
- [ ] Integration test: `bootstrap` is refused once an Instance administrator exists, a rerun replaces the earlier code, and the code appears in no log record
- [ ] Consistency map rows for Issue bootstrap Invitation and Redeem Invitation match the code
