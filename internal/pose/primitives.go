package pose

import "math"

// DeflectionOverSpan returns the perpendicular displacement produced over an
// orthogonal span of spanMm at angleDeg (spanMm * tan(angleDeg)). Used to
// project out-of-plane angles (such as top-view handlebar backsweep or
// front-view upsweep) onto a local component plane.
func DeflectionOverSpan(spanMm float64, angleDeg Deg) float64 {
	return spanMm * angleDeg.tan()
}

// RightTriangleLeg returns the length of the remaining leg of a right triangle
// given hypotenuseMm and otherLegMm. Callers must ensure |otherLegMm| <=
// hypotenuseMm (with hypotenuseMm >= 0); otherwise the result is NaN.
func RightTriangleLeg(hypotenuseMm, otherLegMm float64) float64 {
	return math.Sqrt(hypotenuseMm*hypotenuseMm - otherLegMm*otherLegMm)
}

// Along returns the displacement vector in heading's CoordinateSystem of
// Euclidean length lengthMm in the direction of heading (pass lengthMm = 1 for
// a unit direction vector; negative lengthMm points opposite heading).
func Along(heading Heading, lengthMm float64) Vector {
	requireValidSystem(heading.system)
	sin, cos := heading.deg.sincos()
	return Vector{
		x:      lengthMm * cos,
		y:      lengthMm * sin,
		system: heading.system,
	}
}

// ProjectOnto returns the signed scalar projection in millimeters of v onto the
// unit-length direction vector at heading in the same CoordinateSystem.
func ProjectOnto(v Vector, heading Heading) float64 {
	requireSameSystem(v.system, heading.system)
	sin, cos := heading.deg.sincos()
	return v.x*cos + v.y*sin
}

// DecomposeAlong returns the signed coefficients (a, b) in millimeters with
// v = a·dir(headingA) + b·dir(headingB), where dir(h) is the unit-length
// direction vector at heading h in the same CoordinateSystem as v. ok is false
// when the directions are parallel or antiparallel
// (|sin(headingB − headingA)| < 1e-12).
func DecomposeAlong(v Vector, headingA, headingB Heading) (a, b float64, ok bool) {
	requireSameSystem(v.system, headingA.system)
	requireSameSystem(v.system, headingB.system)
	dA := Along(headingA, 1)
	dB := Along(headingB, 1)
	den := cross(dA, dB)
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	return cross(v, dB) / den, cross(dA, v) / den, true
}

func cross(p, q Vector) float64 {
	requireSameSystem(p.system, q.system)
	return p.x*q.y - p.y*q.x
}
