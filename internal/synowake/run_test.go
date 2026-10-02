package synowake

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunLifecyclePersistsAndHonorsPackageStop(t *testing.T) {
	for _, name := range []string{"PATH", "LD_PRELOAD", "LD_LIBRARY_PATH", "GCONV_PATH", "BASH_ENV", "ENV", "PYTHONPATH"} {
		t.Setenv(name, os.Getenv(name))
	}
	root, err := os.MkdirTemp("", "sw-lifecycle-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	t.Setenv("SYNOWAKE_VAR", root)
	socket := filepath.Join(root, "runtime", "backend.sock")
	t.Setenv("SYNOWAKE_SOCKET", socket)
	t.Setenv("GATEWAY_INTERFACE", "")
	if err := Run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"status"}); !errors.Is(err, ErrStopped) {
		t.Fatalf("initial status=%v", err)
	}
	app := &App{Root: root}
	done := make(chan error, 1)
	go func() { done <- app.serveBackend(socket) }()
	t.Cleanup(func() {
		app.stopBackend(socket)
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("service did not stop")
		}
	})
	waitBackend(t, socket)
	if err := Run([]string{"start"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	if !testStore(t, &App{Root: root}).Active {
		t.Fatal("start did not persist active state")
	}
	if err := Run([]string{"stop"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"status"}); !errors.Is(err, ErrStopped) {
		t.Fatalf("stopped status=%v", err)
	}
	if testStore(t, &App{Root: root}).Active {
		t.Fatal("stop did not persist inactive state")
	}
	for _, args := range [][]string{nil, {"unknown"}, {"--serve-demo", "0.0.0.0:1234", "."}, {"--run-schedule", "bad", "--token", "short", "--callback", "http://example.com"}} {
		if err := Run(args); err == nil {
			t.Fatalf("accepted invalid CLI command: %q", args)
		}
	}
}
