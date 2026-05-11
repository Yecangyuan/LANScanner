package scanner

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

type OpenPort struct {
	Number  int    `json:"number"`
	Service string `json:"service,omitempty"`
}

func (p OpenPort) Label() string {
	if p.Service == "" {
		return fmt.Sprintf("%d", p.Number)
	}
	return fmt.Sprintf("%d/%s", p.Number, p.Service)
}

type Host struct {
	IP         netip.Addr `json:"ip"`
	Hostname   string     `json:"hostname,omitempty"`
	MAC        string     `json:"mac,omitempty"`
	Vendor     string     `json:"vendor,omitempty"`
	Source     string     `json:"source,omitempty"`
	DetectedAt time.Time  `json:"detected_at"`
	OpenPorts  []OpenPort `json:"open_ports,omitempty"`
}

type Progress struct {
	Subnet    string
	Total     int
	Completed int
	Alive     int
	Current   string
	StartedAt time.Time
}

type EventType string

const (
	EventStarted   EventType = "started"
	EventProgress  EventType = "progress"
	EventHostFound EventType = "host_found"
	EventDone      EventType = "done"
	EventError     EventType = "error"
)

type Event struct {
	Type     EventType
	Progress Progress
	Host     Host
	Err      error
}

type ProbeResult struct {
	Alive     bool
	Hostname  string
	MAC       string
	Vendor    string
	Source    string
	OpenPorts []OpenPort
}

type Prober interface {
	Probe(ctx context.Context, ip netip.Addr) (ProbeResult, error)
}

type PortScanner interface {
	Scan(ctx context.Context, ip netip.Addr) ([]OpenPort, error)
}

type ScanSnapshot struct {
	Subnet      string    `json:"subnet"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	HostCount   int       `json:"host_count"`
	Hosts       []Host    `json:"hosts"`
}

func CloneHosts(hosts []Host) []Host {
	cloned := make([]Host, 0, len(hosts))
	for _, host := range hosts {
		copyHost := host
		copyHost.OpenPorts = append([]OpenPort(nil), host.OpenPorts...)
		cloned = append(cloned, copyHost)
	}
	return cloned
}

func JoinPortLabels(ports []OpenPort, separator string) string {
	if len(ports) == 0 {
		return ""
	}

	labels := make([]string, 0, len(ports))
	for _, port := range ports {
		labels = append(labels, port.Label())
	}

	return strings.Join(labels, separator)
}

func SummarizePorts(ports []OpenPort, limit int) string {
	if len(ports) == 0 {
		return ""
	}
	if limit <= 0 || len(ports) <= limit {
		return JoinPortLabels(ports, ", ")
	}

	labels := make([]string, 0, limit+1)
	for _, port := range ports[:limit] {
		labels = append(labels, port.Label())
	}
	labels = append(labels, fmt.Sprintf("+%d more", len(ports)-limit))
	return strings.Join(labels, ", ")
}
