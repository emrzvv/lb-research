package balancer

import (
	"sync"

	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
)

type Balancer interface {
	PickServer(sessionID int64) *model.Server
	GetServers() []*model.Server
}

func NewBalancer(cfg *config.Config, servers []*model.Server) Balancer {
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
		}
	case "ch":
		return NewCHBalancer(servers, 100, nil)
	default:
		panic("no such strategy has been implemented")
	}
}
