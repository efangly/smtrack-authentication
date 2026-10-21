package middleware

import (
	"fmt"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// TrustedProxies is the set of networks (e.g. the KrakenD pods) whose
// X-Forwarded-For entries are believed when working out who a client is.
type TrustedProxies []*net.IPNet

// ParseTrustedProxies accepts CIDR ranges ("10.0.0.0/8") and plain addresses
// ("10.1.2.3"), which are treated as a single-host range.
func ParseTrustedProxies(entries []string) (TrustedProxies, error) {
	proxies := make(TrustedProxies, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted proxy %q", entry)
			}
			bits := 128
			if ip.To4() != nil {
				ip, bits = ip.To4(), 32
			}
			entry = fmt.Sprintf("%s/%d", ip, bits)
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q", entry)
		}
		proxies = append(proxies, network)
	}
	return proxies, nil
}

func (t TrustedProxies) contains(ip net.IP) bool {
	for _, network := range t {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP is the address the rate limiter keys on. It is the TCP peer unless
// that peer is a trusted proxy, in which case X-Forwarded-For is walked from
// the right — the end each proxy appends to — and the first hop that is not
// itself a trusted proxy is the client.
//
// The leftmost entry (what fiber.Config.ProxyHeader returns) is deliberately
// not used: a client can put anything there, so keying on it would let one
// client rotate through unlimited rate-limit buckets.
func (t TrustedProxies) ClientIP(c fiber.Ctx) string {
	return t.resolve(c.RequestCtx().RemoteIP(), c.Get(fiber.HeaderXForwardedFor))
}

func (t TrustedProxies) resolve(remote net.IP, forwardedFor string) string {
	if len(t) == 0 || remote == nil || !t.contains(remote) || forwardedFor == "" {
		return remote.String()
	}
	hops := strings.Split(forwardedFor, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			// A malformed hop means the chain can no longer be trusted past
			// this point; key on the proxy rather than guess.
			break
		}
		if !t.contains(ip) {
			return ip.String()
		}
	}
	return remote.String()
}
