# 03: Record cash Transactions

**What to build:** a member records a cash payment or receipt. It has one date, a signed amount, a description, an optional payee, and a Category, or none for Uncategorised. It can be dated in the Open month or any later month, up to today. Corrections go through the existing Reverse and Set Category. Spec: `../spec.md`, "Ledger: cash Accounts", stories 5–10.

**Blocked by:** 02

**Status:** resolved

- [x] `recordCashTransaction`:
  - refused for a non-cash Account (`not-a-cash-account`);
  - refused for a zero amount, too many decimal places, or a date after today (`invalid-transaction`);
  - Category rules as in Accept;
  - the date is used as both Booking and Transaction date;
  - delete the stub
- [x] Account Decider: for a cash Account, `BookTransaction` and `BookReversal` accept any month from the Open month on. Before it is `month-closed`, and before the start date `ErrBookedBeforeStartDate`. Non-cash Accounts are unchanged
- [x] `counterpart_account_id` is refused with 501 until ticket 07 (legs) and ticket 09 (cash ↔ cash)
- [x] Reverse and Set Category work on cash Transactions, including in a later month
- [x] Consistency row: Record cash Transaction
- [x] Decider tests and HTTP tests: record in the Open month, a later month and a closed month; on a bank Account; Uncategorised, then categorised

## Comments

2026-11-25, implementation notes:

- **Domain** (`internal/ledger/domain/account.go`): `bookTransaction` refuses a month after the Open month only for a statement Account, so a cash Account books in its Open month and any later one. A month before it is still `month-closed`, and a date before the start date `ErrBookedBeforeStartDate`. `bookReversal` already refused only closed months, so it needed no change. Its comment now says so.
- **App** (`internal/ledger/app/cash.go`): `Transactions.RecordCash(household, CashTransaction)` loads the Account and refuses, in this order:
  - not found;
  - a statement Account (`ErrNotACashAccount`);
  - another currency (`ErrCurrencyMismatch`);
  - a zero amount (`ErrZeroAmount`);
  - a date after today in UTC, by `tx.Now()` (`ErrDateInTheFuture`).

  Then `chooseCategory` applies Accept's Category rules, the Account Decider books it, and the Transaction is recorded with the date as both Booking and Transaction date, with no Value date. Too many decimal places and a date before the start date come from the Account Decider.
- **HTTP** (`internal/ledger/adapters/http/cash.go`): `recordCashTransaction` answers 201 with the `Transaction`. Refusals:
  - a statement Account: 422 `not-a-cash-account`;
  - a zero amount, another currency, too many decimal places, a date after today or before the start date: 422 `invalid-transaction`;
  - an Internal Category: 422 `internal-category`; an unknown one: 422 `invalid-category`;
  - `category_id` with `counterpart_account_id`: 422 `invalid-transfer`, reason `CATEGORY_AND_TRANSFER`. This is checked before anything else, so it works already;
  - a closed month: 409 `month-closed`, with `account_id` set to the Account.

  `counterpart_account_id` alone answers 501 (`transferLegPending` in `pending.go`) until tickets 07 and 09. The 501 test now sends a leg instead of a plain cash Transaction.
- **No contract change, no migration, no fold schema bump.**
- **Tests:**
  - Decider tests in `cash_test.go`: booking cash in the Open month, a later month and after a close; refused in a closed month and before the start date; a reversal in a later month, and refused in a closed month. The statement Account's later-month refusal is already in `closed_month_test.go`.
  - HTTP tests in `cash_transactions_integration_test.go`, with the clock pinned to 2026-03-10: record Uncategorised in January and categorised in March; categorise the January one later; close January; then recording and reversing in January get `month-closed`, and the March one reverses. Every refusal, with the balance unchanged.
  - **Note for ticket 04:** `TestRecordingCashTransactions` closes January with only `statement_balance`. Once ticket 04 requires `counted_on` for cash, add it there too.
- **Consistency row:** Record cash Transaction.
- **Checks:** `make lint-go` (0 issues) and `make test-integration` pass.
