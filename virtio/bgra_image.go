//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package virtio

import (
	"image"
	"image/color"
)

var (
	_ image.Image = &BGRAImage{}
)

// BGRAImage is an in-memory image that stores pixels in the BGRA
// order.
type BGRAImage struct {
	// Pix holds the image's pixels, in B, G, R, A order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4].
	Pix []uint8

	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int

	// Rect is the image's bounds.
	Rect image.Rectangle
}

func NewBGRAImage(r image.Rectangle) *BGRAImage {
	return &BGRAImage{
		Pix:    make([]byte, 4*r.Dx()*r.Dy()),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

// ColorModel implements Image.ColorModel.
func (p *BGRAImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds implements Image.Bounds.
func (p *BGRAImage) Bounds() image.Rectangle {
	return p.Rect
}

// At implements Image.At.
func (p *BGRAImage) At(x, y int) color.Color {
	return p.RGBAAt(x, y)
}

// RGBAAt returns the RGBA color from the point x,y.
func (p *BGRAImage) RGBAAt(x, y int) color.RGBA {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	// Small cap improves performance, see https://golang.org/issue/27857
	s := p.Pix[i : i+4 : i+4]
	return color.RGBA{s[2], s[1], s[0], s[3]}
}

func (p *BGRAImage) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.RGBAModel.Convert(c).(color.RGBA)
	// Small cap improves performance, see https://golang.org/issue/27857
	s := p.Pix[i : i+4 : i+4]
	s[0] = c1.B
	s[1] = c1.G
	s[2] = c1.R
	s[3] = c1.A
}

func (p *BGRAImage) SetRGBA(x, y int, c color.RGBA) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	// Small cap improves performance, see https://golang.org/issue/27857
	s := p.Pix[i : i+4 : i+4]
	s[0] = c.B
	s[1] = c.G
	s[2] = c.R
	s[3] = c.A
}

// PixOffset returns the index of the first element of Pix that
// corresponds to the pixel at (x, y).
func (p *BGRAImage) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}
