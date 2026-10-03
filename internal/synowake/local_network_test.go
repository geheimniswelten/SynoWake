package synowake

import (
	"encoding/json"
	"strings"
	"testing"
)

func fixtureNetwork(t *testing.T, name, address string) LocalNetwork {
	t.Helper()
	result, ok := parseLocalNetwork(name, address)
	if !ok {
		t.Fatalf("invalid test network: %s", address)
	}
	return result
}

func TestLocalNetworkSuggestionsRespectActualMasks(t *testing.T) {
	for _, tc := range []struct{ address, cidr, search, broadcast string }{
		{"192.167.178.21/24", "192.167.178.0/24", "192.167.178.0/24", "192.167.178.255"},
		{"192.167.178.200/25", "192.167.178.128/25", "192.167.178.128/25", "192.167.178.255"},
		{"192.167.178.21/30", "192.167.178.20/30", "192.167.178.20/30", "192.167.178.23"},
		{"10.42.7.21/16", "10.42.0.0/16", "10.42.7.0/24", "10.42.255.255"},
		{"192.168.3.21/23", "192.168.2.0/23", "192.168.3.0/24", "192.168.3.255"},
		{"192.167.178.21/31", "192.167.178.20/31", "", ""},
		{"192.167.178.21/32", "192.167.178.21/32", "", ""},
	} {
		n := fixtureNetwork(t, "eth0", tc.address)
		if n.CIDR != tc.cidr || n.SearchCIDR != tc.search || n.Broadcast != tc.broadcast {
			t.Errorf("wrong suggestions for %s: %+v", tc.address, n)
		}
	}
	for _, address := range []string{"127.0.0.1/8", "169.254.27.104/16", "224.0.0.1/24", "0.1.2.3/24", "240.1.2.3/24", "192.167.178.21/0", "fd00::1/64", "invalid"} {
		if _, ok := parseLocalNetwork("ignored", address); ok {
			t.Errorf("offered unsuitable interface address %s", address)
		}
	}
}

func TestDiscoverySupportsNonprivateConnectedSubnetsAndRejectsMaskExpansion(t *testing.T) {
	networks := []LocalNetwork{
		fixtureNetwork(t, "eth0", "192.167.178.21/24"),
		fixtureNetwork(t, "eth1", "10.42.0.130/25"),
	}
	for _, cidr := range []string{"192.167.178.0/24", "192.167.178.21/24", "192.167.178.128/25", "192.167.178.192/26", "10.42.0.128/25", "10.42.0.192/26"} {
		if _, err := discoveryNetworkFor(cidr, networks); err != nil {
			t.Errorf("rejected attached subnet %s: %v", cidr, err)
		}
	}
	for _, cidr := range []string{"192.168.1.0/24", "192.167.179.0/24", "8.8.8.0/24", "10.42.0.0/24", "10.42.0.0/25", "192.167.178.0/23", "192.167.178.20/31", "169.254.27.0/24", "127.0.0.0/24", "224.0.0.0/24", "fd00::/120"} {
		if _, err := discoveryNetworkFor(cidr, networks); err == nil {
			t.Errorf("accepted nonlocal or unsuitable subnet %s", cidr)
		}
	}
	if _, err := discoveryNetworkFor("192.167.178.0/24", nil); err == nil {
		t.Fatal("scan without an attached interface accepted")
	}
}

func TestDeviceImportAcceptsConnectedNonprivateIPAndBroadcast(t *testing.T) {
	networks := []LocalNetwork{fixtureNetwork(t, "eth0", "192.167.178.21/24")}
	for _, address := range []string{"192.167.178.70", "192.168.1.20"} {
		d := Device{Name: "Gefunden", IP: address, MAC: "02:11:22:33:44:55", Broadcast: "192.167.178.255"}
		if err := validateDeviceForNetworks(&d, networks); err != nil {
			t.Errorf("discovered device could not be saved: %+v: %v", d, err)
		}
	}
	for _, address := range []string{"192.167.179.70", "192.167.178.0", "192.167.178.255", "8.8.8.8", "169.254.27.10", "127.0.0.1"} {
		d := Device{Name: "Falsch", IP: address, MAC: "02:11:22:33:44:55"}
		if err := validateDeviceForNetworks(&d, networks); err == nil {
			t.Errorf("nonlocal/special device address accepted: %s", address)
		}
	}
	for _, broadcast := range []string{"192.167.179.255", "192.167.178.22", "8.8.8.8", "239.0.0.1"} {
		d := Device{Name: "Falsch", IP: "192.167.178.70", MAC: "02:11:22:33:44:55", Broadcast: broadcast}
		if err := validateDeviceForNetworks(&d, networks); err == nil {
			t.Errorf("unattached nonprivate broadcast accepted: %s", broadcast)
		}
	}
}

func TestNetworkSuggestionsPreferDSMServerAddressAndPreserveInventory(t *testing.T) {
	networks := []LocalNetwork{
		fixtureNetwork(t, "docker0", "172.17.0.1/16"),
		fixtureNetwork(t, "eth0", "192.167.178.21/24"),
		fixtureNetwork(t, "eth1", "10.42.0.130/25"),
	}
	preferred := preferredLocalNetworks(networks, "192.167.178.21", "10.42.0.130")
	if preferred[0].IP != "192.167.178.21" || preferred[1].IP != "10.42.0.130" || networks[0].Interface != "docker0" {
		t.Fatalf("wrong server selection or inventory mutated: %+v", preferred)
	}
	preferred = preferredLocalNetworks(networks, "127.0.0.1", "10.42.0.130")
	if preferred[0].IP != "10.42.0.130" {
		t.Fatal("Host fallback for reverse proxy not used")
	}
}

func TestHTTPStateProvidesNASNetworkSuggestions(t *testing.T) {
	a := testApp(t)
	result := testServe(t, a, testRequest(t, a, "GET", "state", nil), 200)
	var data struct {
		Networks     []LocalNetwork `json:"networks"`
		NetworkError string         `json:"networkError"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil || len(data.Networks) != 1 || data.NetworkError != "" || data.Networks[0].SearchCIDR != "192.168.1.0/24" {
		t.Fatalf("network suggestions missing from state: %s: %v", result.Data, err)
	}
	if strings.Contains(string(result.Data), "169.254.") {
		t.Fatal("link-local interface offered for discovery")
	}
}
