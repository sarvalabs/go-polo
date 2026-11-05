package polo

import (
	"bytes"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

func TestIsBitSize(t *testing.T) {
	var x int

	f := fuzz.New()
	for i := 0; i < 10000; i++ {
		f.Fuzz(&x)

		switch x {
		case 8, 16, 32, 64:
			assert.True(t, isBitSize(x))
		default:
			assert.False(t, isBitSize(x))
		}
	}
}

func TestSizeVarint(t *testing.T) {
	var (
		v        uint64
		expected int
	)

	f := fuzz.New().Funcs(func(v *uint64, c fuzz.Continue) {
		x := c.Int63n(64)
		y := c.Int63n(128)
		*v = uint64(y << (x))
	})

	for i := 0; i < 10000; i++ {
		f.Fuzz(&v)

		switch {
		case v < 1<<7:
			expected = 1
		case v < 1<<14:
			expected = 2
		case v < 1<<21:
			expected = 3
		case v < 1<<28:
			expected = 4
		case v < 1<<35:
			expected = 5
		case v < 1<<42:
			expected = 6
		case v < 1<<49:
			expected = 7
		case v < 1<<56:
			expected = 8
		case v < 1<<63:
			expected = 9
		default:
			expected = 10
		}

		result := sizeVarint(v)
		assert.Equal(t, expected, result, "Input: %v", v)
	}
}

func TestAppendVarint(t *testing.T) {
	var v uint64

	f := fuzz.New().Funcs(func(v *uint64, c fuzz.Continue) {
		x := c.Int63n(64)
		y := c.Int63n(128)
		*v = uint64(y << (x))
	})

	for i := 0; i < 10000; i++ {
		f.Fuzz(&v)

		b := make([]byte, 0, 10)
		b = appendVarint(b, v)

		assert.Equal(t, encodeVarint(v), b, "Input: %v", v)
	}
}

//nolint:lll
func TestConsumeVarint(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		value    uint64
		consumed int
		err      string
	}{
		{"valid", []byte{0}, 0, 1, ""},
		{"valid", []byte{1}, 1, 1, ""},
		{"valid", []byte{127}, 127, 1, ""},
		{"valid", []byte{128, 1}, 128, 2, ""},
		{"valid", []byte{128, 2}, 256, 2, ""},
		{"valid", []byte{140, 204, 239, 5}, 12314124, 4, ""},

		{"invalid: varint terminated", []byte{129}, 1, 2, errVarintTerminated.Error()},
		{"invalid: varint overflow", []byte{255, 128, 128, 128, 128, 128, 128, 128, 128, 127}, 127, 10, errVarintOverflow.Error()},
		{"invalid: varint overflow", []byte{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 127}, 0, 11, errVarintOverflow.Error()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := bytes.NewReader(test.input)
			val, con, err := consumeVarint(r)

			assert.Equal(t, test.value, val)
			assert.Equal(t, test.consumed, con, " reader: %v", r)

			if test.err == "" {
				assert.Nil(t, err)
			} else {
				assert.Equal(t, test.err, err.Error())
			}
		})
	}
}

func TestConsumeTag(t *testing.T) {
	// reference is the rule for a tag: consumeVarint, and the varint must be minimally encoded
	reference := func(input []byte) (uint64, int, error) {
		x, n, err := consumeVarint(bytes.NewReader(input))
		if err == nil && n != sizeVarint(x) {
			return x, n, errVarintNotMinimal
		}

		return x, n, err
	}

	check := func(t *testing.T, input []byte) {
		t.Helper()

		wantX, wantN, wantErr := reference(input)
		x, n, err := consumeTag(input)

		assert.Equal(t, wantErr, err, "input: %x", input)
		assert.Equal(t, wantX, x, "input: %x", input)
		assert.LessOrEqual(t, n, len(input), "input: %x", input)

		if wantErr == nil {
			assert.Equal(t, wantN, n, "input: %x", input)
		}
	}

	t.Run("Fixed", func(t *testing.T) {
		tests := []struct {
			input    []byte
			value    uint64
			consumed int
			err      error
		}{
			{[]byte{0}, 0, 1, nil},
			{[]byte{127, 5}, 127, 1, nil},
			{[]byte{128, 1}, 128, 2, nil},
			{[]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 1}, 1<<64 - 1, 10, nil},
			{[]byte{}, 0, 0, errVarintTerminated},
			{[]byte{129}, 1, 1, errVarintTerminated},
			{[]byte{128, 0}, 0, 2, errVarintNotMinimal},
			{[]byte{255, 128, 0}, 127, 3, errVarintNotMinimal},
			{[]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 2}, 1<<63 - 1, 10, errVarintOverflow},
			{[]byte{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 1}, 0, 10, errVarintOverflow},
		}

		for _, test := range tests {
			x, n, err := consumeTag(test.input)

			assert.Equal(t, test.err, err, "input: %x", test.input)
			assert.Equal(t, test.value, x, "input: %x", test.input)
			assert.Equal(t, test.consumed, n, "input: %x", test.input)
		}
	})

	t.Run("AllTwoBytes", func(t *testing.T) {
		for i := range 1 << 16 {
			check(t, []byte{byte(i >> 8), byte(i)})
			check(t, []byte{byte(i)})
		}
	})

	t.Run("Fuzz", func(t *testing.T) {
		f := fuzz.New().NilChance(0).NumElements(0, 12)

		var (
			input []byte
			value uint64
		)

		for range 100000 {
			f.Fuzz(&input)
			check(t, input)

			f.Fuzz(&value)
			check(t, appendVarint(nil, value))
		}
	})
}
