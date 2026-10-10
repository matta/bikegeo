package canonicalizer

import (
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

var CommonForkPresets = []model.ForkPreset{
	{
		ID:                 "fox34_130",
		Name:               "Fox Float 34 130mm",
		TravelMm:           130,
		StaticA2c:          541,
		StandardSagPercent: 20,
		Description:        "Trail hardtail benchmark (static 541mm, 20% sag = 515mm)",
	},
	{
		ID:                 "sid_110",
		Name:               "RockShox SID 110mm",
		TravelMm:           110,
		StaticA2c:          521,
		StandardSagPercent: 20,
		Description:        "Downcountry / Marathon fork (static 521mm, 20% sag = 499mm)",
	},
	{
		ID:                 "sid_100",
		Name:               "RockShox SID 100mm",
		TravelMm:           100,
		StaticA2c:          506,
		StandardSagPercent: 20,
		Description:        "XC Race fork (static 506mm, 20% sag = 486mm)",
	},
	{
		ID:                 "fox34_120",
		Name:               "Fox Float 34 120mm",
		TravelMm:           120,
		StaticA2c:          530,
		StandardSagPercent: 20,
		Description:        "Downcountry standard (static 530mm, 20% sag = 506mm)",
	},
	{
		ID:                 "pike_140",
		Name:               "RockShox Pike 140mm",
		TravelMm:           140,
		StaticA2c:          551,
		StandardSagPercent: 20,
		Description:        "Aggressive trail fork (static 551mm, 20% sag = 523mm)",
	},
	{
		ID:                 "fox36_150",
		Name:               "Fox 36 / Lyrik 150mm",
		TravelMm:           150,
		StaticA2c:          561,
		StandardSagPercent: 20,
		Description:        "Hardcore hardtail / Enduro (static 561mm, 20% sag = 531mm)",
	},
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
