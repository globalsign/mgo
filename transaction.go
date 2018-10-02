package mgo

// mgo - MongoDB driver for Go
//
// Copyright (c) 2010-2012 - Gustavo Niemeyer <gustavo@niemeyer.net>
// transaction.go (c) 2018 Russell Miller/The Home Depot <russell_j_miller@homedepot.com>
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

// The transaction struct is only initialized with a valid Session, and that does not
// change.  The struct contains state information for the transaction.  The transaction
// is started when the first write operation is created using it, and it is finished when
// it is either committed or aborted.  If the session is killed out from under it, of
// course, it will be left in an inconsistent state, but the transaction will be dead and
// presumably already aborted.
type Transaction struct {
	session   *Session
	started   bool
	finished  bool
	txnNumber int64
}

// NewTransaction creates a new Transaction object.
func NewTransaction(s *Session) Transaction {
	return Transaction{
		session: s,
	}
}

// Commit commits and finalizes the transaction.
func (t *Transaction) Commit() error {
	// check errors
	err := t.session.CommitTransaction(t.txnNumber)
	t.finished = true
	return err
}

// Abort aborts and closes the transaction.
func (t *Transaction) Abort() error {
	// check errors
	err := t.session.AbortTransaction(t.txnNumber)
	t.finished = true
	return err
}
