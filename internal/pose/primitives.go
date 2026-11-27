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

// Along returns the displacement vector in heading's FrameID of Euclidean length
// lengthMm in the direction of heading (pass lengthMm = 1 for a unit direction
// vector; negative lengthMm points opposite heading).
func Along(heading Heading, lengthMm float64) Vec {
	requireValidFrame(heading.frame)
	sin, cos := heading.deg.sincos()
	return Vec{
		x:     lengthMm * cos,
		y:     lengthMm * sin,
		frame: heading.frame,
	}
}

// ProjectOnto returns the signed scalar projection in millimeters of v onto the
// unit-length direction vector at heading in the same FrameID.
func ProjectOnto(v Vec, heading Heading) float64 {
	requireSameFrame(v.frame, heading.frame)
	sin, cos := heading.deg.sincos()
	return v.x*cos + v.y*sin
}

// DecomposeAlong returns the signed coefficients (a, b) in millimeters with
// v = a·dir(headingA) + b·dir(headingB), where dir(h) is the unit-length
// direction vector at heading h in the same FrameID as v. ok is false when
// the directions are parallel or antiparallel (|sin(headingB − headingA)| < 1e-12).
func DecomposeAlong(v Vec, headingA, headingB Heading) (a, b float64, ok bool) {
	requireSameFrame(v.frame, headingA.frame)
	requireSameFrame(v.frame, headingB.frame)
	dA := Along(headingA, 1)
	dB := Along(headingB, 1)
	den := cross(dA, dB)
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	return cross(v, dB) / den, cross(dA, v) / den, true
}

func cross(p, q Vec) float64 {
	requireSameFrame(p.frame, q.frame)
	return p.x*q.y - p.y*q.x
}
