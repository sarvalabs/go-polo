package polo

import "math"

// wireConfig defines the wire encoding/decoding
// configuration for Polorizer or Depolorizer
type wireConfig struct {
	packBytes  bool
	docStructs bool
	docStrMaps bool

	// maxDepth is the maximum nesting depth of compound wires a decoder descends into (0 = unlimited)
	maxDepth int32
}

// DefaultMaxDepth is the default maximum nesting depth of compound wires for decoding.
// It bounds the recursion of the decoder so that a deeply nested wire cannot overflow the stack.
const DefaultMaxDepth = 10000

// defaultConfig returns a default wireConfig object
func defaultWireConfig() *wireConfig {
	return &wireConfig{
		packBytes:  false,
		docStructs: false,
		docStrMaps: false,
		maxDepth:   DefaultMaxDepth,
	}
}

// checkDepth returns ErrMaxDepthExceeded if the given nesting depth exceeds the configured maximum
func (cfg wireConfig) checkDepth(depth int) error {
	if cfg.maxDepth > 0 && depth > int(cfg.maxDepth) {
		return ErrMaxDepthExceeded
	}

	return nil
}

func (cfg *wireConfig) apply(options ...EncodingOptions) {
	for _, opt := range options {
		opt(cfg)
	}
}

// EncodingOptions represents options that can be provided to
// encoding/decoding functions or buffers to modify the wire form
type EncodingOptions func(*wireConfig)

// PackedBytes is an EncodingOption that sets the encoding/decoding
// to allow interpretation of bytes from pack-encoded uint8 values
func PackedBytes() EncodingOptions {
	return func(config *wireConfig) {
		config.packBytes = true
	}
}

// DocStructs is an EncodingOption that sets the encoding/decoding
// to interpret structs as document-encoded values instead of a pack
func DocStructs() EncodingOptions {
	return func(config *wireConfig) {
		config.docStructs = true
	}
}

// DocStringMaps is an EncodingOption that sets the encoding/decoding to
// interpret string keyed maps as document-encoded values instead of a pack
func DocStringMaps() EncodingOptions {
	return func(config *wireConfig) {
		config.docStrMaps = true
	}
}

// MaxDepth is an EncodingOption that sets the maximum nesting depth of compound wires
// (packs and documents) that a decoder will descend into. Deeper wires are rejected with
// ErrMaxDepthExceeded. Defaults to DefaultMaxDepth. A depth less than 1 removes the limit.
func MaxDepth(depth int) EncodingOptions {
	return func(config *wireConfig) {
		switch {
		case depth < 1:
			config.maxDepth = 0
		case depth > math.MaxInt32:
			config.maxDepth = math.MaxInt32
		default:
			config.maxDepth = int32(depth)
		}
	}
}

// inheritCfg is an EncodingOption that inherits the full config
func inheritCfg(inherit wireConfig) EncodingOptions {
	return func(config *wireConfig) {
		*config = inherit
	}
}
