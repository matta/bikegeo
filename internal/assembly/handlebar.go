package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// HandlebarSetup specifies a handlebar, grip inset, and clamp roll angle.
//
// Roll convention:
//   - RollDeg is measured relative to the leveled bicycle world frame
//     (+X forward horizontal, +Y upward vertical), not the stem extension angle.
//   - Positive RollDeg rolls the top of the bar forward (clockwise in +X/+Y).
type HandlebarSetup struct {
	Spec        model.HandlebarSpec
	GripInsetMm float64
	RollDeg     model.Deg
}

// PlaceHandlebar places a handlebar at barClamp using a world-relative reference
// attitude and returns the sagittal hand-grip position.
// Callers must check the returned error before using the result.
func PlaceHandlebar(barClamp geom2d.Point2, setup HandlebarSetup) (geom2d.Point2, error) {
	handGrip, _, err := placeHandlebar(barClamp, setup)
	return handGrip, err
}

// HandlebarOffset3D returns the 3D offset (ΔX, ΔY, ΔZ) from the bar-clamp center
// to the hand-grip center in the world-relative reference frame.
// Callers must check the returned error before using the result.
func HandlebarOffset3D(setup HandlebarSetup) (model.Point3D, error) {
	handGrip, zHand, err := placeHandlebar(geom2d.Origin(), setup)
	if err != nil {
		return model.Point3D{}, err
	}
	return model.Point3D{X: handGrip.X, Y: handGrip.Y, Z: zHand}, nil
}

// placeHandlebar computes the sagittal hand-grip coordinate from barClamp and the
// lateral grip coordinate zHand in the handlebar's world-relative reference frame:
//   - Effective width is CurrentWidthMm if positive, otherwise StockWidthMm.
//   - Lateral hand center is z_hand = EffectiveWidth/2 - GripInsetMm.
//   - Swept span outboard of the bend start is lz = max(0, z_hand - EffectiveBendStartMm).
//   - Base rise at the bend intersection (y_k) is Spec.BaseRiseAtBend().
//   - Neutral grip coordinates are (-lz*tan(BacksweepDeg), y_k + lz*tan(UpsweepDeg), z_hand).
func placeHandlebar(barClamp geom2d.Point2, setup HandlebarSetup) (handGrip geom2d.Point2, zHand float64, err error) {
	if !isFinitePoint(barClamp) {
		return geom2d.Point2{}, 0, fmt.Errorf("bar clamp position must be finite, got (%v, %v)", barClamp.X, barClamp.Y)
	}
	spec := setup.Spec
	width := spec.EffectiveWidthMm()
	if !isFinite(width) || width <= 0 {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar effective width must be finite and positive, got %v mm", width)
	}
	switch spec.RiseMode {
	case "", model.RiseModeStandardKnuckle, model.RiseModeBarEndCenter:
	default:
		return geom2d.Point2{}, 0, fmt.Errorf("unsupported handlebar rise mode %q", spec.RiseMode)
	}
	if spec.RiseMode == model.RiseModeBarEndCenter && (!isFinite(spec.StockWidthMm) || spec.StockWidthMm <= 0) {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar stock width must be finite and positive for bar_end_center rise mode, got %v mm", spec.StockWidthMm)
	}
	if !isFinite(spec.EffectiveBendStartMm) || spec.EffectiveBendStartMm < 0 {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar bend start must be finite and non-negative, got %v mm", spec.EffectiveBendStartMm)
	}
	if !isFinite(spec.StatedRiseMm) {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar stated rise must be finite, got %v mm", spec.StatedRiseMm)
	}
	if !isFinite(float64(spec.BacksweepDeg)) || spec.BacksweepDeg <= -90 || spec.BacksweepDeg >= 90 {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar backsweep %v° must be in (-90°, 90°)", spec.BacksweepDeg)
	}
	if !isFinite(float64(spec.UpsweepDeg)) || spec.UpsweepDeg <= -90 || spec.UpsweepDeg >= 90 {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar upsweep %v° must be in (-90°, 90°)", spec.UpsweepDeg)
	}
	if !isFinite(setup.GripInsetMm) || setup.GripInsetMm <= 0 || setup.GripInsetMm >= width/2.0 {
		return geom2d.Point2{}, 0, fmt.Errorf("grip inset %v mm must be in (0, %.1f)", setup.GripInsetMm, width/2.0)
	}
	if !isFinite(float64(setup.RollDeg)) {
		return geom2d.Point2{}, 0, fmt.Errorf("bar roll angle must be finite, got %v°", setup.RollDeg)
	}

	zHand = width/2.0 - setup.GripInsetMm
	sweptSpan := math.Max(0, zHand-spec.EffectiveBendStartMm)
	baseRise := spec.BaseRiseAtBend()
	dx0 := -sweptSpan * spec.BacksweepDeg.Angle().Tan()
	dy0 := baseRise + sweptSpan*spec.UpsweepDeg.Angle().Tan()

	// World-relative reference attitude at barClamp; positive RollDeg rolls the
	// top of the bar forward (clockwise = -RollDeg counter-clockwise).
	rolledPose := newPose2(barClamp, 0).turn((-setup.RollDeg).Angle())
	handGrip = rolledPose.point(dx0, dy0)
	if !isFinite(zHand) || !isFinitePoint(handGrip) {
		return geom2d.Point2{}, 0, fmt.Errorf("handlebar placement overflow from bar clamp (%v, %v)", barClamp.X, barClamp.Y)
	}
	return handGrip, zHand, nil
}
