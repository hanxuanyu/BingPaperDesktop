package bing

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"BingPaperDesktop/internal/util"
)

type Meta struct {
	Copyright     string    `json:"copyright"`
	CopyrightLink string    `json:"copyrightlink"`
	Date          string    `json:"date"`
	FullStartDate string    `json:"fullstartdate"`
	Hsh           string    `json:"hsh"`
	Mkt           string    `json:"mkt"`
	Quiz          string    `json:"quiz"`
	StartDate     string    `json:"startdate"`
	Title         string    `json:"title"`
	Variants      []Variant `json:"variants"`
}

type Variant struct {
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	StorageKey string `json:"storage_key"`
	URL        string `json:"url"`
	Variant    string `json:"variant"` // "1920x1080" or "UHD"
}

func FetchMeta(url string) (*Meta, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "BingPaperDesktop/1.0")

	var resp *http.Response
	err = util.Retry(3, 500*time.Millisecond, func() error {
		var rerr error
		resp, rerr = client.Do(req)
		return rerr
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var meta Meta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func SelectVariant(meta *Meta, screenW, screenH int, forceUHD, preferAspectMatch bool) Variant {
	if forceUHD {
		for _, v := range meta.Variants {
			if v.Variant == "UHD" {
				return v
			}
		}
	}

	screenAspect := float64(screenW) / float64(screenH)
	if screenH == 0 {
		screenAspect = 16.0 / 9.0
	}

	var best Variant
	minAspectDiff := math.MaxFloat64

	type variantInfo struct {
		v    Variant
		w, h int
	}

	infos := []variantInfo{}
	for _, v := range meta.Variants {
		w, h := parseResolution(v.Variant)
		infos = append(infos, variantInfo{v, w, h})
	}

	// 1. Filter by aspect ratio if preferred
	candidates := []variantInfo{}
	if preferAspectMatch {
		for _, info := range infos {
			aspect := float64(info.w) / float64(info.h)
			diff := math.Abs(aspect - screenAspect)
			if diff < minAspectDiff {
				minAspectDiff = diff
			}
		}

		for _, info := range infos {
			aspect := float64(info.w) / float64(info.h)
			if math.Abs(aspect-screenAspect) <= minAspectDiff+0.02*screenAspect {
				candidates = append(candidates, info)
			}
		}
	} else {
		candidates = infos
	}

	// 2. Select from candidates
	var largerThanScreen []variantInfo
	for _, c := range candidates {
		if c.w >= screenW && c.h >= screenH {
			largerThanScreen = append(largerThanScreen, c)
		}
	}

	if len(largerThanScreen) > 0 {
		// Pick smallest that is still larger than screen
		minPixels := math.MaxFloat64
		for _, c := range largerThanScreen {
			pixels := float64(c.w * c.h)
			if pixels < minPixels {
				minPixels = pixels
				best = c.v
			}
		}
	} else {
		// Pick largest among those smaller than screen
		maxPixels := -1.0
		for _, c := range candidates {
			pixels := float64(c.w * c.h)
			if pixels > maxPixels {
				maxPixels = pixels
				best = c.v
			}
		}
	}

	return best
}

func parseResolution(res string) (int, int) {
	if res == "UHD" {
		return 3840, 2160
	}
	parts := strings.Split(res, "x")
	if len(parts) != 2 {
		return 1920, 1080 // Fallback
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

func DownloadImage(url, destPath string) error {
	tmpPath := destPath + ".part"

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "BingPaperDesktop/1.0")

	err = util.Retry(3, 500*time.Millisecond, func() error {
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %d", resp.StatusCode)
		}

		out, err := os.Create(tmpPath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		return err
	})

	if err != nil {
		return err
	}

	return os.Rename(tmpPath, destPath)
}
