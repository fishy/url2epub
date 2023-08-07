package grayscale

import (
	"bytes"
	"image"
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
func FromReader(r io.Reader) (_ *Image, orig *bytes.Buffer, _ error) {
	orig = new(bytes.Buffer)
	r = io.TeeReader(r, orig)
	defer func() {
		io.Copy(io.Discard, r)
	}()
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, orig, err
	}
	return &Image{img}, orig, nil
}
