package store

import (
	"embed"
	"fmt"
)

//go:embed embedded/holiday/*.json
var embeddedHolidayFS embed.FS

func LoadEmbeddedHoliday(year int) (*HolidayData, []byte, error) {
	path := fmt.Sprintf("embedded/holiday/%d.json", year)
	data, err := embeddedHolidayFS.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	holiday, err := decodeHolidayData(data, year)
	if err != nil {
		return nil, nil, fmt.Errorf("embedded holiday data invalid: %w", err)
	}
	return holiday, data, nil
}

func HasEmbeddedHoliday(year int) bool {
	_, _, err := LoadEmbeddedHoliday(year)
	return err == nil
}
