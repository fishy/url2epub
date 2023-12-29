package grayscale

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
)

// FromReader grayscales an image from original raw data (r).
//
// Note that you need to blank import image type packages in order to be able to
// decode them in this function, for example:
//
//	import (
//	  _ "image/gif"
//	  _ "image/jpeg"
//	  _ "image/png"
//	)
//
// It returns the original data via orig, in case any decoding fails and you
// want to fallback to the original image.
func FromReader(r io.Reader) (_ *image.Gray16, orig *bytes.Buffer, _ error) {
	orig = new(bytes.Buffer)
	r = io.TeeReader(r, orig)
	defer func() {
		io.Copy(io.Discard, r)
	}()
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, orig, err
	}
	return Grayscale(img), orig, nil
}

func Grayscale(img image.Image) *image.Gray16 {
	gray := image.NewGray16(img.Bounds())
	origMinX := img.Bounds().Min.X
	origMinY := img.Bounds().Min.Y
	newMinX := gray.Bounds().Min.X
	newMinY := gray.Bounds().Min.Y
	for x := newMinX; x < gray.Bounds().Max.X; x++ {
		for y := newMinY; y < gray.Bounds().Max.Y; y++ {
			origX := x - newMinX + origMinX
			origY := y - newMinY + origMinY
			gray.Set(x, y, img.At(origX, origY))
		}
	}
	return gray
}

// ToJPEG encodes the image to JPEG with default quality.
func ToJPEG(img image.Image) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, nil); err != nil {
		return nil, err
	}
	return buf, nil
}
