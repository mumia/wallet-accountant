# 11: Later imports: Balance check and certain repeats

**What to build:** a User uploads the next statement file for an Account with its Export balance, typed once. Rows already imported (same bank transaction ID, Booking date and amount) are skipped. The import is refused if the Account's Imported transactions, with the new rows, don't add up to the Export balance, showing the expected balance, the Export balance and the difference. Spec: `../spec.md` stories 38, 40, 43a, 43b, "Ingestion: Statement import: Later imports"; ADR-0018.

**Blocked by:** 08 (First Statement import: preview, confirm and Opening balance)

**Status:** ready-for-agent

- [ ] Later uploads refuse `preview_id`
- [ ] Balance check: Opening balance + every non-Discarded Imported transaction of the Account (pending, Accepted, kept, and this file's new rows) = Export balance. Ledger Transactions aren't used
- [ ] A failed check refuses the whole import with expected balance, Export balance and difference; a passing import is recorded with the check passed
- [ ] A row whose bank transaction ID, Booking date and amount all equal an earlier Imported transaction on the Account is skipped
- [ ] The import result reports rows imported, skipped as certain repeats, and ignored before the start date
- [ ] Decider tests: Balance check pass and fail, certain-repeat skipping
- [ ] HTTP tests: overlapping files with bank IDs skip repeats; Balance check pass and refusal; a wrong mapping caught by the check; the check still counts Accepted rows once Accept exists
- [ ] Consistency map row for Upload Statement import (later imports) matches the code
