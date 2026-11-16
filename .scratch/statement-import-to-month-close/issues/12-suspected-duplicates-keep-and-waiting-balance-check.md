# 12: Suspected duplicates, Keep, and the waiting Balance check

**What to build:** when a later file has rows that only look like earlier ones, they are imported as Suspected duplicates rather than skipped, and each is shown next to the row it resembles. A User Keeps one (it becomes pending) or Discards it. The import's Balance check waits until every Suspected duplicate is decided, then passes or fails. Spec: `../spec.md` stories 39, 40, 40b–40e, 43c, "Ingestion: Imported transaction".

**Blocked by:** 10 (Recategorise, reverse Transactions and Discard pending Imported transactions), 11 (Later imports: Balance check and certain repeats)

**Status:** ready-for-agent

- [ ] Suspected duplicate: same bank transaction ID with a different Booking date or amount; or, without a bank transaction ID, same Account, Booking date, amount and normalised description as a row from an earlier import. Never compared within the same file
- [ ] Each Suspected duplicate records the earlier Imported transaction it resembles, and the list (`state=suspected_duplicate`) shows both
- [ ] `POST …/imported-transactions/{imported_transaction_id}/keep` moves a Suspected duplicate to pending; Discard (from ticket 10) also accepts Suspected duplicates
- [ ] An import with Suspected duplicates is recorded with its Balance check waiting; deciding the last one runs it and records `balancecheckpassed` or `balancecheckfailed`
- [ ] The import result reports the Suspected duplicate count
- [ ] Decider tests: Suspected duplicate transitions, waiting check resolving to passed and to failed
- [ ] HTTP tests: overlapping files without bank IDs; same bank ID with a different date or amount; identical rows within one file both imported; Keep; Discard; check waiting then passing; check waiting then failing
- [ ] Consistency map row for Keep or Discard Imported transaction matches the code
