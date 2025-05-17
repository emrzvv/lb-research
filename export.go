package main

import (
	"encoding/csv"
	"fmt"
	"os"
)

func writeServersCfgToCSV(servers []*Server, path string) error {
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

func writeSummaryToCSV(stats *Statistics, servers []*Server, path string) error {
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
		dropped[d.ServerID-1]++
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
	return w.Error()
}

func writeSnapshotsToCSV(servers []*Server, path string) error {
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

func writeStatisticsToCSV(stats *Statistics,
	arrivalsPath,
	requestsPath,
	dropsPath string) error {
	fa, err := os.Create(arrivalsPath)
	if err != nil {
		return err
	}

	aw := csv.NewWriter(fa)
	_ = aw.Write([]string{"time_s"})
	for _, event := range stats.Arrivals {
		aw.Write([]string{
			fmt.Sprintf("%.5f", event.T),
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

	_ = dw.Write([]string{"server_id", "time_s", "reason"})
	for _, event := range stats.Drops {
		dw.Write([]string{
			fmt.Sprintf("%d", event.ServerID),
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
	_ = rw.Write([]string{"server_id", "start_s", "end_s", "duration"})
	for _, event := range stats.ServerRequests {
		rw.Write([]string{
			fmt.Sprintf("%d", event.ServerID),
			fmt.Sprintf("%.5f", event.T1),
			fmt.Sprintf("%.5f", event.T2),
			fmt.Sprintf("%.5f", event.Duration),
		})
	}
	rw.Flush()
	fr.Close()
	return rw.Error()
}
