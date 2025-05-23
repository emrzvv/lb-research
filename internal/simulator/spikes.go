package simulator

import (
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/fschuetz04/simgo"
)

func generateSpikes(
	proc simgo.Process,
	cfg *config.Config,
	rc *rateCtrl) {

	for _, sp := range cfg.Spikes {
		wait := sp.At - proc.Now()
		if wait > 0 {
			proc.Wait(proc.Timeout(wait))
		}
		rc.Set(cfg.Traffic.BaseRPS * sp.Factor)
		proc.Wait(proc.Timeout(sp.Duration))
		rc.Set(cfg.Traffic.BaseRPS)
	}
}
