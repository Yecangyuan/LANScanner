package network

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
)

type InterfaceSubnet struct {
	Name    string
	Address netip.Addr
	Prefix  netip.Prefix
	Private bool
}

func (s InterfaceSubnet) CIDR() string {
	return s.Prefix.String()
}

func DiscoverSubnets() ([]InterfaceSubnet, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var subnets []InterfaceSubnet
	seen := make(map[string]struct{})

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}

			ip, ok := netip.AddrFromSlice(ipNet.IP.To4())
			if !ok || !ip.Is4() {
				continue
			}

			bits, _ := ipNet.Mask.Size()
			prefix := netip.PrefixFrom(ip.Unmap(), bits).Masked()
			key := iface.Name + ":" + prefix.String()
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			subnets = append(subnets, InterfaceSubnet{
				Name:    iface.Name,
				Address: ip.Unmap(),
				Prefix:  prefix,
				Private: ip.IsPrivate(),
			})
		}
	}

	sort.Slice(subnets, func(i, j int) bool {
		if subnets[i].Private != subnets[j].Private {
			return subnets[i].Private
		}
		if subnets[i].Name != subnets[j].Name {
			return subnets[i].Name < subnets[j].Name
		}
		return subnets[i].CIDR() < subnets[j].CIDR()
	})

	return subnets, nil
}

func DefaultSubnet() (InterfaceSubnet, error) {
	subnets, err := DiscoverSubnets()
	if err != nil {
		return InterfaceSubnet{}, err
	}
	if len(subnets) == 0 {
		return InterfaceSubnet{}, fmt.Errorf("no active IPv4 subnet found")
	}
	return subnets[0], nil
}
