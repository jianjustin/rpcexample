package service

import (
	"log"
	"strings"
	"sync"
)

type Server struct {
	Mu       sync.Mutex
	Services map[string]*Service
	Count    int
}

func (rs *Server) AddService(svc *Service) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.Services[svc.Name] = svc
}

func (rs *Server) dispatch(req ReqMsg) ReplyMsg {
	rs.Mu.Lock()

	rs.Count += 1

	// split Raft.AppendEntries into service and method
	dot := strings.LastIndex(req.SvcMeth, ".")
	serviceName := req.SvcMeth[:dot]
	methodName := req.SvcMeth[dot+1:]

	service, ok := rs.Services[serviceName]

	rs.Mu.Unlock()

	if ok {
		return service.dispatch(methodName, req)
	} else {
		choices := []string{}
		for k, _ := range rs.Services {
			choices = append(choices, k)
		}
		log.Fatalf("labrpc.Server.dispatch(): unknown service %v in %v.%v; expecting one of %v\n",
			serviceName, serviceName, methodName, choices)
		return ReplyMsg{false, nil}
	}
}

func (rs *Server) GetCount() int {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.Count
}
