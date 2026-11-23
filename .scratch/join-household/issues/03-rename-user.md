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
