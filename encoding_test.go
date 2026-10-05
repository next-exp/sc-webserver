package main

import (
	"bytes"
	"image"
	"image/color"
	"net/http/httptest"
	"testing"
	"time"
)

func TestImageFormatsAndRoutes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 7), uint8(y * 13), 33, 255})
		}
	}
	for _, format := range []string{"jpeg", "png"} {
		t.Run(format, func(t *testing.T) {
			enc := encodingOptions{Format: format, Quality: 85}
			if err := enc.validate(); err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err := enc.encode(&b, img); err != nil {
				t.Fatal(err)
			}
			s := &store{frame: b.Bytes(), state: status{Format: format, CapturedAt: time.Now(), Sequence: 1}}
			for _, path := range []string{"/screenshot", "/screenshot." + enc.extension()} {
				r := httptest.NewRecorder()
				s.handler().ServeHTTP(r, httptest.NewRequest("GET", path, nil))
				if r.Code != 200 || r.Header().Get("Content-Type") != enc.contentType() {
					t.Fatalf("%s: %d %s", path, r.Code, r.Header().Get("Content-Type"))
				}
				decoded, _, err := image.Decode(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Bounds() != img.Bounds() {
					t.Fatal("dimensions changed")
				}
				if format == "png" {
					for y := 0; y < 16; y++ {
						for x := 0; x < 32; x++ {
							if color.RGBAModel.Convert(decoded.At(x, y)) != img.RGBAAt(x, y) {
								t.Fatal("PNG pixels changed")
							}
						}
					}
				}
			}
			other := "/screenshot.png"
			if format == "png" {
				other = "/screenshot.jpg"
			}
			r := httptest.NewRecorder()
			s.handler().ServeHTTP(r, httptest.NewRequest("GET", other, nil))
			if r.Code != 404 {
				t.Fatal("mismatched extension accepted")
			}
			s.state.Error = "desktop unavailable"
			r = httptest.NewRecorder()
			s.handler().ServeHTTP(r, httptest.NewRequest("GET", "/screenshot", nil))
			if r.Code != 503 {
				t.Fatal("failed capture served as fresh")
			}
		})
	}
	if err := (encodingOptions{Format: "gif", Quality: 85}).validate(); err == nil {
		t.Fatal("invalid format accepted")
	}
}
