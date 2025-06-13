package stats

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/emrzvv/lb-research/internal/config"
	"github.com/emrzvv/lb-research/internal/export"
	"github.com/emrzvv/lb-research/internal/model"
)

type Statistics interface {
	AddArrival(ae *ArrivalEvent)
	AddPick(id int)
	AddDrop(de *DropEvent)
	AddRequest(re *RequestEvent)
	AddRedirect(re *RedirectEvent)
	AddSnapshot(se *SnapshotEvent)
}

type StatisticsNaive struct {
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

type SnapshotEvent struct {
	T           float64
	ServerID    int
	Connections int
	OWD         float64
}

func NewStatisticsNaive(cfg *config.Config) *StatisticsNaive {
	return &StatisticsNaive{
		mu:             sync.Mutex{},
		Arrivals:       make([]*ArrivalEvent, 0),
		ServerRequests: make([]*RequestEvent, 0),
		Drops:          make([]*DropEvent, 0),
		Redirects:      make([]*RedirectEvent, 0),
		Picks:          make([]int, cfg.Cluster.Servers),
	}
}

func (st *StatisticsNaive) AddArrival(ae *ArrivalEvent) {
	st.mu.Lock()
	st.Arrivals = append(st.Arrivals, ae)
	st.mu.Unlock()
}

func (st *StatisticsNaive) AddPick(id int) {
	st.mu.Lock()
	st.Picks[id]++
	st.mu.Unlock()
}

func (st *StatisticsNaive) AddDrop(de *DropEvent) {
	st.mu.Lock()
	st.Drops = append(st.Drops, de)
	st.mu.Unlock()
}

func (st *StatisticsNaive) AddRequest(re *RequestEvent) {
	st.mu.Lock()
	st.ServerRequests = append(st.ServerRequests, re)
	st.mu.Unlock()
}

func (st *StatisticsNaive) AddRedirect(re *RedirectEvent) {
	st.mu.Lock()
	st.Redirects = append(st.Redirects, re)
	st.mu.Unlock()
}

type StatisticsConcurrent struct {
	Arrivals       chan *ArrivalEvent
	ServerRequests chan *RequestEvent
	Drops          chan *DropEvent
	Redirects      chan *RedirectEvent
	Snapshots      chan *SnapshotEvent
	Picks          []int32

	cancel context.CancelFunc
	Wg     sync.WaitGroup
}

func NewStatisticsConcurrent(cfg *config.Config, out string, servers []*model.Server) *StatisticsConcurrent {
	stc := &StatisticsConcurrent{
		Arrivals:       make(chan *ArrivalEvent, 1<<14),
		ServerRequests: make(chan *RequestEvent, 1<<14),
		Drops:          make(chan *DropEvent, 1<<14),
		Redirects:      make(chan *RedirectEvent, 1<<14),
		Snapshots:      make(chan *SnapshotEvent, 1<<14),
		Picks:          make([]int32, cfg.Cluster.Servers),
	}

	ctx, cancel := context.WithCancel(context.Background())
	stc.cancel = cancel
	stc.Wg.Add(1)
	go export.CsvWriter(stc, ctx, out, servers)

	return stc
}

func (st *StatisticsConcurrent) AddArrival(ae *ArrivalEvent) {
	st.Arrivals <- ae
}

func (st *StatisticsConcurrent) AddDrop(de *DropEvent) {
	st.Drops <- de
}

func (st *StatisticsConcurrent) AddRequest(re *RequestEvent) {
	st.ServerRequests <- re
}

func (st *StatisticsConcurrent) AddRedirect(re *RedirectEvent) {
	st.Redirects <- re
}

func (st *StatisticsConcurrent) AddSnapshot(se *SnapshotEvent) {
	st.Snapshots <- se
}

func (st *StatisticsConcurrent) AddPick(id int) {
	atomic.AddInt32(&st.Picks[id], 1)
}

func (st *StatisticsConcurrent) Close() {
	close(st.Arrivals)
	close(st.ServerRequests)
	close(st.Drops)
	close(st.Redirects)
	st.cancel()
	st.Wg.Wait()
}
