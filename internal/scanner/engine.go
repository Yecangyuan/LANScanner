package scanner

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"lanscanner/internal/network"
)

type Engine struct {
	prober      Prober
	portScanner PortScanner
	concurrency int
}

type Option func(*Engine)

func WithConcurrency(concurrency int) Option {
	return func(e *Engine) {
		if concurrency > 0 {
			e.concurrency = concurrency
		}
	}
}

func NewEngine(prober Prober, opts ...Option) *Engine {
	engine := &Engine{
		prober:      prober,
		concurrency: 64,
	}

	for _, opt := range opts {
		opt(engine)
	}

	if engine.concurrency <= 0 {
		engine.concurrency = 64
	}

	return engine
}

func WithPortScanner(portScanner PortScanner) Option {
	return func(e *Engine) {
		e.portScanner = portScanner
	}
}

func (e *Engine) Scan(ctx context.Context, prefix netip.Prefix) (<-chan Event, error) {
	total, err := network.HostCount(prefix)
	if err != nil {
		return nil, err
	}

	events := make(chan Event)

	go func() {
		defer close(events)

		scanCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		progress := Progress{
			Subnet:    prefix.String(),
			Total:     total,
			StartedAt: time.Now(),
		}

		events <- Event{Type: EventStarted, Progress: progress}

		if total == 0 {
			events <- Event{Type: EventDone, Progress: progress}
			return
		}

		type probeMessage struct {
			ip     netip.Addr
			result ProbeResult
			err    error
		}

		jobs := make(chan netip.Addr)
		results := make(chan probeMessage)

		var workers sync.WaitGroup
		for i := 0; i < e.concurrency; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()

				for ip := range jobs {
					result, err := e.prober.Probe(scanCtx, ip)
					if err == nil && result.Alive && e.portScanner != nil {
						openPorts, portErr := e.portScanner.Scan(scanCtx, ip)
						if portErr != nil {
							if errors.Is(portErr, context.Canceled) {
								err = portErr
							}
						} else {
							result.OpenPorts = openPorts
						}
					}
					select {
					case results <- probeMessage{ip: ip, result: result, err: err}:
					case <-scanCtx.Done():
						return
					}
				}
			}()
		}

		go func() {
			defer close(jobs)
			_ = network.WalkHosts(prefix, func(addr netip.Addr) bool {
				select {
				case jobs <- addr:
					return true
				case <-scanCtx.Done():
					return false
				}
			})
		}()

		go func() {
			workers.Wait()
			close(results)
		}()

		var doneErr error

		for item := range results {
			if item.err != nil {
				if errors.Is(item.err, context.Canceled) {
					continue
				}
				if doneErr == nil {
					doneErr = item.err
					events <- Event{Type: EventError, Progress: progress, Err: item.err}
				}
				cancel()
				continue
			}

			progress.Completed++
			progress.Current = item.ip.String()

			if !item.result.Alive {
				events <- Event{Type: EventProgress, Progress: progress}
				continue
			}

			progress.Alive++
			events <- Event{
				Type:     EventHostFound,
				Progress: progress,
				Host: Host{
					IP:         item.ip,
					Hostname:   item.result.Hostname,
					MAC:        item.result.MAC,
					Vendor:     item.result.Vendor,
					Source:     item.result.Source,
					DetectedAt: time.Now(),
					OpenPorts:  append([]OpenPort(nil), item.result.OpenPorts...),
				},
			}
		}

		if doneErr == nil && ctx.Err() != nil {
			doneErr = ctx.Err()
		}

		events <- Event{
			Type:     EventDone,
			Progress: progress,
			Err:      doneErr,
		}
	}()

	return events, nil
}
