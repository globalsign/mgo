package dbtest_test

import (
	"os"
	"testing"
	"time"

	mgo "github.com/globalsign/mgo"
	. "gopkg.in/check.v1"

	"github.com/globalsign/mgo/dbtest"
)

type M map[string]interface{}

func TestAll(t *testing.T) {
	TestingT(t)
}

type S struct {
	oldCheckSessions string
}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	s.oldCheckSessions = os.Getenv("CHECK_SESSIONS")
	os.Setenv("CHECK_SESSIONS", "")
}

func (s *S) TearDownTest(c *C) {
	os.Setenv("CHECK_SESSIONS", s.oldCheckSessions)
}

func (s *S) TestWipeData(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	session := server.Session()
	err := session.DB("mydb").C("mycoll").Insert(M{"a": 1})
	session.Close()
	c.Assert(err, IsNil)

	server.Wipe()

	session = server.Session()
	names, err := session.DatabaseNames()
	session.Close()
	c.Assert(err, IsNil)
	for _, name := range names {
		if name != "local" && name != "admin" {
			c.Fatalf("Wipe should have removed this database: %s", name)
		}
	}
}

func (s *S) TestStop(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	// Server should not be running.
	process := server.ProcessTest()
	c.Assert(process, IsNil)

	session := server.Session()
	addr := session.LiveServers()[0]
	session.Close()

	// Server should be running now.
	process = server.ProcessTest()
	p, err := os.FindProcess(process.Pid)
	c.Assert(err, IsNil)
	p.Release()

	server.Stop()

	// Server should not be running anymore.
	session, _ = mgo.DialWithTimeout(addr, 500*time.Millisecond)
	if session != nil {
		session.Close()
		c.Fatalf("Stop did not stop the server")
	}
}

func (s *S) TestCheckSessions(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	session := server.Session()
	defer session.Close()
	c.Assert(server.Wipe, PanicMatches, "There are mgo sessions still alive.")
}

func (s *S) TestCheckSessionsDisabled(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	os.Setenv("CHECK_SESSIONS", "0")

	// Should not panic, although it looks to Wipe like this session will leak.
	session := server.Session()
	defer session.Close()
	server.Wipe()
}

func (s *S) TestSetEngine(c *C) {
	status := struct {
		StorageEngine struct {
			Name string `bson:"name"`
		} `bson:"storageEngine"`
	}{}
	wtStatus := status

	// mmapv1 (default)
	var mmapServer dbtest.DBServer
	mmapServer.SetPath(c.MkDir())
	defer mmapServer.Stop()

	mSession := mmapServer.Session()
	defer mSession.Close()

	err := mSession.Run("serverStatus", &status)
	c.Assert(err, IsNil)
	c.Assert(status.StorageEngine.Name, Equals, "mmapv1")

	// wiredTiger
	var wtServer dbtest.DBServer
	wtServer.SetPath(c.MkDir())
	wtServer.SetEngine("wiredTiger")
	defer wtServer.Stop()

	wSession := wtServer.Session()
	defer wSession.Close()

	err = wSession.Run("serverStatus", &wtStatus)
	c.Assert(err, IsNil)
	c.Assert(wtStatus.StorageEngine.Name, Equals, "wiredTiger")
}

func (s *S) TestReplicaSet(c *C) {

	type Member struct {
		State string `bson:"stateStr"`
	}
	type ReplSetStatus struct {
		Members []Member `bson:"members"`
	}
	rsStatus := ReplSetStatus{}

	var mServer dbtest.DBServer
	mServer.SetPath(c.MkDir())
	mServer.SetEngine("wiredTiger")
	mServer.SetReplicaSet(true)

	defer mServer.Stop()

	mSession := mServer.Session()
	defer mSession.Close()

	err := mSession.Run("replSetGetStatus", &rsStatus)
	c.Assert(err, IsNil)
	c.Assert(len(rsStatus.Members), Equals, 1)
	c.Assert(rsStatus.Members[0].State, Equals, "PRIMARY")
}
