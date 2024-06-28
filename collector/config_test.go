package collector

import (
	"math/rand"
	"testing"

	"inet.af/netaddr"

	"github.com/stretchr/testify/assert"
)

func TestSelectIP(t *testing.T) {
	check := func(expected string, from ...string) {
		var ips []netaddr.IP
		for _, s := range from {
			ips = append(ips, netaddr.MustParseIP(s))
		}
		for i := 0; i < 100; i++ {
			rand.Shuffle(len(ips), func(i, j int) {
				ips[i], ips[j] = ips[j], ips[i]
			})
			assert.Equal(t, expected, SelectIP(ips).String())
		}
	}

	assert.Nil(t, SelectIP(nil))

	check("127.0.0.1", "127.0.0.1")
	check("192.168.0.1", "127.0.0.1", "192.168.0.1")
	check("192.168.0.1", "127.0.0.1", "192.168.0.2", "192.168.0.1")
	check("8.8.8.8", "127.0.0.1", "8.8.8.8")
	check("1.1.1.1", "127.0.0.1", "8.8.8.8", "1.1.1.1")
	check("192.168.0.1", "127.0.0.1", "8.8.8.8", "192.168.0.1")
	check("100.64.0.1", "127.0.0.1", "8.8.8.8", "192.168.0.1", "100.64.0.1")
	check("172.17.0.1", "127.0.0.1", "172.17.0.1", "172.17.0.3", "172.18.0.1")
}
