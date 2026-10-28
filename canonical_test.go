package polo

// Tests for the decoding rules of v0.4.11: integer bounds and NaN (GO-1, GO-6, GO-10), Null semantics (N1-N3),
// canonical decoding (C1-C7) and UTF-8 strings (U1, U2). Every wire that the decoder accepts must encode back
// into the same bytes, so that every value has exactly one encoding (see FuzzCanonical).

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireRoundtrip decodes the wire into a new value of the type of target and
// checks that the value is expected and that it encodes back into the same wire
func requireRoundtrip(t *testing.T, wire []byte, expected any, options ...EncodingOptions) {
	t.Helper()

	object := reflect.New(reflect.TypeOf(expected))
	require.NoError(t, Depolorize(object.Interface(), wire, options...))
	require.Equal(t, expected, object.Elem().Interface())

	encoded, err := Polorize(object.Elem().Interface(), options...)
	require.NoError(t, err)
	require.Equal(t, wire, encoded)
}

// packWire returns a WirePack of the elements written by the given function
func packWire(write func(*Polorizer)) []byte {
	polorizer := NewPolorizer()
	write(polorizer)

	return polorizer.Packed()
}

// docWire returns a WireDoc with the given keys in the given order (each with the value 1)
func docWire(keys ...string) []byte {
	document := NewPolorizer()

	for _, key := range keys {
		document.PolorizeString(key)
		document.PolorizeRaw(Raw{3, 1})
	}

	var wb writebuffer

	wb.write(WireDoc, document.wb.load())

	return wb.bytes()
}

func TestGO1_MinInt64(t *testing.T) {
	wire := hexWire(t, "048000000000000000")

	// MinInt64 decodes and encodes back into the same wire
	requireRoundtrip(t, wire, int64(math.MinInt64))
	requireRoundtrip(t, wire, math.MinInt)

	depolorizer, err := NewDepolorizer(wire)
	require.NoError(t, err)

	value, err := depolorizer.DepolorizeInt64()
	require.NoError(t, err)
	assert.Equal(t, int64(math.MinInt64), value)

	depolorizer, err = NewDepolorizer(wire)
	require.NoError(t, err)

	integer, err := depolorizer.DepolorizeInt()
	require.NoError(t, err)
	assert.Equal(t, math.MinInt, integer)

	// The magnitudes 2^63 for a positive and 2^63 + 1 for a negative integer overflow
	for _, overflow := range []string{"038000000000000000", "048000000000000001", "04ffffffffffffffff"} {
		var object int64

		err := Depolorize(&object, hexWire(t, overflow))
		require.EqualError(t, err, "incompatible value error: overflow for signed integer", overflow)

		var valueErr IncompatibleValueError

		require.ErrorAs(t, err, &valueErr)
	}
}

func TestGO6_SignedIntegerBounds(t *testing.T) {
	tests := []struct {
		wire   string
		value  any // nil if the wire overflows
		method func(*Depolorizer) (any, error)
	}{
		{"037f", int8(math.MaxInt8), func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() }},
		{"0380", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() }},
		{"03ff", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() }},
		{"0480", int8(math.MinInt8), func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() }},
		{"0481", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() }},
		{"037fff", int16(math.MaxInt16), func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() }},
		{"038000", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() }},
		{"048000", int16(math.MinInt16), func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() }},
		{"048001", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() }},
		{"04ffff", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() }},
		{"037fffffff", int32(math.MaxInt32), func(d *Depolorizer) (any, error) { return d.DepolorizeInt32() }},
		{"0380000000", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt32() }},
		{"0480000000", int32(math.MinInt32), func(d *Depolorizer) (any, error) { return d.DepolorizeInt32() }},
		{"0480000001", nil, func(d *Depolorizer) (any, error) { return d.DepolorizeInt32() }},
	}

	for _, test := range tests {
		t.Run(test.wire, func(t *testing.T) {
			wire := hexWire(t, test.wire)

			depolorizer, err := NewDepolorizer(wire)
			require.NoError(t, err)

			value, err := test.method(depolorizer)

			if test.value == nil {
				require.EqualError(t, err, "incompatible value error: overflow for signed integer")

				var valueErr IncompatibleValueError

				require.ErrorAs(t, err, &valueErr)

				// The reflective decoder has the same bounds (also for a named type)
				object := reflect.New(reflect.TypeOf(value))
				require.Error(t, Depolorize(object.Interface(), wire))

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.value, value)
			requireRoundtrip(t, wire, test.value)
		})
	}

	t.Run("named type", func(t *testing.T) {
		type Small int8

		requireRoundtrip(t, hexWire(t, "0480"), Small(math.MinInt8))

		var object Small

		err := Depolorize(&object, hexWire(t, "0380"))
		require.EqualError(t, err, "incompatible value error: overflow for signed integer")
	})
}

func TestGO10_NaN(t *testing.T) {
	t.Run("float64", func(t *testing.T) {
		wire := hexWire(t, "077ff8000000000001")

		var object float64

		require.NoError(t, Depolorize(&object, wire))
		require.True(t, math.IsNaN(object))
		require.Equal(t, uint64(0x7ff8000000000001), math.Float64bits(object))

		encoded, err := Polorize(object)
		require.NoError(t, err)
		require.Equal(t, wire, encoded)

		depolorizer, err := NewDepolorizer(wire)
		require.NoError(t, err)

		value, err := depolorizer.DepolorizeFloat64()
		require.NoError(t, err)
		require.Equal(t, uint64(0x7ff8000000000001), math.Float64bits(value))
	})

	t.Run("float32", func(t *testing.T) {
		type Named float32

		type Object struct {
			F float32
			N Named
			P *float32
		}

		// Quiet and signaling NaNs with payloads keep their bits. The encoder must not convert the float32
		// into a float64, because the conversion turns a signaling NaN into a quiet NaN (other bits)
		for _, bits := range []uint32{0x7fc00001, 0x7f800001, 0xff800001, 0x7fffffff} {
			wire := []byte{7, byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits)}

			var object float32

			require.NoError(t, Depolorize(&object, wire))
			require.Equal(t, bits, math.Float32bits(object))

			encoded, err := Polorize(object)
			require.NoError(t, err)
			require.Equal(t, wire, encoded)

			// Inside a struct, as a named type and behind a pointer
			nan := math.Float32frombits(bits)

			structure, err := Polorize(Object{F: nan, N: Named(nan), P: &nan})
			require.NoError(t, err)

			var decoded Object

			require.NoError(t, Depolorize(&decoded, structure))
			require.Equal(t, bits, math.Float32bits(decoded.F))
			require.Equal(t, bits, math.Float32bits(float32(decoded.N)))
			require.Equal(t, bits, math.Float32bits(*decoded.P))

			reencoded, err := Polorize(decoded)
			require.NoError(t, err)
			require.Equal(t, structure, reencoded)

			depolorizer, err := NewDepolorizer(wire)
			require.NoError(t, err)

			value, err := depolorizer.DepolorizeFloat32()
			require.NoError(t, err)
			require.Equal(t, bits, math.Float32bits(value))
		}
	})
}

// customDecodes counts the calls of customNullable.Depolorize
var customDecodes int

// customNullable has a custom codec in the style of go-pisa and go-moi:
// its decoder treats a WireNull as success and leaves the zero value
type customNullable struct{ A uint64 }

func (c customNullable) Polorize() (*Polorizer, error) {
	polorizer := NewPolorizer()
	polorizer.PolorizeUint(c.A)

	return polorizer, nil
}

func (c *customNullable) Depolorize(depolorizer *Depolorizer) error {
	customDecodes++

	if depolorizer.IsNull() {
		return nil
	}

	value, err := depolorizer.DepolorizeUint64()
	c.A = value

	return err
}

// customStrict has a custom decoder that rejects a WireNull with ErrNullValue
type customStrict struct{ A bool }

func (c *customStrict) Depolorize(depolorizer *Depolorizer) error {
	value, err := depolorizer.DepolorizeBool()
	c.A = value

	return err
}

func TestN1_NullIsNil(t *testing.T) {
	type Object struct {
		A *customNullable
		B *uint64
		C []uint64
		D map[string]uint64
		E *big.Int
		F Document
		G Any
		H Raw
		I []byte
	}

	t.Run("pointer to custom type", func(t *testing.T) {
		customDecodes = 0
		object := &customNullable{A: 5}

		// A WireNull is a nil pointer, the custom decoder is not called (go-pisa #130)
		require.NoError(t, Depolorize(&object, []byte{0}))
		require.Nil(t, object)
		require.Zero(t, customDecodes)

		// A WireNull struct field of a pointer to a custom type
		var structure struct{ A, B *customNullable }

		wire := packWire(func(p *Polorizer) {
			p.PolorizeNull()
			p.PolorizeUint(7)
		})

		require.NoError(t, Depolorize(&structure, wire))
		require.Nil(t, structure.A)
		require.Equal(t, &customNullable{A: 7}, structure.B)
		require.Equal(t, 1, customDecodes)

		// A nil pointer of a custom type roundtrips as nil
		requireRoundtrip(t, wire, struct{ A, B *customNullable }{nil, &customNullable{7}})
		requireRoundtrip(t, packWire(func(p *Polorizer) {
			p.PolorizeNull()
			p.PolorizeUint(3)
		}), []*customNullable{nil, {3}})
	})

	t.Run("custom value type decides", func(t *testing.T) {
		customDecodes = 0

		// A custom decoder of a value type receives the WireNull
		var object customNullable

		require.NoError(t, Depolorize(&object, []byte{0}))
		require.Equal(t, customNullable{}, object)
		require.Equal(t, 1, customDecodes)

		var strict customStrict

		require.ErrorIs(t, Depolorize(&strict, []byte{0}), ErrNullValue)
	})

	t.Run("struct fields", func(t *testing.T) {
		wire := packWire(func(p *Polorizer) {
			for range 9 {
				p.PolorizeNull()
			}
		})

		object := Object{
			A: &customNullable{1}, B: new(uint64), C: []uint64{}, D: map[string]uint64{}, E: big.NewInt(1),
			F: Document{}, G: Any{0}, H: Raw{0}, I: []byte{},
		}

		require.NoError(t, Depolorize(&object, wire))
		require.Equal(t, Object{}, object)
	})

	t.Run("map values", func(t *testing.T) {
		wire := packWire(func(p *Polorizer) {
			p.PolorizeString("a")
			p.PolorizeNull()
		})

		requireRoundtrip(t, wire, map[string]*uint64{"a": nil})
		requireRoundtrip(t, wire, map[string][]string{"a": nil})
	})

	t.Run("document string maps", func(t *testing.T) {
		// A WireNull value is kept as a nil entry (it used to be dropped)
		wire := Document{"a": Raw{0}}.Bytes()

		requireRoundtrip(t, wire, map[string]*uint64{"a": nil}, DocStringMaps())
	})
}

func TestN2_NullIntoValue(t *testing.T) {
	type Inner struct{ A uint64 }

	tests := []struct {
		name   string
		wire   []byte
		target any
	}{
		{
			// GO-5: the slice used to be left untouched without an error
			"slice element",
			packWire(func(p *Polorizer) {
				p.PolorizeUint(1)
				p.PolorizeNull()
				p.PolorizeUint(3)
			}),
			&[]uint64{42},
		},
		{
			// The map entry used to be dropped without an error
			"map value",
			packWire(func(p *Polorizer) {
				p.PolorizeString("a")
				p.PolorizeNull()
			}),
			&map[string]uint64{"x": 1},
		},
		{
			"map key",
			packWire(func(p *Polorizer) {
				p.PolorizeNull()
				p.PolorizeUint(1)
			}),
			&map[string]uint64{"x": 1},
		},
		{
			"array element",
			packWire(func(p *Polorizer) {
				p.PolorizeUint(1)
				p.PolorizeNull()
			}),
			&[2]uint64{4, 2},
		},
		{
			"struct field of struct type",
			packWire(func(p *Polorizer) { p.PolorizeNull() }),
			&struct{ In Inner }{Inner{5}},
		},
		{
			"string struct field",
			packWire(func(p *Polorizer) { p.PolorizeNull() }),
			&struct{ S string }{"x"},
		},
		{
			"big.Int struct field",
			packWire(func(p *Polorizer) { p.PolorizeNull() }),
			&struct{ B big.Int }{},
		},
		{
			"byte array element",
			packWire(func(p *Polorizer) { p.PolorizeNull() }),
			&[][32]byte{{1}},
		},
		{
			"pointer to struct with a null field",
			packWire(func(p *Polorizer) { p.PolorizeNull() }),
			new(*Inner),
		},
		{
			"document struct field",
			Document{"A": Raw{0}}.Bytes(),
			&Inner{5},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := reflect.ValueOf(test.target).Elem().Interface()

			err := Depolorize(test.target, test.wire, DocStructs())
			require.ErrorIs(t, err, ErrNullValue)

			var wireErr IncompatibleWireError

			require.ErrorAs(t, err, &wireErr)
			require.ErrorIs(t, wireErr, ErrNullValue)

			// The target is not modified
			require.Equal(t, before, reflect.ValueOf(test.target).Elem().Interface())
		})
	}
}

func TestN3_TypedMethods(t *testing.T) {
	// newNull returns a Depolorizer with a single WireNull element
	newNull := func(t *testing.T) *Depolorizer {
		t.Helper()

		depolorizer, err := NewDepolorizer([]byte{0})
		require.NoError(t, err)

		return depolorizer
	}

	t.Run("nillable", func(t *testing.T) {
		nillable := map[string]func(*Depolorizer) (any, error){
			"Bytes":    func(d *Depolorizer) (any, error) { return d.DepolorizeBytes() },
			"BigInt":   func(d *Depolorizer) (any, error) { return d.DepolorizeBigInt() },
			"Document": func(d *Depolorizer) (any, error) { return d.DepolorizeDocument() },
			"Any":      func(d *Depolorizer) (any, error) { return d.DepolorizeAny() },
			"Raw":      func(d *Depolorizer) (any, error) { return d.DepolorizeRaw() },
			"Packed":   func(d *Depolorizer) (any, error) { return d.DepolorizePacked() },
		}

		for name, method := range nillable {
			t.Run(name, func(t *testing.T) {
				depolorizer := newNull(t)

				value, err := method(depolorizer)
				require.NoError(t, err)
				require.True(t, reflect.ValueOf(value).IsNil())
				require.True(t, depolorizer.Done())
			})
		}
	})

	t.Run("not nillable", func(t *testing.T) {
		methods := map[string]func(*Depolorizer) (any, error){
			"Bool":    func(d *Depolorizer) (any, error) { return d.DepolorizeBool() },
			"String":  func(d *Depolorizer) (any, error) { return d.DepolorizeString() },
			"Uint":    func(d *Depolorizer) (any, error) { return d.DepolorizeUint() },
			"Uint64":  func(d *Depolorizer) (any, error) { return d.DepolorizeUint64() },
			"Uint32":  func(d *Depolorizer) (any, error) { return d.DepolorizeUint32() },
			"Uint16":  func(d *Depolorizer) (any, error) { return d.DepolorizeUint16() },
			"Uint8":   func(d *Depolorizer) (any, error) { return d.DepolorizeUint8() },
			"Int":     func(d *Depolorizer) (any, error) { return d.DepolorizeInt() },
			"Int64":   func(d *Depolorizer) (any, error) { return d.DepolorizeInt64() },
			"Int32":   func(d *Depolorizer) (any, error) { return d.DepolorizeInt32() },
			"Int16":   func(d *Depolorizer) (any, error) { return d.DepolorizeInt16() },
			"Int8":    func(d *Depolorizer) (any, error) { return d.DepolorizeInt8() },
			"Float32": func(d *Depolorizer) (any, error) { return d.DepolorizeFloat32() },
			"Float64": func(d *Depolorizer) (any, error) { return d.DepolorizeFloat64() },
			"Bytes32": func(d *Depolorizer) (any, error) { return d.DepolorizeBytes32() },
		}

		for name, method := range methods {
			t.Run(name, func(t *testing.T) {
				value, err := method(newNull(t))
				require.ErrorIs(t, err, ErrNullValue)
				require.True(t, reflect.ValueOf(value).IsZero())
			})
		}
	})

	t.Run("nil depolorizer", func(t *testing.T) {
		var depolorizer *Depolorizer

		require.True(t, depolorizer.Done())
		require.False(t, depolorizer.IsNull())
		require.ErrorIs(t, depolorizer.DepolorizeNull(), ErrInsufficientWire)

		_, err := depolorizer.DepolorizeUint64()
		require.ErrorIs(t, err, ErrInsufficientWire)

		_, err = depolorizer.Unpacked()
		require.ErrorIs(t, err, ErrInsufficientWire)

		var object struct{ A uint64 }

		require.ErrorIs(t, depolorizer.Depolorize(&object), ErrInsufficientWire)
	})

	t.Run("IsNull", func(t *testing.T) {
		// IsNull is false once the element of an atomic Depolorizer is consumed
		depolorizer := newNull(t)
		require.True(t, depolorizer.IsNull())
		require.NoError(t, depolorizer.DepolorizeNull())
		require.False(t, depolorizer.IsNull())
	})

	t.Run("interface", func(t *testing.T) {
		// An interface can only be decoded from a WireNull
		var object any = 5

		require.NoError(t, Depolorize(&object, []byte{0}))
		require.Nil(t, object)

		err := Depolorize(&object, []byte{3, 5})
		require.EqualError(t, err, "incompatible value error: unsupported type: interface {} [interface]")

		_, err = (&Depolorizer{done: true}).depolorizeValue(reflect.TypeFor[any]())
		require.EqualError(t, err, "incompatible value error: unsupported type: interface {} [interface]")
	})
}

// TestCanonical_Rejected checks every non-canonical wire that v0.4.10 accepted (and more)
func TestCanonical_Rejected(t *testing.T) {
	type Pair struct {
		A uint64
		B string
	}

	nan := math.NaN()

	tests := []struct {
		name   string
		wire   []byte
		target any
		err    string
	}{
		// C1: minimal varints
		{"overlong top-level tag", hexWire(t, "830005"), new(uint64), "malformed tag: varint is not minimally encoded"},
		{"overlong top-level tag (any)", hexWire(t, "830005"), new(Any), "malformed tag: varint is not minimally encoded"},
		{
			"overlong head tag", hexWire(t, "0e3f830003050607"), new([]uint64),
			"malformed tag: varint is not minimally encoded",
		},
		{
			"overlong later head tag", hexWire(t, "0e3f0393000506"), new([]uint64),
			"malformed tag: varint is not minimally encoded",
		},
		{
			"overlong load tag", hexWire(t, "0eaf800003130506"), new([]uint64),
			"load convert fail: malformed tag: varint is not minimally encoded",
		},
		// C2: the offset of a top-level tag is zero
		{"top-level offset", hexWire(t, "1305"), new(uint64), "malformed tag: non-zero offset in top-level tag"},
		{"top-level offset (any)", hexWire(t, "7305"), new(Any), "malformed tag: non-zero offset in top-level tag"},
		{
			"top-level offset (document value)", Document{"A": Raw{0x13, 5}}.Bytes(), new(struct{ A uint64 }),
			"malformed tag: non-zero offset in top-level tag",
		},
		// C3: minimal integers
		{"leading zero (uint64)", hexWire(t, "030005"), new(uint64), "incompatible wire: leading zero byte in integer"},
		{"leading zero (uint8)", hexWire(t, "0300"), new(uint8), "incompatible wire: leading zero byte in integer"},
		{"leading zero (int64)", hexWire(t, "040005"), new(int64), "incompatible wire: leading zero byte in integer"},
		{"leading zero (positive int8)", hexWire(t, "0300"), new(int8), "incompatible wire: leading zero byte in integer"},
		{"negative zero", hexWire(t, "04"), new(int64), "incompatible wire: negative zero integer"},
		{"negative zero (int8)", hexWire(t, "04"), new(int8), "incompatible wire: negative zero integer"},
		{"leading zero (big.Int)", hexWire(t, "0300ff"), new(big.Int), "incompatible wire: leading zero byte in integer"},
		{
			"leading zero (negative big.Int)", hexWire(t, "0400ff"), new(*big.Int),
			"incompatible wire: leading zero byte in integer",
		},
		{"negative zero (big.Int)", hexWire(t, "04"), new(*big.Int), "incompatible wire: negative zero integer"},
		// C4: empty payloads
		{"true with data", hexWire(t, "02ff"), new(bool), "incompatible wire: unexpected data for wiretype 'true'"},
		{"false with data", hexWire(t, "01ff"), new(bool), "incompatible wire: unexpected data for wiretype 'false'"},
		{"null with data", hexWire(t, "00ff"), new(*uint64), "incompatible wire: unexpected data for wiretype 'null'"},
		{"null with data (any)", hexWire(t, "00ff"), new(Any), "incompatible wire: unexpected data for wiretype 'null'"},
		{
			"null element with data", hexWire(t, "0e2f001301"), new([]*uint64),
			"incompatible wire: unexpected data for wiretype 'null'",
		},
		// C5: exact lengths
		{"empty word as hash", hexWire(t, "06"), new([32]byte), "incompatible wire: mismatched data length for byte array"},
		{
			"excess array elements", hexWire(t, "0e3f030303"), new([2]uint64),
			"incompatible wire: excess elements for array [2]uint64",
		},
		{"missing array element", hexWire(t, "0e1f03"), new([2]uint64), "insufficient data in wire for decode"},
		{"short float", hexWire(t, "07000000"), new(float32), "incompatible wire: malformed data for 32-bit float"},
		// C6: pack framing
		{
			"first offset not zero", hexWire(t, "0e2f13230005"), new([]uint64),
			"malformed tag: non-zero offset for the first element",
		},
		{"data in empty pack", hexWire(t, "0e0f05"), new([]uint64), "malformed tag: data in a pack without elements"},
		{"data in empty document", hexWire(t, "0d0f05"), new(Document), "malformed tag: data in a pack without elements"},
		{
			"extra struct element", hexWire(t, "0e3f031626054142"), new(Pair),
			"incompatible wire: excess elements for struct polo.Pair",
		},
		{
			"missing struct element", hexWire(t, "0e1f0305"), new(Pair),
			"incompatible wire: struct field [polo.Pair.B <string>]: insufficient data in wire for decode",
		},
		{
			"packed bytes without option", hexWire(t, "0e1f0305"), new([]byte),
			"incompatible wire: unexpected wiretype 'pack'. expected one of: {null, word}",
		},
		// C7: sorted, unique keys
		{
			"unsorted map keys", hexWire(t, "0e4f0613263362026101"), new(map[string]uint64),
			"incompatible wire: map keys are not in strictly ascending order",
		},
		{
			"duplicate map keys", hexWire(t, "0e4f0613263361016102"), new(map[string]uint64),
			"incompatible wire: map keys are not in strictly ascending order",
		},
		{
			"duplicate array map keys", packWire(func(p *Polorizer) {
				p.PolorizeBytes([]byte{1, 2})
				p.PolorizeBool(true)
				p.PolorizeBytes([]byte{1, 2})
				p.PolorizeBool(false)
			}), new(map[[2]byte]bool),
			"incompatible wire: map keys are not in strictly ascending order",
		},
		{
			"unsorted integer map keys", packWire(func(p *Polorizer) {
				p.PolorizeInt(1)
				p.PolorizeBool(true)
				p.PolorizeInt(-1)
				p.PolorizeBool(false)
			}), new(map[int8]bool),
			"incompatible wire: map keys are not in strictly ascending order",
		},
		{
			"negative and positive zero map keys", packWire(func(p *Polorizer) {
				p.PolorizeFloat64(math.Copysign(0, -1))
				p.PolorizeBool(true)
				p.PolorizeFloat64(0)
				p.PolorizeBool(false)
			}), new(map[float64]bool),
			"incompatible wire: map keys are not in strictly ascending order",
		},
		{
			"NaN map key", packWire(func(p *Polorizer) {
				p.PolorizeFloat64(nan)
				p.PolorizeBool(true)
			}), new(map[float64]bool),
			"incompatible wire: NaN map key",
		},
		{
			"NaN in an array map key", packWire(func(p *Polorizer) {
				array := NewPolorizer()
				array.PolorizeFloat32(1)
				array.PolorizeFloat32(float32(nan))
				p.PolorizePacked(array)
				p.PolorizeBool(true)
			}), new(map[[2]float32]bool),
			"incompatible wire: NaN map key",
		},
		{
			"unsorted document keys", docWire("b", "a"), new(Document),
			"incompatible wire: document keys are not in strictly ascending order",
		},
		{
			"duplicate document keys", docWire("a", "a"), new(Document),
			"incompatible wire: document keys are not in strictly ascending order",
		},
		{
			"duplicate empty document keys", docWire("", ""), new(Document),
			"incompatible wire: document keys are not in strictly ascending order",
		},
		{
			"unsorted document struct keys", docWire("B", "A"), new(struct{ A, B uint64 }),
			"incompatible wire: document keys are not in strictly ascending order",
		},
		{
			"null document value", hexWire(t, "0d1f06"+"61"), new(Document),
			"insufficient data in wire for decode",
		},
		{
			"null as document value", packWireOf(t, WireDoc, func(p *Polorizer) {
				p.PolorizeString("a")
				p.PolorizeNull()
			}), new(Document),
			"incompatible wire: unexpected wiretype 'null'. expected one of: {raw}",
		},
		{
			"null as document key", packWireOf(t, WireDoc, func(p *Polorizer) {
				p.PolorizeNull()
				p.PolorizeRaw(Raw{0})
			}), new(Document),
			"incompatible wire: nil value",
		},
		// Packed bytes: every byte is a WirePosInt with exactly one byte (as the encoder writes it)
		{"packed byte without data", hexWire(t, "0e1f03"), new([]byte), "incompatible wire: missing data for packed byte"},
		{"packed byte as null", hexWire(t, "0e1f00"), new([]byte), "incompatible wire: nil value"},
		{
			"packed byte with excess data", hexWire(t, "0e1f030102"), new([]byte),
			"incompatible value error: excess data for 8-bit integer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := []EncodingOptions{DocStructs()}
			if test.name == "packed byte without data" || test.name == "packed byte as null" ||
				test.name == "packed byte with excess data" {
				options = append(options, PackedBytes())
			}

			err := Depolorize(test.target, test.wire, options...)
			require.EqualError(t, err, test.err)
		})
	}

	t.Run("packed bytes with zero bytes", func(t *testing.T) {
		// The encoder writes a zero byte as a WirePosInt with one zero byte
		wire, err := Polorize([]byte{0, 1, 0}, PackedBytes())
		require.NoError(t, err)
		require.Equal(t, hexWire(t, "0e3f0313230001"+"00"), wire)

		requireRoundtrip(t, wire, []byte{0, 1, 0}, PackedBytes())
	})
}

// packWireOf returns a compound wire of the given wire type with the elements written by the given function
func packWireOf(t *testing.T, wire WireType, write func(*Polorizer)) []byte {
	t.Helper()

	polorizer := NewPolorizer()
	write(polorizer)

	var wb writebuffer

	wb.write(wire, polorizer.wb.load())

	return wb.bytes()
}

// TestCanonical_Accepted checks wires that are canonical and decode (the edge cases of the rules above)
func TestCanonical_Accepted(t *testing.T) {
	requireRoundtrip(t, hexWire(t, "03"), uint64(0))
	requireRoundtrip(t, hexWire(t, "03"), int8(0))
	requireRoundtrip(t, hexWire(t, "03ff"), uint8(math.MaxUint8))
	requireRoundtrip(t, hexWire(t, "03ffffffffffffffff"), uint64(math.MaxUint64))
	requireRoundtrip(t, hexWire(t, "037fffffffffffffff"), int64(math.MaxInt64))
	requireRoundtrip(t, hexWire(t, "03"), *big.NewInt(0))
	requireRoundtrip(t, hexWire(t, "0401"), big.NewInt(-1))
	requireRoundtrip(t, hexWire(t, "0780000000"), float32(math.Copysign(0, -1)))
	requireRoundtrip(t, hexWire(t, "06"), [0]byte{})
	requireRoundtrip(t, hexWire(t, "06"), []byte{})
	requireRoundtrip(t, hexWire(t, "06"), "")
	requireRoundtrip(t, hexWire(t, "0e0f"), []uint64{})
	requireRoundtrip(t, hexWire(t, "0e0f"), map[string]uint64{})
	requireRoundtrip(t, hexWire(t, "0e0f"), struct{}{})
	requireRoundtrip(t, hexWire(t, "0d0f"), Document{})
	requireRoundtrip(t, hexWire(t, "0e2f0303"), []uint64{0, 0})
	requireRoundtrip(t, hexWire(t, "0e2f0000"), []*uint64{nil, nil})
	requireRoundtrip(t, docWire("", "a", "b"), Document{"": Raw{3, 1}, "a": Raw{3, 1}, "b": Raw{3, 1}})
	requireRoundtrip(t, Document{"A": Raw{3, 1}}.Bytes(), struct{ A uint64 }{1}, DocStructs())
	requireRoundtrip(t, Document{"a": Raw{3, 1}, "b": Raw{3, 2}}.Bytes(),
		map[string]uint64{"a": 1, "b": 2}, DocStringMaps())

	// An Any holds any element as is (only its tag must be canonical)
	requireRoundtrip(t, hexWire(t, "0e2f13230005"), Any(hexWire(t, "0e2f13230005")))
	requireRoundtrip(t, hexWire(t, "08ff"), Any(hexWire(t, "08ff")))
	requireRoundtrip(t, hexWire(t, "00"), Any(nil))

	// Map keys in the order of the encoder
	wire, err := Polorize(map[int16]string{-300: "a", -1: "b", 0: "c", 1: "d", 300: "e"})
	require.NoError(t, err)
	requireRoundtrip(t, wire, map[int16]string{-300: "a", -1: "b", 0: "c", 1: "d", 300: "e"})

	floats := map[float64]bool{math.Inf(-1): true, -1: false, math.Copysign(0, -1): true, math.Inf(1): false}

	wire, err = Polorize(floats)
	require.NoError(t, err)
	requireRoundtrip(t, wire, floats)

	wire, err = Polorize(map[[2]bool]uint8{{false, false}: 1, {false, true}: 2, {true, false}: 3})
	require.NoError(t, err)
	requireRoundtrip(t, wire, map[[2]bool]uint8{{false, false}: 1, {false, true}: 2, {true, false}: 3})
}

func TestC7_NaNMapKeyEncode(t *testing.T) {
	nan := math.NaN()

	for name, object := range map[string]any{
		"float64 key":     map[float64]uint64{nan: 1},
		"float32 key":     map[float32]uint64{1: 2, float32(nan): 1},
		"two NaN keys":    map[float64]uint64{nan: 1, math.NaN(): 2},
		"array key":       map[[2]float64]uint64{{1, nan}: 1},
		"nested map":      []map[float64]bool{{nan: true}},
		"struct with map": struct{ M map[float64]bool }{map[float64]bool{nan: true}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Polorize(object)
			require.EqualError(t, err, "incompatible value error: unsupported map key: NaN")
		})
	}
}

// utf8Vectors are Word bodies and whether they are valid UTF-8 (go-polo-bench probe, 2026-10-26)
var utf8Vectors = []struct {
	body  string
	valid bool
}{
	{"", true},
	{"61", true},
	{"00", true},       // NUL
	{"efbbbf", true},   // BOM
	{"efbfbd", true},   // U+FFFD
	{"f09f9880", true}, // emoji
	{"f48fbfbf", true}, // U+10FFFF
	{"ff", false},
	{"80", false}, // lone continuation byte
	{"c3", false}, // truncated
	{"61ff62", false},
	{"c080", false},   // overlong
	{"e080af", false}, // overlong
	{"f0808080", false},
	{"eda080", false}, // surrogate
	{"edbfbf", false}, // surrogate
	{"f4908080", false},
	{"f5808080", false},
	{"f888808080", false}, // 5-byte form
}

func TestU1_DecodeUTF8(t *testing.T) {
	type Name string

	for _, vector := range utf8Vectors {
		t.Run(vector.body, func(t *testing.T) {
			body := hexWire(t, vector.body)
			word := append([]byte{byte(WireWord)}, body...)

			// String targets require valid UTF-8, at every position
			stringTargets := map[string]struct {
				wire   []byte
				target any
			}{
				"string":         {word, new(string)},
				"named string":   {word, new(Name)},
				"string pointer": {word, new(*string)},
				"slice element":  {packWire(func(p *Polorizer) { p.PolorizeBytes(body) }), new([]string)},
				"struct field":   {packWire(func(p *Polorizer) { p.PolorizeBytes(body) }), new(struct{ S string })},
				"map key": {packWire(func(p *Polorizer) {
					p.PolorizeBytes(body)
					p.PolorizeUint(1)
				}), new(map[string]uint64)},
				"named map key": {packWire(func(p *Polorizer) {
					p.PolorizeBytes(body)
					p.PolorizeUint(1)
				}), new(map[Name]uint64)},
				"map value": {packWire(func(p *Polorizer) {
					p.PolorizeUint(1)
					p.PolorizeBytes(body)
				}), new(map[uint64]string)},
				"document key": {packWireOf(t, WireDoc, func(p *Polorizer) {
					p.PolorizeBytes(body)
					p.PolorizeRaw(Raw{3, 1})
				}), new(Document)},
			}

			for name, target := range stringTargets {
				err := Depolorize(target.target, target.wire)
				if vector.valid {
					require.NoError(t, err, name)
				} else {
					require.ErrorIs(t, err, ErrInvalidUTF8, name)
					require.ErrorContains(t, err, "incompatible wire: invalid UTF-8 string", name)
				}
			}

			depolorizer, err := NewDepolorizer(word)
			require.NoError(t, err)

			_, err = depolorizer.DepolorizeString()
			require.Equal(t, vector.valid, err == nil)

			// Bytes targets accept any data
			requireRoundtrip(t, word, body)
			requireRoundtrip(t, word, Any(word))

			array := reflect.New(reflect.ArrayOf(len(body), byteType))
			require.NoError(t, Depolorize(array.Interface(), word))
			require.Equal(t, body, array.Elem().Bytes()[:len(body):len(body)])

			raw := append([]byte{byte(WireRaw)}, word...)
			requireRoundtrip(t, raw, Raw(word))

			depolorizer, err = NewDepolorizer(word)
			require.NoError(t, err)

			decoded, err := depolorizer.DepolorizeBytes()
			require.NoError(t, err)
			require.True(t, bytes.Equal(body, decoded))
		})
	}
}

func TestU2_EncodeUTF8(t *testing.T) {
	type Name string

	type Tagged struct {
		A uint64 `polo:"\xff"`
	}

	invalid := "a\xffb"

	for name, object := range map[string]any{
		"string":           invalid,
		"named string":     Name(invalid),
		"string pointer":   &invalid,
		"slice element":    []string{"a", invalid},
		"struct field":     struct{ S string }{invalid},
		"map key":          map[string]uint64{invalid: 1},
		"map value":        map[uint64]string{1: invalid},
		"document key":     Document{invalid: Raw{3, 1}},
		"document in data": []Document{{"a": Raw{0}}, {invalid: Raw{0}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Polorize(object)
			require.ErrorIs(t, err, ErrInvalidUTF8)
			require.EqualError(t, err, "incompatible value error: invalid UTF-8 string")
		})
	}

	t.Run("document encoding", func(t *testing.T) {
		_, err := Polorize(map[string]uint64{invalid: 1}, DocStringMaps())
		require.ErrorIs(t, err, ErrInvalidUTF8)

		_, err = Polorize(struct{ S string }{invalid}, DocStructs())
		require.ErrorIs(t, err, ErrInvalidUTF8)

		_, err = Polorize(Tagged{1}, DocStructs())
		require.ErrorIs(t, err, ErrInvalidUTF8)

		_, err = PolorizeDocument(map[string]uint64{invalid: 1})
		require.ErrorIs(t, err, ErrInvalidUTF8)
	})

	t.Run("valid strings", func(t *testing.T) {
		for _, vector := range utf8Vectors {
			if vector.valid {
				requireRoundtrip(t, append([]byte{byte(WireWord)}, hexWire(t, vector.body)...), string(hexWire(t, vector.body)))
			}
		}
	})

	t.Run("bytes", func(t *testing.T) {
		// The same data as bytes encodes and decodes (the wire is the same as for the string)
		wire, err := Polorize([]byte(invalid))
		require.NoError(t, err)
		require.Equal(t, append([]byte{byte(WireWord)}, invalid...), wire)
		requireRoundtrip(t, wire, []byte(invalid))
	})

	t.Run("PolorizeString", func(t *testing.T) {
		// PolorizeString does not check, so the caller must pass valid UTF-8
		polorizer := NewPolorizer()
		polorizer.PolorizeString(invalid)

		var object string

		require.ErrorIs(t, Depolorize(&object, polorizer.Bytes()), ErrInvalidUTF8)
	})
}

func TestErrors(t *testing.T) {
	// The sentinel errors are matched through struct fields at every depth
	type Inner struct{ S string }

	type Outer struct{ In Inner }

	wire, err := Polorize(Outer{Inner{"x"}})
	require.NoError(t, err)

	invalid := bytes.Replace(wire, []byte("x"), []byte{0xff}, 1)

	var object Outer

	err = Depolorize(&object, invalid)
	require.ErrorIs(t, err, ErrInvalidUTF8)
	require.False(t, errors.Is(err, ErrNullValue))

	var wireErr IncompatibleWireError

	require.ErrorAs(t, err, &wireErr)
	require.Equal(t, err.Error(), wireErr.Error())
	require.ErrorIs(t, wireErr, ErrInvalidUTF8)

	// A struct field error is not retrievable as an unrelated error type
	var valueErr IncompatibleValueError

	require.False(t, errors.As(err, &valueErr))
}

type (
	canonicalName  string
	canonicalBytes []byte
	canonicalHash  [4]byte
	canonicalInt   int16
)

// canonicalObject covers the kinds of values that the reflective codec supports. Custom
// Depolorizable types and Raw values (a nil Raw is written as Raw{0}) are not covered.
type canonicalObject struct {
	B    bool
	S    string
	N    canonicalName
	U8   uint8
	U16  uint16
	U32  uint32
	U    uint64
	I8   int8
	I16  canonicalInt
	I32  int32
	I    int64
	F32  float32
	F64  float64
	Big  big.Int
	PBig *big.Int
	Ptr  *canonicalObject
	Bs   []byte
	Nb   canonicalBytes
	H    [32]byte
	Nh   canonicalHash
	Any  Any
	Doc  Document
	Us   []uint64
	Ss   []string
	Arr  [2]int16
	Ms   map[string]uint64
	Mh   map[[2]byte]bool
	Mf   map[float64]int8
	Mb   map[bool]string
	Mp   map[string]*uint64
	In   struct{ A, B string }
	Emp  struct{}
	Skip uint64 `polo:"-"`
}

// canonicalTargets are the types that FuzzCanonical decodes every input into
var canonicalTargets = []reflect.Type{
	reflect.TypeFor[bool](), reflect.TypeFor[string](), reflect.TypeFor[canonicalName](),
	reflect.TypeFor[uint8](), reflect.TypeFor[uint16](), reflect.TypeFor[uint32](), reflect.TypeFor[uint64](),
	reflect.TypeFor[uint](), reflect.TypeFor[int8](), reflect.TypeFor[int16](), reflect.TypeFor[int32](),
	reflect.TypeFor[int64](), reflect.TypeFor[int](), reflect.TypeFor[float32](), reflect.TypeFor[float64](),
	reflect.TypeFor[big.Int](), reflect.TypeFor[*big.Int](), reflect.TypeFor[[]byte](), reflect.TypeFor[canonicalBytes](),
	reflect.TypeFor[[32]byte](), reflect.TypeFor[[0]byte](), reflect.TypeFor[Any](), reflect.TypeFor[Document](),
	reflect.TypeFor[[]uint64](), reflect.TypeFor[[]string](), reflect.TypeFor[[2]int16](), reflect.TypeFor[[][]byte](),
	reflect.TypeFor[map[string]uint64](), reflect.TypeFor[map[int16]string](), reflect.TypeFor[map[[2]byte]bool](),
	reflect.TypeFor[map[float32]uint8](), reflect.TypeFor[map[bool]bool](), reflect.TypeFor[*uint64](),
	reflect.TypeFor[**string](), reflect.TypeFor[[]*uint64](), reflect.TypeFor[struct{}](),
	reflect.TypeFor[canonicalObject](), reflect.TypeFor[*canonicalObject](), reflect.TypeFor[[]canonicalObject](),
	reflect.TypeFor[map[string]canonicalObject](),
}

// FuzzCanonical checks that the decoder accepts only canonical wires: every wire that decodes into one
// of the canonicalTargets (without a panic) must encode back into exactly the same bytes. The encoder
// rejects strings that are not valid UTF-8, so this also checks that every decoded string is valid UTF-8.
//
// Run it with: go test -run '^$' -fuzz FuzzCanonical -fuzztime 30m
func FuzzCanonical(f *testing.F) {
	nan32 := math.Float32frombits(0x7f800001)
	value := uint64(9)

	seeds := []any{
		true, "orange", uint8(0), uint64(300), int8(-128), int64(math.MinInt64), float32(1.5), math.NaN(),
		nan32, big.NewInt(-1 << 40),
		[]byte{1, 2},
		[32]byte{1},
		Any{3, 5},
		Document{"a": Raw{3, 1}},
		[]uint64{1, 2, 3},
		[]string{"a", ""},
		[2]int16{-1, 1},
		map[string]uint64{"a": 1, "b": 2},
		map[int16]string{-5: "x", 5: "y"},
		map[[2]byte]bool{{1, 2}: true},
		map[float32]uint8{-1: 1, 2: 2},
		&value,
		[]*uint64{nil, &value},
		canonicalObject{
			S: "s", U: 7, F32: nan32, Big: *big.NewInt(9), PBig: big.NewInt(-9), Ptr: &canonicalObject{B: true},
			Bs: []byte{}, Any: Any{0x0e, 0x0f}, Doc: Document{}, Us: []uint64{}, Ms: map[string]uint64{"a": 1},
			Mh: map[[2]byte]bool{{9, 9}: false}, Mf: map[float64]int8{-0.5: 1}, Mb: map[bool]string{true: "t"},
			Mp: map[string]*uint64{"n": nil, "v": &value},
		},
	}

	for _, seed := range seeds {
		wire, err := Polorize(seed)
		require.NoError(f, err)
		f.Add(wire)
	}

	for _, seed := range []string{
		"830005", "030005", "1305", "04", "040005", "02ff", "01ff", "00ff", "06", "0e2f13230005",
		"0e3f830003050607", "0eaf800003130506", "0e3f031626054142", "0e4f0613263362026101",
		"0e4f0613263361016102", "048000000000000000", "0380", "077ff8000000000001", "06ff", "06c080",
	} {
		f.Add(hexWire(f, seed))
	}

	f.Fuzz(func(t *testing.T, wire []byte) {
		for _, target := range canonicalTargets {
			object := reflect.New(target)
			if err := Depolorize(object.Interface(), wire, MaxDepth(64)); err != nil {
				continue
			}

			encoded, err := Polorize(object.Elem().Interface())
			if err != nil {
				t.Fatalf("%v: %x decodes but does not encode: %v", target, wire, err)
			}

			if !bytes.Equal(wire, encoded) {
				t.Fatalf("%v: %x decodes but encodes as %x", target, wire, encoded)
			}
		}
	})
}
