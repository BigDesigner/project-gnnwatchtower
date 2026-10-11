//go:build !linux && !windows

package main

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64) {
	return 0, 0, 0
}
