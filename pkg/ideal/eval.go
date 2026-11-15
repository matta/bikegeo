package ideal

import (
	"fmt"
	"math"
	"strconv"

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
	if minV, ok := idealRange.Min.Get(); ok {
		minP := RoundPrinted(minV, decimals)
		if printedVal < minP {
			return RoundPrinted(printedVal-minP, decimals)
		}
	}
	if maxV, ok := idealRange.Max.Get(); ok {
		maxP := RoundPrinted(maxV, decimals)
		if printedVal > maxP {
			return RoundPrinted(printedVal-maxP, decimals)
		}
	}
	return 0
}

// OvalCostPerR is the Oval Fits price in points of total per unit of normalized
// radius R beyond the oval edge (R = 1). Inside the oval is free; there is no tail.
const OvalCostPerR = 22.0

// OvalPointsLost returns the Oval Fits points lost for a selected cockpit setup:
// OvalCostPerR per unit of R beyond the oval edge. The selector's stem penalty
// only steers which setup is selected; it costs no points.
func OvalPointsLost(cfr *model.CockpitFitResult, cost *Cost) float64 {
	if cfr == nil {
		return 0
	}
	perR := OvalCostPerR
	if cost != nil {
		perR = cost.Above
	}
	return perR * math.Max(0, cfr.BestR-1.0)
}

// hardwareLimitAttributes lists the attributes whose hardware limits HardwareLimits knows.
var hardwareLimitAttributes = map[MetricID]struct{}{
	MetricRailOffset: {},
	MetricStemLength: {},
	MetricStemAngle:  {},
	MetricSpacers:    {},
}

// HardwareLimits returns the [lo, hi] range the setup's stock hardware allows for a
// hardware band: the saddle rail window (from the mapped BRP candidates), the stem
// catalog's length and angle range, and 0 up to the frame's steerer spacer stack.
func HardwareLimits(id MetricID, row *model.CanonicalComparisonRow) (lo, hi float64, ok bool) {
	if row == nil {
		return 0, 0, false
	}
	switch id {
	case MetricRailOffset:
		brps := row.Canonical.PossibleBRPs
		if len(brps) == 0 {
			return 0, 0, false
		}
		lo, hi = brps[0].RailOffsetMm, brps[0].RailOffsetMm
		for _, b := range brps[1:] {
			lo, hi = math.Min(lo, b.RailOffsetMm), math.Max(hi, b.RailOffsetMm)
		}
		return lo, hi, true
	case MetricStemLength:
		return row.Bike.GetExpectedStemRangeMm()
	case MetricStemAngle:
		stems := row.Bike.Stems
		if len(stems) == 0 {
			return 0, 0, false
		}
		lo, hi = float64(stems[0].AngleDeg), float64(stems[0].AngleDeg)
		for _, s := range stems[1:] {
			lo, hi = math.Min(lo, float64(s.AngleDeg)), math.Max(hi, float64(s.AngleDeg))
		}
		return lo, hi, true
	case MetricSpacers:
		return 0, math.Max(0, row.Bike.GetMaxSpacerMm()), true
	case MetricSetback, MetricEtt, MetricAett, MetricSpareVert, MetricSpareAxial,
		MetricSeatpostExtension, MetricSeatToRc, MetricChainstay, MetricBbDrop,
		MetricHta, MetricTrail, MetricSta, MetricReach, MetricStack, MetricWheelbase,
		MetricRcWbRatio, MetricSeatTube, MetricStandover, MetricBrpSetback,
		MetricSaddleHb, MetricSaddleHbDrop, MetricBestStem, MetricStemSpacerFit,
		MetricSitBoneSetback, MetricIdealSta, MetricHandlebarClampFit:
		return 0, 0, false
	default:
		return 0, 0, false
	}
}

// LimitPointsLost prices val under a hardware band: free inside spec.Ideal, then
// linear from 0 at the free bound to spec.LimitPoints at the hardware limit lo/hi
// on that side (continuing at the same slope past it). A side whose limit lies
// inside the free range is never priced.
func LimitPointsLost(spec AttributeSpec, val, lo, hi float64) float64 {
	if spec.Ideal == nil || spec.LimitPoints <= 0 {
		return 0
	}
	if minV, ok := spec.Ideal.Min.Get(); ok && val < minV {
		if span := minV - lo; span > 0 {
			return spec.LimitPoints * (minV - val) / span
		}
		return 0
	}
	if maxV, ok := spec.Ideal.Max.Get(); ok && val > maxV {
		if span := hi - maxV; span > 0 {
			return spec.LimitPoints * (val - maxV) / span
		}
		return 0
	}
	return 0
}

// RowPointsLost returns the points of total lost by val under spec for a row,
// resolving hardware limits for LimitPoints bands (0 when the row is unknown).
func RowPointsLost(spec AttributeSpec, val float64, row *model.CanonicalComparisonRow) float64 {
	if spec.LimitPoints > 0 {
		lo, hi, ok := HardwareLimits(spec.ID, row)
		if !ok {
			return 0
		}
		return LimitPointsLost(spec, val, lo, hi)
	}
	return PointsLost(spec, val)
}

// PointsLost returns the points of total lost by an unrounded value under spec:
// zero inside the Ideal (free) range, and Cost per unit outside it.
func PointsLost(spec AttributeSpec, val float64) float64 {
	if !spec.IsScored() || spec.Cost == nil {
		return 0
	}
	if minV, ok := spec.Ideal.Min.Get(); ok && val < minV {
		return spec.Cost.Below * (minV - val)
	}
	if maxV, ok := spec.Ideal.Max.Get(); ok && val > maxV {
		return spec.Cost.Above * (val - maxV)
	}
	return 0
}

// Evaluate evaluates a metric value against an AttributeSpec and returns an Evaluation.
// row can be nil if evaluating abstract test values.
func Evaluate(spec AttributeSpec, meta AttributeMeta, val float64, row *model.CanonicalComparisonRow) Evaluation {
	if spec.ID == MetricHandlebarClampFit && row != nil && row.Canonical.CockpitFitResult != nil {
		cfr := row.Canonical.CockpitFitResult
		printedSub := math.Round(cfr.FinalSubScore)
		idealMin := 100.0
		if spec.Ideal != nil {
			if v, ok := spec.Ideal.Min.Get(); ok {
				idealMin = math.Round(v)
			}
		}
		accMin := 75.0
		if spec.Acceptable != nil {
			if v, ok := spec.Acceptable.Min.Get(); ok {
				accMin = math.Round(v)
			}
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

		lost := 0.0
		if spec.IsScored() {
			lost = OvalPointsLost(cfr, spec.Cost)
		}

		return Evaluation{
			Value:           cfr.FinalSubScore,
			Zone:            zone,
			DistanceToIdeal: cfr.BestR,
			PointsLost:      lost,
			FormattedCell:   fmt.Sprintf("%d %s", cfr.CountInside, sym),
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

	// Points lost use the unrounded value so the cost stays linear.
	lost := RowPointsLost(spec, val, row)

	// Zone, marker, and delta are decided from the value and bounds as printed.
	printedDist := PrintedDistanceToIdeal(spec.Ideal, printedVal, meta.Decimals)
	if printedDist == 0 {
		return Evaluation{
			Value:           val,
			Zone:            ZoneIdeal,
			DistanceToIdeal: 0.0,
			PointsLost:      lost,
			FormattedCell:   fmt.Sprintf("%s %s", baseValStr, SymbolIdeal),
		}
	}

	zone, symbol := ZoneOutside, SymbolOutside
	if spec.Acceptable != nil {
		if printedDist < 0 {
			if accMin, ok := spec.Acceptable.Min.Get(); !ok || printedVal >= RoundPrinted(accMin, meta.Decimals) {
				zone, symbol = ZoneAcceptable, SymbolAcceptable
			}
		} else {
			if accMax, ok := spec.Acceptable.Max.Get(); !ok || printedVal <= RoundPrinted(accMax, meta.Decimals) {
				zone, symbol = ZoneAcceptable, SymbolAcceptable
			}
		}
	}

	return Evaluation{
		Value:           val,
		Zone:            zone,
		DistanceToIdeal: printedDist,
		PointsLost:      lost,
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

	minV, hasMin := r.Min.Get()
	maxV, hasMax := r.Max.Get()
	if hasMin && hasMax {
		if minV == maxV {
			return fmt.Sprintf("%.*f%s", decimals, minV, unitSuffix)
		}
		return fmt.Sprintf("%.*f–%.*f%s", decimals, minV, decimals, maxV, unitSuffix)
	}
	if hasMin {
		return fmt.Sprintf("≥%.*f%s", decimals, minV, unitSuffix)
	}
	if hasMax {
		return fmt.Sprintf("≤%.*f%s", decimals, maxV, unitSuffix)
	}
	return ""
}
