// mgo - MongoDB driver for Go
//
// Copyright (c) 2019 - Canonical Ltd
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package mgo_test

import (
	"math/rand"
	"regexp"
	"sync"
	"time"

	. "gopkg.in/check.v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func (s *S) setupTxnSession(c *C) *mgo.Session {
	// get the test infrastructure ready for doing transactions.
	if !s.versionAtLeast(4, 0) {
		c.Skip("transactions not supported before 4.0")
	}
	session, err := mgo.Dial("localhost:40011")
	c.Assert(err, IsNil)
	return session
}

func (s *S) setupTxnSessionAndCollection(c *C) (*mgo.Session, *mgo.Collection) {
	session := s.setupTxnSession(c)
	// Collections must be created outside of a transaction
	coll := session.DB("mydb").C("mycoll")
	err := coll.Create(&mgo.CollectionInfo{})
	if err != nil {
		session.Close()
		c.Assert(err, IsNil)
	}
	return session, coll
}

func (s *S) setup2Sessions(c *C) (*mgo.Session, *mgo.Collection, *mgo.Session, *mgo.Collection) {
	session1, coll1 := s.setupTxnSessionAndCollection(c)
	session2 := session1.Copy()
	coll2 := session2.DB("mydb").C("mycoll")
	return session1, coll1, session2, coll2
}

func (s *S) TestTransactionInsertCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	var res bson.M
	// Should be visible in the session that has the transaction
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	// Since the change was made in a transaction, session 2 should not see the document
	err := coll2.Find(bson.M{"a": "a"}).One(&res)
	c.Check(err, Equals, mgo.ErrNotFound)
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, session2 should see it
	err = coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res)
	c.Check(err, IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
}

func (s *S) TestTransactionInsertAborted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	var res bson.M
	// Should be visible in the session that has the transaction
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	// Since the change was made in a transaction, session 2 should not see the document
	err := coll2.Find(bson.M{"a": "a"}).One(&res)
	c.Check(err, Equals, mgo.ErrNotFound)
	c.Assert(session1.AbortTransaction(), IsNil)
	// Since it is Aborted, nobody should see the object
	err = coll2.Find(bson.M{"a": "a"}).One(&res)
	c.Check(err, Equals, mgo.ErrNotFound)
	err = coll1.Find(bson.M{"a": "a"}).One(&res)
	c.Check(err, Equals, mgo.ErrNotFound)

}

func (s *S) TestTransactionUpdateCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	c.Assert(coll1.Update(bson.M{"a": "a"}, bson.M{"$set": bson.M{"b": "c"}}), IsNil)
	// Should be visible in the session that has the transaction
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
	// Since the change was made in a transaction, session 2 should not see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, session2 should see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
}

func (s *S) TestTransactionUpdateAllCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(coll1.Insert(bson.M{"a": "2", "b": "b"}), IsNil)
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	changeInfo, err := coll1.UpdateAll(nil, bson.M{"$set": bson.M{"b": "c"}})
	c.Assert(err, IsNil)
	c.Check(changeInfo.Matched, Equals, 2)
	c.Check(changeInfo.Updated, Equals, 2)
	// Should be visible in the session that has the transaction
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
	c.Assert(coll1.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "c"})
	// Since the change was made in a transaction, session 2 should not see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(coll2.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "b"})
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, session2 should see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
	c.Assert(coll2.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "c"})
}

func (s *S) TestTransactionUpsertCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	// One Upsert updates, the other Upsert creates
	changeInfo, err := coll1.Upsert(bson.M{"a": "a"}, bson.M{"$set": bson.M{"b": "c"}})
	c.Assert(err, IsNil)
	c.Check(changeInfo.Matched, Equals, 1)
	c.Check(changeInfo.Updated, Equals, 1)
	changeInfo, err = coll1.Upsert(bson.M{"a": "2"}, bson.M{"$set": bson.M{"b": "c"}})
	c.Assert(err, IsNil)
	c.Check(changeInfo.Matched, Equals, 0)
	c.Check(changeInfo.UpsertedId, NotNil)
	// Should be visible in the session that has the transaction
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
	c.Assert(coll1.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "c"})
	// Since the change was made in a transaction, session 2 should not see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(coll2.Find(bson.M{"a": "2"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, session2 should see it
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "c"})
	c.Assert(coll2.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "c"})
}

func (s *S) TestTransactionRemoveCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	c.Assert(coll1.Remove(bson.M{"a": "a"}), IsNil)
	// Should be gone in the session that has the transaction
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	// Since the change was made in a transaction, session 2 should still see the document
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, it should be gone
	c.Assert(coll1.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(coll2.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
}

func (s *S) TestTransactionRemoveAllCommitted(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(coll1.Insert(bson.M{"a": "2", "b": "b"}), IsNil)
	c.Assert(session1.StartTransaction(), IsNil)
	// call Abort in case there is a problem, but ignore an error if it was committed,
	// otherwise the server will block in DropCollection because the transaction is active.
	defer session1.AbortTransaction()
	changeInfo, err := coll1.RemoveAll(bson.M{"a": bson.M{"$exists": true}})
	c.Assert(err, IsNil)
	c.Check(changeInfo.Matched, Equals, 2)
	c.Check(changeInfo.Removed, Equals, 2)
	// Should be gone in the session that has the transaction
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(coll1.Find(bson.M{"a": "2"}).One(&res), Equals, mgo.ErrNotFound)
	// Since the change was made in a transaction, session 2 should still see the document
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(coll2.Find(bson.M{"a": "2"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "2", "b": "b"})
	c.Assert(session1.CommitTransaction(), IsNil)
	// Now that it is committed, it should be gone
	c.Assert(coll1.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(coll1.Find(bson.M{"a": "2"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(coll2.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(coll2.Find(bson.M{"a": "2"}).One(&res), Equals, mgo.ErrNotFound)
}

func (s *S) TestStartAbortTransactionMultithreaded(c *C) {
	// While calling StartTransaction doesn't actually make sense, it shouldn't corrupt
	// memory to do so. This should trigger go's '-race' detector if we don't
	// have the code structured correctly.
	session := s.setupTxnSession(c)
	defer session.Close()
	// Collections must be created outside of a transaction
	coll1 := session.DB("mydb").C("mycoll")
	err := coll1.Create(&mgo.CollectionInfo{})
	c.Assert(err, IsNil)
	var wg sync.WaitGroup
	startFunc := func() {
		err := session.StartTransaction()
		if err != nil {
			// Don't use Assert as we are being called in a goroutine
			c.Check(err, ErrorMatches, "transaction already started")
		} else {
			c.Check(session.AbortTransaction(), IsNil)
		}
		wg.Done()
	}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go startFunc()
	}
	wg.Wait()
}

func (s *S) TestStartCommitTransactionMultithreaded(c *C) {
	// While calling StartTransaction doesn't actually make sense, it shouldn't corrupt
	// memory to do so. This should trigger go's '-race' detector if we don't
	// have the code structured correctly.
	session := s.setupTxnSession(c)
	defer session.Close()
	// Collections must be created outside of a transaction
	coll1 := session.DB("mydb").C("mycoll")
	err := coll1.Create(&mgo.CollectionInfo{})
	c.Assert(err, IsNil)
	var wg sync.WaitGroup
	startFunc := func() {
		err := session.StartTransaction()
		if err != nil {
			// Don't use Assert as we are being called in a goroutine
			c.Check(err, ErrorMatches, "transaction already started")
		} else {
			c.Check(session.CommitTransaction(), IsNil)
		}
		wg.Done()
	}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go startFunc()
	}
	wg.Wait()
}

func (s *S) TestAbortTransactionNotStarted(c *C) {
	session := s.setupTxnSession(c)
	defer session.Close()
	err := session.AbortTransaction()
	c.Assert(err, ErrorMatches, "no transaction in progress")
}

func (s *S) TestCommitTransactionNotStarted(c *C) {
	session := s.setupTxnSession(c)
	defer session.Close()
	err := session.CommitTransaction()
	c.Assert(err, ErrorMatches, "no transaction in progress")
}

func (s *S) TestAbortTransactionNoChanges(c *C) {
	session := s.setupTxnSession(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	c.Assert(session.AbortTransaction(), IsNil)
}

func (s *S) TestCommitTransactionNoChanges(c *C) {
	session := s.setupTxnSession(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	c.Assert(session.CommitTransaction(), IsNil)
}

func (s *S) TestAbortTransactionTwice(c *C) {
	session, coll := s.setupTxnSessionAndCollection(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	// The DB transaction isn't started until we make a change
	c.Assert(coll.Insert(bson.M{"a": "a"}), IsNil)
	c.Assert(session.AbortTransaction(), IsNil)
	err := session.AbortTransaction()
	c.Assert(err, ErrorMatches, "no transaction in progress")
}

func (s *S) TestCommitTransactionTwice(c *C) {
	session, coll := s.setupTxnSessionAndCollection(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	// The DB transaction isn't started until we make a change
	c.Assert(coll.Insert(bson.M{"a": "a"}), IsNil)
	c.Assert(session.CommitTransaction(), IsNil)
	err := session.CommitTransaction()
	c.Assert(err, ErrorMatches, "no transaction in progress")
}

func (s *S) TestStartTransactionTwice(c *C) {
	session, coll := s.setupTxnSessionAndCollection(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	c.Assert(coll.Insert(bson.M{"a": "a"}), IsNil)
	c.Assert(session.StartTransaction(), ErrorMatches, "transaction already started")
	c.Assert(coll.Insert(bson.M{"b": "b"}), IsNil)
	c.Assert(session.CommitTransaction(), IsNil)
	// Calling StartTransaction a second time doesn't currently abort
	// the txn in progress. Maybe we should as it is a sign that the driver
	// is being used incorrectly?
	var res bson.M
	c.Assert(coll.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a"})
	c.Assert(coll.Find(bson.M{"b": "b"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"b": "b"})
}

func (s *S) TestStartCommitAbortStartCommitTransaction(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(session1.StartTransaction(), IsNil)
	c.Assert(session1.CommitTransaction(), IsNil)
	err := session1.AbortTransaction()
	c.Assert(err, ErrorMatches, "no transaction in progress")
	// We should be able to recover
	c.Assert(session1.StartTransaction(), IsNil)
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	c.Assert(session1.CommitTransaction(), IsNil)
	var res bson.M
	// Should be visible in the session that has the transaction
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
}

func (s *S) TestCloseWithOpenTransaction(c *C) {
	session1, coll1, session2, coll2 := s.setup2Sessions(c)
	defer session1.Close()
	defer session2.Close()
	c.Assert(session1.StartTransaction(), IsNil)
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	var res bson.M
	c.Assert(coll1.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	// Close should abort the current session, which aborts the active transaction
	session1.Close()
	c.Assert(coll2.Find(bson.M{"a": "2"}).One(&res), Equals, mgo.ErrNotFound)
}

func (s *S) TestRefreshDuringTransaction(c *C) {
	session, coll := s.setupTxnSessionAndCollection(c)
	defer session.Close()
	c.Assert(session.StartTransaction(), IsNil)
	c.Assert(coll.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	// Refresh closes the active connection, but as long as we preserve the
	// SessionId and txnNumber, we should still be able to see the in-progress
	// changes
	session.Refresh()
	var res bson.M
	c.Assert(coll.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Assert(session.CommitTransaction(), IsNil)
	c.Assert(coll.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
}

func (s *S) TestCloneDifferentSessionTransaction(c *C) {
	session1, coll1 := s.setupTxnSessionAndCollection(c)
	defer session1.Close()
	c.Assert(session1.StartTransaction(), IsNil)
	c.Assert(coll1.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	session2 := session1.Clone()
	defer session2.Close()
	coll2 := session2.DB("mydb").C("mycoll")
	var res bson.M
	c.Check(coll2.Find(bson.M{"a": "a"}).One(&res), Equals, mgo.ErrNotFound)
	c.Assert(session2.StartTransaction(), IsNil)
	c.Assert(coll2.Insert(bson.M{"a": "second", "b": "c"}), IsNil)
	c.Check(coll1.Find(bson.M{"a": "second"}).One(&res), Equals, mgo.ErrNotFound)
	session1.CommitTransaction()
	session2.CommitTransaction()
	c.Check(coll2.Find(bson.M{"a": "a"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "a", "b": "b"})
	c.Check(coll1.Find(bson.M{"a": "second"}).Select(bson.M{"a": 1, "b": 1, "_id": 0}).One(&res), IsNil)
	c.Check(res, DeepEquals, bson.M{"a": "second", "b": "c"})
}

func (s *S) TestMultithreadedTransactionStartAbortAllActions(c *C) {
	session, coll := s.setupTxnSessionAndCollection(c)
	defer session.Close()
	// It isn't particularly sane to have one thread starting and aborting transactions
	// while other goroutines make modifications, but we want to be -race safe.
	// This doesn't assert that things are sequenced correctly, just that the driver
	// won't break if it is abused.
	c.Assert(coll.Insert(bson.M{"a": "a", "b": "b"}), IsNil)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	// Start and Abort the transaction every millisecond
	go func() {
		defer wg.Done()
		started := false
		for {
			select {
			case <-stop:
				if started {
					session.AbortTransaction()
					started = false
				}
				return
			case <-time.After(1 * time.Millisecond):
				if started {
					session.AbortTransaction()
					started = false
				} else {
					session.StartTransaction()
					started = true
				}
			}
		}
	}()
	possibleErrors := []string{
		"^Transaction .* has been aborted\\.$",
		"^Given transaction number .* does not match any in-progress transactions\\.$",
		"^Cannot specify 'startTransaction' on transaction .* since it is already in progress\\.$",
		"^Cannot start transaction .* on session .* because a newer transaction .* has already started\\.",
	}
	timeoutRegex := "i/o timeout"
	checkError := func(err error) {
		if err == nil {
			return
		}
		if err == mgo.ErrNotFound {
			return
		}
		errStr := err.Error()
		for _, errRegex := range possibleErrors {
			matched, _ := regexp.MatchString(errRegex, errStr)
			if matched {
				return
			}
		}
		if matched, _ := regexp.MatchString(timeoutRegex, errStr); matched {
			// When we get i/o timeout, that means we have to reset the Session
			session.Refresh()
			return
		}
		c.Errorf("error did not match a known response: %v", err.Error())
	}
	doInsert := func() {
		err := coll.Insert(bson.M{"a": "alt", "b": "something"})
		checkError(err)
	}
	doUpsert := func() {
		_, err := coll.Upsert(bson.M{"a": "upserted"}, bson.M{"$inc": bson.M{"b": 1}})
		checkError(err)
	}
	doRemove := func() {
		err := coll.Remove(bson.M{"a": "a"})
		checkError(err)
	}
	doFind := func() {
		var res bson.M
		err := coll.Find(bson.M{"a": "a"}).One(&res)
		if err == nil {
			c.Check(res["b"], Equals, "b")
		} else {
			checkError(err)
		}
	}
	funcs := []func(){
		doInsert,
		doUpsert,
		doRemove,
		doFind,
	}
	// do Insert/Update/Remove/Find concurrently with starting & aborting the transaction
	for _, tFunc := range funcs {
		wg.Add(1)
		go func(f func()) {
			for {
				nextSleep := time.Duration(rand.Int63n(int64(time.Millisecond)))
				select {
				case <-stop:
					wg.Done()
					return
				case <-time.After(nextSleep):
					f()
				}
			}
		}(tFunc)
	}
	// Let those run for a bit
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
