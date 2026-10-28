package polo

import (
	"bytes"
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
// Returns an error if the first tag in the head is malformed or declares an offset outside the body.
// Returns an error if the framing is not canonical (C6): a pack without elements must
// not have a body, and the first element must start at the beginning of the body.
func newpackbuffer(head, body []byte) (*packbuffer, error) {
	// A pack without elements has nothing in its body
	if len(head) == 0 && len(body) != 0 {
		return nil, MalformedTagError{"data in a pack without elements"}
	}

	// Initialize a packbuffer with a reader over the head data and the body
	lr := &packbuffer{head: bytes.NewReader(head), body: body}

	// Seed the offset values of the packbuffer by iterating once
	if _, err := lr.next(); err != nil {
		return nil, err
	}

	// The first element must start at offset zero (no bytes before it in the body)
	if lr.noff > 0 {
		return nil, MalformedTagError{"non-zero offset for the first element"}
	}

	return lr, nil
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
// Returns an error if the tag for the next element is malformed or if its offset
// precedes the offset of the current element or lies outside the body.
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

		// Create a readbuffer from the current wiretype and the rest of data in the body and return it.
		// The current offset has already been checked against the body when its tag was consumed.
		return readbuffer{lr.cw, lr.body[lr.coff:]}, nil
	}

	// Attempt to consume a minimally encoded varint from the head reader
	tag, _, err := consumeTag(lr.head)
	if err != nil {
		return readbuffer{}, MalformedTagError{err.Error()}
	}

	// The tag's offset is where the next element starts in the body. It must not precede
	// the start of the current element and must not exceed the length of the body.
	// Compared as uint64 so that offsets that do not fit in an int cannot wrap around.
	offset := tag >> 4
	if offset < uint64(lr.noff) || offset > uint64(len(lr.body)) { //nolint:gosec // noff and len are never negative
		return readbuffer{}, MalformedTagError{"element offset out of bounds"}
	}

	// Update the current values from the next values
	lr.coff, lr.cw = lr.noff, lr.nw
	// Set the next values based on the tag data (first 4 bits represent wiretype, rest the offset position of the dats)
	lr.noff, lr.nw = int(offset), WireType(tag&15)

	// Create a readbuffer from the current wiretype and body bytes between the two offset positions
	return readbuffer{lr.cw, lr.body[lr.coff:lr.noff]}, nil
}

// prepend is a generic function that accepts an object of some any type and a slice of objects of
// the same type and inserts the object to the front of the slice and shifts the other elements.
func prepend[Element any](y Element, x []Element) []Element {
	x = append(x, *new(Element))
	copy(x[1:], x)
	x[0] = y

	return x
}
