# 06: Accept a bank row as a Transfer to a cash Account

**What to build:** Accepting an Imported transaction with `transfer_account_id` (a cash Account) does four things in one database transaction:
- records the bank leg, exactly as an Accept does today;
- records the cash leg automatically, dated on the row's Transaction date, with the opposite amount;
- confirms the Match between them;
- writes both Account folds in Account ID order.

Spec: `../spec.md`, "Ledger: Transfers" (Accept as Transfer), "Ingestion", stories 14, 15 and 22.

**Blocked by:** 03, 05

**Status:** ready-for-agent

- [ ] Ledger port `BookTransfer`, called by Ingestion's Accept inside its transaction
- [ ] Refusals (`invalid-transfer`):
  - `category_id` together with `transfer_account_id`;
  - a non-cash counterpart (`counterpart-not-cash`);
  - a different currency;
  - the row's own Account
- [ ] A cash leg date in a closed cash month: `month-closed`, naming the cash Account
- [ ] `match_transaction_id` is refused with 501 until ticket 07
- [ ] Consistency row: Accept as Transfer
- [ ] HTTP tests:
  - an ATM row: both balances move, both legs share `match_id`, and `getMatch` returns them;
  - a deposit in the other direction;
  - Category totals leave out Account transfer;
  - each refusal
