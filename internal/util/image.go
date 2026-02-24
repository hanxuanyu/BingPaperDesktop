package util

import (
	"image"
	"image/jpeg"
	_ "image/png"
	"os"

	"golang.org/x/image/draw"
)

// GenerateThumbnail generates a thumbnail for the given image path and saves it to destPath.
func GenerateThumbnail(srcPath string, destPath string, targetWidth int) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	img, _, err := image.Decode(srcFile)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	if srcWidth <= targetWidth {
		// No need to resize, just copy or re-save
		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()
		return jpeg.Encode(destFile, img, &jpeg.Options{Quality: 85})
	}

	targetHeight := (srcHeight * targetWidth) / srcWidth
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Use BiLinear for better quality than NearestNeighbor
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	return jpeg.Encode(destFile, dst, &jpeg.Options{Quality: 85})
}
