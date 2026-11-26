package polo

import (
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression tests for inputs that used to crash the decoder (or encoder) instead of returning an error.
// All of the decoder cases are reachable from untrusted wire data.

// hexWire decodes a hex string into a wire
func hexWire(tb testing.TB, s string) []byte {
	tb.Helper()

	wire, err := hex.DecodeString(s)
	require.NoError(tb, err)

	return wire
}

// hugeLoadWire returns a WirePack whose load tag declares a head of 2^k bytes that is not in the wire
func hugeLoadWire(k uint) []byte {
	return binary.AppendUvarint([]byte{byte(WirePack)}, (uint64(1)<<k)<<4|uint64(WireLoad))
}

// nestedPackWire wraps the given wire into levels of packs. Every
// level is a WirePack whose only element is the level below it.
func nestedPackWire(seed []byte, levels int) []byte {
	wire := seed

	for i := 0; i < levels; i++ {
		nested := make([]byte, 0, len(wire)+2)
		// pack tag, load tag (head of 1 byte), head: a pack tag at offset 0
		nested = append(nested, byte(WirePack), 1<<4|byte(WireLoad), byte(WirePack))
		// body: the level below without its tag
		nested = append(nested, wire[1:]...)
		wire = nested
	}

	return wire
}

var (
	emptyPackWire = []byte{byte(WirePack), byte(WireLoad)}                        // pack with no elements
	nullPackWire  = []byte{byte(WirePack), 1<<4 | byte(WireLoad), 0}              // pack with a single null element
	wordPackWire  = []byte{byte(WirePack), 1<<4 | byte(WireLoad), byte(WireWord)} // pack with a single (empty) word
)

func TestDepolorize_LoadLengthExceedsData(t *testing.T) {
	// The load tag declares a head length that is not backed by data in the wire. Decoding
	// must fail without first allocating the declared length (fatal for large declarations)
	wires := map[string][]byte{
		"2^47 head (0e8f80808080808004)": hexWire(t, "0e8f80808080808004"),
		"2^50 head":                      hugeLoadWire(50),
		"2^36 head":                      hugeLoadWire(36),
		"2^20 head":                      hugeLoadWire(20),
		"max uint64 tag":                 {14, 255, 255, 255, 255, 255, 255, 255, 255, 255, 1},
		"1 byte short":                   {14, 47, 3},
	}

	targets := map[string]func() any{
		"slice":  func() any { return new([]uint64) },
		"array":  func() any { return new([2]uint64) },
		"map":    func() any { return new(map[string]uint64) },
		"struct": func() any { return new(struct{ A uint64 }) },
		"any":    func() any { return new(Any) },
	}

	for wireName, wire := range wires {
		for targetName, target := range targets {
			t.Run(wireName+"/"+targetName, func(t *testing.T) {
				object := target()
				err := Depolorize(object, wire)

				// Any accepts the wire as is, every other target has to unpack it
				if _, isAny := object.(*Any); isAny {
					require.NoError(t, err)

					return
				}

				require.EqualError(t, err, "load convert fail: missing head: insufficient data in reader")
			})
		}
	}

	t.Run("manual decoding", func(t *testing.T) {
		depolorizer, err := NewDepolorizer(hexWire(t, "0e8f80808080808004"))
		require.NoError(t, err)

		_, err = depolorizer.DepolorizePacked()
		require.EqualError(t, err, "load convert fail: missing head: insufficient data in reader")
	})
}

func TestDepolorize_ElementOffsetOutOfBounds(t *testing.T) {
	tests := []struct {
		name   string
		wire   []byte
		target any
	}{
		{"offset past body", hexWire(t, "0e2f0313"), new([]uint64)},
		{"offset before current element", hexWire(t, "0e3f03130305"), new([]uint64)},
		{"last element past body", hexWire(t, "0e1f33"), new([]uint64)},
		{"first element past body", hexWire(t, "0e1f33"), new([2]uint64)},
		{"nested pack", hexWire(t, "0e1f0e1f33"), new([][]uint64)},
		{"map", hexWire(t, "0e2f0633"), new(map[string]uint64)},
		{"struct", hexWire(t, "0e2f0633"), new(struct {
			A string
			B uint64
		})},
		{"doc wire", hexWire(t, "0d2f0635"), new(Document)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Depolorize(test.target, test.wire)
			require.ErrorContains(t, err, "malformed tag: element offset out of bounds")

			var tagErr MalformedTagError

			require.ErrorAs(t, err, &tagErr)
		})
	}

	t.Run("manual decoding", func(t *testing.T) {
		depolorizer, err := NewDepolorizer(hexWire(t, "0e2f0313"))
		require.NoError(t, err)

		pack, err := depolorizer.DepolorizePacked()
		require.NoError(t, err)

		_, err = pack.DepolorizeUint64()
		require.EqualError(t, err, "malformed tag: element offset out of bounds")

		depolorizer, err = NewDepolorizer(hexWire(t, "0e1f33"))
		require.NoError(t, err)

		_, err = depolorizer.DepolorizePacked()
		require.EqualError(t, err, "malformed tag: element offset out of bounds")
	})
}

func TestDepolorize_NullMapKey(t *testing.T) {
	type Key struct{ A uint64 }

	t.Run("byte array key", func(t *testing.T) {
		// pack of a null key and a null value: a byte array key cannot be nil
		var nulls map[[32]byte][32]byte

		require.ErrorIs(t, Depolorize(&nulls, hexWire(t, "0e2f0000")), ErrNullValue)

		// pack of a null key and the integer 0
		var zero map[[32]byte]uint64

		require.ErrorIs(t, Depolorize(&zero, hexWire(t, "0e2f0003")), ErrNullValue)
	})

	t.Run("struct key", func(t *testing.T) {
		// The encoder cannot order struct keys, so the decoder does not accept them either
		var object map[Key]Key

		err := Depolorize(&object, hexWire(t, "0e2f0000"))
		require.EqualError(t, err, "incompatible value error: unsupported type: polo.Key [struct]")
	})

	t.Run("pointer key", func(t *testing.T) {
		// pack of a null key and the word "nil"
		var object map[*uint64]string

		err := Depolorize(&object, hexWire(t, "0e2f00066e696c"))
		require.EqualError(t, err, "incompatible value error: unsupported type: *uint64 [ptr]")
	})
}

func TestDepolorize_NamedByteElements(t *testing.T) {
	type Flag uint8

	type Flags []Flag

	type FlagArray [4]Flag

	t.Run("slice", func(t *testing.T) {
		wire, err := Polorize(Flags{1, 2, 3})
		require.NoError(t, err)

		var flags Flags

		require.NoError(t, Depolorize(&flags, wire))
		assert.Equal(t, Flags{1, 2, 3}, flags)

		// A null wire decodes into a nil slice
		flags = Flags{9}
		require.NoError(t, Depolorize(&flags, []byte{0}))
		assert.Nil(t, flags)
	})

	t.Run("array", func(t *testing.T) {
		wire, err := Polorize(FlagArray{1, 2, 3, 4})
		require.NoError(t, err)

		var flags FlagArray

		require.NoError(t, Depolorize(&flags, wire))
		assert.Equal(t, FlagArray{1, 2, 3, 4}, flags)

		// A null wire cannot be decoded into an array
		var null FlagArray

		require.ErrorIs(t, Depolorize(&null, []byte{0}), ErrNullValue)
	})

	t.Run("struct fields", func(t *testing.T) {
		type Object struct {
			S Flags
			A FlagArray
		}

		object := Object{Flags{7}, FlagArray{1, 2, 3, 4}}

		wire, err := Polorize(object)
		require.NoError(t, err)

		var decoded Object

		require.NoError(t, Depolorize(&decoded, wire))
		assert.Equal(t, object, decoded)
	})
}

func TestPolorize_UnorderableMapKeys(t *testing.T) {
	type Key struct{ A uint64 }

	a, b := uint64(1), uint64(2)

	unorderable := map[string]any{
		"struct keys":   map[Key]uint64{{1}: 1, {2}: 2},
		"single struct": map[Key]uint64{{1}: 1},
		"pointer keys":  map[*uint64]uint64{&a: 1, &b: 2},
		"channel keys":  map[chan int]uint64{make(chan int): 1},
		"interface key": map[any]uint64{nil: 1, 2: 2},
	}

	for name, object := range unorderable {
		t.Run(name, func(t *testing.T) {
			_, err := Polorize(object)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported type")
		})
	}

	t.Run("bool array keys", func(t *testing.T) {
		object := map[[2]bool]uint64{{true, false}: 1, {false, true}: 2, {false, false}: 3, {true, true}: 4}

		wire, err := Polorize(object)
		require.NoError(t, err)

		// Key ordering is deterministic
		for i := 0; i < 10; i++ {
			again, err := Polorize(object)
			require.NoError(t, err)
			require.Equal(t, wire, again)
		}

		var decoded map[[2]bool]uint64

		require.NoError(t, Depolorize(&decoded, wire))
		assert.Equal(t, object, decoded)
	})
}

// recursiveDepolorizable is a custom Depolorizable that recursively decodes a chain of packs
type recursiveDepolorizable struct {
	Next *recursiveDepolorizable
}

func (r *recursiveDepolorizable) Depolorize(depolorizer *Depolorizer) error {
	pack, err := depolorizer.DepolorizePacked()
	if err != nil {
		return err
	}

	if pack.IsNull() {
		return pack.DepolorizeNull()
	}

	r.Next = new(recursiveDepolorizable)

	return pack.Depolorize(r.Next)
}

// docNode is a DocumentStruct that holds a chain of documents
type docNode struct{ Next *docNode }

func (docNode) PoloDocument() {}

func TestDepolorize_MaxDepth(t *testing.T) {
	type Tree []Tree

	type Node struct{ Next *Node }

	t.Run("limit", func(t *testing.T) {
		var tree Tree

		// rs-polo has the same limit. Change both together
		require.Equal(t, 1000, MaxDepth)

		// nestedPackWire(seed, n) is n+1 packs deep
		require.NoError(t, Depolorize(&tree, nestedPackWire(emptyPackWire, MaxDepth-1)))

		err := Depolorize(&tree, nestedPackWire(emptyPackWire, MaxDepth))
		require.ErrorIs(t, err, ErrMaxDepthExceeded)
	})

	t.Run("encoder has no limit", func(t *testing.T) {
		tree := Tree{}
		for range MaxDepth {
			tree = Tree{tree}
		}

		wire, err := Polorize(tree)
		require.NoError(t, err)

		var decoded Tree

		require.ErrorIs(t, Depolorize(&decoded, wire), ErrMaxDepthExceeded)
	})

	t.Run("recursive struct", func(t *testing.T) {
		var node Node

		require.NoError(t, Depolorize(&node, nestedPackWire(nullPackWire, MaxDepth-1)))
		require.NotNil(t, node.Next.Next)

		err := Depolorize(&node, nestedPackWire(nullPackWire, MaxDepth))
		require.ErrorIs(t, err, ErrMaxDepthExceeded)
	})

	t.Run("document structs", func(t *testing.T) {
		// Every level is a document with the level below as its "Next" value
		wire, err := Polorize(docNode{})
		require.NoError(t, err)

		for i := 0; i < MaxDepth-1; i++ {
			wire = Document{"Next": wire}.Bytes()
		}

		var node docNode

		require.NoError(t, Depolorize(&node, wire))
		require.NotNil(t, node.Next.Next)

		err = Depolorize(&node, Document{"Next": wire}.Bytes())
		require.ErrorIs(t, err, ErrMaxDepthExceeded)
	})

	t.Run("manual decoding", func(t *testing.T) {
		depolorizer, err := NewDepolorizer(nestedPackWire(emptyPackWire, MaxDepth+2))
		require.NoError(t, err)

		for i := 0; i < MaxDepth; i++ {
			depolorizer, err = depolorizer.DepolorizePacked()
			require.NoError(t, err)
		}

		_, err = depolorizer.DepolorizePacked()
		require.ErrorIs(t, err, ErrMaxDepthExceeded)
	})

	t.Run("depolorizable", func(t *testing.T) {
		var object recursiveDepolorizable

		require.NoError(t, Depolorize(&object, nestedPackWire(nullPackWire, MaxDepth-1)))
		require.NotNil(t, object.Next.Next)

		err := Depolorize(&object, nestedPackWire(nullPackWire, MaxDepth))
		require.ErrorIs(t, err, ErrMaxDepthExceeded)
	})
}

func TestDepolorize_NestedStructFieldError(t *testing.T) {
	type Inner struct{ A int }

	type Outer struct{ In Inner }

	type Node struct{ Next *Node }

	t.Run("message", func(t *testing.T) {
		wire, err := Polorize(struct{ In struct{ A string } }{struct{ A string }{"x"}})
		require.NoError(t, err)

		err = Depolorize(new(Outer), wire)
		require.EqualError(t, err, "incompatible wire: struct field [polo.Outer.In <polo.Inner>]: "+
			"incompatible wire: struct field [polo.Inner.A <int>]: "+
			"incompatible wire: unexpected wiretype 'word'. expected one of: {null, posint, negint}")

		// The error is retrievable as an IncompatibleWireError
		var wireErr IncompatibleWireError

		require.ErrorAs(t, err, &wireErr)
		assert.Equal(t, err.Error(), wireErr.Error())
	})

	t.Run("deep", func(t *testing.T) {
		// A chain of nodes at the nesting limit that ends with a word instead of a node
		err := Depolorize(new(Node), nestedPackWire(wordPackWire, MaxDepth-1))
		require.Error(t, err)

		prefix := "incompatible wire: struct field [polo.Node.Next <*polo.Node>]: "
		assert.True(t, strings.HasPrefix(err.Error(), prefix))
		assert.Equal(t, MaxDepth, strings.Count(err.Error(), prefix))
		assert.True(t, strings.HasSuffix(err.Error(), "unexpected wiretype 'word'. expected one of: {null, pack}"))
	})
}

// fuzzObject covers the different kinds of values the reflective decoder handles
type fuzzObject struct {
	B   bool
	S   string
	U8  uint8
	I8  int8
	U   uint64
	I   int64
	F32 float32
	F64 float64
	Big big.Int
	Ptr *fuzzObject
	Bs  []byte
	H   [32]byte
	Any Any
	Raw Raw
	Doc Document
	Us  []uint64
	Arr [2]int16
	Ms  map[string]uint64
	Mh  map[[32]byte][32]byte
	In  struct{ A, B string }
}

// FuzzDepolorizeNoPanic checks that decoding arbitrary bytes into any
// supported type returns an error instead of panicking
func FuzzDepolorizeNoPanic(f *testing.F) {
	seeds := [][]byte{
		{0},
		{3, 5},
		{6, 102, 111, 111},
		{14, 47, 6, 54, 112, 98, 98, 112, 98, 98},
		{14, 79, 6, 99, 142, 1, 111, 114, 97, 110, 103, 101, 1, 44, 63, 6, 150, 1, 116, 97},
		{13, 47, 6, 53, 98, 111, 111, 3, 1, 44},
		{14, 143, 128, 128, 128, 128, 128, 128, 4},
		{14, 47, 3, 19},
		{14, 47, 35, 3, 14, 15},
		{14, 47, 0, 0},
		{14, 31, 51},
		nestedPackWire(emptyPackWire, 4),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, wire []byte) {
		targets := []any{
			new(bool), new(string), new(uint8), new(int8), new(uint64), new(int64), new(float32), new(float64),
			new(big.Int), new([]byte), new([32]byte), new(Any), new(Raw), new(Document), new([]uint64),
			new([2]int16), new(map[string]uint64), new(map[[32]byte][32]byte), new(fuzzObject), new([]fuzzObject),
			new(map[string]fuzzObject), new(recursiveDepolorizable),
		}

		for _, target := range targets {
			_ = Depolorize(target, wire)
		}

		// Walk the wire manually: unpack every compound element recursively
		depolorizer, err := NewDepolorizer(wire)
		if err != nil {
			return
		}

		var walk func(*Depolorizer)

		walk = func(depolorizer *Depolorizer) {
			for !depolorizer.Done() {
				if depolorizer.IsNull() {
					_ = depolorizer.DepolorizeNull()

					continue
				}

				if pack, err := depolorizer.DepolorizePacked(); err == nil {
					walk(pack)
				}
			}
		}

		walk(depolorizer)
	})
}
