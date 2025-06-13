package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
)

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

func CsvWriter(st *stats.StatisticsConcurrent, ctx context.Context, out string, servers []*model.Server) {
	defer st.Wg.Done()

	if err := writeServersCfgToCSV(servers, out+"/servers.csv"); err != nil {
		log.Fatalf("cannot write servers.csv: %v", err)
	}

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

	served := make([]uint64, len(servers))
	dropped := make([]uint64, len(servers))
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
		// arrivals
		case ev, ok := <-st.Arrivals:
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
			if !ok {
				st.Snapshots = nil
				continue
			}
			_ = snapW.w.Write([]string{
				fmt.Sprintf("%.5f", ev.T),
				fmt.Sprintf("%.5f", ev.ServerID),
				fmt.Sprintf("%d", ev.Connections),
				fmt.Sprintf("%.5f", ev.OWD),
			})
		case <-flushTicker.C:
			flushAll()
		case <-ctx.Done():
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
		}
	}
}
