package service

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
)

type Service struct {
	Name    string
	Rcvr    reflect.Value
	Typ     reflect.Type
	Methods map[string]reflect.Method
}

func (svc *Service) dispatch(methname string, req ReqMsg) ReplyMsg {
	if method, ok := svc.Methods[methname]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(req.ArgsType)

		// decode the argument.
		ab := bytes.NewBuffer(req.Args)
		gob.NewDecoder(ab).Decode(args.Interface())

		// allocate space for the reply.
		replyType := method.Type.In(2)
		replyType = replyType.Elem()
		replyv := reflect.New(replyType)

		// call the method.
		function := method.Func
		function.Call([]reflect.Value{svc.Rcvr, args.Elem(), replyv})

		// encode the reply.
		rb := new(bytes.Buffer)
		gob.NewEncoder(rb).EncodeValue(replyv)

		return ReplyMsg{true, rb.Bytes()}
	} else {
		choices := []string{}
		for k, _ := range svc.Methods {
			choices = append(choices, k)
		}
		log.Fatalf("labrpc.Service.dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, req.SvcMeth, choices)
		return ReplyMsg{false, nil}
	}
}
