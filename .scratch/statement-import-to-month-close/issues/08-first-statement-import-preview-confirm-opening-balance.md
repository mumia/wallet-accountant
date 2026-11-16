# 08: First Statement import: preview, confirm and Opening balance

**What to build:** a User uploads an Account's first statement file with its Export balance typed twice, previews the Opening balance it would produce, and confirms. The import is recorded, every row becomes a pending Imported transaction, and the Opening balance Transaction is recorded on the start date. The Account now shows its balance, and its single-entry transaction list shows the Opening balance. Spec: `../spec.md` stories 17, 17a, 21–23, 34–37, 41–42, 43a, 43g, 43h, 44, "Ingestion: Statement import", "Opening balance"; ADR-0001, ADR-0011, ADR-0018.

**Blocked by:** 07 (Connections and CSV mapping)

**Status:** ready-for-agent

- [ ] `POST …/connections/{connection_id}/statement-imports/previews` (multipart: file, `export_balance`, `export_balance_confirmation`) refuses mismatched entries and otherwise returns `preview_id`, the Opening balance on the start date, the row count, rows ignored before the start date and the rows' total. It records nothing
- [ ] `preview_id` is a hash of Account ID, file hash, Export balance and mapping version, so the server keeps no state
- [ ] `POST …/statement-imports` with `preview_id` recomputes it and refuses a different file, Export balance or mapping version; refuses if the Account already got its Opening balance; refuses a first import without `preview_id`
- [ ] Confirm records the Statement import, its Imported transactions (pending) and the Opening balance Transaction in one database transaction, through a Ledger `app` port. The Account stops awaiting its first import
- [ ] Ledger Transaction Decider: Postings sum to zero per currency, each names one Account or one Category; its fold saves one lookup row per Account Posting in the same database transaction
- [ ] Rows booked before the start date are ignored and counted; a file with any unreadable row is rejected whole with row numbers and reasons; the same file again is refused with a pointer to the earlier import
- [ ] `GET …/accounts/{account_id}` shows the balance; `GET …/accounts/{account_id}/transactions?booking_month=YYYY-MM` returns the single-entry list (signed amount, Booking date, Transaction date, description, Category) with cursor paging, read inside the request transaction
- [ ] `GET …/statement-imports` lists a Connection's imports; `GET …/accounts/{account_id}/imported-transactions?state=pending&booking_month=YYYY-MM` lists rows waiting for review
- [ ] HTTP tests: preview records nothing, every confirm refusal, Opening balance with rows before the start date ignored, duplicate file, row-level rejection, Idempotency-Key replay of the upload, transaction list paging and month filter; another Household gets 404
- [ ] Row-level security test for every table this ticket adds; new folds registered for rebuild
- [ ] Consistency map rows for Preview first Statement import and Upload Statement import (first import) match the code
