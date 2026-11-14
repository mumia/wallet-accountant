# Double-entry ledger internally, single-entry view externally

Internally every Transaction is a set of Postings across Accounts and Categories that sums to zero. The API and UI show the familiar single-entry view: a list of transactions per Account with signed amounts. We picked double-entry over a plain per-Account transaction list because the product needs transfers between Accounts, payments split across Categories, currency conversion within and between Accounts, and possibly investments later. In single-entry, each of those becomes a special case with linking rules that can quietly break (an unlinked credit card payment counts as spending twice). In double-entry they are all ordinary balanced Transactions, and the ledger cannot drift out of balance.

## Consequences

- Every import must be turned into a balanced Transaction. A bank line only knows one side, so the other side starts in the Uncategorised Category until a User categorises it.
- The API has to translate between the two views. Consumers should never need to understand Postings to list an account's transactions.
