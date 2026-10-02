package synowake

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type App struct {
	Root string
	Demo bool
}

func newStore() Store {
	return Store{Version: 1, CSRFSecret: randomHex(32), Devices: []Device{}, Schedules: []savedSchedule{}, Pending: []savedSchedule{}, Logs: []Log{}, Diagnostics: []Diagnostic{}}
}
func (a *App) readStore() (Store, error) {
	data, err := os.ReadFile(filepath.Join(a.Root, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return newStore(), nil
	}
	if err != nil {
		return Store{}, err
	}
	if len(data) > 4<<20 {
		return Store{}, errors.New("Datendatei ist zu groß.")
	}
	var s Store
	if err = json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("Datendatei beschädigt: %w", err)
	}
	if s.Version != 1 || s.CSRFSecret == "" {
		return s, errors.New("Unbekanntes Datenformat.")
	}
	return s, nil
}
func (a *App) update(f func(*Store) error) error {
	if err := os.MkdirAll(a.Root, 0700); err != nil {
		return err
	}
	unlock, err := lockStore(a.Root)
	if err != nil {
		return err
	}
	defer unlock()
	s, err := a.readStore()
	if err != nil {
		return err
	}
	if err = f(&s); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 4<<20 {
		return errors.New("Paketdaten überschreiten 4 MiB. Ausstehende Protokolle übertragen.")
	}
	file, err := os.CreateTemp(a.Root, ".state-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	destination := filepath.Join(a.Root, "state.json")
	deadline := time.Now().Add(time.Second)
	for {
		err := os.Rename(name, destination)
		if runtime.GOOS != "windows" || !os.IsPermission(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func addLog(s *Store, l Log) {
	if l.ID == "" {
		l.ID = randomID()
	}
	l.Time = time.Now().UTC()
	s.Logs = append([]Log{l}, s.Logs...)
	// Failed center deliveries must survive normal history retention.
	kept := make([]Log, 0, len(s.Logs))
	delivered := 0
	for _, item := range s.Logs {
		if item.CenterPending {
			kept = append(kept, item)
		} else if delivered < 300 {
			kept = append(kept, item)
			delivered++
		}
	}
	s.Logs = kept
}
func setDiagnostic(s *Store, code, message string) {
	next := []Diagnostic{}
	for _, d := range s.Diagnostics {
		if d.Code != code {
			next = append(next, d)
		}
	}
	if message != "" {
		next = append(next, Diagnostic{code, "warning", message})
	}
	s.Diagnostics = next
}
