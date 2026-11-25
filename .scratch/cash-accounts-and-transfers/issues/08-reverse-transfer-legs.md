# 08: Reversing a Transfer leg

**What to build:** reversing a Matched leg removes the Match and unmatches both legs.
- **Bank leg:** reversing it returns its row to pending and also reverses an automatic cash leg. A cash leg entered by hand stays, In transit.
- **Cash leg alone:** reversing it leaves the bank leg Accepted, In transit.
- **Closed months:** the whole command is refused if any Transaction it would reverse is in a closed month.

Spec: `../spec.md`, "Ledger: Transfers" (Reversing a Transfer leg), story 21.

**Blocked by:** 06 (the manual-leg case needs 07)

**Status:** resolved

- [x] `RemoveMatch` and `TransactionUnmatched` on both legs, in Reverse's database transaction
- [x] Cascade to a `createdbytransfer` leg. No cascade to a manual leg
- [x] Closed-month check across every Transaction the command would reverse, before anything is recorded
- [x] Update the consistency row: Reverse Transaction (Transfer leg)
- [x] HTTP tests for each case in the spec's Testing Decisions ("Reversing")

## Comments

2026-11-25, implementation notes:

- **Domain:** `reverseTransaction` refuses a leg that is still Matched (`ErrTransactionMatched`), so nothing can reverse one without removing its Match first.
- **App** (`Transactions.Reverse`, `internal/ledger/app/corrections.go`), in one database transaction:
  1. Load the Transaction (not found: 404).
  2. If it is Matched, `unmatch` removes the Match (`RemoveMatch`) and records `TransactionUnmatched` on both legs. It also returns the other leg.
  3. The other leg is reversed too only when it is `CreatedByTransfer` and the one asked for isn't. That covers the bank leg with its automatic cash leg. A manual cash leg stays, In transit, and so does a bank leg whose automatic cash leg is reversed alone. Ticket 09 adds cash ↔ cash ("reversing either reverses both") to this rule.
  4. Every Transaction to reverse is sorted by its Account ID. Then `reverseOne` (the old Reverse body) marks it reversed, books the reversal on each Account, records it, and returns an Accepted row to pending.
  - The two reversal IDs are made outside the retried transaction.
- **Closed months:**
  - `BookReversal` refusals are wrapped in `*AccountError`, so `month-closed` now names the Account in `account_id`, for every Reverse.
  - The check isn't a separate pass before anything is recorded. Any refusal fails the one database transaction, which rolls back the Match removal and any reversal already booked. The test checks that nothing changed: the Match is still there, the bank leg is still Matched, the row is still Accepted, and the balance is unchanged.
- **A reversal never copies the counterpart** (`TransactionState.Reversal` doesn't), so it is never a leg and never In transit.
- **No contract change, no migration.**
- **Tests:**
  - Decider test: reversing a Matched leg is refused; once unmatched, it reverses.
  - HTTP tests in `internal/ingestion/adapters/http/reverse_transfer_integration_test.go`:
    - the bank leg with an automatic cash leg: both are reversed, the row is pending, the Match gets 404, both balances are back, and nothing is In transit;
    - the bank leg with a manual cash leg: the leg stays, In transit and unmatched, the row is pending, and the row can be Accepted against the leg again;
    - the cash leg alone: the bank leg stays Accepted and In transit;
    - a closed wallet January: reversing either leg gets 409 naming the wallet, and nothing changes.
- **Consistency row:** Reverse Transaction (Transfer leg).
