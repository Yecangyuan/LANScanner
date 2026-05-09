package scanner

import (
	"context"
	"net/netip"
	"time"
)

type Host struct {
	IP         netip.Addr
	Hostname   string
	MAC        string
	Vendor     string
	Source     string
	DetectedAt time.Time
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
	Alive    bool
	Hostname string
	MAC      string
	Vendor   string
	Source   string
}

type Prober interface {
	Probe(ctx context.Context, ip netip.Addr) (ProbeResult, error)
}
