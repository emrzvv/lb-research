package simulator

import (
	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/fschuetz04/simgo"
)

func jitterTick(
	proc simgo.Process,
	cfg *config.Config,
	server *model.Server,
	rng *common.RNG) {

	base := server.Parameters.OWD
	for proc.Now() < cfg.Simulation.TimeSeconds {
		proc.Wait(proc.Timeout(cfg.Jitter.Tick))
		server.Lock()
		now := proc.Now()
		if now < server.SpikeUntil {
			server.CurrentOWD = base + cfg.Jitter.SpikeExtra
			server.Unlock()
			continue
		}

		if rng.Float64() < cfg.Jitter.SpikeP {
			server.SpikeUntil = now + cfg.Jitter.SpikeDur
			server.CurrentOWD = base + cfg.Jitter.SpikeExtra
			server.Unlock()
			continue
		}

		server.CurrentOWD = model.RandGamma(cfg.Cluster.OWDMean, cfg.Cluster.OWDCV, rng)
		server.Unlock()
	}
}
