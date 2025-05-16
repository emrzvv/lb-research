package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/fschuetz04/simgo"
)

const (
	labmda         = 200.0     // rps
	segmentBytes   = 1_000_000 // сколько весит (в байтах) 1 .ts-фрагмент
	simulationTime = 600.0     // секунд
	simulationStep = 1.0
	serversAmount  = 5    // кол-во серверов
	sigmaServer    = 0.25 // CV лог-нормального шума

	meanMbps = 100.0 // средняя пропускная способность
	stdMbps  = 10.0  // стандартное отклонение для Mbps

	meanRTT = 100.0 // среднее RTT в мс
	stdRTT  = 30.0  // стандартное отклонение RTT
)

type ServerParameters struct {
	Mbps           float64
	RTT            float64
	MaxConnections int
	QueryLength    int
}

type Statistics struct {
	mu             sync.Mutex
	Arrivals       []*ArrivalEvent
	ServerRequests []*RequestEvent
}

type ArrivalEvent struct {
	T float64
}

type RequestEvent struct {
	ServerID int
	T1       float64
	T2       float64
	Duration float64
}

type ServerSnapshot struct {
	T           float64
	Connections int
	RTT         float64
}

type Server struct {
	ID                 int
	CurrentConnections int
	CurrentRTT         float64
	Parameters         *ServerParameters
	Snapshots          []*ServerSnapshot
	mu                 sync.Mutex
}

// пока безотказное обслуживание
func (s *Server) HandleRequest(p simgo.Process, start float64, statistics *Statistics) {
	s.mu.Lock()
	s.CurrentConnections += 1
	s.mu.Unlock()

	duration := s.getDuration()
	p.Wait(p.Timeout(duration))

	s.mu.Lock()
	s.CurrentConnections -= 1
	s.mu.Unlock()
	statistics.mu.Lock()
	statistics.ServerRequests = append(statistics.ServerRequests, &RequestEvent{
		ServerID: s.ID,
		T1:       start,
		T2:       start + duration,
		Duration: duration,
	})
	statistics.mu.Unlock()
}

func (s *Server) getDuration() float64 {
	meainTx := float64(segmentBytes*8) / (s.Parameters.Mbps * 1_000_000)
	lnMean := math.Log(meainTx)
	tx := math.Exp(rand.NormFloat64()*sigmaServer + lnMean)
	rtt := s.CurrentRTT / 1000.0 // to seconds
	return tx + rtt
}

type Balancer struct {
	servers []*Server
}

func (b *Balancer) PickServer() *Server {
	return b.servers[rand.Intn(len(b.servers))]
}

func generateRequest(p simgo.Process,
	sim *simgo.Simulation,
	balancer *Balancer,
	statistics *Statistics) {
	// отдельная горутина
	// которая раз в simulationStep делает снэпшоты каждого сервера
	sim.Process(func(sp simgo.Process) {
		for t := 0.0; t < simulationTime; t += simulationStep {
			sp.Wait(sp.Timeout(simulationStep))
			for _, s := range balancer.servers {
				s.mu.Lock()
				snap := &ServerSnapshot{
					T:           sp.Now(),
					Connections: s.CurrentConnections,
					RTT:         s.CurrentRTT,
				}
				s.Snapshots = append(s.Snapshots, snap)
				s.mu.Unlock()
			}
		}
	})

	for {
		ia := rand.ExpFloat64() / labmda
		if ia < 1e-6 {
			ia = 1e-6
		}
		p.Wait(p.Timeout(ia))
		now := p.Now()
		statistics.mu.Lock()
		statistics.Arrivals = append(statistics.Arrivals, &ArrivalEvent{T: now})
		statistics.mu.Unlock()
		pickedServer := balancer.PickServer()
		sim.Process(func(proc simgo.Process) { pickedServer.HandleRequest(proc, now, statistics) })
	}
}

func main() {
	simulation := simgo.NewSimulation()
	var servers []*Server
	statistics := &Statistics{
		Arrivals: make([]*ArrivalEvent, 0),
	}

	for i := range serversAmount {
		// normalMbps := distuv.Normal{Mu: meanMbps, Sigma: stdMbps, Src: rand.NewSource(time.Now().UnixNano())}
		// normalRTT  := distuv.Normal{Mu: meanRTT, Sigma: stdRTT, Src: rand.NewSource(time.Now().UnixNano())}

		p := &ServerParameters{
			Mbps:           rand.NormFloat64()*stdMbps + meanMbps,
			RTT:            rand.NormFloat64()*stdRTT + meanRTT,
			MaxConnections: 10,
			QueryLength:    10,
		}
		s := &Server{
			ID:         i + 1,
			Parameters: p,
			Snapshots:  make([]*ServerSnapshot, 0),
		}

		servers = append(servers, s)
	}
	balancer := &Balancer{
		servers: servers,
	}

	simulation.Process(func(p simgo.Process) { generateRequest(p, simulation, balancer, statistics) })
	simulation.RunUntil(simulationTime)

	counts := aggregateArrivals(statistics.Arrivals, simulationStep, simulationTime)
	if err := plotArrivals(counts, simulationStep, "./results/arrivals_ts.png"); err != nil {
		fmt.Print("plot error: ", err)
	} else {
		fmt.Println("arrivals_ts.png saved")
	}

}
