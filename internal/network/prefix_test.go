package network

import (
	"net/netip"
	"testing"
)

func TestHostCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		want   int
	}{
		{name: "slash24", prefix: "192.168.1.0/24", want: 254},
		{name: "slash31", prefix: "192.168.1.0/31", want: 2},
		{name: "slash32", prefix: "192.168.1.7/32", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefix := netip.MustParsePrefix(tt.prefix)
			got, err := HostCount(prefix)
			if err != nil {
				t.Fatalf("HostCount() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("HostCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHosts(t *testing.T) {
	t.Parallel()

	prefix := netip.MustParsePrefix("192.168.1.0/30")
	got, err := Hosts(prefix)
	if err != nil {
		t.Fatalf("Hosts() error = %v", err)
	}

	want := []netip.Addr{
		netip.MustParseAddr("192.168.1.1"),
		netip.MustParseAddr("192.168.1.2"),
	}

	if len(got) != len(want) {
		t.Fatalf("Hosts() length = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Hosts()[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestParsePrefixRejectsIPv6(t *testing.T) {
	t.Parallel()

	if _, err := ParsePrefix("fd00::/64"); err == nil {
		t.Fatal("ParsePrefix() expected IPv6 error")
	}
}
