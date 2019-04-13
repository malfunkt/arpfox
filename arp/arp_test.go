package arp

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

func TestArp(t *testing.T) {
	ip := net.IP{10, 0, 0, 1}
	addr, err := Lookup(binary.BigEndian.Uint32(ip))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("addr: %v", addr)
}

func TestWinArp(t *testing.T) {
	addr, err := dowinARPLookup("192.168.3.102")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(addr)
}
