# 03: Bootstrap and registration

**What to build:** the first person on a new instance becomes a User and Instance administrator. The operator runs `make dev-bootstrap` (or `make bootstrap`) to get a one-time Invitation code. The person signs in and redeems the code with `POST /registrations`. `GET /me` then returns the User and that they are Instance administrator. Spec: `../spec.md` stories 1–2d, 66a–66c, "Registration (bootstrap only)"; ADR-0017.

**Blocked by:** 02 (Event store, Decider runtime and event forwarding)

**Status:** resolved

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

## Comments

Implementation notes (implementer agent):

- **Where things live.** Deciders in `internal/households/domain` (`invitation.go`, `user.go`); the service `app.Users` (`Me`, `Register`, `IssueBootstrapInvitation`, `IssueInvitation`) and the aggregates' codecs and event types in `internal/households/app`; fold stores and lookups in `internal/households/adapters/postgres`; tables in migration `000002_users_and_invitations`. `wiring.NewUsers(pool)` builds the service; both aggregates are registered as rebuild targets.
- **Roles.** An Invitation grants `instance_administrator` or `user` (no instance role). Seeded test Invitations grant `user`. The bootstrap Invitation's fixed ID is `00000000-0000-7000-8000-000000000001`.
- **Codes.** 32 bytes from `crypto/rand`, base64url (43 characters); the SHA-256 of the code string is stored hex-encoded. The Decider compares hashes in constant time. A reissue replaces the lookup row, so a superseded code is simply unknown.
- **One generic error.** Every invalid code gets 422 with the problem type `/problems/invalid-invitation` and the same title and detail; an already registered caller gets 409 `/problems/already-registered`. The identity lookup row's primary key is the backstop for one identity redeeming two codes at once: its upsert affects no row for another User, and the fold store returns `app.ErrAlreadyRegistered`.
- **422 in the Zalando ruleset.** Zalando's list of standard status codes has no 422, so `api/zalando-ruleset.yaml` now allows it, with a comment. The spec uses 422 here and for Idempotency-Key mismatches (ticket 04).
- **Shared kernel ID.** depguard lets a domain import only the standard library and the shared kernel, so `internal/sharedkernel/id.go` adds `ID` (an alias of `uuid.UUID`), `NilID`, `NewID` and `MustParseID`. The `domain` depguard rule also matched the domain's own external tests (`package domain_test`), which must import the domain they test, so the four context domains are now allowed; the per-context rules still deny cross-context imports.
- **Bootstrap.** `bootstrap` needs only `WA_DATABASE_URL` (as `wa_app`). It prints the code with `fmt` to `Options.Stdout` (default `os.Stdout`) and logs only "bootstrap Invitation issued" with the expiry. Once redeemed, it fails with `app.ErrBootstrapRefused`. `make dev-bootstrap` runs it; `make bootstrap` (Compose) is left to ticket 17. `ErrModeNotAvailable` is gone, since every run mode now exists.
- **Harness.** `apitest.App` gained `SeedInvitation`, `SeedInvitationWith(role, expiresAt)` (an expiry in the past gives an expired code), `Register(issuer, subject)` (returns a token), `RunBootstrap` (returns the printed code), `Rebuild`, `Post` and `Response.AssertJSON`.
- **Tests outside the agreed seams.** One PostgreSQL-level test (the spec's required exception) shows that the four Users and Invitations tables can't be read or written while a Household is set. Mutation checks done by hand: removing the identity backstop fails the two-codes race test, and logging the code fails the bootstrap test.
