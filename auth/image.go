package auth

import (
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"

	"bytes"
	"errors"
	// "fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type riLevel []int

var RICompressionLevel = struct {
	NoCompression      riLevel
	DefaultCompression riLevel
	BestCompression    riLevel
	BestSpeed          riLevel
}{riLevel{100, -1}, riLevel{70, 0}, riLevel{50, -3}, riLevel{100, -2}}

func ResizeImage(imageBuf []byte, fn string, nwidth, nheight int, quality riLevel) error {
	ext := strings.ToLower(filepath.Ext(fn))
	// if ext == ".gif" {
	// 	return ioutil.WriteFile(fn, imageBuf, 0777)
	// }

	img, _, err := image.Decode(bytes.NewReader(imageBuf))
	if err != nil {
		return err
	}

	rect := img.Bounds()
	width := rect.Size().X
	height := rect.Size().Y

	if width <= nwidth && height <= nheight {
		return ioutil.WriteFile(fn, imageBuf, 0777)
	}

	nheight = int(float64(nwidth) / float64(width) * float64(nwidth) * float64(height) / float64(nheight))
	sc := float64(width) / float64(nwidth)

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	final := image.NewRGBA(image.Rect(0, 0, nwidth, nheight))
	draw.Draw(rgba, rect, img, rect.Min, draw.Src)

	for x := 0; x < nwidth; x++ {
		for y := 0; y < nheight; y++ {
			l := x*4 + y*nwidth*4
			_x := int(float64(x) * sc)
			_y := int(float64(y) * sc)
			_l := _x*4 + _y*width*4

			copy(final.Pix[l:l+4], rgba.Pix[_l:_l+4])
		}
	}

	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	switch ext {
	case ".jpg", ".jpeg", ".gif":
		jpeg.Encode(f, final, &jpeg.Options{Quality: quality[0]})
	case ".png":
		enc := png.Encoder{}
		enc.CompressionLevel = png.CompressionLevel(quality[1])
		enc.Encode(f, final)
	default:
		return errors.New("Format not supported")
	}

	return nil
}

func RotateImage(imageBuf []byte, fn string, clockwise bool, quality riLevel) error {
	ext := strings.ToLower(filepath.Ext(fn))
	if ext == ".gif" {
		return ioutil.WriteFile(fn, imageBuf, 0777)
	}

	img, _, err := image.Decode(bytes.NewReader(imageBuf))
	if err != nil {
		return err
	}

	rect := img.Bounds()
	width := rect.Size().X
	height := rect.Size().Y

	final := image.NewRGBA(image.Rect(0, 0, height, width))
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rect, img, rect.Min, draw.Src)

	if clockwise {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				idx := ((height - 1 - y) + x*height) << 2
				idx2 := (y*width + x) << 2

				final.Pix[idx] = rgba.Pix[idx2]
				final.Pix[idx+1] = rgba.Pix[idx2+1]
				final.Pix[idx+2] = rgba.Pix[idx2+2]
				final.Pix[idx+3] = rgba.Pix[idx2+3]
			}
		}
	} else {
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				idx := ((width-x-1)*height + y) << 2
				idx2 := (y*width + x) << 2

				final.Pix[idx] = rgba.Pix[idx2]
				final.Pix[idx+1] = rgba.Pix[idx2+1]
				final.Pix[idx+2] = rgba.Pix[idx2+2]
				final.Pix[idx+3] = rgba.Pix[idx2+3]
			}
		}
	}

	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	switch ext {
	case ".jpg", ".jpeg":
		jpeg.Encode(f, final, &jpeg.Options{Quality: quality[0]})
	case ".png":
		enc := png.Encoder{}
		enc.CompressionLevel = png.CompressionLevel(quality[1])
		enc.Encode(f, final)
	default:
		return errors.New("Format not supported")
	}

	return nil
}
