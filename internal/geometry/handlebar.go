package geometry

import (
	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// GetHandlebarOffset computes the 3D offset vector (ΔX, ΔY, ΔZ) from HB (stem clamp center)
// to Grip (hand center).
//
// Parameters:
//   - spec: Handlebar geometric specification
//   - gripInsetMm: Distance from bar end to hand center along lateral axis (default 65mm if <= 0)
//   - rollDeg: Bar roll angle in clamp in degrees (+ rolls top of bar forward, default 0)
//
// Coordinate Frame:
//   - X: Reach / Longitudinal axis (+ forward, horizontal)
//   - Y: Stack / Vertical axis (+ upward, vertical)
//   - Z: Lateral axis (+ right of centerline, symmetric for left)
func GetHandlebarOffset(spec model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) (model.Point3D, error) {
	if gripInsetMm <= 0 {
		gripInsetMm = model.DefaultGripInsetMm
	}
	bar, err := assembly.NewHandlebar(spec, gripInsetMm)
	if err != nil {
		return model.Point3D{}, err
	}
	return bar.GripOffset3D(rollDeg)
}

// HandlebarDelta computes the hand grip coordinate shift when switching from specFrom to specTo
// with the stem clamp held fixed at (0, 0, 0).
// Positive X means grips move forward; positive Y means grips move upward.
func HandlebarDelta(specFrom, specTo model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) (model.Point3D, error) {
	offFrom, err := GetHandlebarOffset(specFrom, gripInsetMm, rollDeg)
	if err != nil {
		return model.Point3D{}, err
	}
	offTo, err := GetHandlebarOffset(specTo, gripInsetMm, rollDeg)
	if err != nil {
		return model.Point3D{}, err
	}
	return offTo.Sub(offFrom), nil
}

// HandlebarStemCompensation computes the stem clamp displacement (delta Reach, delta Stack)
// required to keep the rider's hands at the exact same location in space when switching
// from specFrom to specTo.
func HandlebarStemCompensation(specFrom, specTo model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) (model.Point3D, error) {
	offFrom, err := GetHandlebarOffset(specFrom, gripInsetMm, rollDeg)
	if err != nil {
		return model.Point3D{}, err
	}
	offTo, err := GetHandlebarOffset(specTo, gripInsetMm, rollDeg)
	if err != nil {
		return model.Point3D{}, err
	}
	// Target clamp must shift by (offFrom - offTo)
	return offFrom.Sub(offTo), nil
}

// CalculateHBPointWithHandlebar computes the HB reference point, hand grip position,
// and Saddle-Handlebar distance/drop using a specified handlebar, grip inset, and roll angle.
func CalculateHBPointWithHandlebar(
	reach, stack float64, htaDeg model.Deg,
	brp model.BRPPoint,
	stem model.StemSpec,
	spacerMm float64,
	bar model.HandlebarSpec,
	gripInsetMm float64, rollDeg model.Deg,
) (model.HBPoint, error) {
	headTubeTop := geom2d.Point2{X: reach, Y: stack}
	cockpitPts, err := placeFrontCockpit(headTubeTop, htaDeg, CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          spacerMm,
		Stem:              stem,
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         bar,
		BarRollDeg:        rollDeg,
		GripInsetMm:       gripInsetMm,
	})
	if err != nil {
		return model.HBPoint{}, err
	}

	clampPoint := cockpitPts.StemClamp
	hbPoint := cockpitPts.HandGrip
	brpPoint := brp.BrpPoint()
	sitBonePoint := brp.SitBonePoint()
	dist := hbPoint.DistanceTo(brpPoint)
	drop := hbPoint.Y - sitBonePoint.Y

	return model.HBPoint{
		Stem:           stem,
		SpacerMm:       spacerMm,
		ClampX:         clampPoint.X,
		ClampY:         clampPoint.Y,
		HbX:            hbPoint.X,
		HbY:            hbPoint.Y,
		SaddleHbMm:     dist,
		SaddleHbDropMm: drop,
	}, nil
}
