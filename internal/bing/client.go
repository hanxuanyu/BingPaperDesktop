package bing

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// httpClient 复用单个 HTTP 客户端实例，避免每次请求都创建新连接
var httpClient = &http.Client{Timeout: 15 * time.Second}

// FetchMeta 根据 apiType 从指定 URL 拉取今日壁纸元数据，并统一转换为内部 Meta 结构体。
// 支持两种格式：
//   - "bing": 必应官方 HPImageArchive JSON（仅含单张 UHD 图片）
//   - "custom"(默认): BingPaper 服务端自定义格式（含多分辨率变体列表）
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
					URL:     "https://www.bing.com" + img.URL,
					Format:  "jpg",
				},
			},
		}
		slog.Debug("Fetched meta from Bing official", "title", meta.Title, "date", meta.Date)
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

// SelectVariant 从 meta.Variants 中为给定的物理屏幕尺寸选择最优图片变体。
// 选择策略（优先级递减）：
//  1. forceUHD=true 时直接返回 UHD 变体（若存在）
//  2. 宽高比最接近屏幕的变体集合（允许 2% 误差）
//  3. 在候选集合中，优先 UHD；其次选分辨率≥屏幕且最小的变体；最后选最大的变体
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

	// 1. Filter by aspect ratio (default)
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

	// 2. Select from candidates
	// Priority 1: UHD among candidates
	for _, c := range candidates {
		if c.v.Variant == "UHD" {
			return c.v
		}
	}

	// Priority 2: Larger than screen, pick smallest
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
		// Priority 3: Smaller than screen, pick largest
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

// DownloadImage 从 url 下载图片并保存到 destPath，使用临时文件（.part）保证下载原子性。
// 内置最多 3 次指数退避重试（500ms / 1s / 2s）。
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
