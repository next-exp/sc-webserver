//go:build !windows

package main

import (
	"fmt"
	"image"
)

func captureScreen(rect image.Rectangle) (image.Image, error) {
	return nil, fmt.Errorf("desktop capture requires Windows and an unlocked interactive session")
}

func captureWithOptions(o captureOptions) (image.Image, error) {
	return nil, fmt.Errorf("capture requires Windows")
}
func listWindows() ([]windowInfo, error) {
	return nil, fmt.Errorf("window enumeration requires Windows")
}

func captureWorkerSetup(o captureOptions) (func(), error) { return func() {}, nil }
