package synowake

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// A TCP connection or a refused connection proves a live IP response without
// raw sockets. Timeouts do not prove that a host is powered off.
func tcpReachable(ip string, ports []int, timeout time.Duration) (bool, error) {
	if net.ParseIP(ip).To4() == nil || len(ports) == 0 {
		return false, errors.New("Ungültige IPv4-Erreichbarkeitsprüfung.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := make(chan bool, len(ports))
	for _, port := range ports {
		port := port
		go func() {
			dialer := net.Dialer{}
			conn, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ip, strconv.Itoa(port)))
			if conn != nil {
				conn.Close()
			}
			// Winsock uses 10061 rather than Go's portable errno constant.
			refused := errors.Is(err, syscall.ECONNREFUSED) || (runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10061)))
			results <- err == nil || refused
		}()
	}
	for range ports {
		if <-results {
			return true, nil
		}
	}
	return false, errors.New("Keine TCP-Antwort. Der Status ist unbekannt; eine Firewall kann die Prüfung blockieren.")
}

func probeHostWith(ip string, icmp, tcp func(string) (bool, error)) (bool, string, error) {
	online, err := icmp(ip)
	if err == nil {
		return online, "icmp", nil
	}
	online, tcpErr := tcp(ip)
	if tcpErr != nil {
		return false, "tcp", fmt.Errorf("Erreichbarkeitsprüfung ohne ICMP: %w", tcpErr)
	}
	return online, "tcp", nil
}

func probeHost(ip string) (bool, string, error) {
	return probeHostWith(ip, ping, func(address string) (bool, error) {
		return tcpReachable(address, []int{445, 80, 443, 22, 3389, 5000}, 800*time.Millisecond)
	})
}
