package ideal

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"bikegeo/pkg/curve"
	"bikegeo/pkg/model"
)

// RoundPrinted rounds v to decimals decimal places using the same rule as
// fmt.Sprintf("%.*f", decimals, v) (round-half-to-even), normalizing -0 to 0.
func RoundPrinted(v float64, decimals int) float64 {
	s := fmt.Sprintf("%.*f", decimals, v)
	r, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return v
	}
	if r == 0 {
		return 0
	}
	return r
}

// PrintedDistanceToIdeal returns the signed distance from printedVal to the
// printed bounds of idealRange at the given decimal precision (0 if inside).
func PrintedDistanceToIdeal(idealRange *Range, printedVal float64, decimals int) float64 {
	if idealRange == nil {
		return 0
	}
	if idealRange.Min != nil {
		minP := RoundPrinted(*idealRange.Min, decimals)
		if printedVal < minP {
			return RoundPrinted(printedVal-minP, decimals)
		}
	}
	if idealRange.Max != nil {
		maxP := RoundPrinted(*idealRange.Max, decimals)
		if printedVal > maxP {
			return RoundPrinted(printedVal-maxP, decimals)
		}
	}
	return 0
}

// Evaluate evaluates a metric value against an AttributeSpec and returns an Evaluation.
// row can be nil if evaluating abstract test values.
func Evaluate(spec AttributeSpec, meta AttributeMeta, val float64, row *model.CanonicalComparisonRow) Evaluation {
	if strings.EqualFold(spec.ID, "handlebar_clamp_fit") && row != nil && row.Canonical.CockpitFitResult != nil {
		cfr := row.Canonical.CockpitFitResult
		subScore := cfr.FinalSubScore
		printedSub := math.Round(subScore)
		idealMin := 100.0
		if spec.Ideal != nil && spec.Ideal.Min != nil {
			idealMin = math.Round(*spec.Ideal.Min)
		}
		accMin := 75.0
		if spec.Acceptable != nil && spec.Acceptable.Min != nil {
			accMin = math.Round(*spec.Acceptable.Min)
		}
		zone := ZoneOutside
		sym := SymbolOutside
		if printedSub >= idealMin {
			zone = ZoneIdeal
			sym = SymbolIdeal
		} else if printedSub >= accMin {
			zone = ZoneAcceptable
			sym = SymbolAcceptable
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

	printedVal := RoundPrinted(val, meta.Decimals)
	baseValStr := fmt.Sprintf("%.*f", meta.Decimals, printedVal)
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

	// Score keeps using the unrounded value so curves remain strictly decreasing.
	subScore := curve.Peak
	switch {
	case spec.Ideal.Min != nil && val < *spec.Ideal.Min:
		subScore = curves.below.falloff.Score(*spec.Ideal.Min - val)
	case spec.Ideal.Max != nil && val > *spec.Ideal.Max:
		subScore = curves.above.falloff.Score(val - *spec.Ideal.Max)
	}

	// Zone, marker, and delta are decided from the value and bounds as printed.
	printedDist := PrintedDistanceToIdeal(spec.Ideal, printedVal, meta.Decimals)
	if printedDist == 0 {
		return Evaluation{
			Value:           val,
			Zone:            ZoneIdeal,
			DistanceToIdeal: 0.0,
			SubScore:        subScore,
			FormattedCell:   fmt.Sprintf("%s %s", baseValStr, SymbolIdeal),
		}
	}

	zone, symbol := ZoneOutside, SymbolOutside
	if spec.Acceptable != nil {
		if printedDist < 0 {
			if spec.Acceptable.Min == nil || printedVal >= RoundPrinted(*spec.Acceptable.Min, meta.Decimals) {
				zone, symbol = ZoneAcceptable, SymbolAcceptable
			}
		} else {
			if spec.Acceptable.Max == nil || printedVal <= RoundPrinted(*spec.Acceptable.Max, meta.Decimals) {
				zone, symbol = ZoneAcceptable, SymbolAcceptable
			}
		}
	}

	return Evaluation{
		Value:           val,
		Zone:            zone,
		DistanceToIdeal: printedDist,
		SubScore:        subScore,
		FormattedCell:   formatCellWithDelta(baseValStr, symbol, printedDist, meta),
	}
}

func formatCellWithDelta(baseValStr, symbol string, delta float64, meta AttributeMeta) string {
	absDelta := RoundPrinted(math.Abs(delta), meta.Decimals)
	if absDelta == 0 {
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
		return fmt.Sprintf("%s %s %.*f%s %s", baseValStr, symbol, meta.Decimals, absDelta, unitSuffix, word)
	}

	return fmt.Sprintf("%s %s %s", baseValStr, symbol, formatDeltaVal(delta, meta.Decimals))
}

func formatDeltaVal(delta float64, decimals int) string {
	rounded := RoundPrinted(delta, decimals)
	if rounded >= 0 {
		return fmt.Sprintf("+%.*f", decimals, rounded)
	}
	return fmt.Sprintf("−%.*f", decimals, math.Abs(rounded))
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
