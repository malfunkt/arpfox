package arp

import (
	"errors"
	"net"
	"os/exec"
	"strings"

	"github.com/google/gopacket/pcap"
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

func dowinARPLookup(ip string) (*Address, error) {
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

//GetRealCardName : windows real network card name like "\Device\NPF_{8D51979B-6048-4472-BBA9-379CF7C7A339}"
func GetRealCardName(ip string) string {
	devices, _ := pcap.FindAllDevs()
	for _, data := range devices {
		//fmt.Println(data)
		if strings.Contains(data.Addresses[1].IP.String(), ip[0:6]) {
			return data.Name
		}
	}
	return ""
}
