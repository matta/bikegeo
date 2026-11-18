package geometry

import (
	"fmt"
	"math"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// SwapFitOutcome encapsulates the quantitative fit metrics resulting from either
// matching BRP coordinates or preserving the modeled support point.
type SwapFitOutcome struct {
	TargetBrpSetbackMm      float64   // Caliper setback to 70mm BRP
	BrpSetbackShiftMm       float64   // Shift in BRP setback relative to baseline source BRP
	TargetBrpHeightMm       float64   // Vertical height of BRP above BB spindle
	TargetSaddleHeightMm    float64   // Radial distance from BB spindle to 70mm BRP
	BrpXMm                  float64   // Horizontal X coordinate of BRP from BB spindle
	BrpYMm                  float64   // Vertical Y coordinate of BRP from BB spindle
	BrpAngleDeg             model.Deg // Polar angle from BB spindle to BRP
	AchievedSitBoneMm       float64   // Modeled horizontal SIT setback behind BB
	AchievedSitBoneHeightMm float64   // Modeled vertical SIT height above BB
	SitBoneDeltaMm          float64   // Difference from baseline sit bone setback (Achieved - Desired)
	SitBoneDeltaYMm         float64   // Difference from baseline sit bone height (Achieved - Desired)
	EffectiveStaDeg         model.Deg // Polar seat tube angle from BB to sit bone support point
	CockpitReachMm          float64   // Direct 2D Euclidean distance from sit bones to handlebar clamp
	CockpitDeltaMm          float64   // Change in cockpit reach from baseline
	BBToSitDistanceMm       float64   // Radial Euclidean distance from BB spindle to sit bones
	BBToSitDistanceDeltaMm  float64   // Change in BB-to-SIT distance from baseline
	RequiredClampOffsetMm   float64   // Required fore-aft rail clamp offset
	IsFeasible              bool      // Whether the assumed clamp fits at the required offset
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
	SourceBrpAngleDeg              model.Deg
	SourceSitBoneHeightMm          float64
	SourceStepElevationMm          float64
	TargetStepElevationMm          float64
	StepElevationDeltaMm           float64 // Target step - Source step
	TargetEquivalentBrpSetbackMm   float64
	TargetEquivalentBrpHeightMm    float64
	TargetEquivalentSaddleHeightMm float64
	BrpSetbackShiftMm              float64 // Target equivalent BRP setback - Source BRP setback
	BrpHeightShiftMm               float64 // Target equivalent BRP height - Source BRP height
	BrpSaddleHeightShiftMm         float64 // Target equivalent SH - Source SH
	TargetEquivalentBrpX           float64
	TargetEquivalentBrpY           float64
	TargetEquivalentBrpAngleDeg    model.Deg
	BrpDeltaXMm                    float64   // Target X - Source X
	BrpDeltaYMm                    float64   // Target Y - Source Y
	BrpDeltaAngleDeg               model.Deg // Target angle - Source angle
	SeatpostSetbackMm              float64
	ClampLengthMm                  float64
	HbDistanceMm                   float64
	HbDropMm                       float64
	BrpToSitBonesSpanDeltaMm       float64 // Target span - Source span (BRP skew)
	RailCenterToBrpDeltaMm         float64 // Target - Source
	RailCenterToSitBonesDeltaMm    float64 // Target - Source (local rail coordinates)
	CenteredStaDeltaDeg            model.Deg
	TargetBrpToCockpitMm           float64
	BaselineCockpitReachMm         float64
	BaselineBBToSitDistanceMm      float64
	Naive                          SwapFitOutcome
	PreserveSIT                    SwapFitOutcome
}

// AnalyzeSaddleSwap compares two ways to transfer a modeled saddle position.
func AnalyzeSaddleSwap(
	source model.SaddleSpec,
	target model.SaddleSpec,
	saddleHeightMm float64,
	sitBoneSetbackMm float64,
	seatpostSetbackMm float64,
	clampLengthMm float64,
	hbDistanceMm float64,
	hbDropMm float64,
) (SaddleSwapReport, error) {
	if !isPositive(saddleHeightMm) {
		return SaddleSwapReport{}, fmt.Errorf("saddleHeightMm must be positive, got %v", saddleHeightMm)
	}
	if !isPositive(sitBoneSetbackMm) {
		return SaddleSwapReport{}, fmt.Errorf("sitBoneSetbackMm must be positive, got %v", sitBoneSetbackMm)
	}
	if !isPositive(clampLengthMm) {
		return SaddleSwapReport{}, fmt.Errorf("clampLengthMm must be positive, got %v", clampLengthMm)
	}
	if !isPositive(hbDistanceMm) {
		return SaddleSwapReport{}, fmt.Errorf("hbDistanceMm must be positive, got %v", hbDistanceMm)
	}

	if !isNonNegative(seatpostSetbackMm) || !isFinite(hbDropMm) || math.Abs(hbDropMm) > hbDistanceMm {
		return SaddleSwapReport{}, fmt.Errorf("invalid seatpost setback or cockpit distance/rise")
	}
	sourceSpan := source.DeltaBrpToSitBonesMm()
	targetSpan := target.DeltaBrpToSitBonesMm()
	spanDelta := targetSpan - sourceSpan
	rcBrpDelta := target.RailCenterToBrpMm - source.RailCenterToBrpMm
	rcSitBoneDelta := target.RailCenterToSitBonesMm - source.RailCenterToSitBonesMm

	// 1. Baseline Coordinates (Source Saddle clamped at dead-center 0.0mm)
	srcBrpSetback := source.BrpSetbackFromSitBones(sitBoneSetbackMm)
	sourceSol, err := SolveSaddleSTAForBRPTarget(source, saddleHeightMm, srcBrpSetback, seatpostSetbackMm, 0)
	if err != nil {
		return SaddleSwapReport{}, fmt.Errorf("source saddle centered STA: %w", err)
	}
	sourceSTA := sourceSol.EffStaDeg
	reference := BRPPointFromSaddlePlacement(sourceSol.Placement)
	srcBrp := sourceSol.Placement.BRP
	translation, err := TranslateSaddleFit(reference, target, sourceSTA, clampLengthMm)
	if err != nil {
		return SaddleSwapReport{}, err
	}
	srcBrpY := srcBrp.Y
	srcBrpAngle := model.DegOf(geom2d.Point2{X: srcBrpSetback, Y: srcBrpY}.PolarAngle())

	// Modeled support point in the sagittal plane.
	// Sit position is translated from BRP via source saddle's 2D BRP-to-SitBones vector.
	baseSitBone := source.SitBonePointFromBrp(srcBrp)
	baseBBToSit := baseSitBone.DistanceToOrigin()
	baseStaDeg := model.DegOf(geom2d.Point2{X: -baseSitBone.X, Y: baseSitBone.Y}.PolarAngle())

	hbDeltaX := math.Sqrt(hbDistanceMm*hbDistanceMm - hbDropMm*hbDropMm)
	hb := geom2d.Point2{
		X: srcBrp.X + hbDeltaX,
		Y: srcBrp.Y + hbDropMm,
	}
	baseCockpit := hb.DistanceTo(baseSitBone)

	// 2. Translate the target BRP to preserve the modeled SIT point.
	tgtEquivBrp := target.BrpPointFromSitBones(baseSitBone)
	tgtEquivBrpSetback := -tgtEquivBrp.X
	tgtEquivSaddleHeight := tgtEquivBrp.DistanceToOrigin()
	tgtEquivBrpAngle := model.DegOf(geom2d.Point2{X: tgtEquivBrpSetback, Y: tgtEquivBrp.Y}.PolarAngle())

	brpSetbackShift := tgtEquivBrpSetback - srcBrpSetback
	brpDelta := tgtEquivBrp.Sub(srcBrp)
	brpDeltaX := brpDelta.X
	brpDeltaY := brpDelta.Y
	brpHeightShift := brpDeltaY
	brpSaddleHeightShift := tgtEquivSaddleHeight - saddleHeightMm
	brpDeltaAngle := tgtEquivBrpAngle - srcBrpAngle
	stepElevationDelta := target.BrpToSitBonesVec().Y - source.BrpToSitBonesVec().Y

	// 3. Naive BRP Alignment:
	// Naively matches 70mm BRP caliper setback and radial saddle height (target BRP placed at srcBrp):
	naiveSitBone := target.SitBonePointFromBrp(srcBrp)
	naiveBrpSetback := srcBrpSetback
	naiveSitBoneSetback := -naiveSitBone.X
	naiveSitBoneDelta := naiveSitBoneSetback - sitBoneSetbackMm
	naiveSitBoneDeltaY := naiveSitBone.Y - baseSitBone.Y
	naiveBBToSit := naiveSitBone.DistanceToOrigin()
	naiveStaDeg := model.DegOf(geom2d.Point2{X: naiveSitBoneSetback, Y: naiveSitBone.Y}.PolarAngle())
	naiveCockpit := hb.DistanceTo(naiveSitBone)

	// Keep the source frame axis and post; solve the target's full placement.
	naiveSol, err := SolveSaddleForBRPTarget(target, sourceSTA, saddleHeightMm, srcBrpSetback, seatpostSetbackMm)
	if err != nil {
		return SaddleSwapReport{}, fmt.Errorf("naive BRP alignment: %w", err)
	}
	naiveClampOffset := naiveSol.Placement.RailOffsetMm
	naiveFeasible := target.ClampFitsAt(clampLengthMm, naiveClampOffset)

	naiveOutcome := SwapFitOutcome{
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
		BBToSitDistanceMm:       Round(naiveBBToSit, 1),
		BBToSitDistanceDeltaMm:  Round(naiveBBToSit-baseBBToSit, 1),
		RequiredClampOffsetMm:   naiveClampOffset,
		IsFeasible:              naiveFeasible,
	}

	// 4. Preserve SIT coordinates.
	// Target Sit Bone Setback and Height are held invariant at (baseSitBone.X, baseSitBone.Y):
	supportClampOffset := translation.TargetClampOffset
	supportFeasible := translation.IsFeasible

	supportOutcome := SwapFitOutcome{
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
		BBToSitDistanceMm:       Round(baseBBToSit, 1),
		BBToSitDistanceDeltaMm:  0.0,
		RequiredClampOffsetMm:   supportClampOffset,
		IsFeasible:              supportFeasible,
	}

	targetCenteredSol, err := SolveSaddleSTAForBRPTarget(target, tgtEquivSaddleHeight, tgtEquivBrpSetback, seatpostSetbackMm, 0)
	if err != nil {
		return SaddleSwapReport{}, fmt.Errorf("target saddle centered STA: %w", err)
	}
	centeredStaDeltaDeg := targetCenteredSol.EffStaDeg - sourceSTA

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
		CenteredStaDeltaDeg:            centeredStaDeltaDeg,
		TargetBrpToCockpitMm:           hb.DistanceTo(tgtEquivBrp),
		BaselineCockpitReachMm:         Round(baseCockpit, 1),
		BaselineBBToSitDistanceMm:      Round(baseBBToSit, 1),
		Naive:                          naiveOutcome,
		PreserveSIT:                    supportOutcome,
	}, nil
}
