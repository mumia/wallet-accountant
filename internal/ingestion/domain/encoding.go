package domain

import (
	"bytes"
	"unicode/utf8"
)

// Encoding is how a statement file's bytes are decoded into text before
// the file is parsed. The zero Encoding is EncodingUTF8.
type Encoding string

// The encodings.
const (
	EncodingUTF8        Encoding = "utf_8"
	EncodingWindows1252 Encoding = "windows_1252"
)

// utf8BOM is the UTF-8 byte order mark, stripped from the start of a file.
var utf8BOM = []byte("\xef\xbb\xbf")

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

// decode turns a statement file's bytes into UTF-8 text: a leading UTF-8
// byte order mark is stripped, then the bytes are decoded with the
// encoding. A UTF-8 file is passed through as it is.
func (e Encoding) decode(file []byte) []byte {
	file = bytes.TrimPrefix(file, utf8BOM)

	if e != EncodingWindows1252 {
		return file
	}

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
