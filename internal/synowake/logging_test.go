package synowake

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogCenterFailedDeliveryRemainsPendingAndRetriesLocally(t *testing.T) {
	if _, err := os.Stat("/usr/syno/bin/synologset1"); err == nil {
		t.Skip("integration test requires absent native DSM logger to force isolated TCP fallback")
	}
	a := testApp(t)
	a.Demo = false
	if err := a.event(Log{Level: "error", Source: "schedule", Message: "Packet gesendet\r\nzweite Zeile"}, false); err != nil {
		t.Fatal(err)
	}
	s := testStore(t, a)
	if len(s.Logs) != 1 || !s.Logs[0].CenterPending || len(s.Diagnostics) != 1 || s.Diagnostics[0].Code != "log-center" {
		t.Fatalf("failed Log Center delivery was lost: %+v", s)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := a.update(func(s *Store) error { s.LogCenterPort = port; return nil }); err != nil {
		t.Fatal(err)
	}
	line := make(chan string, 1)
	readError := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			readError <- err
			return
		}
		defer connection.Close()
		connection.SetReadDeadline(time.Now().Add(5 * time.Second))
		message, err := bufio.NewReader(connection).ReadString('\n')
		if err != nil {
			readError <- err
			return
		}
		line <- message
	}()
	result, err := a.retryCenter()
	if err != nil || result["attempted"] != 1 || result["remaining"] != 0 {
		t.Fatalf("retry=%v error=%v", result, err)
	}
	select {
	case message := <-line:
		if !strings.HasPrefix(message, "<11>") || !strings.Contains(message, "SynoWake: Packet gesendet  zweite Zeile") || strings.Count(message, "\n") != 1 {
			t.Fatalf("incorrect syslog framing or newline injection: %q", message)
		}
	case err := <-readError:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("local syslog receiver got no message")
	}
	s = testStore(t, a)
	if len(s.Logs) != 1 || s.Logs[0].CenterPending || len(s.Diagnostics) != 0 {
		t.Fatalf("successful retry did not clear delivery diagnostic: %+v", s)
	}
}

func TestLogCenterRetriesBoundedBatchWithoutDiscardingRemainingEvents(t *testing.T) {
	if _, err := os.Stat("/usr/syno/bin/synologset1"); err == nil {
		t.Skip("integration test requires absent native DSM logger")
	}
	a := testApp(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := a.update(func(s *Store) error {
		s.LogCenterPort = listener.Addr().(*net.TCPAddr).Port
		for i := 0; i < 23; i++ {
			addLog(s, Log{CenterPending: true, Level: "info", Source: "schedule", Message: fmt.Sprintf("Event %d", i)})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	readError := make(chan error, 1)
	received := make(chan bool, 1)
	go func() {
		for i := 0; i < 20; i++ {
			connection, err := listener.Accept()
			if err != nil {
				readError <- err
				return
			}
			connection.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, err = bufio.NewReader(connection).ReadString('\n')
			connection.Close()
			if err != nil {
				readError <- err
				return
			}
		}
		received <- true
	}()
	result, err := a.retryCenter()
	if err != nil || result["attempted"] != 20 || result["remaining"] != 3 {
		t.Fatalf("bounded retry=%v error=%v", result, err)
	}
	select {
	case <-received:
	case err := <-readError:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("local syslog batch did not complete")
	}
	s := testStore(t, a)
	if len(s.Logs) != 23 || len(s.Diagnostics) != 1 || s.Diagnostics[0].Code != "log-center" {
		t.Fatal("bounded retry discarded records or hid pending delivery")
	}
}

func TestLogRetentionPreservesEveryPendingDelivery(t *testing.T) {
	s := newStore()
	for i := 0; i < 350; i++ {
		addLog(&s, Log{CenterPending: true, Message: fmt.Sprintf("Pending %d", i)})
		addLog(&s, Log{Message: fmt.Sprintf("Delivered %d", i)})
	}
	pending, delivered := 0, 0
	for _, l := range s.Logs {
		if l.CenterPending {
			pending++
		} else {
			delivered++
		}
	}
	if pending != 350 || delivered != 300 || len(s.Logs) != 650 {
		t.Fatalf("retention dropped pending deliveries or retained excess history: pending=%d delivered=%d total=%d", pending, delivered, len(s.Logs))
	}
	if s.Logs[0].Message != "Delivered 349" || s.Logs[len(s.Logs)-1].Message != "Pending 0" {
		t.Fatal("retention lost newest delivered history or oldest pending record")
	}
}

func TestFullLogQueueBlocksWakeBeforeChangingDevice(t *testing.T) {
	a := testApp(t)
	d := testDevice(t, a)
	if err := a.update(func(s *Store) error {
		for i := 0; i < 3000; i++ {
			addLog(s, Log{CenterPending: true, Message: "Pending"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.wakeDevice(d.ID, false, "manual", ""); err == nil || !strings.Contains(err.Error(), "Warteschlange voll") {
		t.Fatalf("full queue did not reject wake: %v", err)
	}
	s := testStore(t, a)
	if !s.Devices[0].LastWake.IsZero() || s.Devices[0].WakeState != "" || len(s.Logs) != 3000 {
		t.Fatal("blocked wake modified device or discarded a pending delivery")
	}
}

func TestFailedWakeUpdatesSingleDurableAttemptLog(t *testing.T) {
	if _, err := os.Stat("/usr/syno/bin/synologset1"); err == nil {
		t.Skip("isolated test requires absent DSM logger")
	}
	a := testApp(t)
	d := testDevice(t, a)
	if err := a.update(func(s *Store) error { s.Devices[0].MAC = "invalid"; return nil }); err != nil {
		t.Fatal(err)
	}
	a.Demo = false
	if err := a.wakeDevice(d.ID, false, "schedule", "test-plan"); err == nil {
		t.Fatal("invalid wire destination unexpectedly succeeded")
	}
	s := testStore(t, a)
	if s.Devices[0].LastWake.IsZero() || s.Devices[0].WakeState != "failed" || len(s.Logs) != 1 || s.Logs[0].Level != "error" || s.Logs[0].Source != "schedule" || s.Logs[0].ScheduleID != "test-plan" || !s.Logs[0].CenterPending || !strings.Contains(s.Logs[0].Message, "fehlgeschlagen") {
		t.Fatalf("wake failure lost durable execution record: %+v", s)
	}
}

func TestOversizedStoreTransactionPreservesReadableCommittedState(t *testing.T) {
	a := testApp(t)
	before := testStore(t, a)
	err := a.update(func(s *Store) error { s.Logs = append(s.Logs, Log{Message: strings.Repeat("X", 4<<20)}); return nil })
	if err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatalf("oversized transaction was not rejected: %v", err)
	}
	after := testStore(t, a)
	if after.CSRFSecret != before.CSRFSecret || len(after.Logs) != 0 || after.Active != before.Active {
		t.Fatal("oversized update corrupted committed state")
	}
}

func TestNotificationFailureIsVisibleAndPreservesExecutionLog(t *testing.T) {
	if _, err := os.Stat("/usr/syno/bin/synodsmnotify"); err == nil {
		t.Skip("isolated test requires absent DSM notifier")
	}
	for _, notify := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", notify), func(t *testing.T) {
			a := testApp(t)
			a.Demo = false
			id := randomID()
			message := "Magic Packet gesendet: Büro <PC> 100%"
			if err := a.event(Log{ID: id, Level: "info", Source: "schedule", Message: message}, notify); err != nil {
				t.Fatal(err)
			}
			s := testStore(t, a)
			foundExecution, notificationDiagnostic, notificationWarning := false, false, false
			for _, entry := range s.Logs {
				if entry.ID == id && entry.Message == message && entry.Source == "schedule" {
					foundExecution = true
				}
				if entry.Source == "integration" && entry.Level == "warning" && strings.Contains(entry.Message, "DSM-Benachrichtigung fehlgeschlagen") {
					notificationWarning = true
				}
			}
			for _, diagnostic := range s.Diagnostics {
				if diagnostic.Code == "notification" && strings.Contains(diagnostic.Message, "synodsmnotify") {
					notificationDiagnostic = true
				}
			}
			if !foundExecution || notificationDiagnostic != notify || notificationWarning != notify {
				t.Fatalf("execution record or notification option/error handling incorrect: %+v", s)
			}
		})
	}
}
