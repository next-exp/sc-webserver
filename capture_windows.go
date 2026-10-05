//go:build windows

package main

import (
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	gdi32        = syscall.NewLazyDLL("gdi32.dll")
	getDC        = user32.NewProc("GetDC")
	releaseDC    = user32.NewProc("ReleaseDC")
	metrics      = user32.NewProc("GetSystemMetrics")
	openDesktop  = user32.NewProc("OpenInputDesktop")
	closeDesktop = user32.NewProc("CloseDesktop")
	objectInfo   = user32.NewProc("GetUserObjectInformationW")
	createDC     = gdi32.NewProc("CreateCompatibleDC")
	deleteDC     = gdi32.NewProc("DeleteDC")
	createBitmap = gdi32.NewProc("CreateDIBSection")
	selectObject = gdi32.NewProc("SelectObject")
	deleteObject = gdi32.NewProc("DeleteObject")
	bitBlt       = gdi32.NewProc("BitBlt")
	gdiFlush     = gdi32.NewProc("GdiFlush")
)

func init() { user32.NewProc("SetProcessDPIAware").Call() }

type bitmapInfo struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
	Colors                       [1]uint32
}

func metric(n uintptr) int { v, _, _ := metrics.Call(n); return int(int32(v)) }
func captureScreen(rect image.Rectangle) (image.Image, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	desktop, _, err := openDesktop.Call(0, 0, 1) // DESKTOP_READOBJECTS
	if desktop == 0 {
		return nil, fmt.Errorf("interactive desktop unavailable: %v", err)
	}
	defer closeDesktop.Call(desktop)
	var name [256]uint16
	var needed uint32
	ok, _, err := objectInfo.Call(desktop, 2, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)*2), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 || syscall.UTF16ToString(name[:]) != "Default" {
		return nil, fmt.Errorf("desktop is locked or unavailable")
	}
	bounds := image.Rect(metric(76), metric(77), metric(76)+metric(78), metric(77)+metric(79))
	if rect.Empty() {
		rect = bounds
	}
	if rect.Empty() || !rect.In(bounds) || rect.Dx() > 16384 || rect.Dy() > 16384 || int64(rect.Dx())*int64(rect.Dy()) > 40000000 {
		return nil, fmt.Errorf("invalid capture bounds: %v (desktop %v)", rect, bounds)
	}
	screen, _, err := getDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("GetDC: %v", err)
	}
	defer releaseDC.Call(0, screen)
	memory, _, err := createDC.Call(screen)
	if memory == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC: %v", err)
	}
	defer deleteDC.Call(memory)
	info := bitmapInfo{Size: 40, Width: int32(rect.Dx()), Height: -int32(rect.Dy()), Planes: 1, BitCount: 32, SizeImage: uint32(rect.Dx() * rect.Dy() * 4)}
	var bits unsafe.Pointer
	bitmap, _, err := createBitmap.Call(screen, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection: %v", err)
	}
	defer deleteObject.Call(bitmap)
	old, _, err := selectObject.Call(memory, bitmap)
	if old == 0 || old == ^uintptr(0) {
		return nil, fmt.Errorf("SelectObject: %v", err)
	}
	ok, _, err = bitBlt.Call(memory, 0, 0, uintptr(rect.Dx()), uintptr(rect.Dy()), screen, uintptr(rect.Min.X), uintptr(rect.Min.Y), 0x40CC0020) // SRCCOPY | CAPTUREBLT
	selectObject.Call(memory, old)                                                                                                               // Restore the stock bitmap before deletion.
	if ok == 0 {
		return nil, fmt.Errorf("BitBlt: %v", err)
	}
	// GDI must finish writing before Go reads the DIB section's pixel memory.
	ok, _, err = gdiFlush.Call()
	if ok == 0 {
		return nil, fmt.Errorf("GdiFlush: %v", err)
	}
	pixels := make([]byte, rect.Dx()*rect.Dy()*4)
	copy(pixels, unsafe.Slice((*byte)(bits), len(pixels)))
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
		pixels[i+3] = 255
	}
	return &image.RGBA{Pix: pixels, Stride: rect.Dx() * 4, Rect: image.Rect(0, 0, rect.Dx(), rect.Dy())}, nil
}
