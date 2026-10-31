package catalog

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"bikegeo/pkg/model"
)

// CommonForkCatalog holds the global catalog of forks loaded from data/forks.json.
var CommonForkCatalog []model.ForkCatalogEntry

// CommonStemCatalog holds the global catalog of stems loaded from data/stems.json.
var CommonStemCatalog []model.StemCatalogEntry

func init() {
	forkCandidates := []string{"data/forks.json", "../data/forks.json", "../../data/forks.json"}
	for _, path := range forkCandidates {
		if data, err := os.ReadFile(path); err == nil {
			var cat []model.ForkCatalogEntry
			if err := json.Unmarshal(data, &cat); err == nil && len(cat) > 0 {
				CommonForkCatalog = cat
				break
			}
		}
	}

	stemCandidates := []string{"data/stems.json", "../data/stems.json", "../../data/stems.json"}
	for _, path := range stemCandidates {
		if data, err := os.ReadFile(path); err == nil {
			var cat []model.StemCatalogEntry
			if err := json.Unmarshal(data, &cat); err == nil && len(cat) > 0 {
				CommonStemCatalog = cat
				break
			}
		}
	}
}

// GetCatalogStem looks up a stem set by ID.
func GetCatalogStem(id string) (model.StemCatalogEntry, bool) {
	for _, p := range CommonStemCatalog {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return model.StemCatalogEntry{}, false
}

// ExpandStemSets expands and deduplicates a slice of stem catalog IDs into []model.StemSpec.
func ExpandStemSets(ids []string) []model.StemSpec {
	var results []model.StemSpec
	seen := make(map[string]bool)
	for _, id := range ids {
		entry, found := GetCatalogStem(id)
		if !found {
			continue
		}
		for _, s := range entry.Expand() {
			key := fmt.Sprintf("%.1f/%.1f", s.LengthMm, s.AngleDeg)
			if !seen[key] {
				seen[key] = true
				results = append(results, s)
			}
		}
	}
	return results
}

// GetCatalogFork looks up a fork entry by ID or Name.
func GetCatalogFork(id string) (model.ForkCatalogEntry, bool) {
	for _, p := range CommonForkCatalog {
		if strings.EqualFold(p.ID, id) || strings.EqualFold(p.Name, id) {
			return p, true
		}
	}
	return model.ForkCatalogEntry{}, false
}

// GetStandardForkForTravel returns the standard catalog fork for a given travel in mm.
func GetStandardForkForTravel(travelMm float64) (model.ForkCatalogEntry, bool) {
	for _, p := range CommonForkCatalog {
		if p.Standard && p.TravelMm > 0 && math.Abs(p.TravelMm-travelMm) < 0.1 {
			return p, true
		}
	}
	// Fallback to first matching travel if no standard flag
	for _, p := range CommonForkCatalog {
		if p.TravelMm > 0 && math.Abs(p.TravelMm-travelMm) < 0.1 {
			return p, true
		}
	}
	return model.ForkCatalogEntry{}, false
}

// ForkSuggestion represents a candidate fork from the catalog satisfying a target geometry.
type ForkSuggestion struct {
	ForkID    string   `json:"fork_id"`
	ForkName  string   `json:"fork_name"`
	SagPct    *float64 `json:"sag_pct,omitempty"`
	StaticA2c float64  `json:"static_a2c"`
	Rake      float64  `json:"rake"`
	TravelMm  float64  `json:"travel_mm"`
}

// FindForkSuggestions searches data/forks.json for forks capable of satisfying the target (A2C, rake).
func FindForkSuggestions(targetA2c, targetRake float64) []ForkSuggestion {
	var suggestions []ForkSuggestion

	for _, fork := range CommonForkCatalog {
		for _, opt := range fork.Options {
			if targetRake > 0 && math.Abs(opt.RakeMm-targetRake) > 0.1 {
				continue
			}

			if fork.TravelMm == 0 {
				// Rigid fork
				if math.Abs(opt.A2cMm-targetA2c) <= 0.5 {
					suggestions = append(suggestions, ForkSuggestion{
						ForkID:    fork.ID,
						ForkName:  fork.Name,
						StaticA2c: opt.A2cMm,
						Rake:      opt.RakeMm,
						TravelMm:  0,
					})
				}
			} else {
				// Suspension fork
				compression := opt.A2cMm - targetA2c
				if compression >= 0 && compression <= fork.TravelMm*0.45 {
					sagPct := math.Round(((compression/fork.TravelMm)*100.0)*10.0) / 10.0
					suggestions = append(suggestions, ForkSuggestion{
						ForkID:    fork.ID,
						ForkName:  fork.Name,
						SagPct:    &sagPct,
						StaticA2c: opt.A2cMm,
						Rake:      opt.RakeMm,
						TravelMm:  fork.TravelMm,
					})
				}
			}
		}
	}

	return suggestions
}
