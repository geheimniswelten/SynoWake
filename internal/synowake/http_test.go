package synowake

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

type testResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func testApp(t *testing.T) *App {
	t.Helper()
	a := &App{Root: t.TempDir(), Demo: true}
	if err := a.update(func(s *Store) error { s.Active = true; return nil }); err != nil {
		t.Fatal(err)
	}
	return a
}

func testStore(t *testing.T, a *App) Store {
	t.Helper()
	s, err := a.readStore()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testRequest(t *testing.T, a *App, method, action string, body any) *http.Request {
	t.Helper()
	var content []byte
	if body != nil {
		var err error
		content, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, "https://nas.example:5001/webman/3rdparty/h5uSynoWake/api.cgi?action="+action, bytes.NewReader(content))
	r.Header.Set("Cookie", "id=authenticated-demo-session")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-SynoWake-CSRF", csrfFor(testStore(t, a), "Demo-Administrator", r))
	r.RemoteAddr = "192.168.1.10:12345"
	return r
}

func testServe(t *testing.T, a *App, r *http.Request, wantCode int) testResponse {
	t.Helper()
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != wantCode {
		t.Fatalf("HTTP %s %s: got %d, want %d: %s", r.Method, r.URL.Query().Get("action"), w.Code, wantCode, w.Body.String())
	}
	var result testResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON response: %v: %s", err, w.Body.String())
	}
	if result.Success != (wantCode == http.StatusOK) {
		t.Fatalf("unexpected success flag: %s", w.Body.String())
	}
	return result
}

func testDevice(t *testing.T, a *App) Device {
	t.Helper()
	result := testServe(t, a, testRequest(t, a, "POST", "device-save", Device{Name: "Test-PC", IP: "192.168.1.20", MAC: "02:11:22:33:44:55"}), 200)
	var d Device
	if err := json.Unmarshal(result.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDeviceCreationRejectsAlreadySavedMACWithoutChangingExistingDevice(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	before := testStore(t, a).Devices[0]
	for _, mac := range []string{"02:11:22:33:44:55", "02-11-22-33-44-55"} {
		duplicate := Device{Name: "Anderer DNS-Name", IP: "192.168.1.70", MAC: mac}
		result := testServe(t, a, testRequest(t, a, "POST", "device-save", duplicate), 400)
		if !strings.Contains(result.Error, "bereits vorhanden") {
			t.Fatalf("missing duplicate explanation: %s", result.Error)
		}
		devices := testStore(t, a).Devices
		if len(devices) != 1 || devices[0] != before {
			t.Fatal("duplicate import changed the existing device or inventory")
		}
	}
	d.Name = "Manuell bearbeitet"
	testServe(t, a, testRequest(t, a, "POST", "device-save", d), 200)
	if testStore(t, a).Devices[0].Name != d.Name {
		t.Fatal("duplicate protection prevented editing the existing device")
	}
	other := Device{Name: "Andere Netzwerkkarte", IP: d.IP, MAC: "02:11:22:33:44:66"}
	testServe(t, a, testRequest(t, a, "POST", "device-save", other), 200)
	if len(testStore(t, a).Devices) != 2 {
		t.Fatal("IP equality falsely classified a different MAC as already saved")
	}
}

func TestHTTPRequiresSessionBoundCSRFAndSameOrigin(t *testing.T) {
	a := testApp(t)
	state := testServe(t, a, testRequest(t, a, "GET", "state", nil), 200)
	var data struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(state.Data, &data); err != nil || len(data.CSRF) != 64 {
		t.Fatalf("invalid state CSRF: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*http.Request)
	}{
		{"missing token", func(r *http.Request) { r.Header.Del("X-SynoWake-CSRF") }},
		{"wrong token", func(r *http.Request) { r.Header.Set("X-SynoWake-CSRF", strings.Repeat("0", 64)) }},
		{"different session", func(r *http.Request) { r.Header.Set("Cookie", "id=other-session") }},
		{"external origin", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example:5001") }},
		{"userinfo origin", func(r *http.Request) { r.Header.Set("Origin", "https://user@nas.example:5001") }},
		{"cross site", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testRequest(t, a, "POST", "settings-save", map[string]int{"logCenterPort": 514})
			r.Header.Set("X-SynoWake-CSRF", data.CSRF)
			tc.edit(r)
			testServe(t, a, r, 403)
			if testStore(t, a).LogCenterPort != 0 {
				t.Fatal("rejected request changed persisted settings")
			}
		})
	}
	r := testRequest(t, a, "POST", "settings-save", map[string]int{"logCenterPort": 514})
	r.Header.Set("Origin", "https://nas.example:5001")
	testServe(t, a, r, 200)
	if testStore(t, a).LogCenterPort != 514 {
		t.Fatal("valid same-origin request did not persist")
	}
	first := testRequest(t, a, "GET", "state", nil)
	s := testStore(t, a)
	if csrfFor(s, "another-user", first) == csrfFor(s, "Demo-Administrator", first) {
		t.Fatal("CSRF token is not bound to authenticated username")
	}
}

func TestHTTPRejectsInvalidJSONAndStoppedMutations(t *testing.T) {
	a := testApp(t)
	for _, content := range []string{`{"logCenterPort":514,"unexpected":true}`, `{"logCenterPort":514}{}`, `{"logCenterPort":"514"}`, `{`} {
		r := testRequest(t, a, "POST", "settings-save", nil)
		r.Body = io.NopCloser(strings.NewReader(content))
		testServe(t, a, r, 400)
	}
	r := testRequest(t, a, "POST", "settings-save", map[string]int{"logCenterPort": 514})
	r.Header.Set("Content-Type", "text/plain")
	testServe(t, a, r, 400)
	testServe(t, a, testRequest(t, a, "PUT", "settings-save", map[string]int{"logCenterPort": 514}), 405)
	testServe(t, a, testRequest(t, a, "GET", "settings-save", nil), 404)
	if testStore(t, a).LogCenterPort != 0 {
		t.Fatal("invalid JSON or method changed settings")
	}
	if err := a.update(func(s *Store) error { s.Active = false; return nil }); err != nil {
		t.Fatal(err)
	}
	testServe(t, a, testRequest(t, a, "GET", "state", nil), 200)
	testServe(t, a, testRequest(t, a, "POST", "settings-save", map[string]int{"logCenterPort": 514}), 409)
	testServe(t, a, testRequest(t, a, "GET", "status", nil), 409)
}

func TestSchedulePrepareAbortCommitLifecycle(t *testing.T) {
	t.Setenv("SERVER_PORT", "5001")
	a := testApp(t)
	d := testDevice(t, a)
	plan := Schedule{Name: "Morgens", DeviceID: d.ID, Time: "07:30", Days: []int{1, 2, 3, 4, 5}, Enabled: true}
	prepare := func(plan Schedule) savedSchedule {
		t.Helper()
		result := testServe(t, a, testRequest(t, a, "POST", "schedule-prepare", plan), 200)
		var response struct {
			Schedule Schedule `json:"schedule"`
			Command  string   `json:"command"`
			Owner    string   `json:"owner"`
		}
		if err := json.Unmarshal(result.Data, &response); err != nil {
			t.Fatal(err)
		}
		s := testStore(t, a)
		for _, p := range s.Pending {
			if p.ID == response.Schedule.ID {
				if len(p.Secret) != 64 || response.Owner != "Demo-Administrator" || response.Command != commandFor(p) {
					t.Fatalf("invalid prepared schedule contract: %+v", response)
				}
				return p
			}
		}
		t.Fatal("prepared schedule was not persisted")
		return savedSchedule{}
	}
	p := prepare(plan)
	if len(testStore(t, a).Schedules) != 0 {
		t.Fatal("uncommitted schedule became active")
	}
	testServe(t, a, testRequest(t, a, "POST", "device-delete", map[string]string{"id": d.ID}), 400)
	testServe(t, a, testRequest(t, a, "POST", "schedule-commit", map[string]any{"id": p.ID, "taskId": 9, "taskOwner": "root"}), 400)
	testServe(t, a, testRequest(t, a, "POST", "schedule-commit", map[string]any{"id": p.ID, "taskId": 0, "taskOwner": "Demo-Administrator"}), 400)
	testServe(t, a, testRequest(t, a, "POST", "schedule-abort", map[string]string{"id": p.ID}), 200)
	if s := testStore(t, a); len(s.Pending) != 0 || len(s.Schedules) != 0 {
		t.Fatal("abort did not remove uncommitted schedule")
	}
	p = prepare(plan)
	testServe(t, a, testRequest(t, a, "POST", "schedule-commit", map[string]any{"id": p.ID, "taskId": 9, "taskOwner": "Demo-Administrator"}), 200)
	s := testStore(t, a)
	if len(s.Pending) != 0 || len(s.Schedules) != 1 || s.Schedules[0].TaskID != 9 || s.Schedules[0].Secret != p.Secret {
		t.Fatalf("commit did not activate exactly the prepared record: %+v", s.Schedules)
	}
	changed := s.Schedules[0].Schedule
	changed.Time = "08:15"
	newPending := prepare(changed)
	s = testStore(t, a)
	if s.Schedules[0].Time != "07:30" || newPending.Time != "08:15" || newPending.Secret != p.Secret || newPending.Callback != p.Callback {
		t.Fatal("edit changed active schedule before commit or rotated its DSM command")
	}
	secondEdit := changed
	secondEdit.Time = "09:00"
	testServe(t, a, testRequest(t, a, "POST", "schedule-prepare", secondEdit), 400)
	if s := testStore(t, a); len(s.Pending) != 1 || s.Pending[0].Time != "08:15" {
		t.Fatal("a second edit replaced the pending DSM transaction")
	}
	testServe(t, a, testRequest(t, a, "POST", "schedule-abort", map[string]string{"id": p.ID}), 200)
	if s := testStore(t, a); len(s.Pending) != 0 || s.Schedules[0].Time != "07:30" {
		t.Fatal("edit abort did not preserve active schedule")
	}
	testServe(t, a, testRequest(t, a, "POST", "schedule-delete", map[string]string{"id": p.ID}), 200)
	testServe(t, a, testRequest(t, a, "POST", "device-delete", map[string]string{"id": d.ID}), 200)
	if s := testStore(t, a); len(s.Schedules) != 0 || len(s.Pending) != 0 || len(s.Devices) != 0 {
		t.Fatal("schedule/device deletion left records")
	}
}

func TestScheduleConcurrentEditsAndCommitAreBoundToOwner(t *testing.T) {
	a := testApp(t)
	p := testRunnableSchedule(t, a)
	var requests []*http.Request
	for _, at := range []string{"08:10", "10:45"} {
		edit := p.Schedule
		edit.Time = at
		requests = append(requests, testRequest(t, a, "POST", "schedule-prepare", edit))
	}
	responses := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for _, r := range requests {
		wg.Add(1)
		go func() { defer wg.Done(); w := httptest.NewRecorder(); a.ServeHTTP(w, r); responses <- w }()
	}
	wg.Wait()
	close(responses)
	accepted, rejected := 0, 0
	for response := range responses {
		switch response.Code {
		case 200:
			accepted++
		case 400:
			rejected++
		default:
			t.Fatalf("unexpected concurrent prepare response: %d %s", response.Code, response.Body.String())
		}
	}
	if accepted != 1 || rejected != 1 || len(testStore(t, a).Pending) != 1 {
		t.Fatal("concurrent edits did not reserve exactly one pending transaction")
	}
	if err := a.update(func(s *Store) error { s.Pending[0].TaskOwner = "Other-Administrator"; return nil }); err != nil {
		t.Fatal(err)
	}
	testServe(t, a, testRequest(t, a, "POST", "schedule-commit", map[string]any{"id": p.ID, "taskId": 23, "taskOwner": "Demo-Administrator"}), 400)
	s := testStore(t, a)
	if len(s.Pending) != 1 || s.Schedules[0].TaskID != p.TaskID || s.Schedules[0].Time != p.Time {
		t.Fatal("different administrator claimed pending transaction")
	}
	testServe(t, a, testRequest(t, a, "POST", "schedule-abort", map[string]string{"id": p.ID}), 200)
	if err := a.update(func(s *Store) error { s.Schedules[0].TaskOwner = "Other-Administrator"; return nil }); err != nil {
		t.Fatal(err)
	}
	edit := p.Schedule
	edit.Time = "08:10"
	testServe(t, a, testRequest(t, a, "POST", "schedule-prepare", edit), 400)
	if len(testStore(t, a).Pending) != 0 {
		t.Fatal("different administrator edited a foreign-owned DSM task")
	}
}

func testRunnableSchedule(t *testing.T, a *App) savedSchedule {
	t.Helper()
	d := testDevice(t, a)
	now := time.Now()
	p := savedSchedule{Schedule: Schedule{ID: randomID(), Name: "Jetzt", DeviceID: d.ID, Time: now.Format("15:04"), Days: []int{int(now.Weekday())}, Enabled: true, TaskID: 12, TaskOwner: "Demo-Administrator"}, Secret: randomHex(32), Callback: "https://127.0.0.1:5001/webman/3rdparty/h5uSynoWake/api.cgi"}
	if err := a.update(func(s *Store) error { s.Schedules = append(s.Schedules, p); return nil }); err != nil {
		t.Fatal(err)
	}
	return p
}

func testScheduledRequest(t *testing.T, a *App, p savedSchedule) *http.Request {
	t.Helper()
	r := testRequest(t, a, "POST", "scheduled-run", map[string]string{"id": p.ID})
	r.Header.Del("Cookie")
	r.Header.Del("X-SynoWake-CSRF")
	r.Header.Set("X-SynoWake-Task", p.Secret)
	r.RemoteAddr = "127.0.0.1:12345"
	return r
}

func TestScheduledExecutionRejectsRemoteSecretsAndInactivePlans(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		request func(*http.Request)
		store   func(*Store)
	}{
		{name: "remote caller", code: 403, request: func(r *http.Request) { r.RemoteAddr = "192.168.1.10:1"; r.Header.Set("X-Forwarded-For", "127.0.0.1") }},
		{name: "GET caller", code: 403, request: func(r *http.Request) { r.Method = "GET" }},
		{name: "wrong secret", code: 409, request: func(r *http.Request) { r.Header.Set("X-SynoWake-Task", strings.Repeat("0", 64)) }},
		{name: "missing secret", code: 409, request: func(r *http.Request) { r.Header.Del("X-SynoWake-Task") }},
		{name: "stopped", code: 409, store: func(s *Store) { s.Active = false }},
		{name: "disabled", code: 409, store: func(s *Store) { s.Schedules[0].Enabled = false }},
		{name: "unassigned", code: 409, store: func(s *Store) { s.Schedules[0].TaskID = 0 }},
		{name: "wrong day", code: 409, store: func(s *Store) { s.Schedules[0].Days = []int{(int(time.Now().Weekday()) + 3) % 7} }},
		{name: "wrong time", code: 409, store: func(s *Store) { s.Schedules[0].Time = time.Now().Add(6 * time.Hour).Format("15:04") }},
		{name: "pending only", code: 409, store: func(s *Store) { s.Pending = s.Schedules; s.Schedules = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t)
			p := testRunnableSchedule(t, a)
			if tc.store != nil {
				if err := a.update(func(s *Store) error { tc.store(s); return nil }); err != nil {
					t.Fatal(err)
				}
			}
			r := testScheduledRequest(t, a, p)
			if tc.request != nil {
				tc.request(r)
			}
			testServe(t, a, r, tc.code)
			s := testStore(t, a)
			if !s.Devices[0].LastWake.IsZero() || len(s.Logs) != 0 {
				t.Fatal("rejected execution woke device or added execution log")
			}
		})
	}
}

func TestScheduledExecutionIsPersistentAndConcurrentAtMostOnce(t *testing.T) {
	a := testApp(t)
	p := testRunnableSchedule(t, a)
	const callers = 16
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, callers)
	for i := 0; i < callers; i++ {
		r := testScheduledRequest(t, a, p)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			responses <- w
		}()
	}
	wg.Wait()
	close(responses)
	executed, duplicates := 0, 0
	for w := range responses {
		if w.Code != 200 {
			t.Fatalf("concurrent execution failed: HTTP %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Data struct{ Executed, Duplicate bool }
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Data.Executed {
			executed++
		}
		if response.Data.Duplicate {
			duplicates++
		}
	}
	if executed != 1 || duplicates != callers-1 {
		t.Fatalf("executions=%d duplicates=%d", executed, duplicates)
	}
	s := testStore(t, a)
	if s.Devices[0].LastWake.IsZero() || s.Devices[0].WakeState != "waking" || len(s.Logs) != 1 || s.Logs[0].Source != "schedule" || s.Logs[0].ScheduleID != p.ID || s.Schedules[0].LastMinute == "" {
		t.Fatalf("execution was not durably logged and claimed: %+v", s)
	}
	restarted := &App{Root: a.Root, Demo: true}
	result := testServe(t, restarted, testScheduledRequest(t, restarted, p), 200)
	if !bytes.Contains(result.Data, []byte(`"duplicate":true`)) || len(testStore(t, restarted).Logs) != 1 {
		t.Fatal("restart permitted a replay")
	}
	if err := a.update(func(s *Store) error { s.Active = false; return nil }); err != nil {
		t.Fatal(err)
	}
	testServe(t, a, testScheduledRequest(t, a, p), 409)
}

func TestScheduledPreflightFailureDoesNotConsumeMinute(t *testing.T) {
	for _, reason := range []string{"full queue", "device cooldown"} {
		t.Run(reason, func(t *testing.T) {
			a := testApp(t)
			p := testRunnableSchedule(t, a)
			if err := a.update(func(s *Store) error {
				if reason == "full queue" {
					for i := 0; i < 3000; i++ {
						s.Logs = append(s.Logs, Log{ID: randomID(), CenterPending: true, Message: "Pending", Source: "integration"})
					}
				} else {
					s.Devices[0].LastWake = time.Now().UTC()
					s.Devices[0].WakeState = "waking"
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			before := testStore(t, a)
			testServe(t, a, testScheduledRequest(t, a, p), 500)
			after := testStore(t, a)
			if after.Schedules[0].LastMinute != "" || !after.Devices[0].LastWake.Equal(before.Devices[0].LastWake) || len(after.Logs) != len(before.Logs) {
				t.Fatal("preflight failure consumed the minute or mutated durable execution state")
			}
			if err := a.update(func(s *Store) error {
				for i := range s.Logs {
					s.Logs[i].CenterPending = false
				}
				s.Devices[0].LastWake = time.Now().Add(-6 * time.Second).UTC()
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			result := testServe(t, a, testScheduledRequest(t, a, p), 200)
			if !bytes.Contains(result.Data, []byte(`"executed":true`)) {
				t.Fatal("retry after queue/cooldown recovery did not execute")
			}
			after = testStore(t, a)
			attempts := 0
			for _, l := range after.Logs {
				if l.Source == "schedule" && l.ScheduleID == p.ID {
					attempts++
				}
			}
			if after.Schedules[0].LastMinute == "" || attempts != 1 || time.Since(after.Devices[0].LastWake) > 5*time.Second {
				t.Fatal("recovered execution did not atomically claim and log one wake")
			}
			result = testServe(t, a, testScheduledRequest(t, a, p), 200)
			if !bytes.Contains(result.Data, []byte(`"duplicate":true`)) {
				t.Fatal("successful recovery permitted replay")
			}
		})
	}
}

func TestInvokeScheduleThroughLocalHTTPAndTLS(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(fmt.Sprintf("TLS=%v", tls), func(t *testing.T) {
			a := testApp(t)
			p := testRunnableSchedule(t, a)
			var server *httptest.Server
			if tls {
				server = httptest.NewTLSServer(a)
			} else {
				server = httptest.NewServer(a)
			}
			defer server.Close()
			callback := server.URL + "/webman/3rdparty/h5uSynoWake/api.cgi"
			if err := invokeSchedule(p.ID, p.Secret, callback); err != nil {
				t.Fatal(err)
			}
			if s := testStore(t, a); s.Devices[0].LastWake.IsZero() || len(s.Logs) != 1 {
				t.Fatal("local CLI callback did not execute")
			}
			if err := invokeSchedule(p.ID, p.Secret, callback); err != nil {
				t.Fatal(err)
			}
			if len(testStore(t, a).Logs) != 1 {
				t.Fatal("CLI callback replay added a second wake")
			}
		})
	}
}

func TestInvokeScheduleRedirectPreservesPOSTAndRejectsExternalTargets(t *testing.T) {
	a := testApp(t)
	p := testRunnableSchedule(t, a)
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Query().Get("action") != "scheduled-run" || r.Header.Get("X-SynoWake-Task") != p.Secret || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "redirect lost execution request", 400)
			return
		}
		a.ServeHTTP(w, r)
	}))
	defer destination.Close()
	for _, code := range []int{301, 302, 307, 308} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, destination.URL+"/webman/3rdparty/h5uSynoWake/api.cgi", code)
			}))
			defer source.Close()
			if err := invokeSchedule(p.ID, p.Secret, source.URL+"/webman/3rdparty/h5uSynoWake/api.cgi"); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(testStore(t, a).Logs) != 1 {
		t.Fatal("redirected replay executed twice")
	}
	for _, target := range []string{"https://example.invalid:5001/webman/3rdparty/h5uSynoWake/api.cgi", destination.URL + "/another.cgi"} {
		source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target, 302)
		}))
		err := invokeSchedule(p.ID, p.Secret, source.URL+"/webman/3rdparty/h5uSynoWake/api.cgi")
		source.Close()
		if err == nil || !strings.Contains(err.Error(), "außerhalb") {
			t.Fatalf("accepted unexpected redirect %q: %v", target, err)
		}
	}
}

func TestStoreConcurrentUpdatesAndFailedTransaction(t *testing.T) {
	a := testApp(t)
	const writers, increments = 6, 6
	var wg sync.WaitGroup
	errorsSeen := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				if err := a.update(func(s *Store) error { s.LogCenterPort++; return nil }); err != nil {
					errorsSeen <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("concurrent update failed: %v; persisted %d of %d increments", err, testStore(t, a).LogCenterPort, writers*increments)
	}
	if got := testStore(t, a).LogCenterPort; got != writers*increments {
		t.Fatalf("lost persisted updates: got %d, want %d", got, writers*increments)
	}
	path := filepath.Join(a.Root, "state.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantError := errors.New("abort transaction")
	if err := a.update(func(s *Store) error { s.LogCenterPort = 0; return wantError }); !errors.Is(err, wantError) {
		t.Fatalf("transaction error=%v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed transaction modified committed data: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(a.Root, ".state-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary state files remain: %v %v", files, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"csrfSecret":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.update(func(s *Store) error { s.Active = true; return nil }); err == nil {
		t.Fatal("corrupt state was silently overwritten")
	}
}
