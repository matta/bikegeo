package ideal

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed default.json
var DefaultProfileJSON []byte

// ParseProfileJSON parses and strictly validates an ideal frame profile from JSON bytes.
// Unknown fields are rejected. Defaults (such as floor = -200) are populated.
func ParseProfileJSON(data []byte) (*Profile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var p Profile
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("invalid ideal profile JSON: %w", err)
	}

	for i := range p.Attributes {
		attr := &p.Attributes[i]
		if attr.Ideal != nil {
			if attr.Acceptable == nil {
				attr.Acceptable = &Range{
					Min: attr.Ideal.Min,
					Max: attr.Ideal.Max,
				}
			}
			if attr.Floor == nil {
				fl := -200.0
				attr.Floor = &fl
			}
		}
	}

	if err := ValidateProfile(&p); err != nil {
		return nil, err
	}

	return &p, nil
}

// LoadProfile loads an ideal profile by name from the given directory (e.g. data/ideals).
// If name is "default" (or empty) and not found on disk, it falls back to the embedded default.
func LoadProfile(dir string, name string) (*Profile, error) {
	cleanName := strings.TrimSuffix(strings.TrimSpace(name), ".json")
	if cleanName == "" {
		cleanName = "default"
	}

	if dir != "" {
		path := filepath.Join(dir, cleanName+".json")
		if data, err := os.ReadFile(path); err == nil {
			p, err := ParseProfileJSON(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			return p, nil
		}
	}

	if strings.EqualFold(cleanName, "default") {
		return ParseProfileJSON(DefaultProfileJSON)
	}

	return nil, fmt.Errorf("ideal profile %q not found in %s", cleanName, dir)
}

// ListProfiles discovers and returns summaries of all valid ideal profiles in the directory.
// Always includes the embedded default profile if not present on disk.
func ListProfiles(dir string) ([]ProfileSummary, error) {
	seen := make(map[string]bool)
	var summaries []ProfileSummary

	if dir != "" {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				path := filepath.Join(dir, entry.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				p, err := ParseProfileJSON(data)
				if err != nil {
					continue
				}
				seen[strings.ToLower(p.Name)] = true
				summaries = append(summaries, summarizeProfile(p))
			}
		}
	}

	if !seen["default"] {
		if def, err := ParseProfileJSON(DefaultProfileJSON); err == nil {
			summaries = append(summaries, summarizeProfile(def))
		}
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	return summaries, nil
}

func summarizeProfile(p *Profile) ProfileSummary {
	scoredCount := 0
	for _, attr := range p.Attributes {
		if attr.Weight > 0 && attr.Ideal != nil {
			scoredCount++
		}
	}
	return ProfileSummary{
		Name:           p.Name,
		Description:    p.Description,
		SaddleHeightMm: p.SaddleHeightMm,
		AttributeCount: len(p.Attributes),
		ScoredCount:    scoredCount,
	}
}
