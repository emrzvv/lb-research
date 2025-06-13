package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/emrzvv/lb-research/internal/model"
	"github.com/emrzvv/lb-research/internal/stats"
)

func writeServersCfgToCSV(servers []*model.Server, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"id", "mbps", "owd_ms", "max_conn"})
	for _, s := range servers {
		w.Write([]string{
			fmt.Sprintf("%d", s.ID),
			fmt.Sprintf("%.1f", s.Parameters.Mbps),
			fmt.Sprintf("%.1f", s.Parameters.OWD),
			fmt.Sprintf("%d", s.Parameters.MaxConnections),
		})
	}
	w.Flush()
	return w.Error()
}

func writeSummaryToCSV(stats *stats.StatisticsNaive, servers []*model.Server, path, pathDropsNoServer string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"id", "picked", "served", "dropped"})
	served := make([]int, len(servers))
	for _, r := range stats.ServerRequests {
		served[r.ServerID-1]++
	}
	dropped := make([]int, len(servers))
	for _, d := range stats.Drops {
		if d.ServerID != 0 {
			dropped[d.ServerID-1]++
		}
	}
	for i := range servers {
		w.Write([]string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%d", stats.Picks[i]),
			fmt.Sprintf("%d", served[i]),
			fmt.Sprintf("%d", dropped[i]),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	fd, err := os.Create(pathDropsNoServer)
	if err != nil {
		return err
	}
	defer fd.Close()
	droppedNoServer := 0
	wd := csv.NewWriter(fd)
	_ = wd.Write([]string{"dropped_no_server"})
	for _, d := range stats.Drops {
		if d.ServerID == 0 {
			droppedNoServer++
		}
	}
	wd.Write([]string{fmt.Sprintf("%d", droppedNoServer)})
	wd.Flush()
	return wd.Error()
}

func writeSnapshotsToCSV(servers []*model.Server, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	wr := csv.NewWriter(f)
	_ = wr.Write([]string{"time_s", "server_id", "connections", "owd_ms"})

	for _, s := range servers {
		for _, snap := range s.Snapshots {
			wr.Write([]string{
				fmt.Sprintf("%.5f", snap.T),
				fmt.Sprintf("%d", s.ID),
				fmt.Sprintf("%d", snap.Connections),
				fmt.Sprintf("%.5f", snap.OWD),
			})
		}
	}
	wr.Flush()
	return wr.Error()
}

func writeStatisticsToCSV(stats *stats.StatisticsNaive,
	arrivalsPath,
	requestsPath,
	dropsPath,
	redirectsPath string) error {
	fa, err := os.Create(arrivalsPath)
	if err != nil {
		return err
	}

	aw := csv.NewWriter(fa)
	_ = aw.Write([]string{"time_s", "session_id"})
	for _, event := range stats.Arrivals {
		aw.Write([]string{
			fmt.Sprintf("%.5f", event.T),
			fmt.Sprintf("%d", event.SessionID),
		})
	}
	aw.Flush()
	if err := aw.Error(); err != nil {
		return err
	}
	fa.Close()

	fr, err := os.Create(requestsPath)
	if err != nil {
		return err
	}

	fd, err := os.Create(dropsPath)
	if err != nil {
		return err
	}
	dw := csv.NewWriter(fd)

	_ = dw.Write([]string{"server_id", "session_id", "time_s", "reason"})
	for _, event := range stats.Drops {
		dw.Write([]string{
			fmt.Sprintf("%d", event.ServerID),
			fmt.Sprintf("%d", event.SessionID),
			fmt.Sprintf("%.5f", event.T),
			fmt.Sprintf("%s", event.Reason),
		})
	}
	dw.Flush()
	if err := dw.Error(); err != nil {
		return err
	}
	fd.Close()

	rw := csv.NewWriter(fr)
	_ = rw.Write([]string{"server_id", "session_id", "start_s", "end_s", "duration"})
	for _, event := range stats.ServerRequests {
		rw.Write([]string{
			fmt.Sprintf("%d", event.ServerID),
			fmt.Sprintf("%d", event.SessiontID),
			fmt.Sprintf("%.5f", event.T1),
			fmt.Sprintf("%.5f", event.T2),
			fmt.Sprintf("%.5f", event.Duration),
		})
	}
	rw.Flush()
	if err := rw.Error(); err != nil {
		return err
	}
	fr.Close()

	frd, err := os.Create(redirectsPath)
	if err != nil {
		return err
	}
	rwred := csv.NewWriter(frd)
	_ = rwred.Write([]string{"session_id", "from_id", "to_id", "time_s"})
	for _, ev := range stats.Redirects {
		rwred.Write([]string{
			fmt.Sprintf("%d", ev.SessionID),
			fmt.Sprintf("%d", ev.FromID),
			fmt.Sprintf("%d", ev.ToID),
			fmt.Sprintf("%.5f", ev.T),
		})
	}
	rwred.Flush()
	frd.Close()
	return rwred.Error()
}

func ToCSV(dir string, statistics *stats.StatisticsNaive, servers []*model.Server) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}
	if strings.HasSuffix(dir, "/") {
		dir = dir[:len(dir)-1]
	}
	err = writeServersCfgToCSV(servers, fmt.Sprintf("%s/servers.csv", dir))
	if err != nil {
		return err
	}
	err = writeSummaryToCSV(statistics,
		servers,
		fmt.Sprintf("%s/summary.csv", dir),
		fmt.Sprintf("%s/summary_drops_no_server.csv", dir))
	if err != nil {
		return err
	}
	err = writeSnapshotsToCSV(servers, fmt.Sprintf("%s/snapshots.csv", dir))
	if err != nil {
		return err
	}
	err = writeStatisticsToCSV(statistics,
		fmt.Sprintf("%s/arrivals.csv", dir),
		fmt.Sprintf("%s/requests.csv", dir),
		fmt.Sprintf("%s/drops.csv", dir),
		fmt.Sprintf("%s/redirects.csv", dir))
	if err != nil {
		return err
	}
	return nil
}
