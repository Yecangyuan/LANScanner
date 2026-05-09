package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strings"
)

var ErrIPv4Only = errors.New("only IPv4 CIDRs are supported in v1")

func ParsePrefix(raw string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parse CIDR: %w", err)
	}
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, ErrIPv4Only
	}
	return prefix, nil
}

func HostCount(prefix netip.Prefix) (int, error) {
	if !prefix.Addr().Is4() {
		return 0, ErrIPv4Only
	}

	bits := prefix.Bits()
	switch bits {
	case 32:
		return 1, nil
	case 31:
		return 2, nil
	}

	count := (uint64(1) << uint(32-bits)) - 2
	if count > uint64(math.MaxInt) {
		return 0, fmt.Errorf("CIDR is too large to scan on this platform")
	}
	return int(count), nil
}

func Hosts(prefix netip.Prefix) ([]netip.Addr, error) {
	count, err := HostCount(prefix)
	if err != nil {
		return nil, err
	}

	hosts := make([]netip.Addr, 0, count)
	err = WalkHosts(prefix, func(addr netip.Addr) bool {
		hosts = append(hosts, addr)
		return true
	})
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func WalkHosts(prefix netip.Prefix, yield func(netip.Addr) bool) error {
	start, end, err := hostRange(prefix)
	if err != nil {
		return err
	}

	for current := start; current <= end; current++ {
		if !yield(uint32ToAddr(current)) {
			return nil
		}
		if current == math.MaxUint32 {
			return nil
		}
	}

	return nil
}

func hostRange(prefix netip.Prefix) (uint32, uint32, error) {
	if !prefix.Addr().Is4() {
		return 0, 0, ErrIPv4Only
	}

	base := addrToUint32(prefix.Masked().Addr())
	bits := prefix.Bits()

	if bits == 32 {
		return base, base, nil
	}

	total := uint64(1) << uint(32-bits)
	last := uint32(uint64(base) + total - 1)

	if bits == 31 {
		return base, last, nil
	}

	return base + 1, last - 1, nil
}

func addrToUint32(addr netip.Addr) uint32 {
	raw := addr.As4()
	return binary.BigEndian.Uint32(raw[:])
}

func uint32ToAddr(value uint32) netip.Addr {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	return netip.AddrFrom4(raw)
}
