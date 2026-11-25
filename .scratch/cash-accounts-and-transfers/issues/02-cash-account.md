# 02: Add a cash Account with its Opening count, and Recount opening

**What to build:** a member adds a cash Account with an Opening count typed twice, which records its Opening balance on the start date. While the start month is still the Open month, the member can recount. Creating a Connection on a cash Account is refused. Spec: `../spec.md`, "Ledger: cash Accounts", stories 1–4.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `AddAccount.OpeningCount`:
  - `CASH` requires it, and any other kind refuses it;
  - remove `ErrCashAccountsNotSupported`;
  - the app checks the two counts are equal (`opening-count-mismatch`);
  - zero is valid, and too many decimal places is refused
- [ ] For cash, the Opening balance Transaction (Account ↔ Opening balance) is recorded on the start date in the same database transaction, with `RecordOpeningBalance` and its Account Posting lookup row. The Account never awaits a first Statement import
- [ ] `RecountOpening`:
  - refused unless the Account is cash (`not-a-cash-account`), and unless the start month is the Open month (`opening-count-locked`);
  - reverses the old Opening balance Transaction (`ReverseOpeningBalance`) and records a new one;
  - the same count records nothing
- [ ] `createConnection` on a cash Account gets 422 `invalid-connection`
- [ ] Consistency rows: Add Account (cash), Recount opening, Create Connection
- [ ] Decider tests and HTTP tests, as listed in the spec's Testing Decisions
