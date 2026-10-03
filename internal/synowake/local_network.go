package synowake

import (
	"errors"
	"net"
	"sort"
	"strings"
)

type LocalNetwork struct {
	Interface  string `json:"interface"`
	IP         string `json:"ip"`
	CIDR       string `json:"cidr"`
	SearchCIDR string `json:"searchCidr"`
	Broadcast  string `json:"broadcast"`
}

func lanIPv4(value string) net.IP {
	ip := net.ParseIP(value).To4()
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() || ip[0] == 0 || ip[0] >= 224 {
		return nil
	}
	return ip
}

func networkEnd(network *net.IPNet) net.IP {
	end := append(net.IP(nil), network.IP.To4()...)
	for i := range end {
		end[i] |= ^network.Mask[i]
	}
	return end
}

func parseLocalNetwork(name, address string) (LocalNetwork, bool) {
	ip, network, err := net.ParseCIDR(address)
	if err != nil || lanIPv4(ip.String()) == nil {
		return LocalNetwork{}, false
	}
	ones, bits := network.Mask.Size()
	if bits != 32 || ones < 1 {
		return LocalNetwork{}, false
	}
	result := LocalNetwork{Interface: name, IP: ip.String(), CIDR: network.String()}
	if ones <= 30 {
		result.Broadcast = networkEnd(network).String()
		prefix := ones
		if prefix < 24 {
			prefix = 24
		}
		mask := net.CIDRMask(prefix, 32)
		result.SearchCIDR = (&net.IPNet{IP: ip.Mask(mask), Mask: mask}).String()
	}
	return result, true
}

func localNetworks() ([]LocalNetwork, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Index < interfaces[j].Index })
	networks := []LocalNetwork{}
	for _, it := range interfaces {
		if it.Flags&net.FlagUp == 0 || it.Flags&net.FlagRunning == 0 || it.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := it.Addrs()
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if network, ok := parseLocalNetwork(it.Name, address.String()); ok {
				networks = append(networks, network)
			}
		}
	}
	return networks, nil
}

func preferredLocalNetworks(networks []LocalNetwork, addresses ...string) []LocalNetwork {
	result := append([]LocalNetwork{}, networks...)
	priority := func(network LocalNetwork) int {
		for i, address := range addresses {
			if network.IP == address {
				return i
			}
		}
		return len(addresses)
	}
	sort.SliceStable(result, func(i, j int) bool { return priority(result[i]) < priority(result[j]) })
	return result
}

// Retain a successful manual search only while its entire range is on-link.
// Otherwise prefer the active interface serving the DSM request.
func preferredDiscoveryCIDR(networks []LocalNetwork, saved string) string {
	if saved != "" {
		if network, err := discoveryNetworkFor(saved, networks); err == nil {
			return network.String()
		}
	}
	for _, network := range networks {
		if network.SearchCIDR != "" {
			return network.SearchCIDR
		}
	}
	return ""
}

func isOnLinkHost(ip net.IP, networks []LocalNetwork) bool {
	for _, local := range networks {
		_, network, err := net.ParseCIDR(local.CIDR)
		if err != nil || !network.Contains(ip) {
			continue
		}
		prefix, _ := network.Mask.Size()
		if prefix <= 30 && (ip.Equal(network.IP) || ip.Equal(networkEnd(network))) {
			continue
		}
		return true
	}
	return false
}

func discoveryNetworkFor(cidr string, networks []LocalNetwork) (*net.IPNet, error) {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ip.To4() == nil {
		return nil, errors.New("Ein IPv4-Netz als CIDR eingeben, z. B. 192.168.1.0/24.")
	}
	prefix, _ := network.Mask.Size()
	if prefix < 24 || prefix > 30 {
		return nil, errors.New("Suche auf /24 bis /30 begrenzt (höchstens 254 Geräte).")
	}
	end := networkEnd(network)
	if lanIPv4(network.IP.String()) == nil || lanIPv4(end.String()) == nil {
		return nil, errors.New("Nur lokale IPv4-LAN-Netze können durchsucht werden.")
	}
	for _, local := range networks {
		_, attached, err := net.ParseCIDR(local.CIDR)
		if err == nil && attached.Contains(network.IP) && attached.Contains(end) {
			return network, nil
		}
	}
	return nil, errors.New("Das Suchnetz muss vollständig innerhalb einer aktiven NAS-Netzwerkschnittstelle liegen. Bitte ein erkanntes NAS-Netz auswählen.")
}
