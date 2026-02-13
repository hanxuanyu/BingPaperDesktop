package util

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"time"

	"golang.org/x/image/draw"
)

func Retry(attempts int, sleep time.Duration, f func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(sleep)
			sleep *= 2 // Exponential backoff
		}
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func NormalizeDate(d string) string {
	// If it's already YYYY-MM-DD, return as is
	if len(d) == 10 && d[4] == '-' && d[7] == '-' {
		return d
	}
	// If it's YYYYMMDD, convert to YYYY-MM-DD
	if len(d) == 8 {
		// Check if it's all digits
		isAllDigits := true
		for _, r := range d {
			if r < '0' || r > '9' {
				isAllDigits = false
				break
			}
		}
		if isAllDigits {
			return fmt.Sprintf("%s-%s-%s", d[0:4], d[4:6], d[6:8])
		}
	}
	return d
}

func ResizeIcon(data []byte, size int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	newImg := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(newImg, newImg.Bounds(), img, img.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, newImg); err != nil {
		return data
	}
	return buf.Bytes()
}
