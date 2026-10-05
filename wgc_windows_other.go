//go:build windows && !amd64

package main

import (
	"fmt"
	"image"
)

func captureWGC(o captureOptions) (image.Image, error) {
	return nil, fmt.Errorf("WGC backend currently requires Windows amd64")
}

func initWGCThread() (func(), error) { return nil, fmt.Errorf("WGC requires Windows amd64") }
