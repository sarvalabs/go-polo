package polo

import (
	"encoding/binary"
	"unicode/utf8"
)

// The UTF-8 checks of the encoder and the decoder give the same result as utf8.ValidString and utf8.Valid.
// They first check if short data is only ASCII, because most strings in a POLO wire are short
// (field names, map keys and words), and utf8.ValidString and utf8.Valid examine short data
// one byte at a time. Data of asciiLimit bytes or more goes directly to the utf8 package,
// which is fast for long data.

// asciiLimit is the length from which the UTF-8 checks use the utf8 package directly
const asciiLimit = 32

// hiBits4 and hiBits8 select the high bit of each byte in 4 and 8 bytes. A byte with the high bit is not ASCII
const (
	hiBits4 = 0x80808080
	hiBits8 = 0x8080808080808080
)

// validString returns whether s is valid UTF-8, with the same result as utf8.ValidString
func validString(s string) bool {
	return (len(s) < asciiLimit && asciiString(s)) || utf8.ValidString(s)
}

// validBytes returns whether b is valid UTF-8, with the same result as utf8.Valid
func validBytes(b []byte) bool {
	return (len(b) < asciiLimit && asciiBytes(b)) || utf8.Valid(b)
}

// asciiString returns whether s contains only ASCII bytes.
// It reads 4 or 8 bytes at a time. The last read can overlap with the read before it.
func asciiString(s string) bool {
	n := len(s)

	switch {
	case n >= 8:
		last := s[n-8:]

		for len(s) > 8 {
			if word8(s)&hiBits8 != 0 {
				return false
			}

			s = s[8:]
		}

		return word8(last)&hiBits8 == 0

	case n >= 4:
		return (word4(s)|word4(s[n-4:]))&hiBits4 == 0
	}

	for i := range n {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// asciiBytes returns whether b contains only ASCII bytes.
// It reads 4 or 8 bytes at a time. The last read can overlap with the read before it.
func asciiBytes(b []byte) bool {
	n := len(b)

	switch {
	case n >= 8:
		last := b[n-8:]

		for len(b) > 8 {
			if binary.LittleEndian.Uint64(b)&hiBits8 != 0 {
				return false
			}

			b = b[8:]
		}

		return binary.LittleEndian.Uint64(last)&hiBits8 == 0

	case n >= 4:
		return (binary.LittleEndian.Uint32(b)|binary.LittleEndian.Uint32(b[n-4:]))&hiBits4 == 0
	}

	for i := range n {
		if b[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// word4 returns the first 4 bytes of s as a little-endian uint32 (the compiler makes this one load)
func word4(s string) uint32 {
	return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
}

// word8 returns the first 8 bytes of s as a little-endian uint64 (the compiler makes this one load)
func word8(s string) uint64 {
	return uint64(word4(s)) | uint64(word4(s[4:]))<<32
}
