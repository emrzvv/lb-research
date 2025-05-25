package balancer

import (
	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/model"
)

type RandomBalancer struct {
	servers []*model.Server
	rng     *common.RNG
}

func (b *RandomBalancer) PickServer(sessionID int64) *model.Server {
	return b.servers[b.rng.Intn(len(b.servers))]
}

func (b *RandomBalancer) GetServers() []*model.Server {
	return b.servers
}
