package pose

import "math"

// DeflectionOverSpan returns the perpendicular displacement produced over an
// orthogonal span of spanMm at angleDeg (spanMm * tan(angleDeg)). Used to
// project out-of-plane angles (such as top-view handlebar backsweep or
// front-view upsweep) onto a sagittal axis.
func DeflectionOverSpan(spanMm float64, angleDeg Deg) float64 {
	return spanMm * angleDeg.tan()
}

// RightTriangleLeg returns the length of the remaining leg of a right triangle
// given hypotenuseMm and otherLegMm. Callers must ensure |otherLegMm| <=
// hypotenuseMm (with hypotenuseMm >= 0); otherwise the result is NaN.
func RightTriangleLeg(hypotenuseMm, otherLegMm float64) float64 {
	return math.Sqrt(hypotenuseMm*hypotenuseMm - otherLegMm*otherLegMm)
}

// Along returns the vector of length lengthMm at sagittal world heading heading.
func Along(heading Deg, lengthMm float64) Vec {
	return Vec{X: lengthMm * heading.cos(), Y: lengthMm * heading.sin()}
}

// ProjectOnto returns the signed length of v's component along the unit vector
// at heading.
func ProjectOnto(v Vec, heading Deg) float64 {
	return v.X*heading.cos() + v.Y*heading.sin()
}

// DecomposeAlong returns the coefficients (a, b) with
// v = a·dir(headingA) + b·dir(headingB), where dir(h) is the unit vector at
// sagittal world heading h. ok is false when the headings are parallel or
// antiparallel (|sin(headingB − headingA)| < 1e-12).
func DecomposeAlong(v Vec, headingA, headingB Deg) (a, b float64, ok bool) {
	dA := Along(headingA, 1)
	dB := Along(headingB, 1)
	den := cross(dA, dB)
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	return cross(v, dB) / den, cross(dA, v) / den, true
}

func cross(v, w Vec) float64 {
	return v.X*w.Y - v.Y*w.X
}
