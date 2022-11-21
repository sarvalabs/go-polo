package polo

import (
	"errors"
	"fmt"
)

// varint errors
var (
	errVarintTerminated = errors.New("varint terminated prematurely")
	errVarintOverflow   = errors.New("varint overflows 64-bit integer")
)

var (
	// ErrNullStruct is an error for when a struct has been decoded from a null key
	ErrNullStruct = errors.New("null struct")
	// ErrObjectNotPtr is an error for when a non pointer object is passed to the Depolorize function
	ErrObjectNotPtr = errors.New("object not a pointer")
)

// MalformedTagError is an error for when a consumed varint for a tag is malformed
type MalformedTagError struct {
	msg string
}

// Error implements the error interface for MalformedTagError
func (err MalformedTagError) Error() string {
	return fmt.Sprintf("malformed tag: %v", err.msg)
}

// IncompatibleWireError is an error for when an object cannot be decoded from some wire data
type IncompatibleWireError struct {
	expected, actual WireType
}

// Error implements the error interface for IncompatibleWireError
func (err IncompatibleWireError) Error() string {
	return fmt.Sprintf("incompatible wire type. expected: %v. got: %v", err.expected, err.actual)
}

// DecodeError is an error for when an error occurs during decode/wire parsing
type DecodeError struct {
	msg string
}

// Error implements the error interface for DecodeError
func (err DecodeError) Error() string {
	return fmt.Sprintf("decode error: %v", err.msg)
}
