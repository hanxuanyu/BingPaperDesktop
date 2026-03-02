package app

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"
)

var thumbnailLocks sync.Map

func (a *App) GetImageDataURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}
	absPath := filepath.Join(store.GetBaseDir(), relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	mime := "image/jpeg"
	if filepath.Ext(absPath) == ".png" {
		mime = "image/png"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func (a *App) GetThumbnailURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}

	thumbRelPath := filepath.Join("thumbnails", relPath)
	thumbAbsPath := filepath.Join(store.GetBaseDir(), thumbRelPath)
	srcAbsPath := filepath.Join(store.GetBaseDir(), relPath)
	srcURLPath := filepath.ToSlash(relPath)

	if _, err := os.Stat(srcAbsPath); err != nil {
		return "", err
	}

	lock := getThumbnailLock(thumbAbsPath)
	lock.Lock()
	defer lock.Unlock()

	if !isImageFileReady(thumbAbsPath) {
		_ = os.Remove(thumbAbsPath)

		if err := os.MkdirAll(filepath.Dir(thumbAbsPath), 0755); err != nil {
			return "", err
		}

		slog.Info("Generating thumbnail", "src", relPath, "dest", thumbRelPath)
		if err := util.GenerateThumbnail(srcAbsPath, thumbAbsPath, 400); err != nil {
			slog.Error("Failed to generate thumbnail", "src", relPath, "error", err)
			return withVersion("/images/"+srcURLPath, fileVersion(srcAbsPath)), nil
		}
	}

	if !isImageFileReady(thumbAbsPath) {
		slog.Warn("Thumbnail still invalid after generation, fallback to original", "thumb", thumbRelPath)
		return withVersion("/images/"+srcURLPath, fileVersion(srcAbsPath)), nil
	}

	urlPath := filepath.ToSlash(thumbRelPath)
	return withVersion("/images/"+urlPath, fileVersion(thumbAbsPath)), nil
}

func (a *App) GetImageURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}
	urlPath := filepath.ToSlash(relPath)
	return "/images/" + urlPath, nil
}

func (a *App) AssetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/images/") {
			relPath := strings.TrimPrefix(path, "/images/")
			relPath = filepath.FromSlash(relPath)
			relPath = filepath.Clean(relPath)
			if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			absPath := filepath.Join(store.GetBaseDir(), relPath)
			baseDir := store.GetBaseDir()
			if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(baseDir)) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}

			urlRelPath := filepath.ToSlash(relPath)
			if strings.HasPrefix(urlRelPath, "thumbnails/") {
				w.Header().Set("Cache-Control", "no-cache")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, r, absPath)
			return
		}
		http.NotFound(w, r)
	})
}

func getThumbnailLock(path string) *sync.Mutex {
	v, _ := thumbnailLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func isImageFileReady(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return false
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	_, _, err = image.DecodeConfig(f)
	return err == nil
}

func fileVersion(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UnixNano()
	}
	return info.ModTime().UnixNano()
}

func withVersion(url string, version int64) string {
	if strings.Contains(url, "?") {
		return fmt.Sprintf("%s&v=%d", url, version)
	}
	return fmt.Sprintf("%s?v=%d", url, version)
}
