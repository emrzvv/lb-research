package balancer

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"

	"github.com/emrzvv/lb-research/internal/model"
)

type vnode struct {
	hash   uint32
	server *model.Server
}

type ring struct {
	vnodes []vnode
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func fnv32int64(x int64) uint32 {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(x))
	h := fnv.New32a()
	h.Write([]byte(b[:]))
	return h.Sum32()
}

func newRing(servers []*model.Server, replicas int) *ring {
	var v []vnode
	for _, s := range servers {
		for i := 0; i < replicas; i++ {
			key := fmt.Sprintf("%d-%d", s.ID, i)
			h := fnv32(key)
			v = append(v, vnode{hash: h, server: s})
		}
	}

	sort.Slice(v, func(i, j int) bool { return v[i].hash < v[j].hash })
	return &ring{
		vnodes: v,
	}
}

func (r *ring) get(hash uint32) *model.Server {
	idx := sort.Search(len(r.vnodes), func(i int) bool { return r.vnodes[i].hash >= hash })
	if idx == len(r.vnodes) {
		idx = 0
	}
	return r.vnodes[idx].server
}

type CHBalancer struct {
	mu       sync.Mutex
	servers  []*model.Server
	ring     *ring
	replicas int
}

func NewCHBalancer(servers []*model.Server, replicas int) *CHBalancer {
	ring := newRing(servers, replicas)
	return &CHBalancer{
		mu:       sync.Mutex{},
		servers:  servers,
		ring:     ring,
		replicas: replicas,
	}
}

func (chb *CHBalancer) PickServer(sessionID int64) *model.Server {
	sh := fnv32int64(sessionID)
	chb.mu.Lock()
	s := chb.ring.get(sh)
	chb.mu.Unlock()
	if s.IsOverLoaded() {
		return nil
	}
	return s
}

func (chb *CHBalancer) GetServers() []*model.Server {
	return chb.servers
}
