# 01: Contract: land the new `api/openapi.yaml`

**What to build:** land the contract this spec settled, so the UI can pin its `openapi.yaml` to the merge commit. `api/openapi.yaml`, `api/vacuum-ignore.yaml` and the `lint-api` change are already written on branch `ui-first-slice-backend`. What's left is to regenerate, compile and keep CI green with interim behaviour. Spec: `../spec.md`, "Contract" and "Delivery".

**Blocked by:** —

**Status:** ready-for-agent

- [ ] `make generate` (ogen server and Bruno collection), with the generated code committed
- [ ] Every new operation (`listAccountMonths`, `reopenAccountMonth`, `changeAccount`, `changeConnection`, `testCsvMapping`) is wired, answering 501 with a problem until its ticket lands
- [ ] Interim values that pass response validation: `open_month` = the first month from the start date's month that isn't in `Closed`; `pending`/`suspected_duplicates` 0; `reopen_blocked` true; `unchecked_statement_imports` from the existing `MonthImports`; `void_blocked_by` empty; `rows_ignored_in_closed_months` 0; `state` from the interim `open_month`; `flagged` no longer emitted
- [ ] Connection `name` is required on create, so it is recorded in `ConnectionCreated` and on the lookup row and returned (uniqueness and rename come in ticket 10); existing tests send names
- [ ] `make lint` and `make test-integration` pass
- [ ] After merge: annotated tag `v0.2.0` on the merge commit, then tell the UI repo
- [ ] Do not change the contract's shape: anything that seems wrong is a comment on the spec

## Comments
