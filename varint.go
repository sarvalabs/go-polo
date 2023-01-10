package polo

import (
	"encoding/binary"
	"errors"
	"io"
	"math/bits"
)

// varint errors
var (
	errVarintTerminated = errors.New("varint terminated prematurely")
	errVarintOverflow   = errors.New("varint overflows 64-bit integer")
)

// isBitSize returns whether x is a valid bit-size.
// Must be a power of 2 between 8 and 64. (8, 16, 32, 64)
func isBitSize(x int) bool {
	return (x != 0) && ((x & (x - 1)) == 0) && (x <= 64) && (x >= 8)
}

// sizeInteger returns the min number of bytes
// required to represent an unsigned 64-bit integer.
func sizeInteger(v uint64) int {
	return (bits.Len64(v) + 8 - 1) / 8
}

// sizeVarint returns the length of an encoded varint slice of bytes for a given uint64.
func sizeVarint(v uint64) int {
	return int(9*uint32(bits.Len64(v))+64) / 64
}

// encodeVarint encodes a given uint64 into slice of bytes with varint encoding.
func encodeVarint(v uint64) []byte {
	varint := make([]byte, sizeVarint(v))
	n := binary.PutUvarint(varint, v)

	return varint[:n]
}

// appendVarint appends a given uint64 into a given slice of bytes as varint-encoded dat.
func appendVarint(b []byte, v uint64) []byte {
	switch {
	case v < 1<<7:
		b = append(b, byte(v))

	case v < 1<<14:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte(v>>7))

	case v < 1<<21:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte(v>>14))

	case v < 1<<28:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte(v>>21))

	case v < 1<<35:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte(v>>28))

	case v < 1<<42:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte(v>>35))

	case v < 1<<49:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte(v>>42))

	case v < 1<<56:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte(v>>49))

	case v < 1<<63:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte(v>>56))

	default:
		b = append(b,
			byte((v>>0)&0x7f|0x80),
			byte((v>>7)&0x7f|0x80),
			byte((v>>14)&0x7f|0x80),
			byte((v>>21)&0x7f|0x80),
			byte((v>>28)&0x7f|0x80),
			byte((v>>35)&0x7f|0x80),
			byte((v>>42)&0x7f|0x80),
			byte((v>>49)&0x7f|0x80),
			byte((v>>56)&0x7f|0x80),
			1)
	}

	return b
}

// consumeVarint reads an encoded unsigned integer from r and returns it as an uint64.
func consumeVarint(r io.ByteReader) (x uint64, i int, err error) {
	var s uint

	for i = 0; i < binary.MaxVarintLen64; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return x, i + 1, errVarintTerminated
		}

		if b < 0x80 {
			if i == binary.MaxVarintLen64-1 && b > 1 {
				return x, i + 1, errVarintOverflow
			}

			return x | uint64(b)<<s, i + 1, nil
		}

		x |= uint64(b&0x7f) << s
		s += 7
	}

	return x, 11, errVarintOverflow
}
