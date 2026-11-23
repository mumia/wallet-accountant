# 11: CSV mapping encoding

**What to build:** a CSV mapping says whether its files are UTF-8 or Windows-1252, and the file is decoded before it is parsed. Spec: `../spec.md`, "Connections and the CSV mapping" (Encoding), story 23.

**Blocked by:** 01

**Status:** resolved

- [x] `Encoding` on `CSVMapping` and `CSVMappingSaved`; absent means `UTF_8`; an unknown value is a mapping problem
- [x] Decode before parse, in the domain with a stdlib table (0x80–0x9F through the Windows-1252 table, other bytes to the same code point; the domain's depguard rule forbids `golang.org/x/text`); BOM strip unchanged; the hash stays over raw bytes
- [x] Domain test: a Windows-1252 file with `é`/`€` reads correctly, and the same bytes as UTF-8 give unreadable rows or mojibake as today
- [x] HTTP test: save with `WINDOWS_1252`, import a Windows-1252 file

## Comments

- 2026-11-23, implementation notes:
  - **Domain:** `Encoding` (`utf_8` | `windows_1252`, the empty value meaning UTF-8) on `CSVMapping` as `encoding,omitempty`, so it travels in `CSVMappingSaved` and older events read as UTF-8. No new event and no fold schema bump. `Validate` refuses any other value with `ErrInvalidCSVMapping`, "the encoding must be UTF-8 or Windows-1252".
  - **Decode:** `internal/ingestion/domain/encoding.go`. A UTF-8 BOM is stripped first, as before, for either encoding. Then Windows-1252 maps 0x80–0x9F through the 27-entry table, and every other byte (the five undefined ones included) maps to its own code point. `parseStatementFile` decodes before skipping lines, so the hash in `statement.go` is still over the raw bytes.
  - **HTTP:** `mappingIn`/`mappingOut` map `UTF_8`/`WINDOWS_1252`. An unknown value becomes `api:<value>`, like the other enums, so it is refused as a mapping problem (422 `invalid-csv-mapping`). ogen fills in the contract's `default: UTF_8` on decode, and responses always carry `encoding`, so `signed()`/`debitCredit()` in `connections_integration_test.go` now include `"encoding": "UTF_8"`.
  - **Tests:** domain tests in `statementfile_test.go` (a Windows-1252 file with `é`/`€`/curly quotes/an undefined byte; the same bytes as UTF-8 come through raw, i.e. mojibake, as today; UTF-8 with a BOM) and in `connection_test.go` (valid encodings, unknown encoding). HTTP tests in `mapping_encoding_integration_test.go`: save and list with each encoding or none, unknown → 422, and confirming an import of a Windows-1252 file.
