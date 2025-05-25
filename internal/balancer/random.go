package balancer

import (
	"math/rand"

	"github.com/emrzvv/lb-research/internal/model"
)

type RandomBalancer struct {
	servers []*model.Server
}

func (b *RandomBalancer) PickServer(sessionID int64) *model.Server {
	return b.servers[rand.Intn(len(b.servers))]
}

func (b *RandomBalancer) GetServers() []*model.Server {
	return b.servers
}
