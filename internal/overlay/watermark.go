package overlay

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func AddWatermark(srcPath, destPath, title, date, copyright string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// New canvas
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)

	// Text content
	text := title + " | " + date + "\n" + copyright
	lines := wrapText(text, int(float64(w)*0.7/7)) // Rough estimate of char width

	lineHeight := 20
	padding := 20
	boxW := int(float64(w) * 0.75)
	boxH := len(lines)*lineHeight + padding

	// Draw semi-transparent background box at bottom-left
	boxRect := image.Rect(padding, h-boxH-padding, padding+boxW, h-padding)
	draw.Draw(rgba, boxRect, &image.Uniform{color.RGBA{0, 0, 0, 128}}, image.Point{}, draw.Over)

	// Draw text
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(color.White),
		Face: basicfont.Face7x13,
	}

	for i, line := range lines {
		d.Dot = fixed.Point26_6{
			X: fixed.I(padding + 10),
			Y: fixed.I(h - boxH - padding + 20 + i*lineHeight),
		}
		d.DrawString(line)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, rgba, &jpeg.Options{Quality: 90})
}

func wrapText(text string, maxChars int) []string {
	var lines []string
	paragraphs := strings.Split(text, "\n")
	for _, p := range paragraphs {
		words := strings.Fields(p)
		if len(words) == 0 {
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if len(line)+1+len(w) > maxChars {
				lines = append(lines, line)
				line = w
			} else {
				line += " " + w
			}
		}
		lines = append(lines, line)
	}
	return lines
}
