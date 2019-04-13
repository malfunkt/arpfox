// Package arp provides utilities for looking up MAC/IP addresses on a LAN.
package arp

import (
	"encoding/binary"
	"net"
	"runtime"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var (
	table   map[uint32]net.HardwareAddr
	tableMu sync.RWMutex
)

var defaultSerializeOpts = gopacket.SerializeOptions{
	FixLengths:       true,
	ComputeChecksums: true,
}

func init() {
	table = make(map[uint32]net.HardwareAddr)
}

// Add adds a IP-MAC map to a runtime ARP table.
func Add(ip net.IP, hwaddr net.HardwareAddr) {
	tableMu.Lock()
	defer tableMu.Unlock()
	table[binary.BigEndian.Uint32(ip)] = hwaddr
}

// Delete removes an IP from the runtime ARP table.
func Delete(ip net.IP) {
	tableMu.Lock()
	defer tableMu.Unlock()
	delete(table, binary.BigEndian.Uint32(ip))
}

// List returns the current runtime ARP table.
func List() map[uint32]net.HardwareAddr {
	tableMu.RLock()
	defer tableMu.RUnlock()
	return table
}

// Address represents an ARP address.
type Address struct {
	IP           net.IP
	HardwareAddr net.HardwareAddr
	Interface    net.Interface
}

// Lookup returns the Address given an IP, if the IP is not found within the
// table, a lookup is attempted.
func Lookup(ip uint32) (*Address, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, ip)
	if hwaddr, ok := table[ip]; ok {
		return &Address{
			IP:           net.IP(b),
			HardwareAddr: hwaddr,
		}, nil
	}
	if runtime.GOOS != "windows" {
		return doARPLookup(net.IP(b).To4().String())
	}
	return dowinARPLookup(net.IP(b).To4().String())
}

// NewARPRequest creates a bew ARP packet of type "request.
func NewARPRequest(src *Address, dst *Address) ([]byte, error) {
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

// NewARPReply creates a new ARP packet of type "reply".
func NewARPReply(src *Address, dst *Address) ([]byte, error) {
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

// buildPacket creates an template ARP packet with the given source and
// destination.
func buildPacket(src *Address, dst *Address) (layers.Ethernet, layers.ARP, error) {
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
