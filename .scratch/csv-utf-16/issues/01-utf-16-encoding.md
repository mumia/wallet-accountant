# 01: UTF-16 CSV mapping encoding

**What to build:** a CSV mapping can say its files are UTF-16, and such a file is decoded before it is parsed, as `WINDOWS_1252` already is (ui-first-slice-backend ticket 11).

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

## Why

A User's bank export said to be Windows-1252 was really UTF-16, as Excel's "Unicode Text" export and many bank exports write it. Decoded as Windows-1252, every character keeps a NUL byte beside it, so every row fails with problems like:

> the Booking date "\x001\x006\x00-\x001\x001\x00-\x002\x000\x002\x006\x00" is not a date in the format DD-MM-YYYY; … the amount "\x00-\x007\x006\x00.\x002\x008\x00" is not a number

Today the only workaround is to convert the file to UTF-8 by hand.

## What to build

- [ ] `api/openapi.yaml`: `CsvMapping.encoding` gains `UTF_16`. Its description says:
  - the byte order comes from the byte order mark (`FF FE` little-endian, `FE FF` big-endian), which is stripped;
  - a file with no byte order mark is read as little-endian, which is what Windows writes.
  
  `encoding` is an extensible enum, so this is not a breaking change. It ships in the next release.
- [ ] Domain: `EncodingUTF16 Encoding = "utf_16"` in `internal/ingestion/domain/encoding.go`.
  - Decode with the standard library only (`unicode/utf16`, `encoding/binary`): the domain's depguard rule forbids `golang.org/x/text`.
  - A lone surrogate becomes U+FFFD.
  - A file with an odd number of bytes is refused as unreadable, not silently truncated. Pick the problem that fits how other unreadable files are reported.
  - The UTF-8 BOM strip still applies to the other two encodings. The hash stays over the raw bytes.
- [ ] `Validate` accepts `utf_16`. Its message for an unknown encoding names all three encodings.
- [ ] HTTP: `mappingIn`/`mappingOut` map `UTF_16`, for save and list, and for the mapping test endpoint (`POST …/csv-mapping-test`).
- [ ] Domain tests in `statementfile_test.go`:
  - a UTF-16LE file with a BOM, and a UTF-16BE file with a BOM, each with `é`/`€` and a `;` delimiter, read correctly;
  - a UTF-16LE file with no BOM reads correctly;
  - an odd-length file is refused.
- [ ] HTTP tests in `mapping_encoding_integration_test.go`:
  - save and list with `UTF_16`;
  - test a mapping against a UTF-16 sample;
  - confirm an import of a UTF-16 file.

## Then

The UI moves to the release with `make contract-update` and offers the encoding. Its ticket is `../wallet-accountant-ui/.scratch/csv-utf-16/issues/01-offer-utf-16.md`.
