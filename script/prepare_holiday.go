package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type holidayData struct {
	Year int `json:"year"`
	Days []struct {
		Name     string `json:"name"`
		Date     string `json:"date"`
		IsOffDay bool   `json:"isOffDay"`
	} `json:"days"`
}

const defaultHolidayURL = "https://github.com/NateScarlet/holiday-cn/raw/refs/heads/master/yyyy.json"

func main() {
	year := time.Now().Year()
	url := os.Getenv("HOLIDAY_API_URL")
	if strings.TrimSpace(url) == "" {
		url = defaultHolidayURL
	}
	url = buildHolidayURL(url, year)

	dest := filepath.Join("internal", "store", "embedded", "holiday", fmt.Sprintf("%d.json", year))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		fatal(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if isValidHolidayFile(dest, year) {
			fmt.Printf("holiday: fetch failed, keeping existing embedded data: %v\n", err)
			return
		}
		fatal(fmt.Errorf("fetch holiday data: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if isValidHolidayFile(dest, year) {
			fmt.Printf("holiday: fetch returned %s, keeping existing embedded data\n", resp.Status)
			return
		}
		fatal(fmt.Errorf("fetch holiday data: %s", resp.Status))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fatal(err)
	}
	if err := validateHolidayData(data, year); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		fatal(err)
	}
	fmt.Printf("holiday: embedded %d data from %s -> %s\n", year, url, dest)
}

func buildHolidayURL(raw string, year int) string {
	yearText := fmt.Sprintf("%d", year)
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "%d") {
		return fmt.Sprintf(raw, year)
	}
	replacer := strings.NewReplacer(
		"yyyy", yearText,
		"YYYY", yearText,
		"{year}", yearText,
		"{yyyy}", yearText,
		"{YYYY}", yearText,
	)
	return replacer.Replace(raw)
}

func isValidHolidayFile(path string, year int) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return validateHolidayData(data, year) == nil
}

func validateHolidayData(data []byte, year int) error {
	var holiday holidayData
	if err := json.Unmarshal(data, &holiday); err != nil {
		return fmt.Errorf("invalid holiday JSON: %w", err)
	}
	if holiday.Year != 0 && holiday.Year != year {
		return fmt.Errorf("holiday year mismatch: want %d, got %d", year, holiday.Year)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "holiday:", err)
	os.Exit(1)
}
