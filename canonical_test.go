package polo

// Tests for the decoding rules of v0.5.0: integer bounds and NaN (GO-1, GO-6, GO-10), Null semantics (N1-N3),
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

// canonicalDoc and canonicalDocPtr are DocumentStructs for the canonical tests
type canonicalDoc struct{ A uint64 }

func (canonicalDoc) PoloDocument() {}

type canonicalDocPtr struct{ A *uint64 }

func (canonicalDocPtr) PoloDocument() {}

type canonicalDocAB struct{ A, B uint64 }

func (canonicalDocAB) PoloDocument() {}

// requireRoundtrip decodes the wire into a new value of the type of target and
// checks that the value is expected and that it encodes back into the same wire
func requireRoundtrip(t *testing.T, wire []byte, expected any) {
	t.Helper()

	object := reflect.New(reflect.TypeOf(expected))
	require.NoError(t, Depolorize(object.Interface(), wire))
	require.Equal(t, expected, object.Elem().Interface())

	encoded, err := Polorize(object.Elem().Interface())
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

	t.Run("document struct fields", func(t *testing.T) {
		// A WireNull value is a nil field, and the key stays in the document
		wire := Document{"A": Raw{0}}.Bytes()

		requireRoundtrip(t, wire, canonicalDocPtr{A: nil})
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
			&canonicalDoc{5},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := reflect.ValueOf(test.target).Elem().Interface()

			err := Depolorize(test.target, test.wire)
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

	t.Run("nil raw", func(t *testing.T) {
		// Polorize writes a nil Raw as a Null, and a Null decodes to a nil Raw
		wire, err := Polorize(Raw(nil))
		require.NoError(t, err)
		require.Equal(t, []byte{0}, wire)

		raw := Raw{1}

		require.NoError(t, Depolorize(&raw, wire))
		require.Nil(t, raw)

		// PolorizeRaw writes a nil Raw as a WireRaw that holds a Null, which is a different wire
		polorizer := NewPolorizer()
		polorizer.PolorizeRaw(nil)
		require.Equal(t, []byte{5, 0}, polorizer.Bytes())
	})

	t.Run("interface", func(t *testing.T) {
		// An interface is not a target, also not for a WireNull: the encoder cannot write a nil any,
		// so the decoder must not accept one (as in v0.4.9)
		unsupported := "incompatible value error: unsupported type: interface {} [interface]"

		var object any = 5

		require.EqualError(t, Depolorize(&object, []byte{0}), unsupported)
		require.EqualError(t, Depolorize(&object, []byte{3, 5}), unsupported)
		require.Equal(t, 5, object)

		var structure struct{ X any }

		require.ErrorContains(t, Depolorize(&structure, hexWire(t, "0e1f00")), unsupported)

		var list []any

		require.EqualError(t, Depolorize(&list, hexWire(t, "0e1f00")), unsupported)
		require.Nil(t, list)

		_, err := Polorize(structure)
		require.EqualError(t, err, unsupported)

		_, err = (&Depolorizer{done: true}).depolorizeValue(reflect.TypeFor[any]())
		require.EqualError(t, err, unsupported)
	})
}

// TestCanonical_AnyEncode checks that PolorizeAny rejects every Any that the decoder of an Any rejects,
// and that every Any that it accepts decodes back into the same bytes
func TestCanonical_AnyEncode(t *testing.T) {
	const reserved = "incompatible wire: unexpected wiretype 'reserved' for an undecoded element"

	rejected := []struct {
		name  string
		value Any
		err   string
	}{
		{"C1: tag is not minimal", Any{0x83, 0x00, 0x05}, "malformed tag: varint is not minimally encoded"},
		{"C2: tag has an offset", Any{0x13, 0x05}, "malformed tag: non-zero offset in top-level tag"},
		{"C4: null has data", Any{0x00, 0xff}, "incompatible wire: unexpected data for wiretype 'null'"},
		{"C4: false has data", Any{0x01, 0xff}, "incompatible wire: unexpected data for wiretype 'false'"},
		{"C4: true has data", Any{0x02, 0xff}, "incompatible wire: unexpected data for wiretype 'true'"},
		{"no tag", Any{}, "malformed tag: varint terminated prematurely"},
		// No target type can decode a reserved wire type or a WireLoad
		{"reserved (8)", Any{0x08}, reserved},
		{"reserved (9) with data", Any{0x09, 0x01, 0x02}, reserved},
		{"reserved (12) with data", Any{0x0c, 0x01}, reserved},
		{"load", Any{0x0f}, "incompatible wire: unexpected wiretype 'load' for an undecoded element"},
	}

	for _, test := range rejected {
		t.Run(test.name, func(t *testing.T) {
			// The decoder of an Any rejects the wire with the same error: at the top level,
			// with DepolorizeAny, as a struct field and as an element of a list
			var decoded Any

			require.EqualError(t, Depolorize(&decoded, test.value), test.err)
			require.Nil(t, decoded)

			// The same element in a pack. A tag in a pack is not the tag of a top-level
			// element, so this is only for the tests in which the data is not valid
			if len(test.value) > 0 && test.value[0] < 0x10 {
				wire := packWire(func(p *Polorizer) { p.wb.write(WireType(test.value[0]), test.value[1:]) })

				pack, err := NewDepolorizer(wire)
				require.NoError(t, err)

				pack, err = pack.DepolorizePacked()
				require.NoError(t, err)

				_, err = pack.DepolorizeAny()
				require.EqualError(t, err, test.err)

				var list []Any

				require.EqualError(t, Depolorize(&list, wire), test.err)
				require.Nil(t, list)
			}

			// The encoder rejects the Any: with PolorizeAny, at the top level and as a struct field
			require.EqualError(t, NewPolorizer().PolorizeAny(test.value), test.err)

			_, err := Polorize(test.value)
			require.EqualError(t, err, test.err)

			_, err = Polorize(struct{ X Any }{test.value})
			require.EqualError(t, err, test.err)
		})
	}

	// Any{0} is accepted and gives the same wire as a nil Any, which decodes to nil (N1)
	wire, err := Polorize(struct{ X Any }{Any{0}})
	require.NoError(t, err)
	requireRoundtrip(t, wire, struct{ X Any }{})

	// The data of the other wire types is not examined, because its rules depend on the target type:
	// 03 00 is a packed byte (PackedBytes) and 06 ff is bytes, but they are not an integer and a string
	accepted := []string{
		"01", "02", "0305", "030005", "0300", "04", "0500", "05ff", "0661", "06ff", "07", "0d0f", "0e2f0313",
	}

	for _, value := range accepted {
		t.Run(value, func(t *testing.T) {
			wire := hexWire(t, value)

			requireRoundtrip(t, wire, Any(wire))
			requireRoundtrip(t, packWire(func(p *Polorizer) {
				require.NoError(t, p.PolorizeAny(wire))
			}), struct{ X Any }{wire})
		})
	}
}

// TestPolorizableInterface checks that an interface with a Polorize method encodes the value that it holds,
// with the same wire as a field of the concrete type. A nil interface used to panic. An interface is
// not a target for the decoder, so the wire decodes only into the concrete type.
func TestPolorizableInterface(t *testing.T) {
	type Concrete struct{ X *customNullable }

	type Interface struct{ X Polorizable }

	tests := []struct {
		name     string
		value    Polorizable
		concrete Concrete
		wire     string
	}{
		{"nil", nil, Concrete{}, "0e1f00"},
		{"nil pointer", (*customNullable)(nil), Concrete{}, "0e1f00"},
		{"pointer", &customNullable{A: 7}, Concrete{&customNullable{A: 7}}, "0e1f0307"},
		{"value", customNullable{A: 7}, Concrete{&customNullable{A: 7}}, "0e1f0307"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := Polorize(Interface{test.value})
			require.NoError(t, err)
			require.Equal(t, hexWire(t, test.wire), wire)

			// The same wire as the concrete type, and it decodes into the concrete type
			requireRoundtrip(t, wire, test.concrete)

			// The decoder does not know which concrete type to create
			var object Interface

			require.ErrorContains(t, Depolorize(&object, wire), "unsupported type: polo.Polorizable [interface]")
		})
	}

	// Lists of interfaces encode as lists of the concrete type
	wire, err := Polorize([]Polorizable{nil, &customNullable{A: 3}})
	require.NoError(t, err)
	requireRoundtrip(t, wire, []*customNullable{nil, {A: 3}})

	// Interfaces without a Polorize method are not supported
	_, err = Polorize(struct{ X any }{&customNullable{A: 3}})
	require.EqualError(t, err, "incompatible value error: unsupported type: interface {} [interface]")
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
			"top-level offset (document value)", Document{"A": Raw{0x13, 5}}.Bytes(), new(canonicalDoc),
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
			"negative zero map key", packWire(func(p *Polorizer) {
				p.PolorizeFloat64(math.Copysign(0, -1))
				p.PolorizeBool(true)
			}), new(map[float64]bool),
			"incompatible wire: negative zero map key",
		},
		{
			"negative zero map key (float32)", packWire(func(p *Polorizer) {
				p.PolorizeFloat32(float32(math.Copysign(0, -1)))
				p.PolorizeBool(true)
			}), new(map[float32]bool),
			"incompatible wire: negative zero map key",
		},
		{
			"negative zero in an array map key", packWire(func(p *Polorizer) {
				array := NewPolorizer()
				array.PolorizeFloat64(1)
				array.PolorizeFloat64(math.Copysign(0, -1))
				p.PolorizePacked(array)
				p.PolorizeBool(true)
			}), new(map[[2]float64]bool),
			"incompatible wire: negative zero map key",
		},
		{
			"negative and positive zero map keys", packWire(func(p *Polorizer) {
				p.PolorizeFloat64(math.Copysign(0, -1))
				p.PolorizeBool(true)
				p.PolorizeFloat64(0)
				p.PolorizeBool(false)
			}), new(map[float64]bool),
			"incompatible wire: negative zero map key",
		},
		{
			"positive and negative zero map keys", packWire(func(p *Polorizer) {
				p.PolorizeFloat64(0)
				p.PolorizeBool(true)
				p.PolorizeFloat64(math.Copysign(0, -1))
				p.PolorizeBool(false)
			}), new(map[float64]bool),
			"incompatible wire: negative zero map key",
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
			"unsorted document struct keys", docWire("B", "A"), new(canonicalDocAB),
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
		// Bytes are a word. A pack of bytes is not a form of bytes
		{
			"pack for bytes", hexWire(t, "0e1f0301"), new([]byte),
			"incompatible wire: unexpected wiretype 'pack'. expected one of: {null, word}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.target, test.wire)
			require.EqualError(t, err, test.err)
		})
	}
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
	requireRoundtrip(t, Document{"A": Raw{3, 1}}.Bytes(), canonicalDoc{1})

	// An Any holds any element as is (only its tag must be canonical)
	requireRoundtrip(t, hexWire(t, "0e2f13230005"), Any(hexWire(t, "0e2f13230005")))
	requireRoundtrip(t, hexWire(t, "00"), Any(nil))

	// Map keys in the order of the encoder
	wire, err := Polorize(map[int16]string{-300: "a", -1: "b", 0: "c", 1: "d", 300: "e"})
	require.NoError(t, err)
	requireRoundtrip(t, wire, map[int16]string{-300: "a", -1: "b", 0: "c", 1: "d", 300: "e"})

	floats := map[float64]bool{math.Inf(-1): true, -1: false, 0: true, math.Inf(1): false}

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
		_, err := PolorizeDocument(struct{ S string }{invalid})
		require.ErrorIs(t, err, ErrInvalidUTF8)

		_, err = PolorizeDocument(Tagged{1})
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
		// PolorizeString has no error result. It writes the string and the Polorizer keeps the error
		polorizer := NewPolorizer()
		polorizer.PolorizeString(invalid)

		require.ErrorIs(t, polorizer.Err(), ErrInvalidUTF8)

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

// canonicalObject covers the kinds of values that the reflective codec supports.
// Custom Depolorizable types and Raw values are not covered.
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
	reflect.TypeFor[canonicalDoc](), reflect.TypeFor[canonicalDocObject](), reflect.TypeFor[*canonicalDocObject](),
	reflect.TypeFor[[]canonicalDocObject](), reflect.TypeFor[map[string]canonicalDocObject](),
}

// canonicalDocObject is a DocumentStruct with a field of each form: a nested DocumentStruct, a struct that
// is a pack, and a map and a slice of DocumentStructs
type canonicalDocObject struct {
	U   uint64                  `polo:"u"`
	S   string                  `polo:"s"`
	P   *uint64                 `polo:"p"`
	Doc canonicalDoc            `polo:"doc"`
	In  struct{ A uint64 }      `polo:"in"`
	Ms  map[string]canonicalDoc `polo:"ms"`
	Ls  []canonicalDocPtr       `polo:"ls"`
	Ptr *canonicalDocObject     `polo:"ptr"`
}

func (canonicalDocObject) PoloDocument() {}

// hasDocumentStruct returns whether a type is or holds a DocumentStruct
func hasDocumentStruct(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return hasDocumentStruct(t.Elem())
	case reflect.Map:
		return hasDocumentStruct(t.Key()) || hasDocumentStruct(t.Elem())
	case reflect.Struct:
		return getStructInfo(t).document
	default:
		return false
	}
}

// FuzzCanonical checks that the decoder accepts only canonical wires: every wire that decodes into one
// of the canonicalTargets (without a panic) must encode back into exactly the same bytes. The encoder
// rejects strings that are not valid UTF-8, so this also checks that every decoded string is valid UTF-8.
//
// A DocumentStruct gives the zero value to the field of an absent key, so a wire without a key encodes back
// with the key. For a target that is or holds a DocumentStruct, the check is that the encoded wire decodes
// into the same value and encodes back into the same bytes.
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
		canonicalDoc{A: 1},
		canonicalDocObject{
			U: 1, S: "s", P: &value, Doc: canonicalDoc{A: 2}, In: struct{ A uint64 }{3},
			Ms: map[string]canonicalDoc{"k": {A: 4}}, Ls: []canonicalDocPtr{{A: &value}, {}},
			Ptr: &canonicalDocObject{U: 5},
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
		"0e3f078301800000000000000001", "0e2f07438000000001", "0d0f", "0d6f061526354655410350005affff",
		"08", "0f", "0e2f03130001",
	} {
		f.Add(hexWire(f, seed))
	}

	f.Fuzz(func(t *testing.T, wire []byte) {
		for _, target := range canonicalTargets {
			object := reflect.New(target)
			if err := Depolorize(object.Interface(), wire); err != nil {
				continue
			}

			encoded, err := Polorize(object.Elem().Interface())
			if err != nil {
				t.Fatalf("%v: %x decodes but does not encode: %v", target, wire, err)
			}

			if bytes.Equal(wire, encoded) {
				continue
			}

			if !hasDocumentStruct(target) {
				t.Fatalf("%v: %x decodes but encodes as %x", target, wire, encoded)
			}

			// A document with an absent key: the encoded wire must be a fixed point
			again := reflect.New(target)
			if err := Depolorize(again.Interface(), encoded); err != nil {
				t.Fatalf("%v: %x encodes as %x, which does not decode: %v", target, wire, encoded, err)
			}

			if !reflect.DeepEqual(object.Elem().Interface(), again.Elem().Interface()) {
				t.Fatalf("%v: %x encodes as %x, which decodes into another value", target, wire, encoded)
			}

			reencoded, err := Polorize(again.Elem().Interface())
			if err != nil || !bytes.Equal(encoded, reencoded) {
				t.Fatalf("%v: %x encodes as %x, which encodes as %x (%v)", target, wire, encoded, reencoded, err)
			}
		}
	})
}

// docPair is a DocumentStruct for the form tests
type docPair struct {
	A uint64
	B string
}

func (docPair) PoloDocument() {}

// docPointerPair is a DocumentStruct with a pointer receiver
type docPointerPair struct {
	A uint64
	B string
}

func (*docPointerPair) PoloDocument() {}

// TestCanonical_TypeForms checks that each type has one form: bytes are a word, a map is a pack (also with
// string keys), a struct is a pack, and a DocumentStruct is a document. The decoder rejects the other form.
func TestCanonical_TypeForms(t *testing.T) {
	type Pair struct {
		A uint64
		B string
	}

	type Hash [2]byte

	type Name string

	const (
		wantWord = "incompatible wire: unexpected wiretype 'pack'. expected one of: {null, word}"
		noDoc    = "incompatible wire: unexpected wiretype 'document'. expected one of: {null, pack}"
		wantDoc  = "incompatible wire: unexpected wiretype 'pack'. expected one of: {null, document}"
	)

	packedBytes := hexWire(t, "0e2f03130102")
	pairPack := hexWire(t, "0e2f03160541")
	pairDoc := Document{"A": Raw{3, 5}, "B": Raw{6, 0x41}}.Bytes()

	tests := []struct {
		name  string
		value any
		// wire is the form of the type and other is a wire of the other form
		wire, other []byte
		// err is the error for the other wire
		err string
	}{
		{"bytes", []byte{1, 2}, hexWire(t, "060102"), packedBytes, wantWord},
		{"empty bytes", []byte{}, hexWire(t, "06"), hexWire(t, "0e0f"), wantWord},
		{"named bytes", canonicalBytes{1, 2}, hexWire(t, "060102"), packedBytes, wantWord},
		{"byte array", [2]byte{1, 2}, hexWire(t, "060102"), packedBytes, wantWord},
		{"named byte array", Hash{1, 2}, hexWire(t, "060102"), packedBytes, wantWord},
		{"struct", Pair{5, "A"}, pairPack, pairDoc, noDoc},
		{"empty struct", struct{}{}, hexWire(t, "0e0f"), hexWire(t, "0d0f"), noDoc},
		{"document struct", docPair{5, "A"}, pairDoc, pairPack, wantDoc},
		{"document struct with a pointer receiver", docPointerPair{5, "A"}, pairDoc, pairPack, wantDoc},
		{"pointer to a document struct", &docPair{5, "A"}, pairDoc, pairPack, wantDoc},
		{
			"string map",
			map[string]uint64{"a": 1},
			hexWire(t, "0e2f06136101"), Document{"a": Raw{3, 1}}.Bytes(), noDoc,
		},
		{
			"named string map",
			map[Name]uint64{"a": 1},
			hexWire(t, "0e2f06136101"), Document{"a": Raw{3, 1}}.Bytes(), noDoc,
		},
		{"empty string map", map[string]uint64{}, hexWire(t, "0e0f"), hexWire(t, "0d0f"), noDoc},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The encoder writes the form of the type, and the wire makes a round trip
			wire, err := Polorize(test.value)
			require.NoError(t, err)
			require.Equal(t, test.wire, wire)

			requireRoundtrip(t, wire, test.value)

			// The decoder rejects the other form, and does not change the target
			target := reflect.New(reflect.TypeOf(test.value))

			require.EqualError(t, Depolorize(target.Interface(), test.other), test.err)
			require.True(t, target.Elem().IsZero())
		})
	}

	t.Run("typed methods", func(t *testing.T) {
		hash := bytes.Repeat([]byte{7}, 32)

		words := packWire(func(p *Polorizer) {
			p.PolorizeBytes(hash)
			p.PolorizeBytes(hash)
		})

		// Two packs: a pack is not a form of bytes
		packs := packWire(func(p *Polorizer) {
			require.NoError(t, p.Polorize([]uint16{7}))
			require.NoError(t, p.Polorize([]uint16{7}))
		})

		for _, test := range []struct {
			wire []byte
			err  string
		}{
			{words, ""},
			{packs, wantWord},
		} {
			for _, array := range []bool{false, true} {
				depolorizer, err := NewDepolorizer(test.wire)
				require.NoError(t, err)

				depolorizer, err = depolorizer.DepolorizePacked()
				require.NoError(t, err)

				var decoded []byte

				if array {
					var decoded32 [32]byte

					decoded32, err = depolorizer.DepolorizeBytes32()
					decoded = decoded32[:]
				} else {
					decoded, err = depolorizer.DepolorizeBytes()
				}

				if test.err != "" {
					require.EqualError(t, err, test.err)

					continue
				}

				require.NoError(t, err)
				require.Equal(t, hash, decoded)
			}
		}
	})

	t.Run("a null is nil", func(t *testing.T) {
		requireRoundtrip(t, []byte{0}, []byte(nil))
		requireRoundtrip(t, []byte{0}, map[string]uint64(nil))
		requireRoundtrip(t, []byte{0}, (*Pair)(nil))
		requireRoundtrip(t, []byte{0}, (*docPair)(nil))
	})

	t.Run("nested values", func(t *testing.T) {
		// Each value has the form of its own type, at all levels
		type Outer struct {
			D docPair
			P Pair
			B []byte
			M map[string][]docPair
		}

		object := Outer{docPair{1, "x"}, Pair{2, "y"}, []byte{9}, map[string][]docPair{"k": {{3, "z"}}}}

		wire, err := Polorize(object)
		require.NoError(t, err)

		require.Equal(t, packWire(func(p *Polorizer) {
			p.PolorizeDocument(Document{"A": Raw{3, 1}, "B": Raw{6, 'x'}})
			require.NoError(t, p.Polorize(Pair{2, "y"}))
			p.PolorizeBytes([]byte{9})
			p.PolorizePacked(func() *Polorizer {
				entries := NewPolorizer()
				entries.PolorizeString("k")
				entries.PolorizePacked(func() *Polorizer {
					elements := NewPolorizer()
					elements.PolorizeDocument(Document{"A": Raw{3, 3}, "B": Raw{6, 'z'}})

					return elements
				}())

				return entries
			}())
		}), wire)

		requireRoundtrip(t, wire, object)

		// Document.Set and Document.Get use the same forms
		document := make(Document)
		require.NoError(t, document.Set("k", object))
		require.Equal(t, Raw(wire), document.GetRaw("k"))

		var decoded Outer

		require.NoError(t, document.Get("k", &decoded))
		require.Equal(t, object, decoded)
	})

	t.Run("other types", func(t *testing.T) {
		// A Document, a Raw and an Any keep their bytes, and a big.Int is not a struct for the encoding
		requireRoundtrip(t, hexWire(t, "0d2f061561060102"), Document{"a": Raw{6, 1, 2}})
		requireRoundtrip(t, hexWire(t, "05060102"), Raw{6, 1, 2})
		requireRoundtrip(t, hexWire(t, "060102"), Any{6, 1, 2})
		requireRoundtrip(t, hexWire(t, "0305"), *big.NewInt(5))
	})
}

// keysObject, keysTagged, keysEmpty and keysOuter are DocumentStructs for the key tests
type keysObject struct {
	A int
	P *int
}

func (keysObject) PoloDocument() {}

type keysTagged struct {
	A int `polo:"x"`
	B int `polo:"-"`
	c int //nolint:unused // a field that is not exported has no key
}

func (keysTagged) PoloDocument() {}

type keysEmpty struct{}

func (keysEmpty) PoloDocument() {}

type keysOuter struct{ In keysObject }

func (keysOuter) PoloDocument() {}

// keysDuplicate has two fields with the same key
type keysDuplicate struct {
	A int `polo:"x"`
	B int `polo:"x"`
}

func (keysDuplicate) PoloDocument() {}

// keysEmbedded embeds a DocumentStruct, so it is a DocumentStruct too (the method is promoted)
type keysEmbedded struct {
	keysEmpty
	X int
}

// TestCanonical_DocStructKeys checks the keys of a DocumentStruct. The encoder writes one key for each field.
// The decoder rejects a key that is not the key of a field, and gives the zero value to the field of an absent
// key. Thus, a document with an absent key encodes back with the key: that is the exception to the canonical
// rule, for the documents of an encoder that leaves out a field that has no value.
func TestCanonical_DocStructKeys(t *testing.T) {
	// The encoder writes all keys, also for zero values
	canonical := hexWire(t, "0d4f0615263541035000")

	wire, err := Polorize(keysObject{})
	require.NoError(t, err)
	require.Equal(t, canonical, wire)

	requireRoundtrip(t, canonical, keysObject{})
	requireRoundtrip(t, Document{"x": Raw{3, 7}}.Bytes(), keysTagged{A: 7})
	requireRoundtrip(t, hexWire(t, "0d0f"), keysEmpty{})

	five := 5

	t.Run("absent keys", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			wire   []byte
			object keysObject
		}{
			{"no keys", hexWire(t, "0d0f"), keysObject{}},
			{"absent key", Document{"A": Raw{3, 1}}.Bytes(), keysObject{A: 1}},
			{"absent key of a pointer", Document{"P": Raw{3, 5}}.Bytes(), keysObject{P: &five}},
		} {
			var object keysObject

			require.NoError(t, Depolorize(&object, test.wire), test.name)
			require.Equal(t, test.object, object, test.name)

			// The struct encodes with all keys
			encoded, err := Polorize(object)
			require.NoError(t, err)
			require.NotEqual(t, test.wire, encoded, test.name)
			requireRoundtrip(t, encoded, test.object)
		}
	})

	tests := []struct {
		name   string
		wire   []byte
		target any
		err    string
	}{
		{
			// The value of the unknown key is not a valid wire
			"unknown key", hexWire(t, "0d6f061526354655410350005affff"), new(keysObject),
			"incompatible wire: unknown document key 'Z' for struct polo.keysObject",
		},
		{
			"unknown keys", Document{"A": Raw{3, 1}, "P": Raw{3, 5}, "b": Raw{0}, "Q": Raw{0}}.Bytes(), new(keysObject),
			"incompatible wire: unknown document key 'Q' for struct polo.keysObject",
		},
		{
			"absent key and unknown key", Document{"A": Raw{3, 1}, "Z": Raw{0}}.Bytes(), new(keysObject),
			"incompatible wire: unknown document key 'Z' for struct polo.keysObject",
		},
		{
			"name of a field with another key", Document{"A": Raw{3, 1}, "x": Raw{3, 2}}.Bytes(), new(keysTagged),
			"incompatible wire: unknown document key 'A' for struct polo.keysTagged",
		},
		{
			"key of a skipped field", Document{"B": Raw{3, 1}, "x": Raw{3, 2}}.Bytes(), new(keysTagged),
			"incompatible wire: unknown document key 'B' for struct polo.keysTagged",
		},
		{
			"key of a field that is not exported", Document{"c": Raw{3, 1}, "x": Raw{3, 2}}.Bytes(), new(keysTagged),
			"incompatible wire: unknown document key 'c' for struct polo.keysTagged",
		},
		{
			"key for a struct with no fields", Document{"A": Raw{3, 1}}.Bytes(), new(keysEmpty),
			"incompatible wire: unknown document key 'A' for struct polo.keysEmpty",
		},
		{
			"unknown key in a nested struct",
			Document{"In": Document{"A": Raw{3, 1}, "P": Raw{0}, "Z": Raw{0}}.Bytes()}.Bytes(),
			new(keysOuter),
			"incompatible wire: struct field [polo.keysOuter.In <polo.keysObject>]: " +
				"incompatible wire: unknown document key 'Z' for struct polo.keysObject",
		},
		{
			"null for a field that cannot be nil", Document{"A": Raw{0}}.Bytes(), new(keysObject),
			"incompatible wire: struct field [polo.keysObject.A <int>]: incompatible wire: nil value",
		},
		{
			"value that is not canonical", Document{"A": Raw{3, 0, 5}}.Bytes(), new(keysObject),
			"incompatible wire: struct field [polo.keysObject.A <int>]: incompatible wire: leading zero byte in integer",
		},
		{
			"pack", hexWire(t, "0e2f030000"), new(keysObject),
			"incompatible wire: unexpected wiretype 'pack'. expected one of: {null, document}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, Depolorize(test.target, test.wire), test.err)
			require.True(t, reflect.ValueOf(test.target).Elem().IsZero())
		})
	}

	t.Run("duplicate keys", func(t *testing.T) {
		const duplicate = "incompatible value error: duplicate document key 'x' for struct polo.keysDuplicate"

		_, err := Polorize(keysDuplicate{1, 2})
		require.EqualError(t, err, "could not encode into document: "+duplicate)

		_, err = PolorizeDocument(keysDuplicate{1, 2})
		require.EqualError(t, err, "could not encode into document: "+duplicate)

		require.EqualError(t, Depolorize(new(keysDuplicate), Document{"x": Raw{3, 1}}.Bytes()), duplicate)
	})

	t.Run("embedded document struct", func(t *testing.T) {
		// The embedded field is not exported, so it has no key
		requireRoundtrip(t, Document{"X": Raw{3, 1}}.Bytes(), keysEmbedded{X: 1})
	})
}

// scriptedDecoder is a type whose custom Depolorize method runs scriptedDecode
type scriptedDecoder struct{}

// scriptedDecode is the function that scriptedDecoder.Depolorize runs
var scriptedDecode func(*Depolorizer) error

func (*scriptedDecoder) Depolorize(depolorizer *Depolorizer) error {
	return scriptedDecode(depolorizer)
}

// TestCanonical_CustomDepolorize checks that a custom Depolorize method must read all the data of its element.
// Data that the method does not read can have any content, so different wires would decode into the same value.
func TestCanonical_CustomDepolorize(t *testing.T) {
	const (
		unreadElement = "incompatible wire: the Depolorize method of polo.scriptedDecoder did not read the element"
		unreadPack    = "incompatible wire: the Depolorize method of polo.scriptedDecoder did not read all elements of a pack"
	)

	// readNothing reads nothing
	readNothing := func(*Depolorizer) error { return nil }

	// readFirst unpacks the element and reads the first element of the pack
	readFirst := func(depolorizer *Depolorizer) error {
		pack, err := depolorizer.DepolorizePacked()
		if err != nil {
			return err
		}

		_, err = pack.DepolorizeUint64()

		return err
	}

	// readAll unpacks the element and reads all the elements of the pack as integers
	readAll := func(depolorizer *Depolorizer) error {
		pack, err := depolorizer.Unpacked()
		if err != nil {
			return err
		}

		for !pack.Done() {
			if _, err = pack.DepolorizeUint64(); err != nil {
				return err
			}
		}

		return nil
	}

	// readNested unpacks the element (a pack of packs) and reads the given number of integers from each inner pack
	readNested := func(counts ...int) func(*Depolorizer) error {
		return func(depolorizer *Depolorizer) error {
			outer, err := depolorizer.DepolorizePacked()
			if err != nil {
				return err
			}

			for _, count := range counts {
				inner, err := outer.DepolorizePacked()
				if err != nil {
					return err
				}

				for range count {
					if _, err = inner.DepolorizeUint64(); err != nil {
						return err
					}
				}
			}

			return nil
		}
	}

	list := hexWire(t, "0e3f031323010203")                    // [1, 2, 3]
	lists := hexWire(t, "0e2f0e5e"+"2f03130102"+"2f03130304") // [[1, 2], [3, 4]]
	failure := errors.New("failure")

	tests := []struct {
		name   string
		wire   []byte
		decode func(*Depolorizer) error
		err    string
	}{
		// The element
		{"null, read nothing", hexWire(t, "00"), readNothing, ""},
		{"null, checked with IsNull", hexWire(t, "00"), func(depolorizer *Depolorizer) error {
			if !depolorizer.IsNull() {
				return failure
			}

			return nil
		}, ""},
		{"null, read", hexWire(t, "00"), func(depolorizer *Depolorizer) error { return depolorizer.DepolorizeNull() }, ""},
		{"integer, read nothing", hexWire(t, "0305"), readNothing, unreadElement},
		{"false, read nothing", hexWire(t, "01"), readNothing, unreadElement},
		{"true, read nothing", hexWire(t, "02"), readNothing, unreadElement},
		{"pack, read nothing", list, readNothing, unreadElement},
		{"empty pack, read nothing", hexWire(t, "0e0f"), readNothing, unreadElement},
		{"integer, checked with IsNull", hexWire(t, "0305"), func(depolorizer *Depolorizer) error {
			_ = depolorizer.IsNull()

			return nil
		}, unreadElement},
		{"integer, read", hexWire(t, "0305"), func(depolorizer *Depolorizer) error {
			_, err := depolorizer.DepolorizeUint64()

			return err
		}, ""},
		{"pack, read as an Any", list, func(depolorizer *Depolorizer) error {
			_, err := depolorizer.DepolorizeAny()

			return err
		}, ""},
		{"pack, read with reflection", list, func(depolorizer *Depolorizer) error {
			return depolorizer.Depolorize(new([]uint64))
		}, ""},

		// The packs that the method unpacks
		{"pack, read the first element", list, readFirst, unreadPack},
		{"pack, read all elements", list, readAll, ""},
		{"pack of one element, read the first element", hexWire(t, "0e1f0301"), readFirst, ""},
		{"empty pack, unpacked", hexWire(t, "0e0f"), readAll, ""},
		{"pack, unpacked and read nothing", list, func(depolorizer *Depolorizer) error {
			_, err := depolorizer.DepolorizePacked()

			return err
		}, unreadPack},
		{"document, read the key", Document{"a": Raw{3, 1}}.Bytes(), func(depolorizer *Depolorizer) error {
			pack, err := depolorizer.DepolorizePacked()
			if err != nil {
				return err
			}

			_, err = pack.DepolorizeString()

			return err
		}, unreadPack},
		{"nested packs, read all elements", lists, readNested(2, 2), ""},
		{"nested packs, first inner pack is not read", lists, readNested(1, 2), unreadPack},
		{"nested packs, second inner pack is not read", lists, readNested(2, 1), unreadPack},
		{"nested packs, second inner pack is not unpacked", lists, readNested(2), unreadPack},
		{"nested packs, inner packs are not read", lists, readNested(0, 0), unreadPack},
		{"nested packs, read with reflection", lists, func(depolorizer *Depolorizer) error {
			outer, err := depolorizer.DepolorizePacked()
			if err != nil {
				return err
			}

			var first, second []uint64

			if err = outer.Depolorize(&first); err != nil {
				return err
			}

			return outer.Depolorize(&second)
		}, ""},

		// The error of the method has priority
		{"the method returns an error", list, func(depolorizer *Depolorizer) error {
			if _, err := depolorizer.DepolorizePacked(); err != nil {
				return err
			}

			return failure
		}, "failure"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scriptedDecode = test.decode

			// At the top level, behind a pointer, and in a struct, a list and a map
			var (
				value   scriptedDecoder
				pointer *scriptedDecoder
			)

			type Object struct{ X scriptedDecoder }

			nested := packWire(func(p *Polorizer) { require.NoError(t, p.PolorizeAny(test.wire)) })
			mapped := packWire(func(p *Polorizer) {
				p.PolorizeUint(1)
				require.NoError(t, p.PolorizeAny(test.wire))
			})

			errs := []error{
				Depolorize(&value, test.wire),
				Depolorize(new(Object), nested),
				Depolorize(new([]scriptedDecoder), nested),
				Depolorize(new(map[uint8]scriptedDecoder), mapped),
			}

			// A pointer decodes a Null before the method runs
			if !bytes.Equal(test.wire, []byte{0}) {
				errs = append(errs, Depolorize(&pointer, test.wire))
			}

			for _, err := range errs {
				if test.err == "" {
					require.NoError(t, err)

					continue
				}

				require.ErrorContains(t, err, test.err)
			}

			if test.err == "" {
				return
			}

			require.EqualError(t, errs[0], test.err)

			if test.err != "failure" {
				require.ErrorAs(t, errs[0], new(IncompatibleWireError))
			}
		})
	}

	t.Run("lazy struct decoder of the review", func(t *testing.T) {
		// The wire has the elements 1, 2 and 3, and the method reads only the first element.
		// A struct with one field rejects the same wire
		scriptedDecode = readFirst

		require.EqualError(t, Depolorize(new(scriptedDecoder), list), unreadPack)
		require.EqualError(t, Depolorize(new(struct{ A uint64 }), list),
			"incompatible wire: excess elements for struct struct { A uint64 }")
	})

	t.Run("a direct call is not checked", func(t *testing.T) {
		// Code that calls a Depolorize method itself must check Done
		depolorizer, err := NewDepolorizer(list)
		require.NoError(t, err)
		require.Nil(t, depolorizer.track)

		require.NoError(t, readFirst(depolorizer))
		require.True(t, depolorizer.Done())

		depolorizer, err = NewDepolorizer(list)
		require.NoError(t, err)

		pack, err := depolorizer.DepolorizePacked()
		require.NoError(t, err)
		require.Nil(t, pack.track)

		_, err = pack.DepolorizeUint64()
		require.NoError(t, err)
		require.False(t, pack.Done())
	})
}

// invalidStringCodec is a type whose custom Polorize method writes a value that is not valid UTF-8
// with a method of the Polorizer that has no error result
type invalidStringCodec struct {
	// document selects a Document key in place of a string
	document bool
	// depth is the number of packs around the value
	depth int
}

func (codec invalidStringCodec) Polorize() (*Polorizer, error) {
	polorizer := NewPolorizer()
	polorizer.PolorizeUint(1)

	if codec.document {
		polorizer.PolorizeDocument(Document{"\xff": Raw{3}})
	} else {
		polorizer.PolorizeString("\xff")
	}

	for range codec.depth {
		outer := NewPolorizer()
		outer.PolorizePacked(polorizer)
		polorizer = outer
	}

	return polorizer, nil
}

// TestPolorizer_Err checks that a Polorizer keeps the first error of the methods that have no error result,
// so that a custom Polorize method cannot write a wire that the decoder rejects
func TestPolorizer_Err(t *testing.T) {
	const invalid = "incompatible value error: invalid UTF-8 string"

	t.Run("no error", func(t *testing.T) {
		polorizer := NewPolorizer()
		require.NoError(t, polorizer.Err())

		polorizer.PolorizeString("a\x00�")
		polorizer.PolorizeString("")
		polorizer.PolorizeBytes([]byte{0xff})
		polorizer.PolorizeDocument(Document{"a": Raw{3}})
		polorizer.PolorizeDocument(nil)
		polorizer.PolorizePacked(NewPolorizer())
		polorizer.PolorizePacked(nil)
		require.NoError(t, polorizer.Err())
	})

	t.Run("PolorizeString", func(t *testing.T) {
		polorizer := NewPolorizer()
		polorizer.PolorizeString("\xff")

		require.ErrorIs(t, polorizer.Err(), ErrInvalidUTF8)
		require.EqualError(t, polorizer.Err(), invalid)

		// The bytes are the same as in v0.4.10, and they do not decode into a string
		require.Equal(t, hexWire(t, "06ff"), polorizer.Bytes())
		require.Equal(t, hexWire(t, "0e1f06ff"), polorizer.Packed())
		require.ErrorIs(t, Depolorize(new(string), polorizer.Bytes()), ErrInvalidUTF8)

		// A later valid value does not remove the error
		polorizer.PolorizeString("a")
		require.ErrorIs(t, polorizer.Err(), ErrInvalidUTF8)
	})

	t.Run("PolorizeDocument", func(t *testing.T) {
		polorizer := NewPolorizer()
		polorizer.PolorizeDocument(Document{"a": Raw{3}, "\xff": Raw{3}})

		require.ErrorIs(t, polorizer.Err(), ErrInvalidUTF8)
		require.EqualError(t, polorizer.Err(), invalid)

		// Document.Bytes has no error result. Polorize is the form that checks the keys
		document := Document{"\xff": Raw{3}}

		require.Equal(t, hexWire(t, "0d2f0615ff03"), document.Bytes())
		require.ErrorIs(t, Depolorize(new(Document), document.Bytes()), ErrInvalidUTF8)

		_, err := Polorize(document)
		require.ErrorIs(t, err, ErrInvalidUTF8)
	})

	t.Run("PolorizePacked", func(t *testing.T) {
		inner := NewPolorizer()
		inner.PolorizeString("\xff")

		middle := NewPolorizer()
		middle.PolorizePacked(inner)

		outer := NewPolorizer()
		outer.PolorizePacked(middle)

		require.ErrorIs(t, middle.Err(), ErrInvalidUTF8)
		require.ErrorIs(t, outer.Err(), ErrInvalidUTF8)
	})

	t.Run("the first error is kept", func(t *testing.T) {
		first, second := errors.New("first"), errors.New("second")

		one := NewPolorizer()
		one.fail(first)

		two := NewPolorizer()
		two.fail(second)

		polorizer := NewPolorizer()
		polorizer.PolorizePacked(one)
		polorizer.PolorizePacked(two)
		polorizer.PolorizeString("\xff")

		require.Equal(t, first, polorizer.Err())
	})

	t.Run("Polorize of a custom type", func(t *testing.T) {
		for _, codec := range []invalidStringCodec{
			{false, 0}, {false, 1}, {false, 3}, {true, 0}, {true, 2},
		} {
			// The method returns no error, but its Polorizer has one
			polorizer, err := codec.Polorize()
			require.NoError(t, err)
			require.ErrorIs(t, polorizer.Err(), ErrInvalidUTF8)

			for _, object := range []any{
				codec, &codec,
				[]invalidStringCodec{codec},
				map[uint8]invalidStringCodec{1: codec},
				struct{ X invalidStringCodec }{codec},
				struct{ X Polorizable }{codec},
			} {
				_, err = Polorize(object)
				require.ErrorIs(t, err, ErrInvalidUTF8)
				require.EqualError(t, err, invalid)
			}

			_, err = PolorizeDocument(struct{ X invalidStringCodec }{codec})
			require.ErrorIs(t, err, ErrInvalidUTF8)

			require.ErrorIs(t, make(Document).Set("k", codec), ErrInvalidUTF8)

			// The Polorize method of a Polorizer returns the error, writes nothing and does not keep the error
			outer := NewPolorizer()

			require.ErrorIs(t, outer.Polorize(codec), ErrInvalidUTF8)
			require.NoError(t, outer.Err())
			require.Equal(t, []byte{0}, outer.Bytes())
		}
	})

	t.Run("methods with an error result do not keep the error", func(t *testing.T) {
		polorizer := NewPolorizer()

		require.Error(t, polorizer.Polorize("\xff"))
		require.Error(t, polorizer.Polorize(make(chan int)))
		require.Error(t, polorizer.PolorizeAny(Any{0x13, 5}))
		require.NoError(t, polorizer.Err())
		require.Equal(t, []byte{0}, polorizer.Bytes())
	})
}

// TestC7_NegativeZeroMapKey checks that the encoder writes a negative zero map key as positive zero.
// The two zeros are equal, so they are the same key of a Go map, and that key must have one wire.
// The decoder rejects a negative zero key (see TestCanonical_Rejected).
func TestC7_NegativeZeroMapKey(t *testing.T) {
	type Float float64

	negz := math.Copysign(0, -1)
	negz32 := float32(negz)

	tests := []struct {
		name               string
		negative, positive any
	}{
		{"float64 key", map[float64]uint64{negz: 1}, map[float64]uint64{0: 1}},
		{"float32 key", map[float32]uint64{negz32: 1}, map[float32]uint64{0: 1}},
		{"named float key", map[Float]uint64{Float(negz): 1}, map[Float]uint64{0: 1}},
		{
			"sorted with other keys",
			map[float64]bool{1: true, negz: false, -1: true},
			map[float64]bool{-1: true, 0: false, 1: true},
		},
		{"array key", map[[2]float64]uint64{{negz, 1}: 1}, map[[2]float64]uint64{{0, 1}: 1}},
		{"array key with two zeros", map[[2]float32]uint64{{negz32, negz32}: 1}, map[[2]float32]uint64{{0, 0}: 1}},
		{"nested array key", map[[1][2]float64]bool{{{1, negz}}: true}, map[[1][2]float64]bool{{{1, 0}}: true}},
		{"map in a list", []map[float64]bool{{negz: true}}, []map[float64]bool{{0: true}}},
		{
			"map in a struct",
			struct{ M map[float64]bool }{map[float64]bool{negz: true}},
			struct{ M map[float64]bool }{map[float64]bool{0: true}},
		},
		{"map as a map value", map[string]map[float32]bool{"a": {negz32: true}}, map[string]map[float32]bool{"a": {0: true}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The two maps are equal in Go, and they have the same wire
			require.True(t, reflect.DeepEqual(test.negative, test.positive))

			positive, err := Polorize(test.positive)
			require.NoError(t, err)

			negative, err := Polorize(test.negative)
			require.NoError(t, err)
			require.Equal(t, positive, negative)

			// The wire has no negative zero, and it makes a round trip
			require.NotContains(t, string(negative), "\x80\x00\x00\x00")
			requireRoundtrip(t, negative, test.positive)
		})
	}

	t.Run("wire", func(t *testing.T) {
		wire, err := Polorize(map[float64]int{negz: 1})
		require.NoError(t, err)
		require.Equal(t, hexWire(t, "0e3f078301"+"0000000000000000"+"01"), wire)

		// The decoded key is positive zero
		var decoded map[float64]int

		require.NoError(t, Depolorize(&decoded, wire))

		for key := range decoded {
			require.False(t, math.Signbit(key))
		}
	})

	t.Run("the map is not changed", func(t *testing.T) {
		floats := map[float64]int{negz: 1}
		arrays := map[[1]float64]int{{negz}: 1}

		_, err := Polorize(floats)
		require.NoError(t, err)

		_, err = Polorize(arrays)
		require.NoError(t, err)

		for key := range floats {
			require.True(t, math.Signbit(key))
		}

		for key := range arrays {
			require.True(t, math.Signbit(key[0]))
		}
	})

	t.Run("a negative zero that is not a map key keeps its bits", func(t *testing.T) {
		requireRoundtrip(t, hexWire(t, "078000000000000000"), negz)
		requireRoundtrip(t, hexWire(t, "0780000000"), negz32)

		for _, object := range []any{
			map[uint8]float64{1: negz},
			map[string]float32{"a": negz32},
			map[float64]float64{0: negz},
			[]float64{negz},
			[1]float64{negz},
			struct{ F float64 }{negz},
			map[uint8][1]float64{1: {negz}},
		} {
			wire, err := Polorize(object)
			require.NoError(t, err)
			require.Contains(t, string(wire), "\x80\x00\x00\x00")

			decoded := reflect.New(reflect.TypeOf(object))
			require.NoError(t, Depolorize(decoded.Interface(), wire))

			encoded, err := Polorize(decoded.Elem().Interface())
			require.NoError(t, err)
			require.Equal(t, wire, encoded)
		}
	})
}
