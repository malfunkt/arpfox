package main

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCIDR(t *testing.T) {
	{
		_, ipnet, err := net.ParseCIDR("192.0.2.0/24")
		if err != nil {
			t.Fatal(err)
		}
		out := expandIPRange(cidrRange(ipnet))
		assert.Equal(t, int(0xffffffff-0xffffff00), len(out)-1)
		for i := 0; i < 256; i++ {
			assert.Equal(t, net.IP([]byte{192, 0, 2, byte(i)}), out[i])
		}
	}

	{
		_, ipnet, err := net.ParseCIDR("192.0.2.40/24")
		if err != nil {
			t.Fatal(err)
		}
		out := expandIPRange(cidrRange(ipnet))
		assert.Equal(t, int(0xffffffff-0xffffff00), len(out)-1)
		for i := 0; i < 256; i++ {
			assert.Equal(t, net.IP([]byte{192, 0, 2, byte(i)}), out[i])
		}
	}

	{
		_, ipnet, err := net.ParseCIDR("10.1.2.3/16")
		if err != nil {
			t.Fatal(err)
		}
		out := expandIPRange(cidrRange(ipnet))
		assert.Equal(t, int(0xffffffff-0xffff0000), len(out)-1)
		for i := 0; i < 65536; i++ {
			assert.Equal(t, net.IP([]byte{10, 1, byte(i / 256), byte(i % 256)}), out[i])
		}
	}

	{
		_, ipnet, err := net.ParseCIDR("10.1.2.3/32")
		if err != nil {
			t.Fatal(err)
		}
		out := expandIPRange(cidrRange(ipnet))
		assert.Equal(t, 0, len(out)-1)
		assert.Equal(t, net.IP([]byte{10, 1, 2, 3}), out[0])
	}
}

func TestIPRange(t *testing.T) {
	{
		out := expandIPRange(octectRange("192.168.1.1-254"))

		assert.Equal(t, 254, len(out))
		for i := 0; i < 254; i++ {
			assert.Equal(t, net.IP([]byte{192, 168, 1, 1 + byte(i)}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("192.168.1-254.1"))

		assert.Equal(t, 254, len(out))
		for i := 0; i < 254; i++ {
			assert.Equal(t, net.IP([]byte{192, 168, 1 + byte(i), 1}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("192.0-255.1.1"))

		assert.Equal(t, 256, len(out))
		for i := 0; i < 256; i++ {
			assert.Equal(t, net.IP([]byte{192, byte(i), 1, 1}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("0-255.1.1.1"))

		assert.Equal(t, 256, len(out))
		for i := 0; i < 256; i++ {
			assert.Equal(t, net.IP([]byte{byte(i), 1, 1, 1}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("0-255.255-0.1.1"))

		assert.Equal(t, 65536, len(out))
		for i := 0; i < 65536; i++ {
			assert.Equal(t, net.IP([]byte{byte(i / 256), byte(i % 256), 1, 1}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("1.255-0.1.255-0"))

		assert.Equal(t, 65536, len(out))
		for i := 0; i < 65536; i++ {
			assert.Equal(t, net.IP([]byte{1, byte(i / 256), 1, byte(i % 256)}), out[i])
		}
	}

	{
		out := expandIPRange(octectRange("1-2.3-4.5-6.7-8"))

		assert.Equal(t, 16, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{1, 3, 5, 7}),
			net.IP([]byte{1, 3, 5, 8}),
			net.IP([]byte{1, 3, 6, 7}),
			net.IP([]byte{1, 3, 6, 8}),
			net.IP([]byte{1, 4, 5, 7}),
			net.IP([]byte{1, 4, 5, 8}),
			net.IP([]byte{1, 4, 6, 7}),
			net.IP([]byte{1, 4, 6, 8}),
			net.IP([]byte{2, 3, 5, 7}),
			net.IP([]byte{2, 3, 5, 8}),
			net.IP([]byte{2, 3, 6, 7}),
			net.IP([]byte{2, 3, 6, 8}),
			net.IP([]byte{2, 4, 5, 7}),
			net.IP([]byte{2, 4, 5, 8}),
			net.IP([]byte{2, 4, 6, 7}),
			net.IP([]byte{2, 4, 6, 8}),
		})
	}
}

func TestIPList(t *testing.T) {
	{
		out := expandIPRange(ipList("192.168.1.1, 10.1.1 .1,1.2.3 .4,6.7.8.9"))

		assert.Equal(t, 4, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{192, 168, 1, 1}),
			net.IP([]byte{10, 1, 1, 1}),
			net.IP([]byte{1, 2, 3, 4}),
			net.IP([]byte{6, 7, 8, 9}),
		})
	}
}

func TestParseIPArgument(t *testing.T) {
	{
		ips, err := parseIPArgument("192.168.1.1")
		assert.NoError(t, err)
		out := expandIPRange(ips)
		assert.Equal(t, 1, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{192, 168, 1, 1}),
		})
	}

	{
		ips, err := parseIPArgument("192.168.1.1/32")
		assert.NoError(t, err)
		out := expandIPRange(ips)
		assert.Equal(t, 1, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{192, 168, 1, 1}),
		})
	}

	{
		ips, err := parseIPArgument("192.168.1.1, 1.2.3.4, 5.6.7.8")
		assert.NoError(t, err)
		out := expandIPRange(ips)
		assert.Equal(t, 3, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{192, 168, 1, 1}),
			net.IP([]byte{1, 2, 3, 4}),
			net.IP([]byte{5, 6, 7, 8}),
		})
	}

	{
		ips, err := parseIPArgument("192.168.1.1-5")
		assert.NoError(t, err)
		out := expandIPRange(ips)
		assert.Equal(t, 5, len(out))
		assert.Equal(t, out, []net.IP{
			net.IP([]byte{192, 168, 1, 1}),
			net.IP([]byte{192, 168, 1, 2}),
			net.IP([]byte{192, 168, 1, 3}),
			net.IP([]byte{192, 168, 1, 4}),
			net.IP([]byte{192, 168, 1, 5}),
		})
	}

}
