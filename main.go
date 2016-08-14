package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/xiam/arpspoof/arp"
)

var (
	flagInterface = flag.String("i", "eth0", "Interface.")
	flagTarget    = flag.String("t", "", "Target host")
)

func main() {
	flag.Parse()

	args := flag.Args()
	log.Printf("args: %v", args)

	iface, err := net.InterfaceByName(*flagInterface)
	if err != nil {
		log.Fatal(err)
	}

	handler, err := pcap.OpenLive(iface.Name, 65535, true, pcap.BlockForever)
	if err != nil {
		log.Fatal(err)
	}
	defer handler.Close()

	var targetAddrs *net.IPNet

	var ifaceAddr *net.IPNet
	ifaceAddrs, err := iface.Addrs()
	if err != nil {
		log.Fatal(err)
	}

	for _, addr := range ifaceAddrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				ifaceAddr = &net.IPNet{
					IP:   ip4,
					Mask: ipnet.Mask[len(ipnet.Mask)-4:],
				}
				targetAddrs = &net.IPNet{}
				*targetAddrs = *ifaceAddr
				break
			}
		}
	}

	if ifaceAddr == nil {
		log.Fatal("Could not get interface address.")
	}

	if *flagTarget != "" {
		targetAddr := net.ParseIP(*flagTarget)
		if err != nil {
			log.Fatal(err)
		}
		targetAddrs = &net.IPNet{
			IP:   targetAddr.To4(),
			Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0x00),
		}
		if targetAddrs.IP[3] > 0 {
			targetAddrs.Mask = net.IPv4Mask(0xff, 0xff, 0xff, 0xff-1)
		}
	}

	src := arp.ARPAddress{
		IP:           ifaceAddr.IP,
		HardwareAddr: iface.HardwareAddr,
	}

	stop := make(chan struct{})
	go readARP(handler, iface, stop)

	for _, ip := range ips(targetAddrs) {

		dst := arp.ARPAddress{
			IP:           ip,
			HardwareAddr: net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		}

		log.Printf("Sending packet to: %v", dst.IP)

		buf, err := arp.NewARPRequest(&src, &dst)
		if err != nil {
			log.Fatal(err)
		}

		if err := handler.WritePacketData(buf); err != nil {
			log.Print("WritePacketData: ", err)
		}
	}

	time.Sleep(time.Second * 600)
}

func readARP(handle *pcap.Handle, iface *net.Interface, stop chan struct{}) {
	src := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	in := src.Packets()
	for {
		var packet gopacket.Packet
		select {
		case <-stop:
			return
		case packet = <-in:
			arpLayer := packet.Layer(layers.LayerTypeARP)
			if arpLayer == nil {
				continue
			}
			packet := arpLayer.(*layers.ARP)
			if packet.Operation != layers.ARPReply || bytes.Equal([]byte(iface.HardwareAddr), packet.SourceHwAddress) {
				//if bytes.Equal([]byte(iface.HardwareAddr), arp.SourceHwAddress) {
				// This is a packet I sent.
				continue
			}
			// Note:  we might get some packets here that aren't responses to ones we've sent,
			// if for example someone else sends US an ARP request.  Doesn't much matter, though...
			// all information is good information :)
			arp.Add(net.IP(packet.SourceProtAddress), net.HardwareAddr(packet.SourceHwAddress))
			log.Printf("IP %v is at %v", net.IP(packet.SourceProtAddress), net.HardwareAddr(packet.SourceHwAddress))
		}
	}
}

func ips(n *net.IPNet) (out []net.IP) {
	num := binary.BigEndian.Uint32([]byte(n.IP))
	mask := binary.BigEndian.Uint32([]byte(n.Mask))
	num &= mask
	for mask < 0xffffffff {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], num)
		out = append(out, net.IP(buf[:]))
		mask += 1
		num += 1
	}
	return
}
