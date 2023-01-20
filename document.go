package polo

import (
	"errors"
	"fmt"
	"reflect"
)

// Document is a representation for a string indexed collection of encoded object data.
// It represents an intermediary access format with objects settable/gettable with string keys.
type Document map[string][]byte

// Size returns the number of elements in a Document
func (doc Document) Size() int {
	return len(doc)
}

// Bytes returns the POLO wire representation of a Document
func (doc Document) Bytes() []byte {
	data, _ := Polorize(doc)
	return data
}

// Get retrieves some raw byte data for a given key from a Document.
// Return nil if there is no data for the key.
func (doc Document) Get(key string) []byte {
	return doc[key]
}

// Set inserts some raw byte data for a given key into a Document.
// Any existing data at the given key is overwritten.
func (doc Document) Set(key string, val []byte) {
	doc[key] = val
}

// Is returns whether the POLO encoded data for some given key has a specific wire type.
// If key does not exist or has no data in the document, it is considered as WireNull.
func (doc Document) Is(key string, kind WireType) bool {
	data := doc.Get(key)
	if len(data) == 0 {
		return kind == WireNull
	}

	return data[0] == byte(kind)
}

// GetObject retrieves some object for some given key from a Document.
// The data for the given key is decoded from its POLO form into the given object which must be a pointer.
// Returns an error if there is no data for the key or if the data could not be decoded into the given object.
func (doc Document) GetObject(key string, object any) error {
	// Retrieve the data for the key and error if unavailable
	var data []byte
	if data = doc[key]; data == nil {
		return fmt.Errorf("document value not found for key '%v'", key)
	}

	// Depolorize the data into the given object and return any error
	if err := Depolorize(object, data); err != nil {
		return fmt.Errorf("document value could not be decoded for key '%v': %w", key, err)
	}

	return nil
}

// SetObject inserts some object for some given key into a Document.
// The given object is encoded into its POLO form and inserted, overwriting any existing data.
// Returns an error if the given object cannot be serialized with Polorize().
func (doc Document) SetObject(key string, object any) error {
	// Polorize the object into its wire form. Return any error that occurs
	data, err := Polorize(object)
	if err != nil {
		return fmt.Errorf("document value could not be encoded for key '%v': %w", key, err)
	}

	// Insert the wire data for the key
	doc.Set(key, data)
	return nil
}

// DocumentEncode encodes an object into its POLO bytes such that it can be decoded as a Document.
// The object must either be a map with string keys, a struct or a pointer to either.
// 	- Map objects will use the same key in the object for Document keys
// 	- Struct objects will use the field name as declared unless overridden
// 	with a polo struct tag specifying the field name to use. Private struct fields
//  and fields marked to be skipped will be ignored while converting into a Document.
//
// Returns an error if the object is not a supported type, is a nil pointer
// or if any of the element/field types cannot be serialized with Polorize()
func DocumentEncode(object any) (Document, error) {
	switch v := reflect.ValueOf(object); v.Kind() {
	// Pointers (unwrap and recursively call DocumentEncode)
	case reflect.Ptr:
		if v.IsNil() {
			return nil, errors.New("could not encode into document: unsupported type: nil pointer")
		}

		return DocumentEncode(v.Elem().Interface())

	// Maps
	case reflect.Map:
		// Verify that map has string keys
		if v.Type().Key().Kind() != reflect.String {
			return nil, errors.New("could not encode into document: unsupported type: map type with non string key")
		}

		// Create a new Document object with enough space for the map elements
		doc := make(Document, v.Len())

		// For each key in the map, encode the value and set it with the string key
		for _, k := range v.MapKeys() {
			if err := doc.SetObject(k.String(), v.MapIndex(k).Interface()); err != nil {
				return nil, fmt.Errorf("could not encode into document: %w", err)
			}
		}

		return doc, nil

	// Structs
	case reflect.Struct:
		t := v.Type()

		// Create a new Document object with enough space for the struct fields
		doc := make(Document, v.NumField())

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

			if err := doc.SetObject(fieldName, v.Field(i).Interface()); err != nil {
				return nil, fmt.Errorf("could not encode into document: %w", err)
			}
		}

		return doc, nil

	default:
		return nil, errors.New("could not encode into document: unsupported type")
	}
}

// documentDecode decodes a readbuffer into a Document.
func documentDecode(data readbuffer) (Document, error) {
	switch data.wire {
	case WireDoc:
		// Get the next element as a pack depolorizer with the slice elements
		pack, err := newLoadDepolorizer(data)
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

			// Read the next object from the pack
			val, err := pack.read()
			if err != nil {
				return nil, err
			}

			// Set the value bytes into the document for the decoded key
			doc.Set(docKey, val.data)
		}

		return doc, nil

	// Nil Document
	case WireNull:
		return nil, nil

	default:
		return nil, IncompatibleWireType(data.wire, WireNull, WireDoc)
	}
}
