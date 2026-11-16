# 09: Accept Imported transactions

**What to build:** a User Accepts a pending Imported transaction, with a Category or without one (leaving its other side in Uncategorised). A balanced Ledger Transaction is recorded with exactly the bank row's amount, and the Account balance and transaction list update immediately. Spec: `../spec.md` stories 45–49, 51a, 52, "Cross-context calls: Accept"; ADR-0011, ADR-0013, ADR-0018.

**Blocked by:** 05 (Categories), 08 (First Statement import: preview, confirm and Opening balance)

**Status:** ready-for-agent

- [ ] `POST …/imported-transactions/{imported_transaction_id}/accept` with optional `category_id`
- [ ] The Ingestion application service calls a Ledger `app` port in the same database transaction, running the Imported transaction, Transaction and Account Deciders, each with its own version check, retrying on conflict
- [ ] The Transaction carries the Imported transaction's amount, dates, description and payee, and links to it; no command changes an amount
- [ ] No Category, or Uncategorised, books the other side to Uncategorised; the Account fold tracks the Uncategorised count per Booking month
- [ ] Refused: an Imported transaction already Accepted; an Internal Category other than Uncategorised
- [ ] The Account's balance and its transaction list reflect the Accept immediately
- [ ] Decider tests: Imported transaction state transitions, Transaction balance invariants
- [ ] HTTP tests: Accept with and without a Category, each refusal, Idempotency-Key replay never books twice; another Household gets 404
- [ ] Consistency map row for Accept imported transaction matches the code
