# 03: Suggestions across Accounts, with pending rows

**What to build:** `listTransferSuggestions` works out the other side of a row from every eligible counterpart when `account_id` is left out, and lists pending rows on statement counterparts as well as In transit legs. Spec: `../spec.md`, "Ingestion" (Suggestions), stories 9–11.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] Eligible counterparts: the Household's other Accounts in the row's currency that pass the counterpart check (so no loan or crypto wallet, and nothing for a row on one)
- [ ] With `account_id`: only that Account; one failing the check gets the check's 422
- [ ] `legs`: In transit legs naming the row's Account, opposite amount, Transaction date within ±7 days; the Ledger port query widened to many Accounts
- [ ] `imported_transactions`: pending rows on eligible statement counterparts, opposite amount, Transaction date within ±7 days, read from Ingestion's own tables; rows in a month that isn't open included; undecided Suspected duplicates left out
- [ ] Both lists closest date first, then by ID; never stored
- [ ] Remove the 501 guard for a missing `account_id`
- [ ] HTTP tests:
  - without `account_id`: a cash leg and a pending card row both appear, each with its `account_id`;
  - left out: another currency, a loan Account, 8 days away, another amount, an undecided Suspected duplicate;
  - with `account_id` of a loan, and for a row on a loan: `ACCOUNT_KIND_NOT_SUPPORTED`;
  - order: closest date first;
  - the existing cash suggestion tests still pass

## Comments
