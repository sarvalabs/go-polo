package polo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// readbuffer is a read-only buffer that is obtained from a single tag and its body.
type readbuffer struct {
	wire WireType
	data []byte
}

// newreadbuffer creates a new readbuffer from a given slice of bytes b.
// Returns a readbuffer and an error if one occurs.
// Throws an error if the tag is malformed.
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

// bytes returns the full readbuffer as slice of bytes.
// It prepends its wiretype to rest of the data.
func (rb readbuffer) bytes() []byte {
	rbytes := make([]byte, len(rb.data))
	copy(rbytes, rb.data)

	return prepend(byte(rb.wire), rbytes)
}

// unpack returns a packbuffer from a readbuffer.
// Throws an error if the wiretype of the readbuffer is not compound (pack)
func (rb *readbuffer) unpack() (*packbuffer, error) {
	// Check that readbuffer has a compound wiretype
	if !rb.wire.IsCompound() {
		return nil, errors.New("load convert fail: not a compound wire")
	}

	// Create a reader from the readbuffer data
	r := bytes.NewReader(rb.data)

	// Attempt to consume a varint from the reader for the load tag
	loadtag, _, err := consumeVarint(r)
	if err != nil {
		return nil, fmt.Errorf("load convert fail: %w", MalformedTagError{err.Error()})
	}

	// Check that the tag has a type of WireLoad
	if loadtag&15 != uint64(WireLoad) {
		return nil, errors.New("load convert fail: missing load tag")
	}

	// Read the number of bytes specified by the load for the header
	head, err := read(r, int(loadtag>>4))
	if err != nil {
		return nil, fmt.Errorf("load convert fail: missing head: %w", err)
	}

	// Read the remaining bytes in the reader for the body
	body, _ := read(r, r.Len())

	// Create a new packbuffer and return it
	lr := newpackbuffer(head, body)

	return lr, nil
}

func (rb readbuffer) asAny() Any {
	if rb.wire == WireNull {
		return Any{0}
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
	// Default Value
	case WireNull:
		return false, errNilValue
	default:
		return false, IncompatibleWireType(rb.wire, WireNull, WireTrue, WireFalse)
	}
}

func (rb readbuffer) decodeBytes(allowPack bool) ([]byte, error) {
	switch rb.wire {
	case WireWord:
		return rb.data, nil

	// Packed Bytes Value ([]uint8)
	case WirePack:
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

			// Attempt to decode the item into a uint8
			decoded, err := item.decodeUint8()
			if err != nil {
				return nil, err
			}

			packed = append(packed, decoded)
		}

		return packed, nil

	// Nil Byte Slice (Default)
	case WireNull:
		return nil, errNilValue

	default:
		allowed := []WireType{WireNull, WireWord}
		if allowPack {
			allowed = append(allowed, WirePack)
		}

		return nil, IncompatibleWireType(rb.wire, allowed...)
	}
}

func (rb readbuffer) decodeBytes32(allowPack bool) ([32]byte, error) {
	value, err := rb.decodeBytes(allowPack)
	if err != nil {
		return [32]byte{}, err
	}

	if len(value) > 32 {
		return [32]byte{}, IncompatibleValueError{"excess data for 32-byte array"}
	}

	var bytes32 [32]byte

	copy(bytes32[:], value)

	return bytes32, nil
}

func (rb readbuffer) decodeString() (string, error) {
	switch rb.wire {
	// Convert []byte to string
	case WireWord:
		return string(rb.data), nil
	// Empty String (Default)
	case WireNull:
		return "", errNilValue
	default:
		return "", IncompatibleWireType(rb.wire, WireNull, WireWord)
	}
}

func (rb readbuffer) decodeUint64() (uint64, error) {
	// Check that the data does not overflow for 64-bits
	if len(rb.data) > 8 {
		return 0, IncompatibleValueError{"excess data for 64-bit integer"}
	}

	switch rb.wire {
	case WirePosInt:
		// Decode the data into a uint64
		number := binary.BigEndian.Uint64(append(make([]byte, 8-len(rb.data), 8), rb.data...))

		return number, nil

	case WireNull:
		return 0, errNilValue
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WirePosInt)
	}
}

func (rb readbuffer) decodeUint32() (uint32, error) {
	// Check that the data does not overflow for 32-bits
	if len(rb.data) > 4 {
		return 0, IncompatibleValueError{"excess data for 32-bit integer"}
	}

	decoded, err := rb.decodeUint64()
	if err != nil {
		return 0, err
	}

	return uint32(decoded), nil
}

func (rb readbuffer) decodeUint16() (uint16, error) {
	// Check that the data does not overflow for 16-bits
	if len(rb.data) > 2 {
		return 0, IncompatibleValueError{"excess data for 16-bit integer"}
	}

	decoded, err := rb.decodeUint64()
	if err != nil {
		return 0, err
	}

	return uint16(decoded), nil
}

func (rb readbuffer) decodeUint8() (uint8, error) {
	// Check that the data does not overflow for 8-bits
	if len(rb.data) > 1 {
		return 0, IncompatibleValueError{"excess data for 8-bit integer"}
	}

	decoded, err := rb.decodeUint64()
	if err != nil {
		return 0, err
	}

	return uint8(decoded), nil
}

func (rb readbuffer) decodeInt64() (decoded int64, err error) {
	// Check that the data does not overflow for bit-size
	if len(rb.data) > 8 {
		return 0, IncompatibleValueError{"excess data for 64-bit integer"}
	}

	switch rb.wire {
	case WireNegInt:
		// If the negative, defer the sign flip of the decoded value
		defer func() { decoded = -decoded }()

		fallthrough

	case WirePosInt:
		// Decode the data into a uint64
		number := binary.BigEndian.Uint64(append(make([]byte, 8-len(rb.data), 8), rb.data...))

		// Check that number is within bounds for int64
		if number > math.MaxInt64 {
			return 0, IncompatibleValueError{"overflow for signed integer"}
		}

		// Convert to int64 and return
		return int64(number), nil

	case WireNull:
		return 0, errNilValue
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WirePosInt, WireNegInt)
	}
}

func (rb readbuffer) decodeInt32() (int32, error) {
	// Check that the data does not overflow for 32-bits
	if len(rb.data) > 4 {
		return 0, IncompatibleValueError{"excess data for 32-bit integer"}
	}

	decoded, err := rb.decodeInt64()
	if err != nil {
		return 0, err
	}

	return int32(decoded), nil
}

func (rb readbuffer) decodeInt16() (int16, error) {
	// Check that the data does not overflow for 16-bits
	if len(rb.data) > 2 {
		return 0, IncompatibleValueError{"excess data for 16-bit integer"}
	}

	decoded, err := rb.decodeInt64()
	if err != nil {
		return 0, err
	}

	return int16(decoded), nil
}

func (rb readbuffer) decodeInt8() (int8, error) {
	// Check that the data does not overflow for 8-bits
	if len(rb.data) > 1 {
		return 0, IncompatibleValueError{"excess data for 8-bit integer"}
	}

	decoded, err := rb.decodeInt64()
	if err != nil {
		return 0, err
	}

	return int8(decoded), nil
}

func (rb readbuffer) decodeFloat32() (float32, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 4 {
			return 0, IncompatibleWireError{"malformed data for 32-bit float"}
		}

		// Convert float from IEEE754 binary representation (single point)
		float := math.Float32frombits(binary.BigEndian.Uint32(rb.data))
		if math.IsNaN(float64(float)) {
			return 0, IncompatibleValueError{"float is not a number"}
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		return 0, errNilValue
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WireFloat)
	}
}

func (rb readbuffer) decodeFloat64() (float64, error) {
	switch rb.wire {
	case WireFloat:
		if len(rb.data) != 8 {
			return 0, IncompatibleWireError{"malformed data for 64-bit float"}
		}

		// Convert float from IEEE754 binary representation (double point)
		float := math.Float64frombits(binary.BigEndian.Uint64(rb.data))
		if math.IsNaN(float) {
			return 0, IncompatibleValueError{"float is not a number"}
		}

		return float, nil

	// 0 (Default)
	case WireNull:
		return 0, errNilValue
	default:
		return 0, IncompatibleWireType(rb.wire, WireNull, WireFloat)
	}
}

func (rb readbuffer) decodeBigInt() (*big.Int, error) {
	switch rb.wire {
	case WirePosInt:
		return new(big.Int).SetBytes(rb.data), nil
	case WireNegInt:
		return new(big.Int).Neg(new(big.Int).SetBytes(rb.data)), nil

	// Nil big.Int
	case WireNull:
		return nil, errNilValue
	default:
		return nil, IncompatibleWireType(rb.wire, WireNull, WirePosInt, WireNegInt)
	}
}

func (rb readbuffer) decodeDocument() (Document, error) {
	switch rb.wire {
	case WireDoc:
		// Get the next element as a pack depolorizer with the slice elements
		pack, err := newLoadDepolorizer(rb, nil)
		if err != nil {
			return nil, err
		}

		doc := make(Document)

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the Document key (string)
			docKey, err := pack.DepolorizeString()
			if err != nil {
				return nil, err
			}

			// Depolorize the next object from the pack into the Document val (raw)
			docVal, err := pack.DepolorizeRaw()
			if err != nil {
				return nil, err
			}

			// Set the value bytes into the document for the decoded key
			doc.SetRaw(docKey, docVal)
		}

		return doc, nil

	// Nil Document
	case WireNull:
		return nil, nil

	default:
		return nil, IncompatibleWireType(rb.wire, WireNull, WireDoc)
	}
}
