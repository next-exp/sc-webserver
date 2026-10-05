//go:build windows && amd64

package main

// Minimal Windows Graphics Capture ABI bridge. Interface IDs and vtable layouts:
// https://github.com/microsoft/windows-rs/tree/master/crates/libs/windows/src/Windows/Graphics/Capture
// https://learn.microsoft.com/en-us/windows/win32/api/windows.graphics.capture.interop/nn-windows-graphics-capture-interop-igraphicscaptureiteminterop
// https://learn.microsoft.com/en-us/windows/win32/api/d3d11/nn-d3d11-id3d11devicecontext
// All COM references belong to the capture goroutine's locked MTA thread.

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type winGUID struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

func guid(s string) winGUID {
	b, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(b) != 16 {
		panic("invalid interface GUID")
	}
	g := winGUID{A: binary.BigEndian.Uint32(b[:4]), B: binary.BigEndian.Uint16(b[4:6]), C: binary.BigEndian.Uint16(b[6:8])}
	copy(g.D[:], b[8:])
	return g
}

var (
	iidItemInterop    = guid("3628e81b-3cac-4c60-b7f4-23ce0e0c3356")
	iidItem           = guid("79c3f95b-31f7-4ec2-a464-632ef5d30760")
	iidPoolStatics2   = guid("589b103f-6bbc-5df5-a991-02e28b3b66d5")
	iidClosable       = guid("30d5a829-7fa4-4026-83bb-d75bae4ea99e")
	iidDXGIDevice     = guid("54ec77fa-1377-44e6-8c32-88fd5f44c84c")
	iidDirect3DDevice = guid("a37624ab-8d5f-4650-9d3e-9eae3d9bc670")
	iidDXGIAccess     = guid("a9b3d012-3df2-4ee3-b8d1-8695f457d3c1")
	iidTexture2D      = guid("6f15aaf2-d208-4e89-9ab4-489535d34f9c")
	combase           = syscall.NewLazyDLL("combase.dll")
	roInitialize      = combase.NewProc("RoInitialize")
	roUninitialize    = combase.NewProc("RoUninitialize")
	createString      = combase.NewProc("WindowsCreateString")
	deleteString      = combase.NewProc("WindowsDeleteString")
	activationFactory = combase.NewProc("RoGetActivationFactory")
	d3d11             = syscall.NewLazyDLL("d3d11.dll")
	createDevice      = d3d11.NewProc("D3D11CreateDevice")
	wrapDevice        = d3d11.NewProc("CreateDirect3D11DeviceFromDXGIDevice")
)

type comObject struct{ VTable *[128]uintptr }

func (p *comObject) address() uintptr { return uintptr(unsafe.Pointer(p)) }

//go:uintptrescapes
func (p *comObject) call(slot int, args ...uintptr) uintptr {
	a := make([]uintptr, 1, len(args)+1)
	a[0] = p.address()
	a = append(a, args...)
	r, _, _ := syscall.SyscallN(p.VTable[slot], a...)
	runtime.KeepAlive(p)
	return r
}
func hresult(stage string, r uintptr) error {
	if int32(r) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", stage, uint32(r))
	}
	return nil
}
func (p *comObject) release() {
	if p != nil {
		p.call(2)
	}
}
func (p *comObject) query(iid *winGUID) (*comObject, error) {
	var result *comObject
	err := hresult("QueryInterface", p.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&result))))
	return result, err
}
func (p *comObject) close() {
	if p == nil {
		return
	}
	closable, err := p.query(&iidClosable)
	if err == nil {
		closable.call(6)
		closable.release()
	}
	p.release()
}
func factory(class string, iid *winGUID) (*comObject, error) {
	utf, err := syscall.UTF16FromString(class)
	if err != nil {
		return nil, err
	}
	var str uintptr
	if err = hresult("WindowsCreateString", func() uintptr {
		r, _, _ := createString.Call(uintptr(unsafe.Pointer(&utf[0])), uintptr(len(utf)-1), uintptr(unsafe.Pointer(&str)))
		return r
	}()); err != nil {
		return nil, err
	}
	defer deleteString.Call(str)
	var p *comObject
	r, _, _ := activationFactory.Call(str, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&p)))
	return p, hresult("RoGetActivationFactory (Windows 10 1903+ required)", r)
}

type sizeInt32 struct{ Width, Height int32 }
type textureDesc struct {
	Width, Height, MipLevels, ArraySize, Format uint32
	SampleCount, SampleQuality                  uint32
	Usage, BindFlags, CPUAccessFlags, MiscFlags uint32
}
type mappedResource struct {
	Data                 unsafe.Pointer
	RowPitch, DepthPitch uint32
}

func captureWGC(o captureOptions) (image.Image, error) {
	target, err := findWindow(o)
	if err != nil {
		return nil, err
	}
	interop, err := factory("Windows.Graphics.Capture.GraphicsCaptureItem", &iidItemInterop)
	if err != nil {
		return nil, err
	}
	defer interop.release()
	var item *comObject
	if err = hresult("CreateForWindow", interop.call(3, uintptr(target.HWND), uintptr(unsafe.Pointer(&iidItem)), uintptr(unsafe.Pointer(&item)))); err != nil {
		return nil, err
	}
	defer item.release()
	var size sizeInt32
	if err = hresult("GraphicsCaptureItem.Size", item.call(7, uintptr(unsafe.Pointer(&size)))); err != nil {
		return nil, err
	}
	if !validFrameSize(size.Width, size.Height) {
		return nil, fmt.Errorf("invalid window size %dx%d", size.Width, size.Height)
	}
	var device, context *comObject
	// D3D_DRIVER_TYPE_HARDWARE, D3D11_CREATE_DEVICE_BGRA_SUPPORT, SDK_VERSION.
	r, _, _ := createDevice.Call(0, 1, 0, 0x20, 0, 0, 7, uintptr(unsafe.Pointer(&device)), 0, uintptr(unsafe.Pointer(&context)))
	if err = hresult("D3D11CreateDevice", r); err != nil {
		return nil, err
	}
	defer device.release()
	defer context.release()
	dxgi, err := device.query(&iidDXGIDevice)
	if err != nil {
		return nil, err
	}
	defer dxgi.release()
	var inspectable *comObject
	r, _, _ = wrapDevice.Call(dxgi.address(), uintptr(unsafe.Pointer(&inspectable)))
	if err = hresult("CreateDirect3D11DeviceFromDXGIDevice", r); err != nil {
		return nil, err
	}
	defer inspectable.release()
	winDevice, err := inspectable.query(&iidDirect3DDevice)
	if err != nil {
		return nil, err
	}
	defer winDevice.release()
	statics, err := factory("Windows.Graphics.Capture.Direct3D11CaptureFramePool", &iidPoolStatics2)
	if err != nil {
		return nil, err
	}
	defer statics.release()
	var pool *comObject
	// Win64 passes the eight-byte SizeInt32 value in one integer argument.
	packedSize := uintptr(uint64(uint32(size.Width)) | uint64(uint32(size.Height))<<32)
	if err = hresult("CreateFreeThreaded", statics.call(6, winDevice.address(), 87, 2, packedSize, uintptr(unsafe.Pointer(&pool)))); err != nil {
		return nil, err
	}
	defer pool.close() // DXGI_FORMAT_B8G8R8A8_UNORM
	var session *comObject
	if err = hresult("CreateCaptureSession", pool.call(10, item.address(), uintptr(unsafe.Pointer(&session)))); err != nil {
		return nil, err
	}
	defer session.close()
	if err = hresult("StartCapture", session.call(6)); err != nil {
		return nil, err
	}
	// One-shot session; no background capture remains active between two-second ticks.
	deadline := time.Now().Add(time.Second)
	var frame *comObject
	for frame == nil {
		if err = hresult("TryGetNextFrame", pool.call(7, uintptr(unsafe.Pointer(&frame)))); err != nil {
			return nil, err
		}
		if frame != nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WGC timed out waiting for a fresh frame (window/session may be unavailable)")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer frame.close()
	var content sizeInt32
	if err = hresult("Frame.ContentSize", frame.call(8, uintptr(unsafe.Pointer(&content)))); err != nil {
		return nil, err
	}
	if !validFrameSize(content.Width, content.Height) {
		return nil, fmt.Errorf("invalid frame size")
	}
	var surface *comObject
	if err = hresult("Frame.Surface", frame.call(6, uintptr(unsafe.Pointer(&surface)))); err != nil {
		return nil, err
	}
	defer surface.release()
	access, err := surface.query(&iidDXGIAccess)
	if err != nil {
		return nil, err
	}
	defer access.release()
	var texture *comObject
	if err = hresult("Surface.GetInterface", access.call(3, uintptr(unsafe.Pointer(&iidTexture2D)), uintptr(unsafe.Pointer(&texture)))); err != nil {
		return nil, err
	}
	defer texture.release()
	var desc textureDesc
	texture.call(10, uintptr(unsafe.Pointer(&desc)))
	if int32(desc.Width) < content.Width || int32(desc.Height) < content.Height || !validFrameSize(int32(desc.Width), int32(desc.Height)) || desc.Format != 87 {
		return nil, fmt.Errorf("unexpected capture texture size or pixel format")
	}
	desc.Usage = 3
	desc.BindFlags = 0
	desc.CPUAccessFlags = 0x20000
	desc.MiscFlags = 0 // D3D11_USAGE_STAGING / CPU_ACCESS_READ
	var staging *comObject
	if err = hresult("CreateTexture2D", device.call(5, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&staging)))); err != nil {
		return nil, err
	}
	defer staging.release()
	context.call(47, staging.address(), texture.address()) // CopyResource
	var mapped mappedResource
	for {
		result := context.call(14, staging.address(), 0, 1, 0x100000, uintptr(unsafe.Pointer(&mapped))) // Map READ / DO_NOT_WAIT
		if uint32(result) != 0x887A000A {
			err = hresult("Map staging texture", result)
			break
		} // DXGI_ERROR_WAS_STILL_DRAWING
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WGC timed out reading GPU pixels")
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer context.call(15, staging.address(), 0)
	w, h := int(content.Width), int(content.Height)
	if mapped.Data == nil || int(mapped.RowPitch) < w*4 || uint64(mapped.RowPitch)*uint64(h) > 256*1024*1024 {
		return nil, fmt.Errorf("invalid GPU row pitch")
	}
	src := unsafe.Slice((*byte)(mapped.Data), int(mapped.RowPitch)*h)
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		row := src[y*int(mapped.RowPitch) : y*int(mapped.RowPitch)+w*4]
		dst := pixels[y*w*4 : (y+1)*w*4]
		for x := 0; x < len(row); x += 4 {
			dst[x] = row[x+2]
			dst[x+1] = row[x+1]
			dst[x+2] = row[x]
			dst[x+3] = 255
		}
	}
	return &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}, nil
}
func validFrameSize(w, h int32) bool {
	return w > 0 && h > 0 && w <= 16384 && h <= 16384 && int64(w)*int64(h) <= 40000000
}

func initWGCThread() (func(), error) {
	r, _, _ := roInitialize.Call(1)
	if err := hresult("RoInitialize", r); err != nil {
		return nil, err
	}
	return func() { roUninitialize.Call() }, nil
}
