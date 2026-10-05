//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var processTimes = kernel32.NewProc("GetProcessTimes")

func processCPUTime() (time.Duration, error) {
	var created, exited, kernel, user syscall.Filetime
	// GetCurrentProcess pseudo-handle is -1; it does not need to be closed.
	ok, _, err := processTimes.Call(^uintptr(0), uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return 0, fmt.Errorf("GetProcessTimes: %v", err)
	}
	ticks := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	ticks += uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	return time.Duration(ticks) * 100, nil
}
