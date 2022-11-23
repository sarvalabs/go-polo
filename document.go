package polo

import (
	"fmt"
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

// GetObject retrieves some object for a given from a Document.
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

// SetObject inserts some object for a given key into a Document.
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
