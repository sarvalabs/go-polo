package polo

import (
	"bytes"
	"errors"
	"io"
)

// packbuffer is a read-only buffer that is obtained from a compound wire (pack).
// Iteration over the packbuffer will return elements from the load one by one.
type packbuffer struct {
	head *bytes.Reader
	body []byte

	coff int // represents the offset position for the current element
	noff int // represents the offset position for the next element

	cw WireType // represents the wiretype of the current element
	nw WireType // represents the wiretype of the next element
}

// newpackbuffer creates a new packbuffer for a given head and body slice of bytes.
// The returned packbuffer is seeded and the next iteration will return the first element of the load.
func newpackbuffer(head, body []byte) *packbuffer {
	// Initialize an empty packbuffer
	lr := new(packbuffer)

	// Create a reader from the head data and set it
	lr.head = bytes.NewReader(head)
	lr.body = body

	// Seed the offset values of the packbuffer by iterating once
	_, _ = lr.next()

	return lr
}

// done returns whether all elements in the packbuffer have been read.
func (lr *packbuffer) done() bool {
	// packbuffer is done if the noff is set to -1
	return lr.noff == -1
}

// peek returns the WireType of the next element along with a boolean.
// Returns (WireNull, false) if there are no elements left in the packbuffer.
func (lr *packbuffer) peek() (WireType, bool) {
	if lr.done() {
		return WireNull, false
	}

	return lr.nw, true
}

// next returns the next element from the packbuffer.
// Returns an error if packbuffer is done. (can be checked with a call to done())
func (lr *packbuffer) next() (readbuffer, error) {
	// Check if the head reader is exhausted
	if lr.head.Len() == 0 {
		// Check if load reader is done
		if lr.done() {
			return readbuffer{}, ErrInsufficientWire
		}

		// Update current values from the next values
		lr.coff, lr.cw = lr.noff, lr.nw
		// Update next values to nulls. -1 means the packbuffer is set as done
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
