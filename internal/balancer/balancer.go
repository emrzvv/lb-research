package balancer

import (
	"sync"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
)

type Balancer interface {
	PickServer(sessionID int64) *model.Server
	GetServers() []*model.Server
}

func NewBalancer(cfg *config.Config, servers []*model.Server, rng *common.RNG) Balancer {
	switch cfg.Balancer.Strategy {
	case "rr":
		return &RRBalancer{
			servers: servers,
			mu:      sync.Mutex{},
			idx:     0,
		}
	case "random":
		return &RandomBalancer{
			servers: servers,
			rng:     rng,
		}
	case "ch":
		return NewCHBalancer(servers, cfg.Balancer.CHReplicas, nil)
	case "ch+random":
		return NewCHBalancer(servers, cfg.Balancer.CHReplicas, &RandomBalancer{servers: servers, rng: rng})
	default:
		panic("no such strategy has been implemented")
	}
}
