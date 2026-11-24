package domain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"unicode/utf16"
	"unicode/utf8"
)

// Encoding is how a statement file's bytes are decoded into text before
// the file is parsed. The zero Encoding is EncodingUTF8.
type Encoding string

// The encodings.
const (
	EncodingUTF8        Encoding = "utf_8"
	EncodingWindows1252 Encoding = "windows_1252"
	// EncodingUTF16 takes its byte order from the byte order mark, which is
	// stripped; a file with none is little-endian, as Windows writes it.
	EncodingUTF16 Encoding = "utf_16"
)

// utf8BOM is the UTF-8 byte order mark, stripped from the start of a file.
var utf8BOM = []byte("\xef\xbb\xbf")

// The UTF-16 byte order marks.
var (
	utf16LEBOM = []byte("\xff\xfe")
	utf16BEBOM = []byte("\xfe\xff")
)

// windows1252 maps the bytes 0x80–0x9F that Windows-1252 gives a character
// other than their own code point. Every other byte, including the five it
// leaves undefined, decodes to its own code point (as ISO-8859-1).
var windows1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…',
	0x86: '†', 0x87: '‡', 0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š',
	0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘', 0x92: '’',
	0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ',
	0x9E: 'ž', 0x9F: 'Ÿ',
}

// The reasons a file is refused before it is parsed, as the error's text.
var (
	errOddUTF16 = errors.New("the file has an odd number of bytes, so it is not UTF-16")
	errUTF16BOM = errors.New("the file starts with a UTF-16 byte order mark; set the mapping's encoding to UTF-16")
)

// decode turns a statement file's bytes into UTF-8 text. It refuses a file
// the encoding can't read, with the reason as the error's text: a UTF-16
// file with an odd number of bytes, or a file under another encoding that
// starts with a UTF-16 byte order mark. A UTF-8 file is passed through as
// it is, after a leading UTF-8 byte order mark is stripped, as it is from a
// Windows-1252 file.
func (e Encoding) decode(file []byte) ([]byte, error) {
	if e == EncodingUTF16 {
		return decodeUTF16(file)
	}

	if bytes.HasPrefix(file, utf16LEBOM) || bytes.HasPrefix(file, utf16BEBOM) {
		return nil, errUTF16BOM
	}

	file = bytes.TrimPrefix(file, utf8BOM)

	if e == EncodingWindows1252 {
		return decodeWindows1252(file), nil
	}

	return file, nil
}

func decodeWindows1252(file []byte) []byte {
	out := make([]byte, 0, len(file)+len(file)/2)

	for _, b := range file {
		if b < utf8.RuneSelf {
			out = append(out, b)

			continue
		}

		r, ok := windows1252[b]
		if !ok {
			r = rune(b)
		}

		out = utf8.AppendRune(out, r)
	}

	return out
}

// decodeUTF16 decodes UTF-16 in the byte order its byte order mark gives,
// little-endian when it has none; a lone surrogate becomes U+FFFD.
func decodeUTF16(file []byte) ([]byte, error) {
	if len(file)%2 != 0 {
		return nil, errOddUTF16
	}

	var order binary.ByteOrder = binary.LittleEndian

	switch {
	case bytes.HasPrefix(file, utf16LEBOM):
		file = file[len(utf16LEBOM):]
	case bytes.HasPrefix(file, utf16BEBOM):
		file, order = file[len(utf16BEBOM):], binary.BigEndian
	}

	units := make([]uint16, 0, len(file)/2)
	for i := 0; i < len(file); i += 2 {
		units = append(units, order.Uint16(file[i:]))
	}

	out := make([]byte, 0, len(file))
	for _, r := range utf16.Decode(units) {
		out = utf8.AppendRune(out, r)
	}

	return out, nil
}
