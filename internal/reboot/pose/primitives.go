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
