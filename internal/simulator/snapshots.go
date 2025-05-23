package simulator

import (
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/fschuetz04/simgo"
)

func collectSnapshots(
	proc simgo.Process,
	cfg *config.Config,
	servers []*model.Server) {

	step := cfg.Simulation.StepSeconds
	for t := 0.0; t < cfg.Simulation.TimeSeconds; t += step {
		proc.Wait(proc.Timeout(step))
		now := proc.Now()
		for _, s := range servers {
			s.AddSnapshot(now)
		}
	}
}
