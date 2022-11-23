package polo

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
