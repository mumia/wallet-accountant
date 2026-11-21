# 11: CSV mapping encoding

**What to build:** a CSV mapping says whether its files are UTF-8 or Windows-1252, and the file is decoded before it is parsed. Spec: `../spec.md`, "Connections and the CSV mapping" (Encoding), story 23.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `Encoding` on `CSVMapping` and `CSVMappingSaved`; absent means `UTF_8`; an unknown value is a mapping problem
- [ ] Decode before parse, in the domain with a stdlib table (0x80–0x9F through the Windows-1252 table, other bytes to the same code point; the domain's depguard rule forbids `golang.org/x/text`); BOM strip unchanged; the hash stays over raw bytes
- [ ] Domain test: a Windows-1252 file with `é`/`€` reads correctly, and the same bytes as UTF-8 give unreadable rows or mojibake as today
- [ ] HTTP test: save with `WINDOWS_1252`, import a Windows-1252 file

## Comments
