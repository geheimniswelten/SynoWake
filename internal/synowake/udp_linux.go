//go:build linux

package synowake

import (
	"net"
	"syscall"
	"time"
)

func sendMagic(ip string, port int, packet []byte) error {
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return err
	}
	defer c.Close()
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var optionErr error
	if err = raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	if optionErr != nil {
		return optionErr
	}
	c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return sendMagicPackets(packet, func(payload []byte) error {
		_, err := c.WriteToUDP(payload, &net.UDPAddr{IP: net.ParseIP(ip), Port: port})
		return err
	}, time.Sleep)
}
