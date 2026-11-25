# 02: Add a cash Account with its Opening count, and Recount opening

**What to build:** a member adds a cash Account with an Opening count typed twice, which records its Opening balance on the start date. While the start month is still the Open month, the member can recount. Creating a Connection on a cash Account is refused. Spec: `../spec.md`, "Ledger: cash Accounts", stories 1–4.

**Blocked by:** 01

**Status:** resolved

- [x] `AddAccount.OpeningCount`:
  - `CASH` requires it, and any other kind refuses it;
  - remove `ErrCashAccountsNotSupported`;
  - the app checks the two counts are equal (`opening-count-mismatch`);
  - zero is valid, and too many decimal places is refused
- [x] For cash, the Opening balance Transaction (Account ↔ Opening balance) is recorded on the start date in the same database transaction, with `RecordOpeningBalance` and its Account Posting lookup row. The Account never awaits a first Statement import
- [x] `RecountOpening`:
  - refused unless the Account is cash (`not-a-cash-account`), and unless the start month is the Open month (`opening-count-locked`);
  - reverses the old Opening balance Transaction (`ReverseOpeningBalance`) and records a new one;
  - the same count records nothing
- [x] `createConnection` on a cash Account gets 422 `invalid-connection`
- [x] Consistency rows: Add Account (cash), Recount opening, Create Connection
- [x] Decider tests and HTTP tests, as listed in the spec's Testing Decisions

## Comments

2026-11-25, implementation notes:

- **Domain** (`internal/ledger/domain/cash.go`):
  - `AccountKindCash` is exported and listed in `AccountKinds()`. `ErrCashAccountsNotSupported` is gone.
  - `AddAccount.OpeningCount` is checked by `checkOpeningCount`:
    - a cash Account needs one (`ErrOpeningCountRequired`);
    - a statement Account refuses one (`ErrOpeningCountNotForStatementAccount`);
    - too many decimal places is `ErrTooManyDecimalPlaces`.

    The Decider still records only `AccountAdded`, and the Opening balance follows through the existing `RecordOpeningBalance`.
  - **`RecountOpening { TransactionID, ReversalID, PreviousAmount, Amount }`:**
    - it is refused for a statement Account (`ErrNotACashAccount`), and once the start month isn't the Open month (`ErrOpeningCountLocked`);
    - it records `OpeningBalanceReversed` then `OpeningBalanceRecorded`, which reuse the existing events, so there is no new event type;
    - the same amount records nothing.
  - **No migration and no fold schema bump:** the fold doesn't keep the Opening balance amount, so the app reads it from the current Opening balance Transaction's Posting on the Account and passes it as `PreviousAmount`.
- **App:**
  - `Accounts` gains an `OpeningBalances` port, implemented by `Transactions`, which the registry now builds first.
  - `Accounts.Add` checks the two counts with `openingCount`:
    - a missing one is `ErrOpeningCountRequired`;
    - another currency is `ErrCurrencyMismatch`;
    - counts that differ are `ErrOpeningCountMismatch`.

    For cash, it then records the Opening balance in the same transaction.
  - `Accounts.RecountOpening` and `Transactions.RecountOpening` implement Recount opening. `RecordOpeningBalance`'s Transaction part became `recordOpeningTransaction`, shared by both.
- **Ingestion:** `ImportAccount.Cash` is set by the Ledger port, and `Connections.Create` refuses a cash Account with `ErrCashAccountConnection` (422 `invalid-connection`).
- **HTTP:**
  - **`addAccount`** maps `CASH` and the counts. Refusals:
    - counts that differ: `opening-count-mismatch`;
    - a missing count, another currency, too many decimal places, or a count on a statement Account: `invalid-account`.
  - **`recountOpening`** is implemented. Refusals:
    - a statement Account: `not-a-cash-account`;
    - counts that differ: `opening-count-mismatch`;
    - another currency or too many decimal places: `invalid-account`;
    - the start month has closed: 409 `opening-count-locked`.
  - The ticket 02 stubs and their cases in the 501 test are deleted.
- **Tests:**
  - Decider tests in `cash_test.go`.
  - HTTP tests in `internal/ledger/adapters/http/cash_integration_test.go`:
    - adding a cash Account, including an empty wallet;
    - every refusal;
    - recount, with its reversal linked both ways, and the same count recording nothing;
    - recount refusals, including the lock after closing the start month;
    - a Connection on cash refused.
  - **Note for ticket 04:** the lock test closes January with only `statement_balance`. Once ticket 04 requires `counted_on` for cash, add it there.
- **Consistency rows:** Add Account (cash), Recount opening and Create Connection.
- **Checks:** `make lint-go` (0 issues) and `make test-integration` pass.
