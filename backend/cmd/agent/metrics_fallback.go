//go:build !linux && !windows

package main

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64, netRx float64, netTx float64, temp float64) {
	return 0, 0, 0, 0, 0, 0
}
