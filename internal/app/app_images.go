package app

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"
)

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

	if _, err := os.Stat(thumbAbsPath); os.IsNotExist(err) {
		srcAbsPath := filepath.Join(store.GetBaseDir(), relPath)
		if _, err := os.Stat(srcAbsPath); err != nil {
			return "", err
		}

		if err := os.MkdirAll(filepath.Dir(thumbAbsPath), 0755); err != nil {
			return "", err
		}

		slog.Info("Generating thumbnail", "src", relPath)
		if err := util.GenerateThumbnail(srcAbsPath, thumbAbsPath, 400); err != nil {
			slog.Error("Failed to generate thumbnail", "error", err)
			return "/images/" + relPath, nil
		}
	}

	urlPath := filepath.ToSlash(thumbRelPath)
	return "/images/" + urlPath, nil
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

			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(w, r, absPath)
			return
		}
		http.NotFound(w, r)
	})
}
