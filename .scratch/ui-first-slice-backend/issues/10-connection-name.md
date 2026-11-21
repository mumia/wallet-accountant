# 10: Connection name: uniqueness and rename

**What to build:** Connection names are unique within the Account, ignoring case, and `PATCH …/connections/{connection_id}` renames one. Spec: `../spec.md`, "Connections and the CSV mapping" (Name), stories 21, 22.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Trim and 1–100 rule in the Decider (`ErrConnectionNameInvalid` → 422 `invalid-connection`)
- [ ] Unique index `(household_id, account_id, lower(name))` on the lookup rows → 422 `connection-name-taken` with the spec's copy
- [ ] `RenameConnection` → `ConnectionRenamed` (registered); the same name records nothing
- [ ] HTTP tests: a duplicate on create and on rename, differing only by case; the same name on another Account is allowed; rename to the same name

## Comments
