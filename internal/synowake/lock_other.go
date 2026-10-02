//go:build !linux

package synowake

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

func lockStore(root string) (func(), error) {
	path := filepath.Join(root, ".lock-dir")
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Mkdir(path, 0700)
		if err == nil {
			return func() { os.Remove(path) }, nil
		}
		// Windows may report access denied briefly while another process removes
		// its lock directory; retry within the same bounded deadline.
		if !os.IsExist(err) && !os.IsPermission(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, errors.New("Daten gesperrt")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
