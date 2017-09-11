package mgo

import (
	"testing"

	"github.com/globalsign/mgo/bson"
)

// This file is for testing functions that are not exported outside the mgo
// package - avoid doing so if at all possible.

// Ensures indexed int64 fields do not cause mgo to panic.
//
// See https://github.com/globalsign/mgo/pull/23
func TestIndexedInt64FieldsBug(t *testing.T) {
	input := bson.D{
		{Name: "testkey", Value: int(1)},
		{Name: "testkey", Value: int64(1)},
		{Name: "testkey", Value: "test"},
		{Name: "testkey", Value: float64(1)},
	}

	_ = simpleIndexKey(input)
}
