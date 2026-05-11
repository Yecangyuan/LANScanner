package scanner

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"time"
)

var defaultCommonPorts = []int{
	20, 21, 22, 23, 25, 53, 67, 68, 80, 110, 123, 135, 137, 138, 139, 143, 161,
	389, 443, 445, 465, 587, 631, 993, 995, 1433, 1521, 1723, 1883, 2049, 2375,
	3000, 3306, 3389, 5000, 5432, 5900, 6379, 8000, 8080, 8443, 9000,
}

var commonServices = map[int]string{
	20:   "ftp-data",
	21:   "ftp",
	22:   "ssh",
	23:   "telnet",
	25:   "smtp",
	53:   "dns",
	67:   "dhcp",
	68:   "dhcp",
	80:   "http",
	110:  "pop3",
	123:  "ntp",
	135:  "msrpc",
	137:  "netbios-ns",
	138:  "netbios-dgm",
	139:  "netbios-ssn",
	143:  "imap",
	161:  "snmp",
	389:  "ldap",
	443:  "https",
	445:  "smb",
	465:  "smtps",
	587:  "submission",
	631:  "ipp",
	993:  "imaps",
	995:  "pop3s",
	1433: "mssql",
	1521: "oracle",
	1723: "pptp",
	1883: "mqtt",
	2049: "nfs",
	2375: "docker",
	3000: "dev-http",
	3306: "mysql",
	3389: "rdp",
	5000: "upnp-alt",
	5432: "postgres",
	5900: "vnc",
	6379: "redis",
	8000: "http-alt",
	8080: "http-proxy",
	8443: "https-alt",
	9000: "dev-alt",
}

type TCPPortScanner struct {
	ports       []int
	timeout     time.Duration
	concurrency int
}

type PortScannerOption func(*TCPPortScanner)

func WithPortConcurrency(concurrency int) PortScannerOption {
	return func(scanner *TCPPortScanner) {
		if concurrency > 0 {
			scanner.concurrency = concurrency
		}
	}
}

func DefaultPortTargets() []int {
	return append([]int(nil), defaultCommonPorts...)
}

func NewTCPPortScanner(ports []int, timeout time.Duration, opts ...PortScannerOption) TCPPortScanner {
	if len(ports) == 0 {
		ports = DefaultPortTargets()
	}
	if timeout <= 0 {
		timeout = 350 * time.Millisecond
	}

	scanner := TCPPortScanner{
		ports:       normalizePorts(ports),
		timeout:     timeout,
		concurrency: 16,
	}

	for _, opt := range opts {
		opt(&scanner)
	}
	if scanner.concurrency <= 0 {
		scanner.concurrency = 16
	}

	return scanner
}

func (s TCPPortScanner) Scan(ctx context.Context, ip netip.Addr) ([]OpenPort, error) {
	if len(s.ports) == 0 {
		return nil, nil
	}

	type result struct {
		port OpenPort
		open bool
	}

	jobs := make(chan int)
	results := make(chan result)

	var wg sync.WaitGroup
	workers := s.concurrency
	if workers > len(s.ports) {
		workers = len(s.ports)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dialer := net.Dialer{Timeout: s.timeout}
			for port := range jobs {
				address := net.JoinHostPort(ip.String(), strconv.Itoa(port))
				conn, err := dialer.DialContext(ctx, "tcp", address)
				switch {
				case err == nil:
					_ = conn.Close()
					select {
					case results <- result{port: OpenPort{Number: port, Service: serviceName(port)}, open: true}:
					case <-ctx.Done():
						return
					}
				case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
					if ctx.Err() != nil {
						return
					}
				default:
					select {
					case results <- result{open: false}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, port := range s.ports {
			select {
			case jobs <- port:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	openPorts := make([]OpenPort, 0)
	for result := range results {
		if result.open {
			openPorts = append(openPorts, result.port)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sort.Slice(openPorts, func(i, j int) bool {
		return openPorts[i].Number < openPorts[j].Number
	})
	return openPorts, nil
}

func normalizePorts(ports []int) []int {
	seen := make(map[int]struct{}, len(ports))
	normalized := make([]int, 0, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		normalized = append(normalized, port)
	}
	sort.Ints(normalized)
	return normalized
}

func serviceName(port int) string {
	if name, ok := commonServices[port]; ok {
		return name
	}
	return ""
}
