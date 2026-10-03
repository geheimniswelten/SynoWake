package synowake

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestLocalizedRequestsDoNotChangeNamesOrStoredLogs(t *testing.T) {
	a := testApp(t)
	name := "Arbeitsrechner {0} <img> 100%"
	id := randomID()
	if err := a.update(func(s *Store) error {
		s.Devices = append(s.Devices, Device{ID: id, Name: name})
		s.Logs = append(s.Logs, Log{Message: "Zeitplan gespeichert: " + name})
		s.Diagnostics = append(s.Diagnostics, Diagnostic{Code: "test", Message: "Name enthält Steuerzeichen."})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, language := range []string{"de-DE", "en-US"} {
		language := language
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				r := httptest.NewRequest("GET", "/api.cgi?action=state", nil)
				r.Header.Set("Accept-Language", language)
				w := httptest.NewRecorder()
				a.ServeHTTP(w, r)
				var response struct {
					Data struct {
						Devices []Device     `json:"devices"`
						Logs    []Log        `json:"logs"`
						Diag    []Diagnostic `json:"diagnostics"`
					} `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Error(err)
					return
				}
				if response.Data.Devices[0].Name != name {
					t.Error("localization changed a saved name")
				}
				wantLog, wantDiag := "Zeitplan gespeichert: "+name, "Name enthält Steuerzeichen."
				if language == "en-US" {
					wantLog, wantDiag = "Schedule saved: "+name, "The name contains control characters."
				}
				if response.Data.Logs[0].Message != wantLog || response.Data.Diag[0].Message != wantDiag {
					t.Errorf("language leaked between requests: %s: %s", language, w.Body.String())
				}
			}
		}()
	}
	wg.Wait()
	if testStore(t, a).Logs[0].Message != "Zeitplan gespeichert: "+name {
		t.Fatal("translation modified persistent data")
	}
}

func TestNestedErrorAndDiscoveryTranslation(t *testing.T) {
	message := "Erreichbarkeitsprüfung ohne ICMP: Keine TCP-Antwort. Der Status ist unbekannt; eine Firewall kann die Prüfung blockieren."
	if want := "Reachability check without ICMP: No TCP response. The status is unknown; a firewall may be blocking the check."; translateMessage(message, "en") != want {
		t.Fatal(translateMessage(message, "en"))
	}
	r := httptest.NewRequest("GET", "/api.cgi", nil)
	r.Header.Set("Accept-Language", "en")
	w := httptest.NewRecorder()
	writeJSON(forRequest(w, r), 400, nil, errors.New("Name enthält Steuerzeichen."))
	if w.Code != 400 || w.Header().Get("Content-Language") != "en" || !strings.Contains(w.Body.String(), "The name contains control characters.") {
		t.Fatal(w.Body.String())
	}
	data := map[string]any{"devices": []FoundDevice{{IP: "192.168.1.9", Name: "LAN-Gerät (192.168.1.9)", Type: "LAN-Gerät"}}, "warnings": []string{"ICMP-Prüfung konnte nicht ausgeführt werden."}}
	encoded, _ := json.Marshal(localizeData(data, "en"))
	if !strings.Contains(string(encoded), "LAN device (192.168.1.9)") || !strings.Contains(string(encoded), "The ICMP check could not be performed.") {
		t.Fatal(string(encoded))
	}
}

func TestNotificationUsesRecipientLanguageTemplates(t *testing.T) {
	name := "Büro {0} <PC> 100%"
	for _, item := range []struct{ source, key string }{
		{"Magic Packets gesendet: {0}", "SynoWake:notification:wake_sent"},
		{"Aufwecken fehlgeschlagen: {0} – {1}", "SynoWake:notification:wake_failed"},
	} {
		args := notificationArguments(Log{MessageKey: item.source, MessageArgs: []string{name, "Gerät nicht gefunden."}})
		if len(args) != 8 || args[6] != item.key || args[7] != name {
			t.Fatalf("wrong notification arguments: %q", args)
		}
	}
}

func TestLifecycleLanguage(t *testing.T) {
	for _, item := range []struct{ value, want string }{
		{"ger", "SynoWake ist angehalten"}, {"enu", "SynoWake is stopped"}, {"", "SynoWake is stopped"},
	} {
		t.Setenv("SYNOPKG_DSM_LANGUAGE", item.value)
		if got := ErrorText(ErrStopped); got != item.want {
			t.Fatalf("%s: %s", item.value, got)
		}
	}
}
