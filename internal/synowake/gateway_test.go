package synowake

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitBackend(t *testing.T, socket string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := backendHealth(socket); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("package service did not become ready")
}
func testBackend(t *testing.T) (*App, string) {
	t.Helper()
	root, err := os.MkdirTemp("", "sw-gateway-")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Root: filepath.Join(root, "private"), Demo: true}
	if err := app.update(func(s *Store) error { s.Active = true; return nil }); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(root, "run", "backend.sock")
	done := make(chan error, 1)
	go func() { done <- app.serveBackend(socket) }()
	t.Cleanup(func() {
		if err := app.stopBackend(socket); err != nil {
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(6 * time.Second):
			t.Error("service did not stop")
		}
		os.RemoveAll(root)
	})
	waitBackend(t, socket)
	return app, socket
}
func proxyRequest(t *testing.T, proxy http.Handler, method, action string, payload any, csrf string, remote string) testResponse {
	t.Helper()
	body, _ := json.Marshal(payload)
	r := httptest.NewRequest(method, "https://nas.example:5001/webman/3rdparty/SynoWake/api.cgi?action="+action, bytes.NewReader(body))
	r.Header.Set("Cookie", "id=proxy-session")
	r.Header.Set("X-SYNO-TOKEN", "session-token")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-SynoWake-CSRF", csrf)
	r.RemoteAddr = remote
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, r)
	var result testResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestCGIGatewayRelaysPrivateStateWithoutReadingIt(t *testing.T) {
	app, socket := testBackend(t)
	proxy := newCGIProxy(socket)
	t.Setenv("SYNOWAKE_VAR", filepath.Join(t.TempDir(), "must-not-be-created"))
	result := proxyRequest(t, proxy, "GET", "state", nil, "", "192.168.1.10:12345")
	if !result.Success {
		t.Fatal(result.Error)
	}
	var state struct {
		CSRF    string `json:"csrf"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result.Data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Version != PackageVersion || len(state.CSRF) != 64 {
		t.Fatalf("invalid state: %s", result.Data)
	}
	result = proxyRequest(t, proxy, "POST", "device-save", map[string]any{"name": "Gateway-PC", "ip": "192.168.1.20", "mac": "02:11:22:33:44:55"}, state.CSRF, "192.168.1.10:12345")
	if !result.Success {
		t.Fatal(result.Error)
	}
	var device Device
	json.Unmarshal(result.Data, &device)
	result = proxyRequest(t, proxy, "POST", "wake", map[string]any{"ids": []string{device.ID}, "notify": false}, state.CSRF, "192.168.1.10:12345")
	if !result.Success {
		t.Fatal(result.Error)
	}
	saved := testStore(t, app)
	if len(saved.Devices) != 1 || saved.Devices[0].LastWake.IsZero() || len(saved.Logs) != 1 {
		t.Fatalf("gateway did not execute through service: %+v", saved)
	}
	if _, err := os.Stat(os.Getenv("SYNOWAKE_VAR")); !os.IsNotExist(err) {
		t.Fatal("gateway accessed a caller-selected data root")
	}
}
func TestCGIGatewayReplacesBrowserContextAndKeepsRemoteScheduleGuard(t *testing.T) {
	app, socket := testBackend(t)
	p := testRunnableSchedule(t, app)
	proxy := newCGIProxy(socket)
	body, _ := json.Marshal(map[string]string{"id": p.ID})
	r := httptest.NewRequest("POST", "http://nas.example:5000/api.cgi?action=scheduled-run", bytes.NewReader(body))
	r.RemoteAddr = "192.168.1.10:4444"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-SynoWake-Task", p.Secret)
	forged, _ := json.Marshal(gatewayContext{"127.0.0.1", "127.0.0.1", "5000", "http"})
	r.Header.Set(gatewayHeader, base64.RawURLEncoding.EncodeToString(forged))
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("forged remote context accepted: %d %s", w.Code, w.Body.String())
	}
	if len(testStore(t, app).Logs) != 0 {
		t.Fatal("remote request triggered wake")
	}
	r = httptest.NewRequest("POST", "http://nas.example:5000/api.cgi?action=scheduled-run", bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:4444"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-SynoWake-Task", p.Secret)
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("local scheduled request failed: %d %s", w.Code, w.Body.String())
	}
}
func TestBackendControlRequiresPrivateSecretAndGatewayCannotReachIt(t *testing.T) {
	app, socket := testBackend(t)
	client, closeClient := unixClient(socket, 3*time.Second)
	defer closeClient()
	req, _ := http.NewRequest("POST", "http://synowake/internal/stop", nil)
	req.Header.Set("X-SynoWake-Control", "wrong")
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("untrusted local process stopped package service")
	}
	r := httptest.NewRequest("POST", "http://nas.example:5000/internal/stop", strings.NewReader("{}"))
	r.RemoteAddr = "127.0.0.1:4444"
	r.Header.Set("X-SynoWake-Control", testStore(t, app).CSRFSecret)
	w := httptest.NewRecorder()
	newCGIProxy(socket).ServeHTTP(w, r)
	if active, err := backendHealth(socket); err != nil || !active {
		t.Fatal("gateway reached internal control route")
	}
}
func TestBackendRejectsMissingContextAndUnsafeSocketPath(t *testing.T) {
	app, socket := testBackend(t)
	client, closeClient := unixClient(socket, 3*time.Second)
	defer closeClient()
	response, err := client.Get("http://synowake/api.cgi?action=state")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("direct API call without CGI context accepted")
	}
	root, err := os.MkdirTemp("", "sw-safety-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	regular := filepath.Join(root, "not-a-socket")
	os.WriteFile(regular, []byte("preserve"), 0600)
	if err := (&App{Root: filepath.Join(root, "private")}).serveBackend(regular); err == nil {
		t.Fatal("service overwrote regular socket-path file")
	}
	data, _ := os.ReadFile(regular)
	if string(data) != "preserve" {
		t.Fatal("regular file was changed")
	}
	_ = app
}
func TestAuthenticationEnvironmentIsolatedAcrossConcurrentRequests(t *testing.T) {
	t.Setenv("HTTP_COOKIE", "stale-parent-cookie")
	t.Setenv("HTTP_X_SYNO_TOKEN", "stale-parent-token")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "https://nas.example:5443/api.cgi?action=state", nil)
			cookie := "session-" + string(rune('a'+i))
			token := "token-" + string(rune('a'+i))
			r.Header.Set("Cookie", cookie)
			r.Header.Set("X-SYNO-TOKEN", token)
			r = r.WithContext(context.WithValue(r.Context(), gatewayContextKey{}, gatewayContext{"192.168.1.10:3333", "192.168.1.1", "5443", "https"}))
			values := map[string]string{}
			for _, entry := range authenticationEnvironment(r) {
				key, value, _ := strings.Cut(entry, "=")
				if _, ok := values[key]; ok {
					t.Errorf("duplicate environment field %s", key)
				}
				values[key] = value
			}
			if values["HTTP_COOKIE"] != cookie || values["HTTP_X_SYNO_TOKEN"] != token || values["REMOTE_ADDR"] != "192.168.1.10" || values["HTTPS"] != "on" || values["SERVER_PORT"] != "5443" {
				t.Errorf("request environment mixed: %v", values)
			}
			if callbackFor(r) != "https://127.0.0.1:5443/webman/3rdparty/SynoWake/api.cgi" {
				t.Error("callback lost original DSM port")
			}
		}()
	}
	wg.Wait()
	if os.Getenv("HTTP_COOKIE") != "stale-parent-cookie" || os.Getenv("HTTP_X_SYNO_TOKEN") != "stale-parent-token" {
		t.Fatal("authentication modified process-wide session context")
	}
}
