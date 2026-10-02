package synowake

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultSocket = "/var/packages/SynoWake/target/run/backend.sock"
const gatewayHeader = "X-SynoWake-Gateway"

type gatewayContextKey struct{}
type gatewayContext struct {
	ClientAddress string `json:"clientAddress"`
	ServerAddress string `json:"serverAddress"`
	ServerPort    string `json:"serverPort"`
	Scheme        string `json:"scheme"`
}

func socketPath() string {
	if value := os.Getenv("SYNOWAKE_SOCKET"); value != "" {
		return value
	}
	return defaultSocket
}
func unixClient(socket string, timeout time.Duration) (*http.Client, func()) {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return client, transport.CloseIdleConnections
}
func metadataFor(r *http.Request) gatewayContext {
	if value, ok := r.Context().Value(gatewayContextKey{}).(gatewayContext); ok {
		return value
	}
	scheme := r.URL.Scheme
	if scheme != "https" {
		scheme = "http"
	}
	if r.TLS != nil || os.Getenv("HTTPS") == "on" {
		scheme = "https"
	}
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		_, port, _ = net.SplitHostPort(r.Host)
	}
	if port == "5001" {
		scheme = "https"
	}
	if origin, err := urlScheme(r.Header.Get("Origin")); err == nil && origin == "https" {
		scheme = "https"
	}
	if port == "" {
		if scheme == "https" {
			port = "5001"
		} else {
			port = "5000"
		}
	}
	return gatewayContext{r.RemoteAddr, os.Getenv("SERVER_ADDR"), port, scheme}
}
func urlScheme(value string) (string, error) {
	if strings.HasPrefix(value, "https://") {
		return "https", nil
	}
	if strings.HasPrefix(value, "http://") {
		return "http", nil
	}
	return "", errors.New("no origin scheme")
}
func validGatewayMetadata(m gatewayContext) bool {
	addr := m.ClientAddress
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if net.ParseIP(strings.Trim(addr, "[]")) == nil {
		return false
	}
	if m.ServerAddress != "" && net.ParseIP(m.ServerAddress) == nil {
		return false
	}
	port, err := strconv.Atoi(m.ServerPort)
	return err == nil && port >= 1 && port <= 65535 && (m.Scheme == "http" || m.Scheme == "https")
}

// DSM executes this CGI with its ordinary identity. The gateway only relays
// requests to a local socket; it cannot read or write private package state.
func newCGIProxy(socket string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			writeJSON(w, 405, nil, errors.New("HTTP-Methode nicht zulässig."))
			return
		}
		if r.ContentLength > 128<<10 {
			writeJSON(w, 413, nil, errors.New("Anfrage zu groß."))
			return
		}
		meta := metadataFor(r)
		if !validGatewayMetadata(meta) {
			writeJSON(w, 502, nil, errors.New("DSM-CGI hat keinen gültigen Anfragekontext bereitgestellt."))
			return
		}
		payload, _ := json.Marshal(meta)
		var body io.Reader
		if r.Body != nil {
			body = io.LimitReader(r.Body, (128<<10)+1)
		}
		request, err := http.NewRequestWithContext(r.Context(), r.Method, "http://synowake/api.cgi?"+r.URL.RawQuery, body)
		if err != nil {
			writeJSON(w, 502, nil, err)
			return
		}
		request.Host = r.Host
		for _, header := range []string{"Cookie", "Content-Type", "X-SYNO-TOKEN", "X-SynoWake-CSRF", "X-SynoWake-Task", "Origin", "Sec-Fetch-Site"} {
			if value := r.Header.Get(header); value != "" {
				request.Header.Set(header, value)
			}
		}
		// Replace any browser-supplied forwarding context with CGI's own values.
		request.Header.Set(gatewayHeader, base64.RawURLEncoding.EncodeToString(payload))
		client, closeClient := unixClient(socket, 90*time.Second)
		defer closeClient()
		response, err := client.Do(request)
		if err != nil {
			writeJSON(w, 503, nil, errors.New("SynoWake-Dienst ist nicht erreichbar. Paket starten; bei Startfehlern service.log prüfen."))
			return
		}
		defer response.Body.Close()
		for _, header := range []string{"Content-Type", "Cache-Control", "X-Content-Type-Options"} {
			if value := response.Header.Get(header); value != "" {
				w.Header().Set(header, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	})
}
func (a *App) backendHandler(shutdown func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/health":
			if r.Method != "GET" {
				writeJSON(w, 405, nil, errors.New("Nur GET."))
				return
			}
			s, err := a.readStore()
			if err != nil {
				writeJSON(w, 500, nil, err)
				return
			}
			writeJSON(w, 200, map[string]any{"active": s.Active, "version": PackageVersion}, nil)
			return
		case "/internal/stop":
			s, err := a.readStore()
			if err != nil {
				writeJSON(w, 500, nil, err)
				return
			}
			if r.Method != "POST" || !hmac.Equal([]byte(r.Header.Get("X-SynoWake-Control")), []byte(s.CSRFSecret)) {
				writeJSON(w, 403, nil, errors.New("Ungültiger Paket-Steueraufruf."))
				return
			}
			writeJSON(w, 200, map[string]bool{"stopped": true}, nil)
			shutdown()
			return
		case "/api.cgi":
			encoded := r.Header.Get(gatewayHeader)
			if len(encoded) > 2048 {
				writeJSON(w, 400, nil, errors.New("Anfragekontext zu groß."))
				return
			}
			data, err := base64.RawURLEncoding.DecodeString(encoded)
			var meta gatewayContext
			if err != nil || json.Unmarshal(data, &meta) != nil || !validGatewayMetadata(meta) {
				writeJSON(w, 403, nil, errors.New("Gültiger DSM-CGI-Kontext erforderlich."))
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), gatewayContextKey{}, meta))
			r.RemoteAddr = meta.ClientAddress
			r.URL.Scheme = meta.Scheme
			r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
			a.ServeHTTP(w, r)
			return
		default:
			writeJSON(w, 404, nil, errors.New("Unbekannter Dienstpfad."))
		}
	})
}
func backendHealth(socket string) (bool, error) {
	client, closeClient := unixClient(socket, 3*time.Second)
	defer closeClient()
	response, err := client.Get("http://synowake/internal/health")
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Active  bool   `json:"active"`
			Version string `json:"version"`
		} `json:"data"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result)
	if err != nil || response.StatusCode != 200 || !result.Success || result.Data.Version != PackageVersion {
		return false, errors.New("Ungültige Dienstantwort oder veralteter Dienst.")
	}
	return result.Data.Active, nil
}
func (a *App) stopBackend(socket string) error {
	if err := a.update(func(s *Store) error { s.Active = false; return nil }); err != nil {
		return err
	}
	state, err := a.readStore()
	if err != nil {
		return err
	}
	request, _ := http.NewRequest("POST", "http://synowake/internal/stop", nil)
	request.Header.Set("X-SynoWake-Control", state.CSRFSecret)
	client, closeClient := unixClient(socket, 5*time.Second)
	defer closeClient()
	response, err := client.Do(request)
	if err != nil {
		// Stopping an already stopped/missing service is safe and idempotent.
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil
		}
		return err
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("SynoWake-Dienst konnte nicht angehalten werden.")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := backendHealth(socket); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("SynoWake-Dienst beendet sich noch; service.log prüfen.")
}
func (a *App) serveBackend(socket string) error {
	if !filepath.IsAbs(socket) {
		return errors.New("Dienst-Socket benötigt einen absoluten Pfad.")
	}
	if err := a.update(func(s *Store) error { return nil }); err != nil {
		return err
	}
	guardDir := filepath.Join(a.Root, "service-lock")
	if err := os.MkdirAll(guardDir, 0700); err != nil {
		return err
	}
	unlock, err := lockStore(guardDir)
	if err != nil {
		return fmt.Errorf("Dienst läuft bereits: %w", err)
	}
	defer unlock()
	if err := os.MkdirAll(filepath.Dir(socket), 0755); err != nil {
		return err
	}
	if info, e := os.Lstat(socket); e == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("Socket-Pfad ist keine Socket-Datei; wird nicht überschrieben.")
		}
		if _, e := backendHealth(socket); e == nil {
			return errors.New("Dienst läuft bereits.")
		}
		if e := os.Remove(socket); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(socket, 0666); err != nil {
			return err
		}
	}
	signalContext, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	ctx, cancel := context.WithCancel(signalContext)
	defer cancel()
	server := &http.Server{Handler: a.backendHandler(cancel), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		a.update(func(s *Store) error { s.Active = false; return nil })
		shutdownContext, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			server.Close()
		}
	}()
	err = server.Serve(listener)
	cancel()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func authenticationEnvironment(r *http.Request) []string {
	meta := metadataFor(r)
	values := map[string]string{
		"GATEWAY_INTERFACE": "CGI/1.1", "HTTP_COOKIE": r.Header.Get("Cookie"), "REMOTE_ADDR": meta.ClientAddress, "SERVER_ADDR": meta.ServerAddress,
		"SERVER_PORT": meta.ServerPort, "HTTP_HOST": r.Host, "HTTP_X_SYNO_TOKEN": r.Header.Get("X-SYNO-TOKEN"),
		"REQUEST_METHOD": r.Method, "QUERY_STRING": r.URL.RawQuery, "SCRIPT_NAME": "/webman/3rdparty/SynoWake/api.cgi",
		"REQUEST_URI": "/webman/3rdparty/SynoWake/api.cgi?" + r.URL.RawQuery, "SERVER_PROTOCOL": r.Proto, "HTTPS": "off",
	}
	if host, _, err := net.SplitHostPort(meta.ClientAddress); err == nil {
		values["REMOTE_ADDR"] = host
	}
	if meta.Scheme == "https" {
		values["HTTPS"] = "on"
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; replace || strings.HasPrefix(key, "HTTP_") {
			continue
		}
		environment = append(environment, entry)
	}
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	return environment
}
func runAuthenticationCommand(path string, r *http.Request) (string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path)
	command.Env = authenticationEnvironment(r)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
