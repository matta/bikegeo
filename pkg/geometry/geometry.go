package geometry

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// ToRad converts degrees to radians.
func ToRad(deg float64) float64 {
	return (deg * math.Pi) / 180.0
}

// ToDeg converts radians to degrees.
func ToDeg(rad float64) float64 {
	return (rad * 180.0) / math.Pi
}

// Round rounds a float to the specified decimal precision.
func Round(val float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(val*factor) / factor
}

// CalculateBentSeatTubeOffset derives the horizontal bottom bracket forward offset (Ox) in mm
// for a bent or offset seat tube using the Stack plane (Method A).
func CalculateBentSeatTubeOffset(stack, actualStaDeg, nominalStaDeg float64) float64 {
	if actualStaDeg <= 0 || nominalStaDeg <= 0 || stack <= 0 {
		return 0
	}
	cotAct := 1.0 / math.Tan(ToRad(actualStaDeg))
	cotNom := 1.0 / math.Tan(ToRad(nominalStaDeg))
	return stack * (cotAct - cotNom)
}

// CalculateEffectiveSta computes the true effective seat tube angle in degrees for a rider at
// radial saddle height L (distance from BB spindle center to saddle along the effective angle).
// Law of Sines: alpha = asin((Ox * sin(theta_act)) / L), theta_eff = theta_act + alpha.
func CalculateEffectiveSta(actualStaDeg, offsetXMm, saddleDistanceL float64) float64 {
	if saddleDistanceL <= 0 || actualStaDeg <= 0 {
		return actualStaDeg
	}
	actRad := ToRad(actualStaDeg)
	sinVal := (offsetXMm * math.Sin(actRad)) / saddleDistanceL
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return actualStaDeg + ToDeg(alphaRad)
}

// SeatTubeAxis returns a directional Axis2 along the seat tube starting at origin (typically BB spindle at 0,0).
// In bicycle frame geometry, STA is measured from the rear horizontal ground line, sweeping clockwise.
func SeatTubeAxis(origin geom2d.Point2, staDeg float64) geom2d.Axis2 {
	return geom2d.NewAxisFromPolar(origin, geom2d.FromDeg(180.0-staDeg))
}

// SeatTubeDir returns a unit vector pointing upward along the seat tube axis.
func SeatTubeDir(staDeg float64) geom2d.Vec2 {
	return geom2d.FromPolar(1.0, geom2d.FromDeg(180.0-staDeg))
}

// SteererTubeAxis returns a directional Axis2 along the steerer tube starting at origin (typically head tube top).
// In bicycle frame geometry, HTA is measured as an acute angle from the horizontal ground line, sweeping clockwise.
func SteererTubeAxis(origin geom2d.Point2, htaDeg float64) geom2d.Axis2 {
	return geom2d.NewAxisFromPolar(origin, geom2d.FromDeg(180.0-htaDeg))
}

// SteererTubeDir returns a unit vector pointing upward along the steerer tube axis.
func SteererTubeDir(htaDeg float64) geom2d.Vec2 {
	return geom2d.FromPolar(1.0, geom2d.FromDeg(180.0-htaDeg))
}

// CalculateAett calculates the Actual Effective Top Tube (AETT) length in mm.
// AETT is the horizontal projection from the center of the seat tube
// (at the specified rider saddle height) forward to the steering axis.
func CalculateAett(reach, stack, htaDeg, staDeg, seatHeight float64) float64 {
	seatAxis := SeatTubeAxis(geom2d.Origin, staDeg)
	saddlePoint := seatAxis.PointAt(seatHeight)

	steererAxis := SteererTubeAxis(geom2d.Point2{X: reach, Y: stack}, htaDeg)
	intersect, ok := steererAxis.IntersectHorizontalLine(saddlePoint.Y)
	if !ok {
		return 0
	}

	return intersect.X - saddlePoint.X
}

// CalculateActualSetback calculates the horizontal setback distance in mm from the bottom bracket center (0,0)
// to the center of the seat tube / seatpost at the specified saddle height.
func CalculateActualSetback(staDeg, seatHeight float64) float64 {
	seatAxis := SeatTubeAxis(geom2d.Origin, staDeg)
	saddlePoint := seatAxis.PointAt(seatHeight)
	return Round(-saddlePoint.X, 1)
}

// CalculateBRPPoint computes the saddle Biomechanical Reference Point (BRP) coordinates,
// horizontal setback, and angle from BB at radial distance seatHeight from BB (0,0)
// with perpendicular offset behind the seat tube axis.
func CalculateBRPPoint(
	staDeg, seatHeight, seatpostSetbackMm, saddleBrpOffsetMm, railOffsetMm float64,
) model.BRPPoint {
	seatAxis := SeatTubeAxis(geom2d.Origin, staDeg)
	clampPerp := seatpostSetbackMm + railOffsetMm
	// Forward offset toward nose reduces rearward perpendicular offset from seat tube axis:
	brpPerp := clampPerp - saddleBrpOffsetMm

	axial := math.Sqrt(math.Max(0, seatHeight*seatHeight-brpPerp*brpPerp))
	brpPoint := seatAxis.PointWithOffset(axial, brpPerp, geom2d.CounterClockwise)

	setback := -brpPoint.X
	angleFromBb := 0.0
	if setback > 0 && brpPoint.Y > 0 {
		angleFromBb = geom2d.Point2{X: setback, Y: brpPoint.Y}.PolarAngleDeg()
	}

	return model.BRPPoint{
		SeatpostSetbackMm:     seatpostSetbackMm,
		SaddleBrpOffsetMm:     saddleBrpOffsetMm,
		RailOffsetMm:          railOffsetMm,
		AchievedClampOffsetMm: railOffsetMm,
		TotalPerpOffsetMm:     Round(brpPerp, 1),
		BrpX:                  Round(brpPoint.X, 1),
		BrpY:                  Round(brpPoint.Y, 1),
		BrpSetbackMm:          Round(setback, 1),
		BrpAngleFromBbDeg:     Round(angleFromBb, 1),
	}
}

// MapPossibleBRPs maps candidate BRP points across the usable rail adjustment range
// for the default reference saddle (SQlab 610) with the default seatpost setback.
func MapPossibleBRPs(staDeg, seatHeight float64) []model.BRPPoint {
	return model.ReferenceIdmatchSaddle().MapPossibleBRPs(
		staDeg,
		seatHeight,
		model.DefaultSeatpostSetbackMm,
		model.DefaultSeatpostClampLengthMm,
	)
}

// SelectBestBRP selects the best BRP point from candidate points for a target setback range.
// Ranking hierarchy:
// 1. Min rounded mm distance to target setback range (0 if inside or if target is nil)
// 2. Smallest absolute rail offset (|RailOffsetMm|, closest to dead center 0mm on rails)
// 3. Min exact unrounded distance to target midpoint
func SelectBestBRP(
	possible []model.BRPPoint,
	targetMin, targetMax *float64,
) (model.BRPPoint, bool) {
	if len(possible) == 0 {
		return model.BRPPoint{}, false
	}

	best := possible[0]
	hasTarget := targetMin != nil || targetMax != nil

	calcTargetMid := func() float64 {
		if targetMin != nil && targetMax != nil {
			return (*targetMin + *targetMax) / 2.0
		}
		if targetMin != nil {
			return *targetMin
		}
		if targetMax != nil {
			return *targetMax
		}
		return 0
	}
	targetMid := calcTargetMid()

	rangeDist := func(val float64) float64 {
		if !hasTarget {
			return 0
		}
		if targetMin != nil && val < *targetMin {
			return *targetMin - val
		}
		if targetMax != nil && val > *targetMax {
			return val - *targetMax
		}
		return 0
	}

	better := func(a, b model.BRPPoint) bool {
		if hasTarget {
			distA := rangeDist(a.BrpSetbackMm)
			distB := rangeDist(b.BrpSetbackMm)
			roundDistA := math.Round(distA)
			roundDistB := math.Round(distB)
			if roundDistA != roundDistB {
				return roundDistA < roundDistB
			}
		}

		// Tie-breaker 1: closest to dead center on rails (min |RailOffsetMm|)
		railDiffA := math.Abs(a.RailOffsetMm)
		railDiffB := math.Abs(b.RailOffsetMm)
		if math.Abs(railDiffA-railDiffB) > 0.001 {
			return railDiffA < railDiffB
		}

		// Tie-breaker 2: min unrounded distance to target midpoint
		if hasTarget {
			midDiffA := math.Abs(a.BrpSetbackMm - targetMid)
			midDiffB := math.Abs(b.BrpSetbackMm - targetMid)
			if math.Abs(midDiffA-midDiffB) > 0.001 {
				return midDiffA < midDiffB
			}
		}

		return false
	}

	for _, pt := range possible[1:] {
		if better(pt, best) {
			best = pt
		}
	}

	return best, true
}

// CalculateHBPoint calculates the HB reference point (center of the stem handlebar clamp)
// and the Saddle-Handlebar distance and drop from the saddle BRP reference point.
// Steerer tube extends upward and rearward at polar angle (180° - HTA).
// Stem clamp center sits at d_steerer = spacerMm + StemThicknessMm/2.0 along the steerer axis:
//
//	SteererX = reach - d_steerer * cos(HTA)
//	SteererY = stack + d_steerer * sin(HTA)
//
// Stem angle is rotated 90° clockwise from the upward steerer axis, then rotated AngleDeg:
//
//	theta_stem = (180° - HTA) - 90° + AngleDeg = (90° - HTA) + AngleDeg
//	HbX = SteererX + stem.LengthMm * cos(theta_stem)
//	HbY = SteererY + stem.LengthMm * sin(theta_stem)
//
// Distance & Drop from BRP:
//
//	SaddleHbMm = hypot(HbX - brp.BrpX, HbY - brp.BrpY)
//	SaddleHbDropMm = HbY - brp.BrpY
func CalculateHBPoint(
	reach, stack, htaDeg float64,
	brp model.BRPPoint,
	stem model.StemSpec,
	spacerMm float64,
) model.HBPoint {
	headTubeTop := geom2d.Point2{X: reach, Y: stack}
	steererAxis := SteererTubeAxis(headTubeTop, htaDeg)

	dSteerer := spacerMm + model.StemThicknessMm/2.0
	stemClamp := steererAxis.PointAt(dSteerer)

	// Stem angle is rotated 90° clockwise from upward steerer axis, then rotated AngleDeg:
	stemDir := steererAxis.Normal(geom2d.Clockwise).RotateDeg(stem.AngleDeg)
	hbPoint := stemClamp.Add(stemDir.Scale(stem.LengthMm))

	brpPoint := geom2d.Point2{X: brp.BrpX, Y: brp.BrpY}
	dist := hbPoint.DistanceTo(brpPoint)
	drop := hbPoint.Y - brpPoint.Y

	return model.HBPoint{
		Stem:           stem,
		SpacerMm:       spacerMm,
		HbX:            Round(hbPoint.X, 1),
		HbY:            Round(hbPoint.Y, 1),
		SaddleHbMm:     Round(dist, 1),
		SaddleHbDropMm: Round(drop, 1),
	}
}

// MapPossibleHBs maps all possible HB points for a bike across its allowed stems and spacer stack heights,
// referencing distance and drop from the given BRP reference point.
func MapPossibleHBs(
	bike model.BikeGeometry,
	activeReach, activeStack, activeHta float64,
	brp model.BRPPoint,
) []model.HBPoint {
	stems := bike.Stems
	if len(stems) == 0 && bike.CanonicalStem.LengthMm > 0 {
		stems = []model.StemSpec{bike.CanonicalStem}
	}
	spacers := bike.GetSpacersMm()

	var points []model.HBPoint
	for _, st := range stems {
		for _, sp := range spacers {
			pt := CalculateHBPoint(activeReach, activeStack, activeHta, brp, st, sp)
			points = append(points, pt)
		}
	}
	return points
}

// SelectBestHBWithDrop selects the best HB point from candidate points for desired target distance and drop ranges.
// Ranking hierarchy:
// 1. Primary: Min rounded mm distance to distance target range (saddle_hb, 0 if inside or nil)
// 2. Secondary: Min rounded mm distance to drop target range (saddle_hb_drop, 0 if inside or nil)
// 3. Tie-breaker 1: Closest stem length to canonicalStem
// 4. Tie-breaker 2: Closest stem angle to canonicalStem
// 5. Tie-breaker 3: Smallest spacer stack (spacerMm)
// 6. Tie-breaker 4: Min exact unrounded distance to target midpoints (distance first, then drop)
func SelectBestHBWithDrop(
	possible []model.HBPoint,
	canonicalStem model.StemSpec,
	distMin, distMax *float64,
	dropMin, dropMax *float64,
) (model.HBPoint, bool) {
	if len(possible) == 0 {
		return model.HBPoint{}, false
	}

	best := possible[0]
	hasDistTarget := distMin != nil || distMax != nil
	hasDropTarget := dropMin != nil || dropMax != nil

	calcMid := func(minVal, maxVal *float64) float64 {
		if minVal != nil && maxVal != nil {
			return (*minVal + *maxVal) / 2.0
		}
		if minVal != nil {
			return *minVal
		}
		if maxVal != nil {
			return *maxVal
		}
		return 0
	}
	distMid := calcMid(distMin, distMax)
	dropMid := calcMid(dropMin, dropMax)

	rangeDist := func(val float64, minVal, maxVal *float64) float64 {
		if minVal != nil && val < *minVal {
			return *minVal - val
		}
		if maxVal != nil && val > *maxVal {
			return val - *maxVal
		}
		return 0
	}

	better := func(a, b model.HBPoint) bool {
		// 1. Primary: Saddle-HB distance range
		if hasDistTarget {
			distA := rangeDist(a.SaddleHbMm, distMin, distMax)
			distB := rangeDist(b.SaddleHbMm, distMin, distMax)
			roundDistA := math.Round(distA)
			roundDistB := math.Round(distB)
			if roundDistA != roundDistB {
				return roundDistA < roundDistB
			}
		}

		// 2. Secondary: Saddle-HB drop range
		if hasDropTarget {
			dropA := rangeDist(a.SaddleHbDropMm, dropMin, dropMax)
			dropB := rangeDist(b.SaddleHbDropMm, dropMin, dropMax)
			roundDropA := math.Round(dropA)
			roundDropB := math.Round(dropB)
			if roundDropA != roundDropB {
				return roundDropA < roundDropB
			}
		}

		// Tie-breaker 1: closest stem length to canonical
		lenDiffA := math.Abs(a.Stem.LengthMm - canonicalStem.LengthMm)
		lenDiffB := math.Abs(b.Stem.LengthMm - canonicalStem.LengthMm)
		if math.Abs(lenDiffA-lenDiffB) > 0.001 {
			return lenDiffA < lenDiffB
		}

		// Tie-breaker 2: closest stem angle to canonical
		angDiffA := math.Abs(a.Stem.AngleDeg - canonicalStem.AngleDeg)
		angDiffB := math.Abs(b.Stem.AngleDeg - canonicalStem.AngleDeg)
		if math.Abs(angDiffA-angDiffB) > 0.001 {
			return angDiffA < angDiffB
		}

		// Tie-breaker 3: smallest spacer stack
		if math.Abs(a.SpacerMm-b.SpacerMm) > 0.001 {
			return a.SpacerMm < b.SpacerMm
		}

		// Tie-breaker 4: closest unrounded distance to target midpoints
		if hasDistTarget {
			midDiffA := math.Abs(a.SaddleHbMm - distMid)
			midDiffB := math.Abs(b.SaddleHbMm - distMid)
			if math.Abs(midDiffA-midDiffB) > 0.001 {
				return midDiffA < midDiffB
			}
		}
		if hasDropTarget {
			midDiffA := math.Abs(a.SaddleHbDropMm - dropMid)
			midDiffB := math.Abs(b.SaddleHbDropMm - dropMid)
			if math.Abs(midDiffA-midDiffB) > 0.001 {
				return midDiffA < midDiffB
			}
		}

		return false
	}

	for _, pt := range possible[1:] {
		if better(pt, best) {
			best = pt
		}
	}

	return best, true
}

// SelectBestHB selects the best HB point from candidate points for a desired target distance range.
func SelectBestHB(
	possible []model.HBPoint,
	canonicalStem model.StemSpec,
	targetMin, targetMax *float64,
) (model.HBPoint, bool) {
	return SelectBestHBWithDrop(possible, canonicalStem, targetMin, targetMax, nil, nil)
}

// DefaultFitOval creates a FitOval with standard defaults (a = 10mm horizontal, b = 20mm vertical, 30° clockwise).
func DefaultFitOval(centerX, centerY float64) model.FitOval {
	return model.FitOval{
		CenterX:    centerX,
		CenterY:    centerY,
		SemiMinorA: 10.0,
		SemiMajorB: 20.0,
		AngleDeg:   30.0,
	}
}

// EvaluateFitOval evaluates a single (x, y) point against a FitOval.
// Returns:
// - r: normalized elliptical radius (r <= 1.0 is inside)
// - isInside: true if point is on or within the oval boundary
func EvaluateFitOval(oval model.FitOval, x, y float64) (float64, bool) {
	e := geom2d.Ellipse2{
		Center:     geom2d.Point2{X: oval.CenterX, Y: oval.CenterY},
		SemiMinorA: oval.SemiMinorA,
		SemiMajorB: oval.SemiMajorB,
		Angle:      geom2d.FromDeg(oval.AngleDeg),
	}
	return e.Evaluate(geom2d.Point2{X: x, Y: y})
}

// CalculateCockpitBaseScore calculates the base proximity score for a normalized radius R.
// - Inside oval (R <= 1.0): Quadratic plateau from 100 (center) to 75 (edge).
// - Outside oval (R > 1.0): Steep quadratic roll-off dropping from 75 to 0 at R = 1.25, with a -200 floor.
func CalculateCockpitBaseScore(r float64) float64 {
	if r <= 1.0 {
		return 100.0 - 25.0*r*r
	}
	score := 75.0 - 1200.0*(r-1.0)*(r-1.0)
	if score < -200.0 {
		return -200.0
	}
	return score
}

// EvaluateCockpitFit evaluates candidate HB points against a FitOval.
// It selects the representative HB point, counts valid setups inside the oval,
// and computes the final sub-score including versatility bonus or zero-fit penalty.
func EvaluateCockpitFit(
	possible []model.HBPoint,
	canonicalStem model.StemSpec,
	oval model.FitOval,
) model.CockpitFitResult {
	if len(possible) == 0 {
		return model.CockpitFitResult{
			BestR:          999,
			CountInside:    0,
			BaseScore:      -200.0,
			ZeroFitPenalty: 25.0,
			FinalSubScore:  -200.0,
			HasFit:         false,
		}
	}

	type evaluatedPoint struct {
		pt       model.HBPoint
		r        float64
		isInside bool
	}

	evaluated := make([]evaluatedPoint, len(possible))
	countInside := 0

	for i, pt := range possible {
		r, inside := EvaluateFitOval(oval, pt.HbX, pt.HbY)
		evaluated[i] = evaluatedPoint{pt: pt, r: r, isInside: inside}
		if inside {
			countInside++
		}
	}

	better := func(a, b evaluatedPoint) bool {
		diffR := a.r - b.r
		if math.Abs(diffR) > 0.001 {
			return a.r < b.r
		}

		lenDiffA := math.Abs(a.pt.Stem.LengthMm - canonicalStem.LengthMm)
		lenDiffB := math.Abs(b.pt.Stem.LengthMm - canonicalStem.LengthMm)
		if math.Abs(lenDiffA-lenDiffB) > 0.001 {
			return lenDiffA < lenDiffB
		}

		angDiffA := math.Abs(a.pt.Stem.AngleDeg - canonicalStem.AngleDeg)
		angDiffB := math.Abs(b.pt.Stem.AngleDeg - canonicalStem.AngleDeg)
		if math.Abs(angDiffA-angDiffB) > 0.001 {
			return angDiffA < angDiffB
		}

		if math.Abs(a.pt.SpacerMm-b.pt.SpacerMm) > 0.001 {
			return a.pt.SpacerMm < b.pt.SpacerMm
		}

		return a.r < b.r
	}

	best := evaluated[0]
	for _, ep := range evaluated[1:] {
		if better(ep, best) {
			best = ep
		}
	}

	baseScore := CalculateCockpitBaseScore(best.r)
	var bonus, penalty, finalScore float64

	if countInside >= 1 {
		if countInside >= 4 {
			bonus = 10.0
		} else if countInside >= 2 {
			bonus = 5.0
		} else {
			bonus = 0.0
		}
		finalScore = math.Min(100.0, baseScore+bonus)
	} else {
		penalty = 25.0
		finalScore = baseScore - penalty
		if finalScore < -200.0 {
			finalScore = -200.0
		}
	}

	return model.CockpitFitResult{
		SelectedHB:       best.pt,
		BestR:            Round(best.r, 3),
		CountInside:      countInside,
		BaseScore:        Round(baseScore, 1),
		VersatilityBonus: bonus,
		ZeroFitPenalty:   penalty,
		FinalSubScore:    Round(finalScore, 1),
		HasFit:           countInside > 0,
	}
}

// ApplyCockpitFit evaluates candidate HB points against target distance and drop ranges.
// When distance targets are provided, it constructs the 30° rotated FitOval relative to active BRP,
// evaluates candidate stems, and populates CockpitFitResult, SelectedHB, SaddleHbMm, and SaddleHbDropMm.
// When no distance target is present, it falls back to SelectBestHBWithDrop.
func ApplyCockpitFit(
	can *model.BikeFitAnalysis,
	bike model.BikeGeometry,
	canonicalStem model.StemSpec,
	distMin, distMax, dropMin, dropMax *float64,
) {
	if len(can.PossibleHBs) == 0 {
		return
	}

	hasDistTarget := distMin != nil || distMax != nil
	if hasDistTarget {
		calcMid := func(minVal, maxVal *float64) float64 {
			if minVal != nil && maxVal != nil {
				return (*minVal + *maxVal) / 2.0
			}
			if minVal != nil {
				return *minVal
			}
			if maxVal != nil {
				return *maxVal
			}
			return 0
		}

		distMid := calcMid(distMin, distMax)
		dropMid := calcMid(dropMin, dropMax)

		activeBRP := can.DeadCenterBRP
		if can.SelectedBRP != nil {
			activeBRP = *can.SelectedBRP
		}

		dx0 := math.Sqrt(math.Max(0, distMid*distMid-dropMid*dropMid))
		dy0 := dropMid

		oval := DefaultFitOval(activeBRP.BrpX+dx0, activeBRP.BrpY+dy0)
		fitRes := EvaluateCockpitFit(can.PossibleHBs, canonicalStem, oval)
		can.CockpitFitResult = &fitRes
		can.SelectedHB = &fitRes.SelectedHB
		can.SaddleHbMm = fitRes.SelectedHB.SaddleHbMm
		can.SaddleHbDropMm = fitRes.SelectedHB.SaddleHbDropMm
		return
	}

	if bestHB, ok := SelectBestHBWithDrop(can.PossibleHBs, canonicalStem, distMin, distMax, dropMin, dropMax); ok {
		can.SelectedHB = &bestHB
		can.SaddleHbMm = bestHB.SaddleHbMm
		can.SaddleHbDropMm = bestHB.SaddleHbDropMm
	}
}

// CalculateSteererStack calculates the 'spare' steerer tube required to bring
// the handlebars level with the saddle.
// Returns:
// - verticalStack: Pure vertical distance from head tube top to saddle horizontal line (mm).
// - axialLength: Physical length along the angled steerer tube to achieve that height (mm).
func CalculateSteererStack(stack, staDeg, htaDeg, seatHeight float64) (verticalStack float64, axialLength float64) {
	seatAxis := SeatTubeAxis(geom2d.Origin, staDeg)
	saddlePoint := seatAxis.PointAt(seatHeight)

	verticalStack = saddlePoint.Y - stack
	steererDir := SteererTubeDir(htaDeg)
	axialLength = verticalStack / steererDir.Y

	return verticalStack, axialLength
}

// CalculateSeatpostExtension computes the axial extension from the top of the seat tube
// collar to the saddle reference point along the seat tube axis.
// Returns 0 if either seatHeight or seatTubeLength is non-positive.
func CalculateSeatpostExtension(seatHeight, seatTubeLength float64) float64 {
	if seatHeight <= 0 || seatTubeLength <= 0 {
		return 0
	}
	return Round(seatHeight-seatTubeLength, 1)
}

// CalculateGeometricA2c reverse-engineers the geometric Structural Axle-to-Crown (A2C) length
// that CAD software assumed to generate the published Stack number.
//
// Geometrically, the front axle sits at height Y = bbDrop relative to the BB spindle (0,0).
// Offsetting backward by rake along the perpendicular rake direction locates the steering axis.
// Projecting along the steerer axis up to the horizontal at Y = stack yields the total
// front-end axial length (structuralA2c + headTubeLength).
//
// Returns the structural A2C (total length beneath the head tube, including bare fork plus any lower cup stack).
// Callers needing bare fork A2C can subtract their lower cup stack or spacer from this value.
// Returns an error if the steerer axis is horizontal or if headTubeLength is non-positive.
func CalculateGeometricA2c(stack, bbDrop, headTubeLength, htaDeg, rake float64) (structuralA2c float64, err error) {
	if headTubeLength <= 0 {
		return 0, fmt.Errorf("headTubeLength must be positive, got %.1f mm", headTubeLength)
	}

	steererDir := SteererTubeDir(htaDeg)
	rakeDir := steererDir.Perpendicular(geom2d.Clockwise)

	// Front axle at height Y = bbDrop relative to bottom bracket:
	frontAxle := geom2d.Point2{X: 0, Y: bbDrop}

	// Project axle onto the steering axis by offsetting backward by fork rake:
	steererBase := frontAxle.SubVec(rakeDir.Scale(rake))
	steererAxis := geom2d.NewAxis(steererBase, steererDir)

	// Intersect with the horizontal at Y = stack:
	htTop, ok := steererAxis.IntersectHorizontalLine(stack)
	if !ok {
		return 0, fmt.Errorf("steerer axis is horizontal (HTA %.2f°)", htaDeg)
	}

	// Total axial length along steerer axis from axle projection to head tube top:
	frontEndLength := htTop.DistanceTo(steererBase)

	structuralA2c = frontEndLength - headTubeLength
	return structuralA2c, nil
}

// CalculateDynamicGeometry calculates new frame geometry when fork A2C is changed,
// simulating pitching of the frame around the rear axle.
func CalculateDynamicGeometry(
	baseReach, baseStack, baseHta, baseSta, baseBbDrop, baseStructuralA2c, newStructuralA2c float64,
	wheelbase, chainstayLength float64,
) model.DynamicGeometryResult {
	if wheelbase <= 0 {
		wheelbase = 1150.0
	}
	if chainstayLength <= 0 {
		chainstayLength = 430.0
	}

	deltaA2c := newStructuralA2c - baseStructuralA2c

	// Vertical lift at the front axle
	steererDir := SteererTubeDir(baseHta)
	deltaY := deltaA2c * steererDir.Y

	// Rotation angle around the rear axle
	pitchAngle, _ := geom2d.AngleFromLift(wheelbase, deltaY)
	thetaDeg := pitchAngle.Deg()

	newHta := baseHta - thetaDeg
	newSta := baseSta - thetaDeg

	// Rotate Reach/Stack vector around Bottom Bracket
	reachStack := geom2d.Vec2{X: baseReach, Y: baseStack}.Rotate(pitchAngle)
	newReach := reachStack.X
	newStack := reachStack.Y

	// Compute Bottom Bracket displacement pivoting around rear axle (0,0)
	cs := math.Max(chainstayLength, baseBbDrop+1.0)
	xBb := math.Sqrt(math.Max(0, cs*cs-baseBbDrop*baseBbDrop))

	// In rear axle coordinate system, BB initial coords are (xBb, -baseBbDrop)
	// Rotated by pitchAngle:
	newPBB := geom2d.Vec2{X: xBb, Y: -baseBbDrop}.Rotate(pitchAngle)
	newBbDrop := -newPBB.Y

	return model.DynamicGeometryResult{
		NewHta:        Round(newHta, 2),
		NewSta:        Round(newSta, 2),
		NewReach:      Round(newReach, 1),
		NewStack:      Round(newStack, 1),
		NewBbDrop:     Round(newBbDrop, 1),
		DeltaAngleDeg: Round(thetaDeg, 2),
		DeltaReach:    Round(newReach-baseReach, 1),
		DeltaStack:    Round(newStack-baseStack, 1),
		DeltaBbDrop:   Round(newBbDrop-baseBbDrop, 1),
		TotalNewA2c:   Round(newStructuralA2c, 1),
	}
}

// CalculateFrontCenter calculates theoretical horizontal Front-Center from frame geometry.
func CalculateFrontCenter(reach, htl, htaDeg, rake, structuralA2c float64) float64 {
	steererDir := SteererTubeDir(htaDeg)
	downSteerer := steererDir.Scale(-(structuralA2c + htl))
	rakeOffset := steererDir.Perpendicular(geom2d.Clockwise).Scale(rake)
	return reach + downSteerer.X + rakeOffset.X
}

// CalculateFrontCenter3D calculates direct point-to-point Front-Center from the bottom bracket spindle to the front axle.
func CalculateFrontCenter3D(reach, htl, htaDeg, rake, structuralA2c, bbDrop float64) float64 {
	fcHoriz := CalculateFrontCenter(reach, htl, htaDeg, rake, structuralA2c)
	return geom2d.Point2{X: fcHoriz, Y: -bbDrop}.DistanceToOrigin()
}

// CalculateTrail calculates ground trail in mm from head tube angle (deg),
// fork rake/offset (mm), and loaded wheel radius (mm):
//
//	Trail = (wheelRadiusMm * cos(HTA) - rakeMm) / sin(HTA)
func CalculateTrail(htaDeg, rakeMm, wheelRadiusMm float64) float64 {
	if htaDeg <= 0 || htaDeg >= 90 || wheelRadiusMm <= 0 {
		return 0
	}
	rad := ToRad(htaDeg)
	return (wheelRadiusMm*math.Cos(rad) - rakeMm) / math.Sin(rad)
}

// BiomechanicalPosture models the rider's contact points and upper body posture on a baseline bike.
type BiomechanicalPosture struct {
	BackAngleDeg float64 // Torso angle from horizontal in degrees (hip to shoulder)
	HipX         float64 // Hip joint X coordinate relative to BB (mm)
	HipY         float64 // Hip joint Y coordinate relative to BB (mm)
	GripX        float64 // Grip center X coordinate relative to BB (mm)
	GripY        float64 // Grip center Y coordinate relative to BB (mm)
	RiderCgX     float64 // Rider center of gravity X relative to BB (mm)
	RiderCgY     float64 // Rider center of gravity Y relative to BB (mm)
	SystemCgX    float64 // Combined rider + bike center of gravity X relative to BB (mm)
	SystemCgY    float64 // Combined rider + bike center of gravity Y relative to BB (mm)
}

// WeightDistributionResult holds the static weight distribution on front and rear axles.
type WeightDistributionResult struct {
	FrontPct        float64 // Percentage of system weight on front axle
	RearPct         float64 // Percentage of system weight on rear axle
	FrontCenterCalc float64 // Horizontal distance from BB to front axle (mm)
	RearCenterCalc  float64 // Horizontal distance from BB to rear axle (mm)
	WheelbaseCalc   float64 // Total horizontal wheelbase (mm)
	BackAngleDeg    float64 // Estimated back angle in degrees
}

// GetDefaultBaselineBike returns the canonical Otso Fenrir Stainless Large baseline build
// (rigid 495mm fork, EC44 lower cup, 780mm saddle height).
func GetDefaultBaselineBike() model.BikeGeometry {
	return model.BikeGeometry{
		ID:              "otso-fenrir-stainless-l",
		Manufacturer:    "Otso",
		Model:           "Fenrir",
		Size:            "L",
		Reach:           428.0,
		Stack:           642.0,
		HtaDeg:          68.25,
		StaDeg:          75.25,
		Htl:             140.0,
		BbDrop:          72.0,
		ForkRake:        54.0,
		Wheelbase:       1158.0,
		ChainstayLength: 450.0,
		ForkA2c:         495.0,
		HeadsetCupType:  model.CupEC44,
	}
}

// CalculateBaselinePosture models the rider's contact points, back angle, and Center of Gravity (CoG).
func CalculateBaselinePosture(
	bike model.BikeGeometry,
	profile model.RiderProfile,
) (BiomechanicalPosture, error) {
	if profile.SaddleHeightMm <= 0 {
		return BiomechanicalPosture{}, fmt.Errorf("profile.SaddleHeightMm must be positive, got %.1f", profile.SaddleHeightMm)
	}
	if profile.HeightIn <= 0 || profile.InseamIn <= 0 {
		return BiomechanicalPosture{}, fmt.Errorf("profile must specify positive height and inseam, got height=%.1f inseam=%.1f", profile.HeightIn, profile.InseamIn)
	}

	staDeg := bike.StaDeg
	if staDeg <= 0 {
		staDeg = 74.71
	}

	htaDeg := bike.HtaDeg
	if htaDeg <= 0 {
		htaDeg = 67.71
	}

	// 1. Hip joint coordinates relative to Bottom Bracket (0,0)
	seatAxis := SeatTubeAxis(geom2d.Origin, staDeg)
	saddlePt := seatAxis.PointAt(profile.SaddleHeightMm)
	saddleX := saddlePt.X - profile.BaselineSaddleSetbackOffsetMm
	saddleY := saddlePt.Y
	hip := geom2d.Point2{X: saddleX, Y: saddleY + 35.0} // Greater trochanter / acetabulum sits ~35mm above saddle top

	// 2. Handlebar grip coordinates relative to Bottom Bracket (0,0)
	reach := bike.Reach
	if reach <= 0 {
		reach = 421.9
	}
	stack := bike.Stack
	if stack <= 0 {
		stack = 646.0
	}
	headTubeTop := geom2d.Point2{X: reach, Y: stack}

	steererDir := SteererTubeDir(htaDeg)
	stemDir := steererDir.Perpendicular(geom2d.Clockwise).RotateDeg(profile.BaselineStemAngleDeg)
	clamp := headTubeTop.Add(stemDir.Scale(profile.BaselineStemMm))

	// Bar sweep & rise
	halfWidth := profile.BaselineBarWidthMm / 2.0
	if halfWidth <= 0 {
		halfWidth = 400.0
	}
	sweepOffset := geom2d.Vec2{
		X: -halfWidth * geom2d.FromDeg(profile.BaselineBarSweepDeg).Sin(),
		Y: profile.BaselineBarRiseMm,
	}
	grip := clamp.Add(sweepOffset)

	// 3. Biomechanical Triangle: Hip, Shoulder, Grip
	heightMm := profile.HeightIn * 25.4
	torsoLen := 0.288 * heightMm // Hip to shoulder (~541mm for 74")
	armLen := 0.380 * heightMm   // Shoulder to hand (~714mm for 74")

	// 3D lateral arm spread to 800mm wide bar:
	// Shoulder half-breadth ~200mm, grip half-breadth ~390mm -> lateral offset ~190mm
	lateralOffset := math.Max(0, halfWidth-200.0)
	// Effective arm reach projected in the sagittal plane with comfortable ~25° elbow flexion
	sagittalArmReach := math.Sqrt(math.Max(100.0, (0.88*armLen)*(0.88*armLen)-lateralOffset*lateralOffset))

	vHipGrip := grip.Sub(hip)
	distHipGrip := vHipGrip.Norm()
	alpha := vHipGrip.Angle()

	beta, err := geom2d.SolveTriangleSSS(torsoLen, distHipGrip, sagittalArmReach)
	if err != nil {
		return BiomechanicalPosture{}, err
	}

	backAngle := alpha.Add(beta)
	backAngleDeg := backAngle.Deg()

	// 4. Rider Center of Gravity
	torsoDir := geom2d.FromPolar(1.0, backAngle)
	shoulder := hip.Add(torsoDir.Scale(torsoLen))

	// Torso + Head (58% of body mass)
	upper := hip.Add(torsoDir.Scale(0.62 * torsoLen))

	// Arms (10% of body mass)
	arm := geom2d.Point2{X: (shoulder.X + grip.X) / 2.0, Y: (shoulder.Y + grip.Y) / 2.0}

	// Legs (32% of body mass) between Hip and Pedals (0,0)
	leg := geom2d.Point2{X: 0.45 * hip.X, Y: 0.40 * hip.Y}

	riderCg := geom2d.Point2{
		X: 0.58*upper.X + 0.10*arm.X + 0.32*leg.X,
		Y: 0.58*upper.Y + 0.10*arm.Y + 0.32*leg.Y,
	}

	// 5. System Center of Gravity (Rider + Bike)
	riderMassKg := profile.RiderWeightLbs * 0.45359237
	bikeMassKg := profile.BikeWeightLbs * 0.45359237
	bikeCg := geom2d.Point2{X: 0.0, Y: 280.0} // Center of mass of hardtail frame/wheels/drivetrain near BB

	totalMass := riderMassKg + bikeMassKg
	systemCg := geom2d.Point2{
		X: (riderMassKg*riderCg.X + bikeMassKg*bikeCg.X) / totalMass,
		Y: (riderMassKg*riderCg.Y + bikeMassKg*bikeCg.Y) / totalMass,
	}

	return BiomechanicalPosture{
		BackAngleDeg: Round(backAngleDeg, 1),
		HipX:         Round(hip.X, 1),
		HipY:         Round(hip.Y, 1),
		GripX:        Round(grip.X, 1),
		GripY:        Round(grip.Y, 1),
		RiderCgX:     Round(riderCg.X, 1),
		RiderCgY:     Round(riderCg.Y, 1),
		SystemCgX:    Round(systemCg.X, 1),
		SystemCgY:    Round(systemCg.Y, 1),
	}, nil
}

// CalculateWeightDistribution calculates static weight distribution between the axles for an active frame setup.
func CalculateWeightDistribution(
	activeReach, activeStack, activeHta, activeHtl, activeBbDrop, forkRake, structuralA2c, chainstayLength float64,
	posture BiomechanicalPosture,
) WeightDistributionResult {
	if chainstayLength <= 0 {
		chainstayLength = 440.0
	}
	cs := math.Max(chainstayLength, activeBbDrop+1.0)
	rearCenter := math.Sqrt(math.Max(0, cs*cs-activeBbDrop*activeBbDrop))
	xRear := -rearCenter

	frontCenter := CalculateFrontCenter(activeReach, activeHtl, activeHta, forkRake, structuralA2c)
	wheelbase := frontCenter + rearCenter
	if wheelbase <= 0 {
		wheelbase = 1150.0
	}

	distRear := posture.SystemCgX - xRear
	frontPct := (distRear / wheelbase) * 100.0
	rearPct := 100.0 - frontPct

	return WeightDistributionResult{
		FrontPct:        Round(frontPct, 1),
		RearPct:         Round(rearPct, 1),
		FrontCenterCalc: Round(frontCenter, 1),
		RearCenterCalc:  Round(rearCenter, 1),
		WheelbaseCalc:   Round(wheelbase, 1),
		BackAngleDeg:    Round(posture.BackAngleDeg, 1),
	}
}

// AnalyzeBikeFit runs the master analysis pipeline:
// reverse-engineers CAD baseline, applies optional canonical/target fork modifications,
// and derives all final dynamic cockpit, frame, and weight distribution metrics.
func AnalyzeBikeFit(
	bike model.BikeGeometry,
	targetA2c *float64,
	targetRake *float64,
	profile model.RiderProfile,
) (model.BikeFitAnalysis, error) {
	return AnalyzeBikeFitWithSaddle(bike, targetA2c, targetRake, profile, catalog.DefaultSaddle())
}

// AnalyzeBikeFitWithSaddle runs the master analysis pipeline using an explicit SaddleSpec.
func AnalyzeBikeFitWithSaddle(
	bike model.BikeGeometry,
	targetA2c *float64,
	targetRake *float64,
	profile model.RiderProfile,
	saddle model.SaddleSpec,
) (model.BikeFitAnalysis, error) {
	if saddle.ID == "" {
		saddle = catalog.DefaultSaddle()
	}
	if profile.SaddleHeightMm <= 0 {
		return model.BikeFitAnalysis{}, fmt.Errorf("profile.SaddleHeightMm must be positive, got %.1f", profile.SaddleHeightMm)
	}

	seatHeight := profile.SaddleHeightMm

	cupStack := bike.HeadsetCupType.CupStackMm()
	geoSpacer := bike.GeoHeadsetSpacerMm

	// 1. Reverse engineer baseline CAD geometric A2C
	baseStructuralA2c, err := CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
	)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}
	baseForkA2c := baseStructuralA2c - geoSpacer

	activeReach := bike.Reach
	activeStack := bike.Stack
	activeHta := bike.HtaDeg
	activeSta := bike.StaDeg
	if bike.HasBentSeatTube() {
		activeSta = bike.CalculateEffectiveSta(seatHeight, bike.Stack, bike.StaDeg)
	}
	activeBbDrop := bike.BbDrop
	activeStructuralA2c := baseStructuralA2c
	deltaAngle := 0.0
	deltaReach := 0.0
	deltaStack := 0.0
	deltaBbDrop := 0.0

	// 2. If target A2C provided, apply dynamic pitching using physical headset_cup_type stack
	if targetA2c != nil {
		totalNewA2c := *targetA2c + cupStack

		wb := bike.Wheelbase
		if wb <= 0 {
			wb = 1150.0
		}
		cs := bike.ChainstayLength
		if cs <= 0 {
			cs = 430.0
		}

		dyn := CalculateDynamicGeometry(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			activeSta,
			bike.BbDrop,
			baseStructuralA2c,
			totalNewA2c,
			wb,
			cs,
		)

		activeReach = dyn.NewReach
		activeStack = dyn.NewStack
		activeHta = dyn.NewHta
		activeSta = dyn.NewSta
		activeBbDrop = dyn.NewBbDrop
		activeStructuralA2c = dyn.TotalNewA2c
		deltaAngle = dyn.DeltaAngleDeg
		deltaReach = dyn.DeltaReach
		deltaStack = dyn.DeltaStack
		deltaBbDrop = dyn.DeltaBbDrop
	}

	// 3. Derived cockpit metrics
	aett := CalculateAett(activeReach, activeStack, activeHta, activeSta, seatHeight)
	actualSetback := CalculateActualSetback(activeSta, seatHeight)
	verticalStack, axialLength := CalculateSteererStack(activeStack, activeSta, activeHta, seatHeight)
	seatpostExtension := CalculateSeatpostExtension(seatHeight, bike.SeatTubeLength)

	// 4. Infer CAD publish mode
	inferredPublishMode := "Unspecified"
	cadA2c := bike.GetCadA2c()
	if cadA2c > 0 {
		diff := cadA2c - baseForkA2c
		a2cSlop := bike.GetGeoSlopMm().GetA2c()
		if math.Abs(diff) <= a2cSlop {
			inferredPublishMode = "Static Fork (0% Sag CAD)"
		} else if diff >= 15.0 && diff <= 35.0 {
			inferredPublishMode = fmt.Sprintf("Sagged (~20%% Sag CAD, ~%dmm sag)", int(math.Round(diff)))
		} else {
			inferredPublishMode = fmt.Sprintf("Discrepancy (%dmm deviation)", int(math.Round(diff)))
		}
	}

	// 5. Biomechanical Posture & Weight Distribution
	baselineBike := GetDefaultBaselineBike()
	posture, err := CalculateBaselinePosture(baselineBike, profile)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}

	activeCS := bike.ChainstayLength
	if activeCS <= 0 {
		activeCS = 440.0
	}
	csDesc := fmt.Sprintf("%.1f mm [fixed]", activeCS)
	if bike.HasAdjustableChainstay() {
		activeCS = bike.GetMaxChainstay()
		csDesc = fmt.Sprintf("%.1f mm [max of %.0f-%.0f]", activeCS, bike.GetMinChainstay(), bike.GetMaxChainstay())
	}

	htl := bike.Htl
	if htl <= 0 {
		htl = 120.0
	}
	rake := bike.ForkRake
	if targetRake != nil {
		rake = *targetRake
	}
	if rake <= 0 {
		return model.BikeFitAnalysis{}, fmt.Errorf("fork rake must be positive, got %.1fmm", rake)
	}

	weightDist := CalculateWeightDistribution(
		activeReach,
		activeStack,
		activeHta,
		htl,
		activeBbDrop,
		rake,
		activeStructuralA2c,
		activeCS,
		posture,
	)

	// 6. Cockpit BRP & HB Points and Saddle-Handlebar Distance
	deadCenterBRP := CalculateSaddleBRPPoint(
		activeSta,
		seatHeight,
		model.DefaultSeatpostSetbackMm,
		saddle,
		0.0,
	)
	possibleBRPs := MapPossibleSaddleBRPs(
		activeSta,
		seatHeight,
		model.DefaultSeatpostSetbackMm,
		saddle,
		model.DefaultSeatpostClampLengthMm,
	)
	selectedBRP := deadCenterBRP

	possibleHBs := MapPossibleHBs(bike, activeReach, activeStack, activeHta, selectedBRP)
	var selectedHB *model.HBPoint
	var saddleHbMm float64
	var saddleHbDropMm float64
	if best, ok := SelectBestHB(possibleHBs, bike.GetCanonicalStem(), nil, nil); ok {
		selectedHB = &best
		saddleHbMm = best.SaddleHbMm
		saddleHbDropMm = best.SaddleHbDropMm
	}

	idealSTA := saddle.IdealSeatTubeAngle(292.0, seatHeight, model.DefaultSeatpostSetbackMm)

	return model.BikeFitAnalysis{
		StructuralA2c:       Round(activeStructuralA2c, 1),
		ActiveReach:         Round(activeReach, 1),
		ActiveStack:         Round(activeStack, 1),
		ActiveHta:           Round(activeHta, 2),
		ActiveSta:           Round(activeSta, 2),
		ActiveBbDrop:        Round(activeBbDrop, 1),
		Aett:                Round(aett, 1),
		ActualSetback:       actualSetback,
		SpareVerticalStack:  Round(verticalStack, 1),
		SpareAxialSteerer:   Round(axialLength, 1),
		SeatpostExtensionMm: seatpostExtension,
		DeltaAngleDeg:       Round(deltaAngle, 2),
		DeltaReach:          Round(deltaReach, 1),
		DeltaStack:          Round(deltaStack, 1),
		DeltaBbDrop:         Round(deltaBbDrop, 1),
		InferredPublishMode: inferredPublishMode,

		WeightDistFrontPct: weightDist.FrontPct,
		WeightDistRearPct:  weightDist.RearPct,
		BackAngleDeg:       weightDist.BackAngleDeg,
		FrontCenterCalc:    weightDist.FrontCenterCalc,
		RearCenterCalc:     weightDist.RearCenterCalc,
		SeatToRearCenter:   Round(weightDist.RearCenterCalc-actualSetback, 1),
		WheelbaseCalc:      weightDist.WheelbaseCalc,
		ActiveChainstay:    Round(activeCS, 1),
		ChainstayDesc:      csDesc,
		TrailMm:            Round(CalculateTrail(activeHta, rake, bike.WheelRadiusMm()), 1),

		Saddle:              saddle,
		SitBoneSetbackMm:    deadCenterBRP.SitBoneSetbackMm,
		IdealStaDeg:         Round(idealSTA, 1),
		AchievedClampOffset: deadCenterBRP.AchievedClampOffsetMm,
		DeadCenterBRP:       deadCenterBRP,
		SelectedBRP:         &selectedBRP,
		PossibleBRPs:        possibleBRPs,
		SaddleHbMm:          saddleHbMm,
		SaddleHbDropMm:      saddleHbDropMm,
		SelectedHB:          selectedHB,
		PossibleHBs:         possibleHBs,
	}, nil
}

// MaterializeBikeGeometry projects a FrameModel for a given size into a BikeGeometry instance.
func MaterializeBikeGeometry(frame model.FrameModel, size string) (model.BikeGeometry, error) {
	if size == "" {
		size = frame.GetDefaultSize()
	}

	geo, ok := frame.GetSizeGeometry(size)
	if !ok {
		return model.BikeGeometry{}, fmt.Errorf("size %s not defined in frame %s", size, frame.ID)
	}

	bikeID := frame.ID
	if !strings.HasSuffix(strings.ToLower(bikeID), "-"+strings.ToLower(size)) {
		bikeID = fmt.Sprintf("%s-%s", frame.ID, strings.ToLower(size))
	}

	cadA2c := frame.CadFork.A2cMm
	cadRake := frame.CadFork.RakeMm
	if cadRake <= 0 {
		cadRake = 44.0
	}

	return model.BikeGeometry{
		ID:                 bikeID,
		Manufacturer:       frame.Manufacturer,
		Model:              frame.Model,
		Size:               size,
		Year:               frame.Year,
		Category:           frame.Category,
		Notes:              frame.Notes,
		WheelSize:          frame.WheelSize,
		HeadsetCupType:     frame.HeadsetCupType,
		GeoHeadsetSpacerMm: frame.GeoHeadsetSpacerMm,
		GeoSlopMm:          frame.GeoSlopMm,
		ActualStaDeg:       frame.ActualStaDeg,
		SeatTubeOffsetXMm:  frame.SeatTubeOffsetXMm,
		CadFork:            frame.CadFork,
		ForkA2c:            cadA2c,
		ForkRake:           cadRake,
		Reach:              geo.Reach,
		Stack:              geo.Stack,
		HtaDeg:             geo.HtaDeg,
		StaDeg:             geo.StaDeg,
		Htl:                geo.Htl,
		BbDrop:             geo.BbDrop,
		Wheelbase:          geo.Wheelbase,
		ChainstayLength:    geo.ChainstayLength,
		FrontCenter:        geo.FrontCenter,
		SeatTubeLength:     geo.SeatTubeLength,
		StandoverHeight:    geo.StandoverHeight,
		ChainstayRangeMm:   frame.ChainstayRangeMm,
		MaxSpacerMm:        frame.MaxSpacerMm,
		CanonicalStem:      frame.CanonicalStem,
		Stems:              catalog.ExpandStemSets(frame.Stems),
	}, nil
}
