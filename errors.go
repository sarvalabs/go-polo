package polo

import "errors"

// varint errors
var (
	errVarintTerminated = errors.New("varint terminated prematurely")
	errVarintOverflow   = errors.New("varint overflows 64-bit integer")
)
