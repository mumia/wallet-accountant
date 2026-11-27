# 01: A Category's kind must fit the amount's sign

**What to build:** the Ledger refuses an Expense Category on money in and an Income Category on money out, wherever a User chooses a Category, with a new 422 `/problems/category-direction`. Released as `v0.10.0`.

**Blocked by:** —

**Status:** resolved

**Asked by:** wallet-accountant-ui, while grilling its cash-accounts-and-transfers spec (`../wallet-accountant-ui/.scratch/cash-accounts-and-transfers/spec.md`, "The sign rule"). The UI's ticket 11 waits for the release that carries this. The decisions below were settled by grilling on 2026-11-26.

## The rule

When a User chooses a Category in `acceptImportedTransaction`, `changeTransaction` (Set Category) or `recordCashTransaction`, on every kind of Account:

- **Expense Category:** only when the **Account Posting** is negative (money out).
- **Income Category:** only when the Account Posting is positive (money in).
- **Uncategorised:** either way. The other Internal Categories are already refused by these three operations, so nothing changes there.
- **A zero amount:** either kind. It has no direction. Statement rows can be 0.00; cash Transactions already refuse zero.
- **Refunds:** no special case. A refund is money in, so it goes to an Income Category, for example one the Household calls "Refunds".
- **Closed months:** no exemption. Setting a Category in a closed month (ADR-0024) follows the rule too.
- **Reversals:** outside the rule. A Reversal copies its original's Category with the opposite sign, and nobody can choose a Category for it, because `changeTransaction` refuses a reversal. The rule is about *choosing* a Category, not an invariant checked on every Transaction.

**Sign trap:** the direction comes from the Account Posting's amount. The Category Posting carries its negation (`b.Amount.Neg()` in `RecordImported`, `c.Amount.Neg()` in cash). In `changeTransaction`, read the sign from the Posting with an `AccountID`.

**Why:** filing a salary under Groceries, or a grocery bill under Salary, is always a mistake. The UI filters every picker by the sign, but the rule belongs to the ledger, so that it holds whatever the client.

## Where the check lives and the order of refusals

- **One domain function** in `internal/ledger/domain` decides whether a kind fits an amount: it returns `ErrCategoryDirection` with the kind and the direction. All three operations call it.
- **`chooseCategory`** (`internal/ledger/app/corrections.go`) keeps its current checks (Uncategorised shortcut, not found, Internal) and also returns the chosen Category's kind. It doesn't check direction itself.
- **The direction check runs after the Transaction-state refusals**, so a Transaction that can't take any Category says so, rather than complaining about this Category:
  - **`changeTransaction`:** inside the `SetTransactionCategory` decider, after its existing refusals: `transaction-reversed`, `transaction-is-a-reversal`, `opening-balance-transaction` and `uncategorised-in-closed-month`. The command carries the chosen kind.
  - **`acceptImportedTransaction`:** in `RecordImported`, before anything is booked. The month 409s (`month-closed`, `month-not-open-yet`) that Ingestion checks first keep winning, as today.
  - **`recordCashTransaction`:** before anything is booked, after the existing Category checks. The `invalid-transfer` checks and `invalid-transaction` (zero amount and so on) keep their place in front of it.
- **Unchanged:** `transfer-leg` is still checked first in `changeTransaction`, and Transfers (`transfer_account_id`, `counterpart_account_id`) never meet the rule.

## The problem

422 with `type` `/problems/category-direction`, and these fields beyond a Problem's:

- `detail`: names the kind and the sign, for example "an Expense Category can only take money out, and this amount is money in".
- `category_id`: the Category refused.
- `category_kind`: `INCOME` or `EXPENSE`.
- `direction`: the amount's direction, `IN` or `OUT`.

It's mapped in the Ledger HTTP adapter (`changeTransaction`, `recordCashTransaction`) and in the Ingestion HTTP adapter (`acceptImportedTransaction`, where the Ledger's error comes back through `RecordImported`). Today the 422 of Accept and cash is `TransferRefused` (`TransferProblem`), and `changeTransaction`'s is `UnprocessableEntity`. Give the new fields a schema, and make all three 422 responses able to carry it, without breaking the existing `reason` on `TransferProblem`. Note the chosen shape under `## Comments`.

## Acceptance criteria

- [x] Domain: the direction function, with unit tests for Expense/Income × in/out/zero, and Uncategorised taking either direction.
- [x] `SetTransactionCategory` decider: refuses a wrong-direction kind after its existing refusals. Domain tests cover the precedence: a reversal or a reversed Transaction with a wrong-direction Category still gets its own refusal, and so does a closed-month change to Uncategorised.
- [x] `chooseCategory` returns the kind, and all three call sites apply the check before booking.
- [x] Integration tests, one or more per operation:
  - [x] Accept: an Expense Category on a positive row and an Income Category on a negative row get 422 `category-direction` with all fields, and nothing is booked (the row stays pending). A 0.00 row takes either kind. A row in a closed month still gets 409 `month-closed`.
  - [x] `changeTransaction`: refused the same way, including in a closed month. A reversal still gets `transaction-is-a-reversal`.
  - [x] `recordCashTransaction`: refused the same way, and nothing is recorded.
  - [x] Existing tests and fixtures that book a wrong-direction Category are fixed to book a fitting one, not exempted.
- [x] `api/openapi.yaml`:
  - [x] the new problem type and its schema;
  - [x] each of the three operations' descriptions names the refusal (it's part of "Accept's Category rules", which cash and `changeTransaction` refer to);
  - [x] `make generate`, with the generated code and the Bruno collection committed.
- [x] `internal/ledger/CONTEXT.md`: the rule added under **Category**, saying that an Income Category takes only money in and an Expense Category only money out, and that a zero amount takes either.
- [x] `docs/adr/0004-accounts-hold-money-categories-explain-it.md`: a paragraph saying the rule follows from what Income and Expense mean, and that later Rules and Category suggestions (Ingestion) and Scheduled transactions (Planning) must respect it.
- [x] `make lint` and `make test-integration` pass.
- [ ] After merge: annotated tag `v0.10.0` on the merge commit.

## Existing data

Ignore it. No migration or clean-up is needed, because all current data can be deleted at this stage. Wrong-direction Transactions already booked stay as they are, and the rule only applies to the next Category chosen for them.

## Not asked

- Any change to Transfers, or to the Internal Categories.
- A special case for refunds.
- Bulk Accept: the UI makes one call per row, so each row is refused on its own.

## Comments

2026-11-26, implementation notes:

- **Problem shape:** the three new fields are optional properties of `TransferProblem`, the schema already used by the 422 of Accept and cash (it now describes "a request that can't be carried out", not only Transfers). `changeTransaction`'s 422 moved from `UnprocessableEntity` (`Problem`) to `TransferRefused` (`TransferProblem`), so all three share one shape. On the wire this is a superset of the old body, but `oas.ChangeTransactionUnprocessableEntity` is gone from the generated code. `httpapi.CategoryDirection` builds the problem for both adapters.
- **Domain:** `domain.FitsDirection(kind, amount)` returns a `*domain.DirectionError` (`ErrCategoryDirection`) with the kind and the `Direction` (`in`/`out`). `SetTransactionCategory` carries `Kind`. Its decider checks direction after `correctable`, `transfer-leg`, `transaction-reversed`, a missing Category and a split, and before the "same Category, no change" case. Re-choosing a wrong-direction Category that is already set is refused too. An empty `Kind` fits either way.
- **`uncategorised-in-closed-month`** is checked by the Account Decider *after* the Transaction Decider. It can never collide with the direction check, because Uncategorised fits either way.
- **Cash:** the direction check runs after `chooseCategory` and before booking, so it comes after `not-a-cash-account`, `invalid-transaction`, `invalid-transfer`, `internal-category` and `invalid-category`. A date in a closed month with a wrong-direction Category gets `category-direction`, because the month is checked when the Account books. That matches where `internal-category` already sat.
- **Accept:** Ingestion keeps its month checks first. `wiring/ledgerport.go` turns the Ledger's `*DirectionError` into `*ingestionapp.CategoryDirectionError` with the contract's names.
- **Fixtures:** Ingestion tests that Accepted Salary (and Bonus) into the Expense Category Housing now use an Income Category, Pay. `januaryClosed`'s Salary too.
- **Docs:** besides the glossary and ADR-0004, `docs/architecture/consistency.md` names the check for Record cash Transaction, Accept imported transaction and Set Transaction Category.

