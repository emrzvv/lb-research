package balancer

import (
	"strings"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
)

type Balancer interface {
	PickServer(sessionID int64) *model.Server
	GetServers() []*model.Server
}

type chain struct {
	head Balancer
	next Balancer
}

func (c *chain) PickServer(sessionID int64) *model.Server {
	s := c.head.PickServer(sessionID)
	if s != nil {
		return s
	}
	if c.next != nil {
		return c.next.PickServer(sessionID)
	}
	return s
}

func (c *chain) GetServers() []*model.Server {
	return c.head.GetServers()
}

type factory func([]*model.Server, *config.Config, *common.RNG) Balancer

func BuildChain(cfg *config.Config, servers []*model.Server, rng *common.RNG) Balancer {
	var registry = map[string]factory{
		"ch": func(s []*model.Server, c *config.Config, r *common.RNG) Balancer {
			return NewCHBalancer(servers, cfg.Balancer.CHReplicas)
		},
		"wlc": func(s []*model.Server, c *config.Config, r *common.RNG) Balancer {
			return NewWLCBalancer(servers)
		},
		"p2c": func(s []*model.Server, c *config.Config, r *common.RNG) Balancer {
			return NewP2CBalancer(servers, rng)
		},
	}

	strategies := strings.Split(cfg.Balancer.Strategy, "+")
	if len(strategies) == 0 {
		panic("empty strategy")
	}

	var tail Balancer
	for i := len(strategies) - 1; i >= 0; i-- {
		st := strings.TrimSpace(strategies[i])
		f, ok := registry[st]
		if !ok {
			panic("no such strategy implemented: " + st)
		}

		head := f(servers, cfg, rng)
		if tail != nil {
			head = &chain{head: head, next: tail}
		}
		tail = head
	}
	return tail
}
