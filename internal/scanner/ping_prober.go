package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type PingProber struct {
	timeout         time.Duration
	hostnameTimeout time.Duration
}

func NewPingProber(timeout, hostnameTimeout time.Duration) PingProber {
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	if hostnameTimeout <= 0 {
		hostnameTimeout = 250 * time.Millisecond
	}

	return PingProber{
		timeout:         timeout,
		hostnameTimeout: hostnameTimeout,
	}
}

func (p PingProber) Probe(ctx context.Context, ip netip.Addr) (ProbeResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, "ping", pingArgs(ip)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err := cmd.Run()
	switch {
	case err == nil:
	case errors.Is(ctx.Err(), context.Canceled):
		return ProbeResult{}, ctx.Err()
	case errors.Is(probeCtx.Err(), context.DeadlineExceeded):
		return ProbeResult{Alive: false}, nil
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ProbeResult{Alive: false}, nil
		}
		return ProbeResult{}, fmt.Errorf("run ping: %w", err)
	}

	result := ProbeResult{
		Alive:  true,
		Source: "ping",
	}

	lookupCtx, lookupCancel := context.WithTimeout(ctx, p.hostnameTimeout)
	defer lookupCancel()

	names, err := net.DefaultResolver.LookupAddr(lookupCtx, ip.String())
	if err == nil && len(names) > 0 {
		result.Hostname = strings.TrimSuffix(names[0], ".")
	}

	if mac, err := lookupMAC(ctx, ip); err == nil {
		result.MAC = mac
	}

	return result, nil
}

func pingArgs(ip netip.Addr) []string {
	if runtime.GOOS == "windows" {
		return []string{"-n", "1", ip.String()}
	}
	return []string{"-c", "1", ip.String()}
}

var macAddressPattern = regexp.MustCompile(`(?i)([0-9a-f]{2}[:-]){5}[0-9a-f]{2}`)

func lookupMAC(ctx context.Context, ip netip.Addr) (string, error) {
	cmd := exec.CommandContext(ctx, "arp", "-n", ip.String())
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	match := macAddressPattern.FindString(string(output))
	if match == "" {
		return "", fmt.Errorf("no MAC address in arp output")
	}

	return strings.ToLower(strings.ReplaceAll(match, "-", ":")), nil
}
