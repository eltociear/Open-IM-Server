package openKeeper

import (
	"context"
	"strings"
	"sync"

	"github.com/OpenIMSDK/Open-IM-Server/pkg/common/log"
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	target resolver.Target
	cc     resolver.ClientConn
	addrs  []resolver.Address
	lock   sync.RWMutex

	getConnsRemote func(serviceName string) (conns []resolver.Address, err error)
}

func (r *Resolver) ResolveNow(o resolver.ResolveNowOptions) {
	log.ZDebug(context.Background(), "start resolve now", "target", r.target)
	newConns, err := r.getConnsRemote(strings.TrimLeft(r.target.URL.Path, "/"))
	if err != nil {
		return
	}
	log.ZDebug(context.Background(), "resolve now", "newConns", newConns, "target", r.target)
	r.lock.Lock()
	r.addrs = newConns
	r.lock.Unlock()
	r.cc.UpdateState(resolver.State{Addresses: r.addrs})
	log.ZDebug(context.Background(), "resolve now ok", "newConns", newConns, "target", r.target)
}

func (s *Resolver) Close() {}

func (s *ZkClient) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	log.ZDebug(context.Background(), "build resolver", "target", target, "opts", opts, "cc", cc)
	r := &Resolver{}
	r.target = target
	r.cc = cc
	r.getConnsRemote = s.GetConnsRemote
	r.ResolveNow(resolver.ResolveNowOptions{})
	s.lock.Lock()
	defer s.lock.Unlock()
	s.resolvers[strings.TrimLeft(target.URL.Path, "/")] = r
	log.ZDebug(context.Background(), "build resolver ok", "target", target)
	return r, nil
}

func (s *ZkClient) Scheme() string { return s.scheme }