//go:build linux

package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var prevCpuTotal uint64
var prevCpuIdle uint64

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64) {
	ramPct = getLinuxRAM()
	cpuPct = getLinuxCPU()
	diskPct = getLinuxDisk()
	return cpuPct, ramPct, diskPct
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
