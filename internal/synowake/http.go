package synowake

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func writeJSON(w http.ResponseWriter, code int, data any, err error) {
	language := "de"
	if localized, ok := w.(*languageWriter); ok {
		language = localized.language
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Language", language)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": translateMessage(err.Error(), language)})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"success": true, "data": localizeData(data, language)})
}
func authenticate(r *http.Request) (string, error) {
	for _, path := range []string{"/usr/syno/synoman/webman/authenticate.cgi", "/usr/syno/synoman/webman/modules/authenticate.cgi"} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		// Build a separate CGI environment for each request. The package service
		// handles concurrent users and must never reuse process-wide HTTP values.
		output, err := runAuthenticationCommand(path, r)
		if err != nil {
			continue
		}
		user := strings.TrimSpace(output)
		if user == "" || strings.ContainsAny(user, "\r\n\x00") || len(user) > 256 {
			continue
		}
		idPath, err := exec.LookPath("id")
		if err != nil {
			return "", errors.New("DSM-Gruppenprüfung nicht verfügbar.")
		}
		groups, err := runCommand(idPath, "-Gn", "--", user)
		if err != nil {
			return "", errors.New("DSM-Benutzer konnte nicht geprüft werden.")
		}
		for _, g := range strings.Fields(groups) {
			if g == "administrators" {
				return user, nil
			}
		}
		return "", errors.New("SynoWake ist nur für DSM-Administratoren freigegeben.")
	}
	return "", errors.New("DSM-Anmeldung konnte nicht geprüft werden. Sitzung oder authenticate.cgi-Berechtigung prüfen.")
}
func csrfFor(s Store, user string, r *http.Request) string {
	h := hmac.New(sha256.New, []byte(s.CSRFSecret))
	h.Write([]byte(user + "\x00" + r.Header.Get("Cookie")))
	return hex.EncodeToString(h.Sum(nil))
}
func decodeBody(r *http.Request, value any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("JSON-Anfrage erforderlich.")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 128<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return errors.New("Ungültige JSON-Eingabe: " + err.Error())
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("Nur ein JSON-Objekt ist zulässig.")
	}
	return nil
}

type scheduleView struct {
	Schedule
	Command string `json:"command"`
	Pending bool   `json:"pending"`
}

func commandFor(s savedSchedule) string {
	args := []string{"/var/packages/SynoWake/target/bin/synowake", "--run-schedule", s.ID, "--token", s.Secret, "--callback", s.Callback}
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(args, " ")
}
func callbackFor(r *http.Request) string {
	meta, _ := r.Context().Value(gatewayContextKey{}).(gatewayContext)
	port := meta.ServerPort
	if _, err := strconv.Atoi(port); err != nil {
		_, port, _ = net.SplitHostPort(r.Host)
	}
	scheme := "http"
	if meta.Scheme == "https" || r.TLS != nil || r.URL.Scheme == "https" || port == "5001" {
		scheme = "https"
	}
	if port == "" {
		if scheme == "https" {
			port = "5001"
		} else {
			port = "5000"
		}
	}
	return scheme + "://127.0.0.1:" + port + "/webman/3rdparty/SynoWake/api.cgi"
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w = forRequest(w, r)
	action := r.URL.Query().Get("action")
	if action == "scheduled-run" {
		a.scheduledHTTP(w, r)
		return
	}
	var user string
	var err error
	if a.Demo {
		user = "Demo-Administrator"
	} else {
		user, err = authenticate(r)
	}
	if err != nil {
		writeJSON(w, 403, nil, err)
		return
	}
	s, err := a.readStore()
	if err != nil {
		writeJSON(w, 500, nil, err)
		return
	}
	if r.Method == "POST" {
		if !hmac.Equal([]byte(r.Header.Get("X-SynoWake-CSRF")), []byte(csrfFor(s, user, r))) {
			writeJSON(w, 403, nil, errors.New("Sicherheitsprüfung fehlgeschlagen. Ansicht neu laden."))
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			o, err := parseOrigin(origin)
			if err != nil || !strings.EqualFold(o, r.Host) {
				writeJSON(w, 403, nil, errors.New("Anfrage stammt nicht vom DSM-Ursprung."))
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeJSON(w, 403, nil, errors.New("Fremde Webseiten sind nicht zugelassen."))
			return
		}
	} else if r.Method != "GET" {
		writeJSON(w, 405, nil, errors.New("HTTP-Methode nicht zulässig."))
		return
	}
	if !s.Active && action != "state" {
		writeJSON(w, 409, nil, errors.New("SynoWake ist im Paket-Zentrum angehalten."))
		return
	}
	if r.Method == "GET" {
		switch action {
		case "state":
			networks, networkErr := localNetworks()
			if a.Demo {
				demoNetwork, _ := parseLocalNetwork("Demo-LAN", "192.168.1.2/24")
				networks, networkErr = []LocalNetwork{demoNetwork}, nil
			}
			host := r.Host
			if address, _, err := net.SplitHostPort(host); err == nil {
				host = address
			}
			networks = preferredLocalNetworks(networks, metadataFor(r).ServerAddress, host)
			networkError := ""
			if networkErr != nil {
				networkError = "NAS-Netzwerke konnten nicht ermittelt werden: " + networkErr.Error()
			}
			plans := []scheduleView{}
			for _, p := range s.Schedules {
				pending := false
				for _, q := range s.Pending {
					if q.ID == p.ID {
						pending = true
					}
				}
				plans = append(plans, scheduleView{p.Schedule, commandFor(p), pending})
			}
			pendingPlans := []scheduleView{}
			for _, p := range s.Pending {
				pendingPlans = append(pendingPlans, scheduleView{p.Schedule, commandFor(p), true})
			}
			writeJSON(w, 200, map[string]any{"user": user, "version": PackageVersion, "csrf": csrfFor(s, user, r), "devices": s.Devices, "networks": networks, "discoveryCidr": preferredDiscoveryCIDR(networks, s.DiscoveryCIDR), "networkError": networkError, "schedules": plans, "pendingSchedules": pendingPlans, "logs": s.Logs, "diagnostics": s.Diagnostics, "active": s.Active, "settings": map[string]int{"logCenterPort": s.LogCenterPort}}, nil)
		case "status":
			a.statusHTTP(w, s)
		default:
			writeJSON(w, 404, nil, errors.New("Unbekannte Abfrage."))
		}
		return
	}
	var data any
	switch action {
	case "settings-save":
		var input struct {
			LogCenterPort int `json:"logCenterPort"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			err = a.update(func(s *Store) error {
				if input.LogCenterPort < 0 || input.LogCenterPort > 65535 {
					return errors.New("TCP-Port muss 0 bis 65535 sein.")
				}
				s.LogCenterPort = input.LogCenterPort
				return nil
			})
		}
	case "log-retry":
		var input struct{}
		err = decodeBody(r, &input)
		if err == nil {
			data, err = a.retryCenter()
		}
	case "device-save":
		var d Device
		err = decodeBody(r, &d)
		if err == nil {
			err = validateDevice(&d)
		}
		if err == nil {
			err = a.update(func(s *Store) error {
				for i, old := range s.Devices {
					if d.ID == "" && strings.EqualFold(old.MAC, d.MAC) {
						return errors.New("Gerät mit dieser MAC-Adresse bereits vorhanden: " + old.Name)
					}
					if old.ID == d.ID {
						d.LastWake = old.LastWake
						d.WakeState = old.WakeState
						s.Devices[i] = d
						data = d
						return nil
					}
				}
				if d.ID != "" {
					return errors.New("Gerät nicht gefunden.")
				}
				if len(s.Devices) >= 128 {
					return errors.New("Höchstens 128 Geräte möglich.")
				}
				d.ID = randomID()
				d.LastWake = time.Time{}
				d.WakeState = ""
				s.Devices = append(s.Devices, d)
				data = d
				return nil
			})
		}
	case "device-favorite":
		var input struct {
			ID       string `json:"id"`
			Favorite *bool  `json:"favorite"`
		}
		err = decodeBody(r, &input)
		if err == nil && input.Favorite == nil {
			err = errors.New("Favoriten-Auswahl fehlt.")
		}
		if err == nil {
			err = a.update(func(s *Store) error {
				for i := range s.Devices {
					if s.Devices[i].ID == input.ID {
						s.Devices[i].Favorite = *input.Favorite
						data = s.Devices[i]
						return nil
					}
				}
				return errors.New("Gerät nicht gefunden.")
			})
		}
	case "device-delete":
		var input struct {
			ID string `json:"id"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			err = a.update(func(s *Store) error {
				for _, p := range append(append([]savedSchedule{}, s.Schedules...), s.Pending...) {
					if p.DeviceID == input.ID {
						return errors.New("Zuerst die Zeitpläne dieses Geräts entfernen.")
					}
				}
				for i, d := range s.Devices {
					if d.ID == input.ID {
						s.Devices = append(s.Devices[:i], s.Devices[i+1:]...)
						return nil
					}
				}
				return errors.New("Gerät nicht gefunden.")
			})
		}
	case "wake":
		var input struct {
			IDs    []string `json:"ids"`
			Notify bool     `json:"notify"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			if len(input.IDs) == 0 || len(input.IDs) > 32 {
				err = errors.New("Ein bis 32 Geräte auswählen.")
			} else {
				results := []map[string]any{}
				seen := map[string]bool{}
				for _, id := range input.IDs {
					if seen[id] {
						continue
					}
					seen[id] = true
					e := a.wakeDevice(id, input.Notify, "manual", "")
					result := map[string]any{"id": id, "success": e == nil}
					if e != nil {
						result["error"] = e.Error()
					}
					results = append(results, result)
				}
				data = map[string]any{"results": results}
			}
		}
	case "discover":
		var input struct {
			CIDR string `json:"cidr"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			if a.Demo {
				name := "Arbeitszimmer-PC"
				if localized, ok := w.(*languageWriter); ok {
					name = formatMessage(name, localized.language)
				}
				data = map[string]any{"devices": []FoundDevice{{Name: name, IP: "192.168.1.20", MAC: "00:11:22:33:44:55", Type: "LAN-Gerät", Broadcast: "192.168.1.255"}}, "warnings": []string{"Demo-Suche"}}
			} else {
				var found []FoundDevice
				var warnings []string
				found, warnings, err = discover(input.CIDR)
				if err == nil {
					_, network, _ := net.ParseCIDR(strings.TrimSpace(input.CIDR))
					if saveErr := a.update(func(s *Store) error {
						s.DiscoveryCIDR = network.String()
						return nil
					}); saveErr != nil {
						warnings = append(warnings, "Suchbereich konnte nicht gespeichert werden: "+saveErr.Error())
					}
				}
				data = map[string]any{"devices": found, "warnings": warnings}
			}
		}
	case "schedule-prepare":
		var plan Schedule
		err = decodeBody(r, &plan)
		if err == nil {
			err = a.update(func(s *Store) error {
				if err := validateSchedule(&plan, s); err != nil {
					return err
				}
				if plan.ID == "" && len(s.Schedules)+len(s.Pending) >= 128 {
					return errors.New("Höchstens 128 Zeitpläne möglich.")
				}
				next := savedSchedule{Schedule: plan, Secret: randomHex(32), Callback: callbackFor(r)}
				if next.ID == "" {
					next.ID = randomID()
				} else {
					for _, p := range s.Pending {
						if p.ID == next.ID {
							return errors.New("Dieser Zeitplan wartet bereits auf DSM-Abgleich. Zuerst die Zuordnung reparieren.")
						}
					}
					found := false
					for _, old := range s.Schedules {
						if old.ID == next.ID {
							if old.TaskOwner != user {
								return errors.New("Diesen Zeitplan mit seinem DSM-Eigentümer bearbeiten.")
							}
							next.Secret = old.Secret
							next.Callback = old.Callback
							next.TaskID = old.TaskID
							next.TaskOwner = old.TaskOwner
							next.LastMinute = old.LastMinute
							found = true
						}
					}
					if !found {
						return errors.New("Zeitplan nicht gefunden.")
					}
				}
				if _, e := validatedCallback(next.Callback); e != nil {
					return e
				}
				next.TaskOwner = user
				filtered := []savedSchedule{}
				for _, old := range s.Pending {
					if old.ID != next.ID {
						filtered = append(filtered, old)
					}
				}
				s.Pending = append(filtered, next)
				data = map[string]any{"schedule": next.Schedule, "command": commandFor(next), "owner": user}
				return nil
			})
		}
	case "schedule-commit":
		var input struct {
			ID        string `json:"id"`
			TaskID    int    `json:"taskId"`
			TaskOwner string `json:"taskOwner"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			err = a.update(func(s *Store) error {
				if input.TaskID < 1 || input.TaskOwner != user {
					return errors.New("Ungültige DSM-Aufgabe oder Eigentümer.")
				}
				for i, p := range s.Pending {
					if p.ID == input.ID {
						if p.TaskOwner != user {
							return errors.New("Der vorbereitete Zeitplan gehört einem anderen DSM-Benutzer.")
						}
						p.TaskID = input.TaskID
						p.TaskOwner = input.TaskOwner
						plans := []savedSchedule{}
						for _, old := range s.Schedules {
							if old.ID != p.ID {
								plans = append(plans, old)
							}
						}
						s.Schedules = append(plans, p)
						s.Pending = append(s.Pending[:i], s.Pending[i+1:]...)
						data = p.Schedule
						addLog(s, Log{Level: "info", Source: "configuration", ScheduleID: p.ID, Message: "Zeitplan gespeichert: " + p.Name, MessageKey: "Zeitplan gespeichert: {0}", MessageArgs: []string{p.Name}})
						return nil
					}
				}
				return errors.New("Kein vorbereiteter Zeitplan gefunden.")
			})
		}
	case "schedule-abort":
		var input struct {
			ID string `json:"id"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			err = a.update(func(s *Store) error {
				filtered := []savedSchedule{}
				for _, p := range s.Pending {
					if p.ID != input.ID {
						filtered = append(filtered, p)
					}
				}
				s.Pending = filtered
				return nil
			})
		}
	case "schedule-delete":
		var input struct {
			ID string `json:"id"`
		}
		err = decodeBody(r, &input)
		if err == nil {
			err = a.update(func(s *Store) error {
				plans := []savedSchedule{}
				for _, p := range s.Schedules {
					if p.ID != input.ID {
						plans = append(plans, p)
					}
				}
				s.Schedules = plans
				pending := []savedSchedule{}
				for _, p := range s.Pending {
					if p.ID != input.ID {
						pending = append(pending, p)
					}
				}
				s.Pending = pending
				return nil
			})
		}
	default:
		err = errors.New("Unbekannte Aktion.")
	}
	if err != nil {
		writeJSON(w, 400, nil, err)
		return
	}
	writeJSON(w, 200, data, nil)
}

func (a *App) wakeDevice(id string, notify bool, source, scheduleID string) error {
	_, err := a.wakeDeviceClaim(id, notify, source, scheduleID, nil)
	return err
}

var errDuplicate = errors.New("Zeitplan bereits ausgeführt")

func (a *App) wakeDeviceClaim(id string, notify bool, source, scheduleID string, claim func(*Store) error) (bool, error) {
	var device Device
	logID := randomID()
	logTime := time.Now().UTC()
	err := a.update(func(s *Store) error {
		if claim != nil {
			if err := claim(s); err != nil {
				return err
			}
		}
		if !s.Active {
			return ErrStopped
		}
		pending := 0
		for _, l := range s.Logs {
			if l.CenterPending {
				pending++
			}
		}
		if pending >= 3000 {
			return errors.New("Protokoll-Center-Warteschlange voll. Vor weiteren Weckversuchen ausstehende Einträge übertragen.")
		}
		for i, d := range s.Devices {
			if d.ID == id {
				if time.Since(d.LastWake) < 5*time.Second {
					return errors.New("Gerät wurde gerade aufgeweckt. Bitte kurz warten.")
				}
				device = d
				s.Devices[i].LastWake = time.Now().UTC()
				s.Devices[i].WakeState = "waking"
				addLog(s, Log{ID: logID, Time: logTime, Level: "info", Source: source, DeviceID: id, ScheduleID: scheduleID, Message: "Aufweckversuch: " + d.Name, MessageKey: "Aufweckversuch: {0}", MessageArgs: []string{d.Name}, CenterPending: !a.Demo})
				return nil
			}
		}
		return errors.New("Gerät nicht gefunden.")
	})
	if err != nil {
		if errors.Is(err, errDuplicate) {
			return true, nil
		}
		return false, err
	}
	if !a.Demo {
		err = wake(device)
	}
	level := "info"
	messageKey := "Magic Packets gesendet: {0}"
	messageArgs := []string{device.Name}
	if err != nil {
		level = "error"
		messageKey = "Aufwecken fehlgeschlagen: {0} – {1}"
		messageArgs = append(messageArgs, err.Error())
		a.update(func(s *Store) error {
			for i, d := range s.Devices {
				if d.ID == id {
					s.Devices[i].WakeState = "failed"
				}
			}
			return nil
		})
	}
	message := formatMessage(messageKey, "de", messageArgs...)
	logErr := a.event(Log{ID: logID, Time: logTime, Level: level, Source: source, DeviceID: id, ScheduleID: scheduleID, Message: message, MessageKey: messageKey, MessageArgs: messageArgs}, notify)
	if err != nil {
		return false, err
	}
	return false, logErr
}
func (a *App) statusHTTP(w http.ResponseWriter, s Store) {
	type status struct {
		ID       string    `json:"id"`
		Status   string    `json:"status"`
		LastWake time.Time `json:"lastWake"`
		Error    string    `json:"error,omitempty"`
		Method   string    `json:"probeMethod,omitempty"`
	}
	results := make([]status, len(s.Devices))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i, d := range s.Devices {
		i, d := i, d
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			online := false
			var err error
			method := ""
			if a.Demo {
				online = d.WakeState == "waking" && time.Since(d.LastWake) > 6*time.Second
			} else {
				online, method, err = probeHost(d.IP)
			}
			value := "offline"
			if online {
				value = "online"
			} else if d.WakeState == "waking" && time.Since(d.LastWake) < 90*time.Second {
				value = "waking"
			} else if err != nil {
				value = "unknown"
			}
			entry := status{ID: d.ID, Status: value, LastWake: d.LastWake, Method: method}
			if err != nil {
				entry.Error = err.Error()
			}
			results[i] = entry
			if d.WakeState == "waking" && (online || time.Since(d.LastWake) >= 90*time.Second) {
				changed := false
				a.update(func(store *Store) error {
					for j, current := range store.Devices {
						if current.ID == d.ID && current.WakeState == "waking" && current.LastWake.Equal(d.LastWake) {
							store.Devices[j].WakeState = value
							changed = true
						}
					}
					return nil
				})
				if changed {
					msg := "Gerät ist online: " + d.Name
					key := "Gerät ist online: {0}"
					level := "info"
					if !online {
						msg = "Kein Erreichbarkeitsnachweis nach Aufwecken: " + d.Name
						key = "Kein Erreichbarkeitsnachweis nach Aufwecken: {0}"
						level = "warning"
					}
					a.event(Log{Level: level, Source: "status", DeviceID: d.ID, Message: msg, MessageKey: key, MessageArgs: []string{d.Name}}, false)
				}
			}
		}()
	}
	wg.Wait()
	writeJSON(w, 200, map[string]any{"devices": results}, nil)
}
