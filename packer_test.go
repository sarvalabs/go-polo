package polo

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//func TestPacker(t *testing.T) {
//	t.Run("Simple Struct", func(t *testing.T) {
//		type object struct {
//			A string
//			B int64
//			C float64
//			D bool
//		}
//
//		t.Run("Pack", func(t *testing.T) {
//			f := fuzz.New()
//			var x object
//
//			for i := 0; i < 1000; i++ {
//				f.Fuzz(&x)
//
//				pack := NewPacker()
//				assert.Nil(t, pack.Pack(x.A))
//				assert.Nil(t, pack.Pack(x.B))
//				assert.Nil(t, pack.Pack(x.C))
//				assert.Nil(t, pack.Pack(x.D))
//
//				wire, _ := Polorize(x)
//				assert.Equal(t, wire, pack.Bytes())
//			}
//		})
//
//		t.Run("PackWire", func(t *testing.T) {
//			f := fuzz.New()
//			var x object
//
//			for i := 0; i < 1000; i++ {
//				f.Fuzz(&x)
//
//				pack := NewPacker()
//
//				a, _ := Polorize(x.A)
//				assert.Nil(t, pack.PackWire(a))
//
//				b, _ := Polorize(x.B)
//				assert.Nil(t, pack.PackWire(b))
//
//				c, _ := Polorize(x.C)
//				assert.Nil(t, pack.PackWire(c))
//
//				d, _ := Polorize(x.D)
//				assert.Nil(t, pack.PackWire(d))
//
//				wire, _ := Polorize(x)
//				assert.Equal(t, wire, pack.Bytes())
//			}
//		})
//	})
//
//	t.Run("Complex Struct", func(t *testing.T) {
//		type object struct {
//			A []string
//			B map[string]string
//			C map[string][]uint64
//		}
//
//		t.Run("Pack", func(t *testing.T) {
//			f := fuzz.New()
//			var x object
//
//			for i := 0; i < 1000; i++ {
//				f.Fuzz(&x)
//
//				pack := NewPacker()
//				assert.Nil(t, pack.Pack(x.A))
//				assert.Nil(t, pack.Pack(x.B))
//				assert.Nil(t, pack.Pack(x.C))
//
//				wire, _ := Polorize(x)
//				assert.Equal(t, wire, pack.Bytes())
//			}
//		})
//
//		t.Run("PackWire", func(t *testing.T) {
//			f := fuzz.New()
//			var x object
//
//			for i := 0; i < 1000; i++ {
//				f.Fuzz(&x)
//
//				pack := NewPacker()
//
//				a, _ := Polorize(x.A)
//				assert.Nil(t, pack.PackWire(a))
//
//				b, _ := Polorize(x.B)
//				assert.Nil(t, pack.PackWire(b))
//
//				c, _ := Polorize(x.C)
//				assert.Nil(t, pack.PackWire(c))
//
//				wire, _ := Polorize(x)
//				assert.Equal(t, wire, pack.Bytes())
//			}
//		})
//	})
//
//	t.Run("PackError", func(t *testing.T) {
//		var err error
//		packer := NewPacker()
//
//		err = packer.Pack(make(chan string))
//		assert.EqualError(t, err, "pack error: encode error: unsupported type: chan string [chan]")
//
//		err = packer.PackWire([]byte{})
//		assert.EqualError(t, err, "pack error: wire is empty")
//
//		err = packer.PackWire(nil)
//		assert.EqualError(t, err, "pack error: wire is empty")
//
//		err = packer.PackWire([]byte{45, 23})
//		assert.EqualError(t, err, "pack error: invalid wiretype")
//
//		err = packer.PackWire([]byte{10})
//		assert.EqualError(t, err, "pack error: invalid wiretype")
//	})
//}

func TestUnpacker(t *testing.T) {
	t.Run("SimpleStruct", func(t *testing.T) {
		type object struct {
			A string
			B int64
			C float64
			D bool
		}

		t.Run("Unpack", func(t *testing.T) {
			f := fuzz.New()
			var x object

			for i := 0; i < 1000; i++ {
				f.Fuzz(&x)

				wire, _ := Polorize(x)
				unpacker, err := NewUnpacker(wire)
				require.Nil(t, err)

				a := new(string)
				err = unpacker.Unpack(a)
				assert.Nil(t, err)
				assert.Equal(t, x.A, *a)

				b := new(int64)
				err = unpacker.Unpack(b)
				assert.Nil(t, err)
				assert.Equal(t, x.B, *b)

				c := new(float64)
				err = unpacker.Unpack(c)
				assert.Nil(t, err)
				assert.Equal(t, x.C, *c)

				d := new(bool)
				err = unpacker.Unpack(d)
				assert.Nil(t, err)
				assert.Equal(t, x.D, *d)
			}
		})

		t.Run("UnpackWire", func(t *testing.T) {
			f := fuzz.New()
			var x object

			for i := 0; i < 1000; i++ {
				f.Fuzz(&x)

				wire, _ := Polorize(x)
				unpacker, err := NewUnpacker(wire)
				require.Nil(t, err)

				var data []byte

				data, err = unpacker.UnpackWire()
				a, _ := Polorize(x.A)
				assert.Nil(t, err)
				assert.Equal(t, a, data)

				data, err = unpacker.UnpackWire()
				b, _ := Polorize(x.B)
				assert.Nil(t, err)
				assert.Equal(t, b, data)

				data, err = unpacker.UnpackWire()
				c, _ := Polorize(x.C)
				assert.Nil(t, err)
				assert.Equal(t, c, data)

				data, err = unpacker.UnpackWire()
				d, _ := Polorize(x.D)
				assert.Nil(t, err)
				assert.Equal(t, d, data)
			}
		})
	})

	t.Run("ComplexStruct", func(t *testing.T) {
		type object struct {
			A []string
			B map[string]string
			C map[string][]uint64
		}

		t.Run("Unpack", func(t *testing.T) {
			f := fuzz.New()
			var x object

			for i := 0; i < 1000; i++ {
				f.Fuzz(&x)

				wire, _ := Polorize(x)
				unpacker, err := NewUnpacker(wire)
				require.Nil(t, err)

				a := new([]string)
				err = unpacker.Unpack(a)
				assert.Nil(t, err)
				assert.Equal(t, x.A, *a)

				b := new(map[string]string)
				err = unpacker.Unpack(b)
				assert.Nil(t, err)
				assert.Equal(t, x.B, *b)

				c := new(map[string][]uint64)
				err = unpacker.Unpack(c)
				assert.Nil(t, err)
				assert.Equal(t, x.C, *c)
			}
		})

		t.Run("UnpackWire", func(t *testing.T) {
			f := fuzz.New()
			var x object

			for i := 0; i < 1000; i++ {
				f.Fuzz(&x)

				wire, _ := Polorize(x)
				unpacker, err := NewUnpacker(wire)
				require.Nil(t, err)

				var data []byte

				data, err = unpacker.UnpackWire()
				a, _ := Polorize(x.A)
				assert.Nil(t, err)
				assert.Equal(t, a, data)

				data, err = unpacker.UnpackWire()
				b, _ := Polorize(x.B)
				assert.Nil(t, err)
				assert.Equal(t, b, data)

				data, err = unpacker.UnpackWire()
				c, _ := Polorize(x.C)
				assert.Nil(t, err)
				assert.Equal(t, c, data)
			}
		})
	})

	t.Run("NewUnpackError", func(t *testing.T) {
		tests := []struct {
			wire  []byte
			error string
		}{
			{
				[]byte{255, 128, 128, 128, 128, 128, 128, 128, 128},
				"unpack error: malformed tag: varint terminated prematurely",
			},
			{[]byte{0}, "unpack error: load convert fail: not a compound wire"},
			{[]byte{14, 31}, "unpack error: load convert fail: missing head: insufficient data in reader"},
		}

		for _, test := range tests {
			_, err := NewUnpacker(test.wire)
			assert.EqualError(t, err, test.error)
		}
	})

	t.Run("UnpackError", func(t *testing.T) {
		tests := []struct {
			wire   []byte
			object any
			error  string
		}{
			{
				[]byte{14, 15},
				new(int64),
				"unpack error: no elements left",
			},
			{
				[]byte{14, 191, 1, 6, 255, 128, 128, 128, 128, 128, 128, 128, 128, 127},
				new(int64),
				"unpack error: malformed tag: varint overflows 64-bit integer",
			},
			{
				[]byte{14, 31, 6, 98, 111, 111},
				new(int64),
				"unpack error: decode error: incompatible wire type. expected: posint. got: word",
			},
		}

		for _, test := range tests {
			unpacker, err := NewUnpacker(test.wire)
			require.Nil(t, err)

			err = unpacker.Unpack(test.object)
			assert.EqualError(t, err, test.error)
		}
	})
}
