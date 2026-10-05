package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"
)

//go:embed web/*
var web embed.FS

type status struct {
	Format      string    `json:"format"`
	ImageBytes  int       `json:"imageBytes"`
	EncodeMS    int64     `json:"encodeMs"`
	CapturedAt  time.Time `json:"capturedAt"`
	AttemptedAt time.Time `json:"attemptedAt"`
	Error       string    `json:"error"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CaptureMS   int64     `json:"captureMs"`
	Sequence    uint64    `json:"sequence"`
	IntervalMS  int64     `json:"intervalMs"`
	Backend     string    `json:"backend"`
	WindowTitle string    `json:"windowTitle,omitempty"`
}
type store struct {
	sync.RWMutex
	state status
	frame []byte
}

func (s *store) capture(path string, encoding encodingOptions, options captureOptions) {
	started := time.Now()
	img, err := captureWithOptions(options)
	var buf bytes.Buffer
	encodeStarted := time.Now()
	if err == nil {
		err = encoding.encode(&buf, img)
	}
	encodeMS := time.Since(encodeStarted).Milliseconds()
	// Readers use a complete in-memory frame; the fixed temporary file is overwritten,
	// with no frame history and no concurrent file readers.
	if err == nil {
		err = os.WriteFile(path, buf.Bytes(), 0600)
	}
	s.Lock()
	defer s.Unlock()
	s.state.AttemptedAt = started
	s.state.CaptureMS = time.Since(started).Milliseconds()
	if err != nil {
		s.state.Error = err.Error()
		return
	}
	s.state.Error = ""
	s.state.CapturedAt = started
	s.state.Width, s.state.Height = img.Bounds().Dx(), img.Bounds().Dy()
	s.state.Sequence++
	s.frame = buf.Bytes()
	s.state.Format = encoding.Format
	s.state.ImageBytes = buf.Len()
	s.state.EncodeMS = encodeMS
}
func (s *store) handler() http.Handler {
	assets, _ := fs.Sub(web, "web")
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		s.RLock()
		st := s.state
		s.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		s.RLock()
		st := s.state
		s.RUnlock()
		if st.Error != "" || st.CapturedAt.IsZero() || time.Since(st.CapturedAt) > 6*time.Second {
			http.Error(w, "capture unavailable or stale", 503)
			return
		}
		w.Write([]byte("ok\n"))
	})
	serveFrame := func(w http.ResponseWriter, r *http.Request) {
		s.RLock()
		frame, st := s.frame, s.state
		s.RUnlock()
		encoding := encodingOptions{Format: st.Format}
		if r.URL.Path != "/screenshot" && r.URL.Path != "/screenshot."+encoding.extension() {
			http.NotFound(w, r)
			return
		}
		if len(frame) == 0 || st.Error != "" || time.Since(st.CapturedAt) > 6*time.Second {
			http.Error(w, "capture unavailable or stale", 503)
			return
		}
		w.Header().Set("Content-Type", encoding.contentType())
		w.Header().Set("X-Captured-At", st.CapturedAt.UTC().Format(time.RFC3339Nano))
		w.Write(frame)
	}
	for _, route := range []string{"/screenshot", "/screenshot.jpg", "/screenshot.png"} {
		mux.HandleFunc("GET "+route, serveFrame)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; style-src 'self'; img-src 'self' blob:; object-src 'none'; frame-ancestors 'self'")
		mux.ServeHTTP(w, r)
	})
}
func main() {
	listen := flag.String("listen", "127.0.0.1:8085", "HTTP listen address (use a private interface for remote viewing)")
	temp := flag.String("temp-dir", filepath.Join(os.TempDir(), "sc-webserver"), "temporary directory; only the latest image is retained")
	format := flag.String("format", "jpeg", "image format: jpeg or png (lossless, default compression)")
	benchFrames := flag.Int("benchmark-frames", 0, "benchmark JPEG/PNG from identical captures, at two-second intervals, then exit")
	benchOutput := flag.String("benchmark-output", "", "benchmark JSON output file (default stdout)")
	quality := flag.Int("quality", 85, "JPEG quality, 1..100")
	x := flag.Int("x", 0, "crop left (desktop coordinates)")
	y := flag.Int("y", 0, "crop top")
	width := flag.Int("width", 0, "crop width; 0 captures whole desktop")
	height := flag.Int("height", 0, "crop height")
	backend := flag.String("capture", "desktop", "capture backend: desktop or wgc")
	windowTitle := flag.String("window-title", "", "WGC target title substring (case-insensitive)")
	windowProcess := flag.String("window-process", "", "WGC target executable basename")
	windowHWND := flag.Uint64("window-hwnd", 0, "WGC target window handle (decimal or 0x hex)")
	list := flag.Bool("list-windows", false, "list visible titled windows in this session as JSON and exit")
	flag.Parse()
	if *list {
		windows, err := listWindows()
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(windows)
		return
	}
	options := captureOptions{Backend: *backend, WindowTitle: *windowTitle, WindowProcess: *windowProcess, WindowHWND: *windowHWND, Rect: image.Rect(*x, *y, *x+*width, *y+*height)}
	if err := options.validate(); err != nil {
		log.Fatal(err)
	}
	encoding := encodingOptions{Format: *format, Quality: *quality}
	if err := encoding.validate(); err != nil {
		log.Fatal(err)
	}
	if *width < 0 || *height < 0 || (*width == 0) != (*height == 0) {
		log.Fatal("invalid JPEG quality or crop dimensions")
	}
	if *benchFrames < 0 {
		log.Fatal("benchmark frame count must be nonnegative")
	}
	if *benchFrames > 0 {
		if err := benchmarkCapture(options, *quality, *benchFrames, *benchOutput); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := os.MkdirAll(*temp, 0700); err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(*temp, "latest."+encoding.extension())
	// Remove the other format on startup, including after an abrupt previous exit.
	other := "latest.png"
	if encoding.Format == "png" {
		other = "latest.jpg"
	}
	if err := os.Remove(filepath.Join(*temp, other)); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	defer os.Remove(path)
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	s := &store{state: status{Format: encoding.Format, IntervalMS: 2000, Backend: *backend, WindowTitle: *windowTitle}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		cleanup, err := captureWorkerSetup(options)
		if err != nil {
			s.Lock()
			s.state.Error = err.Error()
			s.state.AttemptedAt = time.Now()
			s.Unlock()
			<-ctx.Done()
			return
		}
		defer cleanup()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			s.capture(path, encoding, options)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	server := &http.Server{Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		<-ctx.Done()
		stop, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		server.Shutdown(stop)
	}()
	fmt.Printf("Listening on http://%s; capture every 2s; latest image: %s\n", listener.Addr(), path)
	if err := server.Serve(listener); err != http.ErrServerClosed {
		log.Print(err)
	}
	cancel()
	<-done
}
