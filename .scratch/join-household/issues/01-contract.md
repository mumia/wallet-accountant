# 01: Contract: the joining operations in `api/openapi.yaml`

**What to build:** write the contract the spec settled, so the UI can pin its `openapi.yaml` to the merge commit. Every new operation is wired and answers 501 until its ticket lands. The one behaviour change is the required `display_name` on `Registration`, which is recorded from now on. Spec: `../spec.md`, "Contract" and "Households: User display name".

**Blocked by:** —

**Status:** ready-for-agent

- [ ] New operations: `createMembership`, `listMembers`, `removeMember`, `createInvitation`, `listInvitations`, `revokeInvitation`, `changeMe`, with their request and response schemas
- [ ] Changed schemas: `Registration.display_name` (required, 1–64 characters), `Me.display_name` (nullable), and `Notification.kind` gains `MEMBER_JOINED`, `MEMBER_LEFT` and `MEMBER_REMOVED`, with nullable `member` and `removed_by`, each `{ user_id, display_name }`
- [ ] New problem types: `/problems/already-a-member` (409) and `/problems/last-member` (409). `createMembership` documents the generic 422 `invalid-invitation` and the 403 `registration-required`
- [ ] `createRegistration`'s description says that a Household code also makes the caller a member, and that `display_name` is new and required (a breaking change for the UI's `/register`)
- [ ] `make generate`, with the generated ogen code and Bruno collection committed
- [ ] Each new operation answers 501 `/problems/not-implemented` through a `pending.go` stub, which its ticket deletes
- [ ] `RegisterUser` and `user.registered` carry `displayname`, trimmed, with a blank one refused (422). The User fold and `GET /me` return it, and it is null for Users registered before this change. Existing tests send names
- [ ] `make lint` and `make test-integration` pass
- [ ] After merge: annotated tag `v0.4.0` on the merge commit, then tell the UI repo
- [ ] Do not change the contract's shape: anything that seems wrong is a comment on the spec
