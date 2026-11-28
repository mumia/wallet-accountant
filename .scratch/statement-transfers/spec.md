# Spec: Transfers between statement Accounts

Status: ready-for-agent

Cash Accounts and Transfers (`cash-accounts-and-transfers/`, ADR-0026) built **Transfer legs** and the **Match**, but only with a cash Account on one side. This spec opens Transfers to two **statement** Accounts: bank ↔ bank and card repayments (bank ↔ credit card). Both sides are Imported rows, each Accepted as a Transfer leg, and the two legs are Matched. It was decided with the user on 2026-11-28.

This spec covers only the backend. Once the slice is implemented, the last ticket writes a hand-off note for `../wallet-accountant-ui`. The contract change is **breaking** for the UI (see "Delivery").

## Problem Statement

A Household that moves money between its own bank Accounts, or pays off its credit card from its bank Account, can't record it truthfully. Each side of the move is a row on a statement, and each row can only be Accepted with a Category. So a card repayment counts as spending on the bank side and as income on the card side, even though no money left the Household. The card's purchases were already recorded as spending when they were Accepted on the card, so the repayment counts the same euros twice. A transfer to a savings Account looks like spending, and the savings look like income.

The legs and the Match already exist, but Accept as Transfer refuses any counterpart that isn't cash (`COUNTERPART_NOT_CASH`).

## Solution

**A statement Account as counterpart.** Accept as Transfer takes a `BANK_ACCOUNT` or `CREDIT_CARD` counterpart as well as a cash one, as long as the row's own Account is one of those two kinds too. Unlike cash, the counterpart gets **no automatic leg**: its leg can only come from its own statement. There are three ways to Accept the row:
- **Alone.** The row's leg is recorded **In transit**, waiting for the other side.
- **Matched to a leg.** The other side was already Accepted and is In transit. The row's leg is recorded and Matched to it. This is the existing `match_transaction_id`.
- **Matched to a pending row (pair Accept).** Both statements are imported and the other side is still pending. One command Accepts both rows, records both legs and confirms their Match. This is the new `match_imported_transaction_id`.

**Suggestions** find the other side for the member: In transit legs and pending rows, on any eligible Account when none is named, with the opposite amount, within ±7 days.

**Reversing** a statement leg reverses only that leg. Its Match is removed and the other leg goes back to In transit, however the two were Accepted. Neither leg exists because of the other (ADR-0026).

**Loans and crypto wallets are left out of Transfers entirely.** No Transfer may have a `LOAN` or `CRYPTO_WALLET` Account on either side, with a cash Account or a statement Account. Today a loan or crypto wallet row can be Accepted as a Transfer to cash; this spec takes that away until they get their own feature (see "Out of Scope").

## User Stories

### Accepting Transfers

1. As a member, I want to Accept a card repayment row on my bank Account as a Transfer to my card Account, so that it isn't counted as spending.
2. As a member, I want to Accept a transfer to my savings Account as a Transfer, so that moving money between my Accounts isn't counted as spending or income.
3. As a member, I want to Accept one side of a Transfer before the other Account's statement is imported, so that I don't have to wait for both statements to review a month.
4. As a member, I want Accepting the second side to Match the first, so that the Transfer is complete and nothing stays In transit.
5. As a member, I want to Accept both sides of a Transfer in one step when both statements are imported, so that reviewing a month doesn't mean handling every Transfer twice.
6. As a member, I want a pair Accept refused when the other row's month isn't open, so that a command never quietly does less than I asked.
7. As a member, I want to Accept a row as a Transfer even when the other Account started after it, so that money that's already in that Account's Opening balance doesn't block me.
8. As a member, I want any Transfer with a loan or crypto wallet Account refused, on either side and with cash too, so that I don't record one before the product can handle it.

### Finding the other side

9. As a member, I want the Accept to suggest the other side of a Transfer without me naming the Account first, so that I can see what the row could be.
10. As a member, I want suggestions to include rows still pending on the other Account, so that I can pair-Accept them.
11. As a member, I want to see which Account a suggested row is on, so that I know which Transfer I'm confirming.

### Closed months and reversing

12. As a member, I want a leg in a closed month to be Matched when its other side arrives, so that money moving across a month end can be completed after the first month is closed.
13. As a member, I want reversing one leg to leave the other leg booked, In transit, so that undoing a wrong match doesn't undo a correct Accept on the other Account.

## Implementation Decisions

### Contract

Ticket 01 writes these into `api/openapi.yaml`. Every other ticket implements part of them and must not change the contract without a comment on this spec.

**Changed schemas and operations**

- **`ImportedTransactionAcceptance`** gains `match_imported_transaction_id` (uuid).
  - It needs `transfer_account_id`: alone, it gets 422 `/problems/invalid-transfer`, reason `MATCH_WITHOUT_TRANSFER`.
  - It can't be sent with `match_transaction_id`: both get reason `MATCH_LEG_AND_ROW`.
- **`acceptImportedTransaction`**'s description: `transfer_account_id` names a cash, bank or card Account. For a statement counterpart, no leg is recorded on it. The row's leg stays In transit, or is Matched to `match_transaction_id` (an In transit leg on the counterpart) or to the leg booked for `match_imported_transaction_id` (a pending row on the counterpart, Accepted in the same command). The response is the row's own `ImportedTransaction`. The paired row's state is read with the usual list operations.
- **`listTransferSuggestions`**:
  - `account_id` becomes **optional**. Without it, suggestions come from every eligible counterpart (see "Suggestions").
  - It answers a new schema, **`TransferSuggestionList`**: `{ legs: Transaction[], imported_transactions: ImportedTransaction[] }`, both required, each closest date first. **This replaces `TransactionList` and is breaking.**
  - Its summary and description cover statement Accounts and pending rows.
- **`ImportedTransaction`** gains a required **`account_id`**: the Account of the row's Statement import. This is additive.
- **`TransferProblem.reason`**:
  - `COUNTERPART_NOT_CASH` is **removed**, replaced by **`ACCOUNT_KIND_NOT_SUPPORTED`**: either side of the Transfer is a `LOAN` or a `CRYPTO_WALLET` Account, whatever the other side is. The problem's `account_id` names that Account (the row's own Account is checked first).
  - **`recordCashTransaction`** with a `counterpart_account_id` that is a loan or crypto wallet gets the same reason. Its description and the cash Transfer wording of `acceptImportedTransaction` say so.
  - New: **`MATCH_LEG_AND_ROW`** (both match fields).
  - New: **`NOT_A_MATCHING_ROW`**: `match_imported_transaction_id` names a row that isn't on `transfer_account_id`, isn't pending (Accepted, Discarded, or an undecided Suspected duplicate), is the row being Accepted, or doesn't have the exact opposite amount in the same currency.
  - `NOT_A_MATCHING_LEG` and `BEFORE_START_DATE` keep their meaning. `BEFORE_START_DATE` stays cash-only.
- **Month problems.** A pair Accept whose paired row is booked in a closed month, or a month not open yet, of the counterpart gets the existing 409 `month-closed` or `month-not-open-yet`, naming the counterpart in `account_id`. This works as it does today for a cash leg.

There are no new operations and no new problem types.

**Version:** the merge commit of ticket 01 is tagged **`v0.11.0`**. Under 0.x, a minor version may break. The only break is the `listTransferSuggestions` response.

### Ledger: Transfers between statement Accounts

**The counterpart check** (`checkCashCounterpart` becomes a counterpart check for every kind):
- The checks for the same Account (`SAME_ACCOUNT`), another Household (`UNKNOWN_ACCOUNT`) and another currency (`CURRENCY_MISMATCH`) are unchanged.
- **Kinds:** a Transfer leg's Account and its counterpart must each be `CASH`, `BANK_ACCOUNT` or `CREDIT_CARD`. A `LOAN` or `CRYPTO_WALLET` on either side gets `ACCOUNT_KIND_NOT_SUPPORTED`, naming it, the leg's own Account checked first. This applies to Accept as Transfer, `recordCashTransaction` with a counterpart, and suggestions.
- **Where it's enforced:** in the Transaction Decider's leg rules, beside the existing same-Account and currency rules, so no path can record a leg with those kinds. The app maps the Decider's error to the reason.
- **Existing legs:** there's no production data, so no existing loan or crypto wallet leg needs handling.

**`BookTransfer`** gains the statement cases. `ImportedTransfer`'s `CashAccount` and `CashLeg` become `Counterpart` and `CounterpartLeg`, and it gains `MatchRow *ImportedBooking`.
- **Cash counterpart:** unchanged.
- **Statement counterpart, alone:** only the row's leg is recorded, naming the counterpart. It's In transit. No Match.
- **Statement counterpart, `MatchLeg`:** `matchingLeg` as today: an In transit leg on the counterpart, naming the row's Account, with the opposite amount and no date window. The row's leg is recorded, then the Match confirmed.
- **Statement counterpart, `MatchRow`:** both legs are recorded, each exactly as a plain Accept records its row: its own amount, dates, description and payee, linked to its own Imported transaction, against Account transfer, naming the other Account. Then the Match is confirmed. The two Account folds are written in Account ID order, as for cash.
- **The automatic leg** (`createdbytransfer`) is only ever recorded for a cash counterpart.

**Matching across a closed month.** Confirming or removing a Match records `TransactionMatched` or `TransactionUnmatched` on each leg's own stream. It changes no amount, description or Posting, so it is allowed when a leg is booked in a closed month (ADR-0024 amendment below). `ConfirmTransfer` doesn't check months today. Tickets 02 and 05 pin this with tests, and the code shouldn't start checking.

**Reversing a statement leg.** The existing rules already give the decided outcome. The other leg has no `createdbytransfer`, so it's never reversed with the reversed leg. It records `TransactionUnmatched` and stays booked, In transit, even in a closed month. The reversed leg's Imported transaction returns to pending. Only the reversed leg's own month is checked against closed months. This is the same for legs Accepted alone, Matched later, or pair-Accepted.

**A row from before the counterpart's start date.** There is no check for a statement counterpart: nothing is recorded on it. The leg may stay In transit for good, if the money was already in the counterpart's Opening balance. In transit never blocks a close.

### Ingestion

**Accept as Transfer with `match_imported_transaction_id`:**
1. Both body checks (`MATCH_WITHOUT_TRANSFER`, `MATCH_LEG_AND_ROW`) are made before any row is read.
2. The row being Accepted gets its usual checks: pending, in its Account's Open month (ADR-0024).
3. **The paired row** is loaded in the same database transaction. It must be pending, on `transfer_account_id`, not the same row, and have the exact opposite amount and currency. Otherwise `NOT_A_MATCHING_ROW`. Its Booking month must be its Account's Open month: `month-closed` or `month-not-open-yet`, naming its Account. There's no date window, as with `match_transaction_id`.
4. Both rows record `Accepted`, each linked to its own leg. The two Imported transaction folds are written in ID order, so two crossing pair Accepts can't deadlock. A concurrent Accept of either row is caught by that row's version check.
5. The Ledger port's `BookTransfer` books both legs and the Match, as above, in the same database transaction.
6. Nothing is booked and both rows stay pending if any step is refused.

There is no new Ingestion state: both rows are **Accepted**. Ingestion's "Matched" state stays reserved for Matching a manual entry to an Imported transaction.

**`ImportedTransaction.account_id`** comes from the row's Statement import, which already knows its Account. Every operation that returns an `ImportedTransaction` returns it. If the read model lacks the column, a migration adds it and backfills it from the Statement import.

**Suggestions** (`listTransferSuggestions`):
- **Eligible counterparts:** the Household's other Accounts in the row's currency that pass the counterpart check above. With `account_id`, only that one: one that fails the check gets the check's 422.
- **`legs`:** In transit legs on an eligible counterpart, naming the row's Account, with the opposite amount, Transaction date within ±7 days of the row's. Read through the Ledger port query, widened to many Accounts.
- **`imported_transactions`:** pending rows on an eligible **statement** counterpart, with the opposite amount, Transaction date within ±7 days. Rows booked in a month that isn't open are included: the Accept will say why it can't pair them (Q14). Undecided Suspected duplicates aren't pending, so they aren't included. Ingestion reads these from its own tables.
- Both lists are closest date first, then by ID. They're worked out on each request and never stored.

### Consistency

Update these rows in `docs/architecture/consistency.md`:

| Command | Contexts | Immediate | Deferred |
|---|---|---|---|
| Accept as Transfer | Ingestion → Ledger | Ingestion's Accept checks as today. With a paired row: that row checked (pending, on the counterpart, opposite amount, counterpart's Open month) and Accepted too, both rows' folds written in ID order. Through the Ledger port: the counterpart checked; for cash, as today; for a statement counterpart, the row's leg recorded, plus the paired row's leg if there is one, then the Match confirmed with the given leg or the paired leg, or no Match (In transit); both Account folds written in Account ID order | — |
| Reverse Transaction (Transfer leg) | Ledger → Ingestion | As today. A statement counterpart's leg is never reversed with it: it's unmatched and stays In transit, even in a closed month | — |

### Documentation

- **ADR-0026 amendment:**
  - statement counterparts: no automatic leg; Accept alone, Matched to an In transit leg, or paired with a pending row in one command;
  - the reverse rule, applied: a statement counterpart's leg never exists because of the other, so it always stays;
  - Matching or unmatching a leg booked in a closed month is allowed;
  - for now, Transfers involve only `CASH`, `BANK_ACCOUNT` and `CREDIT_CARD` Accounts; no Transfer has a loan or crypto wallet on either side;
  - the "Statement-to-statement Transfers can reuse the legs" consequence is resolved.
- **ADR-0024 amendment:** a close locks amounts, descriptions and Postings, not Match state.
- **`internal/ingestion/CONTEXT.md`, Accepted:** add "Two rows on two of the Household's statement Accounts can be Accepted together as the two legs of one Transfer."
- **`internal/ledger/CONTEXT.md`, Transfer leg:** add "Between two statement Accounts, each leg comes from its own Account's statement."
- **`CONTEXT-MAP.md`, Ingestion → Ledger:** Accepting a row as a Transfer to a statement Account books its leg, and the paired row's leg when two rows are Accepted together, and confirms the Match.

### Delivery

- **Ticket 01 (the contract) merges first and is tagged `v0.11.0`.**
  - The existing handler is changed to answer `TransferSuggestionList` (legs only, `imported_transactions: []`), because ogen validates every response.
  - It also starts returning `account_id` on every `ImportedTransaction`, or the response fails validation. If that needs the migration, it lands here.
  - `match_imported_transaction_id` answers 501 until ticket 04.
- **Breaking for the UI.** The UI's cash flow reads `listTransferSuggestions`'s `items`. Don't deploy `v0.11.0` ahead of the UI change that reads `legs`. Production isn't running yet, so only the local cluster is affected. The hand-off note puts this first. It also says the UI must stop offering a Transfer on a loan or crypto wallet row, or to one, which cash allowed before.
- **Planned tickets:**
  - 01: contract, the new response shape, `account_id`, `v0.11.0`;
  - 02: statement counterpart Accept, alone and with `match_transaction_id`; `ACCOUNT_KIND_NOT_SUPPORTED` on every Transfer path, cash included; Matching into a closed month; reversing either leg;
  - 03: suggestions: optional `account_id`, legs across Accounts, pending rows;
  - 04: pair Accept;
  - 05: docs (ADR-0026, ADR-0024, glossaries, CONTEXT-MAP, consistency rows not written by 02–04);
  - 06: UI hand-off note.
- **Order:** 01 → 02 → (03, 04 in parallel) → 06. 05 any time after 01.

## Testing Decisions

- **What a good test checks:** behaviour through the public interface. Decider tests go through `Decide`/`Evolve` (`decidertest`). Application behaviour is tested through HTTP integration tests (`apitest`, testcontainers PostgreSQL), and ogen validates every response against the contract. Tests never check internal structure.
- **HTTP tests:**
  - **Accept alone, then Match:** a −500 row on a bank Account Accepted as a Transfer to a card Account. The bank balance moves, the card's doesn't, and `in_transit=true` lists the leg. Import the card statement, Accept its +500 row with `match_transaction_id`: both legs share a `match_id`, `getMatch` returns both, and Category totals don't include Account transfer.
  - **Bank ↔ bank:** the same flow between two `BANK_ACCOUNT`s.
  - **Pair Accept:** both statements imported, one Accept with `match_imported_transaction_id`. Both rows are Accepted, both balances move, both legs share one `match_id`, and nothing is In transit.
  - **Pair refusals** (nothing booked, both rows still pending):
    - the paired row on another Account;
    - the paired row Accepted, Discarded or an undecided Suspected duplicate;
    - the same row;
    - another amount;
    - both match fields;
    - the paired row's month closed (`month-closed` naming the card Account), or not open yet (`month-not-open-yet`).
  - **Concurrency:** two pair Accepts sharing one paired row: exactly one succeeds.
  - **Closed months:** Accept the bank row on 31 March alone, close March, then Accept the card row on 2 April with `match_transaction_id`. The Match is confirmed. Reverse the card leg: it's allowed, the March leg is unmatched and In transit, and March stays closed.
  - **Reversing:** reversing either leg of a pair-Accepted Transfer leaves the other leg booked and In transit, returns only the reversed leg's row to pending, and the Match gets 404.
  - **Kinds:** `ACCOUNT_KIND_NOT_SUPPORTED`, naming the right Account, for:
    - a bank row with a loan counterpart, and with a crypto wallet counterpart;
    - a loan row with a bank counterpart, and with a cash counterpart (allowed before this slice);
    - a crypto wallet row with a cash counterpart;
    - `recordCashTransaction` with a loan counterpart.
  - **Before the counterpart's start date:** a bank row dated before the card Account's start date is Accepted, In transit.
  - **Suggestions:**
    - without `account_id`: a leg on a cash Account and a pending row on a card Account both appear;
    - an Account in another currency, a loan Account, a row 8 days away, a row with another amount and an undecided Suspected duplicate don't;
    - with `account_id` of a loan: `ACCOUNT_KIND_NOT_SUPPORTED`; for a row on a loan Account: the same;
    - order: closest date first;
    - every suggested row carries its `account_id`.
  - **`account_id` on imported transactions:** returned by listing and by Accept.
  - **Cash flows unchanged:** the existing Accept as Transfer and suggestions tests pass with the new response shape.
- **Clock:** through the existing injected time source, never sleeping.
- **Prior art:**
  - the Transfer tests of `cash-accounts-and-transfers/` (Accept as Transfer, manual leg and suggestions, reversing legs);
  - the closed-month tests (`closed_month_test.go`, `month_reopen_test.go`).

## Out of Scope

- **Transfers with a loan or crypto wallet Account,** on either side, cash included. Loans need the interest in a payment handled, and crypto wallets need a look at how their statements arrive. This is item 4 of `next-steps.md`.
- **A standalone "Match these two legs" command** (`POST /matches`). Two legs left In transit by mistake are fixed by reversing one and Accepting it again while its month is open. This is item 5 of `next-steps.md`.
- **A Transfer whose amounts differ,** such as a fee taken inside the row. That row isn't a Transfer and is Accepted with a Category.
- **Transfers between Accounts with different Account currencies** (ADR-0002).
- **Pointing out likely Transfers at import time.** Suggestions are asked for, row by row.
- **Household transfers,** split Transactions, and Matching a manual entry to an Imported transaction.
- **The UI.** Ticket 06 writes the hand-off note, and the UI gets its own spec in `wallet-accountant-ui`.

## Further Notes

- **Decided in the grilling on 2026-11-28:**
  - bank ↔ bank and cards only; loans and crypto wallets later (Q1, Q9), and left out of every Transfer, cash included (decided at spec review);
  - cards have their own statement files (Q2);
  - widen Accept as Transfer instead of a new endpoint (Q3);
  - the pair Accept, with Accepting alone still possible (Q4);
  - Matching and unmatching across a closed month allowed (Q5);
  - no start-date check for a statement counterpart (Q6);
  - ±7-day suggestions, closest first (Q7);
  - backend spec, then a UI hand-off and a UI spec (Q8);
  - exact opposite amounts only (Q10);
  - suggestions without naming the Account (Q11);
  - reversing one leg leaves the other In transit (Q12);
  - two separate match fields (Q13);
  - a pair with a row in a month that isn't open is refused, no fallback (Q14);
  - no standalone Match command (Q15);
  - no new glossary term (Q16);
  - the docs to amend (Q17);
  - the breaking `{ legs, imported_transactions }` response (Q18);
  - `account_id` on every `ImportedTransaction` (Q19).
- **Decided while writing, for review:**
  - the reason is `ACCOUNT_KIND_NOT_SUPPORTED`, not `COUNTERPART_NOT_SUPPORTED`, because the refused Account can be the row's own; it's enforced in the Transaction Decider;
  - the paired row's month must be its Account's Open month: Accept's own rule, applied to the second row;
  - the paired row has no date window, like `match_transaction_id`;
  - suggestions include pending rows in a month that isn't open, so the Accept can say why it can't pair them;
  - the response to a pair Accept is only the row's own `ImportedTransaction`.
