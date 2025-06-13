package model

import (
	"math"
	"sync"

	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/metric"
	"github.com/emrzvv/lb-research/internal/stats"
	"github.com/fschuetz04/simgo"
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

func NewSnapshot(t float64, connections int, owd float64) *stats.SnapshotEvent {
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
	mu                 sync.Mutex
}

func (s *Server) MakeSnapshot(t float64) *stats.SnapshotEvent {
	s.mu.Lock()
	ss := NewSnapshot(t, s.CurrentConnections, s.CurrentOWD)
	s.mu.Unlock()
	return ss
}

func (s *Server) Lock() {
	s.mu.Lock()
}

func (s *Server) Unlock() {
	s.mu.Unlock()
}

func (s *Server) IsOverLoaded() bool {
	s.mu.Lock()
	result := s.CurrentConnections >= s.Parameters.MaxConnections
	s.mu.Unlock()
	return result
}

func (s *Server) HandleRequest(
	session simgo.Process,
	start float64,
	penalty float64,
	sessionID int64,
	cfg *config.Config,
	st stats.Statistics,
	rng *common.RNG) bool {

	s.Lock()
	if s.CurrentConnections >= s.Parameters.MaxConnections {
		s.Unlock()
		return false
	}

	s.CurrentConnections++
	s.Unlock()

	duration := s.getDuration(cfg, rng) + penalty/1000.0
	session.Wait(session.Timeout(duration))
	s.Lock()
	s.CurrentConnections--
	s.Unlock()

	st.AddRequest(&stats.RequestEvent{
		ServerID:   s.ID,
		SessiontID: sessionID,
		T1:         start,
		T2:         start + duration,
		Duration:   duration,
	})
	if ch := metric.RTTChan; ch != nil {
		select {
		case ch <- metric.RequestRTT{
			ServerID: s.ID,
			RTT:      duration,
			When:     start,
		}:
		default:
		}
	}

	return true
}

func (s *Server) getDuration(cfg *config.Config, rng *common.RNG) float64 {
	txMean := cfg.Cluster.SegmentSizeBytes * 8 / (s.Parameters.Mbps * 1_000_000)
	lnDist := RandLogNormal(math.Log(txMean), cfg.Cluster.SigmaServer, rng)
	rtt := lnDist + 2*s.CurrentOWD/1000.0 // to seconds
	return rtt
}

type Spike struct {
	At       float64
	Duration float64
	Factor   float64
}

func InitServers(cfg *config.Config, rng *common.RNG) []*Server {
	var servers []*Server
	for i := range cfg.Cluster.Servers {
		// mbps := RandNormal(cfg.Cluster.CapMean, cfg.Cluster.CapCV, rng)
		sigmaLn := math.Sqrt(math.Log(1 + cfg.Cluster.CapCV*cfg.Cluster.CapCV))
		mbps := RandLogNormal(math.Log(cfg.Cluster.CapMean)-0.5*sigmaLn*sigmaLn, cfg.Cluster.CapCV, rng)
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
			mu:                 sync.Mutex{},
		}

		servers = append(servers, s)
	}

	return servers
}
