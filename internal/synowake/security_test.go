package synowake

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestMagicPacketWireFormat(t *testing.T) {
	packet, err := magicPacket("02:11:22:33:44:55")
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != 102 || !bytes.Equal(packet[:6], bytes.Repeat([]byte{255}, 6)) {
		t.Fatalf("invalid WoL frame: length=%d, prefix=%x", len(packet), packet[:6])
	}
	for i := 0; i < 16; i++ {
		if !bytes.Equal(packet[6+i*6:12+i*6], []byte{2, 17, 34, 51, 68, 85}) {
			t.Fatalf("MAC repetition %d is incorrect", i)
		}
	}
	for _, mac := range []string{"bad", "02:11:22:33:44:55:66:77", "02:11:22:33:44"} {
		if _, err := magicPacket(mac); err == nil {
			t.Errorf("accepted invalid WoL MAC %q", mac)
		}
	}
}

func TestDeviceValidationRejectsUnsafeAddresses(t *testing.T) {
	valid := Device{Name: "  Rechner  ", IP: "192.168.1.20", MAC: "02-11-22-33-44-55"}
	if err := validateDevice(&valid); err != nil {
		t.Fatal(err)
	}
	if valid.Name != "Rechner" || valid.MAC != "02:11:22:33:44:55" || valid.Port != 9 || valid.Broadcast != "255.255.255.255" {
		t.Fatalf("defaults and normalization are incorrect: %+v", valid)
	}
	tests := []struct {
		name string
		edit func(*Device)
	}{
		{"public IPv4", func(d *Device) { d.IP = "8.8.8.8" }},
		{"loopback", func(d *Device) { d.IP = "127.0.0.1" }},
		{"multicast IPv4", func(d *Device) { d.IP = "224.0.0.1" }},
		{"IPv6 private", func(d *Device) { d.IP = "fd00::1" }},
		{"IPv6 multicast", func(d *Device) { d.IP = "ff02::1" }},
		{"MAC multicast", func(d *Device) { d.MAC = "01:11:22:33:44:55" }},
		{"MAC broadcast", func(d *Device) { d.MAC = "ff:ff:ff:ff:ff:ff" }},
		{"zero MAC", func(d *Device) { d.MAC = "00:00:00:00:00:00" }},
		{"EUI64 MAC", func(d *Device) { d.MAC = "02:11:22:33:44:55:66:77" }},
		{"broadcast multicast", func(d *Device) { d.Broadcast = "239.0.0.1" }},
		{"broadcast IPv6", func(d *Device) { d.Broadcast = "fd00::1" }},
		{"negative port", func(d *Device) { d.Port = -1 }},
		{"oversized port", func(d *Device) { d.Port = 65536 }},
		{"shell ID", func(d *Device) { d.ID = "';touch bad;'" }},
		{"control character name", func(d *Device) { d.Name = "PC\nInjected" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := valid
			tc.edit(&d)
			if err := validateDevice(&d); err == nil {
				t.Fatal("unsafe device was accepted")
			}
		})
	}
}

func TestDiscoveryNetworkRejectsBroadAndNonlocalScans(t *testing.T) {
	for _, cidr := range []string{"0.0.0.0/0", "10.0.0.0/8", "192.168.0.0/16", "192.168.1.0/23", "192.168.1.0/31", "224.0.0.0/24", "8.8.8.0/24", "fd00::/120", "invalid"} {
		if _, err := discoveryNetwork(cidr); err == nil {
			t.Errorf("accepted unsafe scan %q", cidr)
		}
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	var hosts []net.IP
	for _, it := range interfaces {
		addresses, err := it.Addrs()
		if err != nil {
			t.Fatal(err)
		}
		for _, address := range addresses {
			ip, _, _ := net.ParseCIDR(address.String())
			hosts = append(hosts, ip)
		}
	}
	for _, cidr := range []string{"10.254.231.0/24", "172.30.239.0/24", "192.168.253.0/24"} {
		_, n, _ := net.ParseCIDR(cidr)
		local := false
		for _, host := range hosts {
			local = local || n.Contains(host)
		}
		if !local {
			if _, err := discoveryNetwork(cidr); err == nil {
				t.Fatalf("accepted nonlocal private network %q", cidr)
			}
			return
		}
	}
	t.Fatal("could not construct a nonlocal private subnet for this test host")
}

func TestScheduledMinuteDayBoundariesAndTolerance(t *testing.T) {
	zone := time.FixedZone("NAS", 2*60*60)
	saturday := time.Date(2026, 10, 3, 23, 59, 0, 0, zone)
	plan := Schedule{Time: "23:59", Days: []int{int(time.Saturday)}}
	for _, delta := range []time.Duration{0, time.Minute, 2 * time.Minute} {
		minute, ok := scheduledMinute(plan, saturday.Add(delta))
		if !ok || minute != "2026-10-03T23:59" {
			t.Errorf("eligible run at delay %v failed: %q %v", delta, minute, ok)
		}
	}
	for _, now := range []time.Time{saturday.Add(-time.Minute), saturday.Add(3 * time.Minute), saturday.AddDate(0, 0, 1)} {
		if _, ok := scheduledMinute(plan, now); ok {
			t.Errorf("accepted execution outside scheduled day/time: %s", now)
		}
	}
}

func TestCallbackValidationRejectsExternalDestinations(t *testing.T) {
	valid := "https://127.0.0.1:5001/webman/3rdparty/h5uSynoWake/api.cgi"
	if _, err := validatedCallback(valid); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"https://example.com:5001/webman/3rdparty/h5uSynoWake/api.cgi",
		"https://localhost:5001/webman/3rdparty/h5uSynoWake/api.cgi",
		"https://127.0.0.1/webman/3rdparty/h5uSynoWake/api.cgi",
		"https://127.0.0.1:0/webman/3rdparty/h5uSynoWake/api.cgi",
		"https://127.0.0.1:65536/webman/3rdparty/h5uSynoWake/api.cgi",
		"https://user:pass@127.0.0.1:5001/webman/3rdparty/h5uSynoWake/api.cgi",
		"file:///webman/3rdparty/h5uSynoWake/api.cgi",
		"https://127.0.0.1:5001/webman/3rdparty/SynoWake/api.cgi",
		"https://127.0.0.1:5001/webman/3rdparty/Other/api.cgi",
		valid + "?action=evil",
		valid + "#fragment",
	} {
		if _, err := validatedCallback(value); err == nil {
			t.Errorf("accepted unsafe callback %q", value)
		}
	}
}
