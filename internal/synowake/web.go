package synowake

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WebPage struct {
	Port int    `json:"port"`
	URL  string `json:"url"`
}

const webProbeTimeout = 8 * time.Second

func parseWebPorts(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return []int{5000, 80, 8080}, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 16 {
		return nil, errors.New("Höchstens 16 Webports mit Komma getrennt eingeben.")
	}
	ports := []int{}
	seen := map[int]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 || len(part) > 5 || strings.IndexFunc(part, func(c rune) bool { return c < '0' || c > '9' }) != -1 {
			return nil, errors.New("Webports müssen Zahlen von 1 bis 65535 sein, mit Komma getrennt.")
		}
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("Webports müssen Zahlen von 1 bis 65535 sein, mit Komma getrennt.")
		}
		if !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports, nil
}

func formatWebPorts(ports []int) string {
	parts := make([]string, len(ports))
	for i, port := range ports {
		parts[i] = strconv.Itoa(port)
	}
	return strings.Join(parts, ", ")
}

func webSchemes(port int) []string {
	if port == 443 || port == 5001 || port == 8443 || port == 9443 {
		return []string{"https", "http"}
	}
	return []string{"http", "https"}
}

func webURL(ip string, port int, scheme string) string {
	return scheme + "://" + net.JoinHostPort(ip, strconv.Itoa(port)) + "/"
}

// Probe only HTTP headers, with no credentials, proxy or redirect following.
// The API caller validates the stored device IP against permitted LAN networks.
// Self-signed TLS is accepted for detection only; browser certificate checks
// remain in place when the user opens the link.
func probeWebPages(parent context.Context, ip string, ports []int, timeout time.Duration) []WebPage {
	pages := []WebPage{}
	if net.ParseIP(ip).To4() == nil || len(ports) > 16 {
		return pages
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true,
		DialContext:            (&net.Dialer{Timeout: time.Second}).DialContext,
		TLSClientConfig:        &tls.Config{InsecureSkipVerify: true}, // Detection only, never authenticated traffic.
		TLSHandshakeTimeout:    1500 * time.Millisecond,
		ResponseHeaderTimeout:  3 * time.Second,
		MaxResponseHeaderBytes: 32 << 10,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport, Timeout: 4 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	results := make([]*WebPage, len(ports))
	var pending sync.WaitGroup
	for index, port := range ports {
		if port < 1 || port > 65535 {
			continue
		}
		pending.Add(1)
		go func(index, port int) {
			defer pending.Done()
			for _, scheme := range webSchemes(port) {
				url := webURL(ip, port, scheme)
				request, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
				if err != nil {
					continue
				}
				request.Header.Set("User-Agent", "SynoWake/"+PackageVersion)
				response, err := client.Do(request)
				if response != nil {
					response.Body.Close()
				}
				if err == nil && response.StatusCode >= 200 && response.StatusCode <= 599 {
					results[index] = &WebPage{Port: port, URL: url}
					// A TLS listener may answer plain HTTP with 400. Check TLS
					// before retaining that HTTP fallback.
					if scheme != "http" || response.StatusCode != http.StatusBadRequest {
						return
					}
				}
			}
		}(index, port)
	}
	pending.Wait()
	for _, page := range results {
		if page != nil {
			pages = append(pages, *page)
		}
	}
	return pages
}

func deviceWebAddress(d Device, networks []LocalNetwork) (string, error) {
	ip := lanIPv4(d.IP)
	if ip == nil || (!ip.IsPrivate() && !isOnLinkHost(ip, networks)) {
		return "", errors.New("IP muss eine private oder direkt an der NAS angeschlossene IPv4-LAN-Adresse sein.")
	}
	return ip.String(), nil
}
