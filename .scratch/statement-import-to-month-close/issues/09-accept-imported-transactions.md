# 09: Accept Imported transactions

**What to build:** a User Accepts a pending Imported transaction, with a Category or without one (leaving its other side in Uncategorised). A balanced Ledger Transaction is recorded with exactly the bank row's amount, and the Account balance and transaction list update immediately. Spec: `../spec.md` stories 45–49, 51a, 52, "Cross-context calls: Accept"; ADR-0011, ADR-0013, ADR-0018.

**Blocked by:** 05 (Categories), 08 (First Statement import: preview, confirm and Opening balance)

**Status:** resolved

- [ ] `POST …/imported-transactions/{imported_transaction_id}/accept` with optional `category_id`
- [ ] The Ingestion application service calls a Ledger `app` port in the same database transaction, running the Imported transaction, Transaction and Account Deciders, each with its own version check, retrying on conflict
- [ ] The Transaction carries the Imported transaction's amount, dates, description and payee, and links to it; no command changes an amount
- [ ] No Category, or Uncategorised, books the other side to Uncategorised; the Account fold tracks the Uncategorised count per Booking month
- [ ] Refused: an Imported transaction already Accepted; an Internal Category other than Uncategorised
- [ ] The Account's balance and its transaction list reflect the Accept immediately
- [ ] Decider tests: Imported transaction state transitions, Transaction balance invariants
- [ ] HTTP tests: Accept with and without a Category, each refusal, Idempotency-Key replay never books twice; another Household gets 404
- [ ] Consistency map row for Accept imported transaction matches the code

## Comments

Implementation notes (implementer agent):

- **Path.** `POST /households/{household_id}/imported-transactions/{imported_transaction_id}/accept` (under the Household, like ticket 08's Statement imports; vacuum's sub-resource limit rules out nesting under the Account). The body (`ImportedTransactionAcceptance`, `category_id` optional) is itself optional: no body, `{}`, or the Uncategorised Category all book to Uncategorised. 200 returns the Imported transaction, now `ACCEPTED` with `transaction_id`. Problems: 409 `/problems/imported-transaction-already-accepted`; 422 `/problems/internal-category` (Opening balance, Household transfer); 422 `/problems/invalid-category` (unknown or another Household's Category, as ticket 05 answers an unknown parent); 404 for an unknown or another Household's Imported transaction. `state` (list filter and schema) gains `ACCEPTED`.
- **Shape.** Ingestion: Imported transaction command `AcceptImportedTransaction` → `io.walletaccountant.ingestion.importedtransaction.accepted.v1` (refused when not imported, already Accepted, or not pending); the fold keeps `TransactionID`. `ingestionapp.ImportedTransactions.Accept` runs the Imported transaction Decider, then the Ledger port `BookAccepted` (in `internal/wiring/ledgerport.go`) in the same request transaction. Ledger: `ledgerapp.Transactions.RecordImported` resolves the Category (reads its fold; nil or Uncategorised → Uncategorised), runs the Account Decider's new `BookTransaction` → `io.walletaccountant.ledger.account.transactionbooked.v1` (refused while awaiting the first import, before the start date, without a Booking date, or with too many decimal places), then the Transaction Decider with the row's amount, dates, description and payee and `ImportedTransactionID`. The Account fold gains `Uncategorised` (count per Booking month), stored in a new `account_months.uncategorised` column. The Transaction ID is generated once per request, so a retry on a version conflict reuses it.
- **Migration** `000009_accept_imported_transactions`: columns only (`imported_transactions.transaction_id`, `ledger_transactions.imported_transaction_id`, `account_months.uncategorised`), no new table, so no new RLS test. Fold schema versions bumped: Account 3, Transaction 2, Imported transaction 2.
- **Tests.** Decider: Accept and its refusals, the accepted fold; Transaction recorded with its link, payee and Value date; Account booking (balance per month, Uncategorised count per month) and its refusals. HTTP: Accept without a Category (balance, transaction list in Uncategorised, pending/accepted lists), with a Category and with Uncategorised explicitly, every refusal booking nothing, already Accepted 409, unknown 404, Idempotency-Key replay booking once, 8 concurrent Accepts over three rows (one 200 per row, balance counted once), another Household 404 (both paths), rebuild, and the carry-over from 08: a multi-page walk of `…/accounts/{a}/transactions` with and without `booking_month`.
- **Not covered yet.** The Uncategorised count has no read path until ticket 15 (month status), so it is tested only at the Decider seam; its adapter round trip is exercised by the HTTP tests but nothing asserts on it. Closed-month flags (ticket 16) aren't set; the consistency map row says so.
- **Consistency map.** The Accept imported transaction row now spells out what the code does, and notes closed-month flags come with Month close.
