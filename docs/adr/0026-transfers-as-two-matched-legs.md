# Transfers as two Matched legs

Money moving between two of a Household's Accounts is a **Transfer**, recorded as two **Transfer legs**: one ordinary Transaction on each Account, each booked against the Account transfer Internal Category and naming the other Account as its counterpart. A **Match** links the two legs. Account transfer is Internal, so a Transfer is never spending or income (ADR-0004).

- **Each side keeps its own dates and its own month.** A bank leg carries its row's dates exactly (ADR-0018), and the cash leg is dated on the day the cash really moved. Each leg is booked by its own Account, in that Account's month, under that Account's month rules (ADR-0024).
- **In transit is a legitimate state.** A leg without a Match is In transit: money that left the bank on the 31st and reached the wallet on the 2nd really is in transit at month end. An In transit leg never blocks a Month close; the month reports how many it has.
- **The automatic cash leg.** Accepting a bank row as a Transfer to a cash Account records the cash leg too, and Matches the two in the same database transaction. If the member already recorded the cash leg by hand, the Accept Matches that leg instead, so the same withdrawal is never counted twice in cash. A Transfer between two cash Accounts records both legs and their Match in one command.
- **Reverse cascades only to legs that exist because of the one reversed.** Reversing a Matched leg removes its Match. The automatic cash leg is reversed with its bank leg, and both legs of a cash ↔ cash Transfer go together, because neither can stand alone. A cash leg entered by hand stays, In transit, and so does a bank leg whose cash leg is reversed alone. If any leg the command would reverse is in a closed month, the whole command is refused.
- **The Match is its own aggregate,** and each leg records whether it is Matched. Two Accepts racing to Match one leg are caught by that leg's version check.

## Considered Options

- **One Transaction with two Account Postings** (bank −50, wallet +50). Rejected: it has one date for both sides, so it can't say that the money left the bank on the 31st and reached the wallet on the 2nd; both Accounts' months must be open at once; and when both sides are Imported, as they will be for bank ↔ bank Transfers, it can't take two Imported transactions without booking the money twice.
- **Accepting the ATM withdrawal against a spending Category and recording cash spending too.** Rejected: every euro spent in cash would count as spending twice.

## Consequences

- An In transit leg left by mistake is fixed by reversing it and recording it again. In this slice there is no standalone "Match these two legs" command: a Match is confirmed only by an Accept or by a cash ↔ cash entry.
- Statement-to-statement Transfers (bank ↔ bank, card repayments, loans) can reuse the legs and the Match: both sides are Imported, Accepted as legs and Matched.
