package service

import (
	"reflect"
	"sync/atomic"
)

func MakeNetwork() *Network {
	rn := &Network{}
	rn.Reliabled = true
	rn.Ends = map[interface{}]*Client{}
	rn.Enabled = map[interface{}]bool{}
	rn.Servers = map[interface{}]*Server{}
	rn.Connections = map[interface{}](interface{}){}
	rn.EndCh = make(chan ReqMsg)
	rn.Done = make(chan struct{})

	// single goroutine to handle all ClientEnd.Call()s
	go func() {
		for {
			select {
			case xreq := <-rn.EndCh:
				atomic.AddInt32(&rn.Count, 1)
				atomic.AddInt64(&rn.Bytes, int64(len(xreq.Args)))
				go rn.processReq(xreq)
			case <-rn.Done:
				return
			}
		}
	}()

	return rn
}

func MakeServer() *Server {
	rs := &Server{}
	rs.Services = map[string]*Service{}
	return rs
}

func MakeService(rcvr interface{}) *Service {
	svc := &Service{}
	svc.Typ = reflect.TypeOf(rcvr)
	svc.Rcvr = reflect.ValueOf(rcvr)
	svc.Name = reflect.Indirect(svc.Rcvr).Type().Name()
	svc.Methods = map[string]reflect.Method{}

	for m := 0; m < svc.Typ.NumMethod(); m++ {
		method := svc.Typ.Method(m)
		mtype := method.Type
		mname := method.Name

		//fmt.Printf("%v pp %v ni %v 1k %v 2k %v no %v\n",
		//	mname, method.PkgPath, mtype.NumIn(), mtype.In(1).Kind(), mtype.In(2).Kind(), mtype.NumOut())

		if method.PkgPath != "" || // capitalized?
			mtype.NumIn() != 3 ||
			//mtype.In(1).Kind() != reflect.Ptr ||
			mtype.In(2).Kind() != reflect.Ptr ||
			mtype.NumOut() != 0 {
			// the method is not suitable for a handler
			//fmt.Printf("bad method: %v\n", mname)
		} else {
			// the method looks like a handler
			svc.Methods[mname] = method
		}
	}

	return svc
}
