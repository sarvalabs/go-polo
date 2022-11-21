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

// MalformedTagError is an error for when a consumed varint for a tag is malformed
type MalformedTagError struct {
	msg string
}

// Error implements the error interface for MalformedTagError
func (err MalformedTagError) Error() string {
	return fmt.Sprintf("malformed tag: %v", err.msg)
}
