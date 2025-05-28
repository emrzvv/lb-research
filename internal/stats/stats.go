package stats

import (
	"sync"

	"github.com/emrzvv/lb-research/internal/config"
)

type Statistics struct {
	mu             sync.Mutex
	Arrivals       []*ArrivalEvent
	ServerRequests []*RequestEvent
	Drops          []*DropEvent
	Redirects      []*RedirectEvent
	Picks          []int
}

type ArrivalEvent struct {
	T         float64
	SessionID int64
}

type RequestEvent struct {
	ServerID   int
	SessiontID int64
	T1         float64
	T2         float64
	Duration   float64
}

type DropEvent struct {
	ServerID  int
	SessionID int64
	T         float64
	Reason    string
}

type RedirectEvent struct {
	SessionID int64
	FromID    int
	ToID      int
	T         float64
}

func NewStatistics(cfg *config.Config) *Statistics {
	return &Statistics{
		mu:             sync.Mutex{},
		Arrivals:       make([]*ArrivalEvent, 0),
		ServerRequests: make([]*RequestEvent, 0),
		Drops:          make([]*DropEvent, 0),
		Redirects:      make([]*RedirectEvent, 0),
		Picks:          make([]int, cfg.Cluster.Servers),
	}
}

func (st *Statistics) AddArrival(ae *ArrivalEvent) {
	st.mu.Lock()
	st.Arrivals = append(st.Arrivals, ae)
	st.mu.Unlock()
}

func (st *Statistics) AddPick(id int) {
	st.mu.Lock()
	st.Picks[id]++
	st.mu.Unlock()
}

func (st *Statistics) AddDrop(de *DropEvent) {
	st.mu.Lock()
	st.Drops = append(st.Drops, de)
	st.mu.Unlock()
}

func (st *Statistics) AddRequest(re *RequestEvent) {
	st.mu.Lock()
	st.ServerRequests = append(st.ServerRequests, re)
	st.mu.Unlock()
}

func (st *Statistics) AddRedirect(re *RedirectEvent) {
	st.mu.Lock()
	st.Redirects = append(st.Redirects, re)
	st.mu.Unlock()
}
