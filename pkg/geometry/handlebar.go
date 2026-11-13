package geometry

import (
	"math"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
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
func GetHandlebarOffset(spec model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) model.Point3D {
	if gripInsetMm <= 0 {
		gripInsetMm = model.DefaultGripInsetMm
	}

	beta := spec.BacksweepDeg.Angle()
	alpha := spec.UpsweepDeg.Angle()
	roll := rollDeg.Angle()

	width := spec.EffectiveWidthMm()
	zHand := width/2.0 - gripInsetMm
	zb := spec.EffectiveBendStartMm
	lz := math.Max(0, zHand-zb)

	// Base rise at the bend intersection (y_k)
	yk := spec.BaseRiseAtBend()

	// Neutral plane coordinates (roll = 0)
	dx0 := -lz * beta.Tan()
	dy0 := yk + lz*alpha.Tan()
	dz0 := zHand

	// Apply bar roll rotation in the X-Y plane around the lateral Z axis:
	// Positive rollDeg rolls the top of the bar forward (increasing reach, reducing stack).
	dx := dx0*roll.Cos() + dy0*roll.Sin()
	dy := -dx0*roll.Sin() + dy0*roll.Cos()

	return model.Point3D{X: dx, Y: dy, Z: dz0}
}

// HBToGrip performs the forward transform:
// Stem Clamp Center (P_HB) -> Hand Grip Center (P_Grip).
func HBToGrip(hb model.Point3D, spec model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) model.Point3D {
	offset := GetHandlebarOffset(spec, gripInsetMm, rollDeg)
	return model.Point3D{
		X: hb.X + offset.X,
		Y: hb.Y + offset.Y,
		Z: offset.Z,
	}
}

// GripToHB performs the inverse transform:
// Target Hand Grip Center (P_Grip) -> Required Stem Clamp Center (P_HB).
func GripToHB(grip model.Point3D, spec model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) model.Point3D {
	offset := GetHandlebarOffset(spec, gripInsetMm, rollDeg)
	return model.Point3D{
		X: grip.X - offset.X,
		Y: grip.Y - offset.Y,
		Z: 0.0,
	}
}

// HandlebarDelta computes the hand grip coordinate shift when switching from specFrom to specTo
// with the stem clamp held fixed at (0, 0, 0).
// Positive X means grips move forward; positive Y means grips move upward.
func HandlebarDelta(specFrom, specTo model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) model.Point3D {
	offFrom := GetHandlebarOffset(specFrom, gripInsetMm, rollDeg)
	offTo := GetHandlebarOffset(specTo, gripInsetMm, rollDeg)
	return offTo.Sub(offFrom)
}

// HandlebarStemCompensation computes the stem clamp displacement (delta Reach, delta Stack)
// required to keep the rider's hands at the exact same location in space when switching
// from specFrom to specTo.
func HandlebarStemCompensation(specFrom, specTo model.HandlebarSpec, gripInsetMm float64, rollDeg model.Deg) model.Point3D {
	offFrom := GetHandlebarOffset(specFrom, gripInsetMm, rollDeg)
	offTo := GetHandlebarOffset(specTo, gripInsetMm, rollDeg)
	// Target clamp must shift by (offFrom - offTo)
	return offFrom.Sub(offTo)
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
) model.HBPoint {
	headTubeTop := geom2d.Point2{X: reach, Y: stack}
	steererAxis := SteererTubeAxis(headTubeTop, htaDeg)

	dSteerer := spacerMm + model.StemThicknessMm/2.0
	stemClamp := steererAxis.PointAt(dSteerer)

	// Stem angle is rotated 90° clockwise from upward steerer axis, then rotated AngleDeg:
	stemDir := steererAxis.Normal(geom2d.Clockwise).Rotate(stem.AngleDeg.Angle())
	clampPoint := stemClamp.Add(stemDir.Scale(stem.LengthMm))

	offset := GetHandlebarOffset(bar, gripInsetMm, rollDeg)
	hbPoint := geom2d.Point2{
		X: clampPoint.X + offset.X,
		Y: clampPoint.Y + offset.Y,
	}

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
	}
}
