package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
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
	CapturedAt  time.Time `json:"capturedAt"`
	AttemptedAt time.Time `json:"attemptedAt"`
	Error       string    `json:"error"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CaptureMS   int64     `json:"captureMs"`
	Sequence    uint64    `json:"sequence"`
	IntervalMS  int64     `json:"intervalMs"`
}
type store struct {
	sync.RWMutex
	state status
	jpeg  []byte
}

func (s *store) capture(path string, quality int, rect image.Rectangle) {
	started := time.Now()
	img, err := captureScreen(rect)
	var buf bytes.Buffer
	if err == nil {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	}
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
	s.jpeg = buf.Bytes()
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
	mux.HandleFunc("GET /screenshot.jpg", func(w http.ResponseWriter, r *http.Request) {
		s.RLock()
		frame, st := s.jpeg, s.state
		s.RUnlock()
		if len(frame) == 0 || st.Error != "" || time.Since(st.CapturedAt) > 6*time.Second {
			http.Error(w, "capture unavailable or stale", 503)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("X-Captured-At", st.CapturedAt.UTC().Format(time.RFC3339Nano))
		w.Write(frame)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; style-src 'self'; img-src 'self' blob:; object-src 'none'; frame-ancestors 'self'")
		mux.ServeHTTP(w, r)
	})
}
func main() {
	listen := flag.String("listen", "127.0.0.1:8085", "HTTP listen address (use the ZeroTier IP for remote viewing)")
	temp := flag.String("temp-dir", filepath.Join(os.TempDir(), "sc-webserver"), "temporary directory; only latest.jpg is retained")
	quality := flag.Int("quality", 85, "JPEG quality, 1..100")
	x := flag.Int("x", 0, "crop left (desktop coordinates)")
	y := flag.Int("y", 0, "crop top")
	width := flag.Int("width", 0, "crop width; 0 captures whole desktop")
	height := flag.Int("height", 0, "crop height")
	flag.Parse()
	if *quality < 1 || *quality > 100 || *width < 0 || *height < 0 || (*width == 0) != (*height == 0) {
		log.Fatal("invalid JPEG quality or crop dimensions")
	}
	if err := os.MkdirAll(*temp, 0700); err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(*temp, "latest.jpg")
	defer os.Remove(path)
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	s := &store{state: status{IntervalMS: 2000}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			s.capture(path, *quality, image.Rect(*x, *y, *x+*width, *y+*height))
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
