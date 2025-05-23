package model

import (
	"math/rand"

	"gonum.org/v1/gonum/stat/distuv"
)

var fragmentWeights = []struct { // TODO: to config?
	maxFragmentsPerRequest int
	probability            float64
}{
	{15, 0.55}, {100, 0.30}, {300, 0.10}, {900, 0.05},
}

func RandGamma(mean, cv float64) float64 {
	if cv <= 0 {
		panic("cv must be > 0")
	}

	k := 1.0 / (cv * cv)
	theta := mean / k

	g := distuv.Gamma{
		Alpha: k,
		Beta:  1.0 / theta,
	}
	return g.Rand()
}

func RandNormal(mean, cv float64) float64 {
	n := distuv.Normal{
		Mu:    mean,
		Sigma: mean * cv,
	}

	return n.Rand()
}

func RandLogNormal(mean, sigma float64) float64 {
	lnDist := distuv.LogNormal{
		Mu:    mean,
		Sigma: sigma,
	}

	return lnDist.Rand()
}

func RandomFragments() int {
	r := rand.Float64()
	acc := 0.0
	for _, w := range fragmentWeights {
		acc += w.probability
		if r <= acc {
			return 1 + rand.Intn(w.maxFragmentsPerRequest) // равномерно 1..max
		}
	}
	return 10 // fallback
}
