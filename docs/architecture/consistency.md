# Consistency map

What each command changes immediately (in the same database transaction) and what follows later through domain events. The rule behind it is ADR-0013: immediate if a User would notice a delay or an inconsistency, deferred otherwise.

Every spec that adds or changes a command adds or updates its row here. Code review checks the code against this table.

| Command | Context | Immediate (same transaction) | Deferred (via events) |
|---|---|---|---|
| Issue bootstrap Invitation | Households | Bootstrap Invitation issued, or reissued with a new code if still unredeemed; its code lookup row updated | — |
| Redeem Invitation | Households | Invitation redeemed; User registered with the Invitation's role and its identity lookup row saved | — |
| Create Household | Households → Ledger | Household created; creator added as member, with the User's membership index row; the three Internal Categories (Opening balance, Household transfer, Uncategorised) created | — |
| Create Category | Ledger | Household's Category tree locked; parent and sibling names read and checked; Category created | — |
| Add Account | Ledger | Account created, awaiting its first Statement import | — |
| Save CSV mapping | Ingestion | Connection's mapping validated as a whole and saved with a new mapping version (also when a Connection is created with its mapping) | — |
| Upload Statement import | Ingestion → Ledger | Connection's mapping, the Account (through the Ledger port) and earlier imports of the same file read and checked; Statement import recorded with its Export balance; new Imported transactions recorded as pending or Suspected duplicate; certain repeats skipped; Balance check run, or left waiting for Suspected duplicates. First import only (its `preview_id` recomputed and checked): one pending Imported transaction per row on or after the start date, then through the Ledger port the Account's Opening balance recorded (refused if it already has one), so it stops awaiting its first Statement import and its balance is set, and the Opening balance Transaction recorded on the start date with its Account Posting lookup row (the Account transaction list). Later imports only (no `preview_id`, Export balance given once): the Account's Statement imports locked; its Opening balance and every non-Discarded Imported transaction (pending, Accepted, kept) read from Ingestion's own folds, never Ledger Transactions; rows matching an earlier one by bank transaction ID, Booking date and amount skipped and counted; Balance check run on the rest, and a failed check (with no Suspected duplicates) refuses the whole import, recording nothing; otherwise the Statement import recorded with its check passed and its new rows pending | Notification to the Household when Suspected duplicates were found |
| Keep or Discard Imported transaction | Ingestion | Imported transaction moved to pending (keep) or Discarded; a waiting Balance check run once its last Suspected duplicate is decided | — |
| Preview first Statement import | Ingestion ← Ledger | Nothing recorded: the Connection's mapping, the Account (through the Ledger port) and earlier imports of the same file are only read | — |
| Void Statement import | Ingestion → Ledger | Statement import voided; its Imported transactions Discarded; file hash released. First import only: Opening balance Transaction reversed, Account back to awaiting its first Statement import | — |
| Accept imported transaction | Ingestion → Ledger | Imported transaction marked Accepted; Ledger Transaction recorded; touched Accounts' balances, Uncategorised counts and closed-month flags updated; Account transaction list updated | Category totals, Budget actuals, Forecasts, Match suggestions |
| Set Transaction Category | Ledger | Transaction's Category changed; Account's Uncategorised count and closed-month flag updated; Account transaction list updated | Category totals, Budget actuals |
| Reverse Transaction | Ledger → Ingestion | Reversing Transaction recorded and linked; touched Accounts' balances, Uncategorised counts and closed-month flags updated; Account transaction list updated; linked Imported transaction back to pending | Category totals, Budget actuals |
| Close month | Ledger ← Ingestion | Unresolved Imported transactions and Balance checks checked through the Ingestion port; Account month marked closed with its Statement balance | — |
| Rename or move Category | Ledger | Household's Category tree locked; new parent and sibling names read and checked; Category renamed and/or moved | — |
