package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type phaseMeasurement struct {
	WallMS     float64 `json:"wallMs"`
	CPUMS      float64 `json:"cpuMs"`
	AllocBytes uint64  `json:"allocBytes"`
}
type encodingMeasurement struct {
	Bytes  int              `json:"bytes"`
	Encode phaseMeasurement `json:"encode"`
	Write  phaseMeasurement `json:"write"`
}
type benchmarkSample struct {
	Capture phaseMeasurement    `json:"capture"`
	JPEG    encodingMeasurement `json:"jpeg"`
	PNG     encodingMeasurement `json:"png"`
}

func measurePhase(fn func() error) (phaseMeasurement, error) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	cpuBefore, err := processCPUTime()
	if err != nil {
		return phaseMeasurement{}, err
	}
	started := time.Now()
	if err := fn(); err != nil {
		return phaseMeasurement{}, err
	}
	wall := time.Since(started)
	cpuAfter, err := processCPUTime()
	if err != nil {
		return phaseMeasurement{}, err
	}
	runtime.ReadMemStats(&after)
	return phaseMeasurement{WallMS: float64(wall) / float64(time.Millisecond), CPUMS: float64(cpuAfter-cpuBefore) / float64(time.Millisecond), AllocBytes: after.TotalAlloc - before.TotalAlloc}, nil
}
func benchmarkEncode(img image.Image, enc encodingOptions, dir string) (encodingMeasurement, []byte, error) {
	var b bytes.Buffer
	encoding, err := measurePhase(func() error { return enc.encode(&b, img) })
	if err != nil {
		return encodingMeasurement{}, nil, err
	}
	writing, err := measurePhase(func() error { return os.WriteFile(filepath.Join(dir, "latest."+enc.extension()), b.Bytes(), 0600) })
	return encodingMeasurement{Bytes: b.Len(), Encode: encoding, Write: writing}, b.Bytes(), err
}
func benchmarkCapture(options captureOptions, quality, count int, output string) error {
	if _, err := processCPUTime(); err != nil {
		return err
	}
	cleanup, err := captureWorkerSetup(options)
	if err != nil {
		return err
	}
	defer cleanup()
	dir, err := os.MkdirTemp("", "sc-webserver-benchmark-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	encoders := []encodingOptions{{Format: "jpeg", Quality: quality}, {Format: "png", Quality: quality}}
	// Warm up codecs and graphics before measured samples. No HTTP server is started.
	for i := 0; i < 3; i++ {
		img, err := captureWithOptions(options)
		if err != nil {
			return err
		}
		for _, enc := range encoders {
			var b bytes.Buffer
			if err := enc.encode(&b, img); err != nil {
				return err
			}
		}
	}
	runtime.GC()
	samples := make([]benchmarkSample, 0, count)
	var width, height int
	lossless := false
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for i := 0; i < count; i++ {
		var img image.Image
		capture, err := measurePhase(func() error { var err error; img, err = captureWithOptions(options); return err })
		if err != nil {
			return err
		}
		width, height = img.Bounds().Dx(), img.Bounds().Dy()
		sample := benchmarkSample{Capture: capture}
		// Alternate order to avoid consistently favoring the first or second encoder.
		for j := 0; j < 2; j++ {
			enc := encoders[(j+i)%2]
			result, data, err := benchmarkEncode(img, enc, dir)
			if err != nil {
				return err
			}
			if enc.Format == "jpeg" {
				sample.JPEG = result
			} else {
				sample.PNG = result
				if i == 0 {
					decoded, err := png.Decode(bytes.NewReader(data))
					if err != nil {
						return err
					}
					for y := 0; y < height; y++ {
						for x := 0; x < width; x++ {
							r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
							rr, gg, bb, aa := decoded.At(x, y).RGBA()
							if r != rr || g != gg || b != bb || a != aa {
								return fmt.Errorf("PNG pixel mismatch at %d,%d", x, y)
							}
						}
					}
					lossless = true
				}
			}
		}
		samples = append(samples, sample)
		if i+1 < count {
			<-ticker.C
		}
	}
	report := struct {
		Backend        string            `json:"backend"`
		Quality        int               `json:"jpegQuality"`
		PNGCompression string            `json:"pngCompression"`
		Width          int               `json:"width"`
		Height         int               `json:"height"`
		IntervalMS     int               `json:"intervalMs"`
		WarmupFrames   int               `json:"warmupFrames"`
		PNGPixelExact  bool              `json:"pngPixelExact"`
		Notes          string            `json:"notes"`
		Samples        []benchmarkSample `json:"samples"`
	}{options.Backend, quality, "default", width, height, 2000, 3, lossless, "Both encoders use each same raw capture; order alternates. CPU is this benchmark process's kernel+user time (includes GC), not system or LabVIEW CPU; Windows accounting is quantized. AllocBytes is allocated Go heap traffic, not peak resident memory. File writes are buffered, not forced to durable storage.", samples}
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if output != "" {
		return os.WriteFile(output, b.Bytes(), 0600)
	}
	_, err = os.Stdout.Write(b.Bytes())
	return err
}
