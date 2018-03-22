package bson

import (
	"encoding/binary"
	"io"
)

// A Decoder reads and decodes BSON values from an input stream.
type Decoder struct {
	source io.Reader
}

// NewDecoder returns a new Decoder that reads from source.
// It does not add any extra buffering, and may not read data from source beyond the BSON values requested.
func NewDecoder(source io.Reader) Decoder {
	return Decoder{source: source}
}

// Decode reads the next BSON-encoded value from its input and stores it in the value pointed to by v.
// See the documentation for Unmarshal for details about the conversion of BSON into a Go value.
func (dec *Decoder) Decode(v interface{}) error {
	// BSON documents start with their size as a uint32.
	document := make([]byte, 4)

	if _, err := io.ReadFull(dec.source, document); err != nil {
		return err
	}

	docSize := int(binary.LittleEndian.Uint32(document))

	// Read the rest of the BSON document.
	tailSize := docSize - 4
	tail := make([]byte, tailSize)
	if _, err := io.ReadFull(dec.source, tail); err != nil {
		return err
	}
	document = append(document, tail...)

	// Let Unmarshal handle the rest.
	return Unmarshal(document, v)
}

// An Encoder encodes and writes BSON values to an output stream.
type Encoder struct {
	target io.Writer
}

// NewEncoder returns a new Encoder that writes to target.
func NewEncoder(target io.Writer) Encoder {
	return Encoder{target: target}
}

// Encode encodes v to BSON, and if successful writes it to the Encoder's output stream.
// See the documentation for Marshal for details about the conversion of Go values to BSON.
func (enc *Encoder) Encode(v interface{}) error {
	data, err := Marshal(v)
	if err != nil {
		return err
	}

	_, err = enc.target.Write(data)
	return err
}
