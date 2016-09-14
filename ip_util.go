package main

import (
	"encoding/binary"
	"net"
)

func ipRange(lower, upper net.IP) chan net.IP {
	ipchan := make(chan net.IP, 1)

	rangeMask := net.IP([]byte{
		upper[0] - lower[0],
		upper[1] - lower[1],
		upper[2] - lower[2],
		upper[3] - lower[3],
	})

	go func() {
		defer close(ipchan)

		lower32 := binary.BigEndian.Uint32([]byte(lower))
		upper32 := binary.BigEndian.Uint32([]byte(upper))
		diff := upper32 - lower32

		if diff < 0 {
			panic("Lower address is actually higher than upper address.")
		}

		mask := net.IP([]byte{0, 0, 0, 0})

		for {
			ipchan <- net.IP([]byte{
				lower[0] + mask[0],
				lower[1] + mask[1],
				lower[2] + mask[2],
				lower[3] + mask[3],
			})

			if mask.Equal(rangeMask) {
				break
			}

			for i := 3; i >= 0; i-- {
				if rangeMask[i] > 0 {
					if mask[i] < rangeMask[i] {
						mask[i] = mask[i] + 1
						break
					} else {
						mask[i] = mask[i] % rangeMask[i]
						if i < 1 {
							break
						}
					}
				}
			}
		}

	}()

	return ipchan
}

func expandIPRange(c chan net.IP) []net.IP {
	ips := []net.IP{}
	for ip := range c {
		ips = append(ips, ip)
	}
	return ips
}
