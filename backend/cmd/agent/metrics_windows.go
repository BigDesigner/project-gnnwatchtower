//go:build windows

package main

import (
	"encoding/binary"
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = modkernel32.NewProc("GetSystemTimes")
	procGetDiskFreeSpaceExW  = modkernel32.NewProc("GetDiskFreeSpaceExW")

	modiphlpapi    = syscall.NewLazyDLL("iphlpapi.dll")
	procGetIfTable = modiphlpapi.NewProc("GetIfTable")
)

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

type filetime struct {
	dwLowDateTime  uint32
	dwHighDateTime uint32
}

var prevIdleTime uint64
var prevKernelTime uint64
var prevUserTime uint64

var prevWinNetRx uint64
var prevWinNetTx uint64
var prevWinNetTime time.Time

func filetimeToUint64(ft filetime) uint64 {
	return (uint64(ft.dwHighDateTime) << 32) | uint64(ft.dwLowDateTime)
}

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64, netRx float64, netTx float64, temp float64) {
	// 1. Real Windows RAM via GlobalMemoryStatusEx
	var mem memoryStatusEx
	mem.cbSize = uint32(unsafe.Sizeof(mem))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if ret != 0 {
		ramPct = float64(mem.dwMemoryLoad)
	}

	// 2. Real Windows CPU via GetSystemTimes
	var idleTime, kernelTime, userTime filetime
	ret, _, _ = procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret != 0 {
		idle := filetimeToUint64(idleTime)
		kernel := filetimeToUint64(kernelTime)
		user := filetimeToUint64(userTime)

		if prevKernelTime > 0 {
			diffIdle := idle - prevIdleTime
			diffKernel := kernel - prevKernelTime
			diffUser := user - prevUserTime
			diffTotal := diffKernel + diffUser

			if diffTotal > 0 {
				pct := float64(diffTotal-diffIdle) / float64(diffTotal) * 100.0
				if pct < 0 {
					pct = 0
				}
				if pct > 100 {
					pct = 100
				}
				cpuPct = pct
			}
		}
		prevIdleTime = idle
		prevKernelTime = kernel
		prevUserTime = user
	}

	// 3. Real Windows Disk via GetDiskFreeSpaceExW
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	dir, err := syscall.UTF16PtrFromString("C:\\")
	if err == nil {
		ret, _, _ = procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(dir)),
			uintptr(unsafe.Pointer(&freeBytesAvailable)),
			uintptr(unsafe.Pointer(&totalNumberOfBytes)),
			uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
		)
		if ret != 0 && totalNumberOfBytes > 0 {
			diskPct = float64(totalNumberOfBytes-freeBytesAvailable) / float64(totalNumberOfBytes) * 100.0
		}
	}

	// 4. Real Windows Network I/O via GetIfTable
	netRx, netTx = getWindowsNetwork()

	return cpuPct, ramPct, diskPct, netRx, netTx, 0
}

func getWindowsNetwork() (rxKBps float64, txKBps float64) {
	var size uint32
	// First call to determine buffer size
	procGetIfTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if size == 0 {
		return 0, 0
	}

	buf := make([]byte, size)
	ret, _, _ := procGetIfTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0)
	if ret != 0 || len(buf) < 4 {
		return 0, 0
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	const rowSize = 860
	var totalRx, totalTx uint64

	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + int(i)*rowSize
		if offset+584 > len(buf) {
			break
		}
		ifType := binary.LittleEndian.Uint32(buf[offset+520 : offset+524])
		if ifType == 24 { // Loopback
			continue
		}
		inOctets := uint64(binary.LittleEndian.Uint32(buf[offset+556 : offset+560]))
		outOctets := uint64(binary.LittleEndian.Uint32(buf[offset+580 : offset+584]))
		totalRx += inOctets
		totalTx += outOctets
	}

	now := time.Now()
	if !prevWinNetTime.IsZero() && totalRx >= prevWinNetRx && totalTx >= prevWinNetTx {
		durationSec := now.Sub(prevWinNetTime).Seconds()
		if durationSec > 0 {
			rxKBps = float64(totalRx-prevWinNetRx) / durationSec / 1024.0
			txKBps = float64(totalTx-prevWinNetTx) / durationSec / 1024.0
		}
	}

	prevWinNetRx = totalRx
	prevWinNetTx = totalTx
	prevWinNetTime = now
	return rxKBps, txKBps
}
