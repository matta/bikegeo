package ideal

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/curve"
	"bikegeo/pkg/model"
)

// Evaluate evaluates a metric value against an AttributeSpec and returns an Evaluation.
// row can be nil if evaluating abstract test values.
func Evaluate(spec AttributeSpec, meta AttributeMeta, val float64, row *model.CanonicalComparisonRow) Evaluation {
	if strings.EqualFold(spec.ID, "handlebar_clamp_fit") && row != nil && row.Canonical.CockpitFitResult != nil {
		cfr := row.Canonical.CockpitFitResult
		subScore := cfr.FinalSubScore
		zone := ZoneOutside
		sym := SymbolOutside
		if cfr.HasFit {
			if subScore >= 95.0 {
				zone = ZoneIdeal
				sym = SymbolIdeal
			} else {
				zone = ZoneAcceptable
				sym = SymbolAcceptable
			}
		}

		formatted := fmt.Sprintf("%d %s", cfr.CountInside, sym)

		return Evaluation{
			Value:           subScore,
			Zone:            zone,
			DistanceToIdeal: cfr.BestR,
			SubScore:        subScore,
			FormattedCell:   formatted,
		}
	}

	baseValStr := fmt.Sprintf("%.*f", meta.Decimals, val)
	if meta.FormatVal != nil {
		baseValStr = meta.FormatVal(val, row)
	}

	if spec.Ideal == nil {
		return Evaluation{
			Value:         val,
			FormattedCell: baseValStr,
			IsDisplayOnly: true,
		}
	}

	curves, err := spec.compileCurves()
	if err != nil {
		panic(fmt.Sprintf("ideal: cannot evaluate invalid attribute spec: %v", err))
	}

	// Signed distance to the ideal range: negative below, positive above, 0 inside.
	var distToIdeal float64
	var side *sideCurve
	switch {
	case spec.Ideal.Min != nil && val < *spec.Ideal.Min:
		distToIdeal = val - *spec.Ideal.Min
		side = curves.below
	case spec.Ideal.Max != nil && val > *spec.Ideal.Max:
		distToIdeal = val - *spec.Ideal.Max
		side = curves.above
	default:
		return Evaluation{
			Value:           val,
			Zone:            ZoneIdeal,
			DistanceToIdeal: 0.0,
			SubScore:        curve.Peak,
			FormattedCell:   fmt.Sprintf("%s %s", baseValStr, SymbolIdeal),
		}
	}

	zone, symbol := ZoneOutside, SymbolOutside
	if math.Abs(distToIdeal) <= side.acceptableDist {
		zone, symbol = ZoneAcceptable, SymbolAcceptable
	}

	return Evaluation{
		Value:           val,
		Zone:            zone,
		DistanceToIdeal: distToIdeal,
		SubScore:        side.falloff.Score(math.Abs(distToIdeal)),
		FormattedCell:   formatCellWithDelta(baseValStr, symbol, distToIdeal, meta),
	}
}

func formatCellWithDelta(baseValStr, symbol string, delta float64, meta AttributeMeta) string {
	factor := math.Pow(10, float64(meta.Decimals))
	absRounded := math.Round(math.Abs(delta)*factor) / factor
	if absRounded == 0 {
		if symbol == SymbolAcceptable {
			symbol = SymbolIdeal
		}
		return fmt.Sprintf("%s %s", baseValStr, symbol)
	}

	unitSuffix := ""
	if meta.Unit == "°" || meta.Unit == "%" {
		unitSuffix = meta.Unit
	}

	word := meta.AboveWord
	if delta < 0 {
		word = meta.BelowWord
	}

	if word != "" {
		return fmt.Sprintf("%s %s %.*f%s %s", baseValStr, symbol, meta.Decimals, absRounded, unitSuffix, word)
	}

	return fmt.Sprintf("%s %s %s", baseValStr, symbol, formatDeltaVal(delta, meta.Decimals))
}

func formatDeltaVal(delta float64, decimals int) string {
	if decimals == 0 {
		rnd := math.Round(delta)
		if rnd >= 0 {
			return fmt.Sprintf("+%.0f", rnd)
		}
		return fmt.Sprintf("−%.0f", math.Abs(rnd))
	}
	if delta >= 0 {
		return fmt.Sprintf("+%.*f", decimals, delta)
	}
	return fmt.Sprintf("−%.*f", decimals, math.Abs(delta))
}

// FormatRange produces a human-readable range string (e.g. "630–642", "68.0°", "≥470", "≤420").
func FormatRange(r *Range, unit string, decimals int) string {
	if r == nil {
		return ""
	}
	unitSuffix := ""
	if unit == "°" || unit == "%" {
		unitSuffix = unit
	}

	if r.Min != nil && r.Max != nil {
		if *r.Min == *r.Max {
			return fmt.Sprintf("%.*f%s", decimals, *r.Min, unitSuffix)
		}
		return fmt.Sprintf("%.*f–%.*f%s", decimals, *r.Min, decimals, *r.Max, unitSuffix)
	}
	if r.Min != nil {
		return fmt.Sprintf("≥%.*f%s", decimals, *r.Min, unitSuffix)
	}
	if r.Max != nil {
		return fmt.Sprintf("≤%.*f%s", decimals, *r.Max, unitSuffix)
	}
	return ""
}
