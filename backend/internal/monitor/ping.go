package monitor

import (
	"context"
	"errors"
	"fmt"
	"gnnwatchtower/internal/models"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var rttRegex = regexp.MustCompile(`(?:time|zaman)=\s*([0-9.]+)\s*ms`)

// ProbeICMP sends one OS ping. No TCP fallback, retries, raw socket or privilege changes.
// Local execution failures are unknown, not evidence of an unreachable device.
func ProbeICMP(ip string, timeout time.Duration) (bool, *float64, error) {
	if !models.ValidIP(ip) {
		return false, nil, fmt.Errorf("invalid ICMP target")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+250*time.Millisecond)
	defer cancel()
	command := "ping"
	var args []string
	switch runtime.GOOS {
	case "windows":
		args = []string{"-n", "1", "-w", strconv.FormatInt(timeout.Milliseconds(), 10)}
		if net.ParseIP(ip).To4() == nil {
			args = append(args, "-6")
		}
	case "darwin":
		if net.ParseIP(ip).To4() == nil {
			command = "ping6"
			args = []string{"-c", "1"}
		} else {
			args = []string{"-c", "1", "-W", strconv.FormatInt(timeout.Milliseconds(), 10)}
		}
	default:
		args = []string{"-n", "-c", "1", "-W", strconv.Itoa(max(1, int(timeout.Seconds())))}
		if net.ParseIP(ip).To4() == nil {
			args = append(args, "-6")
		}
	}
	args = append(args, ip)
	out, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	output := strings.ToLower(string(out))
	for _, s := range []string{"operation not permitted", "permission denied", "not permitted", "general failure", "invalid option", "illegal option", "network is unreachable", "no route to host"} {
		if strings.Contains(output, s) {
			return false, nil, fmt.Errorf("local ICMP execution unavailable")
		}
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 1 || (runtime.GOOS == "darwin" && exit.ExitCode() == 2) || ctx.Err() != nil) {
			return false, nil, nil
		}
		if ctx.Err() != nil {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("ICMP command unavailable")
	}
	online := strings.Contains(output, "ttl=") || strings.Contains(output, "bytes from") || strings.Contains(output, "1 received") || strings.Contains(output, "1 packets received") || rttRegex.MatchString(output) || strings.Contains(output, "time<") || strings.Contains(output, "zaman<")
	if !online {
		return false, nil, nil
	}
	rtt := parseRTT(output)
	if rtt < 0 {
		return true, nil, nil
	}
	return true, &rtt, nil
}

// Compatibility for existing gateway callers. Negative RTT means unmeasured.
func PingHost(ip string, timeout time.Duration) (bool, float64) {
	online, rtt, _ := ProbeICMP(ip, timeout)
	if rtt == nil {
		return online, -1
	}
	return online, *rtt
}
func parseRTT(output string) float64 {
	m := rttRegex.FindStringSubmatch(strings.ToLower(output))
	if len(m) != 2 {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= 0 {
		return -1
	}
	return v
}
