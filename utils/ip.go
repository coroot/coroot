package utils

import "inet.af/netaddr"

var (
	dockerNetwork = netaddr.MustParseIPPrefix("172.16.0.0/12")
)

func IsIpPrivate(ip netaddr.IP) bool {
	if ip.IsPrivate() {
		return true
	}
	if ip.Is4() {
		parts := ip.As4()
		return parts[0] == 100 && parts[1]&0xc0 == 64 // 100.64.0.0/10
	}
	return false
}

func IsIpDocker(ip netaddr.IP) bool {
	return dockerNetwork.Contains(ip)
}

func IsIpExternal(ip netaddr.IP) bool {
	return !ip.IsLoopback() && !ip.IsPrivate()
}
