//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	enumWindows  = user32.NewProc("EnumWindows")
	windowText   = user32.NewProc("GetWindowTextW")
	isVisible    = user32.NewProc("IsWindowVisible")
	isIconic     = user32.NewProc("IsIconic")
	windowPID    = user32.NewProc("GetWindowThreadProcessId")
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	openProcess  = kernel32.NewProc("OpenProcess")
	processImage = kernel32.NewProc("QueryFullProcessImageNameW")
	closeHandle  = kernel32.NewProc("CloseHandle")
	enumMu       sync.Mutex
	enumResult   []windowInfo
	enumCallback = syscall.NewCallback(func(hwnd, param uintptr) uintptr {
		visible, _, _ := isVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var title [1024]uint16
		n, _, _ := windowText.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if n == 0 {
			return 1
		}
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		name := ""
		handle, _, _ := openProcess.Call(0x1000, 0, uintptr(pid))
		if handle != 0 {
			var path [32768]uint16
			size := uint32(len(path))
			ok, _, _ := processImage.Call(handle, 0, uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&size)))
			closeHandle.Call(handle)
			if ok != 0 {
				name = filepath.Base(syscall.UTF16ToString(path[:size]))
			}
		}
		minimized, _, _ := isIconic.Call(hwnd)
		enumResult = append(enumResult, windowInfo{HWND: uint64(hwnd), Title: syscall.UTF16ToString(title[:]), Process: name, PID: pid, Minimized: minimized != 0})
		return 1
	})
)

func listWindows() ([]windowInfo, error) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumResult = nil
	ok, _, err := enumWindows.Call(enumCallback, 0)
	if ok == 0 {
		return nil, fmt.Errorf("EnumWindows: %v", err)
	}
	result := enumResult
	enumResult = nil
	return result, nil
}
func findWindow(o captureOptions) (windowInfo, error) {
	windows, err := listWindows()
	if err != nil {
		return windowInfo{}, err
	}
	var matches []windowInfo
	for _, w := range windows {
		if o.WindowHWND != 0 && w.HWND != o.WindowHWND {
			continue
		}
		if o.WindowTitle != "" && !strings.Contains(strings.ToLower(w.Title), strings.ToLower(o.WindowTitle)) {
			continue
		}
		if o.WindowProcess != "" && !strings.EqualFold(w.Process, o.WindowProcess) {
			continue
		}
		matches = append(matches, w)
	}
	if len(matches) == 0 {
		return windowInfo{}, fmt.Errorf("target window not found in this interactive session")
	}
	if len(matches) > 1 {
		return windowInfo{}, fmt.Errorf("%d windows match; narrow the title/process or use -window-hwnd (see -list-windows)", len(matches))
	}
	if matches[0].Minimized {
		return windowInfo{}, fmt.Errorf("target window is minimized")
	}
	return matches[0], nil
}
