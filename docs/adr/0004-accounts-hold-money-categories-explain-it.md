# Accounts hold money; Categories explain it

Standard double-entry treats every bookable thing as an "account": bank accounts, but also Groceries, Salary and Equity. We split these into two concepts instead. An **Account** is a place money is held or owed. A **Category** explains what money was received for or spent on, and has a kind: Income, Expense or Internal. A Posting points at either an Account or a Category. We chose this because it matches how Users, the frontend and future contributors already talk ("account" always means a bank account, card or cash), and because each concept has its own set of facts: only Accounts have a currency, a Connection, a Statement balance and a Month close, and only Categories have Budgets.

## Considered Options

- **Everything is an Account, each with a type (Asset, Liability, Income, Expense, Equity).** This is the usual model in ledger tools. We rejected it because "account" would mean two different things in conversation and in code, and half the Account fields would be meaningless for half the types.

## Consequences

- Bookings that are neither income nor spending (Opening balances, Household transfers, Uncategorised) live in system-created **Internal Categories**, which play the role of Equity. They cannot be budgeted or deleted.
- The ledger engine must treat Accounts and Categories the same way when balancing and summing Postings, so the code needs one shared interface for "something a Posting can point at".
