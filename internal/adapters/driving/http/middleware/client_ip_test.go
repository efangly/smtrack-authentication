package middleware

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrustedProxies(t *testing.T) {
	proxies, err := ParseTrustedProxies([]string{"10.0.0.0/8", " 192.168.1.5 ", "", "::1"})
	require.NoError(t, err)
	require.Len(t, proxies, 3)
	assert.True(t, proxies.contains(net.ParseIP("10.42.0.7")))
	assert.True(t, proxies.contains(net.ParseIP("192.168.1.5")))
	assert.False(t, proxies.contains(net.ParseIP("192.168.1.6")))
	assert.True(t, proxies.contains(net.ParseIP("::1")))

	for _, bad := range []string{"not-an-ip", "10.0.0.0/33", "1.2.3"} {
		_, err := ParseTrustedProxies([]string{bad})
		assert.Error(t, err, bad)
	}
}

func TestTrustedProxies_Resolve(t *testing.T) {
	proxies, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	require.NoError(t, err)
	gateway := net.ParseIP("10.0.0.2")

	tests := []struct {
		name    string
		proxies TrustedProxies
		remote  net.IP
		xff     string
		want    string
	}{
		{"no trusted proxies configured: peer, header ignored", nil, gateway, "203.0.113.9", "10.0.0.2"},
		{"untrusted peer: header ignored", proxies, net.ParseIP("198.51.100.1"), "203.0.113.9", "198.51.100.1"},
		{"trusted peer, single hop", proxies, gateway, "203.0.113.9", "203.0.113.9"},
		{"spoofed leftmost entry is ignored", proxies, gateway, "1.1.1.1, 203.0.113.9", "203.0.113.9"},
		{"internal proxy hops are skipped", proxies, gateway, "203.0.113.9, 10.0.0.5", "203.0.113.9"},
		{"trusted peer, no header", proxies, gateway, "", "10.0.0.2"},
		{"malformed hop stops the walk", proxies, gateway, "203.0.113.9, garbage", "10.0.0.2"},
		{"every hop trusted", proxies, gateway, "10.0.0.9, 10.0.0.5", "10.0.0.2"},
		{"ipv6 client", proxies, gateway, "2001:db8::1", "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.proxies.resolve(tt.remote, tt.xff))
		})
	}
}
