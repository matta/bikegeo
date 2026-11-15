package geometry

import (
	"math"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// SelectCheapestBRP returns the BRP candidate (saddle rail position) with the
// lowest cost. Ties go to the position nearer dead center on the rails, then to
// the earlier candidate.
func SelectCheapestBRP(possible []model.BRPPoint, cost func(model.BRPPoint) float64) (model.BRPPoint, bool) {
	if len(possible) == 0 || cost == nil {
		return model.BRPPoint{}, false
	}
	best, bestCost := possible[0], cost(possible[0])
	for _, pt := range possible[1:] {
		c := cost(pt)
		if c < bestCost-1e-9 || (math.Abs(c-bestCost) <= 1e-9 && math.Abs(pt.RailOffsetMm) < math.Abs(best.RailOffsetMm)-1e-9) {
			best, bestCost = pt, c
		}
	}
	return best, true
}

// MapPossibleHBsWithHandlebar maps all possible HB points for a bike across its allowed stems and spacer stack heights
// using the specified handlebar, grip inset, and roll angle.
func MapPossibleHBsWithHandlebar(
	bike model.BikeGeometry,
	activeReach, activeStack float64, activeHta model.Deg,
	brp model.BRPPoint,
	bar model.HandlebarSpec,
	gripInsetMm float64, rollDeg model.Deg,
) []model.HBPoint {
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	stems := bike.Stems
	if len(stems) == 0 && bike.CanonicalStem.LengthMm > 0 {
		stems = []model.StemSpec{bike.CanonicalStem}
	}
	spacers := bike.GetSpacersMm()

	var points []model.HBPoint
	for _, st := range stems {
		for _, sp := range spacers {
			pt := CalculateHBPointWithHandlebar(activeReach, activeStack, activeHta, brp, st, sp, bar, gripInsetMm, rollDeg)
			points = append(points, pt)
		}
	}
	return points
}

// EvaluatedPointForHB returns the 2D cockpit point on pt evaluated against targets
// under targetRef (with optional refBar for TargetReferenceBarClamp).
// An empty targetRef defaults to TargetHandGrip for legacy callers.
func EvaluatedPointForHB(pt model.HBPoint, targetRef model.TargetRef, refBar model.Opt[model.HandlebarSpec]) geom2d.Point2 {
	handGrip := geom2d.Point2{X: pt.HbX, Y: pt.HbY}
	switch targetRef {
	case TargetHandGrip:
		return handGrip
	case TargetStemClamp:
		return geom2d.Point2{X: pt.ClampX, Y: pt.ClampY}
	case TargetReferenceBarClamp:
		if bar, ok := refBar.Get(); ok {
			refOff := GetHandlebarOffset(bar, model.DefaultGripInsetMm, 0.0)
			return handGrip.SubVec(geom2d.Vec2{X: refOff.X, Y: refOff.Y})
		}
		return geom2d.Point2{X: pt.ClampX, Y: pt.ClampY}
	default:
		return handGrip
	}
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
		Angle:      oval.AngleDeg.Angle(),
	}
	return e.Evaluate(geom2d.Point2{X: x, Y: y})
}

// CockpitCost prices one stock cockpit candidate (stem length × angle × spacer
// stack) whose evaluated point sits at normalized oval radius r.
type CockpitCost func(pt model.HBPoint, r float64) float64

// EvaluateCockpitFitByCost scans every stock cockpit candidate against a FitOval
// and keeps the cheapest under cost. Ties go to the candidate nearer the oval
// center, then to the earlier candidate.
func EvaluateCockpitFitByCost(
	possible []model.HBPoint,
	oval model.FitOval,
	targetRef model.TargetRef,
	refBar model.Opt[model.HandlebarSpec],
	cost CockpitCost,
) model.CockpitFitResult {
	if targetRef == "" {
		targetRef = TargetHandGrip
	}
	if len(possible) == 0 || cost == nil {
		return model.CockpitFitResult{
			TargetOval:   oval,
			TargetRef:    targetRef,
			ReferenceBar: refBar,
			BestR:        999,
		}
	}

	type candidate struct {
		pt model.HBPoint
		r  float64
	}
	candidates := make([]candidate, len(possible))
	countInside := 0
	for i, pt := range possible {
		evalPt := EvaluatedPointForHB(pt, targetRef, refBar)
		r, inside := EvaluateFitOval(oval, evalPt.X, evalPt.Y)
		if inside {
			countInside++
		}
		candidates[i] = candidate{pt: pt, r: r}
	}

	bestIdx, bestCost := 0, cost(candidates[0].pt, candidates[0].r)
	for i := 1; i < len(candidates); i++ {
		c := cost(candidates[i].pt, candidates[i].r)
		if c < bestCost-1e-9 || (math.Abs(c-bestCost) <= 1e-9 && candidates[i].r < candidates[bestIdx].r-1e-9) {
			bestIdx, bestCost = i, c
		}
	}
	best := candidates[bestIdx]
	return model.CockpitFitResult{
		TargetOval:   oval,
		TargetRef:    targetRef,
		ReferenceBar: refBar,
		SelectedHB:   best.pt,
		BestR:        Round(best.r, 3),
		CountInside:  countInside,
		HasFit:       countInside > 0,
	}
}

// ApplyCockpitFitByCost places the FitOval for the explicit cockpit target (the
// hands distMm from the active BRP and riseMm above its sit bones), scans every
// stock cockpit candidate (see EvaluateCockpitFitByCost) and records the cheapest
// on can: CockpitFitResult, SelectedHB, SaddleHbMm, SaddleHbDropMm and the
// stem/spacer solve for the oval centre. Does nothing without candidates or a
// positive distMm.
func ApplyCockpitFitByCost(
	can *model.BikeFitAnalysis,
	distMm, riseMm float64,
	targetRef model.TargetRef,
	refBar model.Opt[model.HandlebarSpec],
	cost CockpitCost,
) {
	if len(can.PossibleHBs) == 0 || distMm <= 0 {
		return
	}

	activeBRP := can.DeadCenterBRP
	if can.SelectedBRP != nil {
		activeBRP = *can.SelectedBRP
	}

	brpPt := activeBRP.BrpPoint()
	sitBonePt := activeBRP.SitBonePoint()
	hbY := sitBonePt.Y + riseMm
	dyBrp := hbY - brpPt.Y
	dx0 := math.Sqrt(math.Max(0, distMm*distMm-dyBrp*dyBrp))

	oval := DefaultFitOval(brpPt.X+dx0, hbY)
	fitRes := EvaluateCockpitFitByCost(can.PossibleHBs, oval, targetRef, refBar, cost)
	can.CockpitFitResult = &fitRes
	can.SelectedHB = &fitRes.SelectedHB
	can.SaddleHbMm = fitRes.SelectedHB.SaddleHbMm
	can.SaddleHbDropMm = fitRes.SelectedHB.SaddleHbDropMm
	if can.ActiveHta <= 0 {
		return
	}
	reqClamp, err := RequiredStemClamp(RiderFit{
		TargetPoint:  geom2d.Point2{X: oval.CenterX, Y: oval.CenterY},
		TargetRef:    targetRef,
		ReferenceBar: refBar,
	}, CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         can.Handlebar,
		GripInsetMm:       model.DefaultGripInsetMm,
	})
	if err != nil {
		return
	}
	sc := BikeScene{
		HeadTubeTop:       geom2d.Point2{X: can.ActiveReach, Y: can.ActiveStack},
		HtaDeg:            can.ActiveHta,
		Cockpit:           CockpitSpec{HeadsetTopStackMm: model.DefaultHeadsetTopStackMm, StemClampHeightMm: model.StemThicknessMm},
		RequiredStemClamp: reqClamp,
	}
	if solved, ok := sc.SolveStemSpacerFit(fitRes.SelectedHB.Stem.AngleDeg); ok {
		can.StemSpacerFit = &model.StemSpacerFit{
			StemAngleDeg: solved.StemAngleDeg,
			StemLengthMm: solved.StemLengthMm,
			SpacerMm:     solved.SpacerMm,
		}
	}
}

// canonicalHB returns the bike's canonical build among the candidates: the
// canonical stem's length and angle with the smallest spacer stack, or the nearest
// stock stem when the canonical one is not stocked.
func canonicalHB(possible []model.HBPoint, canonical model.StemSpec) (model.HBPoint, bool) {
	if len(possible) == 0 {
		return model.HBPoint{}, false
	}
	best := possible[0]
	for _, pt := range possible[1:] {
		switch {
		case math.Abs(pt.Stem.LengthMm-canonical.LengthMm) < math.Abs(best.Stem.LengthMm-canonical.LengthMm)-0.001:
			best = pt
		case math.Abs(pt.Stem.LengthMm-canonical.LengthMm) > math.Abs(best.Stem.LengthMm-canonical.LengthMm)+0.001:
		case (pt.Stem.AngleDeg - canonical.AngleDeg).Abs() < (best.Stem.AngleDeg-canonical.AngleDeg).Abs()-0.001:
			best = pt
		case (pt.Stem.AngleDeg - canonical.AngleDeg).Abs() > (best.Stem.AngleDeg-canonical.AngleDeg).Abs()+0.001:
		case pt.SpacerMm < best.SpacerMm-0.001:
			best = pt
		}
	}
	return best, true
}
