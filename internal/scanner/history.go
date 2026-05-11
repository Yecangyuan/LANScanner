package scanner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type SnapshotDiff struct {
	HasPrevious bool
	PreviousAt  time.Time
	Added       []Host
	Removed     []Host
	Unchanged   int
}

func (d SnapshotDiff) Summary() string {
	if !d.HasPrevious {
		return "No previous snapshot for this subnet."
	}

	return fmt.Sprintf(
		"Compared with last snapshot: %d new, %d offline, %d unchanged.",
		len(d.Added),
		len(d.Removed),
		d.Unchanged,
	)
}

func CompareSnapshots(previous, current ScanSnapshot) SnapshotDiff {
	diff := SnapshotDiff{
		HasPrevious: true,
		PreviousAt:  previous.CompletedAt,
	}

	previousByIP := hostsByIP(previous.Hosts)
	currentByIP := hostsByIP(current.Hosts)

	for ip, host := range currentByIP {
		if _, exists := previousByIP[ip]; exists {
			diff.Unchanged++
			continue
		}
		diff.Added = append(diff.Added, host)
	}

	for ip, host := range previousByIP {
		if _, exists := currentByIP[ip]; exists {
			continue
		}
		diff.Removed = append(diff.Removed, host)
	}

	sortHostsByIP(diff.Added)
	sortHostsByIP(diff.Removed)
	return diff
}

type HistoryStore struct {
	dir string
}

func NewHistoryStore(dir string) HistoryStore {
	return HistoryStore{dir: dir}
}

func (s HistoryStore) LoadLatest(subnet string) (ScanSnapshot, bool, error) {
	path := s.latestPath(subnet)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ScanSnapshot{}, false, nil
		}
		return ScanSnapshot{}, false, fmt.Errorf("open latest snapshot: %w", err)
	}
	defer file.Close()

	var snapshot ScanSnapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return ScanSnapshot{}, false, fmt.Errorf("decode latest snapshot: %w", err)
	}
	return snapshot, true, nil
}

func (s HistoryStore) Save(snapshot ScanSnapshot) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	latestPath := s.latestPath(snapshot.Subnet)
	archivePath := s.archivePath(snapshot)

	if err := writeJSONFile(archivePath, snapshot); err != nil {
		return err
	}
	if err := writeJSONFile(latestPath, snapshot); err != nil {
		return err
	}
	return nil
}

func (s HistoryStore) latestPath(subnet string) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s-latest.json", safeSubnetName(subnet)))
}

func (s HistoryStore) archivePath(snapshot ScanSnapshot) string {
	return filepath.Join(
		s.dir,
		fmt.Sprintf(
			"%s-%s.json",
			safeSubnetName(snapshot.Subnet),
			snapshot.CompletedAt.Format("20060102-150405"),
		),
	)
}

func writeJSONFile(path string, snapshot ScanSnapshot) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".lanscanner-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp history snapshot: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		_ = file.Close()
		return fmt.Errorf("write history snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close history snapshot: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace history snapshot: %w", err)
	}
	return nil
}

var invalidPathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeSubnetName(subnet string) string {
	safe := invalidPathChars.ReplaceAllString(subnet, "_")
	if safe == "" {
		return "unknown-subnet"
	}
	return safe
}

func hostsByIP(hosts []Host) map[string]Host {
	index := make(map[string]Host, len(hosts))
	for _, host := range hosts {
		index[host.IP.String()] = host
	}
	return index
}

func sortHostsByIP(hosts []Host) {
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].IP.Less(hosts[j].IP)
	})
}
