package synowake

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestPingPermissionErrorsAreNotReportedAsOffline(t *testing.T) {
	for _, tc := range []struct {
		output              string
		code                int
		online, unavailable bool
	}{
		{"1 packets received", 0, true, false},
		{"100% packet loss", 1, false, false},
		{"ping: socket: Operation not permitted", 1, false, true},
		{"ping: Permission denied", 2, false, true},
		{"usage: ping [-c count]", 1, false, true},
		{"failed", 2, false, true},
	} {
		online, err := pingExitResult(tc.output, tc.code)
		if online != tc.online || (err != nil) != tc.unavailable {
			t.Errorf("%q exit %d: online=%v, err=%v", tc.output, tc.code, online, err)
		}
	}
}

func TestReachabilityKeepsICMPResultsWhenAvailable(t *testing.T) {
	for _, expected := range []bool{true, false} {
		online, method, err := probeHostWith("192.168.1.20", func(string) (bool, error) { return expected, nil }, func(string) (bool, error) { t.Fatal("TCP called despite available ICMP"); return false, nil })
		if online != expected || method != "icmp" || err != nil {
			t.Fatalf("wrong ICMP result: %v %s %v", online, method, err)
		}
	}
}

func TestReachabilityFallsBackWithoutCallingUnresponsiveHostsOffline(t *testing.T) {
	blocked := func(string) (bool, error) { return false, errors.New("Operation not permitted") }
	online, method, err := probeHostWith("192.168.1.20", blocked, func(string) (bool, error) { return true, nil })
	if !online || method != "tcp" || err != nil {
		t.Fatalf("TCP response lost: %v %s %v", online, method, err)
	}
	unknown := errors.New("no TCP response")
	online, method, err = probeHostWith("192.168.1.20", blocked, func(string) (bool, error) { return false, unknown })
	if online || method != "tcp" || !errors.Is(err, unknown) {
		t.Fatalf("no response must remain unknown: %v %s %v", online, method, err)
	}
}

func TestTCPReachabilityRecognizesListeningAndRefusedConnections(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	if online, err := tcpReachable("127.0.0.1", []int{port}, time.Second); !online || err != nil {
		t.Fatalf("open port: %v %v", online, err)
	}
	listener.Close()
	if online, err := tcpReachable("127.0.0.1", []int{port}, time.Second); !online || err != nil {
		t.Fatalf("refused connection still proves response: %v %v", online, err)
	}
	if online, err := tcpReachable("invalid", []int{port}, time.Second); online || err == nil {
		t.Fatal("invalid IP accepted")
	}
}
