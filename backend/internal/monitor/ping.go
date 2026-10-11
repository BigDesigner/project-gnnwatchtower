package monitor

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var rttRegex = regexp.MustCompile(`(?:time|zaman)[=<]\s*([0-9.]+)\s*ms?`)

// PingHost performs a reachability probe using fast TCP ports and real OS ICMP ping fallback.
// Returns isOnline (bool) and responseTimeMs (float64).
func PingHost(ip string, timeout time.Duration) (bool, float64) {
	if ip == "" {
		return false, 0
	}

	// 1. Fast TCP probe on common management & network ports (53 DNS, 80 HTTP, 443 HTTPS, 22 SSH, 3389 RDP, 445 SMB, 8080 Web)
	tcpPorts := []int{53, 80, 443, 22, 3389, 445, 8080}
	start := time.Now()
	for _, port := range tcpPorts {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			return true, ms
		}
	}

	// 2. Real OS ICMP Ping (guaranteed reachability without raw socket / root privileges)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", strconv.Itoa(int(timeout.Milliseconds())), ip)
	} else {
		// Linux / Unix: -c 1 (1 packet), -W timeout in seconds (min 1 sec)
		timeoutSec := int(timeout.Seconds())
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(timeoutSec), ip)
	}

	out, err := cmd.CombinedOutput()
	if err == nil {
		outStr := strings.ToLower(string(out))
		// Check for reachability indicators
		if strings.Contains(outStr, "ttl=") || 
		   strings.Contains(outStr, "1 packets transmitted, 1 received") || 
		   strings.Contains(outStr, "1 received") || 
		   strings.Contains(outStr, "bytes from") {
			rtt := parseRTT(outStr)
			if rtt <= 0 {
				rtt = float64(time.Since(start).Microseconds()) / 1000.0
			}
			return true, rtt
		}
	}

	return false, 0
}

func parseRTT(output string) float64 {
	matches := rttRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return val
		}
	}
	// Fallback for "<1ms"
	if strings.Contains(output, "time<1ms") || strings.Contains(output, "time<1 ms") {
		return 0.8
	}
	return 0
}
