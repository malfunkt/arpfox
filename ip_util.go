package main

import (
	"encoding/binary"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var ipListRe = regexp.MustCompile(`[^0-9\.,]`)

func parseIPArgument(ipargument string) (chan net.IP, error) {
	for i := 0; i < len(ipargument); i++ {
		switch ipargument[i] {
		case '/':
			_, ipnet, err := net.ParseCIDR(ipargument)
			if err != nil {
				return nil, err
			}
			return cidrRange(ipnet), nil
		case '-':
			return octectRange(ipargument), nil
		}
	}
	return ipList(ipargument), nil
}

func ipList(iplist string) chan net.IP {
	ipchan := make(chan net.IP, 1)

	go func() {
		defer close(ipchan)
		ips := strings.Split(ipListRe.ReplaceAllString(iplist, ""), ",")
		for i := range ips {
			ip := net.ParseIP(ips[i])
			if ip == nil {
				log.Fatalf("Invalid IPv4 in %q", iplist)
			}
			ipchan <- ip.To4()
		}
	}()

	return ipchan
}

func cidrRange(n *net.IPNet) chan net.IP {
	ip := binary.BigEndian.Uint32([]byte(n.IP))

	var mask uint32
	if n.Mask == nil {
		mask = 0xffffffff
	} else {
		mask = binary.BigEndian.Uint32([]byte(n.Mask))
	}

	lower := ip & mask
	upper := lower + 0xffffffff - mask

	var lowerIP [4]byte
	binary.BigEndian.PutUint32(lowerIP[:], lower)

	var upperIP [4]byte
	binary.BigEndian.PutUint32(upperIP[:], upper)

	return ipRange(net.IP(lowerIP[:]), net.IP(upperIP[:]))
}

func octectRange(ipaddr string) chan net.IP {

	chunks := strings.Split(ipaddr, ".")
	if len(chunks) != 4 {
		log.Fatalf("Not a valid ipv4 address %q.", ipaddr)
	}

	lower := [4]byte{}
	upper := [4]byte{}

	for i := range chunks {
		var min, max byte
		parts := strings.SplitN(chunks[i], "-", 2)
		if len(parts) == 1 {
			i, err := strconv.ParseInt(chunks[i], 10, 16)
			if err != nil {
				log.Fatalf("Not a valid ipv4 address %q: %v", ipaddr, err)
			}
			min, max = byte(i), byte(i)
		} else if len(parts) == 2 {
			minU64, err := strconv.ParseInt(parts[0], 10, 16)
			if err != nil {
				log.Fatalf("Invalid lower half of segment range %q on %q: %v", parts[0], ipaddr, err)
			}
			maxU64, err := strconv.ParseInt(parts[1], 10, 16)
			if err != nil {
				log.Fatalf("Invalid upper half of segment range %q on %q: %v", parts[1], ipaddr, err)
			}
			if minU64 > maxU64 {
				minU64, maxU64 = maxU64, minU64
			}
			min, max = byte(minU64), byte(maxU64)
		} else {
			log.Fatalf("Not a valid segment range %q on ip %q", chunks[i], ipaddr)
		}
		lower[i] = min
		upper[i] = max
	}

	return ipRange(net.IP(lower[:]), net.IP(upper[:]))
}

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
