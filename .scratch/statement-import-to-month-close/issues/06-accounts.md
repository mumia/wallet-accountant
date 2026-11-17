# 06: Accounts

**What to build:** a User adds an Account with a name, a kind, an Account currency and a start date, and lists or opens the Household's Accounts. A new Account shows that it is awaiting its first Statement import and has no balance. Nobody can type an Opening balance. Spec: `../spec.md` stories 15–20; ADR-0004, ADR-0007, ADR-0018.

**Blocked by:** 04 (Households, membership, Internal Categories and Idempotency-Key)

**Status:** resolved

- [ ] `POST/GET /households/{household_id}/accounts` and `GET …/accounts/{account_id}`
- [ ] Account kinds are bank account, credit card, loan and crypto wallet. Cash Accounts are out of scope for this slice and are refused
- [ ] Account Decider holding kind, Account currency, start date and whether it awaits its first Statement import
- [ ] A new Account reports awaiting its first Statement import, with no balance
- [ ] No field or endpoint sets or edits an Opening balance
- [ ] Amounts are JSON decimals with `format: decimal`, paired with a currency code, using the currency's exact number of decimal places (ADR-0007)
- [ ] HTTP tests: add, list, get; refusing a cash or unknown kind; another Household gets 404
- [ ] Row-level security test for every table this ticket adds; Account fold registered for rebuild
- [ ] Consistency map row for Add Account matches the code

## Comments

Implementation notes (implementer agent):

- **Where things live.** Account Decider in `internal/ledger/domain/account.go` (command `AddAccount`, event `AccountAdded` = `io.walletaccountant.ledger.account.added.v1`, `startdate` written as `YYYY-MM-DD`); service `ledgerapp.Accounts` (`Add`, `List`, `Get`) in `internal/ledger/app/accounts.go`, running through the existing `HouseholdScope` port; fold store and reader in `internal/ledger/adapters/postgres/accounts.go`; HTTP operations on a separate `ledgerhttp.AccountHandler` (`internal/ledger/adapters/http/accounts.go`), embedded in `wiring.apiHandler` beside the Categories handler so ticket 05's edits to `handler.go` don't clash. `wiring.Contexts` gained `Accounts`; the Account aggregate is a rebuild target.
- **Migration number.** `000006_accounts` (ticket 05 takes `000005`). golang-migrate runs the gap fine; whoever merges second should check the order still holds.
- **Contract.** `POST/GET /households/{household_id}/accounts`, `GET …/accounts/{account_id}`. Kinds are an `x-extensible-enum` `BANK_ACCOUNT|CREDIT_CARD|LOAN|CRYPTO_WALLET` (upper case, like Category kinds; only these exact spellings are accepted). `CASH`, any other kind, an unknown currency or a blank name get 422 `/problems/invalid-account`, each with its own detail. The body has no Opening balance field; unknown fields such as `opening_balance` are ignored by ogen, which a test shows leaves the Account awaiting its first import with no balance. The response carries `awaiting_first_statement_import` and an optional `balance`, always absent for now.
- **Money.** A reusable `Money` schema (`amount`: `type: number, format: decimal`; `currency`: `CurrencyCode`) is in the contract for `balance`. ogen maps it to `shopspring/decimal.Decimal` (now a direct dependency), so it is exact. **For ticket 08:** ogen encodes it with `decimal.String()`, which drops trailing zeros (`100.00` goes out as `100`). The value is exact, but to show "the currency's exact number of decimal places" on the wire, either quantise and accept that JSON numbers lose trailing zeros, or change the contract/encoding; nothing in this ticket emits an amount yet, so no test covers it.
- **Fold.** The `accounts` table holds kind, currency, start date and `awaiting_first_statement_import`; no balance columns yet (ticket 08 adds the running balance per Booking month and bumps the fold schema version).
- **Tests.** Decider given/when/then (each kind, cash, unknown kind, unknown currency, blank name, missing start date, adding twice, the new fold awaiting its first import); HTTP: add and get, list (only this Household's, in order), every refusal, a typed Opening balance ignored, rebuild, Idempotency-Key replay, and 404 for another Household (list, get, add, and an Account opened through the caller's own Household). `databasetest.AssertHouseholdIsolation` for `accounts`.
- **Consistency map.** The existing Add Account row ("Account created, awaiting its first Statement import"; nothing deferred) matches the code: one Decider in the request transaction, no handler. Left unchanged.
