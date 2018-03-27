package bson

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// Minimal BSON document: int32 (size) + 0x00 (end of document)
	minDocumentSize = 5
	// Max size = 16 MiB
	// (see https://docs.mongodb.com/manual/reference/limits/)
	maxDocumentSize = 16777216
)

// ErrInvalidDocumentSize is an error returned when a BSON document's header
// contains a size smaller than minDocumentSize or greater than maxDocumentSize.
type ErrInvalidDocumentSize struct {
	DocumentSize int32
}

func (e ErrInvalidDocumentSize) Error() string {
	return fmt.Sprintf("invalid document size %d", e.DocumentSize)
}

// A Decoder reads and decodes BSON values from an input stream.
type Decoder struct {
	source io.Reader
}

// NewDecoder returns a new Decoder that reads from source.
// It does not add any extra buffering, and may not read data from source beyond the BSON values requested.
func NewDecoder(source io.Reader) *Decoder {
	return &Decoder{source: source}
}

// Decode reads the next BSON-encoded value from its input and stores it in the value pointed to by v.
// See the documentation for Unmarshal for details about the conversion of BSON into a Go value.
func (dec *Decoder) Decode(v interface{}) (err error) {
	// BSON documents start with their size as a *signed* int32.
	var docSize int32
	if err = binary.Read(dec.source, binary.LittleEndian, &docSize); err != nil {
		return
	}

	if docSize < minDocumentSize || docSize > maxDocumentSize {
		return ErrInvalidDocumentSize{DocumentSize: docSize}
	}

	docBuffer := bytes.NewBuffer(make([]byte, 0, docSize))
	if err = binary.Write(docBuffer, binary.LittleEndian, docSize); err != nil {
		return
	}

	// docSize is the *full* document's size (including the 4-byte size header,
	// which has already been read).
	if _, err = io.CopyN(docBuffer, dec.source, int64(docSize-4)); err != nil {
		return
	}

	// Let Unmarshal handle the rest.
	defer handleErr(&err)
	return Unmarshal(docBuffer.Bytes(), v)
}

// An Encoder encodes and writes BSON values to an output stream.
type Encoder struct {
	target io.Writer
}

// NewEncoder returns a new Encoder that writes to target.
func NewEncoder(target io.Writer) *Encoder {
	return &Encoder{target: target}
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
