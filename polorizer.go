package polo

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
)

// Polorizer is an encoding buffer that can sequentially polorize objects into it.
// It can be collapsed into its bytes with Bytes() or Packed().
type Polorizer struct {
	wb  *writebuffer
	cfg wireConfig
}

// NewPolorizer creates a new Polorizer.
// Accepts EncodingOptions to modify the encoding behaviour of the Polorizer
func NewPolorizer(options ...EncodingOptions) *Polorizer {
	// Generate a default wire config
	config := defaultWireConfig()
	// Apply any given options to the config
	config.apply(options...)

	return &Polorizer{wb: &writebuffer{}, cfg: *config}
}

// Bytes returns the contents of the Polorizer as bytes.
//   - If no objects were polorized, it returns a WireNull wire
//   - If only one object was polorized, it returns the contents directly
//   - If more than one object was polorized, it returns the contents in a packed wire.
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
//
// If PackedBytes() is used when creating the Polorizer, the
// bytes is encoded as a WirePack element instead of WireWord.
func (polorizer *Polorizer) PolorizeBytes(value []byte) {
	if polorizer.cfg.packBytes {
		polorizer.polorizeByteAsPack(value)
		return
	}

	polorizer.wb.write(WireWord, value)
}

// PolorizeString encodes a string value into the Polorizer.
// Encodes the string as its UTF-8 encoded bytes with the wire type being WireWord.
func (polorizer *Polorizer) PolorizeString(value string) {
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
// A nil Any is encodes as WireNull. Returns an error if the Any wire has an invalid wire tag.
// USE WITH CAUTION: Encoding unsupported wire formats, will lead to serialization failures
func (polorizer *Polorizer) PolorizeAny(value Any) error {
	// If raw value is nil, encode WireNull
	if value == nil {
		polorizer.PolorizeNull()
		return nil
	}

	// Convert the raw value into a readbuffer.
	// This allows us to access the underlying wire type and data
	rb, err := newreadbuffer(value)
	if err != nil {
		return err
	}

	polorizer.wb.write(rb.wire, rb.data)

	return nil
}

// PolorizePacked encodes the contents of another Polorizer as pack-encoded data.
// The contents are packed into a WireLoad message and tagged with the WirePack wire type.
// If the given Polorizer is nil, a WireNull is encoded instead.
func (polorizer *Polorizer) PolorizePacked(pack *Polorizer) {
	// If pack is nil, encode WireNull
	if pack == nil {
		polorizer.PolorizeNull()
		return
	}

	// Encode pack load contents as a WirePack
	polorizer.wb.write(WirePack, pack.wb.load())
}

// PolorizeDocument encodes a Document into the Polorizer.
// Encodes the Document keys and raw values as POLO doc-encoded data with the wire type being WireDoc.
// If the Document is nil, it is encoded as a WireNull.
func (polorizer *Polorizer) PolorizeDocument(document Document) {
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
	documentWire := NewPolorizer(inheritCfg(polorizer.cfg))

	// Serialize each key (string) and value (bytes)
	for _, key := range keys {
		// Write the document key
		documentWire.PolorizeString(key)
		// Write the document value
		documentWire.PolorizeRaw(document[key])
	}

	// Wrap the document polorizer contents as a WireLoad and
	// write to the Polorizer with the WireDoc tag
	polorizer.wb.write(WireDoc, documentWire.wb.load())
}

func (polorizer *Polorizer) polorizeStructIntoDoc(value reflect.Value) (Document, error) {
	t := value.Type()

	// Create a new Document object with enough space for the struct fields
	doc := make(Document, value.NumField())

	// For each struct field that is exported and not skipped, encode
	// the value and set it with the field name (or custom field key)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

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

		if err := doc.Set(fieldName, value.Field(i).Interface(), inheritCfg(polorizer.cfg)); err != nil {
			return nil, fmt.Errorf("could not encode into document: %w", err)
		}
	}

	return doc, nil
}

func (polorizer *Polorizer) polorizeStrMapIntoDoc(value reflect.Value) (Document, error) {
	// Create a new Document object with enough space for the map elements
	doc := make(Document, value.Len())

	// For each key in the map, encode the value and set it with the string key
	for _, k := range value.MapKeys() {
		if err := doc.Set(k.String(), value.MapIndex(k).Interface(), inheritCfg(polorizer.cfg)); err != nil {
			return nil, fmt.Errorf("could not encode into document: %w", err)
		}
	}

	return doc, nil
}

// polorizeInner encodes another Polorizer directly into the Polorizer.
// Unlike PolorizePacked which will always write it as a packed wire while polorizeInner will write an atomic as is.
// If the given Polorizer is nil, a WireNull is encoded.
func (polorizer *Polorizer) polorizeInner(inner *Polorizer) {
	// If inner is nil, encode a WireNull
	if inner == nil {
		polorizer.PolorizeNull()
		return
	}

	// Collapse the inner polorizer into its bytes
	// This will also resolve whether the polorizer is a packed wire
	buffer, _ := newreadbuffer(inner.Bytes())

	// Write the read buffer contents
	polorizer.wb.write(buffer.wire, buffer.data)
}

// polorizeByteAsPack encodes a []byte as WirePack
func (polorizer *Polorizer) polorizeByteAsPack(bytes []byte) {
	// Create a polorizer for the byte pack
	pack := NewPolorizer(inheritCfg(polorizer.cfg))
	// Write each byte as a WirePosInt
	for _, elem := range bytes {
		pack.wb.write(WirePosInt, []byte{elem})
	}

	// Polorize the whole pack into the buffer
	polorizer.PolorizePacked(pack)
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
	array := NewPolorizer(inheritCfg(polorizer.cfg))

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
// The value must be a map and is encoded as key-value pack encoded data.
// Map keys are sorted before being sequentially encoded.
func (polorizer *Polorizer) polorizeMapValue(value reflect.Value) error {
	// Check if the map's key type is string AND the encoding
	// config expects for string maps to be encoded as documents
	if polorizer.cfg.docStrMaps && value.Type().Key().Kind() == reflect.String {
		doc, err := polorizer.polorizeStrMapIntoDoc(value)
		if err != nil {
			return err
		}

		polorizer.PolorizeDocument(doc)

		return nil
	}

	// Sort the map keys
	keys := value.MapKeys()
	sort.Slice(keys, ValueSort(keys))

	// Create a new polorizer for the map elements
	mapping := NewPolorizer(inheritCfg(polorizer.cfg))
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
// The value must be a struct and is encoded as field ordered pack encoded data.
func (polorizer *Polorizer) polorizeStructValue(value reflect.Value) error {
	// Check if the encoder config specifies to encode structs as documents
	if polorizer.cfg.docStructs {
		// Encode the struct into a Document
		doc, err := polorizer.polorizeStructIntoDoc(value)
		if err != nil {
			return err
		}

		// Flatten the document into the encoding buffer
		polorizer.PolorizeDocument(doc)

		return nil
	}

	// Get the Type of the value
	t := value.Type()

	structure := NewPolorizer(inheritCfg(polorizer.cfg))
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
// The value must implement the Polorizable interface.
func (polorizer *Polorizer) polorizePolorizable(value reflect.Value) error {
	// Call the Polorize method of Polorizable (returns a Polorizer and an error)
	outputs := value.MethodByName("Polorize").Call([]reflect.Value{})
	if !outputs[1].IsNil() {
		return outputs[1].Interface().(error) //nolint:forcetypeassert
	}

	// Polorize the inner polorizer
	inner, _ := outputs[0].Interface().(*Polorizer)
	polorizer.polorizeInner(inner)

	return nil
}

// polorizeValue accepts a reflect.Value and encodes it into the Polorizer.
// The underlying value can be any type apart from interfaces, channels and functions.
func (polorizer *Polorizer) polorizeValue(value reflect.Value) (err error) {
	// Untyped Nil
	if value == zeroVal {
		return IncompatibleValueError{"unsupported type: cannot encode untyped nil"}
	}

	// Nil Pointer
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			polorizer.PolorizeNull()
			return nil
		}
	}

	// Polorizable Type
	if value.Type().Implements(reflect.TypeOf((*Polorizable)(nil)).Elem()) {
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

	// String
	case reflect.String:
		polorizer.PolorizeString(value.String())

	// Unsigned Integer
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		polorizer.PolorizeUint(value.Uint())

	// Signed Integer
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		polorizer.PolorizeInt(value.Int())

	// Single Point Float
	case reflect.Float32:
		polorizer.PolorizeFloat32(float32(value.Float()))

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

		// Check if value is a polo.Document and encode as such
		if value.Type() == reflect.TypeOf(Document{}) {
			document, _ := value.Interface().(Document)
			polorizer.PolorizeDocument(document)

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
