package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"net/netip"

	"lanscanner/internal/scanner"
)

func TestPersistHistoryStillSavesWhenLatestIsCorrupted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	historyDir := filepath.Join(dir, ".lanscanner-history")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	corruptedPath := filepath.Join(historyDir, "192.168.1.0_24-latest.json")
	if err := os.WriteFile(corruptedPath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	m := model{
		history: scanner.NewHistoryStore(historyDir),
		progress: scanner.Progress{
			Subnet:    "192.168.1.0/24",
			StartedAt: time.Unix(100, 0),
		},
		results: []scanner.Host{
			{
				IP:         netip.MustParseAddr("192.168.1.10"),
				DetectedAt: time.Unix(200, 0),
			},
		},
	}

	m.persistHistory()

	if m.lastErr == "" {
		t.Fatal("expected load history error to be surfaced")
	}

	loaded, ok, err := m.history.LoadLatest("192.168.1.0/24")
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}
	if !ok {
		t.Fatal("expected current snapshot to be saved despite corrupted history")
	}
	if loaded.HostCount != 1 || loaded.Hosts[0].IP.String() != "192.168.1.10" {
		t.Fatalf("loaded snapshot = %#v", loaded)
	}
}
