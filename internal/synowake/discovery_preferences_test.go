package synowake

import (
	"encoding/json"
	"testing"
)

func TestDiscoveryDefaultUsesConnectedNetworkAndValidatedSavedRange(t *testing.T) {
	networks := []LocalNetwork{
		fixtureNetwork(t, "eth0", "192.167.178.21/24"),
		fixtureNetwork(t, "eth1", "10.42.0.130/25"),
	}
	for _, tc := range []struct{ saved, want string }{
		{"", "192.167.178.0/24"},
		{"192.168.1.0/24", "192.167.178.0/24"},
		{"192.167.178.20/25", "192.167.178.0/25"},
		{"10.42.0.192/26", "10.42.0.192/26"},
		{"10.42.0.0/24", "192.167.178.0/24"},
		{"invalid", "192.167.178.0/24"},
	} {
		if got := preferredDiscoveryCIDR(networks, tc.saved); got != tc.want {
			t.Errorf("saved %q: got %q, want %q", tc.saved, got, tc.want)
		}
	}
	if got := preferredDiscoveryCIDR(nil, "192.168.1.0/24"); got != "" {
		t.Fatalf("no interfaces must leave the field empty, got %s", got)
	}
}

func TestSavedDiscoveryRangeSurvivesRestartAndIgnoresDisconnectedSubnet(t *testing.T) {
	a := testApp(t)
	for _, tc := range []struct{ saved, want string }{
		{"192.168.1.128/25", "192.168.1.128/25"},
		{"192.167.178.0/24", "192.168.1.0/24"},
	} {
		if err := a.update(func(s *Store) error { s.DiscoveryCIDR = tc.saved; return nil }); err != nil {
			t.Fatal(err)
		}
		reopened := &App{Root: a.Root, Demo: true}
		if testStore(t, reopened).DiscoveryCIDR != tc.saved {
			t.Fatal("saved range lost on restart")
		}
		result := testServe(t, reopened, testRequest(t, reopened, "GET", "state", nil), 200)
		var state struct {
			CIDR string `json:"discoveryCidr"`
		}
		if err := json.Unmarshal(result.Data, &state); err != nil || state.CIDR != tc.want {
			t.Fatalf("wrong saved discovery range in state: %s", result.Data)
		}
	}
}

func TestDiscoveredNamesRemoveOnlyKnownLocalDNSSuffixes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"ACER-Frank.fritz.box.", "ACER-Frank"},
		{"homeassistant.FRITZ.BOX", "homeassistant"},
		{"  Studio.local.  ", "Studio"},
		{"Backup.lan", "Backup"},
		{"fritz.box", "fritz.box"},
		{"nas.office.example.com.", "nas.office.example.com"},
		{"LAN-Gerät (192.167.178.24)", "LAN-Gerät (192.167.178.24)"},
		{"", ""},
	} {
		if got := discoveryName(tc.input); got != tc.want {
			t.Errorf("name %q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}
