package scram_test

import (
	"testing"

	"github.com/globalsign/mgo/internal/scram"
	. "gopkg.in/check.v1"
)

var _ = Suite(&S{})

func Test(t *testing.T) { TestingT(t) }

type S struct{}

func (s *S) TestNewMethod(c *C) {
	var err error

	_, err = scram.NewMethod("SCRAM-SHA-1")
	c.Assert(err, Equals, IsNil)

	_, err = scram.NewMethod("SCRAM-SHA-256")
	c.Assert(err, IsNil)

	_, err = scram.NewMethod("example")
	c.Assert(err, NotNil)
}
