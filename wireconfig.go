package polo

import "math"

// wireConfig defines the wire encoding/decoding
// configuration for Polorizer or Depolorizer
type wireConfig struct {
	packBytes  bool
	docStructs bool
	docStrMaps bool

	// docExtensible permits absent keys and unknown keys when a struct is decoded from a document
	docExtensible bool

	// maxDepth is the maximum nesting depth of compound wires a decoder descends into (0 = unlimited)
	maxDepth int32
}

// DefaultMaxDepth is the default maximum nesting depth of compound wires for decoding.
// It bounds the recursion of the decoder so that a deeply nested wire cannot overflow the stack.
// rs-polo has the same default, so change both together: 1000 levels decode and 1001 levels are errors.
const DefaultMaxDepth = 1000

// defaultConfig returns a default wireConfig object
func defaultWireConfig() *wireConfig {
	return &wireConfig{
		packBytes:     false,
		docStructs:    false,
		docStrMaps:    false,
		docExtensible: false,
		maxDepth:      DefaultMaxDepth,
	}
}

// custom returns the wireConfig for the Depolorizer that a custom Depolorize method receives.
// It keeps only the nesting limit. The options that select a wire form (PackedBytes, DocStructs,
// DocStringMaps) are not kept, because a custom Polorize method does not receive the options of
// the caller: it writes the default forms, so its decoder must accept the default forms.
func (cfg wireConfig) custom() wireConfig {
	return wireConfig{maxDepth: cfg.maxDepth}
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
// encoding/decoding functions or buffers to modify the wire form.
//
// PackedBytes, DocStructs and DocStringMaps select the wire form of a type. Each of them is a switch:
// with the option, the encoder writes only the other form and the decoder accepts only that form.
// Thus, the encoder and the decoder must use the same options.
//
// These options do not reach a type with a custom Polorize or Depolorize method. A custom Polorize
// method creates its own Polorizer and does not receive the options of the caller. For the same reason,
// the Depolorizer that a custom Depolorize method receives has the default forms. It keeps only MaxDepth.
type EncodingOptions func(*wireConfig)

// PackedBytes is an EncodingOption that sets the encoding/decoding of bytes ([]byte and [N]byte)
// to a pack of 8-bit integers instead of a word. With this option, the decoder rejects a word for bytes.
func PackedBytes() EncodingOptions {
	return func(config *wireConfig) {
		config.packBytes = true
	}
}

// DocStructs is an EncodingOption that sets the encoding/decoding of structs to a document
// instead of a pack. With this option, the decoder rejects a pack for a struct.
//
// The document must have exactly one key for each encoded field. The decoder rejects a document with
// an absent key or with an unknown key. Use ExtensibleDocStructs to decode a document that can have them.
func DocStructs() EncodingOptions {
	return func(config *wireConfig) {
		config.docStructs = true
	}
}

// ExtensibleDocStructs is an EncodingOption that is the same as DocStructs for the encoder.
// For the decoder, it also permits a document that does not have exactly the keys of the struct:
// the decoder ignores an unknown key, and it sets the field of an absent key to its zero value.
//
// A decode with this option is not canonical: different documents decode into the same struct.
// Do not use it for a wire that is hashed or signed.
func ExtensibleDocStructs() EncodingOptions {
	return func(config *wireConfig) {
		config.docStructs = true
		config.docExtensible = true
	}
}

// DocStringMaps is an EncodingOption that sets the encoding/decoding of string keyed maps to
// a document instead of a pack. With this option, the decoder rejects a pack for a string keyed map.
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
