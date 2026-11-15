package geometry

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// Round rounds a float (a length in mm, or a model.Deg) to the specified decimal precision.
func Round[T ~float64](val T, decimals int) T {
	factor := math.Pow(10, float64(decimals))
	return T(math.Round(float64(val)*factor) / factor)
}

// CalculateBentSeatTubeOffset derives the horizontal bottom bracket forward offset (Ox) in mm
// for a bent or offset seat tube using the Stack plane (Method A).
func CalculateBentSeatTubeOffset(stack float64, actualStaDeg, nominalStaDeg model.Deg) float64 {
	if actualStaDeg <= 0 || nominalStaDeg <= 0 || stack <= 0 {
		return 0
	}
	cotAct := 1.0 / actualStaDeg.Angle().Tan()
	cotNom := 1.0 / nominalStaDeg.Angle().Tan()
	return stack * (cotAct - cotNom)
}

// CalculateEffectiveSta computes the true effective seat tube angle in degrees for a rider at
// radial saddle height L (distance from BB spindle center to saddle along the effective angle).
// Law of Sines: alpha = asin((Ox * sin(theta_act)) / L), theta_eff = theta_act + alpha.
func CalculateEffectiveSta(actualStaDeg model.Deg, offsetXMm, saddleDistanceL float64) model.Deg {
	if saddleDistanceL <= 0 || actualStaDeg <= 0 {
		return actualStaDeg
	}
	sinVal := (offsetXMm * actualStaDeg.Angle().Sin()) / saddleDistanceL
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return actualStaDeg + model.DegOf(geom2d.FromRad(alphaRad))
}

// SeatTubeAxis returns a directional Axis2 along the seat tube starting at origin (typically BB spindle at 0,0).
// In bicycle frame geometry, STA is measured from the rear horizontal ground line, sweeping clockwise.
func SeatTubeAxis(origin geom2d.Point2, staDeg model.Deg) geom2d.Axis2 {
	return geom2d.NewAxisFromPolar(origin, (180.0 - staDeg).Angle())
}

// SteererTubeDir returns a unit vector pointing upward along the steerer tube axis.
func SteererTubeDir(htaDeg model.Deg) geom2d.Vec2 {
	return geom2d.FromPolar(1.0, (180.0 - htaDeg).Angle())
}

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
func CalculateGeometricA2c(stack, bbDrop, headTubeLength float64, htaDeg model.Deg, rake float64) (structuralA2c float64, err error) {
	if headTubeLength <= 0 {
		return 0, fmt.Errorf("headTubeLength must be positive, got %.1f mm", headTubeLength)
	}

	frontEnd, ok := frontEndLength(stack, bbDrop, htaDeg, rake)
	if !ok {
		return 0, fmt.Errorf("steerer axis is horizontal (HTA %.2f°)", htaDeg)
	}

	structuralA2c = frontEnd - headTubeLength
	return structuralA2c, nil
}

// frontEndLength is the total axial length along the steerer axis from the front
// axle's projection onto that axis up to the head tube top (structural A2C plus
// head tube length). ok is false when the steerer axis is horizontal.
func frontEndLength(stack, bbDrop float64, htaDeg model.Deg, rake float64) (length float64, ok bool) {
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
		return 0, false
	}

	return htTop.DistanceTo(steererBase), true
}

// CalculateTrail calculates ground trail in mm from head tube angle (deg),
// fork rake/offset (mm), and loaded wheel radius (mm):
//
//	Trail = (wheelRadiusMm * cos(HTA) - rakeMm) / sin(HTA)
func CalculateTrail(htaDeg model.Deg, rakeMm, wheelRadiusMm float64) float64 {
	if htaDeg <= 0 || htaDeg >= 90 || wheelRadiusMm <= 0 {
		return 0
	}
	hta := htaDeg.Angle()
	return (wheelRadiusMm*hta.Cos() - rakeMm) / hta.Sin()
}

// AnalyzeBikeFitWithSaddle runs the master analysis pipeline using an explicit SaddleSpec.
func AnalyzeBikeFitWithSaddle(
	bike model.BikeGeometry,
	targetA2c model.Opt[float64],
	targetRake model.Opt[float64],
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
	refSQ := model.ReferenceIdmatchSaddle()
	refSitBones := model.ReferenceSitBonePoint(seatHeight, model.DefaultTargetSitBoneSetbackMm)
	brpSeatHeight := saddle.BrpPointFromSitBones(refSitBones).DistanceToOrigin()

	// 1. Validate rake and reverse-engineer baseline CAD geometric A2C for publish-mode inference.
	rake := targetRake.Or(bike.ForkRake)
	if rake <= 0 {
		return model.BikeFitAnalysis{}, fmt.Errorf("fork rake must be positive, got %.1fmm", rake)
	}
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
	geoSpacer := bike.GeoHeadsetSpacerMm
	baseForkA2c := baseStructuralA2c - geoSpacer

	// 2. Construct BikeScene from frame, build, cockpit, seat, and fit inputs.
	frame, err := FrameSizeFromBikeGeometry(bike)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}

	buildFork := frame.CadFork
	buildFork.RakeMm = rake
	buildSag := frame.CadSagPct
	physicalLowerCup := frame.CadLowerCupMm
	if tA2c, ok := targetA2c.Get(); ok {
		if !(bike.CadFork.A2cMm > 0 && tA2c == bike.CadFork.A2cMm) {
			buildFork = Fork{StaticA2cMm: tA2c, RakeMm: rake}
			buildSag = 0
		}
		if bike.HeadsetCupType.IsValid() {
			physicalLowerCup = bike.HeadsetCupType.CupStackMm()
		}
	}
	build := BuildSpec{
		Fork:               buildFork,
		SagPct:             buildSag,
		PhysicalLowerCupMm: physicalLowerCup,
		WheelRadiusFrontMm: bike.WheelRadiusMm(),
		WheelRadiusRearMm:  bike.WheelRadiusMm(),
	}

	bar := profile.Handlebar
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	stems := bike.Stems
	if len(stems) == 0 && bike.CanonicalStem.LengthMm > 0 {
		stems = []model.StemSpec{bike.CanonicalStem}
	}
	baseStem := bike.CanonicalStem
	if baseStem.LengthMm <= 0 {
		if len(stems) > 0 && stems[0].LengthMm > 0 {
			baseStem = stems[0]
		} else {
			baseStem = model.StemSpec{LengthMm: 70.0}
		}
	}
	cockpit := CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          0.0,
		Stem:              baseStem,
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         bar,
		BarRollDeg:        0.0,
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
		RailOffsetMm:  0.0,
	}
	targetRef := profile.TargetRef
	if targetRef == "" {
		targetRef = TargetHandGrip
	} else if !targetRef.IsValid() {
		return model.BikeFitAnalysis{}, fmt.Errorf("invalid RiderProfile TargetRef %q", profile.TargetRef)
	}
	fitTarget := RiderFit{
		FrameSeatAxisHeightMm: seatHeight,
		BrpHeightMm:           brpSeatHeight,
		TargetSitBonesXMm:     -model.DefaultTargetSitBoneSetbackMm,
		TargetPoint:           geom2d.Point2{X: 480.9, Y: 726.1},
		TargetRef:             targetRef,
		ReferenceSaddle:       refSQ,
		ReferenceBar:          profile.ReferenceBar,
	}

	scene, err := BuildScene(frame, build, cockpit, seat, fitTarget)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}

	activeReach := scene.Reach()
	activeStack := scene.Stack()
	activeHta := scene.HtaDeg
	activeSta := scene.EffectiveStaDeg
	activeBbDrop := scene.BbDrop()
	activeStructuralA2c := scene.StructuralA2cMm
	if !targetA2c.IsSet() {
		activeStructuralA2c = baseStructuralA2c
	}
	deltaAngle := scene.PitchDeg
	deltaReach := activeReach - bike.Reach
	deltaStack := activeStack - bike.Stack
	deltaBbDrop := activeBbDrop - bike.BbDrop

	// 3. Derived frame seat-axis & cockpit metrics from BikeScene
	aett := scene.AettMm()
	ett := scene.EttMm()
	actualSetback := scene.SeatAxisActualSetbackMm()
	verticalStack := scene.SpareVerticalStackMm()
	axialLength := scene.SpareAxialSteererMm()
	seatpostExtension, _ := scene.SeatpostExtensionMm()

	// 4. Infer CAD publish mode
	inferredPublishMode := "Unspecified"
	cadA2c := bike.GetCadA2c()
	if cadA2c > 0 {
		diff := cadA2c - baseForkA2c
		if budget, ok := A2cBudget(bike); ok && budget.Within() {
			inferredPublishMode = "Static Fork (0% Sag CAD)"
		} else if diff >= 15.0 && diff <= 35.0 {
			inferredPublishMode = fmt.Sprintf("Sagged (~20%% Sag CAD, ~%dmm sag)", int(math.Round(diff)))
		} else {
			inferredPublishMode = fmt.Sprintf("Discrepancy (%dmm deviation)", int(math.Round(diff)))
		}
	}

	// 5. Chainstay Description
	activeCS := scene.Frame.ChainstayMm
	csDesc := fmt.Sprintf("%.1f mm [fixed]", activeCS)
	if bike.HasAdjustableChainstay() {
		activeCS = bike.GetMaxChainstay()
		csDesc = fmt.Sprintf("%.1f mm [max of %.0f-%.0f]", activeCS, bike.GetMinChainstay(), bike.GetMaxChainstay())
	}

	// 6. Cockpit BRP & HB Points and Saddle-Handlebar Distance from BikeScene
	deadCenterBRP := model.BRPPoint{
		Saddle:                saddle,
		SeatpostSetbackMm:     seat.PostSetbackMm,
		SaddleBrpOffsetMm:     saddle.RailCenterToBrpMm,
		RailOffsetMm:          seat.RailOffsetMm,
		AchievedClampOffsetMm: seat.RailOffsetMm,
		TotalPerpOffsetMm:     seat.PostSetbackMm + seat.RailOffsetMm - saddle.RailCenterToBrpMm,
		BrpX:                  scene.BRP.X,
		BrpY:                  scene.BRP.Y,
		SitBoneX:              scene.SitBones.X,
		SitBoneY:              scene.SitBones.Y,
		BrpSetbackMm:          scene.BrpSetbackMm(),
		SitBoneSetbackMm:      scene.SitBoneSetbackMm(),
		BrpAngleFromBbDeg:     scene.BrpAngleFromBbDeg(),
	}
	possibleBRPs := MapPossibleSaddleBRPs(
		activeSta,
		brpSeatHeight,
		model.DefaultSeatpostSetbackMm,
		saddle,
		model.DefaultSeatpostClampLengthMm,
	)
	selectedBRP := deadCenterBRP

	spacers := bike.GetSpacersMm()
	var possibleHBs []model.HBPoint
	for _, st := range stems {
		for _, sp := range spacers {
			cand := cockpit
			cand.Stem = st
			cand.SpacerMm = sp
			sc, err := scene.WithCockpit(cand)
			if err != nil {
				return model.BikeFitAnalysis{}, err
			}
			possibleHBs = append(possibleHBs, model.HBPoint{
				Stem:           st,
				SpacerMm:       sp,
				ClampX:         sc.StemClamp.X,
				ClampY:         sc.StemClamp.Y,
				HbX:            sc.HandGrip.X,
				HbY:            sc.HandGrip.Y,
				SaddleHbMm:     sc.GripDistFromBrpMm(),
				SaddleHbDropMm: sc.GripRiseAboveSitBonesMm(),
			})
		}
	}
	var selectedHB *model.HBPoint
	var saddleHbMm float64
	var saddleHbDropMm float64
	if best, ok := canonicalHB(possibleHBs, bike.GetCanonicalStem()); ok {
		selectedHB = &best
		saddleHbMm = best.SaddleHbMm
		saddleHbDropMm = best.SaddleHbDropMm
	}

	solveAngle := baseStem.AngleDeg
	if selectedHB != nil {
		solveAngle = selectedHB.Stem.AngleDeg
	}
	var stemSpacerFit *model.StemSpacerFit
	if solved, ok := scene.SolveStemSpacerFit(solveAngle); ok {
		stemSpacerFit = &model.StemSpacerFit{
			StemAngleDeg: solved.StemAngleDeg,
			StemLengthMm: solved.StemLengthMm,
			SpacerMm:     solved.SpacerMm,
		}
	}

	idealSTA := saddle.IdealSeatTubeAngle(model.DefaultTargetSitBoneSetbackMm, brpSeatHeight, model.DefaultSeatpostSetbackMm)

	return model.BikeFitAnalysis{
		StructuralA2c:       activeStructuralA2c,
		ActiveReach:         activeReach,
		ActiveStack:         activeStack,
		ActiveHta:           activeHta,
		ActiveSta:           activeSta,
		ActiveBbDrop:        activeBbDrop,
		Aett:                aett,
		Ett:                 ett,
		ActualSetback:       actualSetback,
		SpareVerticalStack:  verticalStack,
		SpareAxialSteerer:   axialLength,
		SeatpostExtensionMm: seatpostExtension,
		DeltaAngleDeg:       deltaAngle,
		DeltaReach:          deltaReach,
		DeltaStack:          deltaStack,
		DeltaBbDrop:         deltaBbDrop,
		InferredPublishMode: inferredPublishMode,

		RearCenterWheelbaseRatio: scene.RearCenterWheelbaseRatio(),
		SeatToRearCenter:         scene.RearCenter() - actualSetback,
		WheelbaseCalc:            scene.Wheelbase(),
		ActiveChainstay:          activeCS,
		ChainstayDesc:            csDesc,
		TrailMm:                  scene.TrailMm(),

		Saddle:              saddle,
		Handlebar:           bar,
		SitBoneSetbackMm:    deadCenterBRP.SitBoneSetbackMm,
		IdealStaDeg:         Round(idealSTA, 1),
		AchievedClampOffset: deadCenterBRP.AchievedClampOffsetMm,
		DeadCenterBRP:       deadCenterBRP,
		SelectedBRP:         &selectedBRP,
		PossibleBRPs:        possibleBRPs,
		SaddleHbMm:          saddleHbMm,
		SaddleHbDropMm:      saddleHbDropMm,
		SelectedHB:          selectedHB,
		StemSpacerFit:       stemSpacerFit,
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
		return model.BikeGeometry{}, fmt.Errorf("frame %q requires positive CAD fork rake, got %v", frame.ID, cadRake)
	}

	return model.BikeGeometry{
		ID:                  bikeID,
		Manufacturer:        frame.Manufacturer,
		Model:               frame.Model,
		Size:                size,
		Year:                frame.Year,
		Category:            frame.Category,
		Notes:               frame.Notes,
		WheelSize:           frame.WheelSize,
		HeadsetCupType:      frame.HeadsetCupType,
		GeoHeadsetSpacerMm:  frame.GeoHeadsetSpacerMm,
		GeoSlopMm:           frame.GeoSlopMm,
		ChartPrecision:      InferChartPrecision(frame),
		ActualStaDeg:        frame.ActualStaDeg,
		SeatTubeOffsetXMm:   frame.SeatTubeOffsetXMm,
		CadFork:             frame.CadFork,
		ForkA2c:             cadA2c,
		ForkRake:            cadRake,
		Reach:               geo.Reach,
		Stack:               geo.Stack,
		HtaDeg:              geo.HtaDeg,
		StaDeg:              geo.StaDeg,
		Htl:                 geo.Htl,
		BbDrop:              geo.BbDrop,
		Wheelbase:           geo.Wheelbase,
		ChainstayLength:     geo.ChainstayLength,
		FrontCenter:         geo.FrontCenter,
		SeatTubeLength:      geo.SeatTubeLength,
		StandoverHeight:     geo.StandoverHeight,
		ChainstayRangeMm:    frame.ChainstayRangeMm,
		MaxSpacerMm:         frame.MaxSpacerMm,
		CanonicalStem:       frame.CanonicalStem,
		Stems:               catalog.ExpandStemSets(frame.Stems),
		ExpectedStemRangeMm: frame.ExpectedStemRangeMm,
	}, nil
}
