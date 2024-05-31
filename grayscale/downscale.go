package grayscale

import (
	"image"
	"image/color"
	"math"
)

// Downscale downscales g to be able to fit in fit x fit preserving the original
// aspect ratio.
//
// If fit <= 0 or if the original image is already smaller than fit x fit,
// the original image will be returned as-is.
func Downscale(img *image.Gray16, fit int) image.Image {
	if fit <= 0 {
		return img
	}
	var scaled bool
	ratio := 1.0
	origMin := img.Bounds().Min
	// origMax := img.Bounds().Max
	origSizeX := float64(img.Bounds().Max.X - origMin.X)
	origSizeY := float64(img.Bounds().Max.Y - origMin.Y)
	if ratioX := float64(fit) / float64(img.Bounds().Max.X-origMin.X); ratioX < ratio {
		scaled = true
		ratio = ratioX
	}
	if ratioY := float64(fit) / float64(img.Bounds().Max.Y-origMin.Y); ratioY < ratio {
		scaled = true
		ratio = ratioY
	}
	if !scaled {
		return img
	}
	newMax := image.Point{
		X: int(math.Round(float64(img.Bounds().Max.X-origMin.X) * ratio)),
		Y: int(math.Round(float64(img.Bounds().Max.Y-origMin.Y) * ratio)),
	}
	newImg := image.NewGray16(image.Rectangle{
		Min: image.Point{
			X: 0,
			Y: 0,
		},
		Max: newMax,
	})
	yWeights := make([][]float64, newMax.Y)
	for x := 0; x < newMax.X; x++ {
		minX := float64(x) / ratio
		minXInt := int(minX)
		maxX := min(float64(x+1)/ratio, origSizeX)
		maxXInt := int(maxX)
		xWeights := make([]float64, maxXInt-minXInt)
		xWeights[0] = math.Floor(minX+1) - minX
		xWeights[maxXInt-minXInt-1] = maxX - math.Floor(maxX)
		for i := 1; i < maxXInt-minXInt-1; i++ {
			xWeights[i] = 1
		}

		for y := 0; y < newMax.Y; y++ {
			minY := float64(y) / ratio
			minYInt := int(minY)
			maxY := min(float64(y+1)/ratio, origSizeY)
			maxYInt := int(maxY)
			if yWeights[y] == nil {
				yWeights[y] = make([]float64, maxYInt-minYInt)
				yWeights[y][0] = math.Floor(minY+1) - minY
				yWeights[y][maxYInt-minYInt-1] = maxY - math.Floor(maxY)
				for i := 1; i < maxYInt-minYInt-1; i++ {
					yWeights[y][i] = 1
				}
			}
			var c, n float64
			for xx := minXInt; xx < maxXInt; xx++ {
				for yy := minYInt; yy < maxYInt; yy++ {
					weight := xWeights[xx-minXInt] * yWeights[y][yy-minYInt]
					color := img.Gray16At(xx+origMin.X, yy+origMin.Y).Y
					n += weight
					c += float64(color) * weight
				}
			}
			newImg.SetGray16(x, y, color.Gray16{
				Y: uint16(math.Round(c / n)),
			})
		}
	}
	return newImg
}
