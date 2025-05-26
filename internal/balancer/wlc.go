package balancer

import (
	"sort"
	"sync"

	"github.com/emrzvv/lb-research/internal/model"
)

type WLCBalancer struct {
	servers []*model.Server
	mu      sync.Mutex
}

type sorter struct {
	value  float64
	server *model.Server
}

func NewWLCBalancer(servers []*model.Server) *WLCBalancer {
	return &WLCBalancer{
		servers: servers,
		mu:      sync.Mutex{},
	}
}

func (b *WLCBalancer) PickServer(sessionID int64) *model.Server {
	b.mu.Lock()
	toSort := make([]*sorter, 0)
	for _, s := range b.servers {
		s.Lock()
		c := float64(s.CurrentConnections)
		w := s.Parameters.Mbps
		s.Unlock()
		toSort = append(toSort, &sorter{value: c / w, server: s})
	}
	b.mu.Unlock()
	sort.Slice(toSort, func(i, j int) bool { return toSort[i].value < toSort[j].value })
	result := toSort[0].server
	if result.IsOverLoaded() {
		result = nil
	}
	return result
}

func (b *WLCBalancer) GetServers() []*model.Server {
	return b.servers
}
