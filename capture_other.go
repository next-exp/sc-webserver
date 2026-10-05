//go:build !windows

package main

import (
	"fmt"
	"image"
)

func captureScreen(rect image.Rectangle) (image.Image, error) {
	return nil, fmt.Errorf("desktop capture requires Windows and an unlocked interactive session")
}
