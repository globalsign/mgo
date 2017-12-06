// mgo - MongoDB driver for Go
//
// Copyright (c) 2010-2012 - Gustavo Niemeyer <gustavo@niemeyer.net>
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

package mgo

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/globalsign/mgo/bson"
)

type replyFunc func(err error, reply *replyOp, docNum int, docData []byte)

type opMsgReplyFunc func(reply *msgOp, err error)

type mongoSocket struct {
	sync.Mutex
	server          *mongoServer // nil when cached
	conn            net.Conn
	timeout         time.Duration
	addr            string // For debugging only.
	nextRequestId   uint32
	replyFuncs      map[uint32]replyFunc
	opMsgReplyFuncs map[uint32]opMsgReplyFunc
	references      int
	creds           []Credential
	cachedNonce     string
	gotNonce        sync.Cond
	dead            error
	serverInfo      *mongoServerInfo
	closeAfterIdle  bool
}

type queryOpFlags uint32

const (
	_ queryOpFlags = 1 << iota
	flagTailable
	flagSlaveOk
	flagLogReplay
	flagNoCursorTimeout
	flagAwaitData
	// section type, as defined here:
	// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#sections
	msgPayload0 = uint8(0)
	msgPayload1 = uint8(1)
	// all possible opCodes, as defined here:
	// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#request-opcodes
	opInvalid      = 0
	opReply        = 1
	dbMsg          = 1000
	dbUpdate       = 2001
	dbInsert       = 2002
	dbQuery        = 2004
	dbGetMore      = 2005
	dbDelete       = 2006
	dbKillCursors  = 2007
	dbCommand      = 2010
	dbCommandReply = 2011
	dbCompressed   = 2012
	dbMessage      = 2013
	// opMsg flags
	opMsgFlagChecksumPresent = 1
	opMsgFlagMoreToCome      = (1 << 1)
	opMsgFlagExhaustAllowed  = (1 << 16)
)

type queryOp struct {
	query       interface{}
	collection  string
	serverTags  []bson.D
	selector    interface{}
	replyFunc   replyFunc
	mode        Mode
	skip        int32
	limit       int32
	options     queryWrapper
	hasOptions  bool
	flags       queryOpFlags
	readConcern string
}

type queryWrapper struct {
	Query          interface{} `bson:"$query"`
	OrderBy        interface{} `bson:"$orderby,omitempty"`
	Hint           interface{} `bson:"$hint,omitempty"`
	Explain        bool        `bson:"$explain,omitempty"`
	Snapshot       bool        `bson:"$snapshot,omitempty"`
	ReadPreference bson.D      `bson:"$readPreference,omitempty"`
	MaxScan        int         `bson:"$maxScan,omitempty"`
	MaxTimeMS      int         `bson:"$maxTimeMS,omitempty"`
	Comment        string      `bson:"$comment,omitempty"`
	Collation      *Collation  `bson:"$collation,omitempty"`
}

func (op *queryOp) finalQuery(socket *mongoSocket) interface{} {
	if op.flags&flagSlaveOk != 0 && socket.ServerInfo().Mongos {
		var modeName string
		switch op.mode {
		case Strong:
			modeName = "primary"
		case Monotonic, Eventual:
			modeName = "secondaryPreferred"
		case PrimaryPreferred:
			modeName = "primaryPreferred"
		case Secondary:
			modeName = "secondary"
		case SecondaryPreferred:
			modeName = "secondaryPreferred"
		case Nearest:
			modeName = "nearest"
		default:
			panic(fmt.Sprintf("unsupported read mode: %d", op.mode))
		}
		op.hasOptions = true
		op.options.ReadPreference = make(bson.D, 0, 2)
		op.options.ReadPreference = append(op.options.ReadPreference, bson.DocElem{Name: "mode", Value: modeName})
		if len(op.serverTags) > 0 {
			op.options.ReadPreference = append(op.options.ReadPreference, bson.DocElem{Name: "tags", Value: op.serverTags})
		}
	}
	if op.hasOptions {
		if op.query == nil {
			var empty bson.D
			op.options.Query = empty
		} else {
			op.options.Query = op.query
		}
		debugf("final query is %#v\n", &op.options)
		return &op.options
	}
	return op.query
}

type getMoreOp struct {
	collection string
	limit      int32
	cursorId   int64
	replyFunc  replyFunc
}

type replyOp struct {
	flags     uint32
	cursorId  int64
	firstDoc  int32
	replyDocs int32
}

type insertOp struct {
	collection string        // "database.collection"
	documents  []interface{} // One or more documents to insert
	flags      uint32
}

type updateOp struct {
	Collection string      `bson:"-"` // "database.collection"
	Selector   interface{} `bson:"q"`
	Update     interface{} `bson:"u"`
	Flags      uint32      `bson:"-"`
	Multi      bool        `bson:"multi,omitempty"`
	Upsert     bool        `bson:"upsert,omitempty"`
}

type deleteOp struct {
	Collection string      `bson:"-"` // "database.collection"
	Selector   interface{} `bson:"q"`
	Flags      uint32      `bson:"-"`
	Limit      int         `bson:"limit"`
}

type killCursorsOp struct {
	cursorIds []int64
}

type msgSection struct {
	payloadType uint8
	data        interface{}
}

// op_msg is introduced in mongodb 3.6, see
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#op-msg
// for details
type msgOp struct {
	flags    uint32
	sections []msgSection
	checksum uint32
}

// PayloadType1 is a container for the OP_MSG payload data of type 1.
// There is no definition of the type 0 payload because that is simply a
// bson document.
type payloadType1 struct {
	size       int32
	identifier string
	docs       []interface{}
}

type requestInfo struct {
	bufferPos int
	replyFunc replyFunc
}

func newSocket(server *mongoServer, conn net.Conn, timeout time.Duration) *mongoSocket {
	socket := &mongoSocket{
		conn:            conn,
		addr:            server.Addr,
		server:          server,
		replyFuncs:      make(map[uint32]replyFunc),
		opMsgReplyFuncs: make(map[uint32]opMsgReplyFunc),
	}
	socket.gotNonce.L = &socket.Mutex
	if err := socket.InitialAcquire(server.Info(), timeout); err != nil {
		panic("newSocket: InitialAcquire returned error: " + err.Error())
	}
	stats.socketsAlive(+1)
	debugf("Socket %p to %s: initialized", socket, socket.addr)
	socket.resetNonce()
	go socket.readLoop()
	return socket
}

// Server returns the server that the socket is associated with.
// It returns nil while the socket is cached in its respective server.
func (socket *mongoSocket) Server() *mongoServer {
	socket.Lock()
	server := socket.server
	socket.Unlock()
	return server
}

// ServerInfo returns details for the server at the time the socket
// was initially acquired.
func (socket *mongoSocket) ServerInfo() *mongoServerInfo {
	if socket == nil {
		return &mongoServerInfo{}
	}
	socket.Lock()
	serverInfo := socket.serverInfo
	socket.Unlock()
	return serverInfo
}

// InitialAcquire obtains the first reference to the socket, either
// right after the connection is made or once a recycled socket is
// being put back in use.
func (socket *mongoSocket) InitialAcquire(serverInfo *mongoServerInfo, timeout time.Duration) error {
	socket.Lock()
	if socket.references > 0 {
		panic("Socket acquired out of cache with references")
	}
	if socket.dead != nil {
		dead := socket.dead
		socket.Unlock()
		return dead
	}
	socket.references++
	socket.serverInfo = serverInfo
	socket.timeout = timeout
	stats.socketsInUse(+1)
	stats.socketRefs(+1)
	socket.Unlock()
	return nil
}

// Acquire obtains an additional reference to the socket.
// The socket will only be recycled when it's released as many
// times as it's been acquired.
func (socket *mongoSocket) Acquire() (info *mongoServerInfo) {
	socket.Lock()
	if socket.references == 0 {
		panic("Socket got non-initial acquire with references == 0")
	}
	// We'll track references to dead sockets as well.
	// Caller is still supposed to release the socket.
	socket.references++
	stats.socketRefs(+1)
	serverInfo := socket.serverInfo
	socket.Unlock()
	return serverInfo
}

// Release decrements a socket reference. The socket will be
// recycled once its released as many times as it's been acquired.
func (socket *mongoSocket) Release() {
	socket.Lock()
	if socket.references == 0 {
		panic("socket.Release() with references == 0")
	}
	socket.references--
	stats.socketRefs(-1)
	if socket.references == 0 {
		stats.socketsInUse(-1)
		server := socket.server
		closeAfterIdle := socket.closeAfterIdle
		socket.Unlock()
		socket.LogoutAll()
		if closeAfterIdle {
			socket.Close()
		} else if server != nil {
			// If the socket is dead server is nil.
			server.RecycleSocket(socket)
		}
	} else {
		socket.Unlock()
	}
}

// SetTimeout changes the timeout used on socket operations.
func (socket *mongoSocket) SetTimeout(d time.Duration) {
	socket.Lock()
	socket.timeout = d
	socket.Unlock()
}

type deadlineType int

const (
	readDeadline  deadlineType = 1
	writeDeadline deadlineType = 2
)

func (socket *mongoSocket) updateDeadline(which deadlineType) {
	var when time.Time
	if socket.timeout > 0 {
		when = time.Now().Add(socket.timeout)
	}
	whichstr := ""
	switch which {
	case readDeadline | writeDeadline:
		whichstr = "read/write"
		socket.conn.SetDeadline(when)
	case readDeadline:
		whichstr = "read"
		socket.conn.SetReadDeadline(when)
	case writeDeadline:
		whichstr = "write"
		socket.conn.SetWriteDeadline(when)
	default:
		panic("invalid parameter to updateDeadline")
	}
	debugf("Socket %p to %s: updated %s deadline to %s ahead (%s)", socket, socket.addr, whichstr, socket.timeout, when)
}

// Close terminates the socket use.
func (socket *mongoSocket) Close() {
	socket.kill(errors.New("Closed explicitly"), false)
}

// CloseAfterIdle terminates an idle socket, which has a zero
// reference, or marks the socket to be terminate after idle.
func (socket *mongoSocket) CloseAfterIdle() {
	socket.Lock()
	if socket.references == 0 {
		socket.Unlock()
		socket.Close()
		logf("Socket %p to %s: idle and close.", socket, socket.addr)
		return
	}
	socket.closeAfterIdle = true
	socket.Unlock()
	logf("Socket %p to %s: close after idle.", socket, socket.addr)
}

func (socket *mongoSocket) kill(err error, abend bool) {
	socket.Lock()
	if socket.dead != nil {
		debugf("Socket %p to %s: killed again: %s (previously: %s)", socket, socket.addr, err.Error(), socket.dead.Error())
		socket.Unlock()
		return
	}
	logf("Socket %p to %s: closing: %s (abend=%v)", socket, socket.addr, err.Error(), abend)
	socket.dead = err
	socket.conn.Close()
	stats.socketsAlive(-1)
	replyFuncs := socket.replyFuncs
	socket.replyFuncs = make(map[uint32]replyFunc)
	opMsgReplyFuncs := socket.opMsgReplyFuncs
	socket.opMsgReplyFuncs = make(map[uint32]opMsgReplyFunc)
	server := socket.server
	socket.server = nil
	socket.gotNonce.Broadcast()
	socket.Unlock()
	for _, replyFunc := range replyFuncs {
		logf("Socket %p to %s: notifying replyFunc of closed socket: %s", socket, socket.addr, err.Error())
		replyFunc(err, nil, -1, nil)
	}
	for _, opMsgReplyFunc := range opMsgReplyFuncs {
		logf("Socket %p to %s: notifying replyFunc of closed socket: %s", socket, socket.addr, err.Error())
		opMsgReplyFunc(nil, err)
	}
	if abend {
		server.AbendSocket(socket)
	}
}

func (socket *mongoSocket) SimpleQuery(op *queryOp) (data []byte, err error) {
	var wait, change sync.Mutex
	var replyDone bool
	var replyData []byte
	var replyErr error
	wait.Lock()
	op.replyFunc = func(err error, reply *replyOp, docNum int, docData []byte) {
		change.Lock()
		if !replyDone {
			replyDone = true
			replyErr = err
			if err == nil {
				replyData = docData
			}
		}
		change.Unlock()
		wait.Unlock()
	}
	err = socket.Query(op)
	if err != nil {
		return nil, err
	}
	wait.Lock()
	change.Lock()
	data = replyData
	err = replyErr
	change.Unlock()
	return data, err
}

var bytesBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 256)
	},
}

func (socket *mongoSocket) Query(ops ...interface{}) (err error) {

	buf := getSizedBuffer(0)
	defer bytesBufferPool.Put(buf)

	// Serialize operations synchronously to avoid interrupting
	// other goroutines while we can't really be sending data.
	// Also, record id positions so that we can compute request
	// ids at once later with the lock already held.
	requests := make([]requestInfo, len(ops))
	requestCount := 0

	for _, op := range ops {
		debugf("Socket %p to %s: serializing op: %#v", socket, socket.addr, op)
		if qop, ok := op.(*queryOp); ok {
			if cmd, ok := qop.query.(*findCmd); ok {
				debugf("Socket %p to %s: find command: %#v", socket, socket.addr, cmd)
			}
		}
		start := len(buf)
		var replyFunc replyFunc
		switch op := op.(type) {

		case *updateOp:
			buf = addHeader(buf, dbUpdate)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.Collection)
			buf = addInt32(buf, int32(op.Flags))
			debugf("Socket %p to %s: serializing selector document: %#v", socket, socket.addr, op.Selector)
			buf, err = addBSON(buf, op.Selector)
			if err != nil {
				return err
			}
			debugf("Socket %p to %s: serializing update document: %#v", socket, socket.addr, op.Update)
			buf, err = addBSON(buf, op.Update)
			if err != nil {
				return err
			}

		case *insertOp:
			buf = addHeader(buf, dbInsert)
			buf = addInt32(buf, int32(op.flags))
			buf = addCString(buf, op.collection)
			for _, doc := range op.documents {
				debugf("Socket %p to %s: serializing document for insertion: %#v", socket, socket.addr, doc)
				buf, err = addBSON(buf, doc)
				if err != nil {
					return err
				}
			}

		case *queryOp:
			buf = addHeader(buf, dbQuery)
			buf = addInt32(buf, int32(op.flags))
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, op.skip)
			buf = addInt32(buf, op.limit)
			buf, err = addBSON(buf, op.finalQuery(socket))
			if err != nil {
				return err
			}
			if op.selector != nil {
				buf, err = addBSON(buf, op.selector)
				if err != nil {
					return err
				}
			}
			replyFunc = op.replyFunc

		case *getMoreOp:
			buf = addHeader(buf, dbGetMore)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, op.limit)
			buf = addInt64(buf, op.cursorId)
			replyFunc = op.replyFunc

		case *deleteOp:
			buf = addHeader(buf, dbDelete)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.Collection)
			buf = addInt32(buf, int32(op.Flags))
			debugf("Socket %p to %s: serializing selector document: %#v", socket, socket.addr, op.Selector)
			buf, err = addBSON(buf, op.Selector)
			if err != nil {
				return err
			}

		case *killCursorsOp:
			buf = addHeader(buf, dbKillCursors)
			buf = addInt32(buf, 0) // Reserved
			buf = addInt32(buf, int32(len(op.cursorIds)))
			for _, cursorId := range op.cursorIds {
				buf = addInt64(buf, cursorId)
			}

		default:
			panic("internal error: unknown operation type")
		}

		setInt32(buf, start, int32(len(buf)-start))

		if replyFunc != nil {
			request := &requests[requestCount]
			request.replyFunc = replyFunc
			request.bufferPos = start
			requestCount++
		}
	}

	// Buffer is ready for the pipe.  Lock, allocate ids, and enqueue.

	socket.Lock()
	if socket.dead != nil {
		dead := socket.dead
		socket.Unlock()
		debugf("Socket %p to %s: failing query, already closed: %s", socket, socket.addr, socket.dead.Error())
		// XXX This seems necessary in case the session is closed concurrently
		// with a query being performed, but it's not yet tested:
		for i := 0; i != requestCount; i++ {
			request := &requests[i]
			if request.replyFunc != nil {
				request.replyFunc(dead, nil, -1, nil)
			}
		}
		return dead
	}

	wasWaiting := len(socket.replyFuncs) > 0

	// Reserve id 0 for requests which should have no responses.
	requestId := socket.nextRequestId + 1
	if requestId == 0 {
		requestId++
	}
	socket.nextRequestId = requestId + uint32(requestCount)
	for i := 0; i != requestCount; i++ {
		request := &requests[i]
		setInt32(buf, request.bufferPos+4, int32(requestId))
		socket.replyFuncs[requestId] = request.replyFunc
		requestId++
	}
	socket.Unlock()
	debugf("Socket %p to %s: sending %d op(s) (%d bytes)", socket, socket.addr, len(ops), len(buf))

	stats.sentOps(len(ops))
	socket.updateDeadline(writeDeadline)
	_, err = socket.conn.Write(buf)
	if !wasWaiting && requestCount > 0 {
		socket.updateDeadline(readDeadline)
	}
	return err
}

// sendMessage send data to the database using the OP_MSG wire protocol
// introduced in MongoDB 3.6 (require maxWireVersion >= 6)
func (socket *mongoSocket) sendMessage(op *msgOp) (writeCmdResult, error) {
	var wr writeCmdResult
	var err error

	buf := getSizedBuffer(0)
	defer bytesBufferPool.Put(buf)

	buf = addHeader(buf, dbMessage)
	buf = addInt32(buf, int32(op.flags))

	for _, section := range op.sections {
		buf, err = addSection(buf, section)
		if err != nil {
			return wr, err
		}
	}

	if len(buf) > socket.ServerInfo().MaxMessageSizeBytes {
		return wr, fmt.Errorf("message length to long, should be < %v, but was %v", socket.ServerInfo().MaxMessageSizeBytes, len(buf))
	}
	// set the total message size
	setInt32(buf, 0, int32(len(buf)))

	var wait sync.Mutex
	var reply msgOp
	var responseError error
	var wcr writeCmdResult
	// if no response expected, ie op.flags&opMsgFlagMoreToCome == 1,
	// request should have id 0
	var requestID uint32
	// if moreToCome flag is set, we don't want to know the outcome of the message.
	// There is no response to a request where moreToCome has been set.
	expectReply := (op.flags & opMsgFlagMoreToCome) == 0

	socket.Lock()
	if socket.dead != nil {
		dead := socket.dead
		socket.Unlock()
		debugf("Socket %p to %s: failing query, already closed: %s", socket, socket.addr, socket.dead.Error())
		return wr, dead
	}
	if expectReply {
		// Reserve id 0 for requests which should have no responses.
	again:
		requestID = socket.nextRequestId + 1
		socket.nextRequestId++
		if requestID == 0 {
			goto again
		}
		wait.Lock()
		socket.opMsgReplyFuncs[requestID] = func(msg *msgOp, err error) {
			reply = *msg
			responseError = err
			wait.Unlock()
		}
	}
	socket.Unlock()

	setInt32(buf, 4, int32(requestID))
	stats.sentOps(1)

	socket.updateDeadline(writeDeadline)
	_, err = socket.conn.Write(buf)

	if expectReply {
		socket.updateDeadline(readDeadline)
		wait.Lock()

		if responseError != nil {
			return wcr, responseError
		}
		// for the moment, OP_MSG responses return a body section only,
		// cf https://github.com/mongodb/specifications/blob/master/source/message/OP_MSG.rst :
		//
		// "Similarly, certain commands will reply to messages using this technique when possible
		// to avoid the overhead of BSON Arrays. Drivers will be required to allow all command
		// replies to use this technique. Drivers will be required to handle Payload Type 1."
		//
		// so we only return the first section of the response (ie the body)
		wcr = reply.sections[0].data.(writeCmdResult)
	}

	return wcr, err
}

// get a slice of byte of `size` length from the pool
func getSizedBuffer(size int) []byte {
	b := bytesBufferPool.Get().([]byte)
	if len(b) < size {
		for i := len(b); i < size; i++ {
			b = append(b, byte(0))
		}
		return b
	}
	return b[0:size]
}

// Estimated minimum cost per socket: 1 goroutine + memory for the largest
// document ever seen.
func (socket *mongoSocket) readLoop() {
	header := make([]byte, 16) // 16 bytes for header
	p := make([]byte, 20)      // 20 bytes for fixed fields of OP_REPLY
	s := make([]byte, 4)
	var r io.Reader = socket.conn // No locking, conn never changes.
	for {
		_, err := io.ReadFull(r, header)
		if err != nil {
			socket.kill(err, true)
			return
		}

		totalLen := getInt32(header, 0)
		responseTo := getInt32(header, 8)
		opCode := getInt32(header, 12)

		// Don't use socket.server.Addr here.  socket is not
		// locked and socket.server may go away.
		debugf("Socket %p to %s: got reply (%d bytes)", socket, socket.addr, totalLen)
		stats.receivedOps(1)

		switch opCode {
		case opReply:
			_, err := io.ReadFull(r, p)
			if err != nil {
				socket.kill(err, true)
				return
			}
			reply := replyOp{
				flags:     uint32(getInt32(p, 0)),
				cursorId:  getInt64(p, 4),
				firstDoc:  getInt32(p, 12),
				replyDocs: getInt32(p, 16),
			}
			stats.receivedDocs(int(reply.replyDocs))

			socket.Lock()
			replyFunc, ok := socket.replyFuncs[uint32(responseTo)]
			if ok {
				delete(socket.replyFuncs, uint32(responseTo))
			}
			socket.Unlock()

			if replyFunc != nil && reply.replyDocs == 0 {
				replyFunc(nil, &reply, -1, nil)
			} else {
				for i := 0; i != int(reply.replyDocs); i++ {
					_, err := io.ReadFull(r, s)
					if err != nil {
						if replyFunc != nil {
							replyFunc(err, nil, -1, nil)
						}
						socket.kill(err, true)
						return
					}
					b := getSizedBuffer(int(getInt32(s, 0)))
					defer bytesBufferPool.Put(b)

					copy(b[0:4], s)

					_, err = io.ReadFull(r, b[4:])
					if err != nil {
						if replyFunc != nil {
							replyFunc(err, nil, -1, nil)
						}
						socket.kill(err, true)
						return
					}

					if globalDebug && globalLogger != nil {
						m := bson.M{}
						if err := bson.Unmarshal(b, m); err == nil {
							debugf("Socket %p to %s: received document: %#v", socket, socket.addr, m)
						}
					}

					if replyFunc != nil {
						replyFunc(nil, &reply, i, b)
					}
					// XXX Do bound checking against totalLen.
				}
			}

			socket.Lock()
			if len(socket.replyFuncs) == 0 {
				// Nothing else to read for now. Disable deadline.
				socket.conn.SetReadDeadline(time.Time{})
			} else {
				socket.updateDeadline(readDeadline)
			}
			socket.Unlock()

		case dbMessage:
			body := getSizedBuffer(int(totalLen) - 16)
			defer bytesBufferPool.Put(body)
			_, err := io.ReadFull(r, body)
			if err != nil {
				socket.kill(err, true)
				return
			}

			sections, err := getSections(body[4:])
			if err != nil {
				socket.kill(err, true)
				return
			}
			// TODO check CRC-32 checksum if checksum byte is set
			reply := &msgOp{
				flags:    uint32(getInt32(body, 0)),
				sections: sections,
			}

			// TODO update this when msgPayload1 section is implemented in MongoDB
			stats.receivedDocs(1)
			socket.Lock()
			opMsgReplyFunc, ok := socket.opMsgReplyFuncs[uint32(responseTo)]
			if ok {
				delete(socket.opMsgReplyFuncs, uint32(responseTo))
			}
			socket.Unlock()

			if opMsgReplyFunc != nil {
				opMsgReplyFunc(reply, err)
			} else {
				socket.kill(fmt.Errorf("couldn't handle response properly"), true)
				return
			}
			socket.conn.SetReadDeadline(time.Time{})
		default:
			socket.kill(errors.New("opcode != 1 && opcode != 2013, corrupted data?"), true)
			return
		}
	}
}

var emptyHeader = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func addHeader(b []byte, opcode int32) []byte {
	i := len(b)
	b = append(b, emptyHeader...)
	// Enough for current opcodes.
	b[i+12] = byte(opcode)
	b[i+13] = byte(opcode >> 8)
	return b
}

func addInt32(b []byte, i int32) []byte {
	return append(b, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
}

func addInt64(b []byte, i int64) []byte {
	return append(b, byte(i), byte(i>>8), byte(i>>16), byte(i>>24),
		byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
}

func addCString(b []byte, s string) []byte {
	b = append(b, []byte(s)...)
	b = append(b, 0)
	return b
}

// Marshal a section and add it to the provided buffer
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#sections
func addSection(b []byte, s msgSection) ([]byte, error) {
	var err error
	b = append(b, s.payloadType)
	switch s.payloadType {
	case msgPayload0:
		b, err = addBSON(b, s.data)
		if err != nil {
			return b, err
		}
	case msgPayload1:
		pos := len(b)
		b = addInt32(b, 0)
		s1 := s.data.(payloadType1)
		b = addCString(b, s1.identifier)
		for _, doc := range s1.docs {
			b, err = bson.MarshalBuffer(doc, b)
			if err != nil {
				return b, err
			}
		}
		setInt32(b, pos, int32(len(b)-pos))
	default:
		return b, fmt.Errorf("invalid section kind in op_msg: %v", s.payloadType)
	}
	return b, nil
}

func addBSON(b []byte, doc interface{}) ([]byte, error) {
	if doc == nil {
		return append(b, 5, 0, 0, 0, 0), nil
	}
	data, err := bson.MarshalBuffer(doc, b)
	if err != nil {
		return b, err
	}
	return data, nil
}

func setInt32(b []byte, pos int, i int32) {
	b[pos] = byte(i)
	b[pos+1] = byte(i >> 8)
	b[pos+2] = byte(i >> 16)
	b[pos+3] = byte(i >> 24)
}

func getInt32(b []byte, pos int) int32 {
	return (int32(b[pos+0])) |
		(int32(b[pos+1]) << 8) |
		(int32(b[pos+2]) << 16) |
		(int32(b[pos+3]) << 24)
}

func getInt64(b []byte, pos int) int64 {
	return (int64(b[pos+0])) |
		(int64(b[pos+1]) << 8) |
		(int64(b[pos+2]) << 16) |
		(int64(b[pos+3]) << 24) |
		(int64(b[pos+4]) << 32) |
		(int64(b[pos+5]) << 40) |
		(int64(b[pos+6]) << 48) |
		(int64(b[pos+7]) << 56)
}

// UnMarshal an array of bytes into a section
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#sections
func getSections(b []byte) ([]msgSection, error) {
	var sections []msgSection
	pos := 0
	for pos != len(b) {
		sectionLength := int(getInt32(b, pos+1))
		// first byte is section type
		switch b[pos] {
		case msgPayload0:
			var result writeCmdResult
			err := bson.Unmarshal(b[pos+1:pos+sectionLength+1], &result)
			if err != nil {
				return nil, err
			}
			sections = append(sections, msgSection{
				payloadType: b[pos],
				data:        result,
			})
		case msgPayload1:
			// not implemented yet
			//
			// b[0:4] size
			// b[4:?] docSeqID
			// b[?:len(b)] documentSequence
		default:
			return nil, fmt.Errorf("invalid section type: %v", b[0])
		}
		pos += sectionLength + 1
	}
	return sections, nil
}
