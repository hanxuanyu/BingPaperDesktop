package overlay

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func SaveBase64Image(base64Data, destPath string) error {
	// Remove data URL prefix if present
	if i := strings.Index(base64Data, ","); i != -1 {
		base64Data = base64Data[i+1:]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return err
	}

	return os.WriteFile(destPath, data, 0644)
}

// AddWatermark 是纯 Go 实现的水印叠加方案（后备/备用）。
// 当前版本的主要水印渲染由前端 Canvas（watermark.ts）完成，此函数已不在主流程中调用。
// 保留作为无前端环境（CLI 模式）下的回退方案。
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

func Composite(backgroundPath string, overlays []string, destPath string) error {
	bgImg, err := imaging.Open(backgroundPath)
	if err != nil {
		return err
	}

	// imaging.Open returns a bound-correct image.
	// We'll use imaging.Overlay to stack layers.
	result := bgImg
	bgBounds := result.Bounds()
	bgRatio := 0.0
	if bgBounds.Dy() > 0 {
		bgRatio = float64(bgBounds.Dx()) / float64(bgBounds.Dy())
	}
	slog.Info("Composite started",
		"background", backgroundPath,
		"backgroundSize", fmt.Sprintf("%dx%d", bgBounds.Dx(), bgBounds.Dy()),
		"backgroundRatio", bgRatio,
		"overlayCount", len(overlays),
		"dest", destPath,
	)

	for _, overlayPath := range overlays {
		if overlayPath == "" {
			continue
		}
		ovImg, err := imaging.Open(overlayPath)
		if err != nil {
			slog.Warn("Composite skipped overlay: failed to open", "overlay", overlayPath, "error", err)
			continue
		}

		bgBounds := result.Bounds()
		ovBounds := ovImg.Bounds()
		preparedOverlay := ovImg
		resized := false

		if ovBounds.Dx() != bgBounds.Dx() || ovBounds.Dy() != bgBounds.Dy() {
			// Keep proportions as much as possible; fill canvas when aspect ratios differ.
			if ovBounds.Dy() > 0 && bgBounds.Dy() > 0 {
				ovRatio := float64(ovBounds.Dx()) / float64(ovBounds.Dy())
				bgRatio := float64(bgBounds.Dx()) / float64(bgBounds.Dy())
				if absFloat(ovRatio-bgRatio) > 0.001 {
					preparedOverlay = imaging.Fill(ovImg, bgBounds.Dx(), bgBounds.Dy(), imaging.Center, imaging.Lanczos)
				} else {
					preparedOverlay = imaging.Resize(ovImg, bgBounds.Dx(), bgBounds.Dy(), imaging.Lanczos)
				}
			} else {
				preparedOverlay = imaging.Resize(ovImg, bgBounds.Dx(), bgBounds.Dy(), imaging.Lanczos)
			}
			resized = true
		}

		finalOvBounds := preparedOverlay.Bounds()
		currentBgRatio := 0.0
		if bgBounds.Dy() > 0 {
			currentBgRatio = float64(bgBounds.Dx()) / float64(bgBounds.Dy())
		}
		overlayRatio := 0.0
		if finalOvBounds.Dy() > 0 {
			overlayRatio = float64(finalOvBounds.Dx()) / float64(finalOvBounds.Dy())
		}

		slog.Info("Composite overlay applied",
			"overlay", overlayPath,
			"overlaySize", fmt.Sprintf("%dx%d", finalOvBounds.Dx(), finalOvBounds.Dy()),
			"overlayRatio", overlayRatio,
			"backgroundRatio", currentBgRatio,
			"ratioDelta", overlayRatio-currentBgRatio,
			"resizedToBackground", resized,
			"offset", "(0,0)",
		)
		result = imaging.Overlay(result, preparedOverlay, image.Pt(0, 0), 1.0)
	}

	if err := imaging.Save(result, destPath, imaging.JPEGQuality(95)); err != nil {
		return err
	}

	finalBounds := result.Bounds()
	finalRatio := 0.0
	if finalBounds.Dy() > 0 {
		finalRatio = float64(finalBounds.Dx()) / float64(finalBounds.Dy())
	}
	slog.Info("Composite finished",
		"dest", destPath,
		"finalSize", fmt.Sprintf("%dx%d", finalBounds.Dx(), finalBounds.Dy()),
		"finalRatio", finalRatio,
	)
	return nil
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
