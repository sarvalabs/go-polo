package polo

import (
	"fmt"
	"math/big"
	"reflect"
)

var (
	// byteType is the reflect.Type of a byte (uint8). Slices and arrays of
	// exactly this element type can be converted from a []byte directly.
	byteType = reflect.TypeOf(byte(0))

	anyType           = reflect.TypeOf(Any{})
	rawType           = reflect.TypeOf(Raw{})
	documentType      = reflect.TypeOf(Document{})
	bigIntType        = reflect.TypeOf(big.Int{})
	depolorizableType = reflect.TypeOf((*Depolorizable)(nil)).Elem()
)

// Depolorizer is a decoding buffer that can sequentially depolorize object from it.
// It can check whether there are elements left in the buffer with Done()
// and peek the WireType of the next element with Peek().
//
// A nil *Depolorizer (returned by DepolorizePacked for a WireNull) has no elements:
// Done returns true, IsNull returns false and every decode returns ErrInsufficientWire.
type Depolorizer struct {
	done, packed bool

	data readbuffer
	pack *packbuffer

	cfg wireConfig
	// depth is the nesting depth of the Depolorizer (0 for a top-level Depolorizer)
	depth int

	// track is set for the Depolorizer that a custom Depolorize method receives and for each pack that
	// is unpacked from it (DepolorizePacked). It points to the open counter of the first one, so that
	// the decoder can check that the method read all the elements. It is nil for all other Depolorizers.
	track *int
	// open is the number of tracked packs that are not read to the end
	open int
}

// NewDepolorizer returns a new Depolorizer for some given bytes.
// Returns an error if the given bytes is malformed for a POLO wire.
//
// If the given data is a compound wire, the only element in the Depolorizer will be the compound data, and
// it will need to be unwrapped into another Depolorizer with DepolorizePacked() before decoding its elements
func NewDepolorizer(data []byte, options ...EncodingOptions) (*Depolorizer, error) {
	// Generate a default wire config
	config := defaultWireConfig()
	// Apply any given options to the config
	for _, opt := range options {
		opt(config)
	}

	// Copy the wire so that the Depolorizer (and the byte slices
	// it decodes) never alias a buffer that the caller may reuse
	wire := make([]byte, len(data))
	copy(wire, data)

	// Create a top-level non-pack Depolorizer
	return newDepolorizer(wire, *config, 0)
}

// newDepolorizer returns a new Depolorizer for some given bytes with the given config and nesting depth.
// Unlike NewDepolorizer, the bytes are not copied and must not be modified while the Depolorizer is in use.
func newDepolorizer(data []byte, cfg wireConfig, depth int) (*Depolorizer, error) {
	// Create a new readbuffer from the wire (the tag must be canonical)
	rb, err := newcanonicalbuffer(data)
	if err != nil {
		return nil, err
	}

	// Create a non-pack Depolorizer
	return &Depolorizer{data: rb, cfg: cfg, depth: depth}, nil
}

// newLoadDepolorizer returns a new Depolorizer from a given readbuffer with the given config and nesting depth.
// The readbuffer is converted into a packbuffer and the returned Depolorizer is created in packed mode.
// Returns an error if the nesting depth exceeds the maximum allowed by the config.
func newLoadDepolorizer(data readbuffer, cfg wireConfig, depth int) (*Depolorizer, error) {
	// Check the nesting depth of the new Depolorizer against the config
	if err := cfg.checkDepth(depth); err != nil {
		return nil, err
	}

	// Convert the element into a packbuffer
	pack, err := data.unpack()
	if err != nil {
		return nil, err
	}

	// Create a new Depolorizer in packed mode
	return &Depolorizer{pack: pack, packed: true, cfg: cfg, depth: depth}, nil
}

// Unpacked attempts to unpack a Depolorizer that contains a WirePack or WireDoc element.
// A new Depolorizer is returned with the unpacked wire and the original Depolorizer consumes one wire element.
// Returns a nil Depolorizer (which has no elements) if the element is a WireNull.
// Returns an error if there are no elements left or if the element is not a WirePack, WireDoc or WireNull.
//
// note: this method is a wrapper around DepolorizePacked for brevity and aesthetics.
func (depolorizer *Depolorizer) Unpacked() (*Depolorizer, error) {
	return depolorizer.DepolorizePacked()
}

// Done returns whether all elements in the Depolorizer have been read.
// A nil Depolorizer is always done.
func (depolorizer *Depolorizer) Done() bool {
	// A nil Depolorizer has no elements
	if depolorizer == nil {
		return true
	}

	// Check if packbuffer is done if in packed mode
	if depolorizer.packed {
		return depolorizer.pack.done()
	}

	// Return flag for non-pack data
	return depolorizer.done
}

// IsNull returns whether the next element in the Depolorizer is a WireNull without
// consuming it from the buffer. False if there are no elements left in the Depolorizer.
//
// Use DepolorizeNull to consume the WireNull element if further elements are to be read.
func (depolorizer *Depolorizer) IsNull() bool {
	wire, ok := depolorizer.peek()

	return ok && wire == WireNull
}

// peek returns the WireType of the next element in the Depolorizer without consuming it.
// Returns (WireNull, false) if there are no elements left in the Depolorizer.
func (depolorizer *Depolorizer) peek() (WireType, bool) {
	if depolorizer.Done() {
		return WireNull, false
	}

	// Peek the next element from the packbuffer if in packed mode
	if depolorizer.packed {
		return depolorizer.pack.peek()
	}

	return depolorizer.data.wire, true
}

// Depolorize decodes a value from the Depolorizer.
// Decodes the data in the wire into the given object using Go reflection.
// Returns an error if the object is not a pointer or if a decode error occurs.
//
// The decoded value overwrites the object. A WireNull sets an object that can be nil
// (pointer, slice, map) to nil, and is an error (ErrNullValue) for any other object.
// The only exception is an object with a custom Depolorize method, which decides for itself.
func (depolorizer *Depolorizer) Depolorize(object any) error {
	// Reflect the object value
	value := reflect.ValueOf(object)
	if value.Kind() != reflect.Pointer {
		return ErrObjectNotPtr
	}

	// Check that the value is a settable pointer
	if !value.Elem().CanSet() {
		return ErrObjectNotSettable
	}

	// Depolorize the next element to the type of the object
	result, err := depolorizer.depolorizeValue(value.Type().Elem())
	if err != nil {
		return err
	}

	// Set the decoded value
	value.Elem().Set(result)

	return nil
}

// DepolorizeNull attempts to decode a null value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireNull.
func (depolorizer *Depolorizer) DepolorizeNull() error {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return err
	}

	// Error if not WireNull
	if data.wire != WireNull {
		return IncompatibleWireType(data.wire, WireNull)
	}

	return nil
}

// DepolorizeBytes attempts to decode a bytes value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireWord.
// Returns a nil byte slice if the element is a WireNull.
//
// The WireWord can hold any bytes. Unlike DepolorizeString, the bytes do not have to be valid UTF-8.
//
// If the PackedBytes option is used in the decoder configuration, the element must be
// a WirePack of u8 elements instead, and a WireWord is an error.
func (depolorizer *Depolorizer) DepolorizeBytes() ([]byte, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.decodeBytes(depolorizer.cfg.packBytes)
}

// DepolorizeBytes32 attempts to decode a 32-byte value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireWord with exactly 32 bytes.
// Returns an error that matches ErrNullValue if the element is a WireNull.
//
// If the PackedBytes option is used in the decoder configuration, the element must be
// a WirePack of exactly 32 u8 elements instead, and a WireWord is an error.
func (depolorizer *Depolorizer) DepolorizeBytes32() ([32]byte, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return [32]byte{}, err
	}

	return data.decodeBytes32(depolorizer.cfg.packBytes)
}

// DepolorizeString attempts to decode a string value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireWord.
// Returns an error that matches ErrNullValue if the element is a WireNull.
//
// The WireWord must be valid UTF-8, otherwise the error matches ErrInvalidUTF8.
// Use DepolorizeBytes for data that can hold any bytes.
func (depolorizer *Depolorizer) DepolorizeString() (string, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return "", err
	}

	return data.decodeString()
}

// DepolorizeBool attempts to decode a bool value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireTrue or WireFalse.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBool() (bool, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return false, err
	}

	return data.decodeBool()
}

// DepolorizeUint attempts to decode an unsigned integer value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint() (uint, error) {
	value, err := depolorizer.depolorizeUint(int(reflect.TypeFor[uint]().Size()))
	return uint(value), err //nolint:nlreturn
}

// DepolorizeUint64 attempts to decode a 64-bit unsigned integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint64() (uint64, error) {
	return depolorizer.depolorizeUint(8)
}

// DepolorizeUint32 attempts to decode a 32-bit unsigned integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint32() (uint32, error) {
	value, err := depolorizer.depolorizeUint(4)
	return uint32(value), err //nolint:nlreturn
}

// DepolorizeUint16 attempts to decode a 16-bit unsigned integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint16() (uint16, error) {
	value, err := depolorizer.depolorizeUint(2)
	return uint16(value), err //nolint:nlreturn
}

// DepolorizeUint8 attempts to decode an 8-bit unsigned integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeUint8() (uint8, error) {
	value, err := depolorizer.depolorizeUint(1)
	return uint8(value), err //nolint:nlreturn
}

// DepolorizeInt attempts to decode an int value from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt() (int, error) {
	value, err := depolorizer.depolorizeInt(int(reflect.TypeFor[int]().Size()))
	return int(value), err //nolint:nlreturn
}

// DepolorizeInt64 attempts to decode a 64-bit signed integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt64() (int64, error) {
	return depolorizer.depolorizeInt(8)
}

// DepolorizeInt32 attempts to decode a 32-bit signed integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt32() (int32, error) {
	value, err := depolorizer.depolorizeInt(4)
	return int32(value), err //nolint:nlreturn
}

// DepolorizeInt16 attempts to decode a 16-bit signed integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt16() (int16, error) {
	value, err := depolorizer.depolorizeInt(2)
	return int16(value), err //nolint:nlreturn
}

// DepolorizeInt8 attempts to decode an 8-bit signed integer from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns an error that matches ErrNullValue if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeInt8() (int8, error) {
	value, err := depolorizer.depolorizeInt(1)
	return int8(value), err //nolint:nlreturn
}

// DepolorizeFloat32 attempts to decode a single point precision float from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireFloat.
// Returns an error that matches ErrNullValue if the element is a WireNull.
// A NaN is decoded with its bits preserved.
func (depolorizer *Depolorizer) DepolorizeFloat32() (float32, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return data.decodeFloat32()
}

// DepolorizeFloat64 attempts to decode a double point precision float from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireFloat.
// Returns an error that matches ErrNullValue if the element is a WireNull.
// A NaN is decoded with its bits preserved.
func (depolorizer *Depolorizer) DepolorizeFloat64() (float64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return data.decodeFloat64()
}

// DepolorizeBigInt attempts to decode a big.Int from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WirePosInt or WireNegInt.
// Returns a nil big.Int if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeBigInt() (*big.Int, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.decodeBigInt()
}

// DepolorizeDocument attempts to decode a Document from the Depolorizer, consuming one wire element.
// Returns an error if there are no elements left or if the element is not WireDoc.
// Returns nil Document if the element is a WireNull.
func (depolorizer *Depolorizer) DepolorizeDocument() (Document, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	return data.decodeDocument(depolorizer.cfg, depolorizer.depth+1)
}

// DepolorizeAny attempts to decode an Any from the Depolorizer, consuming one wire element.
// Returns a nil Any if the element is a WireNull.
// Returns an error if there are no elements left, or if the element has a reserved wire type or
// is a WireLoad, because no target type can decode such an element. All other wire types succeed.
//
// The data of the element is not examined. The canonical rules apply to it when the Any is decoded.
func (depolorizer *Depolorizer) DepolorizeAny() (Any, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	// The element must be one that an Any can hold
	if err = data.checkAny(); err != nil {
		return nil, err
	}

	return data.asAny(), nil
}

// DepolorizeRaw attempts to decode a Raw from the Depolorizer, consuming one wire element.
// Returns a nil Raw if the element is a WireNull.
// Returns an error if there are no elements left or if the element is not WireRaw.
func (depolorizer *Depolorizer) DepolorizeRaw() (Raw, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	// Nil Raw
	if data.wire == WireNull {
		return nil, nil
	}

	return data.asRaw()
}

// DepolorizePacked attempts to decode another Depolorizer from the Depolorizer, consuming one wire element.
// A new Depolorizer is returned with the unpacked wire if the element is WirePack or WireDoc.
// Returns a nil Depolorizer (which has no elements) if the element is a WireNull.
// Returns an error if there are no elements left or if the element is not WirePack, WireDoc or WireNull.
func (depolorizer *Depolorizer) DepolorizePacked() (*Depolorizer, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	switch data.wire {
	case WirePack, WireDoc:
		pack, err := newLoadDepolorizer(data, depolorizer.cfg, depolorizer.depth+1)
		if err != nil {
			return nil, err
		}

		// Inside a custom Depolorize method, count the pack until all its elements are read
		if depolorizer.track != nil && !pack.Done() {
			pack.track = depolorizer.track
			*pack.track++
		}

		return pack, nil

	// Nil Depolorizer
	case WireNull:
		return nil, nil

	default:
		return nil, IncompatibleWireType(data.wire, WirePack, WireDoc)
	}
}

// depolorizeUint reads the next element and decodes it as an unsigned integer of the given size in bytes
func (depolorizer *Depolorizer) depolorizeUint(size int) (uint64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return data.decodeUint(size)
}

// depolorizeInt reads the next element and decodes it as a signed integer of the given size in bytes
func (depolorizer *Depolorizer) depolorizeInt(size int) (int64, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return 0, err
	}

	return data.decodeInt(size)
}

// depolorizeInner attempts to decode another Depolorizer from the Depolorizer, consuming one wire element.
// Unlike DepolorizePacked which will expect a compound element and convert it into a packed Depolorizer,
// depolorizeInner will return the atomic element as an atomic Depolorizer.
//
// The returned Depolorizer is for a custom Depolorize method. It inherits the nesting depth and the nesting
// limit, but not the options that select a wire form (see wireConfig.custom). It tracks the packs that the
// method unpacks from it (see checkRead).
func (depolorizer *Depolorizer) depolorizeInner() (*Depolorizer, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return nil, err
	}

	// Create a non-pack Depolorizer
	inner := &Depolorizer{data: data, cfg: depolorizer.cfg.custom(), depth: depolorizer.depth}
	inner.track = &inner.open

	return inner, nil
}

// checkRead returns an error if a custom Depolorize method did not read all the data of the element
// that it received in this Depolorizer. Data that the method does not read can have any content, so
// different wires would decode into the same value. Thus, the method must read:
//   - the element. The only exception is a WireNull: it has no data, and a method
//     that finds it with IsNull does not consume it.
//   - all the elements of each pack that it unpacked with DepolorizePacked or Unpacked.
func (depolorizer *Depolorizer) checkRead(target reflect.Type) error {
	if !depolorizer.done && depolorizer.data.wire != WireNull {
		return IncompatibleWireError{
			msg: fmt.Sprintf("the Depolorize method of %v did not read the element", target),
		}
	}

	if depolorizer.open != 0 {
		return IncompatibleWireError{
			msg: fmt.Sprintf("the Depolorize method of %v did not read all elements of a pack", target),
		}
	}

	return nil
}

// depolorizeByteArrayValue accepts a reflect.Type and decodes a byte array from the Depolorizer.
// The target must be an array of bytes and the next value in the Depolorizer must
// be a WireWord with exactly as many bytes as the array (or a WirePack of bytes if
// the PackedBytes option is set). A WireNull is an error because an array cannot be nil.
func (depolorizer *Depolorizer) depolorizeByteArrayValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	// Decode the bytes for the array (checks the length)
	bytes, err := data.decodeByteArray(target.Len(), depolorizer.cfg.packBytes)
	if err != nil {
		return zeroVal, err
	}

	// Create a Byte Array Value
	array := reflect.New(target).Elem()

	// Copy array contents from the slice into the array. reflect.Copy requires
	// identical element types, so arrays of a named byte type are filled element-wise
	if target.Elem() == byteType {
		reflect.Copy(array, reflect.ValueOf(bytes))
	} else {
		for i, b := range bytes {
			array.Index(i).SetUint(uint64(b))
		}
	}

	return array, nil
}

// depolorizeByteSliceValue accepts a reflect.Type and decodes a byte slice from the Depolorizer.
// The target must be a slice of bytes (or of a named byte type) and the next value in the
// Depolorizer must be a WireWord (or a WirePack of bytes if the PackedBytes option is set).
// A WireNull is a nil slice.
func (depolorizer *Depolorizer) depolorizeByteSliceValue(target reflect.Type) (reflect.Value, error) {
	// Depolorize a bytes value
	bytes, err := depolorizer.DepolorizeBytes()
	if err != nil {
		return zeroVal, err
	}

	// A []byte converts directly into any slice type with a byte element
	if target.Elem() == byteType {
		return reflect.ValueOf(bytes), nil
	}

	// A slice of a named byte type cannot be converted from
	// a []byte and is filled element-wise. A nil slice stays nil
	if bytes == nil {
		return reflect.Zero(target), nil
	}

	slice := reflect.MakeSlice(target, len(bytes), len(bytes))
	for i, b := range bytes {
		slice.Index(i).SetUint(uint64(b))
	}

	return slice, nil
}

// depolorizeSliceValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a slice and the next wire element must be WirePack. A WireNull is a nil slice.
func (depolorizer *Depolorizer) depolorizeSliceValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the slice elements
		pack, err := newLoadDepolorizer(data, depolorizer.cfg, depolorizer.depth+1)
		if err != nil {
			return zeroVal, err
		}

		// Make a new slice
		slice := reflect.MakeSlice(target, 0, 0)
		sliceElem := target.Elem()

		// Iterate on the pack until done
		for !pack.Done() {
			// Depolorize the next object from the pack into the element type
			val, err := pack.depolorizeValue(sliceElem)
			if err != nil {
				return zeroVal, err
			}

			// Append to slice value
			slice = reflect.Append(slice, val)
		}

		return slice, nil

	// Nil Slice
	case WireNull:
		return reflect.Zero(target), nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizeArrayValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be an array and the next wire element must be WirePack with exactly
// as many elements as the array (C5). A WireNull is an error because an array cannot be nil.
func (depolorizer *Depolorizer) depolorizeArrayValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	switch data.wire {
	case WirePack:
		// Get the next element as a pack depolorizer with the array elements
		pack, err := newLoadDepolorizer(data, depolorizer.cfg, depolorizer.depth+1)
		if err != nil {
			return zeroVal, err
		}

		// Get the array length and element type
		arrayLen := target.Len()
		arrayElem := target.Elem()

		// Create a new array
		array := reflect.New(target).Elem()

		// Iterate on array indices
		for index := 0; index < arrayLen; index++ {
			// Depolorize the next object from the pack into the element type
			val, err := pack.depolorizeValue(arrayElem)
			if err != nil {
				return zeroVal, err
			}

			// Set index of array value
			array.Index(index).Set(val)
		}

		// The pack must not have more elements than the array
		if !pack.Done() {
			return zeroVal, IncompatibleWireError{msg: fmt.Sprintf("excess elements for array %v", target)}
		}

		return array, nil

	// An array cannot be nil
	case WireNull:
		return zeroVal, nullValueError()

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, WirePack)
	}
}

// depolorizeMapValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a map and the next wire element must have the form that the encoder writes:
// a WirePack, or a WireDoc for a string keyed map with the DocStringMaps option. The other form is
// an error, so that a map has one wire for each configuration. A WireNull is a nil map.
func (depolorizer *Depolorizer) depolorizeMapValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	// The form that the encoder writes for the map
	form := WirePack
	if depolorizer.cfg.docStrMaps && target.Key().Kind() == reflect.String {
		form = WireDoc
	}

	switch data.wire {
	case form:
		if form == WireDoc {
			return depolorizer.depolorizeMapDoc(data, target)
		}

		return depolorizer.depolorizeMapPack(data, target)

	// Nil Map
	case WireNull:
		return reflect.Zero(target), nil

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, form)
	}
}

// depolorizeMapPack decodes a map of type target from a WirePack element.
//
// The keys must be in strictly ascending order (sorted with ValueCmp, no duplicates) as the encoder
// writes them, and must not be NaN or negative zero (C7). Like the encoder, the decoder only supports
// key types that it can order: booleans, strings, integers, floats and arrays of those.
func (depolorizer *Depolorizer) depolorizeMapPack(data readbuffer, target reflect.Type) (reflect.Value, error) {
	keyType, valType := target.Key(), target.Elem()

	// Check that the map keys can be ordered (the same check as for the encoder)
	if unorderable := unorderableKeyType(keyType); unorderable != nil {
		return zeroVal, UnsupportedTypeError(unorderable)
	}

	// Get the element as a pack depolorizer with the map elements
	pack, err := newLoadDepolorizer(data, depolorizer.cfg, depolorizer.depth+1)
	if err != nil {
		return zeroVal, err
	}

	mapping := reflect.MakeMap(target)

	// previous is the key of the previous entry (invalid for the first entry)
	var previous reflect.Value

	// Iterate on the pack until done
	for !pack.Done() {
		// Depolorize the next object from the pack into the map key type
		key, err := pack.depolorizeValue(keyType)
		if err != nil {
			return zeroVal, err
		}

		// Check that the key is in order after the previous key
		if err = checkMapKeyOrder(previous, key); err != nil {
			return zeroVal, err
		}

		// Depolorize the next object from the pack into the map value type
		val, err := pack.depolorizeValue(valType)
		if err != nil {
			return zeroVal, err
		}

		// Set the key-value pair into the map value
		mapping.SetMapIndex(key, val)

		previous = key
	}

	return mapping, nil
}

// depolorizeMapDoc decodes a string keyed map of type target from a WireDoc element (DocStringMaps option)
func (depolorizer *Depolorizer) depolorizeMapDoc(data readbuffer, target reflect.Type) (reflect.Value, error) {
	keyType, valType := target.Key(), target.Elem()

	// Decode the wire object into a Document (checks that the keys are sorted and unique)
	doc, err := data.decodeDocument(depolorizer.cfg, depolorizer.depth+1)
	if err != nil {
		return zeroVal, err
	}

	mapping := reflect.MakeMap(target)

	// Iterate over the document elements
	for key, raw := range doc {
		// Create a new decoder for the raw value (inherits the
		// configuration and is nested one level below the map)
		decoder, err := newDepolorizer(raw, depolorizer.cfg, depolorizer.depth+1)
		if err != nil {
			return zeroVal, err
		}

		// Depolorize the raw value for the key into map's value type
		val, err := decoder.depolorizeValue(valType)
		if err != nil {
			return zeroVal, err
		}

		// Set the key-value pair into the map value (the key is converted to the map's key type)
		mapping.SetMapIndex(reflect.ValueOf(key).Convert(keyType), val)
	}

	return mapping, nil
}

// checkMapKeyOrder returns an error if a decoded map key is NaN or negative zero, or is not strictly greater
// than the previous key. The previous key is invalid for the first key of a map. ValueCmp is used, because
// ValueLt panics for equal arrays.
//
// Negative zero and positive zero are the same map key. The encoder writes that key as positive zero,
// so a negative zero key is not canonical.
func checkMapKeyOrder(previous, key reflect.Value) error {
	if hasNaN(key) {
		return IncompatibleWireError{msg: "NaN map key"}
	}

	if hasNegZero(key) {
		return IncompatibleWireError{msg: "negative zero map key"}
	}

	if previous.IsValid() && ValueCmp(previous, key) >= 0 {
		return IncompatibleWireError{msg: "map keys are not in strictly ascending order"}
	}

	return nil
}

// depolorizeStructValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type must be a struct and the next wire element must have the form that the encoder writes:
// a WirePack, or a WireDoc with the DocStructs option. The other form is an error, so that a struct has
// one wire for each configuration. A WireNull is an error because a struct value cannot be nil.
func (depolorizer *Depolorizer) depolorizeStructValue(target reflect.Type) (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	// The form that the encoder writes for the struct
	form := WirePack
	if depolorizer.cfg.docStructs {
		form = WireDoc
	}

	switch data.wire {
	case form:
		if form == WireDoc {
			return depolorizer.depolorizeStructDoc(data, target)
		}

		return depolorizer.depolorizeStructPack(data, target)

	// A struct value cannot be nil
	case WireNull:
		return zeroVal, nullValueError()

	default:
		return zeroVal, IncompatibleWireType(data.wire, WireNull, form)
	}
}

// depolorizeStructPack decodes a struct of type target from a WirePack element. The pack
// must have exactly one element for every exported field that is not skipped (C6).
func (depolorizer *Depolorizer) depolorizeStructPack(data readbuffer, target reflect.Type) (reflect.Value, error) {
	// Get the element as a pack depolorizer with the struct field elements
	pack, err := newLoadDepolorizer(data, depolorizer.cfg, depolorizer.depth+1)
	if err != nil {
		return zeroVal, err
	}

	// Create a new struct instance
	structure := reflect.New(target).Elem()

	// Iterate on struct fields
	for index := 0; index < target.NumField(); index++ {
		// Obtain field data for field index
		field := target.Field(index)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		if !field.IsExported() || field.Tag.Get("polo") == "-" {
			continue
		}

		// Depolorize the next object from the pack into the field type
		val, err := pack.depolorizeValue(field.Type)
		if err != nil {
			return zeroVal, newStructFieldError(target, field, err)
		}

		structure.Field(index).Set(val)
	}

	// The pack must not have more elements than the struct has fields
	if !pack.Done() {
		return zeroVal, IncompatibleWireError{msg: fmt.Sprintf("excess elements for struct %v", target)}
	}

	return structure, nil
}

// depolorizeStructDoc decodes a struct of type target from a WireDoc element (DocStructs option).
//
// The document must have exactly one key for every exported field that is not skipped: an absent key
// and an unknown key are errors. Otherwise, different documents would decode into the same struct.
// With the ExtensibleDocStructs option, the decoder ignores an unknown key and leaves the field of
// an absent key at its zero value.
func (depolorizer *Depolorizer) depolorizeStructDoc(data readbuffer, target reflect.Type) (reflect.Value, error) {
	doc, err := data.decodeDocument(depolorizer.cfg, depolorizer.depth+1)
	if err != nil {
		return zeroVal, err
	}

	extensible := depolorizer.cfg.docExtensible

	// Create a new struct instance
	structure := reflect.New(target).Elem()

	// Iterate on struct fields
	for index := 0; index < target.NumField(); index++ {
		// Obtain field data for field index
		field := target.Field(index)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		tag := field.Tag.Get("polo")
		if !field.IsExported() || tag == "-" {
			continue
		}

		// Determine doc key for struct field. Field name is used
		// directly if there is no provided in the polo tag.
		fieldName := field.Name
		if tag != "" {
			fieldName = tag
		}

		// Retrieve the data for the field from the document
		raw, ok := doc[fieldName]
		if !ok {
			// An extensible document can have no key for a field. The field keeps its zero value
			if extensible {
				continue
			}

			return zeroVal, newStructFieldError(target, field, IncompatibleWireError{
				msg: fmt.Sprintf("no document key '%v'", fieldName),
			})
		}

		// Remove each key that a field uses. The keys that stay in the document have no field
		if !extensible {
			delete(doc, fieldName)
		}

		// Create a new decoder for the field data (inherits the
		// configuration and is nested one level below the struct)
		object, err := newDepolorizer(raw, depolorizer.cfg, depolorizer.depth+1)
		if err != nil {
			return zeroVal, err
		}

		fieldVal, err := object.depolorizeValue(field.Type)
		if err != nil {
			return zeroVal, newStructFieldError(target, field, err)
		}

		structure.Field(index).Set(fieldVal)
	}

	// The document must not have a key that the struct has no field for
	if !extensible && len(doc) > 0 {
		return zeroVal, IncompatibleWireError{
			msg: fmt.Sprintf("unknown document key '%v' for struct %v", firstKey(doc), target),
		}
	}

	return structure, nil
}

// firstKey returns the smallest key of a Document that is not empty
func firstKey(doc Document) string {
	var (
		first string
		found bool
	)

	for key := range doc {
		if !found || key < first {
			first, found = key, true
		}
	}

	return first
}

// depolorizeBigIntValue decodes a big.Int value (not a pointer) from the Depolorizer.
// A WireNull is an error because a big.Int value cannot be nil.
func (depolorizer *Depolorizer) depolorizeBigIntValue() (reflect.Value, error) {
	// Read the next element
	data, err := depolorizer.read()
	if err != nil {
		return zeroVal, err
	}

	if data.wire == WireNull {
		return zeroVal, nullValueError()
	}

	bigint, err := data.decodeBigInt()
	if err != nil {
		return zeroVal, err
	}

	return reflect.ValueOf(bigint).Elem(), nil
}

// depolorizePointer decodes a value of type target (a pointer type) from the Depolorizer.
// A WireNull is a nil pointer. It is consumed before the element type is decoded, so the
// Depolorize method of an element type with a custom decoder is never called for it.
func (depolorizer *Depolorizer) depolorizePointer(target reflect.Type) (reflect.Value, error) {
	// Nil Pointer
	if wire, ok := depolorizer.peek(); ok && wire == WireNull {
		// Consume the WireNull element
		if _, err := depolorizer.read(); err != nil {
			return zeroVal, err
		}

		return reflect.Zero(target), nil
	}

	// Recursively call depolorize with the pointer element
	value, err := depolorizer.depolorizeValue(target.Elem())
	if err != nil {
		return zeroVal, err
	}

	// Create a new pointer value and set its inner value and return it
	pointer := reflect.New(target.Elem())
	pointer.Elem().Set(value)

	return pointer, nil
}

// depolorizeDepolorizable decodes value of type target from the Depolorizer.
// The target type must implement the Depolorizable interface. Returns an error
// if the Depolorize method does not read all the data of the element (see checkRead).
func (depolorizer *Depolorizer) depolorizeDepolorizable(target reflect.Type) (reflect.Value, error) {
	// Create a value for the target type
	value := reflect.New(target)

	// Retrieve the next element as a Depolorizer
	inner, err := depolorizer.depolorizeInner()
	if err != nil {
		return zeroVal, err
	}

	// Call the Depolorize method of Depolorizable (accepts a Depolorizer and returns an error)
	outputs := value.MethodByName("Depolorize").Call([]reflect.Value{reflect.ValueOf(inner)})
	if !outputs[0].IsNil() {
		return zeroVal, outputs[0].Interface().(error) //nolint:forcetypeassert
	}

	// The method must read all the data of the element
	if err = inner.checkRead(target); err != nil {
		return zeroVal, err
	}

	return value.Elem(), nil
}

// depolorizeValue accepts a reflect.Type and decodes a value from the Depolorizer into it.
// The target type can be any type apart from channels, functions and interfaces.
// The returned value has exactly the target type.
func (depolorizer *Depolorizer) depolorizeValue(target reflect.Type) (reflect.Value, error) {
	value, err := depolorizer.depolorizeKind(target)
	if err != nil {
		return zeroVal, err
	}

	// Convert the decoded value into the target type (decoders may return the underlying type)
	if value.Type() != target {
		value = value.Convert(target)
	}

	return value, nil
}

// depolorizeKind decodes a value for the target type from the Depolorizer based on the kind of the target.
// The returned value has the target type or a type that converts into it.
//
//nolint:cyclop
func (depolorizer *Depolorizer) depolorizeKind(target reflect.Type) (reflect.Value, error) {
	// Pointer Value (checked before custom decoders, so that a WireNull is always a nil pointer)
	if target.Kind() == reflect.Pointer {
		return depolorizer.depolorizePointer(target)
	}

	// Depolorizable Type
	if reflect.PointerTo(target).Implements(depolorizableType) {
		return depolorizer.depolorizeDepolorizable(target)
	}

	switch kind := target.Kind(); kind {
	// Boolean Value
	case reflect.Bool:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeBool())

	// String Value
	case reflect.String:
		// Read the next element
		data, err := depolorizer.read()
		if err != nil {
			return zeroVal, err
		}

		return reflected(data.decodeString())

	// Unsigned Integer Value (the size of the target bounds the value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflected(depolorizer.depolorizeUint(int(target.Size())))

	// Signed Integer Value (the size of the target bounds the value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected(depolorizer.depolorizeInt(int(target.Size())))

	// Single Point Float
	case reflect.Float32:
		return reflected(depolorizer.DepolorizeFloat32())

	// Double Point Float
	case reflect.Float64:
		return reflected(depolorizer.DepolorizeFloat64())

	// Slice Value
	case reflect.Slice:
		switch {
		// Any Bytes
		case target == anyType:
			return reflected(depolorizer.DepolorizeAny())

		// Raw Bytes
		case target == rawType:
			return reflected(depolorizer.DepolorizeRaw())

		// Byte Slice
		case target.Elem().Kind() == reflect.Uint8:
			return depolorizer.depolorizeByteSliceValue(target)

		default:
			return depolorizer.depolorizeSliceValue(target)
		}

	// Array Value
	case reflect.Array:
		// Byte Array
		if target.Elem().Kind() == reflect.Uint8 {
			return depolorizer.depolorizeByteArrayValue(target)
		}

		return depolorizer.depolorizeArrayValue(target)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		// Document
		if target == documentType {
			return reflected(depolorizer.DepolorizeDocument())
		}

		return depolorizer.depolorizeMapValue(target)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		// BigInt
		if target == bigIntType {
			return depolorizer.depolorizeBigIntValue()
		}

		return depolorizer.depolorizeStructValue(target)

	// Unsupported Type (also interfaces: the wire does not tell which concrete type to decode)
	default:
		return zeroVal, UnsupportedTypeError(target)
	}
}

// read returns the next element in the Depolorizer as a readbuffer.
// If it is in packed mode, it reads from the packbuffer, otherwise
// it returns the readbuffer data and set the done flag.
// Returns an error if the element is a WireNull, WireFalse or WireTrue with data (C4).
func (depolorizer *Depolorizer) read() (readbuffer, error) {
	// Check if there is another element to read
	if depolorizer.Done() {
		return readbuffer{}, ErrInsufficientWire
	}

	var data readbuffer

	if depolorizer.packed {
		// Read from the packbuffer if in packed mode
		next, err := depolorizer.pack.next()
		if err != nil {
			return readbuffer{}, err
		}

		data = next

		// A pack that a custom Depolorize method unpacked is read to the end.
		// This point is reached only one time, because a pack that is done has no next element
		if depolorizer.track != nil && depolorizer.pack.done() {
			*depolorizer.track--
		}
	} else {
		// Set the atomic read flag to done and use the data from the atomic buffer
		depolorizer.done = true
		data = depolorizer.data
	}

	// Null, False and True carry no data
	if err := data.checkEmpty(); err != nil {
		return readbuffer{}, err
	}

	return data, nil
}

// reflected is a helper function that accepts an arbitrary object and an error.
// It returns the reflect.ValueOf of the object and the same error
func reflected[T any](value T, err error) (reflect.Value, error) {
	return reflect.ValueOf(value), err
}
