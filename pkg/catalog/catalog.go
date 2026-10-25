package catalog

import (
	"encoding/json"
	"math"
	"os"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

// CommonForkCatalog holds the global catalog of forks loaded from data/forks.json.
var CommonForkCatalog []model.ForkCatalogEntry

func init() {
	candidates := []string{"data/forks.json", "../data/forks.json", "../../data/forks.json"}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			var cat []model.ForkCatalogEntry
			if err := json.Unmarshal(data, &cat); err == nil && len(cat) > 0 {
				CommonForkCatalog = cat
				break
			}
		}
	}
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
					sagPct := geometry.Round((compression/fork.TravelMm)*100.0, 1)
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
