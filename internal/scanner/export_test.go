package scanner

import (
	"context"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportFiles(t *testing.T) {
	t.Parallel()

	snapshot := ScanSnapshot{
		Subnet:      "192.168.1.0/24",
		StartedAt:   time.Unix(100, 0),
		CompletedAt: time.Unix(200, 0),
		HostCount:   1,
		Hosts: []Host{
			{
				IP:         netip.MustParseAddr("192.168.1.1"),
				Hostname:   "router.local",
				MAC:        "aa:bb:cc:dd:ee:ff",
				Source:     "ping",
				DetectedAt: time.Unix(150, 0),
				OpenPorts: []OpenPort{
					{Number: 22, Service: "ssh"},
					{Number: 443, Service: "https"},
				},
			},
		},
	}

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "scan.csv")
	jsonPath := filepath.Join(dir, "scan.json")

	if err := ExportCSV(csvPath, snapshot); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}
	if err := ExportJSON(jsonPath, snapshot); err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("ReadFile(csv) error = %v", err)
	}
	if !strings.Contains(string(csvBytes), "22/ssh, 443/https") {
		t.Fatalf("csv export missing open ports: %s", string(csvBytes))
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile(json) error = %v", err)
	}
	if !strings.Contains(string(jsonBytes), `"open_ports"`) {
		t.Fatalf("json export missing open_ports: %s", string(jsonBytes))
	}
}

func TestTCPPortScanner(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	openPort := listener.Addr().(*net.TCPAddr).Port
	closedPort := reserveClosedPort(t)

	scanner := NewTCPPortScanner([]int{openPort, closedPort}, 200*time.Millisecond, WithPortConcurrency(2))
	ports, err := scanner.Scan(context.Background(), netip.MustParseAddr("127.0.0.1"))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(ports) != 1 {
		t.Fatalf("open ports = %d, want 1", len(ports))
	}
	if ports[0].Number != openPort {
		t.Fatalf("open port = %d, want %d", ports[0].Number, openPort)
	}
}

func reserveClosedPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}
