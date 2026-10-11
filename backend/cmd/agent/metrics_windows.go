//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = modkernel32.NewProc("GetSystemTimes")
	procGetDiskFreeSpaceExW  = modkernel32.NewProc("GetDiskFreeSpaceExW")
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

func filetimeToUint64(ft filetime) uint64 {
	return (uint64(ft.dwHighDateTime) << 32) | uint64(ft.dwLowDateTime)
}

func getSystemMetrics() (cpuPct float64, ramPct float64, diskPct float64) {
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

	return cpuPct, ramPct, diskPct
}
