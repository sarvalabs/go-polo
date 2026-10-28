package polo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"
)

// readbuffer is a read-only buffer that is obtained from a single tag and its body.
type readbuffer struct {
	wire WireType
	data []byte
}

// newreadbuffer creates a new readbuffer from a given slice of bytes b.
// Returns a readbuffer and an error if one occurs.
// Throws an error if the tag is malformed.
//
// newreadbuffer is used by the encoder (to embed Any and inner Polorizer wires) and does not require
// a canonical tag. The decoder uses newcanonicalbuffer instead.
func newreadbuffer(b []byte) (readbuffer, error) {
	// Create a reader from b
	r := bytes.NewReader(b)

	// Attempt to consume a varint from the reader
	tag, consumed, err := consumeVarint(r)
	if err != nil {
		return readbuffer{}, MalformedTagError{err.Error()}
	}

	// Create a readbuffer from the wiretype of the tag (first 4 bits)
	return readbuffer{WireType(tag & 15), b[consumed:]}, nil
}

// newcanonicalbuffer creates a new readbuffer from a given slice of bytes b for decoding.
// The tag must be canonical: a minimally encoded varint (C1) with an offset of zero (C2).
// Throws an error if the tag is malformed or not canonical.
func newcanonicalbuffer(b []byte) (readbuffer, error) {
	// Attempt to consume a minimally encoded varint from the data
	tag, consumed, err := consumeTag(bytes.NewReader(b))
	if err != nil {
		return readbuffer{}, MalformedTagError{err.Error()}
	}

	// The offset bits of a top-level tag must be zero
	if tag>>4 != 0 {
		return readbuffer{}, MalformedTagError{"non-zero offset in top-level tag"}
	}

	// Create a readbuffer from the wiretype of the tag (first 4 bits)
	return readbuffer{WireType(tag & 15), b[consumed:]}, nil
}

// bytes returns the full readbuffer as slice of bytes.
// It prepends its wiretype to rest of the data.
func (rb readbuffer) bytes() []byte {
	rbytes := make([]byte, len(rb.data))
	copy(rbytes, rb.data)

	return prepend(byte(rb.wire), rbytes)
}

// checkEmpty returns an error if the readbuffer is a WireNull, WireFalse or WireTrue with data.
// These wire types are described by their tag alone and carry no data (C4).
func (rb readbuffer) checkEmpty() error {
	switch rb.wire {
	case WireNull, WireFalse, WireTrue:
		if len(rb.data) != 0 {
			return IncompatibleWireError{msg: fmt.Sprintf("unexpected data for wiretype '%v'", rb.wire)}
		}
	}

	return nil
}

// unpack returns a packbuffer from a readbuffer.
// Throws an error if the wiretype of the readbuffer is not compound (pack)
func (rb *readbuffer) unpack() (*packbuffer, error) {
	// Check that readbuffer has a compound wiretype
	if !rb.wire.IsCompound() {
		return nil, errors.New("load convert fail: not a compound wire")
	}

	// Attempt to consume a minimally encoded varint from the readbuffer data for the load tag
	loadtag, consumed, err := consumeTag(bytes.NewReader(rb.data))
	if err != nil {
		return nil, fmt.Errorf("load convert fail: %w", MalformedTagError{err.Error()})
	}

	// Check that the tag has a type of WireLoad
	if loadtag&15 != uint64(WireLoad) {
		return nil, errors.New("load convert fail: missing load tag")
	}

	// The load tag declares the length of the head. Check it against the data that is
	// actually available before slicing. Compared as uint64 so that a declared length
	// can neither trigger a huge allocation nor wrap around when converted to an int.
	rest, headlen := rb.data[consumed:], loadtag>>4
	if headlen > uint64(len(rest)) {
		return nil, errors.New("load convert fail: missing head: insufficient data in reader")
	}

	// Split the rest of the data into the head and the body without copying
	// (the data of a Depolorizer is private to it, see NewDepolorizer)
	return newpackbuffer(rest[:headlen], rest[headlen:])
}

// asAny returns the readbuffer as an Any. A WireNull is a nil Any.
func (rb readbuffer) asAny() Any {
	if rb.wire == WireNull {
		return nil
	}

	return rb.bytes()
}

func (rb readbuffer) asRaw() (Raw, error) {
	if rb.wire != WireRaw {
		return nil, IncompatibleWireType(rb.wire, WireRaw)
	}

	return rb.data, nil
}

func (rb readbuffer) decodeBool() (bool, error) {
	switch rb.wire {
	// True Value
	case WireTrue:
		return true, nil
	// False Value
	case WireFalse:
		return false, nil
	// A bool cannot be nil
	case WireNull:
		return false, nullValueError()
	default:
		return false, IncompatibleWireType(rb.wire, WireNull, WireTrue, WireFalse)
	}
}

// decodeBytes decodes a byte slice from a WireWord (any data is accepted) or, if allowPack
// is set (PackedBytes option), from a WirePack of 8-bit integers. A WireNull is a nil slice.
func (rb readbuffer) decodeBytes(allowPack bool) ([]byte, error) {
	switch {
	case rb.wire == WireWord:
		return rb.data, nil

	// Packed Bytes Value ([]uint8). Only with the PackedBytes option, which
	// is the only configuration in which the encoder produces such a wire
	case rb.wire == WirePack && allowPack:
		return rb.decodePackedBytes()

	// Nil Byte Slice
	case rb.wire == WireNull:
		return nil, nil
	}

	allowed := []WireType{WireNull, WireWord}
	if allowPack {
		allowed = append(allowed, WirePack)
	}

	return nil, IncompatibleWireType(rb.wire, allowed...)
}

// decodePackedBytes decodes a byte slice from a WirePack of 8-bit integers. The encoder writes every byte
// as a WirePosInt with exactly one byte of data (zero included), so that is the only accepted element.
func (rb readbuffer) decodePackedBytes() ([]byte, error) {
	// Unpack the pack encoded data
	pack, err := rb.unpack()
	if err != nil {
		return nil, err
	}

	packed := make([]byte, 0)

	// Iterate on the pack items
	for !pack.done() {
		// Obtain the pack item
		item, err := pack.next()
		if err != nil {
			return nil, err
		}

		// Every item must be a WirePosInt with exactly one byte
		if item.wire != WirePosInt || len(item.data) != 1 {
			// Report the error for an item that is not an 8-bit unsigned integer
			if _, err = item.decodeUint(1); err != nil {
				return nil, err
			}

			// A WirePosInt without data (a zero byte is written as one byte in packed bytes)
			return nil, IncompatibleWireError{msg: "missing data for packed byte"}
		}

		packed = append(packed, item.data[0])
	}

	return packed, nil
}

// decodeByteArray decodes the bytes for a byte array of the given length. An array
// cannot be nil, so a WireNull is an error, and the data must have exactly the length (C5).
func (rb readbuffer) decodeByteArray(length int, allowPack bool) ([]byte, error) {
	if rb.wire == WireNull {
		return nil, nullValueError()
	}

	data, err := rb.decodeBytes(allowPack)
	if err != nil {
		return nil, err
	}

	if len(data) != length {
		return nil, IncompatibleWireError{msg: "mismatched data length for byte array"}
	}

	return data, nil
}

// decodeBytes32 decodes a 32-byte array. A WireNull is an error and the data must have exactly 32 bytes.
func (rb readbuffer) decodeBytes32(allowPack bool) ([32]byte, error) {
	if rb.wire == WireNull {
		return [32]byte{}, nullValueError()
	}

	value, err := rb.decodeBytes(allowPack)
	if err != nil {
		return [32]byte{}, err
	}

	switch {
	case len(value) > 32:
		return [32]byte{}, IncompatibleValueError{msg: "excess data for 32-byte array"}
	case len(value) < 32:
		return [32]byte{}, IncompatibleValueError{msg: "insufficient data for 32-byte array"}
	}

	return [32]byte(value), nil
}

// decodeString decodes a string from a WireWord. The data must be valid UTF-8 (U1).
func (rb readbuffer) decodeString() (string, error) {
	switch rb.wire {
	// Convert []byte to string
	case WireWord:
		if !utf8.Valid(rb.data) {
			return "", invalidUTF8WireError()
		}

		return string(rb.data), nil
	// A string cannot be nil
	case WireNull:
		return "", nullValueError()
	default:
		return "", IncompatibleWireType(rb.wire, WireNull, WireWord)
	}
}

// magnitude returns the big-endian magnitude of an integer wire that has at most 8 bytes.
// The magnitude must be minimal: it has no leading zero byte, so zero has no data (C3).
func (rb readbuffer) magnitude() (uint64, error) {
	if len(rb.data) > 0 && rb.data[0] == 0 {
		return 0, IncompatibleWireError{msg: "leading zero byte in integer"}
	}

	var buffer [8]byte

	copy(buffer[8-len(rb.data):], rb.data)

	return binary.BigEndian.Uint64(buffer[:]), nil
}

// decodeUint decodes an unsigned integer of the given size in bytes (1, 2, 4 or 8) from a WirePosInt
func (rb readbuffer) decodeUint(size int) (uint64, error) {
	// Check that the data does not overflow for the size
	if len(rb.data) > size {
		return 0, IncompatibleValueError{msg: fmt.Sprintf("excess data for %v-bit integer", size*8)}
	}

	switch rb.wire {
	case WirePosInt:
		return rb.magnitude()

	// An integer cannot be nil
	case WireNull:
		return 0, nullValueError()
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WirePosInt)
	}
}

// decodeInt decodes a signed integer of the given size in bytes (1, 2, 4 or 8) from a WirePosInt or WireNegInt.
// The magnitude of a WirePosInt must not exceed the maximum value of the size (GO-6). The magnitude of
// a WireNegInt must not exceed the maximum + 1, which is the minimum value of the size (GO-1, GO-6).
// A WireNegInt with a magnitude of zero (negative zero) is not canonical and is an error (C3).
func (rb readbuffer) decodeInt(size int) (int64, error) {
	// Check that the data does not overflow for the size
	if len(rb.data) > size {
		return 0, IncompatibleValueError{msg: fmt.Sprintf("excess data for %v-bit integer", size*8)}
	}

	// The magnitude of the minimum value for the size (2^(bits-1))
	limit := uint64(1) << (size*8 - 1)

	switch rb.wire {
	case WirePosInt:
		magnitude, err := rb.magnitude()
		if err != nil {
			return 0, err
		}

		if magnitude >= limit {
			return 0, IncompatibleValueError{msg: "overflow for signed integer"}
		}

		return int64(magnitude), nil

	case WireNegInt:
		magnitude, err := rb.magnitude()
		if err != nil {
			return 0, err
		}

		switch {
		case magnitude == 0:
			return 0, IncompatibleWireError{msg: "negative zero integer"}
		case magnitude > limit:
			return 0, IncompatibleValueError{msg: "overflow for signed integer"}
		}

		// For a magnitude of 2^63, the conversion and the negation both
		// wrap around and the result is math.MinInt64, as intended.
		return -int64(magnitude), nil

	// An integer cannot be nil
	case WireNull:
		return 0, nullValueError()
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WirePosInt, WireNegInt)
	}
}

// decodeFloat32 decodes a single point precision float. NaN is accepted and its bits are preserved.
func (rb readbuffer) decodeFloat32() (float32, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 4 {
			return 0, IncompatibleWireError{msg: "malformed data for 32-bit float"}
		}

		// Convert float from IEEE754 binary representation (single point)
		return math.Float32frombits(binary.BigEndian.Uint32(rb.data)), nil

	// A float cannot be nil
	case WireNull:
		return 0, nullValueError()
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WireFloat)
	}
}

// decodeFloat64 decodes a double point precision float. NaN is accepted and its bits are preserved.
func (rb readbuffer) decodeFloat64() (float64, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 8 {
			return 0, IncompatibleWireError{msg: "malformed data for 64-bit float"}
		}

		// Convert float from IEEE754 binary representation (double point)
		return math.Float64frombits(binary.BigEndian.Uint64(rb.data)), nil

	// A float cannot be nil
	case WireNull:
		return 0, nullValueError()
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WireFloat)
	}
}

// decodeBigInt decodes a big.Int from a WirePosInt or WireNegInt. A WireNull is a nil big.Int.
// The magnitude must be minimal (no leading zero byte) and a WireNegInt must not be zero (C3).
func (rb readbuffer) decodeBigInt() (*big.Int, error) {
	switch rb.wire {
	case WirePosInt, WireNegInt:
		if len(rb.data) > 0 && rb.data[0] == 0 {
			return nil, IncompatibleWireError{msg: "leading zero byte in integer"}
		}

		if rb.wire == WirePosInt {
			return new(big.Int).SetBytes(rb.data), nil
		}

		if len(rb.data) == 0 {
			return nil, IncompatibleWireError{msg: "negative zero integer"}
		}

		return new(big.Int).Neg(new(big.Int).SetBytes(rb.data)), nil

	// Nil big.Int
	case WireNull:
		return nil, nil
	default:
		return nil, IncompatibleWireType(rb.wire, WireNull, WirePosInt, WireNegInt)
	}
}

// decodeDocument decodes a Document from a WireDoc. A WireNull is a nil Document.
// The keys must be valid UTF-8 strings in strictly ascending order (sorted, no duplicates), as the
// encoder writes them (C7), and every value must be a WireRaw. The values themselves are not decoded.
func (rb readbuffer) decodeDocument(cfg wireConfig, depth int) (Document, error) {
	switch rb.wire {
	case WireDoc:
		// Get the next element as a pack depolorizer with the document elements
		pack, err := newLoadDepolorizer(rb, cfg, depth)
		if err != nil {
			return nil, err
		}

		doc := make(Document)
		previous := ""

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the Document key (string)
			docKey, err := pack.DepolorizeString()
			if err != nil {
				return nil, err
			}

			// The keys must be strictly ascending, which also excludes duplicate keys
			if len(doc) > 0 && docKey <= previous {
				return nil, IncompatibleWireError{msg: "document keys are not in strictly ascending order"}
			}

			// Read the next object from the pack as the Document val (raw).
			// A WireNull is not a value here: the encoder always writes a WireRaw
			data, err := pack.read()
			if err != nil {
				return nil, err
			}

			docVal, err := data.asRaw()
			if err != nil {
				return nil, err
			}

			// Set the value bytes into the document for the decoded key
			doc.SetRaw(docKey, docVal)
			previous = docKey
		}

		return doc, nil

	// Nil Document
	case WireNull:
		return nil, nil

	default:
		return nil, IncompatibleWireType(rb.wire, WireNull, WireDoc)
	}
}
