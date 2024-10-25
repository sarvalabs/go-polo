package polo

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

func wiretypeFuzzer(w *WireType, c fuzz.Continue) {
	// 0-7 & 13-15 valid range, 9-12 reserved, 16-17 invalid
	*w = WireType(c.Intn(18))
}

func TestWireType(t *testing.T) {
	stringMap := map[WireType]string{
		WireNull:     "null",
		WireFalse:    "false",
		WireTrue:     "true",
		WirePosInt:   "posint",
		WireNegInt:   "negint",
		WireRaw:      "raw",
		WireWord:     "word",
		WireFloat:    "float",
		WireType(8):  "reserved",
		WireType(9):  "reserved",
		WireType(10): "reserved",
		WireType(11): "reserved",
		WireType(12): "reserved",
		WireDoc:      "document",
		WirePack:     "pack",
		WireLoad:     "load",
		WireType(16): "unknown",
		WireType(17): "unknown",
	}

	f := fuzz.New().Funcs(wiretypeFuzzer)

	t.Run("String", func(t *testing.T) {
		var w WireType
		for i := 0; i < 10000; i++ {
			f.Fuzz(&w)
			result := w.String()
			assert.Equal(t, stringMap[w], result, "Input: %d", w)
		}
	})

	t.Run("IsNull", func(t *testing.T) {
		var w WireType
		for i := 0; i < 10000; i++ {
			f.Fuzz(&w)
			result := w.IsNull()

			switch {
			case w > 15, w == 0, w >= 8 && w <= 12:
				assert.True(t, result)
			default:
				assert.False(t, result)
			}
		}
	})

	t.Run("IsCompound", func(t *testing.T) {
		var w WireType
		for i := 0; i < 10000; i++ {
			f.Fuzz(&w)
			result := w.IsCompound()

			if w == WirePack || w == WireDoc {
				assert.True(t, result)
			} else {
				assert.False(t, result)
			}
		}
	})
}

func TestIsWireType(t *testing.T) {
	tests := []struct {
		name string
		wire Any
		is   WireType
		not  WireType
	}{
		{"0", nil, WireNull, WirePack},
		{"1", []byte{}, WireNull, WireWord},
		{"2", []byte{0}, WireNull, WireFloat},
		{"3", []byte{6, 109, 97, 110, 105, 115, 104}, WireWord, WireFloat},
		{"4", []byte{3, 1, 44}, WirePosInt, WireNegInt},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.True(t, IsWireType(test.wire, test.is))
			assert.False(t, IsWireType(test.wire, test.not))
		})
	}
}
