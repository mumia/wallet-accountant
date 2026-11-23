# 13: Rename an Account

**What to build:** `PATCH /households/{household_id}/accounts/{account_id}` renames an Account; its kind, currency and start date stay fixed. Spec: `../spec.md`, "Ledger: Account Decider and fold" (`AccountRenamed`), story 26.

**Blocked by:** 01

**Status:** resolved

- [x] `RenameAccount` → `AccountRenamed` (registered); trim; blank → 422 `invalid-account`; the same name records nothing
- [x] The Account lookup row's name updated
- [x] Decider test plus HTTP tests: rename, a name of only spaces (422), 201 characters (400 from the schema), the same name

## Comments

- 2026-11-23, implementation notes:
  - **Domain:** `RenameAccount{Name}` → `AccountRenamed{Name}` (`io.walletaccountant.ledger.account.renamed.v1`, registered on the codec). It trims the name, refuses a blank one with `ErrAccountNameRequired`, refuses an Account not added with `ErrAccountNotAdded`, and records nothing when the trimmed name equals the current one exactly. A change of case only is a rename. The fold's shape is unchanged, so no fold schema bump.
  - **Lookup row:** `AccountFolds.Save` already writes `name` on every save, so the `accounts` row follows the rename with no store change.
  - **Service:** `Accounts.Rename` checks that the Account is one of the Household's (otherwise 404), then executes the command. The response carries the pending and Suspected duplicate counts, like `GET`.
  - **HTTP:** `ChangeAccount` replaces the 501 stub (`internal/ledger/adapters/http/pending.go` is deleted). A blank name → 422 `/problems/invalid-account`, "The name must not be blank." (the same copy as on create).
  - **Tests:** `internal/ledger/domain/account_rename_test.go`; `internal/ledger/adapters/http/account_rename_integration_test.go` (rename with kind, currency and start date kept, the same name recording no event, rebuild, only spaces 422, empty/201 characters/missing 400, unknown Account and another Household's 404).
