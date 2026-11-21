# 13: Rename an Account

**What to build:** `PATCH /households/{household_id}/accounts/{account_id}` renames an Account; its kind, currency and start date stay fixed. Spec: `../spec.md`, "Ledger: Account Decider and fold" (`AccountRenamed`), story 26.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `RenameAccount` → `AccountRenamed` (registered); trim; blank → 422 `invalid-account`; the same name records nothing
- [ ] The Account lookup row's name updated
- [ ] Decider test plus HTTP tests: rename, blank, 201 characters, the same name

## Comments
