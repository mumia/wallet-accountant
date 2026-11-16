# 07: Connections and CSV mapping

**What to build:** a User creates a statement file import Connection for an Account and describes how that bank's CSV is laid out. An invalid mapping is rejected when saved. The mapping can be changed later, and an Account's Connections can be listed. The pure CSV parser (mapping plus bytes to rows or row errors) is built and tested here, ready for imports. Spec: `../spec.md` stories 24–33, "Ingestion: Connection".

**Blocked by:** 06 (Accounts)

**Status:** claimed

- [ ] `POST/GET …/accounts/{account_id}/connections` and `PUT …/connections/{connection_id}/csv-mapping`
- [ ] The mapping covers delimiter, header row, leading lines to skip, date format, decimal separator, thousands separator; each field's column by header name or by position; one signed amount column or separate debit and credit columns; optional Transaction date, Value date, payee and bank transaction ID; and whether the sign means money in or out
- [ ] Saving validates the whole mapping: missing required field, unknown date format, or the same column used twice are rejected with problem+json
- [ ] The mapping has a version that changes on every save (the first import's preview relies on it)
- [ ] Pure parser: mapping plus file bytes to rows or a list of row numbers and reasons; Transaction date defaults to Booking date
- [ ] Decider-style tests for mapping validation and for every parser variant: signed vs debit/credit, header vs positional, inverted sign, decimal and date formats, skipped leading lines, row errors
- [ ] HTTP tests: create, list, save, change, invalid mappings; another Household gets 404
- [ ] Row-level security test for every table this ticket adds; Connection fold registered for rebuild
- [ ] Consistency map row for Save CSV mapping matches the code
