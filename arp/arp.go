package arp

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var (
	table   map[string]net.HardwareAddr
	tableMu sync.RWMutex
)

var defaultSerializeOpts = gopacket.SerializeOptions{
	FixLengths:       true,
	ComputeChecksums: true,
}

func init() {
	table = make(map[string]net.HardwareAddr)
	go list()
}

func list() {
	for {
		time.Sleep(time.Second * 5)
		for ip, hwaddr := range List() {
			log.Printf("%v -> %v", ip, hwaddr)
		}
	}
}

func Add(ip net.IP, hwaddr net.HardwareAddr) {
	tableMu.Lock()
	defer tableMu.Unlock()
	table[ip.To4().String()] = hwaddr
}

func Delete(ip net.IP) {
	tableMu.Lock()
	defer tableMu.Unlock()
	delete(table, ip.To4().String())
}

func List() map[string]net.HardwareAddr {
	tableMu.RLock()
	defer tableMu.RUnlock()
	return table
}

type ARPAddress struct {
	IP           net.IP
	HardwareAddr net.HardwareAddr
	Interface    net.Interface
}

func Lookup(ip string) (*ARPAddress, error) {
	if hwaddr, ok := table[ip]; ok {
		return &ARPAddress{
			IP:           net.ParseIP(ip),
			HardwareAddr: hwaddr,
		}, nil
	}
	return doARPLookup(ip)
}

func NewARPRequest(src *ARPAddress, dst *ARPAddress) ([]byte, error) {
	ether, arp, err := buildPacket(src, dst)
	if err != nil {
		return nil, err
	}

	arp.Operation = layers.ARPRequest

	buf := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(buf, defaultSerializeOpts, &ether, &arp)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func NewARPReply(src *ARPAddress, dst *ARPAddress) ([]byte, error) {
	ether, arp, err := buildPacket(src, dst)
	if err != nil {
		return nil, err
	}

	arp.Operation = layers.ARPReply

	buf := gopacket.NewSerializeBuffer()

	err = gopacket.SerializeLayers(buf, defaultSerializeOpts, &ether, &arp)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildPacket(src *ARPAddress, dst *ARPAddress) (layers.Ethernet, layers.ARP, error) {
	ether := layers.Ethernet{
		EthernetType: layers.EthernetTypeARP,

		SrcMAC: src.HardwareAddr,
		DstMAC: dst.HardwareAddr,
	}
	arp := layers.ARP{
		AddrType: layers.LinkTypeEthernet,
		Protocol: layers.EthernetTypeIPv4,

		HwAddressSize:   6,
		ProtAddressSize: 4,

		SourceHwAddress:   []byte(src.HardwareAddr),
		SourceProtAddress: []byte(src.IP.To4()),

		DstHwAddress:   []byte(dst.HardwareAddr),
		DstProtAddress: []byte(dst.IP.To4()),
	}
	return ether, arp, nil
}
