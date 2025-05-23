package main

import (
	"flag"
	"log"

	"github.com/emrzvv/lb-research/internal/balancer"
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

	servers := model.InitServers(cfg)

	b := balancer.NewBalancer(cfg, servers)
	if err != nil {
		log.Fatal(err)
	}
	st := simulator.Run(cfg, servers, b)

	export.ToCSV(*outDir, st, servers)
}
