package targetparser

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleAddress(t *testing.T) {
	ipRange, err := Parse("192.168.1.1")
	assert.Nil(t, err)

	assert.True(t, ipRange.Min.Equal(net.IPv4(192, 168, 1, 1)))
	assert.True(t, ipRange.Max.Equal(net.IPv4(192, 168, 1, 1)))
}

func TestCIDRAddress(t *testing.T) {
	ipRange, err := Parse("192.168.1.1/24")
	assert.Nil(t, err)

	assert.True(t, ipRange.Min.Equal(net.IPv4(192, 168, 1, 0)))
	assert.True(t, ipRange.Max.Equal(net.IPv4(192, 168, 1, 255)))
}

func TestWildcardAddress(t *testing.T) {
	ipRange, err := Parse("192.168.1.*")
	assert.Nil(t, err)

	assert.True(t, ipRange.Min.Equal(net.IPv4(192, 168, 1, 0)))
	assert.True(t, ipRange.Max.Equal(net.IPv4(192, 168, 1, 255)))
}

func TestRangeAddress(t *testing.T) {
	ipRange, err := Parse("192.168.1.10-20")
	assert.Nil(t, err)

	assert.True(t, ipRange.Min.Equal(net.IPv4(192, 168, 1, 10)))
	assert.True(t, ipRange.Max.Equal(net.IPv4(192, 168, 1, 20)))
}

func TestMixedAddress(t *testing.T) {
	ipRange, err := Parse("192.168.10-20.*/25")
	assert.Nil(t, err)

	assert.True(t, ipRange.Min.Equal(net.IPv4(192, 168, 10, 0)))
	assert.True(t, ipRange.Max.Equal(net.IPv4(192, 168, 10, 127)))
}

func TestBadAddress(t *testing.T) {
	ipRange, err := Parse("192.168.10")
	assert.Nil(t, ipRange)
	assert.NotNil(t, err)
}
