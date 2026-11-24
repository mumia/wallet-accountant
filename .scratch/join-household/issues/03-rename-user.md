# 03: Change your display name

**What to build:** `PATCH /me` sets the caller's display name. It also gives Users who registered before display names existed a way to set one. Spec: `../spec.md`, "Households: User display name", stories 12–13.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `RenameUser` → `user.renamed { displayname }`, registered on the codec. The name is trimmed; a blank one gets 422; the same name records nothing
- [ ] It runs in instance scope on the caller's own User. Delete the `changeMe` stub
- [ ] Add a "Rename User" row to `docs/architecture/consistency.md`
- [ ] Decider tests, plus HTTP tests:
  - rename;
  - setting a name for a legacy User with none;
  - only spaces (422), and 65 characters (400 from the schema);
  - the same name recording no event;
  - `rebuild` keeps the name

## Comments

2026-11-24, implementation notes:

- **Domain:** `RenameUser { DisplayName }` → `UserRenamed` (`user.renamed.v1`, `{ displayname }`). The name is trimmed; a blank one is refused with `ErrDisplayNameRequired` (422 `/problems/invalid-user`); the same name records nothing. A rename of a stream with no `user.registered` is refused with a new `ErrUserNotRegistered`, which the API can't reach.
- **Fold:** its shape is unchanged, so the User fold's schema version stays 2.
- **App:** `Users.Rename` resolves the caller's User by identity in instance scope (unregistered gets 403 `registration-required`) and records the event with the User as `actorid`.
- **HTTP:** `ChangeMe` replaces the `pending.go` stub. The blank-name problem is now shared with `createRegistration`.
- **Legacy Users in tests:** `apitest.RegisterWithoutDisplayName` registers a User, then as `wa_owner` strips `displayname` from its `user.registered` event and nulls `users.display_name`. The legacy test rebuilds first, so the User is refolded from a name-less event, then sets a name.
- **Tests:** Decider tests for rename, trimming, a legacy User setting a name, the same name, a blank name and an unregistered User. HTTP tests for rename (`PATCH` and `GET /me`), only the caller renamed, the legacy User, only spaces (422, no event), missing/empty/65 characters (400), the same name (200, no event), rebuild keeping the name, unregistered (403) and no token (401).
- **Not here:** the "members list reflects the rename" check from the spec's HTTP tests belongs with ticket 08 (`listMembers`).
- **Checks:** `make lint`, `make test` and `make test-integration` pass.
