package service

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type Network struct {
	Mu          sync.Mutex
	Reliabled   bool
	LongDelay   bool                        // pause a long time on send on disabled connection
	LongReorder bool                        // sometimes delay replies a long time
	Ends        map[interface{}]*Client     // ends, by name
	Enabled     map[interface{}]bool        // by end name
	Servers     map[interface{}]*Server     // servers, by name
	Connections map[interface{}]interface{} // endname -> servername
	EndCh       chan ReqMsg
	Done        chan struct{} // closed when Network is cleaned up
	Count       int32         // total RPC count, for statistics
	Bytes       int64         // total bytes send, for statistics
}

func (rn *Network) Cleanup() {
	close(rn.Done)
}

func (rn *Network) Reliable(yes bool) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.Reliabled = yes
}

func (rn *Network) LongReordering(yes bool) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.LongReorder = yes
}

func (rn *Network) LongDelays(yes bool) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.LongDelay = yes
}

func (rn *Network) readEndnameInfo(endname interface{}) (enabled bool,
	servername interface{}, server *Server, reliable bool, longreordering bool,
) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	enabled = rn.Enabled[endname]
	servername = rn.Connections[endname]
	if servername != nil {
		server = rn.Servers[servername]
	}
	reliable = rn.Reliabled
	longreordering = rn.LongReorder
	return
}

func (rn *Network) isServerDead(endname interface{}, servername interface{}, server *Server) bool {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	if rn.Enabled[endname] == false || rn.Servers[servername] != server {
		return true
	}
	return false
}

func (rn *Network) processReq(req ReqMsg) {
	enabled, servername, server, reliable, longreordering := rn.readEndnameInfo(req.EndName)

	if enabled && servername != nil && server != nil {
		if reliable == false {
			// short delay
			ms := (rand.Int() % 27)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}

		if reliable == false && (rand.Int()%1000) < 100 {
			// drop the request, return as if timeout
			req.ReplyCh <- ReplyMsg{false, nil}
			return
		}

		// execute the request (call the RPC handler).
		// in a separate thread so that we can periodically check
		// if the server has been killed and the RPC should get a
		// failure reply.
		ech := make(chan ReplyMsg)
		go func() {
			r := server.dispatch(req)
			ech <- r
		}()

		// wait for handler to return,
		// but stop waiting if DeleteServer() has been called,
		// and return an error.
		var reply ReplyMsg
		replyOK := false
		serverDead := false
		for replyOK == false && serverDead == false {
			select {
			case reply = <-ech:
				replyOK = true
			case <-time.After(100 * time.Millisecond):
				serverDead = rn.isServerDead(req.EndName, servername, server)
				if serverDead {
					go func() {
						<-ech // drain channel to let the goroutine created earlier terminate
					}()
				}
			}
		}

		// do not reply if DeleteServer() has been called, i.e.
		// the server has been killed. this is needed to avoid
		// situation in which a client gets a positive reply
		// to an Append, but the server persisted the update
		// into the old Persister. config.go is careful to call
		// DeleteServer() before superseding the Persister.
		serverDead = rn.isServerDead(req.EndName, servername, server)

		if replyOK == false || serverDead == true {
			// server was killed while we were waiting; return error.
			req.ReplyCh <- ReplyMsg{false, nil}
		} else if reliable == false && (rand.Int()%1000) < 100 {
			// drop the reply, return as if timeout
			req.ReplyCh <- ReplyMsg{false, nil}
		} else if longreordering == true && rand.Intn(900) < 600 {
			// delay the response for a while
			ms := 200 + rand.Intn(1+rand.Intn(2000))
			// Russ points out that this timer arrangement will decrease
			// the number of goroutines, so that the race
			// detector is less likely to get upset.
			time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
				atomic.AddInt64(&rn.Bytes, int64(len(reply.Reply)))
				req.ReplyCh <- reply
			})
		} else {
			atomic.AddInt64(&rn.Bytes, int64(len(reply.Reply)))
			req.ReplyCh <- reply
		}
	} else {
		// simulate no reply and eventual timeout.
		ms := 0
		if rn.LongDelay {
			// let Raft tests check that leader doesn't send
			// RPCs synchronously.
			ms = (rand.Int() % 7000)
		} else {
			// many kv tests require the client to try each
			// server in fairly rapid succession.
			ms = (rand.Int() % 100)
		}
		time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
			req.ReplyCh <- ReplyMsg{false, nil}
		})
	}

}

// create a client end-point.
// start the thread that listens and delivers.
func (rn *Network) MakeEnd(endname interface{}) *Client {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	if _, ok := rn.Ends[endname]; ok {
		log.Fatalf("MakeEnd: %v already exists\n", endname)
	}

	e := &Client{}
	e.EndName = endname
	e.Ch = rn.EndCh
	e.Done = rn.Done
	rn.Ends[endname] = e
	rn.Enabled[endname] = false
	rn.Connections[endname] = nil

	return e
}

func (rn *Network) AddServer(servername interface{}, rs *Server) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.Servers[servername] = rs
}

func (rn *Network) DeleteServer(servername interface{}) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.Servers[servername] = nil
}

// connect a ClientEnd to a server.
// a ClientEnd can only be connected once in its lifetime.
func (rn *Network) Connect(endname interface{}, servername interface{}) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.Connections[endname] = servername
}

// enable/disable a ClientEnd.
func (rn *Network) Enable(endname interface{}, enabled bool) {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	rn.Enabled[endname] = enabled
}

// get a server's count of incoming RPCs.
func (rn *Network) GetCount(servername interface{}) int {
	rn.Mu.Lock()
	defer rn.Mu.Unlock()

	svr := rn.Servers[servername]
	return svr.GetCount()
}

func (rn *Network) GetTotalCount() int {
	x := atomic.LoadInt32(&rn.Count)
	return int(x)
}

func (rn *Network) GetTotalBytes() int64 {
	x := atomic.LoadInt64(&rn.Bytes)
	return x
}
