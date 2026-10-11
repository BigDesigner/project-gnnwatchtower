package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
)

var commonPorts = []int{22, 80, 443, 902, 3389, 5000, 5001, 8080}

// ScanSubnet scans a CIDR block (e.g., 192.168.1.0/24) and identifies active devices
func ScanSubnet(cidr string) ([]models.DiscoveryHost, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	var ips []net.IP
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		// skip network address and broadcast address in IPv4 /24
		dup := make(net.IP, len(ip))
		copy(dup, ip)
		ips = append(ips, dup)
	}

	if len(ips) > 2 {
		// remove network (.0) and broadcast (.255)
		ips = ips[1 : len(ips)-1]
	}

	// Fetch existing IPs from DB to tag them
	existingIPs := getExistingIPs()

	var results []models.DiscoveryHost
	var mu sync.Mutex

	// Worker pool for concurrency
	sem := make(chan struct{}, 64) // 64 parallel scanners
	var wg sync.WaitGroup

	for _, targetIP := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(tip net.IP) {
			defer wg.Done()
			defer func() { <-sem }()

			openPorts := probePorts(ipStr, commonPorts, 250*time.Millisecond)

			if len(openPorts) > 0 || pingHost(ipStr, 200*time.Millisecond) {
				host := models.DiscoveryHost{
					IPAddress:           ipStr,
					OpenPorts:           openPorts,
					SuggestedDeviceType: classifyDevice(openPorts),
					AlreadyMonitored:    existingIPs[ipStr],
				}

				// Fast reverse DNS lookup with 200ms timeout context
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				names, err := net.DefaultResolver.LookupAddr(ctx, ipStr)
				cancel()
				if err == nil && len(names) > 0 {
					host.Hostname = names[0]
				}

				mu.Lock()
				results = append(results, host)
				mu.Unlock()
			}
		}(targetIP)
	}

	wg.Wait()
	return results, nil
}

func probePorts(ip string, ports []int, timeout time.Duration) []int {
	var open []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			addr := fmt.Sprintf("%s:%d", ip, p)
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				_ = conn.Close()
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()
	return open
}

func pingHost(ip string, timeout time.Duration) bool {
	for _, p := range []int{80, 443, 22} {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, p), timeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

func classifyDevice(openPorts []int) string {
	hasPort := func(p int) bool {
		for _, port := range openPorts {
			if port == p {
				return true
			}
		}
		return false
	}

	if hasPort(5000) || hasPort(5001) {
		return "synology"
	}
	if hasPort(902) {
		return "esxi"
	}
	if hasPort(3389) {
		return "windows"
	}
	if hasPort(22) {
		return "ubuntu"
	}
	if hasPort(80) || hasPort(443) {
		return "other"
	}
	return "other"
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func getExistingIPs() map[string]bool {
	existing := make(map[string]bool)
	if database.DB == nil {
		return existing
	}

	rows, err := database.DB.Query("SELECT ip_address FROM devices")
	if err != nil {
		return existing
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err == nil {
			existing[ip] = true
		}
	}
	return existing
}
