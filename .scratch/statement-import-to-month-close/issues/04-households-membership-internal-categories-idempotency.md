# 04: Households, membership, Internal Categories and Idempotency-Key

**What to build:** a User creates a Household with a name and a Base currency, becomes its member, and can list and open their Households. Each new Household comes with its three Internal Categories (Opening balance, Household transfer, Uncategorised). A User who isn't a member gets 404 for that Household and anything under it. Every POST endpoint, now and later, honours `Idempotency-Key`. Spec: `../spec.md` stories 3–8, 43, 52, 72, "Request transaction", "Authorisation"; ADR-0003, ADR-0014.

**Blocked by:** 03 (Bootstrap and registration)

**Status:** claimed

- [ ] Household Decider with membership in its fold; `POST/GET /households` and `GET /households/{household_id}`
- [ ] Creating a Household seeds its three Internal Categories in the same database transaction (this ticket introduces the Ledger Category aggregate only as far as seeding needs)
- [ ] Each Household-scoped request opens one database transaction as `wa_app`: it resolves the caller's User in instance scope, then sets `app.household_id` from the path before any Household-scoped query; every repository query also filters by `household_id`
- [ ] Household-scoped routes answer 404 to non-members and to unregistered callers; `/households` answers 403 `registration-required` to unregistered callers
- [ ] POST endpoints accept `Idempotency-Key`: a replay with the same key and body returns the stored response; the same key with a different body gets 422. Later POSTs get this by default
- [ ] A reusable integration test pattern proves row-level security blocks another Household's rows with the query filters bypassed, applied to every Household-scoped table this ticket adds
- [ ] Integration test: a transaction with a Household set can't read or write Users, Invitations or instance-level events, and a transaction in instance scope can't touch Household rows
- [ ] HTTP tests: two Users in two Households; each gets 404 for the other's Household; Idempotency-Key replay and mismatch
- [ ] The consistency map's Create Household row says three Internal Categories, not four, matching the spec
