package balancer

import (
	"sync"

	"github.com/emrzvv/lb-research/internal/model"
)

type RRBalancer struct {
	servers []*model.Server
	mu      sync.Mutex
	idx     int
}

func (b *RRBalancer) PickServer() *model.Server {
	b.mu.Lock()
	b.idx = (b.idx + 1) % len(b.servers)
	b.mu.Unlock()
	return b.servers[b.idx]
}

func (b *RRBalancer) GetServers() []*model.Server {
	return b.servers
}
