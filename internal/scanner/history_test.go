package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"net/netip"
)

func TestCompareSnapshots(t *testing.T) {
	t.Parallel()

	previous := ScanSnapshot{
		Subnet:      "192.168.1.0/24",
		CompletedAt: time.Unix(100, 0),
		Hosts: []Host{
			{IP: netip.MustParseAddr("192.168.1.1")},
			{IP: netip.MustParseAddr("192.168.1.2")},
		},
	}
	current := ScanSnapshot{
		Subnet:      "192.168.1.0/24",
		CompletedAt: time.Unix(200, 0),
		Hosts: []Host{
			{IP: netip.MustParseAddr("192.168.1.1")},
			{IP: netip.MustParseAddr("192.168.1.3")},
		},
	}

	diff := CompareSnapshots(previous, current)
	if !diff.HasPrevious {
		t.Fatal("expected previous snapshot to be present")
	}
	if diff.Unchanged != 1 {
		t.Fatalf("unchanged = %d, want 1", diff.Unchanged)
	}
	if len(diff.Added) != 1 || diff.Added[0].IP.String() != "192.168.1.3" {
		t.Fatalf("added = %#v, want 192.168.1.3", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].IP.String() != "192.168.1.2" {
		t.Fatalf("removed = %#v, want 192.168.1.2", diff.Removed)
	}
}

func TestHistoryStoreSaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewHistoryStore(dir)
	snapshot := ScanSnapshot{
		Subnet:      "192.168.1.0/24",
		StartedAt:   time.Unix(100, 0),
		CompletedAt: time.Unix(200, 0),
		HostCount:   1,
		Hosts: []Host{
			{IP: netip.MustParseAddr("192.168.1.1")},
		},
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, ok, err := store.LoadLatest(snapshot.Subnet)
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}
	if !ok {
		t.Fatal("expected latest snapshot to exist")
	}
	if loaded.Subnet != snapshot.Subnet {
		t.Fatalf("loaded subnet = %s, want %s", loaded.Subnet, snapshot.Subnet)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "192.168.1.0_24-*.json"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected archived snapshot file")
	}
	if _, err := os.Stat(matches[0]); err != nil {
		t.Fatalf("expected archived snapshot file: %v", err)
	}
}

func TestHistoryStoreLoadLatestAfterCorruptionAndResave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewHistoryStore(dir)
	subnet := "192.168.1.0/24"
	latestPath := filepath.Join(dir, "192.168.1.0_24-latest.json")

	if err := os.WriteFile(latestPath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, ok, err := store.LoadLatest(subnet); err == nil || ok {
		t.Fatalf("LoadLatest() expected decode failure, got ok=%v err=%v", ok, err)
	}

	snapshot := ScanSnapshot{
		Subnet:      subnet,
		StartedAt:   time.Unix(100, 0),
		CompletedAt: time.Unix(200, 0),
		HostCount:   1,
		Hosts:       []Host{{IP: netip.MustParseAddr("192.168.1.10")}},
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, ok, err := store.LoadLatest(subnet)
	if err != nil {
		t.Fatalf("LoadLatest() after save error = %v", err)
	}
	if !ok {
		t.Fatal("expected latest snapshot to be readable after resave")
	}
	if loaded.HostCount != 1 || loaded.Hosts[0].IP.String() != "192.168.1.10" {
		t.Fatalf("loaded snapshot = %#v", loaded)
	}
}

func TestHistoryStoreLeavesNoTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewHistoryStore(dir)
	snapshot := ScanSnapshot{
		Subnet:      "192.168.1.0/24",
		StartedAt:   time.Unix(100, 0),
		CompletedAt: time.Unix(200, 0),
		HostCount:   1,
		Hosts:       []Host{{IP: netip.MustParseAddr("192.168.1.1")}},
	}

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".lanscanner-tmp-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected temp files left behind: %v", matches)
	}

	fileBytes, err := os.ReadFile(filepath.Join(dir, "192.168.1.0_24-latest.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var loaded ScanSnapshot
	if err := json.Unmarshal(fileBytes, &loaded); err != nil {
		t.Fatalf("latest snapshot should contain valid json: %v", err)
	}
}
