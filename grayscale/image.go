package grayscale

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
)

// Image represents a grayscaled image.
type Image struct {
	image.Image
}

// ColorModel overrides the original ColorModel with color.Gray16Model.
func (g *Image) ColorModel() color.Model {
	return color.Gray16Model
}

// At overrides the original At with color.Gray16Model conversion applied.
func (g *Image) At(x, y int) color.Color {
	return g.ColorModel().Convert(g.Image.At(x, y))
}

// ToJPEG encodes the image to JPEG with default quality.
func (g *Image) ToJPEG() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, g, nil); err != nil {
		return nil, err
	}
	return buf, nil
}
