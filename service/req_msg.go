package service

import "reflect"

type ReqMsg struct {
	EndName  interface{} // name of sending ClientEnd
	SvcMeth  string      // e.g. "Raft.AppendEntries"
	ArgsType reflect.Type
	Args     []byte
	ReplyCh  chan ReplyMsg
}
