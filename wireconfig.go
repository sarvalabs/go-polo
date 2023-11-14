package polo

// wireConfig defines the wire encoding/decoding
// configuration for Polorizer or Depolorizer
type wireConfig struct {
	packedBytes bool
}

// defaultConfig returns a default wireConfig object
func defaultWireConfig() *wireConfig {
	return &wireConfig{
		packedBytes: false,
	}
}

// EncodingOptions represents options that can be provided to
// encoding/decoding functions or buffers to modify the wire form
type EncodingOptions func(*wireConfig)

// PackedBytes is an EncodingOption
func PackedBytes() EncodingOptions {
	return func(config *wireConfig) {
		config.packedBytes = true
	}
}
