package polo

import (
	"errors"
	"fmt"
	"reflect"
)

// PolorizeDocument encodes an object into its POLO bytes such that it can be decoded as a Document.
// The object must either be a map with string keys, a struct or a pointer to either.
//   - Map objects will use the same key in the object for Document keys
//   - Struct objects will use the field name as declared unless overridden
//     with a polo struct tag specifying the field name to use. Private struct fields
//     and fields marked to be skipped will be ignored while converting into a Document.
//
// Returns an error if the object is not a supported type, is a nil pointer
// or if any of the element/field types cannot be serialized with Polorize()
func PolorizeDocument(object any, options ...EncodingOptions) (Document, error) {
	polorizer := NewPolorizer(options...)

	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, errors.New("could not encode into document: unsupported type: nil pointer")
		}

		value = value.Elem()
	}

	switch value.Kind() {
	// Maps
	case reflect.Map:
		// Verify that map has string keys
		if value.Type().Key().Kind() != reflect.String {
			return nil, errors.New("could not encode into document: unsupported type: map type with non string key")
		}

		return polorizer.polorizeStrMapIntoDoc(value)

	// Structs
	case reflect.Struct:
		return polorizer.polorizeStructIntoDoc(value)

	default:
		return nil, errors.New("could not encode into document: unsupported type")
	}
}

// Document is a representation for a string indexed collection of encoded object data.
// It represents an intermediary access format with objects settable/gettable with string keys.
type Document map[string]Raw

// Size returns the number of elements in a Document
func (doc Document) Size() int {
	return len(doc)
}

// Bytes returns the POLO wire representation of a Document
func (doc Document) Bytes() []byte {
	polorizer := NewPolorizer()
	polorizer.PolorizeDocument(doc)

	return polorizer.Bytes()
}

// GetRaw retrieves some raw byte data for a given key from a Document.
// Return nil if there is no data for the key.
func (doc Document) GetRaw(key string) Raw {
	return doc[key]
}

// SetRaw inserts some raw byte data for a given key into a Document.
// Any existing data at the given key is overwritten.
func (doc Document) SetRaw(key string, val Raw) {
	doc[key] = val
}

// Get retrieves some object for some given key from a Document.
// The data for the given key is decoded from its POLO form into the given object which must be a pointer.
// Returns an error if there is no data for the key or if the data could not be decoded into the given object.
func (doc Document) Get(key string, object any) error {
	// Retrieve the data for the key and error if unavailable
	var data []byte
	if data = doc.GetRaw(key); data == nil {
		return fmt.Errorf("document value not found for key '%v'", key)
	}

	// Depolorize the data into the given object and return any error
	if err := Depolorize(object, data); err != nil {
		return fmt.Errorf("document value could not be decoded for key '%v': %w", key, err)
	}

	return nil
}

// Set inserts some object for some given key into a Document.
// The given object is encoded into its POLO form and inserted, overwriting any existing data.
// Returns an error if the given object cannot be serialized with Polorize().
func (doc Document) Set(key string, object any, options ...EncodingOptions) error {
	// Polorize the object into its wire form. Return any error that occurs
	data, err := Polorize(object, options...)
	if err != nil {
		return fmt.Errorf("document value could not be encoded for key '%v': %w", key, err)
	}

	// Insert the wire data for the key
	doc.SetRaw(key, data)

	return nil
}
