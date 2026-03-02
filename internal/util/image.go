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
		// No need to resize, just re-save.
		return writeJPEGAtomic(destPath, img, 85)
	}

	targetHeight := (srcHeight * targetWidth) / srcWidth
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Use BiLinear for better quality than NearestNeighbor
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return writeJPEGAtomic(destPath, dst, 85)
}

func writeJPEGAtomic(destPath string, img image.Image, quality int) error {
	tmpPath := destPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	_ = os.Remove(destPath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
