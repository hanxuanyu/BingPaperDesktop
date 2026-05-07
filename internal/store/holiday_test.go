package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHolidayAPIURL_DefaultWhenEmpty(t *testing.T) {
	got := NormalizeHolidayAPIURL("  ")
	if got != DefaultHolidayUrl {
		t.Fatalf("unexpected holiday API URL:\nwant: %s\ngot:  %s", DefaultHolidayUrl, got)
	}
}

func TestBuildHolidayAPIURL_ReplacesYearPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "lower yyyy",
			raw:  "https://example.com/holiday/yyyy.json",
			want: "https://example.com/holiday/2026.json",
		},
		{
			name: "upper YYYY",
			raw:  "https://example.com/holiday/YYYY.json",
			want: "https://example.com/holiday/2026.json",
		},
		{
			name: "brace year",
			raw:  "https://example.com/holiday/{year}.json",
			want: "https://example.com/holiday/2026.json",
		},
		{
			name: "printf year",
			raw:  "https://example.com/holiday/%d.json",
			want: "https://example.com/holiday/2026.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildHolidayAPIURL(tt.raw, 2026)
			if got != tt.want {
				t.Fatalf("unexpected holiday API URL:\nwant: %s\ngot:  %s", tt.want, got)
			}
		})
	}
}

func TestEnsureEmbeddedHolidayCache_WritesCurrentEmbeddedYear(t *testing.T) {
	setTestBaseDir(t)

	ok, err := EnsureEmbeddedHolidayCache(2026, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected embedded 2026 holiday data to be available")
	}

	path := filepath.Join(baseDir, "data", "holiday", "2026.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected holiday cache file: %v", err)
	}
	if err := validateHolidayDataForTest(data, 2026); err != nil {
		t.Fatalf("invalid holiday cache: %v", err)
	}
}

func TestEnsureEmbeddedHolidayCache_MissingYearReturnsFalse(t *testing.T) {
	setTestBaseDir(t)

	ok, err := EnsureEmbeddedHolidayCache(2099, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected missing embedded holiday year to return false")
	}
}

func TestEnsureHolidayWithConfig_DefaultSourceForceUsesEmbeddedData(t *testing.T) {
	setTestBaseDir(t)

	err := EnsureHolidayWithConfig(2026, Config{HolidayApiUrl: DefaultHolidayUrl}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(baseDir, "data", "holiday", "2026.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected holiday cache file: %v", err)
	}
	if err := validateHolidayDataForTest(data, 2026); err != nil {
		t.Fatalf("invalid holiday cache: %v", err)
	}
}

func setTestBaseDir(t *testing.T) {
	t.Helper()
	oldBaseDir := baseDir
	oldHasUserDir := hasUserDir
	baseDir = t.TempDir()
	hasUserDir = true
	t.Cleanup(func() {
		baseDir = oldBaseDir
		hasUserDir = oldHasUserDir
	})
}

func validateHolidayDataForTest(data []byte, year int) error {
	var holiday HolidayData
	if err := json.Unmarshal(data, &holiday); err != nil {
		return err
	}
	if holiday.Year != year {
		return fmt.Errorf("want year %d, got %d", year, holiday.Year)
	}
	return nil
}
