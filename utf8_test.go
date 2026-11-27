package polo

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

// checkUTF8 checks that validString and validBytes give the same result as the utf8 package
func checkUTF8(t *testing.T, data []byte) {
	t.Helper()

	assert.Equal(t, utf8.ValidString(string(data)), validString(string(data)), "string: %x", data)
	assert.Equal(t, utf8.Valid(data), validBytes(data), "bytes: %x", data)
}

func TestValidUTF8(t *testing.T) {
	t.Run("Fixed", func(t *testing.T) {
		tests := []struct {
			input string
			valid bool
		}{
			{"", true},
			{"a", true},
			{"pride", true},
			{"Sins & Virtues", true},
			{strings.Repeat("x", asciiLimit-1), true},
			{strings.Repeat("x", asciiLimit), true},
			{"čšž", true},
			{"abcdefgh€", true},
			{"\xff", false},
			{"abc\x80", false},
			{"abcdefg\xc3", false},
			{strings.Repeat("x", 30) + "\xe2\x82", false},
		}

		for _, test := range tests {
			assert.Equal(t, test.valid, validString(test.input), "string: %q", test.input)
			assert.Equal(t, test.valid, validBytes([]byte(test.input)), "bytes: %q", test.input)
		}
	})

	// One byte that is not ASCII at each position, for each length around the 4-byte and 8-byte reads
	t.Run("Positions", func(t *testing.T) {
		for n := 0; n <= 2*asciiLimit; n++ {
			for pos := -1; pos < n; pos++ {
				for _, c := range []byte{0x7f, 0x80, 0xbf, 0xc3, 0xe2, 0xf0, 0xff} {
					data := bytes.Repeat([]byte{'a'}, n)
					if pos >= 0 {
						data[pos] = c
					}

					checkUTF8(t, data)
				}
			}
		}
	})

	t.Run("AllTwoBytes", func(t *testing.T) {
		for i := range 1 << 16 {
			checkUTF8(t, []byte{byte(i >> 8), byte(i)})
		}
	})

	t.Run("Fuzz", func(t *testing.T) {
		f := fuzz.New().NilChance(0).NumElements(0, 2*asciiLimit)

		var (
			data []byte
			text string
		)

		for range 100000 {
			f.Fuzz(&data)
			checkUTF8(t, data)

			f.Fuzz(&text)
			checkUTF8(t, []byte(text))
		}
	})
}
