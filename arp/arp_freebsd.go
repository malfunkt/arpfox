// +build freebsd

package arp

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
)

var lineMatch = regexp.MustCompile(`^.+\(([0-9\.]+)\) at ([0-9a-f:]+) on ([^\s]+).+`)

func hexToInt(h string) uint8 {
	v, _ := strconv.ParseInt(h, 16, 16)
	return uint8(v)
}

func doARPLookup(ip string) (*Address, error) {
	ping := exec.Command("ping", "-c1", "-t1", ip)
	if err := ping.Start(); err != nil {
		return nil, err
	}

	if err := ping.Wait(); err != nil {
		return nil, err
	}

	cmd := exec.Command("arp", "-n", ip)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("No entry")
	}

	matches := lineMatch.FindAllStringSubmatch(string(out), 1)
	if len(matches) > 0 && len(matches[0]) > 3 {
		ipAddr := net.ParseIP(matches[0][1])

		macAddr, err := net.ParseMAC(matches[0][2])
		if err != nil {
			return nil, fmt.Errorf("ParseMAC: %v", err)
		}

		iface, err := net.InterfaceByName(matches[0][3])
		if err != nil {
			return nil, fmt.Errorf("InterfaceByName: %v", err)
		}

		localAddr := Address{
			IP:           ipAddr,
			HardwareAddr: macAddr,
			Interface:    *iface,
		}
		return &localAddr, nil
	}
	return nil, errors.New("Lookup failed.")
}
