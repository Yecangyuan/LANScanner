package scanner

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

func NewSnapshot(progress Progress, hosts []Host) ScanSnapshot {
	return ScanSnapshot{
		Subnet:      progress.Subnet,
		StartedAt:   progress.StartedAt,
		CompletedAt: time.Now(),
		HostCount:   len(hosts),
		Hosts:       CloneHosts(hosts),
	}
}

func ExportCSV(path string, snapshot ScanSnapshot) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv export: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	rows := [][]string{
		{"subnet", snapshot.Subnet},
		{"started_at", snapshot.StartedAt.Format(time.RFC3339)},
		{"completed_at", snapshot.CompletedAt.Format(time.RFC3339)},
		{"host_count", strconv.Itoa(snapshot.HostCount)},
		{},
		{"ip", "hostname", "mac", "vendor", "source", "detected_at", "open_port_count", "open_ports"},
	}

	for _, host := range snapshot.Hosts {
		rows = append(rows, []string{
			host.IP.String(),
			host.Hostname,
			host.MAC,
			host.Vendor,
			host.Source,
			host.DetectedAt.Format(time.RFC3339),
			strconv.Itoa(len(host.OpenPorts)),
			JoinPortLabels(host.OpenPorts, ", "),
		})
	}

	if err := writer.WriteAll(rows); err != nil {
		return fmt.Errorf("write csv export: %w", err)
	}
	return writer.Error()
}

func ExportJSON(path string, snapshot ScanSnapshot) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create json export: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("write json export: %w", err)
	}
	return nil
}
