package synowake

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IP        string    `json:"ip"`
	MAC       string    `json:"mac"`
	Broadcast string    `json:"broadcast"`
	Port      int       `json:"port"`
	Favorite  bool      `json:"favorite"`
	LastWake  time.Time `json:"lastWake,omitempty"`
	WakeState string    `json:"wakeState,omitempty"`
}

type Schedule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DeviceID   string `json:"deviceId"`
	Time       string `json:"time"`
	Days       []int  `json:"days"`
	Enabled    bool   `json:"enabled"`
	Notify     bool   `json:"notify"`
	TaskID     int    `json:"taskId"`
	TaskOwner  string `json:"taskOwner"`
	Token      string `json:"-"`
	LastMinute string `json:"lastMinute,omitempty"`
}

// Secrets have a separate persistence representation and never enter UI state.
type savedSchedule struct {
	Schedule
	Secret   string `json:"secret"`
	Callback string `json:"callback"`
}

type Log struct {
	ID            string    `json:"id"`
	Time          time.Time `json:"time"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	MessageKey    string    `json:"messageKey,omitempty"`
	MessageArgs   []string  `json:"messageArgs,omitempty"`
	Source        string    `json:"source"`
	DeviceID      string    `json:"deviceId,omitempty"`
	ScheduleID    string    `json:"scheduleId,omitempty"`
	CenterPending bool      `json:"centerPending"`
}
type Diagnostic struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}
type Store struct {
	Version       int             `json:"version"`
	Active        bool            `json:"active"`
	CSRFSecret    string          `json:"csrfSecret"`
	Devices       []Device        `json:"devices"`
	Schedules     []savedSchedule `json:"schedules"`
	Pending       []savedSchedule `json:"pending"`
	Logs          []Log           `json:"logs"`
	Diagnostics   []Diagnostic    `json:"diagnostics"`
	LogCenterPort int             `json:"logCenterPort"`
	DiscoveryCIDR string          `json:"discoveryCidr,omitempty"`
}

var safeID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var clockTime = regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):[0-5][0-9]$`)

func randomID() string { return randomHex(16) }
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func cleanName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 80 {
		return "", errors.New("Name muss 1 bis 80 Zeichen enthalten.")
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return "", errors.New("Name enthält Steuerzeichen.")
		}
	}
	return value, nil
}
func validateDevice(d *Device) error {
	networks, _ := localNetworks()
	return validateDeviceForNetworks(d, networks)
}
func validateDeviceForNetworks(d *Device, networks []LocalNetwork) error {
	var err error
	d.Name, err = cleanName(d.Name)
	if err != nil {
		return err
	}
	ip := lanIPv4(d.IP)
	if ip == nil || (!ip.IsPrivate() && !isOnLinkHost(ip, networks)) {
		return errors.New("IP muss eine private oder direkt an der NAS angeschlossene IPv4-LAN-Adresse sein.")
	}
	mac, err := net.ParseMAC(d.MAC)
	if err != nil || len(mac) != 6 || mac[0]&1 != 0 || strings.EqualFold(mac.String(), "00:00:00:00:00:00") {
		return errors.New("Ungültige MAC-Adresse: sechs Bytes einer Unicast-Adresse erforderlich.")
	}
	d.MAC = strings.ToUpper(mac.String())
	if d.Broadcast == "" {
		d.Broadcast = "255.255.255.255"
	}
	broadcast := lanIPv4(d.Broadcast)
	validBroadcast := d.Broadcast == "255.255.255.255" || (broadcast != nil && broadcast.IsPrivate())
	for _, local := range networks {
		validBroadcast = validBroadcast || (local.Broadcast != "" && d.Broadcast == local.Broadcast)
	}
	if !validBroadcast {
		return errors.New("Broadcast muss eine lokale NAS-Broadcast-Adresse, eine private IPv4-Adresse oder 255.255.255.255 sein.")
	}
	if d.Port == 0 {
		d.Port = 9
	}
	if d.Port < 1 || d.Port > 65535 {
		return errors.New("UDP-Port muss zwischen 1 und 65535 liegen.")
	}
	if d.ID != "" && !safeID.MatchString(d.ID) {
		return errors.New("Ungültige Geräte-ID.")
	}
	return nil
}
func validateSchedule(s *Schedule, store *Store) error {
	var err error
	s.Name, err = cleanName(s.Name)
	if err != nil {
		return err
	}
	if !clockTime.MatchString(s.Time) {
		return errors.New("Uhrzeit muss HH:mm sein.")
	}
	if len(s.Days) == 0 || len(s.Days) > 7 {
		return errors.New("Mindestens einen Wochentag wählen.")
	}
	seen := map[int]bool{}
	for _, d := range s.Days {
		if d < 0 || d > 6 || seen[d] {
			return errors.New("Ungültige Wochentage.")
		}
		seen[d] = true
	}
	if s.ID != "" && !safeID.MatchString(s.ID) {
		return errors.New("Ungültige Zeitplan-ID.")
	}
	for _, d := range store.Devices {
		if d.ID == s.DeviceID {
			return nil
		}
	}
	return errors.New("Gerät des Zeitplans nicht gefunden.")
}
func magicPacket(mac string) ([]byte, error) {
	addr, err := net.ParseMAC(mac)
	if err != nil || len(addr) != 6 {
		return nil, fmt.Errorf("ungültige MAC-Adresse")
	}
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 255
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], addr)
	}
	return packet, nil
}
