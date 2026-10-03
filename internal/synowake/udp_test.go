package synowake

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestMagicPacketBurstSpacingAndCount(t *testing.T) {
	packet, err := magicPacket("02:11:22:33:44:55")
	if err != nil {
		t.Fatal(err)
	}
	times := []time.Time{}
	err = sendMagicPackets(packet, func(payload []byte) error {
		if !bytes.Equal(payload, packet) {
			t.Fatal("burst changed the magic packet")
		}
		times = append(times, time.Now())
		return nil
	}, time.Sleep)
	if err != nil || len(times) != 3 {
		t.Fatalf("expected exactly three packets: %d, %v", len(times), err)
	}
	for i := 1; i < len(times); i++ {
		if gap := times[i].Sub(times[i-1]); gap < 20*time.Millisecond {
			t.Fatalf("packet interval is too short: %v", gap)
		}
	}
}

func TestMagicPacketBurstStopsOnSendFailure(t *testing.T) {
	want := errors.New("simulated send failure")
	sends, pauses := 0, 0
	err := sendMagicPackets([]byte{1}, func([]byte) error {
		sends++
		return want
	}, func(time.Duration) { pauses++ })
	if !errors.Is(err, want) || sends != 1 || pauses != 0 {
		t.Fatalf("failed send retried: %d sends, %d pauses, %v", sends, pauses, err)
	}
}

func TestWakeBurstHasOneExecutionLog(t *testing.T) {
	a := testApp(t)
	id := randomID()
	if err := a.update(func(s *Store) error {
		s.Devices = append(s.Devices, Device{ID: id, Name: "Büro {0} <PC>", IP: "192.168.1.20", MAC: "02:11:22:33:44:55"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.wakeDevice(id, false, "manual", ""); err != nil {
		t.Fatal(err)
	}
	s := testStore(t, a)
	if len(s.Logs) != 1 || s.Logs[0].Source != "manual" || s.Logs[0].Level != "info" {
		t.Fatalf("burst created several execution logs: %+v", s.Logs)
	}
	if want := "Magic Packets sent: Büro {0} <PC>"; logMessage(s.Logs[0], "en") != want {
		t.Fatalf("localized log changed user name: %+v", s.Logs[0])
	}
}
