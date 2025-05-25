package model

import (
	"github.com/emrzvv/lb-research/internal/common"
	"gonum.org/v1/gonum/stat/distuv"
)

var fragmentWeights = []struct { // TODO: to config?
	maxFragmentsPerRequest int
	probability            float64
}{
	{15, 0.55}, {100, 0.30}, {300, 0.10}, {900, 0.05},
}

func RandGamma(mean, cv float64, rng *common.RNG) float64 {
	if cv <= 0 {
		panic("cv must be > 0")
	}

	k := 1.0 / (cv * cv)
	theta := mean / k

	g := distuv.Gamma{
		Alpha: k,
		Beta:  1.0 / theta,
		Src:   rng,
	}
	return g.Rand()
}

func RandNormal(mean, cv float64, rng *common.RNG) float64 {
	n := distuv.Normal{
		Mu:    mean,
		Sigma: mean * cv,
		Src:   rng,
	}

	return n.Rand()
}

func RandLogNormal(mean, sigma float64, rng *common.RNG) float64 {
	lnDist := distuv.LogNormal{
		Mu:    mean,
		Sigma: sigma,
		Src:   rng,
	}

	return lnDist.Rand()
}

func RandomFragments(rng *common.RNG) int {
	r := rng.Float64()
	acc := 0.0
	for _, w := range fragmentWeights {
		acc += w.probability
		if r <= acc {
			return 1 + rng.Intn(w.maxFragmentsPerRequest) // равномерно 1..max
		}
	}
	return 10 // fallback
}
