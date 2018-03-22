package bson_test

import (
	"bytes"

	"github.com/globalsign/mgo/bson"
	. "gopkg.in/check.v1"
)

// Reusing sampleItems from bson_test

func (s *S) TestEncodeSampleItems(c *C) {
	for i, item := range sampleItems {
		buf := bytes.NewBuffer(nil)
		enc := bson.NewEncoder(buf)

		err := enc.Encode(item.obj)
		c.Assert(err, IsNil)
		c.Assert(string(buf.Bytes()), Equals, item.data, Commentf("Failed on item %d", i))
	}
}

func (s *S) TestDecodeSampleItems(c *C) {
	for i, item := range sampleItems {
		buf := bytes.NewBuffer([]byte(item.data))
		dec := bson.NewDecoder(buf)

		value := bson.M{}
		err := dec.Decode(&value)
		c.Assert(err, IsNil)
		c.Assert(value, DeepEquals, item.obj, Commentf("Failed on item %d", i))
	}
}

func (s *S) TestStreamRoundTrip(c *C) {
	buf := bytes.NewBuffer(nil)
	enc := bson.NewEncoder(buf)

	for _, item := range sampleItems {
		err := enc.Encode(item.obj)
		c.Assert(err, IsNil)
	}

	// Ensure that everything that was encoded is decodable in the same order.
	dec := bson.NewDecoder(buf)
	for i, item := range sampleItems {
		value := bson.M{}
		err := dec.Decode(&value)
		c.Assert(err, IsNil)
		c.Assert(value, DeepEquals, item.obj, Commentf("Failed on item %d", i))
	}
}
