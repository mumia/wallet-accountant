# 05: Account transfer Category, Transfer legs and the Match aggregate

**What to build:** the building blocks of a Transfer, with no endpoint that uses them yet except `getMatch`:
- the Account transfer Internal Category;
- a Transaction that is a Transfer leg naming its counterpart Account;
- the Match aggregate linking two legs.

Spec: `../spec.md`, "Ledger: Transfers".

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `RoleAccountTransfer` ("Account transfer"), seeded by Create Household. There is no migration
- [ ] Accept and Set Category refuse it like any other Internal Category
- [ ] `TransactionRecorded` gains `counterpartaccountid` and `createdbytransfer`. Counterpart checks: same Household, not itself, same currency (`invalid-transfer`)
- [ ] `TransactionMatched` and `TransactionUnmatched`. Matching a Matched, reversed or non-leg Transaction is refused. Set Category on a leg is refused (`transfer-leg`)
- [ ] Match stream: `match.confirmed` and `match.removed`, registered on the codec; `matches` fold table with row-level security; opposite amounts and crosswise counterparts checked
- [ ] `getMatch` (a removed Match gets 404); delete the stub
- [ ] `AccountTransaction` and `Transaction` return `counterpart_account_id`, `match_id` and `created_by_transfer`. `in_transit=true` filter; `AccountMonth.transfer_legs_in_transit`
- [ ] Consistency row: Create Household
- [ ] Decider tests for Transaction and Match; a row-level security adapter test for `matches`
