package arp

import (
	"net"
	"testing"
)

func SkipTestArp(t *testing.T) {
	ip := net.IP{10, 0, 0, 1}
	addr, err := Lookup(ip)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("addr: %v", addr)
}
