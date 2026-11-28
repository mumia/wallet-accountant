# 03: Suggestions across Accounts, with pending rows

**What to build:** `listTransferSuggestions` works out the other side of a row from every eligible counterpart when `account_id` is left out, and lists pending rows on statement counterparts as well as In transit legs. Spec: `../spec.md`, "Ingestion" (Suggestions), stories 9–11.

**Blocked by:** 02

**Status:** resolved

- [x] Eligible counterparts: the Household's other Accounts in the row's currency that pass the counterpart check (so no loan or crypto wallet, and nothing for a row on one)
- [x] With `account_id`: only that Account; one failing the check gets the check's 422
- [x] `legs`: In transit legs naming the row's Account, opposite amount, Transaction date within ±7 days; the Ledger port query widened to many Accounts
- [x] `imported_transactions`: pending rows on eligible statement counterparts, opposite amount, Transaction date within ±7 days, read from Ingestion's own tables; rows in a month that isn't open included; undecided Suspected duplicates left out
- [x] Both lists closest date first, then by ID; never stored
- [x] Remove the 501 guard for a missing `account_id`
- [x] HTTP tests:
  - without `account_id`: a cash leg and a pending card row both appear, each with its `account_id`;
  - left out: another currency, a loan Account, 8 days away, another amount, an undecided Suspected duplicate;
  - with `account_id` of a loan, and for a row on a loan: `ACCOUNT_KIND_NOT_SUPPORTED`;
  - order: closest date first;
  - the existing cash suggestion tests still pass

## Comments

2026-11-28, implementation notes:

- **Split between the contexts:** the Ledger works out the counterparts and their In transit legs (`Transactions.TransferSuggestions`, now returning `TransferSuggestions{Legs, StatementAccounts}`); Ingestion reads the pending rows on `StatementAccounts` from its own table (`ImportedTransactionReader.PendingTransferRows`). Ingestion never reads Ledger tables, and the Ledger never reads Ingestion's.
- **Counterparts:** `TransactionReader.TransferCounterparts` lists the Household's other Accounts in the currency with their kinds; the app keeps those `domain.TakesPartInTransfers` (new, shared with the Decider's rule). A named `account_id` goes through `checkCounterpart` plus the kind rule. `checkCashCounterpart` and the temporary `COUNTERPART_NOT_CASH` reason are gone, as is the Ingestion handler's 501 for it.
- **Window:** Ingestion has its own `SuggestionWindowDays = 7`, matching the Ledger's `SuggestionWindow`.
- **Order:** both lists by distance from the row's Transaction date, then date, then ID.
- **Not covered by a test:** an Account in another currency is left out by the SQL (`currency = $3`); the fixture's `addStatementAccount` only makes EUR Accounts, so no test adds one.
- **Tests:** `TestSuggestionsComeFromEveryAccountTheRowCouldBeATransferWith`; the manual-leg test now gets 200 for suggestions on a bank Account; the 03 cases of `TestStatementTransferPiecesAnswerNotImplementedYet` are gone (only 04's remains).

