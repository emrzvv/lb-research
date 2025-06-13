package main

import (
	"flag"
	"log"

	"github.com/emrzvv/lb-research/internal/balancer"
	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/export"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/simulator"
)

func main() {
	cfgPath := flag.String("cfg", "./config/default.yaml", "path to config")
	outDir := flag.String("out", "./csv", "output directory for csv")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Printf("%v", cfg)
	rng := common.NewRNG(cfg.Simulation.Seed)
	servers := model.InitServers(cfg, rng)

	b := balancer.BuildChain(cfg, servers, rng)
	if err != nil {
		log.Fatal(err)
	}
	st := simulator.Run(cfg, servers, b, rng, *outDir)

	export.ToCSV(*outDir, st, servers)
}
