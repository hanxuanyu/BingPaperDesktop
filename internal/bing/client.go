package bing

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
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

type BingOfficialResponse struct {
	Images []struct {
		StartDate     string `json:"startdate"`
		FullStartDate string `json:"fullstartdate"`
		EndDate       string `json:"enddate"`
		URL           string `json:"url"`
		URLBase       string `json:"urlbase"`
		Copyright     string `json:"copyright"`
		CopyrightLink string `json:"copyrightlink"`
		Title         string `json:"title"`
		Quiz          string `json:"quiz"`
		WP            bool   `json:"wp"`
		Hsh           string `json:"hsh"`
	} `json:"images"`
}

type Variant struct {
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	StorageKey string `json:"storage_key"`
	URL        string `json:"url"`
	Variant    string `json:"variant"` // "1920x1080" or "UHD"
}

// Reuse a single HTTP client to avoid creating new connections each call.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// FetchMeta fetches today's image metadata from either:
// - "bing": Bing official HPImageArchive JSON
// - "custom": custom/BingPaper compatible JSON
func FetchMeta(apiType, url string) (*Meta, error) {
	slog.Info("Fetching meta", "type", apiType, "url", url)
	client := httpClient
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

	if apiType == "bing" {
		var official BingOfficialResponse
		if err := json.NewDecoder(resp.Body).Decode(&official); err != nil {
			return nil, err
		}
		if len(official.Images) == 0 {
			return nil, fmt.Errorf("no images found in official response")
		}

		img := official.Images[0]
		officialImageURL := normalizeOfficialImageURL("https://www.bing.com" + img.URL)

		meta := &Meta{
			Copyright:     img.Copyright,
			CopyrightLink: img.CopyrightLink,
			Date:          util.NormalizeDate(img.EndDate),
			FullStartDate: img.FullStartDate,
			Hsh:           img.Hsh,
			Mkt:           "zh-CN",
			Quiz:          img.Quiz,
			StartDate:     img.StartDate,
			Title:         img.Title,
			Variants: []Variant{
				{
					Variant: "UHD",
					URL:     officialImageURL,
					Format:  "jpg",
				},
			},
		}

		slog.Debug("Fetched meta from Bing official", "title", meta.Title, "date", meta.Date, "imageURL", officialImageURL)
		return meta, nil
	}

	var meta Meta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	meta.Date = util.NormalizeDate(meta.Date)
	slog.Debug("Fetched meta from custom API", "title", meta.Title, "date", meta.Date)
	return &meta, nil
}

// BuildDateMetaURL converts a BingPaper "today/meta" URL into a date-specific endpoint:
// .../date/YYYY-MM-DD/meta
func BuildDateMetaURL(todayMetaURL, date string) (string, error) {
	raw := strings.TrimSpace(todayMetaURL)
	if raw == "" {
		return "", fmt.Errorf("custom api url is empty")
	}

	normalizedDate := util.NormalizeDate(strings.TrimSpace(date))
	if _, err := time.Parse("2006-01-02", normalizedDate); err != nil {
		return "", fmt.Errorf("invalid date %q: %w", date, err)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid custom api url %q: %w", raw, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid custom api url %q", raw)
	}

	p := strings.TrimSuffix(u.Path, "/")
	switch {
	case strings.HasSuffix(p, "/today/meta"):
		p = strings.TrimSuffix(p, "/today/meta")
	case strings.HasSuffix(p, "/today"):
		p = strings.TrimSuffix(p, "/today")
	case strings.HasSuffix(p, "/meta"):
		p = strings.TrimSuffix(p, "/meta")
	}
	u.Path = pathpkg.Join(p, "date", normalizedDate, "meta")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func FetchMetaByDate(todayMetaURL, date string) (*Meta, error) {
	urlByDate, err := BuildDateMetaURL(todayMetaURL, date)
	if err != nil {
		return nil, err
	}
	return FetchMeta("custom", urlByDate)
}

// SelectVariant chooses the best variant based on screen size/aspect.
func SelectVariant(meta *Meta, screenW, screenH int, forceUHD bool) Variant {
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

	candidates := []variantInfo{}
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

	// Priority 1: UHD among candidates.
	for _, c := range candidates {
		if c.v.Variant == "UHD" {
			return c.v
		}
	}

	// Priority 2: larger than screen, pick smallest.
	var largerThanScreen []variantInfo
	for _, c := range candidates {
		if c.w >= screenW && c.h >= screenH {
			largerThanScreen = append(largerThanScreen, c)
		}
	}

	if len(largerThanScreen) > 0 {
		minPixels := math.MaxFloat64
		for _, c := range largerThanScreen {
			pixels := float64(c.w * c.h)
			if pixels < minPixels {
				minPixels = pixels
				best = c.v
			}
		}
	} else {
		// Priority 3: smaller than screen, pick largest.
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
		return 1920, 1080
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

// normalizeOfficialImageURL strips any trailing params after the first image extension.
// Example:
// /th?id=..._UHD.jpg&rf=...&w=1920 -> /th?id=..._UHD.jpg
func normalizeOfficialImageURL(raw string) string {
	if raw == "" {
		return raw
	}

	lower := strings.ToLower(raw)
	exts := []string{".jpg", ".jpeg", ".png", ".webp"}

	cut := len(raw)
	found := false
	for _, ext := range exts {
		if idx := strings.Index(lower, ext); idx >= 0 {
			end := idx + len(ext)
			if !found || end < cut {
				cut = end
				found = true
			}
		}
	}

	if !found {
		return raw
	}
	return raw[:cut]
}

// DownloadImage downloads an image to destPath using a temporary .part file.
func DownloadImage(url, destPath string) error {
	slog.Info("Downloading image", "url", url, "dest", destPath)
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
