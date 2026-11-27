# 01: Refuse a Transfer leg booked before its cash Account's start date with a 422, not a 500

**What to build:** when Accept as Transfer (or a cash ↔ cash Transfer) would book the cash leg on a date before the cash Account's start date, the API answers 422 `/problems/invalid-transfer` with a new `reason`, `BEFORE_START_DATE`, instead of failing with a 500.

**Blocked by:** —

**Status:** ready-for-agent

## How it was found

2026-11-27: a bank row dated 2026-11-02 was accepted as a Transfer to a cash Account whose start date is 2026-11-27. The server logged:

```
"level":"ERROR","msg":"unexpected handler error","error":"accept imported transaction: accept imported transaction 01a21df8-…: in household 01a21df7-…: account 01a21df9-…: book transfer leg: booked before the account's start date: 2026-11-02 is before 2026-11-27"
```

## What happens today

- The refusal itself is right: a cash Account can't hold money before its start date (`domain.ErrBookedBeforeStartDate`, `internal/ledger/domain/account.go`). The automatic cash leg takes the bank row's date, so the Account Decider refuses it.
- `RecordTransferLeg` (`internal/ledger/app/transfers.go`) returns it as a plain `*AccountError` ("book transfer leg"). It isn't a `*TransferRefusedError`, so `internal/wiring/ledgerport.go` doesn't translate it for Ingestion.
- `AcceptImportedTransaction` (`internal/ingestion/adapters/http/importedtransactions.go`) has no case for it, so it falls through to `case err != nil` and returns a 500.
- The cash-accounts-and-transfers spec ("Ledger: Transfers") never lists this refusal for Accept as Transfer. The only start-date refusal it names is for a cash Transaction's own date.

## The fix

- **Ledger:** add `TransferBeforeStartDate` (`BEFORE_START_DATE`) to the Transfer refusal reasons. When booking a leg on the **counterpart** cash Account fails with `ErrBookedBeforeStartDate`, return `&TransferRefusedError{Reason: TransferBeforeStartDate, Err: …}`. Keep the `*AccountError` in the chain so the Account it is about stays known.
  - Accept as Transfer: the automatic cash leg.
  - `recordCashTransaction` as a cash ↔ cash Transfer: the counterpart's leg. The Account's **own** date before its own start date stays 422 `invalid-transaction`, as today.
  - A statement row dated before its own Account's start date can't be imported, so the bank leg needs no new case.
- **Ingestion:** nothing new is needed. The existing `TransferRefusedError` translation in `ledgerport.go` and `transferRefused` in the HTTP adapter carry the new reason through. Add a `transferDetails` entry that names both dates and the way out, for example: "The row's date, 2026-11-02, is before the cash Account's start date, 2026-11-27: the cash Account didn't exist yet. Accept the row against an Expense Category, such as cash spending, or as a Transfer to a cash Account that started on or before the row's date."
- **Problem fields:** also set `account_id` to the cash Account, the same way the month problems name the Account they are about (spec: "For an Accept as Transfer, that can be the cash Account"). If `TransferProblem` has no `account_id` yet, add it as an optional field.
- **Order of refusals:** the existing `invalid-transfer` checks (counterpart not cash, same Account, currency, unknown Account) and the month 409s run first, as today. `BEFORE_START_DATE` comes from booking the leg, so it is checked after them. Nothing is booked: the row stays pending and the bank leg isn't recorded, because the whole Accept is one database transaction.

## Acceptance criteria

- [ ] Ledger: `RecordTransferLeg` returns `TransferRefusedError{BEFORE_START_DATE}` for a cash leg dated before the cash Account's start date. App-level tests cover Accept as Transfer and cash ↔ cash.
- [ ] Integration tests:
  - [ ] Accept as Transfer of a row dated before the cash Account's start date gets 422 `/problems/invalid-transfer`, `reason` `BEFORE_START_DATE` and `account_id` set to the cash Account. Nothing is booked: the row stays pending, and neither Account's balance changes.
  - [ ] A row dated **on** the start date is accepted.
  - [ ] `recordCashTransaction` cash ↔ cash, dated before the counterpart's start date but not the Account's own, gets the same 422. Before the Account's own start date it still gets `invalid-transaction`.
- [ ] `api/openapi.yaml`: `BEFORE_START_DATE` added to `TransferProblem.reason`, `account_id` added if missing, and the refusal named in the descriptions of `acceptImportedTransaction` and `recordCashTransaction`. Run `make generate` and commit the generated code and the Bruno collection.
- [ ] `.scratch/cash-accounts-and-transfers/spec.md`: add the refusal to "Ledger: Transfers" and to the `TransferProblem` reason list.
- [ ] `make lint` and `make test-integration` pass.
- [ ] Hand-off to `wallet-accountant-ui` (see below).
- [ ] After merge: annotated patch tag (`v0.10.1`, unless something else ships first).

## For the UI

The UI should not offer something that can only fail: in the Transfer Account picker, leave out (or disable, with the reason) any cash Account whose `start_date` is after the row's date. Accounts already return `start_date`, so no new API is needed. The Ledger still refuses on its own, because the rule belongs to it whatever the client (the same principle as `category-direction`). The UI should show the new `reason` if it ever gets it, for example from a stale Account list or another client. Send this as a `needs-triage` note to the UI repo's `.scratch/`.

## Not asked

- Changing a cash Account's start date, or removing an Account. Both are fixed today (`changeAccount` only renames), so a member who started tracking cash too late has no way to move the start date. That may deserve its own ticket.
- Booking the cash leg on a later date than the bank row.

## Comments

### 2026-11-27: settled by grilling

- **Refuse it.** A cash leg dated before the cash Account's start date gets 422 `BEFORE_START_DATE`. Booking the leg on the start date instead was rejected: the Opening count may already hold that cash, so it would be counted twice. Letting the member choose the cash leg's date was rejected as a contract change for a rare case. Both stay under "Not asked".
- **Where the row goes instead:** an ordinary Expense Category, such as cash spending. From the bank's side, that cash left the money being tracked. Members still can't pick an Internal Category (Opening balance). The `detail` says so and names both dates.
- **Problem fields:** only `account_id`, set to the cash Account. No `start_date`: the dates go in `detail`, and clients already get `start_date` on the Account.
- **Changing a cash Account's start date:** no separate ticket for now. It stays under "Not asked".
- **Order of checks:** `bookTransaction` checks the start date before the month rules, so a cash leg dated in a month before the start month gets `BEFORE_START_DATE`, never `month-closed`. The `match_transaction_id` path needs nothing new, because a cash leg entered by hand can't be dated before its own start date.
- **Release:** patch tag `v0.10.1`.
