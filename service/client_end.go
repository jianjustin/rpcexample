package service

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
)

type Client struct {
	EndName interface{}   // this end-point's name
	Ch      chan ReqMsg   // copy of Network.endCh
	Done    chan struct{} // closed when Network is cleaned up
}

func (e *Client) Call(svcMeth string, args interface{}, reply interface{}) bool {
	req := ReqMsg{}
	req.EndName = e.EndName
	req.SvcMeth = svcMeth
	req.ArgsType = reflect.TypeOf(args)
	req.ReplyCh = make(chan ReplyMsg)

	qb := new(bytes.Buffer)
	if err := gob.NewEncoder(qb).Encode(args); err != nil {
		panic(err)
	}
	req.Args = qb.Bytes()

	//
	// send the request.
	//
	select {
	case e.Ch <- req:
		// the request has been sent.
	case <-e.Done:
		// entire Network has been destroyed.
		return false
	}

	//
	// wait for the reply.
	//
	rep := <-req.ReplyCh
	if rep.Ok {
		rb := bytes.NewBuffer(rep.Reply)
		if err := gob.NewDecoder(rb).Decode(reply); err != nil {
			log.Fatalf("ClientEnd.Call(): decode reply: %v\n", err)
		}
		return true
	} else {
		return false
	}
}
