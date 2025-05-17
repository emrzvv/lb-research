package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"

	"github.com/fschuetz04/simgo"
)

const (
	segmentBytes   = 1_000_000 // сколько весит (байтах) 1 .ts-фрагмент
	simulationTime = 600.0     // секунд
	simulationStep = 1.0       // шаг симуляции (для сбора snapshot'ов и построения графиков)
	serversAmount  = 20        // кол-во серверов
	sigmaServer    = 0.25      // CV лог-нормального шума

	segmentDuration = 6     // длительность одного фрагмента (секунд)
	bitrate         = 4.0   // fullhd TODO: variying
	meanMbps        = 100.0 // средняя пропускная способность
	stdMbps         = 10.0  // стандартное отклонение для Mbps

	meanOWD = 100.0 // среднее one-way delay (e2e-delay) в мс
	stdOWD  = 30.0  // стандартное отклонение OWD
)

const (
	owdTick       = 1.0   // шаг обновления OWD
	owdJitterSTD  = 10.0  // разброс джиттера, мс
	pSpike        = 0.002 // 0.2% шанс спайка на каждом тике
	spikeExtra    = 300.0 // мс добавки при спайке
	spikeDuration = 5.0   // длительность спайка
)

var (
	fragmentWeights = []struct {
		maxFragmentsPerRequest int
		probability            float64
	}{
		{15, 0.55}, {100, 0.30}, {300, 0.10}, {900, 0.05},
	}

	baseLambda    = 200.0 // rps
	lambdaMu      sync.RWMutex
	currentLambda = baseLambda
	spikes        = []Spike{
		{At: 120, Duration: 30, Factor: 5},
		{At: 300, Duration: 60, Factor: 3},
		{At: 450, Duration: 25, Factor: 2},
	}
)

type ServerParameters struct {
	Mbps           float64
	OWD            float64
	MaxConnections int
	QueryLength    int
}

type Statistics struct {
	mu             sync.Mutex
	Arrivals       []*ArrivalEvent
	ServerRequests []*RequestEvent
	Drops          []*DropEvent
	Picks          []int
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

type DropEvent struct {
	ServerID int
	T        float64
	Reason   string
}

type ServerSnapshot struct {
	T           float64
	Connections int
	OWD         float64
}

type Server struct {
	ID                 int
	CurrentConnections int
	CurrentOWD         float64
	Parameters         *ServerParameters
	Snapshots          []*ServerSnapshot
	mu                 sync.Mutex
}

type Spike struct {
	At       float64
	Duration float64
	Factor   float64
}

// пока безочередное обслуживание. превышаем кол-во допустимых соединений => отказ
func (s *Server) HandleRequest(p simgo.Process, start float64, statistics *Statistics) bool {
	s.mu.Lock()
	if s.CurrentConnections >= s.Parameters.MaxConnections {
		s.mu.Unlock()
		statistics.mu.Lock()
		statistics.Drops = append(statistics.Drops, &DropEvent{
			ServerID: s.ID,
			T:        start,
			Reason:   "max_conn",
		})
		statistics.mu.Unlock()
		return false
	}
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
	return true
}

func (s *Server) getDuration() float64 { // TODO: зависимость от кол-ва соединений?
	meainTx := float64(segmentBytes*8) / (s.Parameters.Mbps * 1_000_000)
	lnMean := math.Log(meainTx)
	tx := math.Exp(rand.NormFloat64()*sigmaServer + lnMean)
	rtt := tx + 2*s.CurrentOWD/1000.0 // to seconds
	return rtt
}

type Balancer struct {
	servers []*Server
}

func (b *Balancer) PickServer() *Server {
	return b.servers[rand.Intn(len(b.servers))]
}

func randomFragments() int {
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

func generateRequest(p simgo.Process,
	sim *simgo.Simulation,
	balancer *Balancer,
	statistics *Statistics) {
	// отдельная горутина
	// которая раз в simulationStep делает снэпшоты каждого сервера
	sim.Process(func(sp simgo.Process) { // TODO: вынести
		for t := 0.0; t < simulationTime; t += simulationStep {
			sp.Wait(sp.Timeout(simulationStep))
			for _, s := range balancer.servers {
				s.mu.Lock()
				snap := &ServerSnapshot{
					T:           sp.Now(),
					Connections: s.CurrentConnections,
					OWD:         s.CurrentOWD,
				}
				s.Snapshots = append(s.Snapshots, snap)
				s.mu.Unlock()
			}
		}
	})

	sim.Process(func(proc simgo.Process) { // TODO: вынести
		for _, sp := range spikes {
			wait := sp.At - proc.Now()
			if wait > 0 {
				proc.Wait(proc.Timeout(wait))
			}
			lambdaMu.Lock()
			currentLambda = baseLambda * sp.Factor
			lambdaMu.Unlock()

			proc.Wait(proc.Timeout(sp.Duration))
			lambdaMu.Lock()
			currentLambda = baseLambda
			lambdaMu.Unlock()
		}
	})

	for {
		lambdaMu.RLock()
		rate := currentLambda
		lambdaMu.RUnlock()
		ia := rand.ExpFloat64() / rate
		if ia < 1e-6 {
			ia = 1e-6
		}
		p.Wait(p.Timeout(ia))
		now := p.Now()
		statistics.mu.Lock()
		statistics.Arrivals = append(statistics.Arrivals, &ArrivalEvent{T: now})
		statistics.mu.Unlock()
		pickedServer := balancer.PickServer()
		statistics.mu.Lock()
		statistics.Picks[pickedServer.ID-1]++
		statistics.mu.Unlock()
		sim.Process(func(session simgo.Process) {
			fragments := randomFragments()
			for n := 0; n < fragments; n++ {
				startFrag := session.Now()
				ok := pickedServer.HandleRequest(session, startFrag, statistics)
				if !ok {
					break
				}
				session.Wait(session.Timeout(segmentDuration))
			}
		})
		// sim.Process(func(proc simgo.Process) { pickedServer.HandleRequest(proc, now, statistics) })
	}
}

func main() {
	simulation := simgo.NewSimulation()
	var servers []*Server
	statistics := &Statistics{
		mu:             sync.Mutex{},
		Arrivals:       make([]*ArrivalEvent, 0),
		ServerRequests: make([]*RequestEvent, 0),
		Drops:          make([]*DropEvent, 0),
		Picks:          make([]int, serversAmount),
	}

	for i := range serversAmount {
		// normalMbps := distuv.Normal{Mu: meanMbps, Sigma: stdMbps, Src: rand.NewSource(time.Now().UnixNano())}
		// normalRTT  := distuv.Normal{Mu: meanRTT, Sigma: stdRTT, Src: rand.NewSource(time.Now().UnixNano())}

		mbps := rand.NormFloat64()*stdMbps + meanMbps
		owd := rand.NormFloat64()*stdOWD + meanOWD

		p := &ServerParameters{
			Mbps:           mbps,
			OWD:            owd,
			MaxConnections: int(math.Floor(mbps / bitrate)),
			QueryLength:    10,
		}
		s := &Server{
			ID:                 i + 1,
			CurrentConnections: 0,
			CurrentOWD:         50.0,
			Parameters:         p,
			Snapshots:          make([]*ServerSnapshot, 0),
		}

		servers = append(servers, s)
	}
	balancer := &Balancer{
		servers: servers,
	}

	simulation.Process(func(p simgo.Process) { generateRequest(p, simulation, balancer, statistics) })
	simulation.RunUntil(simulationTime)

	_ = os.MkdirAll("./csv", 0o755)
	writeServersCfgToCSV(servers, "./csv/servers.csv")
	writeSummaryToCSV(statistics, servers, "./csv/summary.csv")
	writeSnapshotsToCSV(servers, "./csv/snapshots.csv")
	writeStatisticsToCSV(statistics, "./csv/arrivals.csv", "./csv/requests.csv", "./csv/drops.csv")

	fmt.Printf("Done!\n")
}
