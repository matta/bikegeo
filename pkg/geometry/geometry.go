package geometry

import (
	"fmt"
	"math"

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
