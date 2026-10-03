package synowake

import "time"

const magicPacketCount = 3
const magicPacketInterval = 20 * time.Millisecond

// A short burst is one wake action. There is no delayed retry based on status.
func sendMagicPackets(packet []byte, send func([]byte) error, pause func(time.Duration)) error {
	for i := 0; i < magicPacketCount; i++ {
		if i > 0 {
			pause(magicPacketInterval)
		}
		if err := send(packet); err != nil {
			return err
		}
	}
	return nil
}
