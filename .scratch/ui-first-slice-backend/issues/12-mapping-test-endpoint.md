# 12: CSV mapping test endpoint

**What to build:** `POST …/accounts/{account_id}/csv-mapping-tests` reads a sample file with an unsaved mapping and returns the first 10 rows, the readable count and every unreadable line, recording nothing. Spec: `../spec.md`, "Connections and the CSV mapping" (Mapping test), stories 24, 25.

**Blocked by:** 11

**Status:** ready-for-agent

- [ ] A collecting mode in the statement reader (readable rows and `RowError`s together); upload keeps refusing whole files
- [ ] `Connections.TestMapping`: same mapping validation as save, Account currency, no start-date filter, same size limit as an upload
- [ ] HTTP tests: a good file (10 rows plus the count), a mixed file, a fully unreadable file, an invalid mapping (422 `invalid-csv-mapping`), an empty file (422 `empty-statement-file`); nothing written to the event store (`eventstoretest` counter)

## Comments
