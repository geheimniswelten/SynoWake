//go:build linux

package synowake

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func lockStore(root string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(root, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, errors.New("Daten werden gerade bearbeitet. Bitte erneut versuchen.")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
