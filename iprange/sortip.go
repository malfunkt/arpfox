package iprange

import "net"

// Asc implements sorting in ascending order for IP addresses
type Asc []net.IP

func (a Asc) Len() int {
	return len(a)
}

func (a Asc) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a Asc) Less(i, j int) bool {
	if a[i][0] < a[j][0] {
		return true
	}

	if a[i][0] == a[j][0] && a[i][1] < a[j][1] {
		return true
	}

	if a[i][0] == a[j][0] && a[i][1] == a[j][1] &&
		a[i][2] < a[j][2] {
		return true
	}

	if a[i][0] == a[j][0] && a[i][1] == a[j][1] &&
		a[i][2] == a[j][2] && a[i][3] < a[j][3] {
		return true
	}

	return false
}
