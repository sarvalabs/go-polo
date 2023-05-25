package polo

// writebuffer is a write-only byte buffer that appends to a head and body
// buffer simultaneously. It can be instantiated without a constructor
type writebuffer struct {
	head, body      []byte
	offset, counter uint64
}

// write appends v to the body of the writebuffer and a varint tag describing its offset and the given WireType.
// The writebuffer.offset value is incremented by the length of v after both buffers have been updated.
func (wb *writebuffer) write(w WireType, v []byte) {
	wb.head = appendVarint(wb.head, (wb.offset<<4)|uint64(w))
	wb.body = append(wb.body, v...)

	// Increment the offset by the number of bytes written to the body
	wb.offset += uint64(len(v))
	// Increment counter to represent the number of written elements
	wb.counter++
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
