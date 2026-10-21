package model

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

func NormalizeAuditIP(ip string) string {
	host := strings.TrimSpace(ip)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	host = strings.TrimSpace(host)
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	return parsed.WithZone("").String()
}

// MaskAuditIP preserves enough network context for audit correlation without
// exposing a user's complete address.
func MaskAuditIP(ip string) string {
	normalized := NormalizeAuditIP(ip)
	if normalized == "" {
		return ""
	}
	parsed, err := netip.ParseAddr(normalized)
	if err != nil {
		lastDot := strings.LastIndex(normalized, ".")
		if lastDot > 0 {
			return normalized[:lastDot] + ".*"
		}
		return "masked"
	}
	if parsed.Is4() {
		ipv4 := parsed.As4()
		return fmt.Sprintf("%d.%d.%d.*", ipv4[0], ipv4[1], ipv4[2])
	}
	if parsed.Is4In6() {
		ipv4 := parsed.Unmap().As4()
		return fmt.Sprintf("::ffff:%d.%d.%d.*", ipv4[0], ipv4[1], ipv4[2])
	}
	ipv6 := parsed.As16()
	return fmt.Sprintf("%x:%x:*", int(ipv6[0])<<8|int(ipv6[1]), int(ipv6[2])<<8|int(ipv6[3]))
}
