package synowake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func editWakeLink(t *testing.T, a *App, id, operation string) string {
	t.Helper()
	r := testRequest(t, a, http.MethodPost, "device-wake-link", map[string]string{"id": id, "operation": operation})
	result := testServe(t, a, r, http.StatusOK)
	var data map[string]string
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatal(err)
	}
	return data["token"]
}

func wakeLinkRequest(method, query, remote string) *http.Request {
	r := httptest.NewRequest(method, "https://nas.example:5001/webman/3rdparty/h5uSynoWake/api.cgi?"+query, nil)
	r.RemoteAddr = remote
	return r
}

func TestWakeLinkLifecycleAndAuthorization(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	if editWakeLink(t, a, d.ID, "get") != "" {
		t.Fatal("wake links must be disabled by default")
	}
	r := testRequest(t, a, http.MethodPost, "device-wake-link", map[string]string{"id": d.ID, "operation": "create"})
	r.Header.Del("X-SynoWake-CSRF")
	testServe(t, a, r, http.StatusForbidden)
	token := editWakeLink(t, a, d.ID, "create")
	if len(token) != 22 || editWakeLink(t, a, d.ID, "create") != token || editWakeLink(t, a, d.ID, "get") != token {
		t.Fatal("creation must return a short, persistent link without silently rotating it")
	}
	state := testServe(t, a, testRequest(t, a, http.MethodGet, "state", nil), http.StatusOK)
	if strings.Contains(string(state.Data), token) {
		t.Fatal("wake token leaked into normal state")
	}
	local := "192.168.1.10:12345"
	for _, tc := range []struct{ query, remote, header, value string }{
		{"w=" + strings.Repeat("a", 22), local, "", ""},
		{"w=" + token + "&w=" + token, local, "", ""},
		{"w=" + token + "&action=state", local, "", ""},
		{"w=" + token, "10.0.0.10:12345", "X-Forwarded-For", "192.168.1.10"},
		{"w=" + token, "127.0.0.1:12345", "", ""},
		{"w=" + token, local, "Sec-Purpose", "prefetch"},
		{"w=" + token, local, "Sec-Fetch-Site", "cross-site"},
	} {
		r := wakeLinkRequest(http.MethodGet, tc.query, tc.remote)
		if tc.header != "" {
			r.Header.Set(tc.header, tc.value)
		}
		testServe(t, a, r, http.StatusForbidden)
	}
	testServe(t, a, wakeLinkRequest(http.MethodHead, "w="+token, local), http.StatusMethodNotAllowed)
	if len(testStore(t, a).Logs) != 0 {
		t.Fatal("rejected requests must not trigger wake attempts")
	}
	result := testServe(t, a, wakeLinkRequest(http.MethodGet, "w="+token, local), http.StatusOK)
	if !strings.Contains(string(result.Data), `"sent":true`) {
		t.Fatal("wake result did not report sending")
	}
	testServe(t, a, wakeLinkRequest(http.MethodGet, "w="+token, local), http.StatusTooManyRequests)
	logs := testStore(t, a).Logs
	if len(logs) != 1 || logs[0].Source != "url" || logs[0].DeviceID != d.ID || strings.Contains(logs[0].Message, token) {
		t.Fatal("only the selected device's wake should be logged, without the token")
	}
	newToken := editWakeLink(t, a, d.ID, "rotate")
	if newToken == token {
		t.Fatal("rotation must replace the token")
	}
	testServe(t, a, wakeLinkRequest(http.MethodGet, "w="+token, local), http.StatusForbidden)
	if err := a.update(func(s *Store) error { s.Active = false; return nil }); err != nil {
		t.Fatal(err)
	}
	testServe(t, a, wakeLinkRequest(http.MethodPost, "w="+newToken, local), http.StatusConflict)
	if err := a.update(func(s *Store) error { s.Active = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if editWakeLink(t, a, d.ID, "revoke") != "" {
		t.Fatal("revocation must remove the link")
	}
	testServe(t, a, wakeLinkRequest(http.MethodGet, "w="+newToken, local), http.StatusForbidden)
	token = editWakeLink(t, a, d.ID, "create")
	testServe(t, a, testRequest(t, a, http.MethodPost, "device-delete", map[string]string{"id": d.ID}), http.StatusOK)
	testServe(t, a, wakeLinkRequest(http.MethodGet, "w="+token, local), http.StatusForbidden)
	if len(testStore(t, a).WakeLinks) != 0 {
		t.Fatal("device deletion left an orphaned wake link")
	}
}

func TestWakeLinkThroughCGIWithoutSession(t *testing.T) {
	a, socket := testBackend(t)
	d := testDevice(t, a)
	token := editWakeLink(t, a, d.ID, "create")
	proxy := newCGIProxy(socket)
	for _, tc := range []struct {
		remote, prefetch string
		code             int
	}{
		{"203.0.113.10:12345", "", http.StatusForbidden},
		{"192.168.1.10:12345", "prefetch", http.StatusForbidden},
		{"192.168.1.10:12345", "", http.StatusOK},
	} {
		r := wakeLinkRequest(http.MethodPost, "w="+token, tc.remote)
		r.Header.Set("X-Forwarded-For", "192.168.1.10")
		r.Header.Set("Sec-Purpose", tc.prefetch)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		if w.Code != tc.code || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), token) {
			t.Fatalf("CGI wake for %s: got %d, want %d, body %s", tc.remote, w.Code, tc.code, w.Body.String())
		}
	}
}
