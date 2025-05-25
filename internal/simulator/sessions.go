package simulator

import (
	"math"
	"math/rand/v2"

	"github.com/emrzvv/lb-research/internal/balancer"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
	"github.com/fschuetz04/simgo"
)

func chooseSession(cfg *config.Config) int64 {
	return rand.Int64N(cfg.Traffic.UsersAmount) + 1
}

func generateSessions(
	proc simgo.Process,
	sim *simgo.Simulation,
	cfg *config.Config,
	rc *rateCtrl,
	balancer balancer.Balancer,
	servers []*model.Server,
	st *stats.Statistics) {

	for {
		rate := rc.Get()
		ia := rand.ExpFloat64() / rate
		if ia < 1e-6 { // TODO: to config?
			ia = 1e-6
		}
		proc.Wait(proc.Timeout(ia))
		now := proc.Now()

		sessionID := chooseSession(cfg)
		st.AddArrival(&stats.ArrivalEvent{T: now, SessionID: sessionID})

		pickedServer := balancer.PickServer(sessionID)
		st.AddPick(pickedServer.ID - 1)

		sim.Process(func(session simgo.Process) {
			fragments := model.RandomFragments()
			for n := 0; n < fragments; n++ {
				startFrag := session.Now()
				ok := handleRequest(pickedServer, session, startFrag, sessionID, cfg, st)
				if !ok {
					break
				}
				session.Wait(session.Timeout(float64(cfg.Cluster.SegmentDuration)))
			}
		})
	}
}

func handleRequest(
	s *model.Server,
	session simgo.Process,
	start float64,
	sessionID int64,
	cfg *config.Config,
	st *stats.Statistics) bool {

	s.Lock()
	if s.CurrentConnections >= s.Parameters.MaxConnections {
		s.Unlock()
		st.AddDrop(&stats.DropEvent{
			ServerID:  s.ID,
			SessionID: sessionID,
			T:         start,
			Reason:    "max_conn",
		})
		return false
	}

	s.CurrentConnections++
	s.Unlock()

	duration := getDuration(s, cfg)
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

	return true
}

func getDuration(s *model.Server, cfg *config.Config) float64 {
	txMean := cfg.Cluster.SegmentSizeBytes * 8 / (s.Parameters.Mbps * 1_000_000)
	lnDist := model.RandLogNormal(math.Log(txMean), cfg.Cluster.SigmaServer)
	rtt := lnDist + 2*s.CurrentOWD/1000.0 // to seconds
	return rtt
}
