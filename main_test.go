package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFreshStaleAndFailedFrames(t *testing.T) {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	s := &store{state: status{CapturedAt: time.Now(), Sequence: 1}, jpeg: b.Bytes()}
	h := s.handler()
	check := func(path string, want int) {
		t.Helper()
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != want {
			t.Fatalf("%s: %d != %d", path, r.Code, want)
		}
		if r.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
		if path == "/screenshot.jpg" && want == 200 {
			if _, err := jpeg.Decode(r.Body); err != nil {
				t.Fatal(err)
			}
		}
	}
	check("/screenshot.jpg", 200)
	check("/healthz", 200)
	check("/", 200)
	check("/vue.global.prod.js", 200)
	s.state.CapturedAt = time.Now().Add(-7 * time.Second)
	check("/screenshot.jpg", 503)
	check("/healthz", 503)
	s.state.CapturedAt = time.Now()
	s.state.Error = "desktop locked"
	check("/screenshot.jpg", 503)
	check("/healthz", 503)
	check("/api/status", 200)
	s.jpeg = nil
	s.state.Error = ""
	s.state.CapturedAt = time.Time{}
	check("/screenshot.jpg", 503)
	check("/healthz", 503)
}
func TestConcurrentViewersAndFrameUpdates(t *testing.T) {
	s := &store{state: status{CapturedAt: time.Now(), Sequence: 1}, jpeg: []byte("frame")}
	h := s.handler()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r := httptest.NewRecorder()
				h.ServeHTTP(r, httptest.NewRequest("GET", "/screenshot.jpg", nil))
				if r.Code != 200 {
					t.Errorf("status %d", r.Code)
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		s.Lock()
		s.jpeg = []byte("updated frame")
		s.state.CapturedAt = time.Now()
		s.state.Sequence++
		s.Unlock()
	}
	wg.Wait()
}
