package catalog

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/model"
)

// CommonForkCatalog holds the global catalog of forks.
var CommonForkCatalog = []model.ForkCatalogEntry{
	{
		ID:       "sid_100",
		Name:     "RockShox SID 100mm",
		TravelMm: 100,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 506, RakeMm: 44},
			{A2cMm: 506, RakeMm: 51},
		},
		Description: "XC Race fork (static 506mm, 44mm or 51mm rake)",
	},
	{
		ID:       "sid_110",
		Name:     "RockShox SID 110mm",
		TravelMm: 110,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 521, RakeMm: 44},
		},
		Description: "Downcountry / Marathon fork (static 521mm, 44mm rake)",
	},
	{
		ID:       "fox34_120",
		Name:     "Fox Float 34 120mm",
		TravelMm: 120,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 530, RakeMm: 44},
		},
		Description: "Downcountry trail standard (static 530mm, 44mm rake)",
	},
	{
		ID:       "sid_120",
		Name:     "RockShox SID 120mm",
		TravelMm: 120,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 530, RakeMm: 44},
			{A2cMm: 534, RakeMm: 44},
		},
		Description: "XC race / Downcountry fork (static 530mm / 534mm, 44mm rake)",
	},
	{
		ID:       "pike_120",
		Name:     "RockShox Pike 120mm",
		TravelMm: 120,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 531, RakeMm: 44},
		},
		Description: "Trail suspension fork (static 531mm, 44mm rake)",
	},
	{
		ID:       "fox34_130",
		Name:     "Fox Float 34 130mm",
		TravelMm: 130,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 541, RakeMm: 44},
		},
		Description: "Trail hardtail benchmark (static 541mm, 44mm rake)",
	},
	{
		ID:       "pike_140",
		Name:     "RockShox Pike 140mm",
		TravelMm: 140,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 551, RakeMm: 44},
		},
		Description: "Aggressive trail fork (static 551mm, 44mm rake)",
	},
	{
		ID:       "fox36_150",
		Name:     "Fox 36 / Lyrik 150mm",
		TravelMm: 150,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 561, RakeMm: 44},
		},
		Description: "Hardcore hardtail / Enduro (static 561mm, 44mm rake)",
	},
	{
		ID:       "sunliner_440",
		Name:     "Tumbleweed Chromoly Steel Unicrown 440mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 440, RakeMm: 55},
		},
		Description: "Rigid touring unicrown fork (static 440mm, 55mm rake)",
	},
	{
		ID:       "surly_ecr_468",
		Name:     "Surly ECR 4130 Chromoly 468mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 468, RakeMm: 43},
		},
		Description: "Rigid 29+ bikepacking fork (static 468mm, 43mm rake)",
	},
	{
		ID:       "enve_mountain_472",
		Name:     "ENVE Mountain Carbon 472mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 472, RakeMm: 55},
			{A2cMm: 472, RakeMm: 44},
		},
		Description: "Rigid carbon mountain fork with flip-chip (472mm static, 44mm or 55mm rake)",
	},
	{
		ID:       "enve_mountain_boost",
		Name:     "ENVE Mountain Carbon Boost 492mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 492, RakeMm: 52},
			{A2cMm: 492, RakeMm: 55},
			{A2cMm: 490, RakeMm: 44},
		},
		Description: "Rigid carbon mountain fork with flip-chip Boost 15x110 (490/492mm static, 44mm or 52/55mm rake)",
	},
	{
		ID:       "firestarter_deluxe_483",
		Name:     "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 483, RakeMm: 51},
		},
		Description: "Salsa carbon gravel/bikepacking fork (static 483mm, 51mm rake)",
	},
	{
		ID:       "surly_krampus_483",
		Name:     "Surly Krampus 4130 Chromoly 483mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 483, RakeMm: 47},
		},
		Description: "Surly trail bikepacking rigid fork (static 483mm, 47mm rake)",
	},
	{
		ID:       "rigid_483",
		Name:     "Rigid Bikepacking 483mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 483, RakeMm: 44},
			{A2cMm: 483, RakeMm: 51},
		},
		Description: "Rigid expedition fork, suspension-corrected ~100mm (483mm static, 44mm or 51mm rake)",
	},
	{
		ID:       "rigid_485",
		Name:     "Rigid Bikepacking 485mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 485, RakeMm: 45},
			{A2cMm: 485, RakeMm: 52},
		},
		Description: "Rigid adventure fork, suspension-corrected 100mm (485mm static, 45mm or 52mm rake)",
	},
	{
		ID:       "sklar_steel_485",
		Name:     "Sklar Chromoly Steel 485mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 485, RakeMm: 52},
		},
		Description: "Custom steel adventure fork (static 485mm, 52mm rake)",
	},
	{
		ID:       "rigid_490",
		Name:     "Rigid Bikepacking 490mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 490, RakeMm: 44},
		},
		Description: "Rigid adventure fork, suspension-corrected 100-110mm (490mm static, 44mm rake)",
	},
	{
		ID:       "lithic_carbon_mountain",
		Name:     "Lithic Carbon Mountain",
		TravelMm: 0,
		Standard: true,
		Options: []model.ForkCatalogOption{
			{A2cMm: 495, RakeMm: 54},
			{A2cMm: 495, RakeMm: 51},
			{A2cMm: 483, RakeMm: 44},
			{A2cMm: 483, RakeMm: 51},
		},
		Description: "Rigid expedition fork with adjustable dropout flip-chip (static 495mm or 483mm, 51/54mm rake)",
	},
	{
		ID:       "rigid_495",
		Name:     "Rigid Bikepacking 495mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 495, RakeMm: 51},
			{A2cMm: 495, RakeMm: 54},
		},
		Description: "Rigid expedition fork, suspension-corrected 100-120mm (495mm static, 51mm rake)",
	},
	{
		ID:       "rigid_500",
		Name:     "Rigid Bikepacking 500mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 500, RakeMm: 44},
			{A2cMm: 500, RakeMm: 51},
		},
		Description: "Rigid expedition fork, suspension-corrected 120mm (500mm static)",
	},
	{
		ID:       "rigid_510",
		Name:     "Rigid Bikepacking 510mm",
		TravelMm: 0,
		Standard: false,
		Options: []model.ForkCatalogOption{
			{A2cMm: 510, RakeMm: 44},
		},
		Description: "Rigid expedition fork, suspension-corrected 120-130mm (510mm static)",
	},
}

// CommonStemCatalog holds the global catalog of stems.
var CommonStemCatalog = []model.StemCatalogEntry{
	{
		ID:            "mtb",
		LengthRangeMm: []float64{40, 100},
		AnglesDeg:     []float64{-6, 0, 6},
	},
	{
		ID:            "gravel",
		LengthRangeMm: []float64{60, 110},
		AnglesDeg:     []float64{-6, 0, 6},
	},
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
	ForkID    string
	ForkName  string
	SagPct    *float64
	StaticA2c float64
	Rake      float64
	TravelMm  float64
}

// FindForkSuggestions searches CommonForkCatalog for forks capable of satisfying the target (A2C, rake).
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
