package synowake

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebPortsDefaultsValidationAndPersistence(t *testing.T) {
	for _, text := range []string{"", "  "} {
		ports, err := parseWebPorts(text)
		if err != nil || !reflect.DeepEqual(ports, []int{5000, 80, 8080}) {
			t.Fatalf("default ports: %v, %v", ports, err)
		}
	}
	for _, text := range []string{"0", "65536", "-1", "80.0", "+80", "80,,8080", "http://localhost:80", "80/", strings.Repeat("80,", 16) + "80"} {
		if _, err := parseWebPorts(text); err == nil {
			t.Fatalf("invalid ports accepted: %q", text)
		}
	}
	a := testApp(t)
	d := testDevice(t, a) // An older record with no webPorts uses the defaults.
	d.WebPorts = " 8080, 80, 8080, 5001 "
	result := testServe(t, a, testRequest(t, a, "POST", "device-save", d), 200)
	var saved Device
	if err := json.Unmarshal(result.Data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.WebPorts != "8080, 80, 5001" || testStore(t, a).Devices[0].WebPorts != saved.WebPorts {
		t.Fatal("custom port order, deduplication or persistence lost")
	}
	testServe(t, a, testRequest(t, a, "POST", "device-favorite", map[string]any{"id": d.ID, "favorite": true}), 200)
	if testStore(t, a).Devices[0].WebPorts != saved.WebPorts {
		t.Fatal("favorite update lost custom web ports")
	}
	before := testStore(t, a).Devices[0]
	d.WebPorts = "80,70000"
	testServe(t, a, testRequest(t, a, "POST", "device-save", d), 400)
	if testStore(t, a).Devices[0] != before {
		t.Fatal("invalid port edit changed the stored device")
	}
	d.WebPorts = ""
	testServe(t, a, testRequest(t, a, "POST", "device-save", d), 200)
	if testStore(t, a).Devices[0].WebPorts != "" {
		t.Fatal("clearing the field must restore default ports")
	}
}

func TestWebPagesAPIUsesSavedDeviceAndSessionProtection(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	request := testRequest(t, a, "POST", "device-pages", map[string]string{"id": d.ID})
	request.Header.Del("X-SynoWake-CSRF")
	testServe(t, a, request, 403)
	testServe(t, a, testRequest(t, a, "GET", "device-pages", nil), 404)
	testServe(t, a, testRequest(t, a, "POST", "device-pages", map[string]string{"id": d.ID, "url": "http://example.com"}), 400)
	testServe(t, a, testRequest(t, a, "POST", "device-pages", map[string]string{"id": randomID()}), 400)
	result := testServe(t, a, testRequest(t, a, "POST", "device-pages", map[string]string{"id": d.ID}), 200)
	var data struct {
		Ports []int     `json:"ports"`
		Pages []WebPage `json:"pages"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.Ports, []int{5000, 80, 8080}) || len(data.Pages) != 3 || data.Pages[0].URL != "http://192.168.1.20:5000/" {
		t.Fatalf("wrong demo default pages: %+v", data)
	}
	local, _ := parseLocalNetwork("LAN", "192.167.178.21/24")
	if _, err := deviceWebAddress(Device{IP: "192.167.178.20"}, []LocalNetwork{local}); err != nil {
		t.Fatal("directly connected nonprivate NAS network rejected", err)
	}
	for _, address := range []string{"8.8.8.8", "127.0.0.1", "169.254.169.254", "example.com", "192.168.1.20:5000"} {
		if _, err := deviceWebAddress(Device{IP: address}, nil); err == nil {
			t.Fatalf("external, loopback or malformed target accepted: %s", address)
		}
	}
}

func serverWebPort(t *testing.T, server *httptest.Server) int {
	t.Helper()
	_, text, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(text)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestWebProbeDetectsHTTPAndSelfSignedHTTPSWithoutCredentialsOrRedirects(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer destination.Close()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/" || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("X-SYNO-TOKEN") != "" || r.Header.Get("X-SYNO-HASH") != "" {
			t.Error("probe transmitted unexpected method, path, query or credentials")
		}
		w.Header().Set("Location", destination.URL)
		w.WriteHeader(http.StatusFound)
	})
	plain := httptest.NewServer(handler)
	defer plain.Close()
	secure := httptest.NewUnstartedServer(handler)
	secure.Config.ErrorLog = log.New(io.Discard, "", 0)
	secure.StartTLS()
	defer secure.Close()
	plainPort, securePort := serverWebPort(t, plain), serverWebPort(t, secure)
	pages := probeWebPages(context.Background(), "127.0.0.1", []int{plainPort, securePort}, 2*time.Second)
	wanted := []WebPage{{plainPort, webURL("127.0.0.1", plainPort, "http")}, {securePort, webURL("127.0.0.1", securePort, "https")}}
	if !reflect.DeepEqual(pages, wanted) || redirected.Load() != 0 {
		t.Fatalf("protocol detection/order failed or redirect followed: %+v; visits %d", pages, redirected.Load())
	}
}

func TestWebProbeDoesNotExposeClosedOrNonHTTPPortsAndRespectsDeadline(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	started := time.Now()
	pages := probeWebPages(context.Background(), "127.0.0.1", []int{closedPort, serverWebPort(t, slow)}, 100*time.Millisecond)
	if len(pages) != 0 || time.Since(started) > time.Second {
		t.Fatalf("closed/slow service exposed or deadline exceeded: %+v", pages)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pages := probeWebPages(ctx, "127.0.0.1", []int{serverWebPort(t, slow)}, time.Second); len(pages) != 0 {
		t.Fatal("canceled request produced a link")
	}
}

func TestWebProbeRecognizesDelayedDSMResponse(t *testing.T) {
	// DSM may generate the login page more slowly than the former 800 ms header
	// deadline. A port must not vanish just because the first page takes longer.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(1200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	port := serverWebPort(t, slow)
	pages := probeWebPages(context.Background(), "127.0.0.1", []int{port}, webProbeTimeout)
	if !reflect.DeepEqual(pages, []WebPage{{port, webURL("127.0.0.1", port, "http")}}) {
		t.Fatalf("delayed DSM login response was missed: %+v", pages)
	}
}
