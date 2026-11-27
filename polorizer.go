package polo

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
)

var (
	// float32Type is the reflect.Type of a float32
	float32Type = reflect.TypeOf(float32(0))
	// polorizableType is the reflect.Type of the Polorizable interface
	polorizableType = reflect.TypeOf((*Polorizable)(nil)).Elem()
)

// Polorizer is an encoding buffer that can sequentially polorize objects into it.
// It can be collapsed into its bytes with Bytes() or Packed().
//
// PolorizeString, PolorizeDocument and PolorizePacked have no error result. If one of them gets a
// value that the decoder rejects, the Polorizer keeps the error: see Err.
type Polorizer struct {
	wb *writebuffer

	// err is the first error of a method that has no error result (see Err)
	err error
}

// NewPolorizer creates a new Polorizer.
func NewPolorizer() *Polorizer {
	return &Polorizer{wb: &writebuffer{}}
}

// Err returns the first error of the methods of the Polorizer that have no error result. It is nil if
// there is none. PolorizeString and PolorizeDocument set it for a string or a Document key that is not
// valid UTF-8 (the error matches ErrInvalidUTF8). PolorizePacked sets it to the error of the given Polorizer.
//
// The decoder rejects the wire of a Polorizer that has an error. Polorize returns this error for
// a type whose custom Polorize method returns such a Polorizer. Code that uses a Polorizer directly
// must check Err before it uses Bytes or Packed.
func (polorizer *Polorizer) Err() error {
	return polorizer.err
}

// fail keeps err as the error of the Polorizer if it does not have one
func (polorizer *Polorizer) fail(err error) {
	if polorizer.err == nil {
		polorizer.err = err
	}
}

// Bytes returns the contents of the Polorizer as bytes.
//   - If no objects were polorized, it returns a WireNull wire
//   - If only one object was polorized, it returns the contents directly
//   - If more than one object was polorized, it returns the contents in a packed wire.
//
// Bytes does not report the error of the Polorizer. See Err.
func (polorizer Polorizer) Bytes() []byte {
	switch polorizer.wb.counter {
	case 0:
		return []byte{0}
	case 1:
		return polorizer.wb.bytes()
	default:
		return polorizer.Packed()
	}
}

// Packed returns the contents of the Polorizer as bytes after packing it and tagging with WirePack.
// Packed does not report the error of the Polorizer. See Err.
func (polorizer Polorizer) Packed() []byte {
	// Declare a new writebuffer
	var wb writebuffer
	// Write the contents of the polorized buffer
	// into the writebuffer and tag with WirePack
	wb.write(WirePack, polorizer.wb.load())

	return wb.bytes()
}

// Polorize encodes a value into the Polorizer.
// Encodes the object based on its type using the Go reflection.
// Returns an error that matches ErrInvalidUTF8 if a string, a string map key or
// a Document key is not valid UTF-8, and an error for a map with a NaN key.
// Returns the error of a custom Polorize method, or of the Polorizer that it returns (see Err).
// If Polorize returns an error, it writes nothing, and the Polorizer does not keep the error.
func (polorizer *Polorizer) Polorize(value any) error {
	return polorizer.polorizeValue(reflect.ValueOf(value))
}

// PolorizeNull encodes a null value into the Polorizer.
// Encodes a WireNull into the head, consuming a position on the wire.
func (polorizer *Polorizer) PolorizeNull() {
	polorizer.wb.write(WireNull, nil)
}

// PolorizeBytes encodes a bytes value into the Polorizer.
// Encodes the bytes as is with the wire type being WireWord.
func (polorizer *Polorizer) PolorizeBytes(value []byte) {
	polorizer.wb.write(WireWord, value)
}

// PolorizeString encodes a string value into the Polorizer.
// Encodes the string as its UTF-8 encoded bytes with the wire type being WireWord.
//
// The string must be valid UTF-8 (see utf8.ValidString), because the decoder rejects a string that is not.
// If it is not valid UTF-8, PolorizeString still writes it, and the Polorizer keeps an error that matches
// ErrInvalidUTF8 (see Err). Use PolorizeBytes for data that can hold any bytes: the wire is the same and
// the decoder accepts any bytes for it.
func (polorizer *Polorizer) PolorizeString(value string) {
	if !validString(value) {
		polorizer.fail(invalidUTF8ValueError())
	}

	polorizer.wb.write(WireWord, []byte(value))
}

// PolorizeBool encodes a bool value into the Polorizer.
// Encodes the boolean as either WireTrue or WireFalse, depending on its value.
func (polorizer *Polorizer) PolorizeBool(value bool) {
	wiretype := WireFalse
	if value {
		wiretype = WireTrue
	}

	polorizer.wb.write(wiretype, nil)
}

// PolorizeUint encodes a signed integer value into the Polorizer.
// Encodes the integer as it's the binary form (big-endian) with the wire type being WirePosInt.
func (polorizer *Polorizer) PolorizeUint(value uint64) {
	var buffer [8]byte

	binary.BigEndian.PutUint64(buffer[:], value)
	polorizer.wb.write(WirePosInt, buffer[8-sizeInteger(value):])
}

// PolorizeInt encodes a signed integer value into the Polorizer.
// Encodes the integer as the binary form of its absolute value with the wire type
// being WirePosInt or WireBigInt based on polarity, with zero considered as positive.
func (polorizer *Polorizer) PolorizeInt(value int64) {
	var (
		buffer   [8]byte
		unsigned uint64
		wiretype WireType
	)

	if value < 0 {
		unsigned = uint64(-value)
		wiretype = WireNegInt
	} else {
		unsigned = uint64(value)
		wiretype = WirePosInt
	}

	binary.BigEndian.PutUint64(buffer[:], unsigned)
	polorizer.wb.write(wiretype, buffer[8-sizeInteger(unsigned):])
}

// PolorizeFloat32 encodes a single point precision float into the Polorizer.
// Encodes the float as its IEEE754 binary form (big-endian) with the wire type being WireFloat.
func (polorizer *Polorizer) PolorizeFloat32(value float32) {
	var buffer [4]byte

	// Convert float into IEEE754 binary representation (single point)
	binary.BigEndian.PutUint32(buffer[:], math.Float32bits(value))
	polorizer.wb.write(WireFloat, buffer[:])
}

// PolorizeFloat64 encodes a double point precision float into the Polorizer.
// Encodes the float as its IEEE754 binary form (big-endian) with the wire type being WireFloat.
func (polorizer *Polorizer) PolorizeFloat64(value float64) {
	var buffer [8]byte

	// Convert float into IEEE754 binary representation (double point)
	binary.BigEndian.PutUint64(buffer[:], math.Float64bits(value))
	polorizer.wb.write(WireFloat, buffer[:])
}

// PolorizeBigInt encodes a big.Int into the Polorizer.
// Encodes the big.Int as its binary form with the wire type being WirePosInt or WireNegInt based on polarity,
// with zero considered as positive. A nil big.Int is encoded as WireNull.
func (polorizer *Polorizer) PolorizeBigInt(value *big.Int) {
	if value == nil {
		polorizer.PolorizeNull()

		return
	}

	switch value.Sign() {
	case 0:
		polorizer.wb.write(WirePosInt, nil)
	case -1:
		polorizer.wb.write(WireNegInt, value.Bytes())
	case 1:
		polorizer.wb.write(WirePosInt, value.Bytes())
	}
}

// PolorizeRaw encodes a Raw into the Polorizer. Encodes the Raw with the wire type being WireRaw.
// No check is performed on the Raw wire, and is assumed to be a valid POLO Wire. A nil Raw = Raw{0}.
// USE WITH CAUTION: Encoding unsupported wire formats, will lead to serialization failures
func (polorizer *Polorizer) PolorizeRaw(value Raw) {
	// If raw value is nil, encode WireNull
	if value == nil {
		value = Raw{0}
	}

	polorizer.wb.write(WireRaw, value)
}

// PolorizeAny encodes an Any into the Polorizer. Encodes the Any as its native type.
// A nil Any is encoded as WireNull.
//
// Returns an error if the Any is not an element that the decoder of an Any accepts: the tag must be
// minimal with an offset of zero, the wire type must not be reserved or WireLoad, and a WireNull,
// WireFalse or WireTrue must have no data.
// The data of other wire types is not examined, so the canonical rules do not apply to it here.
// USE WITH CAUTION: Encoding unsupported wire formats, will lead to serialization failures
func (polorizer *Polorizer) PolorizeAny(value Any) error {
	// If raw value is nil, encode WireNull
	if value == nil {
		polorizer.PolorizeNull()

		return nil
	}

	// Convert the raw value into a readbuffer with a canonical tag (C1, C2).
	// This allows us to access the underlying wire type and data
	rb, err := newcanonicalbuffer(value)
	if err != nil {
		return err
	}

	// The element must be one that an Any can hold: no reserved wire type and no
	// WireLoad, and Null, False and True carry no data (C4)
	if err = rb.checkAny(); err != nil {
		return err
	}

	polorizer.wb.write(rb.wire, rb.data)

	return nil
}

// PolorizePacked encodes the contents of another Polorizer as pack-encoded data.
// The contents are packed into a WireLoad message and tagged with the WirePack wire type.
// If the given Polorizer is nil, a WireNull is encoded instead.
// If the given Polorizer has an error, the Polorizer keeps it (see Err).
func (polorizer *Polorizer) PolorizePacked(pack *Polorizer) {
	// If pack is nil, encode WireNull
	if pack == nil {
		polorizer.PolorizeNull()

		return
	}

	// Keep the error of the pack
	if pack.err != nil {
		polorizer.fail(pack.err)
	}

	// Encode pack load contents as a WirePack
	polorizer.wb.write(WirePack, pack.wb.load())
}

// PolorizeDocument encodes a Document into the Polorizer.
// Encodes the Document keys and raw values as POLO doc-encoded data with the wire type being WireDoc.
// If the Document is nil, it is encoded as a WireNull.
//
// The keys must be valid UTF-8, because the decoder rejects a key that is not. If a key is not valid
// UTF-8, PolorizeDocument still writes the Document, and the Polorizer keeps an error that matches
// ErrInvalidUTF8 (see Err).
func (polorizer *Polorizer) PolorizeDocument(document Document) {
	for key := range document {
		if !validString(key) {
			polorizer.fail(invalidUTF8ValueError())

			break
		}
	}

	polorizer.polorizeDocument(document)
}

// polorizeDocument encodes a Document into the Polorizer without a check of its keys.
// The caller must make sure that the keys are valid UTF-8.
func (polorizer *Polorizer) polorizeDocument(document Document) {
	// Nil Document
	if document == nil {
		polorizer.PolorizeNull()

		return
	}

	// Collect all the document keys
	keys := make([]string, 0, document.Size())
	for key := range document {
		keys = append(keys, key)
	}

	// Sort the document keys
	sort.Strings(keys)
	// Create a new polorizer for the document elements
	documentWire := NewPolorizer()

	// Serialize each key (string) and value (bytes)
	for _, key := range keys {
		// Write the document key
		documentWire.wb.write(WireWord, []byte(key))
		// Write the document value
		documentWire.PolorizeRaw(document[key])
	}

	// Wrap the document polorizer contents as a WireLoad and
	// write to the Polorizer with the WireDoc tag
	polorizer.wb.write(WireDoc, documentWire.wb.load())
}

// structIntoDoc encodes each field of a struct into a Document. The key of each exported field that is
// not skipped is the field name or the name in the polo tag. Each value has the form of its own type.
func structIntoDoc(value reflect.Value) (Document, error) {
	info := getStructInfo(value.Type())

	// A key that is not valid UTF-8, or two fields with the same key
	if info.err != nil {
		return nil, fmt.Errorf("could not encode into document: %w", info.err)
	}

	// Create a new Document object with enough space for the struct fields
	doc := make(Document, len(info.fields))

	for _, field := range info.fields {
		if err := doc.Set(field.key, value.Field(field.index).Interface()); err != nil {
			return nil, fmt.Errorf("could not encode into document: %w", err)
		}
	}

	return doc, nil
}

func strMapIntoDoc(value reflect.Value) (Document, error) {
	// Create a new Document object with enough space for the map elements
	doc := make(Document, value.Len())

	// For each key in the map, encode the value and set it with the string key
	for _, k := range value.MapKeys() {
		// The document key must be valid UTF-8
		if !validString(k.String()) {
			return nil, fmt.Errorf("could not encode into document: %w", invalidUTF8ValueError())
		}

		if err := doc.Set(k.String(), value.MapIndex(k).Interface()); err != nil {
			return nil, fmt.Errorf("could not encode into document: %w", err)
		}
	}

	return doc, nil
}

// polorizeInner encodes another Polorizer directly into the Polorizer.
// Unlike PolorizePacked which will always write it as a packed wire while polorizeInner will write an atomic as is.
// If the given Polorizer is nil, a WireNull is encoded.
// Returns the error of the given Polorizer (see Err) and writes nothing if it has one.
func (polorizer *Polorizer) polorizeInner(inner *Polorizer) error {
	// If inner is nil, encode a WireNull
	if inner == nil {
		polorizer.PolorizeNull()

		return nil
	}

	// The inner polorizer holds a value that the decoder rejects
	if inner.err != nil {
		return inner.err
	}

	// Collapse the inner polorizer into its bytes
	// This will also resolve whether the polorizer is a packed wire
	buffer, _ := newreadbuffer(inner.Bytes())

	// Write the read buffer contents
	polorizer.wb.write(buffer.wire, buffer.data)

	return nil
}

// polorizeByteArrayValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be an array of bytes and is encoded as WireWord.
func (polorizer *Polorizer) polorizeByteArrayValue(value reflect.Value) {
	// Determine array element type (uint8) and size
	arrsize, arrelem := value.Len(), value.Type().Elem()
	// Create a Byte Slice Value
	slice := reflect.MakeSlice(reflect.SliceOf(arrelem), arrsize, arrsize)
	// Copy array contents into slice
	reflect.Copy(slice, value)

	polorizer.PolorizeBytes(slice.Bytes())
}

// polorizeArrayValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be an array or slice and is encoded as element pack encoded data.
func (polorizer *Polorizer) polorizeArrayValue(value reflect.Value) error {
	array := NewPolorizer()

	// Serialize each element into the writebuffer
	for i := 0; i < value.Len(); i++ {
		if err := array.polorizeValue(value.Index(i)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(array)

	return nil
}

// polorizeMapValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be a map and is encoded as key-value pack encoded data, also for a map with string keys.
// Map keys are sorted before being sequentially encoded.
// A NaN key is an error. A negative zero key (also in an array key) is written as positive zero.
func (polorizer *Polorizer) polorizeMapValue(value reflect.Value) error {
	// Check that the map keys can be ordered before sorting them.
	// (sorting keys of an unsupported type would panic)
	if unorderable := unorderableKeyType(value.Type().Key()); unorderable != nil {
		return UnsupportedTypeError(unorderable)
	}

	// Collect the map keys
	keys := value.MapKeys()

	// Float keys (and array keys that hold floats) have two special values
	if mayHoldNaN(value.Type().Key()) {
		for i, key := range keys {
			// A NaN key has no order (and a map can hold several of them),
			// so a map with a NaN key has no canonical encoding and is rejected
			if hasNaN(key) {
				return IncompatibleValueError{msg: "unsupported map key: NaN"}
			}

			// Negative zero and positive zero are the same key, so the key is written as positive zero.
			// The positive zero key is equal to the key of the map, so it finds the same entry
			if hasNegZero(key) {
				keys[i] = withoutNegZero(key)
			}
		}
	}

	// Sort the map keys
	sort.Slice(keys, ValueSort(keys))

	// Create a new polorizer for the map elements
	mapping := NewPolorizer()
	// Serialize each key and its value into the polorizer
	for _, k := range keys {
		// Polorize the key into the buffer
		if err := mapping.polorizeValue(k); err != nil {
			return err
		}
		// Polorize the value into the buffer
		if err := mapping.polorizeValue(value.MapIndex(k)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(mapping)

	return nil
}

// polorizeStructValue accepts a reflect.Value and encodes it into the Polorizer.
// The value must be a struct. A DocumentStruct is encoded as a document,
// and any other struct as field ordered pack encoded data.
func (polorizer *Polorizer) polorizeStructValue(value reflect.Value) error {
	// Get the Type of the value
	t := value.Type()

	// A DocumentStruct is a document
	if getStructInfo(t).document {
		// Encode the struct into a Document
		doc, err := structIntoDoc(value)
		if err != nil {
			return err
		}

		// Flatten the document into the encoding buffer (the keys are already checked)
		polorizer.polorizeDocument(doc)

		return nil
	}

	structure := NewPolorizer()
	// Serialize each field into the writebuffer
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip the field if it is not exported or if it
		// is manually tagged to be skipped with a '-' tag
		if !field.IsExported() || field.Tag.Get("polo") == "-" {
			continue
		}

		if err := structure.polorizeValue(value.Field(i)); err != nil {
			return err
		}
	}

	polorizer.PolorizePacked(structure)

	return nil
}

// polorizePolorizable accepts a reflect.Value and encodes it into the Polorizer.
// The value must implement the Polorizable interface. Returns the error of the Polorize
// method, or the error of the Polorizer that the method returns (see Polorizer.Err).
func (polorizer *Polorizer) polorizePolorizable(value reflect.Value) error {
	// Call the Polorize method of Polorizable (returns a Polorizer and an error)
	outputs := value.MethodByName("Polorize").Call([]reflect.Value{})
	if !outputs[1].IsNil() {
		return outputs[1].Interface().(error) //nolint:forcetypeassert
	}

	// Polorize the inner polorizer
	inner, _ := outputs[0].Interface().(*Polorizer)

	return polorizer.polorizeInner(inner)
}

// polorizeValue accepts a reflect.Value and encodes it into the Polorizer.
// The underlying value can be any type apart from channels, functions and interfaces.
// The only supported interfaces are those with a Polorize method: they encode the value that they hold.
func (polorizer *Polorizer) polorizeValue(value reflect.Value) (err error) {
	// Untyped Nil
	if value == zeroVal {
		return IncompatibleValueError{msg: "unsupported type: cannot encode untyped nil"}
	}

	// Nil Pointer
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			polorizer.PolorizeNull()

			return nil
		}
	}

	// Polorizable Type
	if value.Type().Implements(polorizableType) {
		// An interface with a Polorize method encodes the value that it holds. Thus, it gives the
		// same wire as a field of the concrete type: a nil interface and a nil pointer are a WireNull.
		if value.Kind() == reflect.Interface {
			if value.IsNil() {
				polorizer.PolorizeNull()

				return nil
			}

			return polorizer.polorizeValue(value.Elem())
		}

		return polorizer.polorizePolorizable(value)
	}

	// Check the kind of value
	switch kind := value.Kind(); kind {
	// Pointer
	case reflect.Ptr:
		return polorizer.polorizeValue(value.Elem())

	// Boolean
	case reflect.Bool:
		polorizer.PolorizeBool(value.Bool())

	// String (must be valid UTF-8)
	case reflect.String:
		str := value.String()
		if !validString(str) {
			return invalidUTF8ValueError()
		}

		polorizer.wb.write(WireWord, []byte(str))

	// Unsigned Integer
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		polorizer.PolorizeUint(value.Uint())

	// Signed Integer
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		polorizer.PolorizeInt(value.Int())

	// Single Point Float
	case reflect.Float32:
		polorizer.PolorizeFloat32(float32Of(value))

	// Double Point Float
	case reflect.Float64:
		polorizer.PolorizeFloat64(value.Float())

	// Slice Value
	case reflect.Slice:
		// Nil Slice
		if value.IsNil() {
			polorizer.PolorizeNull()

			return nil
		}

		// Any Bytes
		if value.Type() == reflect.TypeOf(Any{}) {
			return polorizer.PolorizeAny(value.Bytes())
		}

		// Raw Bytes
		if value.Type() == reflect.TypeOf(Raw{}) {
			polorizer.PolorizeRaw(value.Bytes())

			return nil
		}

		// Byte Slice
		if value.Type().Elem().Kind() == reflect.Uint8 {
			polorizer.PolorizeBytes(value.Bytes())

			return nil
		}

		return polorizer.polorizeArrayValue(value)

	// Array Value
	case reflect.Array:
		// Byte Array
		if value.Type().Elem().Kind() == reflect.Uint8 {
			polorizer.polorizeByteArrayValue(value)

			return nil
		}

		return polorizer.polorizeArrayValue(value)

	// Map Value (Pack Encoded. Key-Value. Sorted Keys)
	case reflect.Map:
		// Nil Map
		if value.IsNil() {
			polorizer.PolorizeNull()

			return nil
		}

		// Check if value is a polo.Document and encode as such (the keys must be valid UTF-8)
		if value.Type() == documentType {
			document, _ := value.Interface().(Document)
			for key := range document {
				if !validString(key) {
					return invalidUTF8ValueError()
				}
			}

			polorizer.polorizeDocument(document)

			return nil
		}

		return polorizer.polorizeMapValue(value)

	// Struct Value (Field Ordered Pack Encoded)
	case reflect.Struct:
		// Check if value is a big.Int and encode as such
		if value.Type() == reflect.TypeOf(*big.NewInt(0)) {
			bignumber, _ := value.Interface().(big.Int)
			polorizer.PolorizeBigInt(&bignumber)

			return nil
		}

		return polorizer.polorizeStructValue(value)

	// Unsupported Type
	default:
		return UnsupportedTypeError(value.Type())
	}

	return nil
}

// float32Of returns the float32 of a reflected float32 value with its bits preserved. reflect.Value.Float
// converts it to a float64, which turns a signaling NaN into a quiet NaN, so a NaN is read without it.
func float32Of(value reflect.Value) float32 {
	if float := value.Float(); !math.IsNaN(float) {
		// The float64 holds the float32 exactly
		return float32(float)
	}

	float, _ := value.Convert(float32Type).Interface().(float32)

	return float
}
