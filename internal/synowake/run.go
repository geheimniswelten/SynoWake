package synowake

import (
	"bytes"
	"crypto/hmac"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cgi"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrStopped = errors.New("SynoWake ist angehalten")

const PackageVersion = "0.1.13-0014"

func Run(args []string) error {
	isCGI := os.Getenv("GATEWAY_INTERFACE") != ""
	os.Setenv("PATH", "/usr/syno/bin:/usr/bin:/bin")
	for _, name := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "GCONV_PATH", "BASH_ENV", "ENV", "PYTHONPATH"} {
		os.Unsetenv(name)
	}
	if isCGI {
		// This unprivileged gateway never opens package data. The service owns it.
		return cgi.Serve(newCGIProxy(defaultSocket))
	}
	root := os.Getenv("SYNOWAKE_VAR")
	if root == "" {
		root = "/var/packages/SynoWake/var"
	}
	app := &App{Root: root}
	if len(args) == 0 {
		return errors.New("CGI oder init/start/stop/status/--serve/--run-schedule/--serve-demo erwartet.")
	}
	switch args[0] {
	case "init":
		return app.update(func(s *Store) error { return nil })
	case "start":
		return app.update(func(s *Store) error { s.Active = true; return nil })
	case "stop":
		return app.stopBackend(socketPath())
	case "status":
		s, err := app.readStore()
		if err != nil {
			return err
		}
		if !s.Active {
			return ErrStopped
		}
		active, err := backendHealth(socketPath())
		if err != nil || !active {
			return ErrStopped
		}
		return nil
	case "--serve":
		return app.serveBackend(socketPath())
	case "--serve-demo":
		if len(args) != 3 {
			return errors.New("--serve-demo 127.0.0.1:PORT UI_DIRECTORY")
		}
		host, _, err := net.SplitHostPort(args[1])
		if err != nil || host != "127.0.0.1" {
			return errors.New("Demo nur auf 127.0.0.1 erlaubt.")
		}
		app.Demo = true
		app.Root = filepath.Join(os.TempDir(), "synowake-demo-"+randomID())
		defer os.RemoveAll(app.Root)
		if err := app.update(func(s *Store) error { s.Active = true; return nil }); err != nil {
			return err
		}
		mux := http.NewServeMux()
		mux.Handle("/api.cgi", app)
		mux.Handle("/", http.FileServer(http.Dir(args[2])))
		fmt.Printf("SynoWake-Demo: http://%s/?demo=1\n", args[1])
		return http.ListenAndServe(args[1], mux)
	case "--run-schedule":
		flags := flag.NewFlagSet("SynoWake-Zeitplan", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		id := flags.String("run-schedule", "", "")
		token := flags.String("token", "", "")
		callback := flags.String("callback", "", "")
		if err := flags.Parse(args); err != nil {
			return errors.New("Ungültige Zeitplanargumente.")
		}
		if flags.NArg() != 0 || !safeID.MatchString(*id) || len(*token) != 64 {
			return errors.New("Ungültige Zeitplan-ID oder Ausführungsschlüssel.")
		}
		return invokeSchedule(*id, *token, *callback)
	default:
		return errors.New("Unbekannter Befehl.")
	}
}
func parseOrigin(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("Ursprung ungültig")
	}
	return u.Host, nil
}
func validatedCallback(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/webman/3rdparty/SynoWake/api.cgi" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("Zeitplan-Callback muss die lokale DSM-CGI-Adresse mit Port sein.")
	}
	port, e := strconv.Atoi(u.Port())
	if e != nil || port < 1 || port > 65535 {
		return nil, errors.New("Ungültiger lokaler DSM-Port.")
	}
	return u, nil
}
func invokeSchedule(id, token, callback string) error {
	u, err := validatedCallback(callback)
	if err != nil {
		return err
	}
	u.RawQuery = "action=scheduled-run"
	body, _ := json.Marshal(map[string]string{"id": id})
	// Certificate trust is bypassed solely for fixed 127.0.0.1 loopback; no external hosts are accepted.
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 25 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	var response *http.Response
	for redirects := 0; redirects < 3; redirects++ {
		req, e := http.NewRequest("POST", u.String(), bytes.NewReader(body))
		if e != nil {
			return e
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", systemLanguage())
		req.Header.Set("X-SynoWake-Task", token)
		response, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("Lokale DSM-CGI nicht erreichbar: %w", err)
		}
		if response.StatusCode < 300 || response.StatusCode > 399 {
			break
		}
		location, e := response.Location()
		response.Body.Close()
		if e != nil {
			return errors.New("Ungültige lokale DSM-Weiterleitung.")
		}
		copyURL := *location
		copyURL.RawQuery = ""
		if _, e := validatedCallback(copyURL.String()); e != nil {
			return errors.New("DSM leitet außerhalb der lokalen CGI-Adresse um.")
		}
		if redirects == 2 {
			return errors.New("Zu viele lokale DSM-Weiterleitungen.")
		}
		// DSM can enforce HTTPS via 301/302; replay the same POST only at the
		// validated loopback CGI instead of downgrading it to a GET.
		u = location
		u.RawQuery = "action=scheduled-run"
	}
	defer response.Body.Close()
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&result); err != nil {
		return errors.New("DSM lieferte keine gültige SynoWake-Antwort. Lokalen Port und CGI prüfen.")
	}
	if response.StatusCode != 200 || !result.Success {
		return errors.New("Zeitplan konnte nicht ausgeführt werden: " + result.Error)
	}
	return nil
}
func isLoopback(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
func scheduledMinute(s Schedule, now time.Time) (string, bool) {
	for late := 0; late <= 2; late++ {
		target := now.Add(-time.Duration(late) * time.Minute)
		if target.Format("15:04") != s.Time {
			continue
		}
		for _, d := range s.Days {
			if int(target.Weekday()) == d {
				return target.Format("2006-01-02T15:04"), true
			}
		}
	}
	return "", false
}
func (a *App) scheduledHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || !isLoopback(r.RemoteAddr) {
		writeJSON(w, 403, nil, errors.New("Zeitplanaufruf nur lokal zulässig."))
		return
	}
	var input struct {
		ID string `json:"id"`
	}
	if err := decodeBody(r, &input); err != nil {
		writeJSON(w, 400, nil, err)
		return
	}
	snapshot, err := a.readStore()
	if err != nil {
		writeJSON(w, 500, nil, err)
		return
	}
	var plan savedSchedule
	found := false
	for _, p := range snapshot.Schedules {
		if p.ID == input.ID {
			plan = p
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, 409, nil, errors.New("Zeitplan nicht gefunden oder noch nicht mit DSM synchronisiert."))
		return
	}
	var guardErr error
	claim := func(s *Store) (err error) {
		defer func() { guardErr = err }()
		if !s.Active {
			return ErrStopped
		}
		for i, p := range s.Schedules {
			if p.ID == input.ID {
				if !hmac.Equal([]byte(p.Secret), []byte(r.Header.Get("X-SynoWake-Task"))) {
					return errors.New("Ungültiger Ausführungsschlüssel.")
				}
				if !p.Enabled || p.TaskID < 1 {
					return errors.New("Zeitplan ist nicht aktiv.")
				}
				minute, ok := scheduledMinute(p.Schedule, time.Now())
				if !ok {
					return errors.New("Aufruf außerhalb des eingestellten Zeitfensters.")
				}
				if p.LastMinute == minute {
					return errDuplicate
				}
				if p.DeviceID != plan.DeviceID || p.Notify != plan.Notify {
					return errors.New("Zeitplan wurde parallel geändert. Bitte erneut aufrufen.")
				}
				s.Schedules[i].LastMinute = minute
				return nil
			}
		}
		return errors.New("Zeitplan nicht gefunden oder noch nicht mit DSM synchronisiert.")
	}
	// Minute claim, queue capacity, cooldown and durable intent share one locked
	// transaction. A failure before UDP sends rolls back the minute claim.
	duplicate, err := a.wakeDeviceClaim(plan.DeviceID, plan.Notify, "schedule", plan.ID, claim)
	if err != nil {
		code := 500
		if guardErr != nil {
			code = 409
		}
		writeJSON(w, code, nil, err)
		return
	}
	if duplicate {
		writeJSON(w, 200, map[string]bool{"duplicate": true}, nil)
		return
	}
	writeJSON(w, 200, map[string]bool{"executed": true}, nil)
}
