package polo

import (
	"bytes"
	"errors"
	"fmt"
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
