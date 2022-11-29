package polo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// writebuffer is a write-only byte buffer that appends to a head and body
// buffer simultaneously. It can be instantiated without a constructor
type writebuffer struct {
	head   []byte
	body   []byte
	offset uint64
}

// write appends v to the body of the writebuffer and a varint tag describing its offset and the given WireType.
// The writebuffer.offset value is incremented by the length of v after both buffers have been updated.
func (wb *writebuffer) write(w WireType, v []byte) {
	wb.head = appendVarint(wb.head, (wb.offset<<4)|uint64(w))
	wb.body = append(wb.body, v...)
	wb.offset += uint64(len(v))
}

// bytes returns the contents of the writebuffer as a single slice of bytes.
// The returned bytes is the head followed by the body of the writebuffer.
func (wb writebuffer) bytes() []byte {
	return append(wb.head, wb.body...)
}

// load returns contents of the writebuffer as a slice of bytes tagged by a WireLoad,
// i.e, a load key with the length of the head is prefixed before the head followed by the body.
func (wb writebuffer) load() (buf []byte) {
	key := (uint64(len(wb.head)) << 4) | uint64(WireLoad)
	size := len(wb.head) + len(wb.body) + sizeVarint(key)

	buf = appendVarint(make([]byte, 0, size), key)
	buf = append(buf, wb.head...)
	buf = append(buf, wb.body...)

	return
}

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

// load returns a loadreader from a readbuffer.
// Throws an error if the wiretype of the readbuffer is not compound (pack)
func (rb *readbuffer) load() (*loadreader, error) {
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

	// Create a new loadreader and return it
	lr := newloadreader(head, body)

	return lr, nil
}

// loadreader is a read-only buffer that is obtained from a compound wire (pack).
// Iteration over the loadreader will return elements from the load one by one.
type loadreader struct {
	head *bytes.Reader
	body []byte

	coff int // represents the offset position for the current element
	noff int // represents the offset position for the next element

	cw WireType // represents the wiretype of the current element
	nw WireType // represents the wiretype of the next element
}

// newloadreader creates a new loadreader for a given head and body slice of bytes.
// The returned loadreader is seeded and the next iteration will return the first element of the load.
func newloadreader(head, body []byte) *loadreader {
	// Initialize an empty loadreader
	lr := new(loadreader)

	// Create a reader from the head data and set it
	lr.head = bytes.NewReader(head)
	lr.body = body

	// Seed the offset values of the loadreader by iterating once
	_, _ = lr.next()

	return lr
}

// done returns whether all elements in the loadreader have been read.
func (lr *loadreader) done() bool {
	// loadreader is done if the noff is set to -1
	return lr.noff == -1
}

// next returns the next element from the loadreader.
// Returns an error if loadreader is done. (can be checked with a call to done())
func (lr *loadreader) next() (readbuffer, error) {
	// Check if the head reader is exhausted
	if lr.head.Len() == 0 {
		// Check if load reader is done
		if lr.done() {
			return readbuffer{}, errors.New("loadreader exhausted")
		}

		// Update current values from the next values
		lr.coff, lr.cw = lr.noff, lr.nw
		// Update next values to nulls. -1 means the loadreader is set as done
		lr.noff, lr.nw = -1, WireNull

		// Create a readbuffer from the current wiretype and the rest of data in the body and return it
		return readbuffer{lr.cw, lr.body[lr.coff:]}, nil
	}

	// Attempt to consume a varint from the head reader
	tag, _, err := consumeVarint(lr.head)
	if err != nil {
		return readbuffer{}, MalformedTagError{err.Error()}
	}

	// Update the current values from the next values
	lr.coff, lr.cw = lr.noff, lr.nw
	// Set the next values based on the tag data (first 4 bits represent wiretype, rest the offset position of the dats)
	lr.noff, lr.nw = int(tag>>4), WireType(tag&15)

	// Create a readbuffer from the current wiretype and body bytes between the two offset positions
	return readbuffer{lr.cw, lr.body[lr.coff:lr.noff]}, nil
}

// read consumes n number of bytes from the reader and returns it as a slice of bytes.
// Throws an error if the r does not have n number of bytes.
func read(r io.Reader, n int) ([]byte, error) {
	// If n == 0, return an empty byte slice
	if n == 0 {
		return []byte{}, nil
	}

	// Create a slice of bytes with the specified length
	d := make([]byte, n)

	// Read from the reader into d
	if rn, err := r.Read(d); err != nil || rn != n {
		return nil, errors.New("insufficient data in reader")
	}

	return d, nil
}

// prepend is a generic function that accepts an object of some any type and a slice of objects of
// the same type and inserts the object to the front of the slice and shifts the other elements.
func prepend[Element any](y Element, x []Element) []Element {
	x = append(x, *new(Element))
	copy(x[1:], x)
	x[0] = y

	return x
}
