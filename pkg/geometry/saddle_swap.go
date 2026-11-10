package geometry

import (
	"math"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// SwapFitOutcome encapsulates the quantitative fit metrics resulting from either
// naive BRP alignment or true pelvic (sit-bone) fit translation in 2D space.
type SwapFitOutcome struct {
	Mode                    string  // e.g. "Naive BRP Alignment" or "True Pelvic Alignment"
	TargetBrpSetbackMm      float64 // Caliper setback to 70mm BRP
	BrpSetbackShiftMm       float64 // Shift in BRP setback relative to baseline source BRP
	TargetBrpHeightMm       float64 // Vertical height of BRP above BB spindle
	TargetSaddleHeightMm    float64 // Radial distance from BB spindle to 70mm BRP
	BrpXMm                  float64 // Horizontal X coordinate of BRP from BB spindle
	BrpYMm                  float64 // Vertical Y coordinate of BRP from BB spindle
	BrpAngleDeg             float64 // Polar angle from BB spindle to BRP
	AchievedSitBoneMm       float64 // Actual horizontal sit bone setback behind BB
	AchievedSitBoneHeightMm float64 // Actual vertical sit bone height above BB
	SitBoneDeltaMm          float64 // Difference from baseline sit bone setback (Achieved - Desired)
	SitBoneDeltaYMm         float64 // Difference from baseline sit bone height (Achieved - Desired)
	EffectiveStaDeg         float64 // Polar seat tube angle from BB to sit bone support point
	CockpitReachMm          float64 // Direct 2D Euclidean distance from sit bones to handlebar clamp
	CockpitDeltaMm          float64 // Change in cockpit reach from baseline
	LegExtensionMm          float64 // Radial Euclidean distance from BB spindle to sit bones
	LegExtensionDeltaMm     float64 // Change in leg extension from baseline
	RequiredClampOffsetMm   float64 // Required fore-aft rail clamp offset
	ClampMinMm              float64 // Minimum safe rail clamp offset
	ClampMaxMm              float64 // Maximum safe rail clamp offset
	IsFeasible              bool    // Whether required offset falls within [ClampMin, ClampMax]
	MarginFromLimitMm       float64 // Distance to closest rail limit (negative if exceeded)
}

// HardwareSwapRow captures seat tube angle and rail clamp requirements across
// standard seatpost hardware options for the true pelvic fit.
type HardwareSwapRow struct {
	PostName           string
	PostSetbackMm      float64
	SeatTubeSetbackMm  float64
	SourceIdealStaDeg  float64
	TargetIdealStaDeg  float64
	StaDeltaDeg        float64
	TargetRailOffsetMm float64
	IsFeasible30Mm     bool
	IsFeasible45Mm     bool
}

// SaddleSwapReport encapsulates the complete comparative analysis between
// a source saddle and a target saddle under both fit paradigms.
type SaddleSwapReport struct {
	SourceSaddle                   model.SaddleSpec
	TargetSaddle                   model.SaddleSpec
	SaddleHeightMm                 float64
	SitBoneSetbackMm               float64
	SourceBrpSetbackMm             float64
	SourceBrpHeightMm              float64
	SourceBrpX                     float64
	SourceBrpY                     float64
	SourceBrpAngleDeg              float64
	SourceSitBoneHeightMm          float64
	SourceStepElevationMm          float64
	TargetStepElevationMm          float64
	StepElevationDeltaMm           float64 // Target step - Source step
	TargetEquivalentBrpSetbackMm   float64
	TargetEquivalentBrpHeightMm    float64
	TargetEquivalentSaddleHeightMm float64
	BrpSetbackShiftMm              float64 // Target equivalent BRP setback - Source BRP setback (-15.0mm)
	BrpHeightShiftMm               float64 // Target equivalent BRP height - Source BRP height (+10.0mm)
	BrpSaddleHeightShiftMm         float64 // Target equivalent SH - Source SH (+5.4mm)
	TargetEquivalentBrpX           float64
	TargetEquivalentBrpY           float64
	TargetEquivalentBrpAngleDeg    float64
	BrpDeltaXMm                    float64 // Target X - Source X (+15.0mm forward)
	BrpDeltaYMm                    float64 // Target Y - Source Y (+10.0mm vertical)
	BrpDeltaAngleDeg               float64 // Target angle - Source angle (+1.31deg)
	SeatpostSetbackMm              float64
	ClampLengthMm                  float64
	HbDistanceMm                   float64
	HbDropMm                       float64
	BrpToSitBonesSpanDeltaMm       float64 // Target span - Source span (BRP skew)
	RailCenterToBrpDeltaMm         float64 // Target - Source
	RailCenterToSitBonesDeltaMm    float64 // Target - Source (true pelvic rail delta)
	BaselineCockpitReachMm         float64
	BaselineLegExtensionMm         float64
	BaselineStaDeg                 float64
	Naive                          SwapFitOutcome
	Pelvic                         SwapFitOutcome
	SeatpostRows                   []HardwareSwapRow
}

// AnalyzeSaddleSwap performs a rigorous biomechanical and physical comparison
// of transferring a fit from source to target saddle.
func AnalyzeSaddleSwap(
	source model.SaddleSpec,
	target model.SaddleSpec,
	saddleHeightMm float64,
	sitBoneSetbackMm float64,
	seatpostSetbackMm float64,
	clampLengthMm float64,
	hbDistanceMm float64,
	hbDropMm float64,
) SaddleSwapReport {
	if saddleHeightMm <= 0 {
		saddleHeightMm = 746.0
	}
	if sitBoneSetbackMm <= 0 {
		sitBoneSetbackMm = 292.0
	}
	if clampLengthMm <= 0 {
		clampLengthMm = model.DefaultSeatpostClampLengthMm
	}
	if hbDistanceMm <= 0 {
		hbDistanceMm = 700.0
	}
	if hbDropMm == 0 {
		hbDropMm = 13.0
	}

	sourceSpan := source.DeltaBrpToSitBonesMm()
	targetSpan := target.DeltaBrpToSitBonesMm()
	spanDelta := targetSpan - sourceSpan
	rcBrpDelta := target.RailCenterToBrpMm - source.RailCenterToBrpMm
	rcSitBoneDelta := target.RailCenterToSitBonesMm - source.RailCenterToSitBonesMm

	// 1. Baseline Coordinates (Source Saddle clamped at dead-center 0.0mm)
	srcBrpSetback := source.BrpSetbackFromSitBones(sitBoneSetbackMm)
	srcBrp := source.BrpPointAtHeightAndSetback(saddleHeightMm, srcBrpSetback)
	srcBrpY := srcBrp.Y
	srcBrpAngle := geom2d.Point2{X: srcBrpSetback, Y: srcBrpY}.PolarAngleDeg()

	// Pelvic Coordinates in Sagittal 2D Plane:
	// Sit position is translated from BRP via source saddle's 2D BRP-to-SitBones vector.
	baseSitBone := source.SitBonePointFromBrp(srcBrp)
	baseLegExt := baseSitBone.DistanceToOrigin()
	baseStaDeg := geom2d.Point2{X: -baseSitBone.X, Y: baseSitBone.Y}.PolarAngleDeg()

	hbDeltaX := math.Sqrt(math.Max(0, hbDistanceMm*hbDistanceMm-hbDropMm*hbDropMm))
	hb := geom2d.Point2{
		X: srcBrp.X + hbDeltaX,
		Y: srcBrp.Y + hbDropMm,
	}
	baseCockpit := hb.DistanceTo(baseSitBone)

	// 2. Target Saddle Equivalent Coordinates (Preserving Exact 2D Pelvic Position):
	// To hold the rider's pelvis invariant at baseSitBone:
	tgtEquivBrp := target.BrpPointFromSitBones(baseSitBone)
	tgtEquivBrpSetback := -tgtEquivBrp.X
	tgtEquivSaddleHeight := tgtEquivBrp.DistanceToOrigin()
	tgtEquivBrpAngle := geom2d.Point2{X: tgtEquivBrpSetback, Y: tgtEquivBrp.Y}.PolarAngleDeg()

	brpSetbackShift := tgtEquivBrpSetback - srcBrpSetback
	brpDelta := tgtEquivBrp.Sub(srcBrp)
	brpDeltaX := brpDelta.X
	brpDeltaY := brpDelta.Y
	brpHeightShift := brpDeltaY
	brpSaddleHeightShift := tgtEquivSaddleHeight - saddleHeightMm
	brpDeltaAngle := tgtEquivBrpAngle - srcBrpAngle
	stepElevationDelta := target.BrpToSitBonesVec().Y - source.BrpToSitBonesVec().Y

	// Clamp window for target saddle
	tMinClamp, tMaxClamp := target.ClampWindowMm(clampLengthMm)

	// 3. Naive BRP Alignment:
	// Naively matches 70mm BRP caliper setback and radial saddle height (target BRP placed at srcBrp):
	naiveSitBone := target.SitBonePointFromBrp(srcBrp)
	naiveBrpSetback := srcBrpSetback
	naiveSitBoneSetback := -naiveSitBone.X
	naiveSitBoneDelta := naiveSitBoneSetback - sitBoneSetbackMm
	naiveSitBoneDeltaY := naiveSitBone.Y - baseSitBone.Y
	naiveLegExt := naiveSitBone.DistanceToOrigin()
	naiveStaDeg := geom2d.Point2{X: naiveSitBoneSetback, Y: naiveSitBone.Y}.PolarAngleDeg()
	naiveCockpit := hb.DistanceTo(naiveSitBone)

	// Required clamp offset to put Target BRP at sourceBrpSetback:
	// Rail center setback on target = naiveBrpSetback + target.RailCenterToBrpMm.
	// Fixed seatpost hardware setback = source.RailCenterSetbackFromBrp(sourceBrpSetback).
	// Required clamp offset = fixedHardware - targetRailCenterSetback = -(target.RailCenterToBrpMm - source.RailCenterToBrpMm)
	// Positive clamp offset pushes saddle rearward, negative pushes saddle forward.
	// Since target BRP is further forward on target saddle (+43.5 vs +23.0), the saddle must move forward relative to clamp,
	// meaning clamp offset = -rcBrpDelta.
	naiveClampOffset := -rcBrpDelta
	naiveFeasible := naiveClampOffset >= tMinClamp-0.001 && naiveClampOffset <= tMaxClamp+0.001
	var naiveMargin float64
	if naiveClampOffset < tMinClamp {
		naiveMargin = naiveClampOffset - tMinClamp
	} else if naiveClampOffset > tMaxClamp {
		naiveMargin = tMaxClamp - naiveClampOffset
	} else {
		naiveMargin = math.Min(naiveClampOffset-tMinClamp, tMaxClamp-naiveClampOffset)
	}

	naiveOutcome := SwapFitOutcome{
		Mode:                    "Naive BRP Alignment",
		TargetBrpSetbackMm:      Round(naiveBrpSetback, 1),
		BrpSetbackShiftMm:       0.0,
		TargetBrpHeightMm:       Round(srcBrp.Y, 1),
		TargetSaddleHeightMm:    Round(saddleHeightMm, 1),
		BrpXMm:                  Round(srcBrp.X, 1),
		BrpYMm:                  Round(srcBrp.Y, 1),
		BrpAngleDeg:             Round(srcBrpAngle, 1),
		AchievedSitBoneMm:       Round(naiveSitBoneSetback, 1),
		AchievedSitBoneHeightMm: Round(naiveSitBone.Y, 1),
		SitBoneDeltaMm:          Round(naiveSitBoneDelta, 1),
		SitBoneDeltaYMm:         Round(naiveSitBoneDeltaY, 1),
		EffectiveStaDeg:         Round(naiveStaDeg, 1),
		CockpitReachMm:          Round(naiveCockpit, 1),
		CockpitDeltaMm:          Round(naiveCockpit-baseCockpit, 1),
		LegExtensionMm:          Round(naiveLegExt, 1),
		LegExtensionDeltaMm:     Round(naiveLegExt-baseLegExt, 1),
		RequiredClampOffsetMm:   Round(naiveClampOffset, 1),
		ClampMinMm:              Round(tMinClamp, 1),
		ClampMaxMm:              Round(tMaxClamp, 1),
		IsFeasible:              naiveFeasible,
		MarginFromLimitMm:       Round(naiveMargin, 1),
	}

	// 4. True Pelvic Alignment:
	// Target Sit Bone Setback and Height are held invariant at (baseSitBone.X, baseSitBone.Y):
	pelvicClampOffset := source.TranslateClampOffset(0.0, target)
	pelvicFeasible := pelvicClampOffset >= tMinClamp-0.001 && pelvicClampOffset <= tMaxClamp+0.001
	var pelvicMargin float64
	if pelvicClampOffset < tMinClamp {
		pelvicMargin = pelvicClampOffset - tMinClamp
	} else if pelvicClampOffset > tMaxClamp {
		pelvicMargin = tMaxClamp - pelvicClampOffset
	} else {
		pelvicMargin = math.Min(pelvicClampOffset-tMinClamp, tMaxClamp-pelvicClampOffset)
	}

	targetIdealSTA := target.IdealSeatTubeAngle(sitBoneSetbackMm, tgtEquivSaddleHeight, seatpostSetbackMm)

	pelvicOutcome := SwapFitOutcome{
		Mode:                    "True Pelvic Alignment",
		TargetBrpSetbackMm:      Round(tgtEquivBrpSetback, 1),
		BrpSetbackShiftMm:       Round(brpSetbackShift, 1),
		TargetBrpHeightMm:       Round(tgtEquivBrp.Y, 1),
		TargetSaddleHeightMm:    Round(tgtEquivSaddleHeight, 1),
		BrpXMm:                  Round(tgtEquivBrp.X, 1),
		BrpYMm:                  Round(tgtEquivBrp.Y, 1),
		BrpAngleDeg:             Round(tgtEquivBrpAngle, 1),
		AchievedSitBoneMm:       Round(sitBoneSetbackMm, 1),
		AchievedSitBoneHeightMm: Round(baseSitBone.Y, 1),
		SitBoneDeltaMm:          0.0,
		SitBoneDeltaYMm:         0.0,
		EffectiveStaDeg:         Round(baseStaDeg, 1),
		CockpitReachMm:          Round(baseCockpit, 1),
		CockpitDeltaMm:          0.0,
		LegExtensionMm:          Round(baseLegExt, 1),
		LegExtensionDeltaMm:     0.0,
		RequiredClampOffsetMm:   Round(pelvicClampOffset, 1),
		ClampMinMm:              Round(tMinClamp, 1),
		ClampMaxMm:              Round(tMaxClamp, 1),
		IsFeasible:              pelvicFeasible,
		MarginFromLimitMm:       Round(pelvicMargin, 1),
	}

	// 4. Hardware Rows Across Seatposts (True Pelvic Fit)
	type postSpec struct {
		name    string
		setback float64
	}
	posts := []postSpec{
		{"0mm Inline Post", 0.0},
		{"Redshift ShockStop (static 7mm)", 7.0},
		{"Redshift ShockStop (sagged 11mm)", 11.0},
		{"Standard 15mm Setback", 15.0},
		{"Standard 25mm Setback", 25.0},
		{"Redshift + Drop Best (static 35mm)", 35.0},
		{"Redshift + Drop Best (sagged 39mm)", 39.0},
	}

	tMin30, tMax30 := target.ClampWindowMm(model.DefaultSeatpostClampLengthMm)
	tMin45, tMax45 := target.ClampWindowMm(model.DropBestClampLengthMm)

	var hwRows []HardwareSwapRow
	for _, p := range posts {
		resSrc := CalculateIdealSeatTubeAngle(source, sitBoneSetbackMm, saddleHeightMm, p.setback, p.name, model.DefaultSeatpostClampLengthMm)
		resTgt := CalculateIdealSeatTubeAngle(target, sitBoneSetbackMm, tgtEquivSaddleHeight, p.setback, p.name, model.DefaultSeatpostClampLengthMm)

		feasible30 := pelvicClampOffset >= tMin30-0.001 && pelvicClampOffset <= tMax30+0.001
		feasible45 := target.RailLengthMm >= model.DropBestClampLengthMm &&
			pelvicClampOffset >= tMin45-0.001 && pelvicClampOffset <= tMax45+0.001

		hwRows = append(hwRows, HardwareSwapRow{
			PostName:           p.name,
			PostSetbackMm:      p.setback,
			SeatTubeSetbackMm:  Round(resTgt.SeatTubeSetbackMm, 1),
			SourceIdealStaDeg:  resSrc.IdealStaDeg,
			TargetIdealStaDeg:  resTgt.IdealStaDeg,
			StaDeltaDeg:        Round(resTgt.IdealStaDeg-resSrc.IdealStaDeg, 2),
			TargetRailOffsetMm: Round(pelvicClampOffset, 1),
			IsFeasible30Mm:     feasible30,
			IsFeasible45Mm:     feasible45,
		})
	}

	return SaddleSwapReport{
		SourceSaddle:                   source,
		TargetSaddle:                   target,
		SaddleHeightMm:                 Round(saddleHeightMm, 1),
		SitBoneSetbackMm:               Round(sitBoneSetbackMm, 1),
		SourceBrpSetbackMm:             Round(srcBrpSetback, 1),
		SourceBrpHeightMm:              Round(srcBrp.Y, 1),
		SourceBrpX:                     Round(srcBrp.X, 1),
		SourceBrpY:                     Round(srcBrp.Y, 1),
		SourceBrpAngleDeg:              Round(srcBrpAngle, 2),
		SourceSitBoneHeightMm:          Round(baseSitBone.Y, 1),
		SourceStepElevationMm:          Round(source.SitBonesElevationMm, 1),
		TargetStepElevationMm:          Round(target.SitBonesElevationMm, 1),
		StepElevationDeltaMm:           Round(stepElevationDelta, 1),
		TargetEquivalentBrpSetbackMm:   Round(tgtEquivBrpSetback, 1),
		TargetEquivalentBrpHeightMm:    Round(tgtEquivBrp.Y, 1),
		TargetEquivalentSaddleHeightMm: Round(tgtEquivSaddleHeight, 1),
		BrpSetbackShiftMm:              Round(brpSetbackShift, 1),
		BrpHeightShiftMm:               Round(brpHeightShift, 1),
		BrpSaddleHeightShiftMm:         Round(brpSaddleHeightShift, 1),
		TargetEquivalentBrpX:           Round(tgtEquivBrp.X, 1),
		TargetEquivalentBrpY:           Round(tgtEquivBrp.Y, 1),
		TargetEquivalentBrpAngleDeg:    Round(tgtEquivBrpAngle, 2),
		BrpDeltaXMm:                    Round(brpDeltaX, 1),
		BrpDeltaYMm:                    Round(brpDeltaY, 1),
		BrpDeltaAngleDeg:               Round(brpDeltaAngle, 2),
		SeatpostSetbackMm:              Round(seatpostSetbackMm, 1),
		ClampLengthMm:                  Round(clampLengthMm, 1),
		HbDistanceMm:                   Round(hbDistanceMm, 1),
		HbDropMm:                       Round(hbDropMm, 1),
		BrpToSitBonesSpanDeltaMm:       Round(spanDelta, 1),
		RailCenterToBrpDeltaMm:         Round(rcBrpDelta, 1),
		RailCenterToSitBonesDeltaMm:    Round(rcSitBoneDelta, 1),
		BaselineCockpitReachMm:         Round(baseCockpit, 1),
		BaselineLegExtensionMm:         Round(baseLegExt, 1),
		BaselineStaDeg:                 Round(targetIdealSTA, 2),
		Naive:                          naiveOutcome,
		Pelvic:                         pelvicOutcome,
		SeatpostRows:                   hwRows,
	}
}
