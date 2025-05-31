package simulator

import (
	"github.com/emrzvv/lb-research/internal/balancer"
	"github.com/emrzvv/lb-research/internal/common"
	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
	"github.com/fschuetz04/simgo"
)

func chooseSession(cfg *config.Config, rng *common.RNG) int64 {
	return rng.Int63n(cfg.Traffic.UsersAmount) + 1
}

func generateSessions(
	proc simgo.Process,
	sim *simgo.Simulation,
	cfg *config.Config,
	rc *rateCtrl,
	balancer balancer.Balancer,
	servers []*model.Server,
	st *stats.Statistics,
	rng *common.RNG) {

	for {
		rate := rc.Get()
		ia := rng.ExpFloat64() / rate
		if ia < 1e-6 { // TODO: to config?
			ia = 1e-6
		}
		proc.Wait(proc.Timeout(ia))
		now := proc.Now()

		sessionID := chooseSession(cfg, rng)
		st.AddArrival(&stats.ArrivalEvent{T: now, SessionID: sessionID})

		pickedServer := balancer.PickServer(sessionID)
		if pickedServer == nil {
			st.AddDrop(&stats.DropEvent{
				ServerID: 0, SessionID: sessionID, T: now, Reason: "no_server"})
			continue
		}
		st.AddPick(pickedServer.ID - 1)

		sim.Process(func(session simgo.Process) {
			fragments := model.RandomFragments(rng)

			switches := 0
			penalty := 0.0

			for n := 0; n < fragments; n++ {
				retries := 0

				for {
					start := session.Now()
					ok := pickedServer.HandleRequest(session, start, penalty, sessionID, cfg, st, rng)
					if penalty > 0 {
						penalty = 0.0
					}
					if ok {
						break
					}
					retries++
					if retries <= cfg.Cluster.MaxRetriesPerSegment {
						continue
					}

					if switches >= cfg.Cluster.MaxSwitchesPerSession {
						st.AddDrop(&stats.DropEvent{
							ServerID:  pickedServer.ID,
							SessionID: sessionID,
							T:         start,
							Reason:    "max_switches",
						})
						return
					}

					newPickedServer := balancer.PickServer(sessionID)
					if newPickedServer == nil {
						st.AddDrop(&stats.DropEvent{
							ServerID: 0, SessionID: sessionID, T: now, Reason: "no_server"})
						return
					}
					st.AddRedirect(&stats.RedirectEvent{
						SessionID: sessionID,
						FromID:    pickedServer.ID,
						ToID:      newPickedServer.ID,
						T:         start,
					})
					pickedServer = newPickedServer
					switches++
					penalty += 100 // TODO: distribution
					retries = 0
				}

				session.Wait(session.Timeout(float64(cfg.Cluster.SegmentDuration)))
			}
		})
	}
}
