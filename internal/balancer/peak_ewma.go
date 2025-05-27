package balancer

import (
	"math"
	"sync"

	"github.com/emrzvv/lb-research/internal/metric"
	"github.com/emrzvv/lb-research/internal/model"
)

type PeakEWMABalancer struct {
	servers     []*model.Server
	alpha       float64
	mu          sync.Mutex
	ewma        []float64 // S_i
	lastUpdated []float64
}

func NewPeakEWMABalancer(servers []*model.Server, alpha float64) *PeakEWMABalancer {
	metric.RTTChan = make(chan metric.RequestRTT, 1<<14)

	p := &PeakEWMABalancer{
		servers:     servers,
		alpha:       alpha,
		mu:          sync.Mutex{},
		ewma:        make([]float64, len(servers)),
		lastUpdated: make([]float64, len(servers)),
	}
	go p.collect()
	return p
}

func (b *PeakEWMABalancer) collect() {
	for ev := range metric.RTTChan {
		idx := ev.ServerID - 1

		b.mu.Lock()
		prev := b.ewma[idx]
		peak := max(prev, ev.RTT)

		b.ewma[idx] = b.alpha*peak + (1-b.alpha)*prev
		b.lastUpdated[idx] = ev.When
		b.mu.Unlock()
	}
}

func (b *PeakEWMABalancer) PickServer(sessionID int64) *model.Server {
	b.mu.Lock()
	defer b.mu.Unlock()

	best, bestScore := -1, math.MaxFloat64
	for i, s := range b.servers {
		s.Lock()
		score := b.ewma[i] * float64(s.CurrentConnections+1)
		if score < bestScore {
			best, bestScore = i, score
		}
		s.Unlock()
	}
	if best >= 0 {
		return b.servers[best]
	}
	return nil
}

func (b *PeakEWMABalancer) GetServers() []*model.Server {
	return b.servers
}
