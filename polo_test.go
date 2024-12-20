package polo

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fruit is an example for a Go struct
type Fruit struct {
	Name  string
	Cost  int      `polo:"cost"`
	Alias []string `polo:"alias"`
}

// ExamplePolorize is an example for using the Polorize function to
// encode a Fruit object into its POLO wire form using Go reflection
//
//nolint:lll
func ExamplePolorize() {
	// Create a Fruit object
	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Serialize the Fruit object
	wire, err := Polorize(orange)
	if err != nil {
		log.Fatalln(err)
	}

	// Print the serialized bytes
	fmt.Println(wire)

	// Output:
	// [14 79 6 99 142 1 111 114 97 110 103 101 1 44 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
}

// ExampleDepolorize is an example for using the Depolorize function to
// decode a Fruit object from its POLO wire form using Go reflection
func ExampleDepolorize() {
	wire := []byte{
		14, 79, 6, 99, 142, 1, 111, 114, 97, 110, 103, 101, 1, 44, 63, 6, 150, 1, 116,
		97, 110, 103, 101, 114, 105, 110, 101, 109, 97, 110, 100, 97, 114, 105, 110,
	}

	// Create a new instance of Fruit
	object := new(Fruit)
	// Deserialize the wire into the Fruit object (must be a pointer)
	if err := Depolorize(object, wire); err != nil {
		log.Fatalln(err)
	}

	// Print the deserialized object
	fmt.Println(object)

	// Output:
	// &{orange 300 [tangerine mandarin]}
}

// CustomFruit is an example for a Go struct that
// implements the Polorizable and Depolorizable interfaces
type CustomFruit struct {
	Name  string
	Cost  int
	Alias []string
}

// Polorize implements the Polorizable interface for
// CustomFruit and allows custom serialization of Fruit objects
func (fruit CustomFruit) Polorize() (*Polorizer, error) {
	fmt.Println("Custom Serialize for Fruit Invoked")

	// Create a new Polorizer
	polorizer := NewPolorizer()

	// Encode the Name field as a string
	polorizer.PolorizeString(fruit.Name)
	// Encode the Cost field as an integer
	polorizer.PolorizeInt(int64(fruit.Cost))

	// Create a new Polorizer to serialize the Alias field (slice)
	aliases := NewPolorizer()
	// Encode each element in the Alias slice as a string
	for _, alias := range fruit.Alias {
		aliases.PolorizeString(alias)
	}
	// Encode the Polorizer containing the alias field contents as packed data
	polorizer.PolorizePacked(aliases)

	return polorizer, nil
}

// Depolorize implements the Depolorizable interface for
// CustomFruit and allows custom deserialization of Fruit objects
func (fruit *CustomFruit) Depolorize(depolorizer *Depolorizer) (err error) {
	fmt.Println("Custom Deserialize for Fruit Invoked")

	// Convert the Depolorizer into a pack Depolorizer
	depolorizer, err = depolorizer.DepolorizePacked()
	if err != nil {
		return fmt.Errorf("invalid wire: not a pack: %w", err)
	}

	// Decode the Name field as a string
	fruit.Name, err = depolorizer.DepolorizeString()
	if err != nil {
		log.Fatalln("invalid field 'Name':", err)
	}

	// Decode the Cost field as a string
	fruit.Cost, err = depolorizer.DepolorizeInt()
	if err != nil {
		log.Fatalln("invalid field 'Cost':", err)
	}

	// Decode a new Depolorizer to deserialize the Alias field (slice)
	aliases, err := depolorizer.DepolorizePacked()
	if err != nil {
		log.Fatalln("invalid field 'Alias':", err)
	}

	// Decode each element from the Alias decoder as a string
	for !aliases.Done() {
		alias, err := aliases.DepolorizeString()
		if err != nil {
			log.Fatalln("invalid field element 'Alias':", err)
		}

		fruit.Alias = append(fruit.Alias, alias)
	}

	return nil
}

// ExampleCustomEncoding is an example for using custom serialization and deserialization on the
// CustomFruit type by implementing the Polorizable and Depolorizable interfaces for it.
//
//nolint:govet, lll
func ExampleCustomEncoding() {
	// Create a CustomFruit object
	orange := &CustomFruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Serialize the Fruit object
	wire, err := Polorize(orange)
	if err != nil {
		log.Fatalln(err)
	}

	// Print the serialized bytes
	fmt.Println(wire)

	// Create a new instance of CustomFruit
	object := new(CustomFruit)
	// Deserialize the wire into the CustomFruit object (must be a pointer)
	if err := Depolorize(object, wire); err != nil {
		log.Fatalln(err)
	}

	// Print the deserialized object
	fmt.Println(object)

	// Output:
	// Custom Serialize for Fruit Invoked
	// [14 79 6 99 142 1 111 114 97 110 103 101 1 44 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
	// Custom Deserialize for Fruit Invoked
	// &{orange 300 [tangerine mandarin]}
}

// ExampleWireDecoding is an example for using the Any type to capture
// the raw POLO encoded bytes for a specific field of the Fruit object.
//
//nolint:govet, lll
func ExampleWireDecoding() {
	// RawFruit is a struct that can capture the raw POLO bytes of each field
	type RawFruit struct {
		Name  Any
		Cost  int
		Alias []string
	}

	// Create a Fruit object
	orange := &Fruit{"orange", 300, []string{"tangerine", "mandarin"}}

	// Serialize the Fruit object
	wire, err := Polorize(orange)
	if err != nil {
		log.Fatalln(err)
	}

	// Print the serialized bytes
	fmt.Println(wire)

	// Create a new instance of RawFruit
	object := new(RawFruit)
	// Deserialize the wire into the RawFruit object (must be a pointer)
	if err := Depolorize(object, wire); err != nil {
		log.Fatalln(err)
	}

	// Print the deserialized object
	fmt.Println(object)

	// Output:
	// [14 79 6 99 142 1 111 114 97 110 103 101 1 44 63 6 150 1 116 97 110 103 101 114 105 110 101 109 97 110 100 97 114 105 110]
	// &{[6 111 114 97 110 103 101] 300 [tangerine mandarin]}
}

// testSerialization is a generic function that tests the serialization consistency for a given object.
// It is to be used on fuzzed values and accepts some encoding options that are applied at every encode/decode step.
// The steps performed in this function are as follows
//  1. Encode the given object (options applied)
//  2. Decode the wire output of step 1 (options applied)
//  3. Verify object equality (given object == decoded object)
//  4. Encode the decoded object (options applied)
//  5. Verify wire equality (step 1 wire == step 4 wire)
func testSerialization[T any](t *testing.T, x T, options ...EncodingOptions) {
	t.Helper()

	wire, err := Polorize(x, options...)
	require.Nil(t, err)

	y := new(T)
	err = Depolorize(y, wire, options...)

	require.Nil(t, err, "Unexpected Error. Input: %v", x)
	require.Equal(t, x, *y, "Object Mismatch. Input: %v", x)

	rewire, err := Polorize(*y, options...)
	require.Nil(t, err)
	require.Equal(t, wire, rewire, "Wire Mismatch. Input: %v", x)
}

func fuzzAny(val *Any, c fuzz.Continue) {
	polorizer := NewPolorizer()

	for i := 0; i <= c.Intn(2); i++ {
		switch c.Intn(8) {
		case 0:
			polorizer.PolorizeNull()
		case 1:
			polorizer.PolorizeBool(c.RandBool())
		case 2:
			polorizer.PolorizeBytes([]byte(c.RandString()))
		case 3:
			polorizer.PolorizeString(c.RandString())
		case 4:
			polorizer.PolorizeUint(c.Uint64())
		case 5:
			polorizer.PolorizeInt(c.Int63())
		case 6:
			polorizer.PolorizeFloat64(c.Float64())
		case 7:
			polorizer.PolorizeFloat32(c.Float32())
		}
	}

	*val = polorizer.Bytes()
}

func fuzzRaw(val *Raw, c fuzz.Continue) {
	var anybytes Any

	c.Fuzz(&anybytes)

	if anybytes == nil {
		anybytes = Any{0}
	}

	*val = Raw(anybytes)
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
	K *uint16
	L *int32
	M *int16
	N *int8
}

func TestInteger(t *testing.T) {
	f := fuzz.New()

	t.Run("Int", func(t *testing.T) {
		var x int

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint", func(t *testing.T) {
		var x uint

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Int8", func(t *testing.T) {
		var x int8

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint8", func(t *testing.T) {
		var x uint8

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Int16", func(t *testing.T) {
		var x int16

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint16", func(t *testing.T) {
		var x uint16

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Int32", func(t *testing.T) {
		var x int32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint32", func(t *testing.T) {
		var x uint32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Int64", func(t *testing.T) {
		var x int64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint64", func(t *testing.T) {
		var x uint64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("IntegerObject", func(t *testing.T) {
		var x IntegerObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
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
			testSerialization(t, x)
		}
	})

	t.Run("BoolObject", func(t *testing.T) {
		var x BoolObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
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

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Bytes", func(t *testing.T) {
		var x []byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Bytes-Packed", func(t *testing.T) {
		var x []byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x, PackedBytes())
		}
	})

	t.Run("WordObject", func(t *testing.T) {
		var x WordObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})
}

type FloatObject struct {
	A float32
	B float32
	C float64
	D float64
	E *float32
	F *float64
}

func TestFloat(t *testing.T) {
	f := fuzz.New().Funcs(
		func(float *float32, c fuzz.Continue) {
			if c.Intn(1000) == 0 {
				*float = 0
			} else {
				*float = c.Float32()
			}
		},
		func(float *float64, c fuzz.Continue) {
			if c.Intn(1000) == 0 {
				*float = 0
			} else {
				*float = c.Float64()
			}
		},
	)

	t.Run("Float32", func(t *testing.T) {
		var x float32

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Float64", func(t *testing.T) {
		var x float64

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("FloatObject", func(t *testing.T) {
		var x FloatObject

		for i := 0; i < 100000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
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
	L [2][]*uint32
	M [][10]*string
}

func TestSequence(t *testing.T) {
	f := fuzz.New().NumElements(0, 8).NilChance(0.3)

	t.Run("String Slice", func(t *testing.T) {
		var x []string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Uint64 Slice", func(t *testing.T) {
		var x []uint64

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Map Slice", func(t *testing.T) {
		var x []map[string]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Double String Slice", func(t *testing.T) {
		var x [][]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Double Byte Slice", func(t *testing.T) {
		var x [][]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Array Slice", func(t *testing.T) {
		var x [][8]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("WordObject Slice", func(t *testing.T) {
		var x []WordObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("String Array", func(t *testing.T) {
		var x [4]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Byte Array", func(t *testing.T) {
		var x [32]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Byte Array Packed", func(t *testing.T) {
		var x [32]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x, PackedBytes())
		}
	})

	t.Run("Map Array", func(t *testing.T) {
		var x [2]map[uint64]bool

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Double Float Array", func(t *testing.T) {
		var x [4][4]float32

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Slice Array", func(t *testing.T) {
		var x [4][]string

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Bytes Array", func(t *testing.T) {
		var x [4][]byte

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("IntegerObject Array", func(t *testing.T) {
		var x [6]IntegerObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("SequenceObject", func(t *testing.T) {
		var x SequenceObject

		for i := 0; i < 1000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
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
	K map[string]*string
}

func TestMapping(t *testing.T) {
	f := fuzz.New().NumElements(0, 8)

	t.Run("String Map", func(t *testing.T) {
		var x map[string]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Integer Map", func(t *testing.T) {
		var x map[int32]float32

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Array Map", func(t *testing.T) {
		var x map[[2]string]string

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Double Map", func(t *testing.T) {
		var x map[string]map[uint64]bool

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Bytes Map", func(t *testing.T) {
		var x map[[32]byte][]byte

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Pointer Map", func(t *testing.T) {
		var x map[string]*uint8

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Array Keyed Map", func(t *testing.T) {
		var err error

		_, err = Polorize(map[[3]uint64]string{{10, 12, 12}: "foo", {10, 11, 11}: "boo"})
		require.Nil(t, err)

		_, err = Polorize(map[[3]int64]string{{10, 12, 12}: "foo", {10, 11, 11}: "boo"})
		require.Nil(t, err)

		_, err = Polorize(map[[3]float32]string{{10, 12, 12}: "foo", {10, 11, 11}: "boo"})
		require.Nil(t, err)
	})

	t.Run("MapObject", func(t *testing.T) {
		var x MapObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})
}

type PointerObject struct {
	A *WordObject
	B *IntegerObject
	C *FloatObject
	D *string
	E *uint8
}

func TestPointer(t *testing.T) {
	f := fuzz.New()

	var x PointerObject

	for i := 0; i < 1000; i++ {
		f.Fuzz(&x)
		testSerialization(t, x)
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
		testSerialization(t, x)
	}
}

type BigObject struct {
	A big.Int
	B big.Int
	C *big.Int
}

func TestBig(t *testing.T) {
	f := fuzz.New().NilChance(0.2).Funcs(func(bignum **big.Int, c fuzz.Continue) {
		switch c.Intn(10) {
		case 0:
			*bignum = nil
		case 1:
			*bignum = big.NewInt(0)
		case 2, 3, 4, 5:
			*bignum = big.NewInt(c.Int63())
		case 6, 7, 8, 9:
			*bignum = new(big.Int).Neg(big.NewInt(c.Int63()))
		}
	})

	t.Run("Big Int", func(t *testing.T) {
		var x *big.Int

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("BigObject", func(t *testing.T) {
		var x BigObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})
}

type AnyObject struct {
	A Any
	B Any
	C Any
}

func TestRawAny(t *testing.T) {
	f := fuzz.New().Funcs(fuzzAny, fuzzRaw)

	t.Run("Raw", func(t *testing.T) {
		var x Raw

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Any Object", func(t *testing.T) {
		var x AnyObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("Any Decode", func(t *testing.T) {
		type Object struct {
			A, B, C int
		}

		var x Object

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)

			wire, err := Polorize(x)
			require.Nil(t, err)

			y := new(AnyObject)
			err = Depolorize(y, wire)

			require.Nil(t, err, "Unexpected Error. Input: %v", x)

			wireA, _ := Polorize(x.A)
			require.Equal(t, Any(wireA), y.A)

			wireB, _ := Polorize(x.B)
			require.Equal(t, Any(wireB), y.B)

			wireC, _ := Polorize(x.C)
			require.Equal(t, Any(wireC), y.C)
		}
	})
}

func TestDocument(t *testing.T) {
	f := fuzz.New().NilChance(0.01).Funcs(fuzzRaw)

	var x Document

	for i := 0; i < 10000; i++ {
		f.Fuzz(&x)
		testSerialization(t, x)
	}
}

type SimpleInterface interface{}

type InterfaceObject struct {
	A SimpleInterface
	B SimpleInterface
	C SimpleInterface
}

func TestInterface(t *testing.T) {
	var (
		x1, x2, x3 string
		x          InterfaceObject
	)

	x = InterfaceObject{x1, x2, x3}
	wire, err := Polorize(x)

	require.Nil(t, wire)
	require.EqualError(t, err, "incompatible value error: unsupported type: polo.SimpleInterface [interface]")
}

func TestUnsupported(t *testing.T) {
	var err error

	t.Run("Channels", func(t *testing.T) {
		_, err = Polorize(make(chan string))
		require.EqualError(t, err, "incompatible value error: unsupported type: chan string [chan]")

		err = Depolorize(new(chan string), []byte{0})
		require.EqualError(t, err, "incompatible value error: unsupported type: chan string [chan]")
	})

	t.Run("Functions", func(t *testing.T) {
		_, err = Polorize(new(func(string)))
		require.EqualError(t, err, "incompatible value error: unsupported type: func(string) [func]")
	})

	t.Run("Slice of Unsupported Types", func(t *testing.T) {
		_, err = Polorize(make([]func(string), 2))
		require.EqualError(t, err, "incompatible value error: unsupported type: func(string) [func]")
	})

	t.Run("Array of Unsupported Types", func(t *testing.T) {
		_, err = Polorize(new([2]chan string))
		require.EqualError(t, err, "incompatible value error: unsupported type: chan string [chan]")
	})

	t.Run("Map with Unsupported Type for Keys", func(t *testing.T) {
		_, err = Polorize(map[SimpleInterface]string{"foo": "bar", "boo": "far"})
		require.EqualError(t, err, "incompatible value error: unsupported type: polo.SimpleInterface [interface]")

		_, err = Polorize(map[[2]SimpleInterface]string{{"foo", "fon"}: "bar", {"boo", "bon"}: "far"})
		require.EqualError(t, err, "incompatible value error: unsupported type: polo.SimpleInterface [interface]")
	})

	t.Run("Map with Unsupported Type for Values", func(t *testing.T) {
		_, err = Polorize(map[string]chan int{"foo": make(chan int)})
		require.EqualError(t, err, "incompatible value error: unsupported type: chan int [chan]")
	})
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
		testSerialization(t, x)
	}
}

func TestEmpty(t *testing.T) {
	x := struct{}{}
	testSerialization(t, x)
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
		assert.EqualError(t, err, "incompatible value error: unsupported type: cannot encode untyped nil")
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
		assert.Equal(t, [4]string{}, *x)
	})

	t.Run("Map", func(t *testing.T) {
		x := new(map[string]string)
		err = Depolorize(x, wire)

		var nilmap map[string]string

		require.Nil(t, err)
		assert.Equal(t, nilmap, *x)
	})

	t.Run("Any", func(t *testing.T) {
		x := new(Any)
		err = Depolorize(x, wire)

		require.NoError(t, err)
		require.Equal(t, Any{0}, *x)
	})

	t.Run("Raw", func(t *testing.T) {
		x := new(Raw)
		err = Depolorize(x, wire)
		require.EqualError(t, err, "incompatible wire: unexpected wiretype 'null'. expected one of: {raw}")
	})
}

func TestExcessIntegerData(t *testing.T) {
	tests := []struct {
		name   string
		wire   []byte
		object any
		size   int
		signed bool
	}{
		{
			"8-bit unsigned",
			[]byte{3, 255, 255},
			new(uint8), 8, false,
		},
		{
			"8-bit signed",
			[]byte{3, 255, 255},
			new(int8), 8, true,
		},
		{
			"16-bit unsigned",
			[]byte{3, 255, 255, 255},
			new(uint16), 16, false,
		},
		{
			"16-bit signed",
			[]byte{3, 255, 255, 255},
			new(int16), 16, true,
		},
		{
			"32-bit unsigned",
			[]byte{3, 255, 255, 255, 255, 255, 255},
			new(uint32), 32, false,
		},
		{
			"32-bit signed",
			[]byte{3, 255, 255, 255, 255, 255, 255},
			new(int32), 32, true,
		},
		{
			"64-bit unsigned",
			[]byte{3, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(uint64), 64, false,
		},
		{
			"64-bit signed",
			[]byte{3, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(int64), 64, true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.object, test.wire)
			if test.signed {
				assert.EqualError(t, err, fmt.Sprintf("incompatible value error: excess data for %v-bit integer", test.size))
			}
		})
	}
}

func TestMalformedFloatData(t *testing.T) {
	tests := []struct {
		name   string
		wire   []byte
		object any
		err    error
	}{
		{
			"malformed for 32-bit",
			[]byte{7, 111, 114, 97, 110, 103, 101, 103, 101, 120},
			new(float32),
			IncompatibleWireError{"malformed data for 32-bit float"},
		},
		{
			"malformed for 64-bit",
			[]byte{7, 111, 114, 97},
			new(float64),
			IncompatibleWireError{"malformed data for 64-bit float"},
		},
		{
			"NaN 32-bit",
			[]byte{7, 255, 255, 0, 0},
			new(float32),
			IncompatibleValueError{"float is not a number"},
		},
		{
			"NaN 64-bit",
			[]byte{7, 255, 255, 0, 0, 0, 0, 0, 0},
			new(float64),
			IncompatibleValueError{"float is not a number"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.object, test.wire)
			assert.EqualError(t, err, test.err.Error())
		})
	}
}

//nolint:lll
func TestIncompatibleWireType(t *testing.T) {
	tests := []struct {
		name    string
		wire    []byte
		object  any
		options []EncodingOptions
		err     error
	}{
		{
			"WireTrue -> float32",
			[]byte{2},
			new(float32),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'true'. expected one of: {null, float}"},
		},
		{
			"WireNegInt -> float64",
			[]byte{4, 1},
			new(float64),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'negint'. expected one of: {null, float}"},
		},
		{
			"WireFloat -> string",
			[]byte{7, 111, 114, 97, 110, 103, 101},
			new(string),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'float'. expected one of: {null, word}"},
		},
		{
			"WirePosInt -> bool",
			[]byte{3, 44},
			new(bool),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'posint'. expected one of: {null, true, false}"},
		},
		{
			"WireTrue -> uint64",
			[]byte{2},
			new(uint64),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'true'. expected one of: {null, posint}"},
		},
		{
			"WireNegInt -> []string",
			[]byte{4, 45, 22},
			new([]string),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'negint'. expected one of: {null, pack}"},
		},
		{
			"WireNegInt -> []byte",
			[]byte{4, 45, 22},
			new([]byte),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'negint'. expected one of: {null, word}"},
		},
		{
			"WirePosInt -> [4]string",
			[]byte{3, 45, 22},
			new([4]string),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'posint'. expected one of: {null, pack}"},
		},
		{
			"WireRaw -> map[string]string",
			[]byte{5, 45, 22},
			new(map[string]string),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'raw'. expected one of: {null, pack}"},
		},
		{
			"WireFloat -> big.Int",
			[]byte{7, 45, 22, 56, 34},
			new(big.Int),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'float'. expected one of: {null, posint, negint}"},
		},
		{
			"WirePosInt -> struct",
			[]byte{3, 45, 22},
			new(*IntegerObject),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'posint'. expected one of: {null, pack}"},
		},
		{
			"struct field: WirePosInt -> string",
			[]byte{14, 95, 3, 3, 3, 3, 3},
			&WordObject{},
			[]EncodingOptions{},
			IncompatibleWireError{"struct field [polo.WordObject.A <string>]: incompatible wire: unexpected wiretype 'posint'. expected one of: {null, word}"},
		},
		{
			"struct field: WireFalse -> int",
			[]byte{14, 95, 1, 0, 0, 0, 0},
			&IntegerObject{},
			[]EncodingOptions{},
			IncompatibleWireError{"struct field [polo.IntegerObject.A <int>]: incompatible wire: unexpected wiretype 'false'. expected one of: {null, posint, negint}"},
		},
		{
			"struct from document with DocStruct disabled",
			[]byte{13, 47, 6, 21, 65, 1},
			&IntegerObject{},
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'document'. expected one of: {null, pack}"},
		},
		{
			"document field: WireFloat -> string",
			[]byte{13, 95, 7, 53, 86, 133, 1, 102, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114},
			new(Document),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'float'. expected one of: {null, word}"},
		},
		{
			"WireNegInt -> []uint64",
			[]byte{14, 31, 4, 132},
			new([]uint64),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'negint'. expected one of: {null, posint}"},
		},
		{
			"WireWord -> []byte",
			[]byte{3, 76},
			new([]byte),
			[]EncodingOptions{PackedBytes()},
			IncompatibleWireError{"unexpected wiretype 'posint'. expected one of: {null, word, pack}"},
		},
		{
			"string map from document with DocStringMaps disabled",
			[]byte{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
			new(map[string]int),
			[]EncodingOptions{},
			IncompatibleWireError{"unexpected wiretype 'document'. expected one of: {null, pack}"},
		},
		{
			"string map from document with DocStringMaps enabled",
			[]byte{13, 47, 5, 53, 98, 111, 111, 3, 1, 44},
			new(map[string]int),
			[]EncodingOptions{DocStringMaps()},
			IncompatibleWireError{"unexpected wiretype 'raw'. expected one of: {null, word}"},
		},
		{
			"string map from document with DocStringMaps enabled",
			[]byte{13, 47, 6, 53, 98, 111, 111, 6, 145, 12},
			new(map[string]int),
			[]EncodingOptions{DocStringMaps()},
			IncompatibleWireError{"unexpected wiretype 'word'. expected one of: {null, posint, negint}"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.object, test.wire, test.options...)
			assert.EqualError(t, err, test.err.Error())
		})
	}
}

func TestMalformed(t *testing.T) {
	tests := []struct {
		name    string
		wire    []byte
		object  any
		options []EncodingOptions
		err     error
	}{
		{
			"non-pointer target",
			[]byte{14, 78, 3, 3, 3, 3},
			IntegerObject{},
			[]EncodingOptions{},
			ErrObjectNotPtr,
		},
		{
			"signed integer overflow",
			[]byte{3, 255, 255, 255, 255, 255, 255, 255, 255},
			new(int64),
			[]EncodingOptions{},
			IncompatibleValueError{"overflow for signed integer"},
		},
		{
			"excess 8-bit data",
			[]byte{14, 47, 3, 19, 102, 45, 67},
			new([]byte),
			[]EncodingOptions{PackedBytes()},
			IncompatibleValueError{"excess data for 8-bit integer"},
		},
		{
			"varint overflow",
			[]byte{255, 128, 128, 128, 128, 128, 128, 128, 128, 127, 93, 3, 3, 3, 3, 3},
			&IntegerObject{},
			[]EncodingOptions{},
			MalformedTagError{errVarintOverflow.Error()},
		},
		{
			"varint premature termination",
			[]byte{14, 47, 6, 134},
			new([][2]byte),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"mismatched data length for byte array",
			[]byte{6, 255, 255, 255},
			new([2]byte),
			[]EncodingOptions{},
			IncompatibleWireError{"mismatched data length for byte array"},
		},
		{
			"missing load tag for pack wire",
			[]byte{14, 78, 3, 3, 3, 3},
			&IntegerObject{},
			[]EncodingOptions{},
			errors.New("load convert fail: missing load tag"),
		},
		{
			"malformed load tag for pack wire",
			[]byte{14, 255, 128, 128, 128, 128, 128, 128, 128, 128, 127, 3, 3, 3, 3, 3},
			&IntegerObject{},
			[]EncodingOptions{},
			errors.New("load convert fail: malformed tag: varint overflows 64-bit integer"),
		},
		{
			"insufficient data for struct decode",
			[]byte{14, 79, 3, 3, 3, 3, 0, 0, 0, 0},
			&IntegerObject{},
			[]EncodingOptions{},
			IncompatibleWireError{fmt.Sprintf("struct field [polo.IntegerObject.E <int64>]: %v", ErrInsufficientWire)},
		},
		{
			"malformed varint when decoding document",
			[]byte{13, 175},
			new(Document),
			[]EncodingOptions{},
			errors.New("load convert fail: malformed tag: varint terminated prematurely"),
		},
		{
			"malformed varint when decoding slice",
			[]byte{14, 175},
			new([]string),
			[]EncodingOptions{},
			errors.New("load convert fail: malformed tag: varint terminated prematurely"),
		},
		{
			"malformed varint when decoding array",
			[]byte{14, 175},
			new([2]float32),
			[]EncodingOptions{},
			errors.New("load convert fail: malformed tag: varint terminated prematurely"),
		},
		{
			"malformed varint when decoding mapping",
			[]byte{14, 175},
			new(map[uint64]string),
			[]EncodingOptions{},
			errors.New("load convert fail: malformed tag: varint terminated prematurely"),
		},
		{
			"malformed varint when decoding packed bytes",
			[]byte{14, 175},
			new([]byte),
			[]EncodingOptions{PackedBytes()},
			errors.New("load convert fail: malformed tag: varint terminated prematurely"),
		},
		{
			"insufficient data for document decode",
			[]byte{13, 63, 6, 53, 86, 101, 97, 114, 3, 123, 102, 111, 111, 6, 98, 97, 114},
			new(Document),
			[]EncodingOptions{},
			errors.New("insufficient data in wire for decode"),
		},
		{
			"load tag varint terminated prematurely: string array",
			[]byte{14, 47, 6, 230, 102, 111, 111},
			new([]string),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: bytes slice",
			[]byte{14, 47, 6, 230, 1, 1, 1},
			new([][]byte),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: bytes array",
			[]byte{14, 47, 6, 230, 1, 1, 1},
			new([2][]byte),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: string map",
			[]byte{14, 79, 6, 54, 102, 230, 1, 1, 1, 1, 1, 1},
			new(map[string]string),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: bytes",
			[]byte{14, 47, 7, 231, 102, 111, 111},
			new([]byte),
			[]EncodingOptions{PackedBytes()},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: string map",
			[]byte{14, 63, 6, 54, 230, 1, 1, 1, 1, 1, 1},
			new(map[string]string),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: float32 slice",
			[]byte{14, 47, 7, 231, 102, 111, 111},
			new([]float32),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: float64 slice",
			[]byte{14, 47, 7, 231, 102, 111, 111, 231, 102, 111, 111},
			new([]float64),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: big int slice",
			[]byte{14, 47, 5, 165, 1, 44, 250},
			new([]big.Int),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: uint64 slice",
			[]byte{14, 47, 3, 131},
			new([]uint64),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: custom struct",
			[]byte{14, 47, 6, 203},
			new(CustomEncodeObject),
			[]EncodingOptions{},
			MalformedTagError{"varint terminated prematurely"},
		},
		{
			"load tag varint terminated prematurely: doc string map",
			[]byte{13, 47, 6, 53, 98, 111, 111, 131},
			new(map[string]int),
			[]EncodingOptions{DocStringMaps()},
			MalformedTagError{"varint terminated prematurely"},
		},
	}

	for tno, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.object, test.wire, test.options...)
			assert.EqualError(t, err, test.err.Error(), "[%v] Input: %v", tno, test.wire)
		})
	}
}

type CustomEncodeObject struct {
	A string
	B int32
	C []string
	D map[string]string
	E float64
}

func (object CustomEncodeObject) Polorize() (*Polorizer, error) {
	polorizer := NewPolorizer()

	polorizer.PolorizeString(object.A)
	polorizer.PolorizeInt(int64(object.B))

	if object.C != nil {
		C := NewPolorizer()
		for _, elem := range object.C {
			C.PolorizeString(elem)
		}

		polorizer.PolorizePacked(C)
	} else {
		polorizer.PolorizeNull()
	}

	if object.D != nil {
		keys := make([]string, 0, len(object.D))
		for key := range object.D {
			keys = append(keys, key)
		}

		sort.Strings(keys)

		D := NewPolorizer()

		for _, key := range keys {
			D.PolorizeString(key)
			D.PolorizeString(object.D[key])
		}

		polorizer.PolorizePacked(D)
	} else {
		polorizer.PolorizeNull()
	}

	polorizer.PolorizeFloat64(object.E)

	return polorizer, nil
}

func (object *CustomEncodeObject) Depolorize(depolorizer *Depolorizer) (err error) {
	if depolorizer.IsNull() {
		return nil
	}

	depolorizer, err = depolorizer.Unpacked()
	if err != nil {
		return err
	}

	object.A, err = depolorizer.DepolorizeString()
	if err != nil {
		return err
	}

	object.B, err = depolorizer.DepolorizeInt32()
	if err != nil {
		return err
	}

	if depolorizer.IsNull() {
		object.C = nil
		_ = depolorizer.DepolorizeNull()
	} else {
		c, err := depolorizer.DepolorizePacked()
		if err != nil {
			return err
		}

		object.C = make([]string, 0, 5)

		for !c.Done() {
			element, err := c.DepolorizeString()
			if err != nil {
				return err
			}

			object.C = append(object.C, element)
		}
	}

	if depolorizer.IsNull() {
		object.D = nil
		_ = depolorizer.DepolorizeNull()
	} else {
		d, err := depolorizer.DepolorizePacked()
		if err != nil {
			return err
		}

		object.D = make(map[string]string)

		for !d.Done() {
			key, err := d.DepolorizeString()
			if err != nil {
				return err
			}

			val, err := d.DepolorizeString()
			if err != nil {
				return err
			}

			object.D[key] = val
		}
	}

	if object.E, err = depolorizer.DepolorizeFloat64(); err != nil {
		return err
	}

	return nil
}

func TestCustomEncoding(t *testing.T) {
	t.Run("CustomEncodeObject", func(t *testing.T) {
		f := fuzz.New()

		var x CustomEncodeObject

		for i := 0; i < 10000; i++ {
			f.Fuzz(&x)
			testSerialization(t, x)
		}
	})

	t.Run("BadCustomObject", func(t *testing.T) {
		object := BadCustomObject{make(chan string)}
		bytes, err := Polorize(object)

		assert.Nil(t, bytes)
		assert.EqualError(t, err, "incompatible value error: unsupported type: chan string [chan]")
	})
}

type BadCustomObject struct {
	A chan string
}

func (object BadCustomObject) Polorize() (*Polorizer, error) {
	polorizer := NewPolorizer()
	if err := polorizer.Polorize(object.A); err != nil {
		return nil, err
	}

	return polorizer, nil
}

func TestUnSettableValue(t *testing.T) {
	object := BoolObject{true, false}
	wire, err := Polorize(object)
	require.NoError(t, err)

	var decoded *BoolObject
	err = Depolorize(decoded, wire)
	require.EqualError(t, err, "object is not settable")
}

func TestPointerAlias(t *testing.T) {
	type (
		Alias1 string
		Alias2 uint16
	)

	type Object struct {
		A *Alias1
		B *Alias2
	}

	object := Object{nil, nil}
	testSerialization(t, object)
}

func TestSpecialBytes(t *testing.T) {
	t.Run("Nil Bytes", func(t *testing.T) {
		wire := []byte{0}

		var x [10]byte

		err := Depolorize(&x, wire)
		require.NoError(t, err)
		require.Equal(t, x, [10]byte{})
	})

	t.Run("Bytes32", func(t *testing.T) {
		t.Run("Excess", func(t *testing.T) {
			depolorizer, err := NewDepolorizer([]byte{
				6, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
				255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
			})
			require.NoError(t, err)

			_, err = depolorizer.DepolorizeBytes32()
			require.EqualError(t, err, "incompatible value error: excess data for 32-byte array")
		})

		t.Run("Invalid", func(t *testing.T) {
			depolorizer, err := NewDepolorizer([]byte{3, 56})
			require.NoError(t, err)

			_, err = depolorizer.DepolorizeBytes32()
			require.EqualError(t, err, "incompatible wire: unexpected wiretype 'posint'. expected one of: {null, word}")
		})

		t.Run("Insufficient", func(t *testing.T) {
			depolorizer := Depolorizer{data: readbuffer{}, done: true}

			_, err := depolorizer.DepolorizeBytes32()
			require.EqualError(t, err, "insufficient data in wire for decode")
		})

		t.Run("Perfect", func(t *testing.T) {
			depolorizer, err := NewDepolorizer([]byte{
				6, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
				255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
			})
			require.NoError(t, err)

			decoded, err := depolorizer.DepolorizeBytes32()
			require.NoError(t, err)
			require.Equal(t, [32]byte{
				255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
				255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
			}, decoded,
			)
		})
	})
}
