package arp

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
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
	ping := exec.Command("cmd", "/C", "ping -n 1", ip)
	ping.Run()

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
		if len(data.Addresses) == 0 {
			continue
		}
		if strings.Contains(data.Addresses[1].IP.String(), ip[0:6]) {
			return data.Name
		}
	}
	return ""
}

func BindArpByMyself(ip string) error {
	macs, nCardIndexs := getMacAddrs()
	ips := getIPs()

	for i, data := range ips {
		if strings.Contains(data, ip[0:9]) {
			if checkBind, _ := dowinARPLookup(data); checkBind == nil {
				bmac := strings.Replace(macs[i], ":", "-", -1)
				cmdExec := "netsh interface ipv4 add neighbors \"" + strconv.Itoa(nCardIndexs[i]) + "\" " + data + " " + bmac
				//need administartors privileges
				exec.Command("cmd", "/C", cmdExec).Output()
				return nil
			} else {
				return nil
			}
		}
	}
	return errors.New("bind arp error")
}

//bind ip and mac by myself
func getMacAddrs() (macAddrs []string, netCards []int) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("fail to get net interfaces: %v", err)
		return macAddrs, nil
	}

	for _, netInterface := range netInterfaces {
		macAddr := netInterface.HardwareAddr.String()
		netCardIndex := netInterface.Index
		if len(macAddr) == 0 {
			continue
		}

		macAddrs = append(macAddrs, macAddr)
		netCards = append(netCards, netCardIndex)
	}
	return macAddrs, netCards
}

func getIPs() (ips []string) {

	interfaceAddr, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Printf("fail to get net interface addrs: %v", err)
		return ips
	}

	for _, address := range interfaceAddr {
		ipNet, isValidIpNet := address.(*net.IPNet)
		if isValidIpNet && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	return ips
}
