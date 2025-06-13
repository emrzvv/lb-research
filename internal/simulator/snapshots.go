package simulator

import (
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
	"github.com/fschuetz04/simgo"
)

func collectSnapshots(
	proc simgo.Process,
	cfg *config.Config,
	servers []*model.Server,
	st stats.Statistics) {

	step := cfg.Simulation.StepSeconds
	for t := 0.0; t < cfg.Simulation.TimeSeconds; t += step {
		proc.Wait(proc.Timeout(step))
		now := proc.Now()
		for _, s := range servers {
			st.AddSnapshot(s.MakeSnapshot(now))
		}
	}
}
