package balancer

import (
	"sync"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/model"
)

type P2CBalancer struct {
	servers []*model.Server
	rng     *common.RNG
	mu      sync.RWMutex
}

func NewP2CBalancer(servers []*model.Server, rng *common.RNG) *P2CBalancer {
	return &P2CBalancer{
		servers: servers,
		rng:     rng,
		mu:      sync.RWMutex{},
	}
}

func (b *P2CBalancer) PickServer(_ int64) *model.Server {
	b.mu.RLock()
	n := len(b.servers)
	if n == 0 {
		b.mu.RUnlock()
		return nil
	}

	i1 := b.rng.Intn(n)
	i2 := b.rng.Intn(n - 1)
	if i2 >= i1 {
		i2++
	}
	s1, s2 := b.servers[i1], b.servers[i2]
	b.mu.RUnlock()
	s1.Lock()
	s2.Lock()
	if s1.CurrentConnections <= s2.CurrentConnections {
		s1.Unlock()
		s2.Unlock()
		return s1
	}
	s1.Unlock()
	s2.Unlock()
	return s2
}

func (b *P2CBalancer) GetServers() []*model.Server {
	return b.servers
}
