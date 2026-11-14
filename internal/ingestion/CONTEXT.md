# Ingestion

Getting a Household's real-world transactions into the Ledger: from statement files and bank feeds, through categorisation, to being Matched or Accepted.

## Language

**Connection**:
A link between an Account and an outside source of its transactions. It is either a statement file import (the main kind) or a live bank feed.
_Avoid_: Integration, sync, link, feed

**Statement import**:
One upload of a bank statement file through a Connection, which produces Imported transactions.
_Avoid_: Upload, file sync

**Imported transaction**:
A transaction brought in through a Connection, as opposed to one a User entered by hand.
_Avoid_: Bank transaction, synced transaction

**Accepted**:
The state of an Imported transaction that a User has confirmed as new because no manual entry corresponds to it, and whose Category the User has confirmed. The system suggests a Category; if it has none, the transaction goes to Uncategorised.
_Avoid_: Approved, cleared

**Rule**:
A Household's instruction for categorising Imported transactions automatically, such as "payee contains MERCADONA → Groceries". When a Rule applies, it takes priority over suggestions learned from past transactions.
_Avoid_: Filter, automation, mapping
