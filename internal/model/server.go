package model

import (
	"math"
	"sync"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
)

type ServerParameters struct {
	Mbps           float64
	OWD            float64
	MaxConnections int
}

type ServerSnapshot struct {
	T           float64
	Connections int
	OWD         float64
}

func NewSnapshot(t float64, connections int, owd float64) *ServerSnapshot {
	return &ServerSnapshot{
		T:           t,
		Connections: connections,
		OWD:         owd,
	}
}

type Server struct {
	ID                 int
	CurrentConnections int
	CurrentOWD         float64
	SpikeUntil         float64
	Parameters         *ServerParameters
	Snapshots          []*ServerSnapshot
	mu                 sync.Mutex
}

func (s *Server) AddSnapshot(t float64) {
	s.mu.Lock()
	ss := NewSnapshot(t, s.CurrentConnections, s.CurrentOWD)
	s.Snapshots = append(s.Snapshots, ss)
	s.mu.Unlock()
}

func (s *Server) Lock() {
	s.mu.Lock()
}

func (s *Server) Unlock() {
	s.mu.Unlock()
}

func (s *Server) IsOverLoaded() bool {
	return s.CurrentConnections >= s.Parameters.MaxConnections
}

type Spike struct {
	At       float64
	Duration float64
	Factor   float64
}

func InitServers(cfg *config.Config, rng *common.RNG) []*Server {
	var servers []*Server
	for i := range cfg.Cluster.Servers {
		mbps := RandNormal(cfg.Cluster.CapMean, cfg.Cluster.CapCV, rng)
		owd := RandGamma(cfg.Cluster.OWDMean, cfg.Cluster.OWDCV, rng)

		p := &ServerParameters{
			Mbps:           mbps,
			OWD:            owd,
			MaxConnections: int(math.Floor(mbps / float64(cfg.Cluster.Bitrate))),
		}

		s := &Server{
			ID:                 i + 1,
			CurrentConnections: 0,
			CurrentOWD:         p.OWD,
			Parameters:         p,
			Snapshots:          make([]*ServerSnapshot, 0),
			mu:                 sync.Mutex{},
		}

		servers = append(servers, s)
	}

	return servers
}
