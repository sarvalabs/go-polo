package polo

import (
	"fmt"
	"math/big"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testObject[T any](t *testing.T, x T) {
	wire, err := Polorize(x)
	require.Nil(t, err)

	y := new(T)
	err = Depolorize(y, wire)

	require.Nil(t, err, "Unexpected Error. Input: %v", x)
	require.Equal(t, x, *y, "Object Mismatch. Input: %v", x)

	rewire, err := Polorize(*y)
	require.Nil(t, err)
	require.Equal(t, wire, rewire, "Wire Mismatch. Input: %v", x)
}

type IntegerObject struct {
	A int
	B int8
	C int16
	D int32
	E int64
	F uint
	G uint8
	H uint16
	I uint32
	J uint64
}

func TestInteger(t *testing.T) {
	f := fuzz.New()

	t.Run("Int", func(t *testing.T) {
		var x int

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint", func(t *testing.T) {
		var x uint

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Int8", func(t *testing.T) {
		var x int8

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint8", func(t *testing.T) {
		var x uint8

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Int16", func(t *testing.T) {
		var x int16

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint16", func(t *testing.T) {
		var x uint16

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Int32", func(t *testing.T) {
		var x int32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint32", func(t *testing.T) {
		var x uint32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Int64", func(t *testing.T) {
		var x int64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint64", func(t *testing.T) {
		var x uint64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("IntegerObject", func(t *testing.T) {
		var x IntegerObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type BoolObject struct {
	A bool
	B bool
}

func TestBool(t *testing.T) {
	f := fuzz.New()

	t.Run("Bool", func(t *testing.T) {
		var x bool

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("BoolObject", func(t *testing.T) {
		var x BoolObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type WordObject struct {
	A string
	B string
	C []byte
	D []byte
}

func TestWord(t *testing.T) {
	f := fuzz.New()
	f.NumElements(0, 10)

	t.Run("String", func(t *testing.T) {
		var x string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Bytes", func(t *testing.T) {
		var x []byte

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("WordObject", func(t *testing.T) {
		var x WordObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type FloatObject struct {
	A float32
	B float32
	C float64
	D float64
}

func TestFloat(t *testing.T) {
	f := fuzz.New()

	t.Run("Float32", func(t *testing.T) {
		var x float32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Float64", func(t *testing.T) {
		var x float64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("FloatObject", func(t *testing.T) {
		var x FloatObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type SequenceObject struct {
	A []string
	B []uint64
	C []map[string]string
	D [][]string
	E [][]byte
	F []WordObject
	G [4]string
	H [32]byte
	I [2]map[uint64]bool
	J [4][4]float32
	K [6]IntegerObject
}

func TestSequence(t *testing.T) {
	f := fuzz.New().NumElements(0, 8)

	t.Run("String Slice", func(t *testing.T) {
		var x []string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Uint64 Slice", func(t *testing.T) {
		var x []uint64

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Map Slice", func(t *testing.T) {
		var x []map[string]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Double String Slice", func(t *testing.T) {
		var x [][]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Double Byte Slice", func(t *testing.T) {
		var x [][]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Array Slice", func(t *testing.T) {
		var x [][8]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("WordObject Slice", func(t *testing.T) {
		var x []WordObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("String Array", func(t *testing.T) {
		var x [4]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Byte Array", func(t *testing.T) {
		var x [32]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Map Array", func(t *testing.T) {
		var x [2]map[uint64]bool

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Double Float Array", func(t *testing.T) {
		var x [4][4]float32

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Slice Array", func(t *testing.T) {
		var x [4][]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Bytes Array", func(t *testing.T) {
		var x [4][]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("IntegerObject Array", func(t *testing.T) {
		var x [6]IntegerObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("SequenceObject", func(t *testing.T) {
		var x SequenceObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type MapObject struct {
	A map[bool]string
	B map[float32]uint64
	C map[float64]map[string]string
	D map[int32][]string
	E map[string]bool
	F map[uint64]string
	G map[[2]uint]string
	H map[[2]int]int
	I map[[4]float32]uint64
	J map[[2][2]string]string
}

func TestMapping(t *testing.T) {
	f := fuzz.New().NumElements(0, 8)

	t.Run("String Map", func(t *testing.T) {
		var x map[string]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Integer Map", func(t *testing.T) {
		var x map[int32]float32

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Array Map", func(t *testing.T) {
		var x map[[2]string]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Double Map", func(t *testing.T) {
		var x map[string]map[uint64]bool

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Bytes Map", func(t *testing.T) {
		var x map[[32]byte][]byte

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("Array Keyed Map", func(t *testing.T) {
		var err error

		_, err = Polorize(map[[3]uint64]string{[3]uint64{10, 12, 12}: "foo", [3]uint64{10, 11, 11}: "boo"})
		require.Nil(t, err)

		_, err = Polorize(map[[3]int64]string{[3]int64{10, 12, 12}: "foo", [3]int64{10, 11, 11}: "boo"})
		require.Nil(t, err)

		_, err = Polorize(map[[3]float32]string{[3]float32{10, 12, 12}: "foo", [3]float32{10, 11, 11}: "boo"})
		require.Nil(t, err)
	})

	t.Run("MapObject", func(t *testing.T) {
		var x MapObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type PointerObject struct {
	A *WordObject
	B *IntegerObject
	C *FloatObject
}

func TestPointer(t *testing.T) {
	f := fuzz.New()

	var x PointerObject

	for i := 0; i < 1000; i++ {
		f.Fuzz(&x)
		testObject(t, x)
	}
}

type NestedObject struct {
	A WordObject
	B IntegerObject
	C FloatObject
}

func TestNested(t *testing.T) {
	f := fuzz.New().NilChance(0.2)

	var x NestedObject

	for i := 0; i < 10000; i++ {
		f.Fuzz(&x)
		testObject(t, x)
	}
}

type BigObject struct {
	A big.Int
	B big.Int
	C *big.Int
}

func TestBig(t *testing.T) {
	f := fuzz.New().NilChance(0.2)

	t.Run("Big Int", func(t *testing.T) {
		var x big.Int

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})

	t.Run("BigObject", func(t *testing.T) {
		var x BigObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testObject(t, x)
		}
	})
}

type SimpleInterface interface{}

type InterfaceObject struct {
	A SimpleInterface
	B SimpleInterface
	C SimpleInterface
}

func TestInterface(t *testing.T) {
	var x1, x2, x3 string
	var x InterfaceObject

	x = InterfaceObject{x1, x2, x3}
	wire, err := Polorize(x)

	require.Nil(t, wire)
	require.EqualError(t, err, "encode error: unsupported type: polo.SimpleInterface [interface]")
}

func TestUnsupported(t *testing.T) {
	var err error

	// Channels
	_, err = Polorize(make(chan string))
	require.EqualError(t, err, "encode error: unsupported type: chan string [chan]")

	// Functions
	_, err = Polorize(new(func(string)))
	require.EqualError(t, err, "encode error: unsupported type: func(string) [func]")

	// Slice of Unsupported Types
	_, err = Polorize(make([]func(string), 2))
	require.EqualError(t, err, "encode error: unsupported type: func(string) [func]")

	// Array of Unsupported Types
	_, err = Polorize(new([2]chan string))
	require.EqualError(t, err, "encode error: unsupported type: chan string [chan]")

	// Map with Unsupported Type for Keys
	_, err = Polorize(map[SimpleInterface]string{"foo": "bar", "boo": "far"})
	require.EqualError(t, err, "encode error: unsupported type: polo.SimpleInterface [interface]")

	// Map with Unsupported Type for Keys
	_, err = Polorize(map[[2]SimpleInterface]string{[2]SimpleInterface{"foo", "fon"}: "bar", [2]SimpleInterface{"boo", "bon"}: "far"})
	require.EqualError(t, err, "encode error: unsupported type: polo.SimpleInterface [interface]")

	// Map with Unsupported Type for Values
	_, err = Polorize(map[string]chan int{"foo": make(chan int)})
	require.EqualError(t, err, "encode error: unsupported type: chan int [chan]")
}

type SkipObject struct {
	A string
	B string `polo:"-"`
	C int32  `polo:"-"`
	d int32
}

func TestSkip(t *testing.T) {
	f := fuzz.New().NilChance(0.2)

	var x SkipObject

	for i := 0; i < 10000; i++ {
		f.Fuzz(&x)
		wire, err := Polorize(x)
		require.Nil(t, err)

		y := new(SkipObject)
		err = Depolorize(y, wire)
		require.Nil(t, err)

		x.B, x.C, x.d = "", 0, 0

		require.Nil(t, err, "Unexpected Error. Input: %v", x)
		require.Equal(t, x, *y, "Object Mismatch. Input: %v", x)

		rewire, err := Polorize(*y)
		require.Nil(t, err)
		require.Equal(t, wire, rewire, "Wire Mismatch. Input: %v", x)
	}
}

type (
	StringAlias string
	BytesAlias  []byte
	HashAlias   [32]byte
)

type AliasObject struct {
	A StringAlias
	B BytesAlias
	C HashAlias
	D []StringAlias
	E [2]BytesAlias
	F map[string]HashAlias
}

func TestAlias(t *testing.T) {
	f := fuzz.New().NilChance(0.2)

	var x AliasObject

	for i := 0; i < 1000; i++ {
		f.Fuzz(&x)
		testObject(t, x)
	}
}

func TestEmpty(t *testing.T) {
	x := struct{}{}
	testObject(t, x)
}

func TestNullObject(t *testing.T) {
	t.Run("Slice", func(t *testing.T) {
		var x []string
		require.Nil(t, x)

		wire, err := Polorize(x)
		assert.Nil(t, err)
		assert.Equal(t, wire, []byte{0})
	})

	t.Run("Map", func(t *testing.T) {
		var x map[string]uint64
		require.Nil(t, x)

		wire, err := Polorize(x)
		assert.Nil(t, err)
		assert.Equal(t, wire, []byte{0})
	})

	t.Run("Struct", func(t *testing.T) {
		var x *IntegerObject
		require.Nil(t, x)

		wire, err := Polorize(x)
		assert.Nil(t, err)
		assert.Equal(t, wire, []byte{0})
	})

	t.Run("Any", func(t *testing.T) {
		var x any
		require.Nil(t, x)

		_, err := Polorize(x)
		assert.EqualError(t, err, "encode error: unsupported type: cannot encode abstract nil")
	})
}

func TestNullWire(t *testing.T) {
	var err error
	wire := []byte{0}

	t.Run("Bool", func(t *testing.T) {
		x := new(bool)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.False(t, *x)
	})

	t.Run("String", func(t *testing.T) {
		x := new(string)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, "", *x)
	})

	t.Run("Uint", func(t *testing.T) {
		x := new(uint64)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, uint64(0), *x)
	})

	t.Run("Int", func(t *testing.T) {
		x := new(int64)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, int64(0), *x)
	})

	t.Run("Float32", func(t *testing.T) {
		x := new(float32)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, float32(0), *x)
	})

	t.Run("Float64", func(t *testing.T) {
		x := new(float64)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, float64(0), *x)
	})

	t.Run("Slice", func(t *testing.T) {
		x := new([]string)
		err = Depolorize(x, wire)

		var nilslc []string
		require.Nil(t, err)
		assert.Equal(t, nilslc, *x)
	})

	t.Run("Array", func(t *testing.T) {
		x := new([4]string)
		err = Depolorize(x, wire)

		require.Nil(t, err)
		assert.Equal(t, *new([4]string), *x)
	})

	t.Run("Map", func(t *testing.T) {
		x := new(map[string]string)
		err = Depolorize(x, wire)

		var nilmap map[string]string
		require.Nil(t, err)
		assert.Equal(t, nilmap, *x)
	})
}

func TestExcessIntegerData(t *testing.T) {
	tests := []struct {
		wire   []byte
		object any
		size   int
		signed bool
	}{
		{
			[]byte{3, 255, 255},
			new(uint8), 8, false,
		},
		{
			[]byte{3, 255, 255},
			new(int8), 8, true,
		},
		{
			[]byte{3, 255, 255, 255},
			new(uint16), 16, false,
		},
		{
			[]byte{3, 255, 255, 255},
			new(int16), 16, true,
		},
		{
			[]byte{3, 255, 255, 255, 255, 255, 255},
			new(uint32), 32, false,
		},
		{
			[]byte{3, 255, 255, 255, 255, 255, 255},
			new(int32), 32, true,
		},
		{
			[]byte{3, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(uint64), 64, false,
		},
		{
			[]byte{3, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(int64), 64, true,
		},
	}

	for tno, test := range tests {
		err := Depolorize(test.object, test.wire)
		if test.signed {
			assert.EqualError(t, err,
				fmt.Sprintf("decode error: excess data for %v-bit signed integer", test.size),
				"[%v] Input: %v", tno, test.wire)
		} else {
			assert.EqualError(t, err,
				fmt.Sprintf("decode error: excess data for %v-bit integer", test.size),
				"[%v] Input: %v", tno, test.wire)
		}
	}
}

func TestMalformedFloatData(t *testing.T) {
	tests := []struct {
		wire   []byte
		object any
		err    error
	}{
		{
			[]byte{7, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(float32),
			DecodeError{"malformed data for 32-bit float"},
		},
		{
			[]byte{7, 111, 114, 97},
			new(float64),
			DecodeError{"malformed data for 64-bit float"},
		},
		{
			[]byte{7, 255, 255, 0, 0},
			new(float32),
			DecodeError{"float is not a number"},
		},
		{
			[]byte{7, 255, 255, 0, 0, 0, 0, 0, 0},
			new(float64),
			DecodeError{"float is not a number"},
		},
	}

	for tno, test := range tests {
		err := Depolorize(test.object, test.wire)
		assert.EqualError(t, err, test.err.Error(), "[%v] Input: %v", tno, test.wire)
	}
}

func TestIncompatibleWireType(t *testing.T) {
	tests := []struct {
		wire   []byte
		object any
		err    error
	}{
		{
			[]byte{2},
			new(float32),
			DecodeError{"incompatible wire type. expected: float. got: true"},
		},
		{
			[]byte{4, 1},
			new(float64),
			DecodeError{"incompatible wire type. expected: float. got: negint"},
		},
		{
			[]byte{7, 111, 114, 97, 110, 103, 101},
			new(string),
			DecodeError{"incompatible wire type. expected: word. got: float"},
		},
		{
			[]byte{3, 44},
			new(bool),
			DecodeError{"incompatible wire type. expected: true. got: posint"},
		},
		{
			[]byte{2},
			new(uint64),
			DecodeError{"incompatible wire type. expected: posint. got: true"},
		},
		{
			[]byte{4, 45, 22},
			new([]string),
			DecodeError{"incompatible wire type. expected: pack. got: negint"},
		},
		{
			[]byte{5, 45, 22},
			new([4]string),
			DecodeError{"incompatible wire type. expected: pack. got: bigint"},
		},
		{
			[]byte{5, 45, 22},
			new(map[string]string),
			DecodeError{"incompatible wire type. expected: pack. got: bigint"},
		},
		{
			[]byte{3, 45, 22},
			new(big.Int),
			DecodeError{"incompatible wire type. expected: bigint. got: posint"},
		},
		{
			[]byte{3, 45, 22},
			new(*IntegerObject),
			DecodeError{"incompatible wire type. expected: pack. got: posint"},
		},
		{
			[]byte{14, 95, 3, 3, 3, 3, 3},
			&WordObject{},
			DecodeError{"struct field [polo.WordObject.A <string>]: " +
				"incompatible wire type. expected: word. got: posint"},
		},
		{
			[]byte{14, 95, 1, 0, 0, 0, 0},
			&IntegerObject{},
			DecodeError{"struct field [polo.IntegerObject.A <int>]: " +
				"incompatible wire type. expected: posint. got: false"},
		},
		{
			[]byte{13, 47, 6, 22, 65, 1},
			&IntegerObject{},
			DecodeError{"struct field [polo.IntegerObject.A <int>]: " +
				"incompatible wire type. expected: posint. got: false"},
		},
	}

	for tno, test := range tests {
		err := Depolorize(test.object, test.wire)
		assert.EqualError(t, err, test.err.Error(), "[%v] Input: %v", tno, test.wire)
	}
}

func TestMalformed(t *testing.T) {
	tests := []struct {
		wire   []byte
		object any
		err    error
	}{
		{
			[]byte{14, 78, 3, 3, 3, 3},
			IntegerObject{},
			ErrObjectNotPtr,
		},
		{
			[]byte{3, 255, 255, 255, 255, 255, 255, 255, 255},
			new(int64),
			DecodeError{"overflow for signed integer"},
		},
		{
			[]byte{255, 128, 128, 128, 128, 128, 128, 128, 128, 127, 93, 3, 3, 3, 3, 3},
			&IntegerObject{},
			DecodeError{"malformed tag: varint overflows 64-bit integer"},
		},
		{
			[]byte{6, 255, 255, 255},
			new([2]byte),
			DecodeError{"mismatched data length for byte array"},
		},

		{
			[]byte{14, 78, 3, 3, 3, 3},
			&IntegerObject{},
			DecodeError{"load convert fail: missing load tag"},
		},
		{
			[]byte{14, 255, 128, 128, 128, 128, 128, 128, 128, 128, 127, 3, 3, 3, 3, 3},
			&IntegerObject{},
			DecodeError{"load convert fail: malformed tag: varint overflows 64-bit integer"},
		},
		{
			[]byte{14, 79, 3, 3, 3, 3, 0, 0, 0, 0},
			&IntegerObject{},
			DecodeError{"loadreader exhausted"},
		},
	}

	for tno, test := range tests {
		err := Depolorize(test.object, test.wire)
		assert.EqualError(t, err, test.err.Error(), "[%v] Input: %v", tno, test.wire)
	}
}
