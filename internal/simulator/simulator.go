package simulator

import (
	"log"
	"sync"

	"github.com/emrzvv/lb-research/internal/balancer"
	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/export"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
	"github.com/fschuetz04/simgo"
)

type rateCtrl struct {
	mu      sync.RWMutex
	base    float64
	current float64
}

func (r *rateCtrl) Get() float64 {
	r.mu.RLock()
	v := r.current
	r.mu.RUnlock()
	return v
}

func (r *rateCtrl) Set(v float64) {
	r.mu.Lock()
	r.current = v
	r.mu.Unlock()
}

func Run(cfg *config.Config, servers []*model.Server, balancer balancer.Balancer, rng *common.RNG, outDir string) stats.Statistics {
	simulation := simgo.NewSimulation()
	// statistics := stats.NewStatistics(cfg)
	if err := export.WriteServersCfgToCSV(servers, outDir+"/servers.csv"); err != nil {
		log.Fatalf("cannot write servers.csv: %v", err)
	}
	statistics := stats.NewStatisticsConcurrent(cfg, len(servers), outDir)

	rc := &rateCtrl{base: cfg.Traffic.BaseRPS, current: cfg.Traffic.BaseRPS}

	simulation.Process(func(proc simgo.Process) { collectSnapshots(proc, cfg, servers, statistics) })
	simulation.Process(func(proc simgo.Process) { generateSpikes(proc, cfg, rc) })
	simulation.Process(func(proc simgo.Process) {
		generateSessions(proc, simulation, cfg, rc, balancer, servers, statistics, rng)
	})

	for _, srv := range servers {
		s := srv
		simulation.Process(func(proc simgo.Process) { jitterTick(proc, cfg, s, rng) })
	}

	simulation.RunUntil(cfg.Simulation.TimeSeconds)

	statistics.Close()
	simulation.Shutdown()
	return statistics
}
