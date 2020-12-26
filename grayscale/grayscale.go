package grayscale

import (
	"image"
	"io"
)

// FromReader grayscales an image from original raw data (r).
//
// Note that you need to blank import image type packages in order to be able to
// decode them in this function, for example:
//
//     import (
//       _ "image/gif"
//       _ "image/jpeg"
//       _ "image/png"
//     )
func FromReader(r io.Reader) (*Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return &Image{img}, nil
}
