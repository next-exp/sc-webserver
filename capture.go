package main

import (
	"fmt"
	"image"
)

type captureOptions struct {
	Backend       string
	WindowTitle   string
	WindowProcess string
	WindowHWND    uint64
	Rect          image.Rectangle
}

func (o captureOptions) validate() error {
	switch o.Backend {
	case "desktop":
		if o.WindowTitle != "" || o.WindowProcess != "" || o.WindowHWND != 0 {
			return fmt.Errorf("window selectors require -capture wgc")
		}
	case "wgc":
		if o.WindowTitle == "" && o.WindowProcess == "" && o.WindowHWND == 0 {
			return fmt.Errorf("wgc requires -window-title, -window-process, or -window-hwnd")
		}
		if !o.Rect.Empty() || o.Rect.Min != (image.Point{}) {
			return fmt.Errorf("desktop crop flags cannot be combined with wgc window capture")
		}
	default:
		return fmt.Errorf("unknown capture backend %q (use desktop or wgc)", o.Backend)
	}
	return nil
}

type windowInfo struct {
	HWND      uint64 `json:"hwnd"`
	Title     string `json:"title"`
	Process   string `json:"process"`
	PID       uint32 `json:"pid"`
	Minimized bool   `json:"minimized"`
}
