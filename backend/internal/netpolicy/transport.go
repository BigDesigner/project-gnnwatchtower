// Package netpolicy binds destination validation to the actual outbound connection.
package netpolicy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func Transport() *http.Transport {
	return &http.Transport{Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 16, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(host, "metadata.google.internal") || strings.EqualFold(host, "instance-data") {
			return nil, fmt.Errorf("destination denied")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no destination addresses")
		}
		for _, a := range ips {
			if a.IP.IsLoopback() || a.IP.IsLinkLocalUnicast() || a.IP.IsLinkLocalMulticast() || a.IP.IsMulticast() || a.IP.IsUnspecified() || a.IP.Equal(net.IPv4bcast) {
				return nil, fmt.Errorf("destination denied")
			}
		}
		// Private unicast destinations remain permitted for explicitly configured customer services.
		dialer := net.Dialer{Timeout: 3 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
}
