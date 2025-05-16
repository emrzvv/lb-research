package main

import (
	"fmt"
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func aggregateArrivals(events []*ArrivalEvent, step, horizon float64) []float64 {
	buckets := int(math.Ceil(horizon / step))
	counts := make([]float64, buckets)

	for _, event := range events {
		index := int(event.T / step)
		if index < buckets {
			counts[index] += 1
		}
	}
	return counts
}

func plotArrivals(counts []float64, step float64, file string) error {
	pts := make(plotter.XYs, len(counts))
	// fmt.Printf("%v+", counts)
	for i, c := range counts {
		pts[i].X = float64(i) * step
		pts[i].Y = c
	}
	p := plot.New()
	p.Title.Text = fmt.Sprintf("Количество запросов за шаг (%.0f с)", step)
	p.X.Label.Text = "Время (с)"
	p.Y.Label.Text = fmt.Sprintf("Количество запросов")
	line, err := plotter.NewLine(pts)
	if err != nil {
		return err
	}
	p.Add(line)
	return p.Save(20*vg.Centimeter, 10*vg.Centimeter, file)
}
