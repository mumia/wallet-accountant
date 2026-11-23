# 01: Contract: the joining operations in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. Every new operation is wired and answers 501 until its ticket lands. The one behaviour change is the required `display_name` on `Registration`, which is recorded from now on. Spec: `../spec.md`, "Contract" and "Households: User display name".

**Blocked by:** —

**Status:** resolved

- [x] New operations: `createMembership`, `listMembers`, `removeMember`, `createInvitation`, `listInvitations`, `revokeInvitation`, `changeMe`, with their request and response schemas
- [x] Changed schemas: `Registration.display_name` (required, 1–64 characters), `Me.display_name` (optional), and `Notification.kind` gains `MEMBER_JOINED`, `MEMBER_LEFT` and `MEMBER_REMOVED`, with optional `member` and `removed_by`, each `{ user_id, display_name }`
- [x] New problem types: `/problems/already-a-member` (409) and `/problems/last-member` (409). `createMembership` documents the generic 422 `invalid-invitation` and the 403 `registration-required`
- [x] `createRegistration`'s description says that a Household code also makes the caller a member, and that `display_name` is new and required (a breaking change for the UI's `/register`)
- [x] `make generate`, with the generated ogen code and Bruno collection committed
- [x] Each new operation answers 501 `/problems/not-implemented` through a `pending.go` stub, which its ticket deletes
- [x] `RegisterUser` and `user.registered` carry `displayname`, trimmed, with a blank one refused (422). The User fold and `GET /me` return it, and it is absent for Users registered before this change. Existing tests send names
- [x] `make lint` and `make test-integration` pass
- [ ] After merge: annotated tag `v0.4.0` on the merge commit, then tell the UI repo
- [x] Do not change the contract's shape: anything that seems wrong is a comment on the spec

## Comments

2026-11-23, implementation notes:

- **Contract:**
  - **New tags:** `Members` and `Invitations`.
  - **New path parameters:** `UserID` and `InvitationID`.
  - **New schemas:** `DisplayName` (input, 1–64), `DisplayNameOut` (output), `MeChange`, `InvitationCode` (shared by `Registration` and the new `MembershipCreation`), `Member`, `MemberList`, `UserRef`, `IssuedInvitation`, `Invitation` (`issued_by` is a `UserRef`) and `InvitationList`.
  - **No-content responses:** `removeMember` and `revokeInvitation` answer 204, the contract's first.
  - **Lint:** vacuum scores 100.
- **Deviations from the spec's wording, now reflected in the spec:**
  - **Absent, not null:** optional fields are left out rather than sent as `null`, as in the rest of the contract (`parent_id` is the one exception, and it means "move to root"). So `Me.display_name`, `Member.display_name`, `Notification.member` and `Notification.removed_by` are absent when there's nothing to say.
  - **Notification fields:** `Notification`'s `statement_import_id`, `connection_id`, `account_id` and `suspected_duplicates` are no longer required, because a member Notification has none of them. The UI's generated type changes for them too.
  - **Display-name problems:** a display name of only spaces gets 422 `/problems/invalid-user`, and an empty or too long one 400 from the schema, the same split as Account names. The spec had said 400 for a blank one.
- **Display name:**
  - `RegisterUser.DisplayName` is trimmed, and a blank one is refused with `domain.ErrDisplayNameRequired`.
  - `UserRegistered.displayname` uses `omitempty`, so old events decode with no name.
  - The User fold's schema version is now 2, so stored folds refold from their events.
  - Migration 000021 adds a nullable `users.display_name`, written as NULL when there is no name. `Lookups.UserByIdentity` returns it, so `GET /me` and `createRegistration` answer with it.
- **501 stubs:** `internal/households/adapters/http/pending.go`, one per new operation, each naming its ticket.
- **Tests:**
  - Every registration body in the tests and in `apitest.Register` sends `display_name`.
  - New Decider tests: trimming, a blank name refused, and no name from a legacy event.
  - New HTTP tests: the name returned by registration and `GET /me`, trimming, only spaces getting 422 with the code still usable afterwards, and missing, empty or 65 characters getting 400.
- **Checks:** `make lint` (golangci-lint 0 issues, vacuum 100) and `go test -tags integration ./...` pass.
