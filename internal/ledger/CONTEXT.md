# Ledger

The Household's books: where money is held, what it was for, every Transaction that moved it, and whether each Account's month agrees with reality.

## Accounts

**Account**:
A place where money is held or owed, such as a bank account, a credit card, a loan, cash or a crypto wallet. It always belongs to exactly one Household.
_Avoid_: Wallet, pocket, source, asset account

**Start date**:
The day an Account's history in Wallet Accountant begins, and the date of its Opening balance. Bank rows booked before it are ignored.
_Avoid_: Opening date, Opening balance date, account opening date

**Account currency**:
The single currency an Account's balance is kept in. Money arriving in another currency is converted into it.
_Avoid_: Working currency, wallet currency

**Opening balance**:
The balance an Account had on its start date, worked out from its first Statement import (the Export balance minus the file's rows) and recorded against an Internal Category. It is never typed (ADR-0018).
_Avoid_: Initial balance, starting balance

## Categories

**Category**:
What money was received for or spent on. A Category belongs to a Household, has one kind (Income, Expense or Internal), and can be nested to any depth, for example Food › Eating out › Coffee.
_Avoid_: Tag, type, bucket, expense account

**Internal Category**:
A Category the system creates for bookings that are neither income nor spending, such as Opening balances, Household transfers and Uncategorised. Internal Categories cannot be budgeted or deleted.
_Avoid_: Equity, system account, special category

**Uncategorised**:
The Internal Category where an Imported transaction's other side waits until a User categorises it.
_Avoid_: Unknown, misc, suspense

## Transactions

**Transaction**:
One financial event, such as a purchase, a salary payment or a transfer, made of Postings that together sum to zero.
_Avoid_: Entry, journal entry, movement

**Booking date**:
The date the bank recorded a Transaction on the Account. It decides which month a Transaction belongs to for the Account's balance, Statement balance and Month close.
_Avoid_: Posting date, ledger date

**Transaction date**:
The date the purchase or payment actually happened, for example when a card was tapped. It decides which month a Transaction counts toward in Budgets and reports.
_Avoid_: Purchase date, event date

**Value date**:
The date the money actually moved, which banks use to calculate interest. It is recorded but does not decide any month.
_Avoid_: Settlement date, effective date

**Posting**:
One line of a Transaction: the amount it moves into or out of a single Account or Category. A payment that covers several Categories has one Posting per Category; a transfer between two Accounts has Postings on Accounts only.
_Avoid_: Split, leg, line

**Reversal**:
A Transaction with the opposite Postings of an earlier one, linked to it, which undoes it without editing or deleting history. Reversing an Accepted Transaction returns its Imported transaction to pending.
_Avoid_: Undo, delete, void, refund

**Household transfer**:
Money moving between two Households. Each Household records its own side against an Internal Category, and the two sides are linked by a Match.
_Avoid_: Contribution, inter-household payment

## Closing a month

**Month close**:
A User's confirmation that one Account's month has been reviewed. A month can close only when every Imported transaction is Matched, Accepted or Discarded, every relevant Balance check has passed, nothing is left in Uncategorised, and the computed balance equals the Statement balance. A gap is closed only by correcting the entry that is wrong, never by a balancing entry (ADR-0018). A closed month can still be edited, but any change is flagged.
_Avoid_: Lock, freeze, period end

**Statement balance**:
The balance an Account should have at the end of a month: the closing balance on the bank statement, or the amount a User counted for cash.
_Avoid_: Bank balance, real balance
