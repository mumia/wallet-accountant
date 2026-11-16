# 16: Changes after close are flagged

**What to build:** a closed month can still be corrected, but any change that touches it (an Accept, a Category change, a reversal or a void) flags that month, and the month status shows it. New Imported transactions arriving in a closed month show as pending. Spec: `../spec.md` stories 62–63, "Ledger: Account".

**Blocked by:** 14 (Void Statement import), 15 (Month status and Month close)

**Status:** claimed

- [ ] Any Transaction booked into, recategorised within or reversed in a closed month sets that month's flag
- [ ] Voiding an import whose rows touch a closed month flags it (including the Opening balance reversal)
- [ ] New Imported transactions for a closed month are listed as pending and counted in the month status
- [ ] Decider tests: flagging on each kind of change
- [ ] HTTP tests: flag after Accept, recategorise, reverse and void in a closed month; a later import adding rows to a closed month shows them as pending
- [ ] Consistency map rows mention closed-month flags wherever the code updates them

## Comments

Implementation notes (implementer agent):

- **Ledger.** The Account fold flags a closed month in `evolveAccount` on `TransactionBooked` (Accept), `ReversalBooked`, `TransactionRecategorised` (always recorded by ticket 10) in their Booking month, and `OpeningBalanceReversed` in the start month. The month close (Statement balance, closed at/by) is kept; the flag is never cleared. Fold schema version 5 (existing folds rebuild on load). No HTTP or migration change: `account_months.flagged` and `AccountMonth.flagged` came with ticket 15.
- **Void of a later import.** Discarding pending rows books nothing on the Account, so it needed its own event: Account Decider `FlagMonth{Month}` → `io.walletaccountant.ledger.account.monthflagged.v1` (`MonthFlagged{Month}`), only for a closed, not yet flagged month (otherwise records nothing). `ledgerapp.Months.FlagClosedMonths`, reached through a new Ingestion `Ledger` port method `FlagClosedMonths` (`internal/wiring/ledgerport.go`). `StatementImports.Void` passes the Booking dates of the rows the void actually Discarded (rows already Discarded don't count). Mutation: dropping that call fails `TestVoidingAnImportWithRowsInAClosedMonthFlagsIt` and the rebuild test.
- **New rows in a closed month** were already counted as pending by ticket 15's `MonthImports` (no closed-month filter); an upload alone doesn't flag the month, the Accept (or void) of those rows does.
- **Not flagged:** a plain Discard or Keep of an Imported transaction in a closed month (not in story 62's list, and nothing is booked). If that should flag too, `FlagClosedMonths` is the hook.
- **Tests.** Decider `internal/ledger/domain/flag_test.go`: each change (booking, recategorisation, reversal, Opening balance reversal) flags only its closed month and keeps the close; a change in an open month flags nothing; `FlagMonth` on closed / already flagged / open / unknown Account. HTTP `internal/ingestion/adapters/http/flag_integration_test.go`: Accept (of a later import's January row), recategorise (to a Category and to Uncategorised), reverse (row back to pending in the closed month), a change in February leaving January unflagged, a later import's rows pending in a closed month (unflagged), void of that import flags, void of the first import flags the closed start month, rebuild.
- **Consistency map.** Upload, Void, Accept, Set Transaction Category, Reverse Transaction and Close month rows updated.
