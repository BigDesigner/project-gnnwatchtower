//go:build linux

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var prevCpuTotal uint64
var prevCpuIdle uint64

var prevNetRx uint64
var prevNetTx uint64
var prevNetTime time.Time

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64, netRx float64, netTx float64, temp float64) {
	ramPct = getLinuxRAM()
	cpuPct = getLinuxCPU()
	diskPct = getLinuxDisk()
	netRx, netTx = getLinuxNetwork()
	temp = getLinuxTemp()
	return cpuPct, ramPct, diskPct, netRx, netTx, temp
}

func getLinuxRAM() float64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	var total, avail float64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "MemTotal:" {
			total, _ = strconv.ParseFloat(fields[1], 64)
		} else if fields[0] == "MemAvailable:" {
			avail, _ = strconv.ParseFloat(fields[1], 64)
		}
	}
	if total > 0 && avail > 0 {
		return ((total - avail) / total) * 100.0
	}
	return 0
}

func getLinuxCPU() float64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:]
			var total, idle uint64
			for i, v := range fields {
				val, _ := strconv.ParseUint(v, 10, 64)
				total += val
				if i == 3 || i == 4 { // idle + iowait
					idle += val
				}
			}

			if prevCpuTotal > 0 && total > prevCpuTotal {
				diffTotal := float64(total - prevCpuTotal)
				diffIdle := float64(idle - prevCpuIdle)
				prevCpuTotal = total
				prevCpuIdle = idle
				if diffTotal > 0 {
					pct := (1.0 - diffIdle/diffTotal) * 100.0
					if pct < 0 {
						pct = 0
					}
					if pct > 100 {
						pct = 100
					}
					return pct
				}
			}
			prevCpuTotal = total
			prevCpuIdle = idle
			return 0
		}
	}
	return 0
}

func getLinuxDisk() float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total > 0 {
		return float64(total-free) / float64(total) * 100.0
	}
	return 0
}

func getLinuxNetwork() (rxKBps float64, txKBps float64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var totalRx, totalTx uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			rx, _ := strconv.ParseUint(fields[0], 10, 64)
			tx, _ := strconv.ParseUint(fields[8], 10, 64)
			totalRx += rx
			totalTx += tx
		}
	}

	now := time.Now()
	if !prevNetTime.IsZero() && totalRx >= prevNetRx && totalTx >= prevNetTx {
		durationSec := now.Sub(prevNetTime).Seconds()
		if durationSec > 0 {
			rxKBps = float64(totalRx-prevNetRx) / durationSec / 1024.0
			txKBps = float64(totalTx-prevNetTx) / durationSec / 1024.0
		}
	}

	prevNetRx = totalRx
	prevNetTx = totalTx
	prevNetTime = now
	return rxKBps, txKBps
}

func getLinuxTemp() float64 {
	// Try thermal_zone
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err == nil && len(zones) > 0 {
		for _, z := range zones {
			content, err := os.ReadFile(z)
			if err == nil {
				val, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
				if err == nil && val > 0 {
					return val / 1000.0
				}
			}
		}
	}

	// Try hwmon
	hwmon, err := filepath.Glob("/sys/class/hwmon/hwmon*/temp1_input")
	if err == nil && len(hwmon) > 0 {
		for _, h := range hwmon {
			content, err := os.ReadFile(h)
			if err == nil {
				val, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64)
				if err == nil && val > 0 {
					return val / 1000.0
				}
			}
		}
	}

	return 0
}
