# 10: Connection name: uniqueness and rename

**What to build:** Connection names are unique within the Account, ignoring case, and `PATCH …/connections/{connection_id}` renames one. Spec: `../spec.md`, "Connections and the CSV mapping" (Name), stories 21, 22.

**Blocked by:** 01

**Status:** resolved

- [x] Rename uses the trim and 1–100 rule that ticket 01 put in the Decider (`ErrConnectionNameInvalid` → 422 `invalid-connection` for a name of only spaces; ogen gives 400 for an empty or too long one)
- [x] Unique index `(household_id, account_id, lower(name))` on the lookup rows → 422 `connection-name-taken` with the spec's copy
- [x] `RenameConnection` → `ConnectionRenamed` (registered); the same name records nothing
- [x] HTTP tests: a duplicate on create and on rename, differing only by case; the same name on another Account is allowed; rename to the same name

## Comments

- 2026-11-23, implementation notes:
  - **Domain:** `RenameConnection{Name}` → `ConnectionRenamed{Name}` (`io.walletaccountant.ingestion.connection.renamed.v1`, registered on the codec). It reuses `connectionName` (trim, 1–100), refuses a Connection not created, and records nothing when the trimmed name equals the current one exactly. A change of case only is a rename. The fold's shape is unchanged, so no fold schema bump.
  - **Uniqueness:** migration `000020_connection_name_unique` adds `connections_name_idx` on `(household_id, account_id, lower(name))`. `ConnectionFolds.Save` maps its 23505 to `app.ErrConnectionNameTaken`, which the request transaction rolls back. Create and rename both answer 422 `/problems/connection-name-taken`, title "Connection name taken", detail `This Account already has a Connection named "<trimmed requested name>".`
  - **Service:** `Connections.Rename`; it and `SaveCSVMapping` now share `inConnection` (the fold must exist and belong to the Account, otherwise 404).
  - **Tests:** domain tests in `connection_test.go`; HTTP tests in `internal/ingestion/adapters/http/connection_name_integration_test.go` (duplicate on create and on rename differing by case, the same name on another Account, rename to its own name, case-only rename, rebuild, the old name freed, 400/404 refusals). `apitest.App.Patch` added. Three existing tests created two "Statement file" Connections on one Account; their second Connection is renamed.
