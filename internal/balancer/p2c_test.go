package balancer

import (
	"math"
	"testing"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
)

func TestP2CDist(t *testing.T) {
	n := 10
	rng := common.NewRNG(42)
	cfg, err := config.Load("../../config/default.yaml")
	if err != nil {
		t.Fatalf("no config")
	}
	servers := model.InitServers(cfg, rng)
	p2c := NewP2CBalancer(servers, common.NewRNG(42))

	const iter = 1_000_000
	count := make([]int, n)
	for i := 0; i < iter; i++ {
		s := p2c.PickServer(int64(i))
		count[s.ID-1]++
	}
	mean := float64(iter) / float64(n)
	var maxDev float64
	for _, c := range count {
		dev := math.Abs(float64(c)-mean) / mean
		if dev > maxDev {
			maxDev = dev
		}
	}
	if maxDev > 0.03 {
		t.Fatalf("imbalance %.1f%%", maxDev*100)
	}
}
