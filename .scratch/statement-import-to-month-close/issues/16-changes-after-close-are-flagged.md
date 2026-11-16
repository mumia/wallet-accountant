# 16: Changes after close are flagged

**What to build:** a closed month can still be corrected, but any change that touches it (an Accept, a Category change, a reversal or a void) flags that month, and the month status shows it. New Imported transactions arriving in a closed month show as pending. Spec: `../spec.md` stories 62–63, "Ledger: Account".

**Blocked by:** 14 (Void Statement import), 15 (Month status and Month close)

**Status:** ready-for-agent

- [ ] Any Transaction booked into, recategorised within or reversed in a closed month sets that month's flag
- [ ] Voiding an import whose rows touch a closed month flags it (including the Opening balance reversal)
- [ ] New Imported transactions for a closed month are listed as pending and counted in the month status
- [ ] Decider tests: flagging on each kind of change
- [ ] HTTP tests: flag after Accept, recategorise, reverse and void in a closed month; a later import adding rows to a closed month shows them as pending
- [ ] Consistency map rows mention closed-month flags wherever the code updates them
