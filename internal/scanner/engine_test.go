package scanner

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

type stubProber struct {
	results map[string]ProbeResult
	delay   time.Duration
}

type stubPortScanner struct {
	results map[string][]OpenPort
	errs    map[string]error
}

func (s stubProber) Probe(ctx context.Context, ip netip.Addr) (ProbeResult, error) {
	if s.delay > 0 {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ProbeResult{}, ctx.Err()
		case <-timer.C:
		}
	}

	result, ok := s.results[ip.String()]
	if !ok {
		return ProbeResult{Alive: false}, nil
	}
	return result, nil
}

func (s stubPortScanner) Scan(_ context.Context, ip netip.Addr) ([]OpenPort, error) {
	if err, ok := s.errs[ip.String()]; ok {
		return nil, err
	}
	return append([]OpenPort(nil), s.results[ip.String()]...), nil
}

func TestEngineScan(t *testing.T) {
	t.Parallel()

	engine := NewEngine(stubProber{
		results: map[string]ProbeResult{
			"192.168.1.1": {
				Alive:    true,
				Hostname: "router.local",
				MAC:      "aa:bb:cc:dd:ee:ff",
				Source:   "stub",
			},
		},
	}, WithConcurrency(2), WithPortScanner(stubPortScanner{
		results: map[string][]OpenPort{
			"192.168.1.1": {
				{Number: 22, Service: "ssh"},
				{Number: 443, Service: "https"},
			},
		},
	}))

	prefix := netip.MustParsePrefix("192.168.1.0/30")
	events, err := engine.Scan(context.Background(), prefix)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	var started, found, done bool
	for event := range events {
		switch event.Type {
		case EventStarted:
			started = true
			if event.Progress.Total != 2 {
				t.Fatalf("started total = %d, want 2", event.Progress.Total)
			}
		case EventHostFound:
			found = true
			if event.Host.IP.String() != "192.168.1.1" {
				t.Fatalf("found host = %s, want 192.168.1.1", event.Host.IP)
			}
			if len(event.Host.OpenPorts) != 2 {
				t.Fatalf("open ports = %d, want 2", len(event.Host.OpenPorts))
			}
		case EventDone:
			done = true
			if event.Progress.Completed != 2 {
				t.Fatalf("completed = %d, want 2", event.Progress.Completed)
			}
			if event.Progress.Alive != 1 {
				t.Fatalf("alive = %d, want 1", event.Progress.Alive)
			}
		}
	}

	if !started || !found || !done {
		t.Fatalf("started=%v found=%v done=%v", started, found, done)
	}
}

func TestEngineCancel(t *testing.T) {
	t.Parallel()

	engine := NewEngine(stubProber{delay: 200 * time.Millisecond}, WithConcurrency(1))

	ctx, cancel := context.WithCancel(context.Background())
	prefix := netip.MustParsePrefix("192.168.1.0/30")
	events, err := engine.Scan(ctx, prefix)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	cancel()

	for event := range events {
		if event.Type == EventDone && event.Err == nil {
			t.Fatal("expected cancel error on done event")
		}
	}
}

func TestEngineKeepsHostWhenPortScanFails(t *testing.T) {
	t.Parallel()

	engine := NewEngine(stubProber{
		results: map[string]ProbeResult{
			"192.168.1.1": {
				Alive:    true,
				Hostname: "router.local",
				Source:   "stub",
			},
		},
	}, WithConcurrency(1), WithPortScanner(stubPortScanner{
		errs: map[string]error{
			"192.168.1.1": errors.New("port scan failed"),
		},
	}))

	events, err := engine.Scan(context.Background(), netip.MustParsePrefix("192.168.1.0/30"))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	var foundHost bool
	for event := range events {
		switch event.Type {
		case EventError:
			t.Fatalf("unexpected scan error: %v", event.Err)
		case EventHostFound:
			foundHost = true
			if event.Host.IP.String() != "192.168.1.1" {
				t.Fatalf("found host = %s, want 192.168.1.1", event.Host.IP)
			}
			if len(event.Host.OpenPorts) != 0 {
				t.Fatalf("open ports = %d, want 0", len(event.Host.OpenPorts))
			}
		case EventDone:
			if event.Err != nil {
				t.Fatalf("done error = %v, want nil", event.Err)
			}
		}
	}

	if !foundHost {
		t.Fatal("expected discovered host to remain in results")
	}
}
