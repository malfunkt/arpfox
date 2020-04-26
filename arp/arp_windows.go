// +build windows

package arp

import (
	"errors"
	"net"
	"os/exec"
	"strings"
)

func winTable() map[string]string {
	data, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil
	}

	var table = make(map[string]string)
	skipNext := false
	for _, line := range strings.Split(string(data), "\n") {
		// skip empty lines
		if len(line) <= 0 {
			continue
		}
		// skip Interface: lines
		if line[0] != ' ' {
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		ip := fields[0]
		// Normalize MAC address to colon-separated format
		table[ip] = strings.Replace(fields[1], "-", ":", -1)
	}

	return table
}

func doARPLookup(ip string) (*Address, error) {
	ping := exec.Command("cmd", "/C", "ping -n 1", ip)
	if err := ping.Start(); err != nil {
		return nil, err
	}

	if err := ping.Wait(); err != nil {
		return nil, err
	}

	arpAllTable := winTable()
	if hwaddr, ok := arpAllTable[ip]; ok {
		ha, _ := net.ParseMAC(hwaddr)
		return &Address{
			IP:           net.ParseIP(ip),
			HardwareAddr: ha,
			//Interface:
		}, nil
	}
	return nil, errors.New("Lookup failed.")
}
