package mgo

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	. "gopkg.in/check.v1"
)

type S struct{}

var _ = Suite(&S{})

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

	indexes, err := simpleIndexKey(input)
	if err != nil {
		t.Error("Received unexpected error: ", err)
	}
	if len(input) != len(indexes) {
		t.Errorf("Expected %d indexes returned; received %d\n", len(input), len(indexes))
	}
}

// Ensures we don't panic (but do return an error) with bad index sort types
func TestIndexedUnknownTypeErr(t *testing.T) {
	input := bson.D{
		{Name: "testkey", Value: bool(false)},
	}

	_, err := simpleIndexKey(input)
	if err == nil {
		t.Error("Expected an error for unknown index value type (bool) but none received")
	}
}

func (s *S) TestGetRFC2253NameStringSingleValued(c *C) {
	var RDNElements = pkix.RDNSequence{
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "GO"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "MGO"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "MGO"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 10}, Value: "MGO"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 11}, Value: "Client"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "localhost"}},
	}

	c.Assert(getRFC2253NameString(&RDNElements), Equals, "CN=localhost,OU=Client,O=MGO,L=MGO,ST=MGO,C=GO")
}

func (s *S) TestGetRFC2253NameStringEscapeChars(c *C) {
	var RDNElements = pkix.RDNSequence{
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "GB"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "MGO "}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 10}, Value: "Sue, Grabbit and Runn < > ;"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "L. Eagle"}},
	}

	c.Assert(getRFC2253NameString(&RDNElements), Equals, "CN=L. Eagle,O=Sue\\, Grabbit and Runn \\< \\> \\;,ST=MGO\\ ,C=GB")
}

func (s *S) TestGetRFC2253NameStringMultiValued(c *C) {
	var RDNElements = pkix.RDNSequence{
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 10}, Value: "Widget Inc."}},
		{{Type: asn1.ObjectIdentifier{2, 5, 4, 11}, Value: "Sales"}, {Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "J. Smith"}},
	}

	c.Assert(getRFC2253NameString(&RDNElements), Equals, "OU=Sales+CN=J. Smith,O=Widget Inc.,C=US")
}

func (s *S) TestDialTimeouts(c *C) {
	info := &DialInfo{}

	c.Assert(info.readTimeout(), Equals, time.Duration(0))
	c.Assert(info.writeTimeout(), Equals, time.Duration(0))
	c.Assert(info.roundTripTimeout(), Equals, time.Duration(0))

	info.Timeout = 60 * time.Second
	c.Assert(info.readTimeout(), Equals, 60*time.Second)
	c.Assert(info.writeTimeout(), Equals, 60*time.Second)
	c.Assert(info.roundTripTimeout(), Equals, 120*time.Second)

	info.ReadTimeout = time.Second
	c.Assert(info.readTimeout(), Equals, time.Second)

	info.WriteTimeout = time.Second
	c.Assert(info.writeTimeout(), Equals, time.Second)
}

func makeTestServer(addr string, master bool, mongos bool, ping time.Duration, tags bson.D) *mongoServer {
	return &mongoServer{
		Addr:         addr,
		ResolvedAddr: addr,
		info: &mongoServerInfo{
			Master: master,
			Mongos: mongos,
			Tags:   tags,
		},
		pingValue: ping,
	}
}

func (s *S) TestReadPreferenceTagSetsPrioritisedPrimaryPreferred(c *C) {
	// Issue #300: With PrimaryPreferred, a secondary matching the first tag set
	// must be chosen over a primary that only matches a fallback empty tag set.
	primary := makeTestServer("1.1.1.1:27017", true, false, 5*time.Millisecond, nil)
	secondary := makeTestServer("1.1.1.2:27017", false, false, 50*time.Millisecond, bson.D{{Name: "foo", Value: "bar"}})

	servers := &mongoServers{}
	servers.Add(primary)
	servers.Add(secondary)

	tagSets := []bson.D{
		{{Name: "foo", Value: "bar"}},
		{},
	}

	best := servers.BestFit(PrimaryPreferred, tagSets)
	c.Assert(best, Equals, secondary)
}

func (s *S) TestReadPreferenceTagSetsPriorityOrder(c *C) {
	// The first matching tag set must be chosen even if a server matching a later tag set has lower ping.
	serverEast := makeTestServer("1.1.1.1:27017", false, false, 100*time.Millisecond, bson.D{{Name: "dc", Value: "east"}})
	serverWest := makeTestServer("1.1.1.2:27017", false, false, 5*time.Millisecond, bson.D{{Name: "dc", Value: "west"}})

	servers := &mongoServers{}
	servers.Add(serverEast)
	servers.Add(serverWest)

	// Priority: east then west
	tagSetsEastFirst := []bson.D{
		{{Name: "dc", Value: "east"}},
		{{Name: "dc", Value: "west"}},
	}
	best := servers.BestFit(SecondaryPreferred, tagSetsEastFirst)
	c.Assert(best, Equals, serverEast)

	// Priority: west then east
	tagSetsWestFirst := []bson.D{
		{{Name: "dc", Value: "west"}},
		{{Name: "dc", Value: "east"}},
	}
	best = servers.BestFit(SecondaryPreferred, tagSetsWestFirst)
	c.Assert(best, Equals, serverWest)
}

func (s *S) TestReadPreferenceTagSetsFallback(c *C) {
	// If no server matches the first tag set, fall back to the second tag set.
	server1 := makeTestServer("1.1.1.1:27017", false, false, 10*time.Millisecond, bson.D{{Name: "use", Value: "reporting"}})

	servers := &mongoServers{}
	servers.Add(server1)

	tagSets := []bson.D{
		{{Name: "use", Value: "nonexistent"}},
		{{Name: "use", Value: "reporting"}},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, Equals, server1)
}

func (s *S) TestReadPreferenceTagSetsMultipleTagsInSet(c *C) {
	// A server must match all tags within a single tag set.
	serverA := makeTestServer("1.1.1.1:27017", false, false, 10*time.Millisecond, bson.D{{Name: "dc", Value: "east"}, {Name: "rack", Value: "1"}})
	serverB := makeTestServer("1.1.1.2:27017", false, false, 10*time.Millisecond, bson.D{{Name: "dc", Value: "east"}, {Name: "rack", Value: "2"}})

	servers := &mongoServers{}
	servers.Add(serverA)
	servers.Add(serverB)

	tagSets := []bson.D{
		{{Name: "dc", Value: "east"}, {Name: "rack", Value: "2"}},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, Equals, serverB)
}

func (s *S) TestReadPreferenceTagSetsMultipleServersInFirstMatch(c *C) {
	// If multiple servers match the first tag set, the nearest server among them is selected.
	slowMatch := makeTestServer("1.1.1.1:27017", false, false, 80*time.Millisecond, bson.D{{Name: "env", Value: "prod"}})
	fastMatch := makeTestServer("1.1.1.2:27017", false, false, 10*time.Millisecond, bson.D{{Name: "env", Value: "prod"}})
	fastestOther := makeTestServer("1.1.1.3:27017", false, false, 1*time.Millisecond, bson.D{{Name: "env", Value: "dev"}})

	servers := &mongoServers{}
	servers.Add(slowMatch)
	servers.Add(fastMatch)
	servers.Add(fastestOther)

	tagSets := []bson.D{
		{{Name: "env", Value: "prod"}},
		{{Name: "env", Value: "dev"}},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, Equals, fastMatch)
}

func (s *S) TestReadPreferenceTagSetsSecondaryModeSkipsPrimary(c *C) {
	// In Secondary mode, a master matching the first tag set is not eligible, so it falls back to a secondary.
	primary := makeTestServer("1.1.1.1:27017", true, false, 5*time.Millisecond, bson.D{{Name: "dc", Value: "east"}})
	secondary := makeTestServer("1.1.1.2:27017", false, false, 20*time.Millisecond, bson.D{{Name: "dc", Value: "west"}})

	servers := &mongoServers{}
	servers.Add(primary)
	servers.Add(secondary)

	tagSets := []bson.D{
		{{Name: "dc", Value: "east"}},
		{{Name: "dc", Value: "west"}},
	}

	best := servers.BestFit(Secondary, tagSets)
	c.Assert(best, Equals, secondary)
}

func (s *S) TestReadPreferenceTagSetsNoMatchReturnsNil(c *C) {
	server := makeTestServer("1.1.1.1:27017", false, false, 10*time.Millisecond, bson.D{{Name: "dc", Value: "east"}})

	servers := &mongoServers{}
	servers.Add(server)

	tagSets := []bson.D{
		{{Name: "dc", Value: "north"}},
		{{Name: "dc", Value: "south"}},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, IsNil)
}

func (s *S) TestReadPreferenceTagSetsEmptySetWildcard(c *C) {
	server1 := makeTestServer("1.1.1.1:27017", false, false, 30*time.Millisecond, bson.D{{Name: "rack", Value: "1"}})
	server2 := makeTestServer("1.1.1.2:27017", false, false, 10*time.Millisecond, bson.D{{Name: "rack", Value: "2"}})

	servers := &mongoServers{}
	servers.Add(server1)
	servers.Add(server2)

	tagSets := []bson.D{
		{{Name: "rack", Value: "99"}},
		{},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, Equals, server2)
}

func (s *S) TestReadPreferenceTagSetsMongos(c *C) {
	mongos := makeTestServer("1.1.1.1:27017", false, true, 10*time.Millisecond, nil)

	servers := &mongoServers{}
	servers.Add(mongos)

	tagSets := []bson.D{
		{{Name: "dc", Value: "east"}},
	}

	best := servers.BestFit(SecondaryPreferred, tagSets)
	c.Assert(best, Equals, mongos)
}
