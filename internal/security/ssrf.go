package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// AssertPublicURL validates scheme/host against SSRF policy before outbound fetch.
func AssertPublicURL(pol *Policy, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must be http or https")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return fmt.Errorf("empty host")
	}

	if pol != nil {
		for _, blocked := range pol.Network.DenyLocalhostHostnames {
			if host == blocked || strings.HasSuffix(host, "."+blocked) {
				return fmt.Errorf("blocked host: %s", host)
			}
		}
		if pol.Network.Enabled {
			ok := false
			for _, allowed := range pol.Network.AllowlistHosts {
				if host == allowed || strings.HasSuffix(host, "."+allowed) {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("host %q not in network allowlist", host)
			}
		}
	} else if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("blocked host: %s", host)
	}

	denyPrivate := pol == nil || pol.Network.DenyPrivateIPs
	if !denyPrivate {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked IP literal: %s", host)
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no ips for host %s", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked private/loopback/metadata IP for host %s", host)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// AWS/GCP metadata and similar
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 0.0.0.0/8
		if ip4[0] == 0 {
			return true
		}
	}
	return false
}
