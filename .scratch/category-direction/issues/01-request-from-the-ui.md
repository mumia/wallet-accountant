# 01: Request from the UI: a Category's kind must fit the amount's sign

Status: needs-triage
Blocked by: —
Asked by: wallet-accountant-ui, while grilling its cash-accounts-and-transfers spec (`../wallet-accountant-ui/.scratch/cash-accounts-and-transfers/spec.md`, "The sign rule"). The UI's ticket 11 waits for the release that carries this.

## The rule

On every kind of Account:

- **Expense Category:** only on money **out** (a negative amount on the Account).
- **Income Category:** only on money **in** (a positive amount).
- **Uncategorised and Account transfer:** either way. So are the other Internal Categories where they're reachable today.
- **Refunds have no special case:** a refund is money in, so it goes to an Income Category, for example one the Household calls "Refunds".

## Why

Filing a salary under Groceries, or a grocery bill under Salary, is always a mistake. The UI now filters every picker by the sign, but the rule belongs to the ledger, so that it holds whatever the client.

## What the UI asks for

- [ ] **Refuse a Category whose kind doesn't fit** in:
  - `acceptImportedTransaction`, including bulk use, which is one call per row;
  - `changeTransaction` (Set Category);
  - `recordCashTransaction`.
- [ ] **A problem type** for the refusal, for example 422 `/problems/category-direction`, with a `detail` naming the kind and the sign.
- [ ] **Release:** the new problem type documented in `api/openapi.yaml` on those three operations, and a release tag.
- [ ] **The glossary:** the rule added to `internal/ledger/CONTEXT.md` (Category / Expense / Income).

## Existing data

Ignore it. No migration or clean-up is needed, because all current data can be deleted at this stage.

## Not asked

- Any change to Transfers, or to the Internal Categories.
- A special case for refunds.
