# Spec: Statement import → Accept → Month close

Status: ready-for-agent

The first vertical slice. A User signs in, sets up a Household with an Account and its Categories, imports a bank statement file through a Connection with a configurable CSV mapping, Accepts each Imported transaction into the Ledger, and closes the Account's month. It also builds the walking skeleton every later slice relies on: the binary and its run modes, migrations, the event store and Decider runtime, Watermill forwarding, Household isolation, the OpenAPI contract with ogen, authentication, Compose, Make targets and lint.

## Problem Statement

A Household that wants to know where its money goes has no trustworthy book of record. Bank statements arrive as CSV files, each bank with its own layout. Typing every line into a spreadsheet by hand is tedious, error-prone and never quite matches what the bank says. Nobody can tell whether a month is "done": whether every bank line has been accounted for, whether every payment has been explained, and whether the balance agrees with the bank.

## Solution

A User creates a Household with a Base currency, adds an Account with its start date, and sets up a Connection for that Account. The Connection describes how that bank's CSV file is laid out. The User uploads a statement file through the Connection together with its Export balance, the bank's booked balance when the file was exported. Every row becomes an Imported transaction waiting for review. The first import works out the Account's Opening balance from the Export balance, so nobody types the Opening balance. Because nothing can check that first Export balance, the User types it twice and previews the resulting Opening balance before confirming the import. Every later import is checked against its own Export balance, so a wrong mapping or a missing row is caught at upload. Duplicate files and duplicate rows are recognised and skipped. The User Accepts each Imported transaction and confirms its Category, or leaves it in Uncategorised to sort out later. Each Accept immediately records a balanced Ledger Transaction and updates the Account's balance. The User sees the Account's transactions as a familiar single-entry list with signed amounts. When the month is reviewed, the User enters the Statement balance and closes the month. If anything is still pending, still in Uncategorised, or the balance does not agree, the close is refused with every reason listed. There is no balancing entry: a gap is closed only by fixing the entry that is wrong, by reversing a mistaken Accept, voiding and re-uploading a wrong import, or uploading a fuller file (ADR-0018). A closed month can still be changed, but any change is flagged.

## User Stories

### Signing in and the Household

1. As a User, I want to sign in through the Household's OpenID Connect provider, so that Wallet Accountant never stores my password.
2. As the first person to use a new instance, I want to redeem the Invitation code the operator gave me after signing in, so that I become a User and the Instance administrator.
2a. As someone who has signed in but not redeemed an Invitation, I want to be told I am not registered, with a clear error, so that I know I need an Invitation.
2b. As someone who has signed in but not redeemed an Invitation, I want every Household route refused, so that being authenticated alone never gives access.
2c. As someone redeeming an Invitation, I want an unknown, expired, superseded or already used code refused with one generic error, so that codes can't be probed.
2d. As a User, I want redeeming a second Invitation to be refused, so that one identity never becomes two Users.
3. As a User, I want to create a Household with a name and a Base currency, so that I have a place to keep our books.
4. As a User, I want to become a member of the Household I create, so that I can work in it immediately.
5. As a User, I want to list the Households I belong to, so that I can choose which one to work in.
6. As a User, I want a new Household to come with its Internal Categories (Opening balance, Household transfer, Uncategorised) already in place, so that imports and closes work without setup.
7. As a User, I want to be refused, with a clear error, when I try to reach a Household I don't belong to, so that our finances stay private.
8. As a User, I want another Household's Accounts, Connections, Imported transactions and Transactions to be invisible to me even if I guess their IDs, so that privacy doesn't depend on IDs staying secret.

### Categories

9. As a User, I want to create Income and Expense Categories in my Household, so that I can say what money was for.
10. As a User, I want to nest a Category under another one, for example Food › Eating out › Coffee, so that I can group spending at the detail I care about.
11. As a User, I want a sub-Category to keep its parent's kind, so that an Expense tree never contains an Income Category.
12. As a User, I want to list my Household's Categories as a tree, including the Internal Categories, so that I can pick one when Accepting.
13. As a User, I want to be prevented from creating Internal Categories myself, so that the system's bookkeeping Categories stay under its control.
14. As a User, I want Category names to be unique among siblings, so that the tree never shows two identical choices side by side.

### Accounts

15. As a User, I want to add an Account with a name, a kind (bank account, credit card, loan, cash or crypto wallet) and an Account currency, so that it mirrors a real place where money is held or owed.
16. As a User, I want to give an Account a start date, so that its history begins where my books begin.
17. As a User, I want the Account's Opening balance worked out from its first Statement import (the Export balance minus the sum of the file's rows on or after the start date) and recorded as a Transaction against the Opening balance Internal Category, so that nobody types it and it agrees with the bank from day one.
17a. As a User, I want an Account to show that it is awaiting its first Statement import, with no balance, until that import arrives, so that I never see a made-up balance.
17b. As a User, I want no way to enter or edit an Opening balance by hand, so that it is always the bank's number.
18. As a User, I want to list my Household's Accounts with their current balances, so that I can see where our money is.
19. As a User, I want to see an Account's current balance immediately after any change, so that I never act on a stale number.
20. As a User, I want an Account's amounts in its Account currency with that currency's exact number of decimal places, so that balances are never rounded wrongly.
21. As a User, I want to list an Account's Transactions as a single-entry list with signed amounts, Booking date, Transaction date, description and Category, so that I don't need to understand Postings.
22. As a User, I want to filter an Account's Transactions by Booking month, so that I can review one month at a time.
23. As a User, I want the Transaction list paged with a cursor, so that long histories load quickly and consistently.

### Connections and CSV mapping

24. As a User, I want to create a Connection of the statement file import kind for an Account, so that I can bring in that Account's bank statements.
25. As a User, I want to describe how the bank's CSV is laid out in the Connection (delimiter, whether there is a header row, how many leading lines to skip, date format, decimal separator, and thousands separator), so that each bank's file is read correctly.
26. As a User, I want to choose which column holds each field (Booking date, description, amount) by header name or by position, so that I can map files with or without headers.
27. As a User, I want to choose between one signed amount column and separate debit and credit columns, so that both common bank layouts work.
28. As a User, I want the Transaction date, Value date, payee and bank transaction ID columns to be optional, so that sparse bank files can still be imported.
29. As a User, I want the Transaction date to default to the Booking date when the file has none, so that Budgets and reports still have a date to use.
30. As a User, I want to choose whether the amount sign in the file means money in or money out, so that banks that invert the sign are read correctly.
31. As a User, I want an invalid mapping (a missing required field, an unknown date format, or the same column used twice) to be rejected when I save the Connection, so that I find mistakes before importing.
32. As a User, I want to change a Connection's mapping, so that I can fix it when the bank changes its export.
33. As a User, I want to list an Account's Connections, so that I can see how its statements arrive.

### Statement import

34. As a User, I want to upload a statement file through a Connection, so that its rows become Imported transactions without typing.
35. As a User, I want each row to become an Imported transaction in the Account currency with its Booking date, Transaction date, Value date, amount, description, payee and bank transaction ID as mapped, so that nothing the bank told me is lost.
36. As a User, I want the whole file rejected with a list of row numbers and reasons when any row can't be read, so that I never end up with half a statement imported.
37. As a User, I want uploading the same file twice to be refused with a pointer to the earlier Statement import, so that nothing is imported twice.
38. As a User, I want a row whose bank transaction ID, Booking date and amount all equal those of a row already imported for the Account to be skipped automatically, so that overlapping statement periods don't create duplicates.
39. As a User, I want a row that only looks like an earlier one (same Booking date, amount and description but no bank transaction ID, or the same bank transaction ID with a different date or amount) to be imported as a Suspected duplicate, not skipped, so that the system never silently drops a real payment.
40. As a User, I want the result of a Statement import to tell me how many rows were imported, how many were skipped as certain repeats, and how many are Suspected duplicates, so that I can trust what happened.
40a. As a Household member, I want to be notified when a Statement import produces Suspected duplicates, so that someone in the Household reviews them.
40b. As a Household member, I want to see each Suspected duplicate next to the earlier Imported transaction it resembles, so that I can decide quickly.
40c. As a Household member, I want to keep a Suspected duplicate as a real transaction, so that two genuinely identical payments are both booked.
40d. As a Household member, I want to Discard a Suspected duplicate, so that a repeated row never reaches the Ledger.
40e. As a Household member, I want identical rows within a single file never to be treated as duplicates of each other, so that two equal coffees on the same day both count.
40f. As a Household member, I want to list my Household's notifications and mark them as read, so that I can tell what still needs attention.
41. As a User, I want rows with a Booking date before the Account's start date to be ignored and counted in the import result, so that history before the Account existed doesn't distort its balance and I don't need to trim the export.
42. As a User, I want to list the Statement imports of a Connection, so that I can see which periods I have already brought in.
43. As a User, I want retrying an upload after a network error, with the same Idempotency-Key, to return the original result, so that retries are safe.
43a. As a User, I want to give every Statement import its Export balance (the bank's booked balance, excluding card authorisations not yet booked, when I exported the file), so that the file can be checked against the bank.
43b. As a User, I want a later Statement import refused when the Account's Imported transactions, including the new rows, don't add up to its Export balance, with the expected balance, the Export balance and the difference shown, so that a wrong mapping or a missing row is caught at upload.
43c. As a User, I want the balance check of an import that produced Suspected duplicates to wait until each one is decided, and then run, so that a real repeat payment and a repeated row are both handled correctly.
43d. As a User, I want to void the latest Statement import of an Account when none of its rows are Accepted, so that I can fix the mapping and upload the same file again.
43e. As a User, I want voiding the first Statement import to also remove the Opening balance, so that a corrected first file works it out again.
43f. As a User, I want voiding any import other than the latest refused, so that every remaining import's balance check stays valid.
43g. As a User, I want the first Statement import of an Account to ask for the Export balance twice, and to be refused if the two entries don't match, so that a typo can't become the Opening balance. Later imports ask once, because a typo there fails the Balance check.
43h. As a User, I want to preview the first Statement import before it is recorded, seeing the Opening balance it would produce on the start date, the row count, the rows ignored before the start date and the rows' total, and then confirm it, so that I check the Opening balance before it enters the books.

### Reviewing and Accepting

44. As a User, I want to list an Account's Imported transactions that are waiting for review, filtered by Booking month, so that I can work through a month.
45. As a User, I want to Accept an Imported transaction with a Category I choose, so that it becomes a Transaction in the Ledger with its purpose explained.
46. As a User, I want to Accept an Imported transaction without choosing a Category, so that it is booked now and its other side waits in Uncategorised.
47. As a User, I want the Account balance to reflect an Accepted transaction immediately, so that I can watch the balance converge on the bank's.
48. As a User, I want Accepting to be refused for an Imported transaction that is already Accepted, so that money is never booked twice.
49. As a User, I want Accepting with an Internal Category to be refused (except Uncategorised, which is the same as giving no Category), so that bookkeeping Categories aren't misused.
50. As a User, I want to give an Uncategorised Transaction its Category later, so that I can finish categorising after booking.
51. As a User, I want to change the Category of a Transaction I Accepted, so that I can correct mistakes.
51a. As a User, I want an Accepted Transaction's amount always to equal its Imported transaction's amount, and no Transaction's amount ever to be editable, so that the Ledger matches the bank exactly.
51b. As a User, I want to reverse an Accepted Transaction, so that a mistaken Accept is undone without editing or deleting history.
51c. As a User, I want a reversal recorded as its own Transaction with opposite Postings, linked to the original, with both shown in the Account's list, so that the correction is visible.
51d. As a User, I want reversing to return the Imported transaction to pending, so that I can Accept it again or Discard it.
51e. As a User, I want to Discard a pending Imported transaction, so that a row that should never have been booked never reaches the Ledger.
51f. As a User, I want reversing the Opening balance Transaction, a reversal, or an already reversed Transaction refused, so that corrections can't pile up or bypass voiding.
52. As a User, I want retrying an Accept with the same Idempotency-Key to be safe, so that a double click or a retry never books twice.

### Month close

53. As a User, I want to close one Account's month by entering its Statement balance, so that I confirm the month has been reviewed and agrees with the bank.
54. As a User, I want months to be decided by Booking date, so that the Account's month lines up with the bank statement.
55. As a User, I want the close refused while any Imported transaction booked in that month is still waiting for review or is an undecided Suspected duplicate, so that no bank line is left unaccounted for.
56. As a User, I want the close refused while any Transaction on the Account in that month is in Uncategorised, so that every payment is explained.
57. As a User, I want the close refused when the computed closing balance differs from the Statement balance, with both amounts and the difference shown, so that I know how far off I am.
58. As a User, I want a refused close to list every reason at once, so that I can fix them all in one go.
58a. As a User, I want the close refused while a Statement import with rows in or before that month is still waiting for its balance check, or failed it, so that a month never closes on rows the bank doesn't agree with.
59. As a User, I want no way to record a balancing entry, so that a gap is never hidden.
60. As a User, I want to close a gap by reversing a wrong Accept, voiding and re-uploading a wrong import, or uploading a fuller file, so that every correction leaves the Ledger matching the bank.
61. As a User, I want to see each month's status for an Account (open or closed, Statement balance, computed balance, pending Imported transactions, undecided Suspected duplicates, Uncategorised count, flagged), so that I know what is left to do.
62. As a User, I want any change that touches a closed month (an Accept, a Category change, a reversal or a void) to be allowed but flag that month, so that I can correct the past without silently breaking a confirmed month.
63. As a User, I want a closed month that later receives new Imported transactions to show them as pending, so that I notice the statement changed.
64. As a User, I want closing an already-closed month to be refused, so that the confirmed Statement balance isn't overwritten by accident.

### Operating it

65. As a self-hoster, I want `make up` to start the app, migrations, PostgreSQL, Zitadel and the collector, so that I can run Wallet Accountant with one command.
66. As a self-hoster, I want migrations to run in their own `migrate` run mode before the app starts, so that the schema is always ready.
66a. As a self-hoster, I want `make bootstrap` to print a one-time Invitation code for the first User, so that only someone with access to the deployment can claim a new instance.
66b. As a self-hoster, I want `bootstrap` refused once an Instance administrator exists, and a rerun before redemption to replace the earlier code, so that at most one bootstrap code is ever valid.
66c. As a self-hoster, I want the Invitation code never written to logs or telemetry, so that it doesn't leak into Loki.
67. As a developer, I want `make dev-up` and `make run` to run the dependencies in Docker and the app on my machine, so that I can iterate quickly.
68. As a developer, I want `make lint` to run golangci-lint with depguard and vacuum with the JavaScript-free Zalando ruleset, so that architecture and API guidelines are enforced.
69. As a developer, I want `make test` and `make test-integration` to run the Decider tests and the HTTP tests against real PostgreSQL, so that I can trust a change.
70. As an operator, I want every request and every event handler to produce logs, traces and metrics, so that I can follow a request into its deferred effects.
71. As an operator, I want `make rebuild` to rebuild every fold from the event store, so that I can recover after a fold schema change.
72. As an API consumer, I want every error returned as `application/problem+json` with a meaningful status, so that the frontend can handle failures uniformly.

## Implementation Decisions

### Walking skeleton (built in this slice, reused by all later slices)

- **Binary and run modes** follow ADR-0009: `api`, `worker`, `all`, `migrate`, `rebuild`, `bootstrap`. Configuration comes only from environment variables.
- **Migrations:** golang-migrate, forward-only, run by `migrate` as `wa_owner`. The first migration creates the three roles from ADR-0014, the event store table from ADR-0011, Watermill's offsets table, the processed-events table, and every fold table this slice needs. Every Household-scoped table has `household_id`, `FORCE ROW LEVEL SECURITY` and the `NULLIF(current_setting('app.household_id', true), '')::uuid` policy. The event store's policy also admits rows with a NULL Household, but only in instance scope. The Users and Invitations fold and lookup tables have a policy that admits them only in instance scope (ADR-0017).
- **Platform module.** It holds the event store repository, the Decider runtime (load fold and version, decide, stamp, encode/decode round-trip, evolve, append, save if the version is unchanged), the CloudEvents codec (ADR-0015), the custom Watermill SchemaAdapter, and handler middleware. The middleware does SAVEPOINT per attempt, processed-events dedup, `SET LOCAL ROLE wa_app` plus `set_config` from `householdid` (left empty for instance-level events, which have none), and `RESET ROLE` before ack. The runtime retries on a version conflict. The spike validated this design (branch `prototype/spike-eventstore-ogen`). Its decision-rich shape:

  ```go
  // From the prototype discussion: the Decider contract every aggregate implements.
  type Decider[S, C, E any] struct {
      Initial S
      Decide  func(S, C) ([]E, error)
      Evolve  func(S, Envelope[E]) S
  }
  ```

- **Request transaction.** Each API request that touches a Household opens one database transaction as `wa_app`. It first resolves the caller's User in instance scope, then sets `app.household_id` before any Household-scoped query. Every repository query also filters by `household_id` (ADR-0014).
- **Authentication.** An HTTP adapter verifies OIDC bearer JWTs against the provider's JWKS (ADR-0010). It sits behind an `Authenticator` port so tests can replace it. The adapter returns the verified identity (issuer and subject). The application resolves it to a User or to unregistered (ADR-0017).
- **Registration (bootstrap only).** `bootstrap` issues the bootstrap Invitation, a fixed-ID stream that grants Instance administrator. Its code is 256 random bits, stored as a SHA-256 hash, valid for 24 hours, and printed to stdout with `fmt`, never logged. A rerun before redemption reissues it with a new code. Once it is redeemed, `bootstrap` is refused. Redeeming runs the Invitation and User Deciders in one database transaction, so two concurrent redemptions can't both succeed. An unregistered caller gets 403 problem+json with a `registration-required` type on `GET /me` and `/households`, and 404 on every Household-scoped route. Invitations issued from inside the app are out of scope.
- **Authorisation.** Every Household-scoped route checks that the User is a member of the Household in the path. Otherwise it answers 404, not 403: a 403 would confirm to a non-member that the Household or resource exists. IDs are UUIDv7, whose leading bits are a timestamp, so they are partly guessable.
- **API.** `api/openapi.yaml` is contract-first (ADR-0008). Server code is generated with ogen (ADR-0012), and the spec passes vacuum with the JavaScript-free Zalando ruleset kept in the repo (moved from the spike).
  - Amounts are JSON decimal numbers with `format: decimal`, paired with a currency code.
  - `NewError` always sets a status, and error details are cleaned of internal decode paths before they are returned.
  - POST endpoints accept `Idempotency-Key`. A replay with the same key and body returns the stored response; the same key with a different body gets 422.
- **Observability.** slog sends JSON to stdout and to the OTLP bridge, and OpenTelemetry traces and metrics go to the collector (ADR-0016). Events carry `traceparent`, so handlers continue the request's trace.
- **Compose and Make.** `compose.dev.yaml` and `compose.yaml`, and every Make target listed in `docs/architecture/conventions.md`.
- **depguard rules** encode the layout in conventions.md: `domain` imports only the standard library and sharedkernel, and contexts talk to each other only through `app` ports.

### Resources (Zalando style: plural kebab-case paths, snake_case properties, cursor pagination)

All Household-scoped resources are nested under the Household, so that the request transaction can set `app.household_id` from the path before any lookup.

- `GET /me` (the User, and whether they are Instance administrator); `POST /registrations` (body: the Invitation code; 201 with the User, 422 for any invalid code, 409 if already registered)
- `POST/GET /households`; `GET /households/{household_id}`
- `POST/GET /households/{household_id}/categories`; `PATCH …/categories/{category_id}` (rename or move)
- `POST/GET /households/{household_id}/accounts` (POST body: name, kind, Account currency, start date); `GET …/accounts/{account_id}` (includes the current balance, or that it is awaiting its first Statement import)
- `GET …/accounts/{account_id}/transactions?booking_month=YYYY-MM` (single-entry view, ADR-0001)
- `PATCH …/transactions/{transaction_id}` (set the Category of the non-Account side); `POST …/transactions/{transaction_id}/reverse`
- `GET …/accounts/{account_id}/months/{booking_month}` (month status); `POST …/accounts/{account_id}/months/{booking_month}/close` (body: Statement balance)
- `POST/GET …/accounts/{account_id}/connections`; `PUT …/connections/{connection_id}/csv-mapping`
- `POST …/connections/{connection_id}/statement-imports/previews` (multipart: the file, `export_balance` and `export_balance_confirmation`; first import only; returns `preview_id`, the Opening balance, the row count, the rows ignored before the start date and the rows' total; records nothing)
- `POST/GET …/connections/{connection_id}/statement-imports` (multipart upload: the file, its Export balance, and `preview_id`, which is required on the first import and refused on later ones; the result includes the balance check); `POST …/statement-imports/{statement_import_id}/void`
- `GET …/accounts/{account_id}/imported-transactions?state=pending|suspected_duplicate&booking_month=YYYY-MM`; `POST …/imported-transactions/{imported_transaction_id}/accept` (optional `category_id`)
- `POST …/imported-transactions/{imported_transaction_id}/keep` (Suspected duplicates only; moves it to pending) and `…/discard` (Suspected duplicates and pending)
- `GET /households/{household_id}/notifications`; `POST …/notifications/{notification_id}/read`

### Aggregates (Deciders) and their rules

- **Households: Household.** Created with a name and a Base currency. Membership is part of the Household fold.
- **Households: Invitation** (instance scope, ADR-0017). Events: `issued`, `reissued`, `redeemed`. Issuing is refused if the Invitation was already issued and redeemed. Reissuing replaces the code hash and expiry. Redeeming is refused for a wrong code, a code past its expiry (judged by the command's timestamp), or an Invitation already redeemed. The fold stores one lookup row from code hash to Invitation.
- **Households: User** (instance scope, ADR-0017). Event: `registered`, with OIDC issuer and subject, the redeemed Invitation and the granted role. The fold holds the Instance administrator flag and one lookup row from issuer and subject to User, whose unique constraint stops one identity from registering twice.
- **Ledger: Category.** Kind is Income, Expense or Internal. A child inherits its parent's kind. Names are unique among siblings. Users can't create Internal Categories. Creating a Household also seeds its three Internal Categories in the same database transaction, because every later command depends on them.
- **Ledger: Transaction.** Postings must sum to zero per currency, and every Posting names exactly one Account or one Category. In this slice every Transaction has one Account Posting and one Category Posting. It carries Booking date, Transaction date, Value date, description, payee and an optional link to its Imported transaction. No command changes an amount: a Transaction recorded by Accept carries its Imported transaction's amount exactly. Changing its Category emits an event. Reversing it emits `reversed` and records a new reversing Transaction with opposite Postings, the same Booking date and a link to the original. Reversing is refused for the Opening balance Transaction, a reversal, or a Transaction already reversed. A reversed Transaction and its reversal can't be recategorised, and neither counts toward the Uncategorised count. Its fold stores one lookup row per Account Posting (Account, Booking date, Transaction date, signed amount, Category), saved in the same database transaction as the fold (ADR-0011 "Reads").
- **Ledger: Account.** Fields: kind, Account currency, start date, whether it is awaiting its first Statement import, and a running balance per Booking month. The fold also tracks, per Booking month, the Uncategorised Posting count, the Month close state (Statement balance, closed at, closed by) and a flag for changes after close. Close is decided here:
  - The Uncategorised count must be zero.
  - The computed closing balance (Opening balance plus every month up to this one) must equal the Statement balance exactly.
  - The Ingestion port must report zero unresolved Imported transactions (pending or Suspected duplicate) for the Account and month.
  - The Ingestion port must report no Statement import with rows in or before the month whose balance check is waiting or failed.
  - The month must not already be closed.

  On rejection, every failed condition is returned together. Any Transaction booked into, or recategorised within, a closed month sets that month's flag. Reopening isn't possible in this slice.
- **Ingestion: Connection.** Holds the kind (only statement file import in this slice), its Account and its CSV mapping. Saving a mapping validates it as a whole.
- **Ingestion: Statement import.** Keyed by the file's hash within the Connection, and a duplicate hash of a non-voided import is refused. Rows are parsed in a pure function, mapping plus bytes to rows or row errors, and the whole file is all-or-nothing. Rows booked before the Account's start date are ignored and counted. Every import carries its Export balance.
  - **First import of the Account:** there are no earlier rows to compare with, so no Suspected duplicates. The Opening balance is the Export balance minus the sum of the file's rows, and the balance check passes by construction. Because nothing checks this Export balance, the first import takes two steps (ADR-0018):
    - **Preview** parses the file with the Connection's mapping and refuses the request if `export_balance` and `export_balance_confirmation` differ. It returns the Opening balance it would produce on the start date, the row count, the rows ignored before the start date, the rows' total, and a `preview_id`. It records nothing. The `preview_id` is a hash of the Account ID, the file hash, the Export balance and the Connection's mapping version, so the server needs no stored state to check it later.
    - **Confirm** is the ordinary upload with `preview_id`. The server recomputes the ID from the uploaded file, its Export balance and the current mapping version, and refuses the upload if it differs. That covers a different file, a different Export balance, and a mapping changed since the preview. Confirm is also refused if the Account got its Opening balance in the meantime. Otherwise it records the Statement import and the Opening balance Transaction in one database transaction.
  - **Later imports:** the balance check is: Opening balance + every non-Discarded Imported transaction of the Account (pending, Accepted, kept, and this file's new rows) = Export balance. Ledger Transactions aren't used, because they lag while rows wait for review. If the file produced no Suspected duplicates, a failed check refuses the whole import. Otherwise the import is recorded with its check waiting. The check runs when the last of its Suspected duplicates is decided, and is recorded as passed or failed (`balancecheckpassed` / `balancecheckfailed`).
  - **Void** (`voided`): allowed only for the Account's latest non-voided import, and only while none of its rows are Accepted. All its Imported transactions become Discarded and its file hash is released. Voiding the Account's only import also reverses the Opening balance Transaction, and the Account goes back to awaiting its first Statement import.
- **Ingestion: Imported transaction.** One aggregate per row that isn't skipped. States: pending → Accepted → pending (when its Transaction is reversed); pending → Discarded; Suspected duplicate → pending (kept) or Discarded; any non-Accepted state → Discarded when its Statement import is voided. Discarded is final.
  - **Skipped automatically (obvious):** the same file again (by hash), or a row whose bank transaction ID, Booking date and amount all equal those of an earlier Imported transaction on the Account.
  - **Suspected duplicate (not obvious):** a row with the same bank transaction ID but a different Booking date or amount; or, without a bank transaction ID, a row whose Account, Booking date, amount and normalised description equal those of an Imported transaction from an earlier Statement import. Comparison is only against earlier imports, never within the same file. The Suspected duplicate records which earlier Imported transaction it resembles.
  - The fold is queryable by Account, Booking month and state, which is how it answers the Ledger's Month close port (pending and Suspected duplicate both count as unresolved; Discarded does not).
- **Households: Notification.** A Household-level message every member sees. It has a kind (only "suspected duplicates found" in this slice), a link to the Statement import, and read state per User. It is created by a deferred handler on the Statement import's event, which makes it the slice's proof of the end-to-end event path. Only in-app delivery exists in this slice.
- **Opening balance.** Adding an Account records no Opening balance. The Account's first Statement import works it out and records the Opening balance Transaction, dated the start date, against the Opening balance Internal Category, in the same database transaction. The only way to change it is to void that import and upload again (ADR-0018).
- **Currency.** Every Imported transaction is in the Account currency. Foreign-currency rows, Bank rate and Reference rate are not handled in this slice.

### Cross-context calls

- **Accept (Ingestion → Ledger, immediate).** The Ingestion application service calls a Ledger `app` port in the same database transaction. It runs the Imported transaction, the Transaction and the Account Deciders, each with its own version check, and retries on a conflict (ADR-0011, ADR-0013).
- **First Statement import (Ingestion → Ledger, immediate).** Recording the first import calls a Ledger `app` port that records the Opening balance Transaction, so the Account stops awaiting its first Statement import, in the same database transaction. Voiding that import calls it again to reverse the Opening balance Transaction.
- **Reverse Transaction (Ledger → Ingestion, immediate).** Reversing a Transaction linked to an Imported transaction returns that Imported transaction to pending through an Ingestion `app` port, in the same database transaction.
- **Month close (Ledger → Ingestion, immediate).** Ledger reads pending counts and balance check states through an Ingestion `app` port.
- **Account transaction list (immediate).** The single-entry list reads the Transaction folds' lookup rows, filtered by Account and Booking month, inside the request transaction (ADR-0011 "Reads"). It is always current and works with the worker down. Balances and month status come from the Account fold and are always current too.

### Consistency map rows added by this slice

Added to `docs/architecture/consistency.md`: Create Household, Create Category, Add Account, Save CSV mapping, Upload Statement import, Keep or Discard Imported transaction, Preview first Statement import, Void Statement import, Accept imported transaction (updated), Set Transaction Category, Reverse Transaction, Close month.

## Testing Decisions

- **What a good test is.** It drives the system only through its public seam and asserts only on what that seam returns: HTTP status codes, problem+json bodies, and the state read back through GET endpoints. Tests never read tables, folds or events directly, never assert on internal calls, and survive any refactor that keeps behaviour the same.
- **Seam 1: the HTTP API (most tests).** The real application runs in-process, ogen server included, against real PostgreSQL started through testcontainers-go. Migrations run as `wa_owner` and the app connects as `wa_app` and `wa_event_store_reader`, as in production. Only two things are swapped:
  - The `Authenticator` port gets a test implementation that maps a test token to an OIDC identity. Tests register identities by redeeming Invitations, either the real bootstrap one or ones seeded through a test helper, and can then act as two Users in two Households. HTTP tests also cover unregistered access, every invalid-code case, double redemption and concurrent redemption of one code. An integration test runs the `bootstrap` run mode and checks that it refuses once an Instance administrator exists, that a rerun replaces the earlier code, and that the code appears in no log record.
  - Watermill handlers are drained on demand by a test helper that processes every pending event, so deferred effects (in this slice, Notifications) are asserted deterministically, without sleeping. Immediate reads such as the transaction list are asserted without draining.

  The HTTP tests cover the whole user journey: CSV mapping variants (signed vs debit/credit columns, header vs positional, inverted sign, decimal and date formats), file-level and row-level rejections, duplicate file, overlapping files with and without bank IDs (skipped vs Suspected duplicate), identical rows within one file, keep and Discard, the notification appearing after events are drained, Accept with and without a Category, recategorising, every Month close rejection reason (singly and together), the Opening balance worked out from the first import (including rows before the start date ignored), the first import's preview and confirm (mismatched Export balance entries refused, a preview records nothing, a confirm records the import and the Opening balance, a confirm whose file, Export balance or mapping doesn't match its preview is refused, a stale preview refused once the Account got its Opening balance in the meantime, a first import without `preview_id` refused), balance check pass and refusal on later imports, a check waiting for Suspected duplicates and then passing or failing, reverse and re-Accept, Discard after reversal, void and re-upload with a fixed mapping, voiding the first import reopening the Opening balance, refusing to void an earlier import or one with Accepted rows, flagging after close, Idempotency-Key replay, and Household isolation (a second Household gets 404 for every resource of the first).
- **Seam 2: Deciders (pure given/when/then).** Given past events, when a command, then new events or an error. This covers the rules where cases multiply: Invitation issue, reissue and redemption (wrong code, expired, already redeemed), Account Month close conditions and flagging, Transaction balance invariants and reversal rules, Statement import balance check and void rules, Imported transaction state transitions, Category kind inheritance and sibling uniqueness, and CSV mapping validation.
- **One required exception.** conventions.md and ADR-0014 require proof that row-level security blocks access on its own, with the query filters bypassed. One integration test at the PostgreSQL adapter level does this for each Household-scoped table. Another shows that a transaction with a Household set can't read or write Users, Invitations or instance-level events, and that a transaction in instance scope can't touch Household rows.
- **Contract.** `make lint` runs vacuum with the JavaScript-free Zalando ruleset on `api/openapi.yaml`. Because the HTTP tests drive the ogen-generated server, every response is also checked against the contract.
- **Prior art.** The repo has no tests yet. The spike on branch `prototype/spike-eventstore-ogen` is the reference for the techniques: driving the custom SchemaAdapter, proving handler behaviour against a real database, and checking every ogen error path for problem+json. Its checks are throwaway, but the scenarios they covered (gap safety, replay, Nack and shutdown redelivery, cross-Household writes from a handler) should survive as integration tests of the platform module.

## Out of Scope

- Manual Transactions entered by hand, and therefore Matching manual entries to Imported transactions. Only Accept exists in this slice.
- Rules and Category suggestions learned from past transactions. The User picks the Category or leaves it Uncategorised.
- Transfers between Accounts, split Transactions with several Categories, and Household transfers.
- Foreign-currency rows, Bank rate and Reference rate.
- Statement formats other than CSV (CAMT.053, OFX, PDF), live bank feeds, and ready-made CSV mapping presets for specific banks.
- Reading the Export balance or a Statement balance from the file. The User enters the Export balance with each upload and the Statement balance at Month close.
- Voiding any Statement import other than the latest one. Fixing an older import means voiding every later one first.
- Reopening a closed month, unflagging, and any ordering rule between months.
- Deleting or archiving Accounts, Categories, Connections, Statement imports or Transactions. Corrections are made by reversal, Discard and void only.
- Invitations issued from inside the app (who may issue them, Household-bound Invitations, listing, revoking, configurable expiry), so a second User can't join yet. Also out: Household roles, leaving a Household, and recovering a lost Instance administrator.
- Notification delivery outside the app (email, push). Notifications are only listed through the API in this slice.
- Budgets, Scheduled transactions, Forecasts, Category totals and reports.
- A message broker. Forwarding stays in-process.

## Further Notes

- Postponed decisions this slice doesn't settle: bank aggregator, other statement formats, Zitadel configuration details, Household roles.
- **Open question: cash Accounts.** An Account gets its Opening balance only from a Statement import, and gaps are closed only by correcting entries. A cash Account has no statement file, and a counted difference (cash lost) is a real event, not a wrong entry. As written, a cash Account can be added but never gets a balance. Options: keep cash Accounts out of this slice, bring manual Transactions in, or give cash Accounts their own way to set an Opening balance.
- `docs/architecture/conventions.md` still says the telemetry backend is undecided. ADR-0016 has decided it (OpenTelemetry Collector and the Grafana stack), so that line should be updated in this slice.
