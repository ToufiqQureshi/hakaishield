package core

import "net"

// PublicEdgeIPv4 accepts only an address suitable for a customer DNS A record.
// The operator must still ensure that this address is stable and routes to the edge.
func PublicEdgeIPv4(raw string) bool {
	ip := net.ParseIP(raw)
	if ip == nil || ip.To4() == nil {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
