//go:build !linux

package synowake

import (
	"errors"
)

func sendMagic(ip string, port int, packet []byte) error {
	return errors.New("Wake-on-LAN ist für den DSM/Linux-Build implementiert.")
}
