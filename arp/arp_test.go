package arp

import (
	"encoding/binary"
	"net"
	"testing"
)

func SkipTestArp(t *testing.T) {
	ip := net.IP{10, 0, 0, 1}
	addr, err := Lookup(binary.BigEndian.Uint32(ip))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("addr: %v", addr)
}
