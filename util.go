package main

import (
	"gonum.org/v1/gonum/stat/distuv"
)

func randGamma(mean, cv float64) float64 {
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
