package synowake

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

func runCommand(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if err != nil {
		return string(data), fmt.Errorf("%s: %w", path, err)
	}
	return string(data), nil
}
func ping(ip string) (bool, error) {
	path, err := exec.LookPath("ping")
	if err != nil {
		return false, errors.New("ping ist nicht verfügbar")
	}
	output, err := runCommand(path, "-n", "-c", "1", "-W", "1", ip)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return pingExitResult(output, exit.ExitCode())
	}
	return false, errors.New("ICMP-Prüfung konnte nicht ausgeführt werden.")
}

func pingExitResult(output string, code int) (bool, error) {
	if code == 0 {
		return true, nil
	}
	lower := strings.ToLower(output)
	for _, problem := range []string{"not permitted", "permission denied", "not allowed", "invalid option", "unrecognized option", "usage:"} {
		if strings.Contains(lower, problem) {
			return false, errors.New("ICMP-Ping ist für das Paketkonto nicht verfügbar.")
		}
	}
	if code == 1 {
		return false, nil
	}
	return false, errors.New("ICMP-Prüfung konnte nicht ausgeführt werden.")
}
func wake(d Device) error {
	packet, err := magicPacket(d.MAC)
	if err != nil {
		return err
	}
	return sendMagic(d.Broadcast, d.Port, packet)
}

type FoundDevice struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Type      string `json:"type"`
	Broadcast string `json:"broadcast,omitempty"`
}

func discoveryName(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	for _, suffix := range []string{".fritz.box", ".local", ".lan"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) && len(name) > len(suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func discoveryNetwork(cidr string) (*net.IPNet, error) {
	networks, err := localNetworks()
	if err != nil {
		return nil, err
	}
	return discoveryNetworkFor(cidr, networks)
}
func discover(cidr string) ([]FoundDevice, []string, error) {
	local, err := localNetworks()
	if err != nil {
		return nil, nil, err
	}
	network, err := discoveryNetworkFor(cidr, local)
	if err != nil {
		return nil, nil, err
	}
	broadcast := ""
	for _, attached := range local {
		_, subnet, err := net.ParseCIDR(attached.CIDR)
		if err == nil && subnet.Contains(network.IP) && subnet.Contains(networkEnd(network)) {
			broadcast = attached.Broadcast
			break
		}
	}
	jobs := make(chan string)
	var wg sync.WaitGroup
	var probeMu sync.Mutex
	var usedTCP bool
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				_, method, _ := probeHost(ip)
				if method == "tcp" {
					probeMu.Lock()
					usedTCP = true
					probeMu.Unlock()
				}
			}
		}()
	}
	base := network.IP.To4()
	bits, _ := network.Mask.Size()
	count := 1 << (32 - bits)
	for i := 1; i < count-1; i++ {
		ip := append(net.IP(nil), base...)
		ip[3] += byte(i)
		jobs <- ip.String()
	}
	close(jobs)
	wg.Wait()
	neighbors := map[string]string{}
	file, err := os.Open("/proc/net/arp")
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 6 && fields[2] != "0x0" {
				ip := net.ParseIP(fields[0])
				mac, e := net.ParseMAC(fields[3])
				if ip != nil && network.Contains(ip) && e == nil && len(mac) == 6 && mac[0]&1 == 0 && fields[3] != "00:00:00:00:00:00" {
					neighbors[ip.String()] = strings.ToUpper(mac.String())
				}
			}
		}
		file.Close()
	}
	if path, e := exec.LookPath("ip"); e == nil {
		out, _ := runCommand(path, "-4", "neigh", "show")
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			ip := net.ParseIP(fields[0])
			if ip == nil || !network.Contains(ip) {
				continue
			}
			for i, f := range fields {
				if f == "lladdr" && i+1 < len(fields) {
					mac, e := net.ParseMAC(fields[i+1])
					if e == nil && len(mac) == 6 && mac[0]&1 == 0 {
						neighbors[ip.String()] = strings.ToUpper(mac.String())
					}
				}
			}
		}
	}
	warnings := []string{"Die Suche findet IPv4-Nachbarn im lokalen LAN. Schlafende Geräte, VLANs und Geräte ohne ARP-Eintrag können fehlen. Namen stammen aus DNS und bleiben bearbeitbar."}
	if usedTCP {
		warnings = append(warnings, "ICMP ist für das Paketkonto nicht verfügbar. SynoWake verwendet eine TCP-Erreichbarkeitsprüfung. Vollständig gefilterte Geräte können fehlen.")
	}
	results := make([]FoundDevice, 0, len(neighbors))
	sem := make(chan struct{}, 16)
	var mu sync.Mutex
	for ip, mac := range neighbors {
		ip, mac := ip, mac
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			name := fmt.Sprintf("LAN-Gerät (%s)", ip)
			ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
			defer cancel()
			names, _ := net.DefaultResolver.LookupAddr(ctx, ip)
			if len(names) > 0 {
				if resolved := discoveryName(names[0]); resolved != "" {
					name = resolved
				}
			}
			mu.Lock()
			results = append(results, FoundDevice{Name: name, IP: ip, MAC: mac, Type: "LAN-Gerät", Broadcast: broadcast})
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].IP < results[j].IP })
	return results, warnings, nil
}

func (a *App) event(l Log, notify bool) error {
	if l.ID == "" {
		l.ID = randomID()
	}
	if l.Time.IsZero() {
		l.Time = time.Now().UTC()
	}
	l.CenterPending = !a.Demo
	if err := a.update(func(s *Store) error {
		for i, old := range s.Logs {
			if old.ID == l.ID {
				s.Logs[i] = l
				return nil
			}
		}
		addLog(s, l)
		return nil
	}); err != nil {
		return err
	}
	if a.Demo {
		return nil
	}
	if err := a.deliverCenter(l); err != nil {
		return err
	}
	if notify {
		// DSM 7 desktop notifications use package i18n keys and positional
		// substitutions. This integration does not require a sysnotify worker.
		_, err := runCommand("/usr/syno/bin/synodsmnotify", notificationArguments(l)...)
		msg := ""
		if err != nil {
			msg = "DSM-Benachrichtigung fehlgeschlagen: " + err.Error()
		}
		if err := a.update(func(s *Store) error {
			setDiagnostic(s, "notification", msg)
			if msg != "" {
				addLog(s, Log{Level: "warning", Source: "integration", Message: msg})
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// Keep failed Log Center deliveries in the application log for explicit retry.
func (a *App) deliverCenter(l Log) error {
	s, err := a.readStore()
	if err != nil {
		return err
	}
	level := "info"
	severity := 6
	if l.Level == "error" {
		level = "err"
		severity = 3
	}
	if l.Level == "warning" {
		level = "warn"
		severity = 4
	}
	localizedMessage := logMessage(l, systemLanguage())
	_, nativeErr := runCommand("/usr/syno/bin/synologset1", "sys", level, "0x11100000", "SynoWake: "+localizedMessage)
	deliveryErr := nativeErr
	if deliveryErr != nil && s.LogCenterPort > 0 {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.LogCenterPort), 2*time.Second)
		if e == nil {
			c.SetWriteDeadline(time.Now().Add(2 * time.Second))
			host, _ := os.Hostname()
			message := strings.NewReplacer("\r", " ", "\n", " ").Replace(localizedMessage)
			when := l.Time
			if when.IsZero() {
				when = time.Now()
			}
			_, e = fmt.Fprintf(c, "<%d>%s %s SynoWake: %s\n", 8+severity, when.Local().Format("Jan _2 15:04:05"), host, message)
			c.Close()
		}
		deliveryErr = e
	}
	message := ""
	if deliveryErr != nil {
		message = "Protokoll-Center-Übertragung ausstehend. Lokalen TCP-Empfänger einrichten oder DSM-Berechtigungen prüfen: " + deliveryErr.Error()
		if path, e := exec.LookPath("logger"); e == nil {
			runCommand(path, "-t", "SynoWake", localizedMessage)
		}
	}
	return a.update(func(s *Store) error {
		pending := false
		for i, item := range s.Logs {
			if item.ID == l.ID {
				s.Logs[i].CenterPending = deliveryErr != nil
			}
			if s.Logs[i].CenterPending {
				pending = true
			}
		}
		if !pending {
			message = ""
		}
		if pending && message == "" {
			message = "Einige Protokoll-Center-Einträge warten auf erneute Übertragung."
		}
		setDiagnostic(s, "log-center", message)
		return nil
	})
}
func (a *App) retryCenter() (map[string]int, error) {
	s, err := a.readStore()
	if err != nil {
		return nil, err
	}
	logs := []Log{}
	for _, l := range s.Logs {
		if l.CenterPending && len(logs) < 20 {
			logs = append(logs, l)
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	var mu sync.Mutex
	var updateErr error
	for _, l := range logs {
		l := l
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if e := a.deliverCenter(l); e != nil {
				mu.Lock()
				updateErr = e
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	s, err = a.readStore()
	if err != nil {
		return nil, err
	}
	remaining := 0
	for _, l := range s.Logs {
		if l.CenterPending {
			remaining++
		}
	}
	return map[string]int{"attempted": len(logs), "remaining": remaining}, updateErr
}
