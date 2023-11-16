package polo

// wireConfig defines the wire encoding/decoding
// configuration for Polorizer or Depolorizer
type wireConfig struct {
	packBytes  bool
	docStructs bool
	docStrMaps bool
}

// defaultConfig returns a default wireConfig object
func defaultWireConfig() *wireConfig {
	return &wireConfig{
		packBytes:  false,
		docStructs: false,
		docStrMaps: false,
	}
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
// to interpret bytes as pack-encoded uint8 values instead of a word
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

func InheritConfig(inherit wireConfig) EncodingOptions {
	return func(config *wireConfig) {
		*config = inherit
	}
}
