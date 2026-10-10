package canonicalizer

import (
	"encoding/json"
	"os"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

var DefaultForkPresets = []model.ForkPreset{
	{
		ID:                 "fox34_130",
		Name:               "Fox Float 34 130mm",
		TravelMm:           130,
		StaticA2c:          541,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Trail hardtail benchmark (static 541mm, 20% sag = 515mm, 44mm rake)",
	},
	{
		ID:                 "sid_110",
		Name:               "RockShox SID 110mm",
		TravelMm:           110,
		StaticA2c:          521,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Downcountry / Marathon fork (static 521mm, 20% sag = 499mm, 44mm rake)",
	},
	{
		ID:                 "sid_100",
		Name:               "RockShox SID 100mm",
		TravelMm:           100,
		StaticA2c:          506,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "XC Race fork (static 506mm, 20% sag = 486mm, 44mm rake)",
	},
	{
		ID:                 "fox34_120",
		Name:               "Fox Float 34 120mm",
		TravelMm:           120,
		StaticA2c:          530,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Downcountry standard (static 530mm, 20% sag = 506mm, 44mm rake)",
	},
	{
		ID:                 "sid_120",
		Name:               "RockShox SID 120mm",
		TravelMm:           120,
		StaticA2c:          530,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "XC race / Downcountry fork (static 530mm, 20% sag = 506mm, 44mm rake)",
	},
	{
		ID:                 "pike_140",
		Name:               "RockShox Pike 140mm",
		TravelMm:           140,
		StaticA2c:          551,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Aggressive trail fork (static 551mm, 20% sag = 523mm, 44mm rake)",
	},
	{
		ID:                 "fox36_150",
		Name:               "Fox 36 / Lyrik 150mm",
		TravelMm:           150,
		StaticA2c:          561,
		Rake:               44,
		StandardSagPercent: 20,
		Description:        "Hardcore hardtail / Enduro (static 561mm, 20% sag = 531mm, 44mm rake)",
	},
	{
		ID:                 "rigid_483",
		Name:               "Rigid Bikepacking 483mm",
		TravelMm:           0,
		StaticA2c:          483,
		Rake:               44,
		StandardSagPercent: 0,
		Description:        "Rigid expedition fork, suspension-corrected ~100mm (483mm static, 44mm rake)",
	},
	{
		ID:                 "rigid_495",
		Name:               "Rigid Bikepacking 495mm",
		TravelMm:           0,
		StaticA2c:          495,
		Rake:               44,
		StandardSagPercent: 0,
		Description:        "Rigid expedition fork, suspension-corrected ~120mm (495mm static, 44mm rake)",
	},
}

var CommonForkPresets = DefaultForkPresets

func init() {
	candidates := []string{"data/forks.json", "../../data/forks.json"}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			var presets []model.ForkPreset
			if err := json.Unmarshal(data, &presets); err == nil && len(presets) > 0 {
				CommonForkPresets = presets
				break
			}
		}
	}
}

// GetForkPreset looks up a fork preset by ID or Name.
func GetForkPreset(id string) (model.ForkPreset, bool) {
	for _, p := range CommonForkPresets {
		if p.ID == id || strings.EqualFold(p.Name, id) {
			return p, true
		}
	}
	return model.ForkPreset{}, false
}

// ResolveForkSpec enriches a ForkSpec with registry metadata.
// If values were populated from the registry or if linked to a registry preset,
// IsInferred is set to true.
func ResolveForkSpec(spec model.ForkSpec) model.ForkSpec {
	if spec.ID == "" {
		return spec
	}
	preset, found := GetForkPreset(spec.ID)
	if !found {
		return spec
	}
	res := spec
	if res.Name == "" {
		res.Name = preset.Name
	}
	if res.TravelMm == 0 {
		res.TravelMm = preset.TravelMm
	}
	if res.A2c == 0 {
		res.A2c = preset.StaticA2c
	}
	if res.Rake == 0 {
		res.Rake = preset.Rake
	}
	if res.StandardSagPercent == 0 {
		res.StandardSagPercent = preset.StandardSagPercent
	}
	if res.Description == "" {
		res.Description = preset.Description
	}
	res.IsInferred = true
	return res
}

// CalculateSaggedA2c calculates effective A2C taking suspension sag into consideration.
func CalculateSaggedA2c(staticA2c, travelMm, sagPercent float64) float64 {
	sagMm := travelMm * (sagPercent / 100.0)
	return geometry.Round(staticA2c-sagMm, 1)
}

// CanonicalizeBikes normalizes and canonicalizes a collection of bikes to an apples-to-apples configuration.
func CanonicalizeBikes(bikes []model.BikeGeometry, config model.CanonicalConfig) []model.CanonicalComparisonRow {
	var rows []model.CanonicalComparisonRow

	for _, bike := range bikes {
		valReport := validator.ValidateBikeGeometry(bike)

		seatHeight := config.SeatHeight
		if seatHeight <= 0 {
			seatHeight = bike.SeatHeight
		}
		if seatHeight <= 0 {
			seatHeight = 750.0
		}

		baselineAett := geometry.CalculateAett(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			bike.StaDeg,
			seatHeight,
		)

		verticalStack, axialLength := geometry.CalculateSteererStack(
			bike.Stack,
			bike.StaDeg,
			bike.HtaDeg,
			seatHeight,
		)

		baseA2c := bike.ForkA2c
		if baseA2c <= 0 {
			baseA2c = valReport.GeometricA2c
		}

		baseline := model.BaselineMetrics{
			Reach:              bike.Reach,
			Stack:              bike.Stack,
			HtaDeg:             bike.HtaDeg,
			StaDeg:             bike.StaDeg,
			BbDrop:             bike.BbDrop,
			A2c:                baseA2c,
			Aett:               geometry.Round(baselineAett, 1),
			SpareVerticalStack: geometry.Round(verticalStack, 1),
			SpareAxialSteerer:  geometry.Round(axialLength, 1),
		}

		canonical := geometry.AnalyzeBikeFit(
			bike,
			&config.TargetA2c,
			&config.CupStack,
			&seatHeight,
		)

		rows = append(rows, model.CanonicalComparisonRow{
			Bike:       bike,
			Validation: valReport,
			Baseline:   baseline,
			Canonical:  canonical,
		})
	}

	return rows
}
