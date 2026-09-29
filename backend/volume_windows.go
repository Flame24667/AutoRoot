//go:build windows

package main

import "golang.org/x/sys/windows"

func volumeFreeBytes(path string) int64 {
	var free, total, totalFree uint64
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0
	}
	return int64(free)
}
