package ideal

import (
	"fmt"
	"math"
	"strings"

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

	floor := -200.0
	if spec.Floor != nil {
		floor = *spec.Floor
	}

	// 1. Inside Ideal Range
	belowIdeal := spec.Ideal.Min != nil && val < *spec.Ideal.Min
	aboveIdeal := spec.Ideal.Max != nil && val > *spec.Ideal.Max

	if !belowIdeal && !aboveIdeal {
		return Evaluation{
			Value:           val,
			Zone:            ZoneIdeal,
			DistanceToIdeal: 0.0,
			SubScore:        100.0,
			FormattedCell:   fmt.Sprintf("%s %s", baseValStr, SymbolIdeal),
		}
	}

	// 2. Below Ideal Range
	if belowIdeal {
		minI := *spec.Ideal.Min
		distToIdeal := val - minI // negative

		inAcceptable := false
		if spec.Acceptable != nil && spec.Acceptable.Min != nil {
			if val >= *spec.Acceptable.Min {
				inAcceptable = true
			}
		}

		if inAcceptable {
			minA := *spec.Acceptable.Min
			edgeScore := 100.0
			if spec.EdgeScore != nil {
				edgeScore = *spec.EdgeScore
			}

			sub := 100.0
			if minI > minA {
				ratio := (val - minA) / (minI - minA)
				sub = edgeScore + (100.0-edgeScore)*ratio
			}

			return Evaluation{
				Value:           val,
				Zone:            ZoneAcceptable,
				DistanceToIdeal: distToIdeal,
				SubScore:        sub,
				FormattedCell:   formatCellWithDelta(baseValStr, SymbolAcceptable, distToIdeal, meta),
			}
		}

		// Outside Acceptable (Low side)
		minA := minI
		startScore := 100.0
		if spec.Acceptable != nil && spec.Acceptable.Min != nil {
			minA = *spec.Acceptable.Min
			if *spec.Acceptable.Min < minI && spec.EdgeScore != nil {
				startScore = *spec.EdgeScore
			}
		}

		distBelow := minA - val
		falloff := 1.0
		if spec.Falloff != nil && spec.Falloff.Below != nil && *spec.Falloff.Below > 0 {
			falloff = *spec.Falloff.Below
		}

		slope := startScore / falloff
		sub := startScore - distBelow*slope
		if sub < floor {
			sub = floor
		}

		return Evaluation{
			Value:           val,
			Zone:            ZoneOutside,
			DistanceToIdeal: distToIdeal,
			SubScore:        sub,
			FormattedCell:   formatCellWithDelta(baseValStr, SymbolOutside, distToIdeal, meta),
		}
	}

	// 3. Above Ideal Range
	maxI := *spec.Ideal.Max
	distToIdeal := val - maxI // positive

	inAcceptable := false
	if spec.Acceptable != nil && spec.Acceptable.Max != nil {
		if val <= *spec.Acceptable.Max {
			inAcceptable = true
		}
	}

	if inAcceptable {
		maxA := *spec.Acceptable.Max
		edgeScore := 100.0
		if spec.EdgeScore != nil {
			edgeScore = *spec.EdgeScore
		}

		sub := 100.0
		if maxA > maxI {
			ratio := (maxA - val) / (maxA - maxI)
			sub = edgeScore + (100.0-edgeScore)*ratio
		}

		return Evaluation{
			Value:           val,
			Zone:            ZoneAcceptable,
			DistanceToIdeal: distToIdeal,
			SubScore:        sub,
			FormattedCell:   formatCellWithDelta(baseValStr, SymbolAcceptable, distToIdeal, meta),
		}
	}

	// Outside Acceptable (High side)
	maxA := maxI
	startScore := 100.0
	if spec.Acceptable != nil && spec.Acceptable.Max != nil {
		maxA = *spec.Acceptable.Max
		if *spec.Acceptable.Max > maxI && spec.EdgeScore != nil {
			startScore = *spec.EdgeScore
		}
	}

	distAbove := val - maxA
	falloff := 1.0
	if spec.Falloff != nil && spec.Falloff.Above != nil && *spec.Falloff.Above > 0 {
		falloff = *spec.Falloff.Above
	}

	slope := startScore / falloff
	sub := startScore - distAbove*slope
	if sub < floor {
		sub = floor
	}

	return Evaluation{
		Value:           val,
		Zone:            ZoneOutside,
		DistanceToIdeal: distToIdeal,
		SubScore:        sub,
		FormattedCell:   formatCellWithDelta(baseValStr, SymbolOutside, distToIdeal, meta),
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
