package stats

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emrzvv/lb-research/internal/config"
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

func NewStatisticsConcurrent(cfg *config.Config, serversAmount int, out string) *StatisticsConcurrent {
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
	go CsvWriter(stc, ctx, serversAmount, out)

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
	st.cancel()
	close(st.Arrivals)
	close(st.ServerRequests)
	close(st.Drops)
	close(st.Redirects)
	close(st.Snapshots)
	st.Wg.Wait()
}

func writeSummary(served, dropped []uint64,
	droppedNoServer int, picks []int32,
	pathSum, pathDrops string) error {
	f, err := os.Create(pathSum)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"id", "picked", "served", "dropped"})
	for i := range served {
		w.Write([]string{
			strconv.Itoa(i + 1),
			strconv.Itoa(int(picks[i])),
			strconv.FormatUint(served[i], 10),
			strconv.FormatUint(dropped[i], 10),
		})
	}
	w.Flush()
	f.Close()

	fd, err := os.Create(pathDrops)
	if err != nil {
		return err
	}
	dw := csv.NewWriter(fd)
	_ = dw.Write([]string{"dropped_no_server"})
	dw.Write([]string{strconv.Itoa(droppedNoServer)})
	dw.Flush()
	fd.Close()
	return nil
}

func CsvWriter(st *StatisticsConcurrent, ctx context.Context, serversAmount int, out string) {
	defer st.Wg.Done()

	type writer struct {
		f *os.File
		w *csv.Writer
	}

	open := func(name string, header []string) (*writer, error) {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return nil, err
		}
		f, err := os.Create(out + "/" + name)
		if err != nil {
			return nil, err
		}
		cw := csv.NewWriter(f)
		if err := cw.Write(header); err != nil {
			f.Close()
			return nil, err
		}
		return &writer{f, cw}, nil
	}

	arrW, _ := open("arrivals.csv", []string{"time_s", "session_id"})
	reqW, _ := open("requests.csv", []string{"server_id", "session_id", "start_s", "end_s", "duration"})
	dropW, _ := open("drops.csv", []string{"server_id", "session_id", "time_s", "reason"})
	redW, _ := open("redirects.csv", []string{"session_id", "from_id", "to_id", "time_s"})
	snapW, _ := open("snapshots.csv", []string{"time_s", "server_id", "connections", "owd_ms"})

	served := make([]uint64, serversAmount)
	dropped := make([]uint64, serversAmount)
	var droppedNoServer uint64

	flushTicker := time.NewTicker(800 * time.Millisecond)
	defer flushTicker.Stop()

	flushAll := func() {
		for _, w := range []*writer{arrW, reqW, dropW, redW, snapW} {
			w.w.Flush()
		}
	}

	for {
		select {
		case <-ctx.Done():
			// fmt.Println("closing statistics!")
			for ev := range st.Arrivals {
				_ = arrW.w.Write([]string{fmt.Sprintf("%.5f", ev.T), fmt.Sprintf("%d", ev.SessionID)})
			}
			for ev := range st.ServerRequests {
				_ = reqW.w.Write([]string{fmt.Sprintf("%d", ev.ServerID), fmt.Sprintf("%d", ev.SessiontID), fmt.Sprintf("%.5f", ev.T1), fmt.Sprintf("%.5f", ev.T2), fmt.Sprintf("%.5f", ev.Duration)})
			}
			for ev := range st.Drops {
				_ = dropW.w.Write([]string{fmt.Sprintf("%d", ev.ServerID), fmt.Sprintf("%d", ev.SessionID), fmt.Sprintf("%.5f", ev.T), ev.Reason})
			}
			for ev := range st.Redirects {
				_ = redW.w.Write([]string{fmt.Sprintf("%d", ev.SessionID), fmt.Sprintf("%d", ev.FromID), fmt.Sprintf("%d", ev.ToID), fmt.Sprintf("%.5f", ev.T)})
			}

			flushAll()
			writeSummary(served, dropped, int(droppedNoServer), st.Picks, out+"/summary.csv", out+"/summary_drops_no_server.csv")
			_ = arrW.f.Close()
			_ = reqW.f.Close()
			_ = dropW.f.Close()
			_ = redW.f.Close()
			_ = snapW.f.Close()
			return
		// arrivals
		case ev, ok := <-st.Arrivals:
			// fmt.Println("event on arrivals!")
			if !ok {
				st.Arrivals = nil
				continue
			}
			_ = arrW.w.Write([]string{
				fmt.Sprintf("%.5f", ev.T),
				fmt.Sprintf("%d", ev.SessionID),
			})
		// requests
		case ev, ok := <-st.ServerRequests:
			// fmt.Println("event on server requests!")
			if !ok {
				st.ServerRequests = nil
				continue
			}
			_ = reqW.w.Write([]string{
				fmt.Sprintf("%d", ev.ServerID),
				fmt.Sprintf("%d", ev.SessiontID),
				fmt.Sprintf("%.5f", ev.T1),
				fmt.Sprintf("%.5f", ev.T2),
				fmt.Sprintf("%.5f", ev.Duration),
			})
			served[ev.ServerID-1]++
		// drops
		case ev, ok := <-st.Drops:
			// fmt.Println("event on server drops!")
			if !ok {
				st.Drops = nil
				continue
			}
			if ev.ServerID == 0 {
				droppedNoServer++
			} else {
				dropped[ev.ServerID-1]++
			}
			_ = dropW.w.Write([]string{
				fmt.Sprintf("%d", ev.ServerID),
				fmt.Sprintf("%d", ev.SessionID),
				fmt.Sprintf("%.5f", ev.T),
				fmt.Sprintf("%s", ev.Reason),
			})
		// redirects
		case ev, ok := <-st.Redirects:
			// fmt.Println("event on redirects!")
			if !ok {
				st.Redirects = nil
				continue
			}
			_ = redW.w.Write([]string{
				fmt.Sprintf("%d", ev.SessionID),
				fmt.Sprintf("%d", ev.FromID),
				fmt.Sprintf("%d", ev.ToID),
				fmt.Sprintf("%.5f", ev.T),
			})
		case ev, ok := <-st.Snapshots:
			// fmt.Println("event on server snapshots!")
			if !ok {
				st.Snapshots = nil
				continue
			}
			_ = snapW.w.Write([]string{
				fmt.Sprintf("%.5f", ev.T),
				fmt.Sprintf("%d", ev.ServerID),
				fmt.Sprintf("%d", ev.Connections),
				fmt.Sprintf("%.5f", ev.OWD),
			})
		case <-flushTicker.C:
			flushAll()
		}
	}
}
