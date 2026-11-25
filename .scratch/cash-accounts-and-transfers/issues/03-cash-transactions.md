# 03: Record cash Transactions

**What to build:** a member records a cash payment or receipt. It has one date, a signed amount, a description, an optional payee, and a Category, or none for Uncategorised. It can be dated in the Open month or any later month, up to today. Corrections go through the existing Reverse and Set Category. Spec: `../spec.md`, "Ledger: cash Accounts", stories 5–10.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `recordCashTransaction`:
  - refused for a non-cash Account (`not-a-cash-account`);
  - refused for a zero amount, too many decimal places, or a date after today (`invalid-transaction`);
  - Category rules as in Accept;
  - the date is used as both Booking and Transaction date;
  - delete the stub
- [ ] Account Decider: for a cash Account, `BookTransaction` and `BookReversal` accept any month from the Open month on. Before it is `month-closed`, and before the start date `ErrBookedBeforeStartDate`. Non-cash Accounts are unchanged
- [ ] `counterpart_account_id` is refused with 501 until ticket 07 (legs) and ticket 09 (cash ↔ cash)
- [ ] Reverse and Set Category work on cash Transactions, including in a later month
- [ ] Consistency row: Record cash Transaction
- [ ] Decider tests and HTTP tests: record in the Open month, a later month and a closed month; on a bank Account; Uncategorised, then categorised
