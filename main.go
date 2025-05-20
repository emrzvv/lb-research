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
	simulationTime = 600.0 // секунд
	simulationStep = 1.0   // шаг симуляции (для сбора snapshot'ов и построения графиков)
	serversAmount  = 10    // кол-во серверов
	sigmaServer    = 0.25  // CV лог-нормального шума

	segmentDuration  = 6                                         // длительность одного фрагмента (секунд)
	bitrate          = 4.0                                       // mbps fullhd bitrate; TODO: variying?
	segmentSizeBytes = bitrate * 1_000_000 / 8 * segmentDuration // сколько весит (байтах) 1 .ts-фрагмент

	meanMbps = 500.0 // средняя пропускная способность
	stdMbps  = 100.0 // стандартное отклонение для Mbps

	meanOWD = 100.0 // среднее one-way delay (e2e-delay) в мс
	stdOWD  = 30.0  // стандартное отклонение OWD
)

const (
	owdTick       = 1.0   // шаг обновления OWD
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
	spikeUntil         float64
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
	// время передачи сегмента в секундах
	tx := segmentSizeBytes * 8 / (s.Parameters.Mbps * 1_000_000)
	ln := math.Log(tx)
	// логнормальное распределение
	txDistr := math.Exp(rand.NormFloat64()*sigmaServer + ln)
	rtt := txDistr + 2*s.CurrentOWD/1000.0 // to seconds
	return rtt
}

type Balancer interface {
	PickServer() *Server
	GetServers() []*Server
}

type RandomBalancer struct {
	servers []*Server
}

func (b *RandomBalancer) PickServer() *Server {
	return b.servers[rand.Intn(len(b.servers))]
}

func (b *RandomBalancer) GetServers() []*Server {
	return b.servers
}

type RRBalancer struct {
	servers []*Server
	mu      sync.Mutex
	idx     int
}

func (b *RRBalancer) PickServer() *Server {
	b.mu.Lock()
	b.idx = (b.idx + 1) % len(b.servers)
	b.mu.Unlock()
	return b.servers[b.idx]
}

func (b *RRBalancer) GetServers() []*Server {
	return b.servers
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
	balancer Balancer,
	statistics *Statistics) {
	// отдельная горутина
	// которая раз в simulationStep делает снэпшоты каждого сервера
	sim.Process(func(sp simgo.Process) { // TODO: вынести
		for t := 0.0; t < simulationTime; t += simulationStep {
			sp.Wait(sp.Timeout(simulationStep))
			for _, s := range balancer.GetServers() {
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

		mbps := rand.NormFloat64()*stdMbps + meanMbps // TODO: new distribution needed
		owd := randGamma(meanOWD, stdOWD/meanOWD)

		p := &ServerParameters{
			Mbps:           mbps,
			OWD:            owd,
			MaxConnections: int(math.Floor(mbps / bitrate)),
			QueryLength:    10,
		}
		s := &Server{
			ID:                 i + 1,
			CurrentConnections: 0,
			CurrentOWD:         p.OWD,
			Parameters:         p,
			Snapshots:          make([]*ServerSnapshot, 0),
		}

		servers = append(servers, s)
	}

	for _, srv := range servers {
		s := srv
		simulation.Process(func(proc simgo.Process) {
			base := s.Parameters.OWD
			for proc.Now() < simulationTime {
				proc.Wait(proc.Timeout(owdTick))
				s.mu.Lock()
				now := proc.Now()
				if now < s.spikeUntil {
					s.CurrentOWD = base + spikeExtra
					s.mu.Unlock()
					continue
				}

				if rand.Float64() < pSpike {
					s.spikeUntil = now + spikeDuration
					s.CurrentOWD = base + spikeExtra
					s.mu.Unlock()
					continue
				}

				s.CurrentOWD = randGamma(meanOWD, stdOWD/meanOWD)
				s.mu.Unlock()
			}
		})
	}

	// balancer := &RandomBalancer{
	// 	servers: servers,
	// }

	balancer := &RRBalancer{
		servers: servers,
		mu:      sync.Mutex{},
		idx:     0,
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
