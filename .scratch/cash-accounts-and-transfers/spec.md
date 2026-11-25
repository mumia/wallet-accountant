# Spec: Cash Accounts and Transfers

Status: ready-for-agent

The first slice dropped cash Accounts. Under ADR-0018 an Opening balance comes only from a Statement import's Export balance, so an Account with no statement could never get a balance or close a month. This spec brings cash back as a **fully manual** Account. It also adds the smallest kind of **Transfer** that makes cash honest: money moving between a statement Account and a cash Account, recorded as two **Transfer legs** linked by a **Match**. This builds Match, the shared-kernel concept that Planning will reuse later. It was decided with the user on 2026-11-25.

This spec covers only the backend. Once the slice is implemented, ticket 11 writes a hand-off note for `../wallet-accountant-ui`.

## Problem Statement

Most Households hold some cash, and a good share of their spending is paid with it. Wallet Accountant can't track any of it. A cash Account can't be added. Even if it could, it would have no Opening balance and could never close a month, because there is no bank statement to agree with.

Worse, the money that goes into cash is already being misrecorded. An ATM withdrawal on a bank statement can only be Accepted with a Category, so it counts as spending, even though the money only moved to the wallet. Once cash purchases are recorded too, every euro spent in cash would count twice.

## Solution

**Cash Accounts.** A **cash Account** is kept entirely by hand.
- It is added with an **Opening count**: the cash counted on its start date, typed twice. That count records its Opening balance, the same way a first Statement import does for a bank Account.
- Members record **cash Transactions** directly: a date, a signed amount, a description, an optional payee, and a Category.
- To close a month, a member counts the cash and gives the count together with the date it was counted. The month closes when the books agree with the count.
- A gap is closed by recording what really happened, such as a loss booked against an Expense Category, never by a balancing entry (ADR-0018).
- Statement Accounts still take Transactions only from their statements.

**Transfers.** Money moving between two of the Household's Accounts is a **Transfer**: one **Transfer leg** on each Account. Each leg is booked against the new Internal Category **Account transfer** and names the other Account as its **counterpart Account**. The two legs are linked by a **Match**.
- **Bank row first.** When a member Accepts a bank row as a Transfer to a cash Account, the cash leg is recorded automatically and Matched in the same database transaction.
- **Cash leg first.** If the member already entered the cash leg by hand, the Accept offers it as a **Match suggestion**, and the member confirms the Match instead of creating a new leg.
- **Cash to cash.** One command records both legs and their Match.
- **In transit.** A leg without a Match is **In transit**. Money that left the bank on the 31st and reached the wallet on the 2nd really is in transit at month end.

## User Stories

### Cash Accounts

1. As a member, I want to add a cash Account with the amount I count on its start date, so that I can track the money in my wallet.
2. As a member, I want to type the Opening count twice, so that a typo doesn't become an Opening balance that no later check can catch.
3. As a member, I want to correct a wrong Opening count while the start month is still open, so that one slip doesn't force me to start over.
4. As a member, I want creating a Connection on a cash Account to be refused, so that nobody imports into an Account that is kept by hand.

### Cash Transactions

5. As a member, I want to record a cash payment or receipt with one date, an amount, a description, an optional payee and a Category, so that cash spending is in the books.
6. As a member, I want to record a cash Transaction without a Category and categorise it later, so that I can capture it on the go.
7. As a member, I want to record cash Transactions in a month after the open one, so that I can keep recording today even though I haven't counted last month yet.
8. As a member, I want a cash Transaction dated in a closed month to be refused, so that a confirmed month stays confirmed.
9. As a member, I want to fix a wrong cash Transaction by reversing it and entering it again, so that history is never silently edited.
10. As a member, I want a manual Transaction on a bank, card, loan or crypto Account to be refused, so that those Accounts keep agreeing with their statements.

### Closing a cash month

11. As a member, I want to close a cash month by giving the amount I counted and the date I counted it, so that I don't have to count at exactly midnight on the last day.
12. As a member, I want the close to tell me the gap between my count and the books, so that I know how much to record as lost or found.
13. As a member, I want to reopen the latest closed cash month even when later months already have Transactions, so that I can fix a mistake without first reversing everything after it.

### Transfers

14. As a member, I want to Accept an ATM withdrawal as a Transfer to my cash Account, so that it isn't counted as spending and the cash appears in the wallet automatically.
15. As a member, I want to Accept a cash deposit row as a Transfer from my cash Account, so that the money leaves the wallet when it reaches the bank.
16. As a member, I want to record a cash Transaction as a Transfer leg naming the bank Account, so that I can note a withdrawal the moment I make it.
17. As a member, I want the Accept to suggest the cash leg I already entered, so that the same withdrawal isn't recorded twice in cash.
18. As a member, I want to move money between two cash Accounts in one step, so that both wallets stay right.
19. As a member, I want to see which Transfer legs are still In transit, so that I know which ones haven't been Matched yet.
20. As a member, I want a month to close while a Transfer leg is In transit, so that money moving across a month end doesn't block the close.
21. As a member, I want reversing a Transfer leg to undo its Match, and to reverse a cash leg that was created automatically, so that undoing an Accept leaves nothing orphaned.
22. As a member, I want Transfers left out of my spending and income, so that moving money isn't counted as using it.

## Implementation Decisions

### Contract

Ticket 01 writes these into `api/openapi.yaml`. Every other ticket implements part of them and must not change the contract without a comment on this spec.

**Changed schemas and operations**

- **`AccountKind`** gains `CASH`. The "not supported yet" wording goes.
- **`AccountCreation`** gains `opening_count` and `opening_count_confirmation`, both `Money` in the Account currency.
  - Both are required for `CASH`. For any other kind they are refused with 422 `/problems/invalid-account`.
  - Two counts that differ get 422 `/problems/opening-count-mismatch`.
  - For `CASH`, `addAccount` answers with the Account's balance already set.
- **`createConnection`** on a cash Account gets 422 `/problems/invalid-connection`, with a reason saying the Account is cash.
- **`MonthClosing`** gains `counted_on`, a date.
  - **Cash Account:** `counted_on` is required, and `statement_balance` is the amount counted on that date.
  - **Any other Account:** `counted_on` is refused with 400 (`/problems/invalid-month-closing`).
  - A `counted_on` that isn't after the month's last day, or is later than today (UTC), gets 422 `/problems/invalid-count-date`.
  - A gap gets the existing `month-close-refused` balance mismatch reason, with the computed and counted amounts and their difference. For cash, "computed" is the balance at the end of `counted_on`.
- **`AccountMonth`** gains:
  - `counted_on`, present on a closed cash month;
  - `transfer_legs_in_transit`, the number of unmatched, non-reversed Transfer legs booked in the month.
- **`ImportedTransactionAcceptance`** gains `transfer_account_id` and `match_transaction_id`.
  - `transfer_account_id` can't be sent together with `category_id`: both together get 422 `/problems/invalid-transfer`.
  - `match_transaction_id` needs `transfer_account_id`.
  - The refusals are listed under "Ledger: Transfers".
- **`AccountTransaction`** and **`Transaction`** gain:
  - `counterpart_account_id`, present on a Transfer leg;
  - `match_id`, present while the leg is Matched;
  - `created_by_transfer`, true on a cash leg that an Accept recorded automatically.

  A Transfer leg's `category_id` and `category_name` are the Account transfer Category's.
- **`changeTransaction`** (Set Category) on a Transfer leg gets 422 `/problems/transfer-leg`.
- **`reverseTransaction`**'s description is extended with the Transfer rules ("Ledger: Transfers").
- **`listAccountTransactions`** gains the query parameter `in_transit=true`, which lists only the Account's unmatched, non-reversed Transfer legs.
- **Categories:** the contract has no internal role field, so Account transfer appears as an `INTERNAL` Category named "Account transfer" (decided in ticket 01).
- **`TransferProblem`** (response `TransferRefused`) is the schema of every 422 of `acceptImportedTransaction`, `recordCashTransaction` and `listTransferSuggestions`: a Problem with an optional `reason` (`CATEGORY_AND_TRANSFER`, `MATCH_WITHOUT_TRANSFER`, `COUNTERPART_NOT_CASH`, `SAME_ACCOUNT`, `CURRENCY_MISMATCH`, `UNKNOWN_ACCOUNT`, `NOT_A_MATCHING_LEG`).
- **`MonthProblem`** gains `account_id`.

**New operations**

- **`POST /households/{household_id}/accounts/{account_id}/transactions`** (`recordCashTransaction`)
  - Body: `{ date, amount, description, payee?, category_id?, counterpart_account_id? }`.
  - `category_id` and `counterpart_account_id` are exclusive. Neither means Uncategorised.
  - Answers 201 with the `Transaction` on this Account. For a cash ↔ cash Transfer that is this Account's leg, with its `match_id`; `getMatch` names the other leg (decided in ticket 01).
  - Takes an Idempotency-Key.
- **`POST /households/{household_id}/accounts/{account_id}/opening-count`** (`recountOpening`)
  - Body: `{ opening_count, opening_count_confirmation }`. Answers 200 with the `Account`. Takes an Idempotency-Key.
- **`GET /households/{household_id}/matches/{match_id}`** (`getMatch`)
  - Answers `{ match_id, kind: ACCOUNT_TRANSFER, legs: [{ transaction_id, account_id }, …], confirmed_by, confirmed_at }`.
  - A removed Match gets 404.
- **`GET /households/{household_id}/imported-transactions/{imported_transaction_id}/transfer-suggestions?account_id=`** (`listTransferSuggestions`)
  - Answers the cash Account's unmatched, non-reversed Transfer legs that:
    - name the Imported transaction's Account as counterpart,
    - have the opposite amount, and
    - are dated within ±7 days of the row's Transaction date.
  - Closest date first, as `Transaction` items.
  - Worked out on each request and never stored.

**New problem types**

| Problem type | Status | When |
|---|---|---|
| `/problems/opening-count-mismatch` | 422 | The two Opening counts differ. |
| `/problems/opening-count-locked` | 409 | Recount opening when the start month isn't the Open month. |
| `/problems/not-a-cash-account` | 422 | `recordCashTransaction` or `recountOpening` on a non-cash Account. |
| `/problems/invalid-transfer` | 422 | See "Ledger: Transfers". The response carries a `reason`. |
| `/problems/transfer-leg` | 422 | Set Category on a Transfer leg. |
| `/problems/invalid-count-date` | 422 | A `counted_on` that isn't after the month's last day, or is later than today (UTC). |

Existing month problems (`month-closed`, `month-not-open-yet`) name the Account they are about, through `account_id`. For an Accept as Transfer, that can be the cash Account.

### Ledger: cash Accounts

- **`AddAccount`** gains `OpeningCount *Decimal`.
  - `CASH` is accepted (remove `ErrCashAccountsNotSupported`) and requires it. Any other kind refuses it.
  - The app checks that the two counts are equal before deciding. Only one is passed to the Decider.
  - For cash, the app also records the Opening balance Transaction (Account ↔ Opening balance Internal Category, on the start date). It runs `RecordOpeningBalance` on the same Account stream in the same database transaction, so `AwaitingFirstStatementImport` is never true for cash.
  - A zero count is valid: the Account starts with an empty wallet.
- **`RecountOpening { TransactionID, ReversalID, Amount }`** (Account command)
  - Refused unless the Account is cash, and refused (`opening-count-locked`) unless the start date's month is the Open month.
  - Records `OpeningBalanceReversed` then `OpeningBalanceRecorded`.
  - The app reverses the old Opening balance Transaction with `ReverseOpeningBalance` and records the new one.
  - A count equal to the current Opening balance records nothing.
- **Booking cash Transactions.** For a cash Account, `BookTransaction` and `BookReversal` accept any Booking month from the Open month onwards. A month before it is refused with `month-closed`, and a date before the start date with `ErrBookedBeforeStartDate`. Non-cash Accounts keep ADR-0024's Open-month-only rule.
- **`RecordCashTransaction`** (app command)
  - Refused for a non-cash Account (`not-a-cash-account`).
  - Refused for a zero amount, or for more decimal places than the currency allows.
  - The date is used as both Booking date and Transaction date. There is no Value date.
  - Category rules are the same as Accept: none or Uncategorised books to Uncategorised, and any other Internal Category is refused.
  - With `counterpart_account_id`, the Transaction is a Transfer leg (see "Ledger: Transfers").
  - **Date window:** a date later than today (UTC) is refused with 422 `/problems/invalid-transaction`. Planned money belongs to Planning, not the Ledger.
- **Closing a cash month.** `CloseMonth` gains `CountedOn *time.Time`.
  - **Cash:** `CountedOn` is required.
    - The balance compared with the count is the Opening balance plus every non-reversed Transaction booked up to and including `CountedOn`. A new fold helper, `BalanceAt(date)`, works this out: the Account fold keeps per-day sums for cash Accounts, or the app reads them from the Account Posting lookup rows. Ticket 04 picks one and notes it.
    - The Statement balance stored on the `MonthClose` is the month-end balance this proves (`ClosingBalance(month)`). The count and `CountedOn` are stored alongside it, and `MonthClosed` gains `countedon` and `count`.
    - The other conditions are unchanged: nothing in Uncategorised in the month. A cash Account has no Imported transactions, so the Ingestion conditions are trivially empty.
  - **Non-cash:** `CountedOn` is refused.
- **Reopening a cash month.** `reopenMonth` skips the "Open month has no Accepted Transactions" condition for cash. It still takes back only the latest close, and the month must be closed again with a new count.

### Ledger: Transfers

**The Category and the legs**

- **New Internal Category role `account_transfer`** ("Account transfer"). It is seeded with the others when a Household is created. **There is no migration:** there is no production data yet. Accept and Set Category refuse it like any other Internal Category; it is reached only through a Transfer.
- **A Transfer leg** is an ordinary Transaction with two Postings: its Account ↔ the Account transfer Category.
  - `TransactionRecorded` gains `counterpartaccountid` and `createdbytransfer`.
  - The counterpart must:
    - be another Account of the same Household,
    - not be the leg's own Account, and
    - have the same Account currency.

    Otherwise the leg is refused with `invalid-transfer`.
  - `SetTransactionCategory` on a leg is refused (`transfer-leg`).

**The Match**

- **The Match aggregate** is a new Ledger stream, `match`, whose ID is a UUIDv7.
  - `ConfirmMatch { Kind, Legs [2]ID }` records `match.confirmed`. `RemoveMatch` records `match.removed`.
  - It has a fold table `matches`, Household-scoped with the usual row-level security policy.
  - **Leg state:** each leg's own Transaction stream records `TransactionMatched { matchid }` or `TransactionUnmatched { matchid }`. The Transaction Decider refuses to match a leg that is already Matched, reversed, or not a Transfer leg. Two Accepts racing for the same manual leg are therefore caught by the leg's version check.
  - **Rules:** a Match needs two legs with opposite amounts, each naming the other's Account as its counterpart.

**Accept as Transfer**

The Imported transaction's own checks are unchanged. The bank row's Account must be in its Open month (ADR-0024).

1. `transfer_account_id` must be a cash Account. In this slice, any other kind is refused with `invalid-transfer`, reason `counterpart-not-cash`.
2. **Bank leg.** The bank leg is recorded exactly as an Accept records today. It has the row's amount, dates, description and payee, it is linked to the Imported transaction, and its counterpart is the cash Account.
3. **Cash leg, without `match_transaction_id`.**
   - It is recorded on the cash Account with the opposite amount and the row's description and payee, and `createdbytransfer` set.
   - Its Booking and Transaction date are **the row's Transaction date**.
   - A date in a closed month of the cash Account is refused with `month-closed`, naming the cash Account. A later month is fine.
4. **Cash leg, with `match_transaction_id`.**
   - The leg must be an unmatched, non-reversed Transfer leg on the cash Account, naming the row's Account as its counterpart, with the opposite amount. Otherwise the Accept is refused with `invalid-transfer`, reason `not-a-matching-leg`.
   - The ±7-day window applies only to suggestions, not to confirming.
5. **Match.** The Match is confirmed between the two legs, with the caller as `confirmed_by`.
6. **Locking.** All of this happens in one database transaction. The two Account folds are written in Account ID order, so two Transfers can't deadlock. The cash Account's fold row is read `FOR UPDATE` through the Account Decider's version check.

**Recording a cash leg first**

`recordCashTransaction` with `counterpart_account_id`:

- **The counterpart is a non-cash Account.** One leg is recorded, In transit, waiting for the bank row's Accept.
- **The counterpart is a cash Account (cash ↔ cash).** Both legs are recorded, and the Match is confirmed. The counterpart's leg has the opposite amount and the same date, description and payee. Both Accounts must accept the date (no closed month on either).

**Reversing a Transfer leg**

`ReverseTransaction` on a Transfer leg:

- **The leg is Matched.**
  - The Match is removed (`match.removed`), and both legs record `TransactionUnmatched`.
  - **Bank leg, Accepted:** its Imported transaction returns to pending, as today. If the cash leg has `createdbytransfer`, the cash leg is reversed too. A cash leg entered by hand stays, In transit, and can be Matched again.
  - **Cash ↔ cash:** neither leg can stand alone, so reversing either one reverses both.
  - **Cash leg reversed on its own** (an automatic or manual one opposite a bank leg): the bank leg stays Accepted, now In transit.
- **Closed months.** If any Transaction that would be reversed is booked in a closed month of its Account, the whole command is refused (`month-closed`, naming that Account). Nothing is partly reversed.
- **The leg is unmatched.** It is reversed like any other Transaction.

**What Transfers don't do**

- **Month close:** In transit legs never block a close. `AccountMonth.transfer_legs_in_transit` reports them.
- **Spending and income:** Account transfer is Internal, so it never counts as either, and can't be budgeted (ADR-0004).

### Ingestion

- **Accept as Transfer.** The Imported transaction is still **Accepted**: the Transaction linked to it is the bank leg. There is no new Ingestion state. Ingestion's "Matched" state stays reserved for manual ↔ Imported Matching, which this slice doesn't build.
- **The Ledger port** gains `BookTransfer`. It takes the bank row's facts, the cash Account and an optional leg to match, and runs the Ledger steps above in the caller's database transaction.
- **`listTransferSuggestions`** reads through a Ledger port query. Ingestion never reads Ledger tables directly.
- **Reversing the bank leg** returns the row to pending through the existing Ingestion port, unchanged.

### Consistency

Add or update these rows in `docs/architecture/consistency.md`. Each ticket writes its own rows.

| Command | Contexts | Immediate | Deferred |
|---|---|---|---|
| Add Account (cash) | Ledger | Account added with its Opening count (the two counts checked equal first); the Opening balance Transaction recorded on the start date with its Account Posting lookup row; the Account's balance set | — |
| Recount opening | Ledger | Refused unless cash and the start month is the Open month; the old Opening balance Transaction reversed and linked; the new one recorded; the balance moved; nothing recorded for the same count | — |
| Record cash Transaction | Ledger | Account read (refused unless cash; a date before the Open month: `month-closed`; after today: refused); Category checked as in Accept; Transaction recorded and booked in its month, with its Account Posting lookup row. As a Transfer leg: counterpart checked; cash ↔ cash also records the counterpart's leg and confirms the Match, writing both Account folds in Account ID order | Category totals, Budget actuals (not for legs) |
| Accept as Transfer | Ingestion → Ledger | Ingestion's Accept checks as today; through the Ledger port, the bank leg recorded and booked; then a new cash leg recorded (refused in a closed cash month) or the given leg checked as matching; Match confirmed and both legs marked Matched; both Account folds written in Account ID order | — |
| Close month (cash) | Ledger | As Close month, except: `counted_on` required and checked; the balance at the end of `counted_on` compared with the count; the month-end balance stored as the Statement balance, with the count and its date; no Ingestion conditions | — |
| Reopen month (cash) | Ledger | As Reopen month, without the Open month's Accepted-Transactions condition | — |
| Reverse Transaction (Transfer leg) | Ledger → Ingestion | As Reverse Transaction; plus the Match removed and both legs unmatched; an automatic cash leg, or the other cash leg of a cash ↔ cash Transfer, reversed too; refused whole if any of them is in a closed month | — |
| Create Household | Households → Ledger | Adds the Account transfer Internal Category to the seeded ones | — |
| Create Connection | Ingestion | Refused for a cash Account | — |

### Documentation

- **New ADR, "Transfers as two Matched legs".** It records:
  - one leg per Account against the Account transfer Internal Category, linked by a Match;
  - In transit as a legitimate state that doesn't block a close;
  - the automatic cash leg;
  - Reverse cascading only to legs that exist because of the one reversed.

  The rejected option is a single Transaction with two Account Postings. It has one date for both sides, both Accounts' months must be open, and it can't take two imported sides without booking twice.
- **ADR-0018 amendment:** for a cash Account, the Opening count is the outside source of truth. It is typed twice, because no later check can test it, and it can be recounted while the start month is open. The "How cash Accounts work is still open" consequence is resolved.
- **ADR-0024 amendment:**
  - a cash Account takes Transactions in its Open month and any later month;
  - its month closes by a count on a date after the month end;
  - Reopen has no Open-month Transactions condition.
- **`internal/ledger/CONTEXT.md`:**
  - **Posting:** reword. Today it says a transfer has Postings on Accounts only. Now each Transfer leg has a Posting on its Account and one on Account transfer.
  - **Add:**
    - **Cash Account** ("An Account kept entirely by hand, with no statement; its truth is a count.")
    - **Opening count** (avoid: initial cash)
    - **Cash count** (the amount and the date it was counted, given at a cash Month close)
    - **Account transfer** (the Internal Category)
    - **Transfer** (money moving between two of the Household's Accounts; avoid: movement, internal payment)
    - **Transfer leg**
    - **Counterpart Account**
    - **In transit** (avoid: pending, which is Ingestion's word)
  - **Statement balance:** for cash, it is the month-end balance a Cash count proves.
  - **Internal Category:** list Account transfer.
- **`CONTEXT-MAP.md`:** Ingestion → Ledger now mentions Accept as Transfer. Ledger owns transfer Matches.

### Delivery

- **Ticket 01 (the contract) merges first.** Its merge commit is tagged **`v0.9.0`**, and the new operations answer 501 until their tickets land.
- **Order after 01:**
  - 02 → 03 → 04 in sequence;
  - 05 in parallel with 02;
  - 06 after 03 and 05;
  - 07 and 08 after 06;
  - 09 after 05 and 03;
  - 10 any time;
  - 11 last, once the rest is merged.

## Testing Decisions

- **What a good test checks:** behaviour through the public interface. Decider tests go through `Decide`/`Evolve` (`decidertest`). Application behaviour is tested through HTTP integration tests (`apitest`, testcontainers PostgreSQL), and ogen validates every response against the contract. Tests never check internal structure.
- **Decider tests** (`internal/ledger/domain`):
  - **Account:**
    - add cash with a count, a zero count, and a count with too many decimals;
    - add non-cash with a count refused;
    - recount: in the start month, the same count records nothing, and after the start month closes it's refused;
    - booking cash in the Open month, a later month and a closed month;
    - cash close: a count that agrees, a count that is short (difference given), `counted_on` on the month's last day refused, `counted_on` in the future refused, non-cash with `counted_on` refused;
    - cash Reopen with Transactions in the Open month.
  - **Transaction:**
    - a leg with a counterpart;
    - Set Category on a leg refused;
    - Match and Unmatch;
    - matching a Matched, reversed or non-leg Transaction refused.
  - **Match:** confirm and remove; legs with the same sign or mismatched counterparts refused.
- **HTTP tests:**
  - **Cash Account:**
    - add it, record Transactions in two months, close the first by a count taken on the 3rd of the next month, and see the stored month-end Statement balance;
    - a short count is refused with the gap; record the loss against an Expense Category; then the close passes.
  - **Accept as Transfer:**
    - an ATM row as a Transfer to cash: both Accounts' balances move, both lists show the legs with the same `match_id`, and `getMatch` returns both;
    - Category totals don't include Account transfer.
  - **Manual leg first:**
    - record a cash leg naming the bank Account, then upload the statement;
    - `listTransferSuggestions` returns the leg; a leg 8 days away or with another amount doesn't appear;
    - Accept with `match_transaction_id`: no second cash leg is recorded;
    - two concurrent Accepts matching the same leg: exactly one succeeds.
  - **Reversing:**
    - the bank leg with an automatic cash leg: both are reversed, the row is pending, and the Match gets 404;
    - the bank leg with a manual cash leg: the cash leg stays, In transit, and `in_transit=true` lists it;
    - the cash leg alone: the bank leg stays Accepted, In transit;
    - either leg in a closed month: refused, nothing changed.
  - **Cash ↔ cash:** both legs and a Match; reversing one reverses both.
  - **Refusals:**
    - a manual Transaction on a bank Account;
    - a Connection on a cash Account;
    - Accept as Transfer to a bank Account (`counterpart-not-cash`);
    - a Transfer between currencies;
    - the cash leg's month closed;
    - `category_id` with `transfer_account_id`.
  - **Closing with legs in transit:** a month closes with an In transit leg, and `AccountMonth.transfer_legs_in_transit` counts it.
  - **New Household:** it has the Account transfer Internal Category.
- **Row-level security:** an adapter-level test shows that `matches` rows are invisible from another Household.
- **Clock:** through the existing injected time source, never sleeping.
- **Prior art:**
  - the Accept and Reverse tests of `.scratch/statement-import-to-month-close`;
  - the closed-month tests of ADR-0024's tickets (`closed_month_test.go`, `month_reopen_test.go`);
  - the Opening balance tests (`opening_balance_test.go`, `void_opening_balance_test.go`).

## Out of Scope

- **Manual Transactions on statement Accounts,** and therefore Matching a manual entry to an Imported transaction. Statement Accounts take Transactions only from their statements.
- **Statement-to-statement Transfers:** bank ↔ bank, card repayments and loan payments, where both sides are Imported and Matched to each other. That's the Transfers slice.
- **Transfers between Accounts with different Account currencies,** Bank rate and Reference rate (ADR-0002).
- **Household transfers** between two Households.
- **Matching Scheduled transaction occurrences** (Planning).
- **Split cash Transactions** with several Categories.
- **Editing a Transaction's** amount, date, description or payee. Corrections go through Reverse.
- **A standalone "Match these two legs" command.** In this slice, a Match is only confirmed by an Accept or by a cash ↔ cash entry. A leg left In transit by mistake is reversed and recorded again.
- **Rules and Category suggestions** for cash Transactions.
- **The UI.** Ticket 11 writes the hand-off note, and the UI gets its own spec in `wallet-accountant-ui`.

## Further Notes

- **Decided in the grilling on 2026-11-25:**
  - cash Accounts are fully manual, and other Accounts take only statement rows (Q1);
  - the Opening count is typed twice (Q2, Q14);
  - Transfers as two Matched legs against Account transfer, with an automatic cash leg (Q3, Q3a);
  - no editing of Transactions (Q5);
  - a cash Transaction has one date (Q6);
  - cash takes later months and closes by a count with a date (Q7);
  - Recount opening while the start month is open (Q8);
  - the automatic leg is dated on the row's Transaction date (Q9);
  - Match suggestions within ±7 days (Q10);
  - the Reverse cascade (Q11);
  - In transit doesn't block a close (Q12);
  - this slice's Transfer scope (Q13);
  - cash Reopen without the Open-month condition (Q15);
  - the `matches` resource (Q16);
  - no migration (Q17);
  - backend only, plus a UI hand-off note (Q18).
- **Decided while writing, for review:**
  - a cash Transaction can't be dated after today;
  - reversing either leg of a cash ↔ cash Transfer reverses both;
  - `match_transaction_id` isn't limited to the ±7-day window.
- **Accepting before the cash leg's month opens.** A bank row whose Transaction date falls in a cash Account's month that is Not open yet is fine, because cash takes later months. A bank row booked days after the withdrawal can't be Accepted until the bank Account's month opens. That is ADR-0024 as it stands.
- **Settles an open question.** This spec settles the question raised when cash was dropped from the first slice: how a cash Account gets an Opening balance and closes a month.
