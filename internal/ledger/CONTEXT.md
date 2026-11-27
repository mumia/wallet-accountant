# Ledger

The Household's books: where money is held, what it was for, every Transaction that moved it, and whether each Account's month agrees with reality.

## Accounts

**Account**:
A place where money is held or owed, such as a bank account, a credit card, a loan, cash or a crypto wallet. It always belongs to exactly one Household.
_Avoid_: Wallet, pocket, source, asset account

**Start date**:
The day an Account's history in Wallet Accountant begins, and the date of its Opening balance. Its month is the Account's first Open month. Bank rows booked before it are ignored.
_Avoid_: Opening date, Opening balance date, account opening date

**Account currency**:
The single currency an Account's balance is kept in. Money arriving in another currency is converted into it.
_Avoid_: Working currency, wallet currency

**Opening balance**:
The balance an Account had on its start date, recorded against an Internal Category. A statement Account's is worked out from its first Statement import (the Export balance minus the file's rows) and is never typed; a cash Account's is its Opening count (ADR-0018).
_Avoid_: Initial balance, starting balance

**Cash Account**:
An Account kept entirely by hand, with no statement; its truth is a count. Every other kind of Account is a statement Account, which takes Transactions only from its statements.
_Avoid_: Manual account, petty cash

**Opening count**:
The cash a User counts on a cash Account's start date, typed twice, which becomes its Opening balance.
_Avoid_: Initial cash, starting cash

## Categories

**Category**:
What money was received for or spent on. A Category belongs to a Household, has one kind (Income, Expense or Internal), and can be nested to any depth, for example Food › Eating out › Coffee. An Income Category takes only money in and an Expense Category only money out, judged by the amount on the Account; Internal Categories and a zero amount take either (ADR-0004). A refund is money in, so it goes to an Income Category.
_Avoid_: Tag, type, bucket, expense account

**Internal Category**:
A Category the system creates for bookings that are neither income nor spending: Opening balance, Household transfer, Account transfer and Uncategorised. Internal Categories cannot be budgeted or deleted.
_Avoid_: Equity, system account, special category

**Account transfer**:
The Internal Category on the other side of every Transfer leg. It is reached only through a Transfer, never chosen directly.
_Avoid_: Transfers category, internal movement

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
One line of a Transaction: the amount it moves into or out of a single Account or Category. A payment that covers several Categories has one Posting per Category; each Transfer leg has a Posting on its Account and one on Account transfer.
_Avoid_: Split, leg, line

**Reversal**:
A Transaction with the opposite Postings of an earlier one, linked to it, which undoes it without editing or deleting history. Reversing an Accepted Transaction returns its Imported transaction to pending.
_Avoid_: Undo, delete, void, refund

**Transfer**:
Money moving between two of the Household's Accounts, recorded as two Transfer legs linked by a Match (ADR-0026).
_Avoid_: Movement, internal payment

**Transfer leg**:
One side of a Transfer: a Transaction on one Account, against Account transfer, naming the other Account as its Counterpart Account.
_Avoid_: Side, half, mirror transaction

**Counterpart Account**:
The other Account a Transfer leg names: where the money came from or went to.
_Avoid_: Target account, other side, destination

**In transit**:
The state of a Transfer leg that has no Match yet and isn't reversed, such as cash withdrawn on the 31st whose bank row arrives in the next statement. It never stops a Month close.
_Avoid_: Pending (Ingestion's word), unmatched, open

**Household transfer**:
Money moving between two Households. Each Household records its own side against an Internal Category, and the two sides are linked by a Match.
_Avoid_: Contribution, inter-household payment

## Closing a month

**Open month**:
The one Booking month of an Account that can be changed and closed. It starts as the start date's month and moves to the next month each time it closes (ADR-0024).
_Avoid_: Current month, active month, period

**Not open yet**:
The state of every Booking month after the Open month. Rows booked in it can be seen, but nothing is Accepted into it until it becomes the Open month.
_Avoid_: Future month, pending month, locked

**Month close**:
A User's confirmation, after the Open month has ended, that it has been reviewed and agrees with the bank. The month can close only when every Imported transaction booked in it is Matched, Accepted or Discarded, every relevant Balance check has passed, nothing is left in Uncategorised, and the computed balance equals the Statement balance. A gap is closed only by correcting the entry that is wrong, never by a balancing entry (ADR-0018). A closed month's amounts and descriptions can't change; only its Transactions' Categories can, and never to Uncategorised (ADR-0024).
_Avoid_: Lock, freeze, period end

**Reopen**:
Taking back the latest Month close of an Account, while the Open month has no Accepted Transactions (a cash Account has no such condition). The reopened month becomes the Open month again, the month after it goes back to Not open yet, and the month must be closed again with its Statement balance.
_Avoid_: Unlock, undo close, unclose

**Statement balance**:
The balance an Account should have at the end of a month: the closing balance on the bank statement, or, for a cash Account, the month-end balance its Cash count proves.
_Avoid_: Bank balance, real balance

**Cash count**:
The amount of cash a User counted and the day they counted it, after the month's last day, which closes a cash Account's month when the books up to that day agree with it.
_Avoid_: Cash balance, till count
