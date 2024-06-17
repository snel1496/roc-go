// Code generated by generate_bindings.py script from roc-streaming/bindgen
// roc-toolkit git tag: v0.4.0, commit: 62401be9

package roc

// Sample format.
//
// Defines how each sample is represented. Does not define channels layout and
// sample rate.
//
//go:generate stringer -type Format -trimprefix Format -output format_string.go
type Format int

const (
	// PCM floats.
	//
	// Uncompressed samples coded as 32-bit native-endian floats in range [-1; 1].
	// Channels are interleaved, e.g. two channels are encoded as "L R L R...".
	FormatPcmFloat32 Format = 1
)
