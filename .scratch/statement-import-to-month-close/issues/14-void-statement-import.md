# 14: Void Statement import

**What to build:** a User who imported a file with a wrong mapping voids the Account's latest Statement import, fixes the mapping and uploads the same file again. Voiding the first import also removes the Opening balance, so a corrected first file works it out again. Spec: `../spec.md` stories 43d–43f, 60, "Ingestion: Statement import: Void"; ADR-0018.

**Blocked by:** 10 (Recategorise, reverse Transactions and Discard pending Imported transactions), 11 (Later imports: Balance check and certain repeats)

**Status:** claimed

- [ ] `POST …/statement-imports/{statement_import_id}/void` emits `voided`
- [ ] Allowed only for the Account's latest non-voided import, and only while none of its rows are Accepted
- [ ] All its Imported transactions become Discarded and its file hash is released
- [ ] Voiding the Account's only import reverses the Opening balance Transaction through a Ledger `app` port in the same database transaction, and the Account goes back to awaiting its first Statement import
- [ ] Decider tests: void rules
- [ ] HTTP tests: void and re-upload with a fixed mapping; voiding the first import reopens the Opening balance and a new preview/confirm works; refusing to void an earlier import; refusing to void one with Accepted rows
- [ ] Consistency map row for Void Statement import matches the code
