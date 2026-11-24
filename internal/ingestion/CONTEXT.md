# Ingestion

Getting a Household's real-world transactions into the Ledger: from statement files and bank feeds, through categorisation, to being Matched or Accepted.

## Language

**Connection**:
A link between an Account and an outside source of its transactions, named uniquely within its Account. It is either a statement file import (the main kind) or a live bank feed.
_Avoid_: Integration, sync, link, feed

**Statement import**:
One upload of a bank statement file through a Connection, which produces Imported transactions.
_Avoid_: Upload, file sync

**Export balance**:
The bank's booked balance, excluding card authorisations not yet booked, when a statement file was exported. Every Statement import carries one. The first import works out the Account's Opening balance from it, and every later import is checked against it (ADR-0018). When the Connection's mapping reads a balance column, every Export balance must also equal the file's balance.
_Avoid_: Current balance, available balance, closing balance

**Balance check**:
The test that an Account's Opening balance plus all its non-Discarded Imported transactions equals a Statement import's Export balance. A failed check refuses the import. When the import produced Suspected duplicates, the check waits until they are decided. A file's rows booked in a closed month are ignored, so a row missing from a closed month shows up as a failed check (ADR-0024).
_Avoid_: Reconciliation, validation

**File's balance**:
The balance on a statement file's newest row, read from its mapping's balance column. The newest row is told by the balances alone: the rows are oldest first when the last row's balance is the first row's plus every later row's amount, and newest first the other way round. Every readable row counts, including rows the import ignores. Through such a mapping, an Export balance that isn't the file's balance, or a file whose balance can't be worked out, refuses the import.
_Avoid_: Closing balance, statement balance

**Void**:
Withdrawing the latest Statement import of an Account, while none of its rows are Accepted and none fall in a closed month, so a corrected file can be uploaded. Its Imported transactions become Discarded.
_Avoid_: Delete, undo, rollback

**Imported transaction**:
A transaction brought in through a Connection, as opposed to one a User entered by hand.
_Avoid_: Bank transaction, synced transaction

**Pending**:
The state of an Imported transaction waiting for a User to Accept or Discard it. A Suspected duplicate becomes pending when it is kept, and an Accepted one returns to pending when its Transaction is reversed. A pending row booked in a month that is Not open yet waits, untouchable, until that month opens.
_Avoid_: Unreviewed, new, open, unconfirmed

**Accepted**:
The state of an Imported transaction that a User has confirmed as new because no manual entry corresponds to it, and whose Category the User has confirmed. The system suggests a Category; if it has none, the transaction goes to Uncategorised.
_Avoid_: Approved, cleared

**Rule**:
A Household's instruction for categorising Imported transactions automatically, such as "payee contains MERCADONA → Groceries". When a Rule applies, it takes priority over suggestions learned from past transactions.
_Avoid_: Filter, automation, mapping

**Suspected duplicate**:
An Imported transaction that looks like one already imported for the same Account, but not certainly. It waits until a Household member decides whether to keep it as a real transaction or Discard it, which can be decided even in a month that is Not open yet. When the bank's own transaction ID proves a repeat, it is skipped at import and never becomes a Suspected duplicate. A Match links two records of one money movement from different sources; a Suspected duplicate is the same source repeating itself.
_Avoid_: Duplicate, conflict, possible match

**Discarded**:
The state of an Imported transaction that must never reach the Ledger: a Suspected duplicate confirmed as a repeat, a pending row a Household member rejected, or a row of a voided Statement import. It does not block a Month close and does not count in a Balance check.
_Avoid_: Deleted, ignored, rejected
