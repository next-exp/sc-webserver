package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

type encodingOptions struct {
	Format  string
	Quality int
}

func (o encodingOptions) validate() error {
	if o.Format != "jpeg" && o.Format != "png" {
		return fmt.Errorf("unknown image format %q (use jpeg or png)", o.Format)
	}
	if o.Quality < 1 || o.Quality > 100 {
		return fmt.Errorf("JPEG quality must be 1..100")
	}
	return nil
}
func (o encodingOptions) extension() string {
	if o.Format == "png" {
		return "png"
	}
	return "jpg"
}
func (o encodingOptions) contentType() string {
	if o.Format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}
func (o encodingOptions) encode(w io.Writer, img image.Image) error {
	if o.Format == "png" {
		return png.Encode(w, img)
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: o.Quality})
}
