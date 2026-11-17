package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/geom2d"
)

const basisOrthoTol = 1e-9

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// localVec is a 2D displacement vector expressed in a pose2's local basis
// (+X forward perpendicular to axis, +Y along axis).
type localVec struct {
	x, y float64
}

// pose2 is a validated right-handed 2D rigid coordinate frame in the sagittal plane
// (+X forward, +Y up, counter-clockwise positive rotation).
type pose2 struct {
	origin geom2d.Point2
	axisX  geom2d.Vec2
	axisY  geom2d.Vec2
	valid  bool
}

func identityPose2() pose2 {
	return pose2{
		origin: geom2d.Point2{},
		axisX:  geom2d.Vec2{X: 1},
		axisY:  geom2d.Vec2{Y: 1},
		valid:  true,
	}
}

func newPose2(origin geom2d.Point2, xAxisAngle geom2d.Angle) (pose2, error) {
	if !isFinite(origin.X) || !isFinite(origin.Y) {
		return pose2{}, fmt.Errorf("pose origin must be finite, got (%v, %v)", origin.X, origin.Y)
	}
	if !isFinite(xAxisAngle.Rad()) {
		return pose2{}, fmt.Errorf("pose orientation angle must be finite, got %v", xAxisAngle.Deg())
	}
	ux, uy := xAxisAngle.Cos(), xAxisAngle.Sin()
	p := pose2{
		origin: origin,
		axisX:  geom2d.Vec2{X: ux, Y: uy},
		axisY:  geom2d.Vec2{X: -uy, Y: ux},
		valid:  true,
	}
	p.mustBeValid()
	return p, nil
}

// mustBeValid panics if p was zero-initialized, corrupted, or drifted from SO(2).
func (p pose2) mustBeValid() {
	if !p.valid {
		panic("assembly: uninitialized zero-value pose2")
	}
	if !isFinite(p.origin.X) || !isFinite(p.origin.Y) {
		panic(fmt.Sprintf("assembly: non-finite pose2 origin (%v, %v)", p.origin.X, p.origin.Y))
	}
	if !isFinite(p.axisX.X) || !isFinite(p.axisX.Y) || !isFinite(p.axisY.X) || !isFinite(p.axisY.Y) {
		panic("assembly: non-finite pose2 basis")
	}
	if math.Abs(p.axisX.Norm()-1.0) > basisOrthoTol || math.Abs(p.axisY.Norm()-1.0) > basisOrthoTol {
		panic(fmt.Sprintf("assembly: non-unit pose2 basis (|X|=%.12f, |Y|=%.12f)", p.axisX.Norm(), p.axisY.Norm()))
	}
	if math.Abs(p.axisX.Dot(p.axisY)) > basisOrthoTol {
		panic(fmt.Sprintf("assembly: non-orthogonal pose2 basis (dot=%.12f)", p.axisX.Dot(p.axisY)))
	}
	if math.Abs(p.axisX.Cross(p.axisY)-1.0) > basisOrthoTol {
		panic(fmt.Sprintf("assembly: pose2 basis is not right-handed (det=%.12f)", p.axisX.Cross(p.axisY)))
	}
}

func (p pose2) translateLocal(v localVec) (pose2, error) {
	p.mustBeValid()
	if !isFinite(v.x) || !isFinite(v.y) {
		return pose2{}, fmt.Errorf("local translation must be finite, got (%v, %v)", v.x, v.y)
	}
	dx := p.axisX.X*v.x + p.axisY.X*v.y
	dy := p.axisX.Y*v.x + p.axisY.Y*v.y
	nx := p.origin.X + dx
	ny := p.origin.Y + dy
	if !isFinite(dx) || !isFinite(dy) || !isFinite(nx) || !isFinite(ny) {
		return pose2{}, fmt.Errorf("geometric translation overflow from (%v, %v) by (%v, %v)", p.origin.X, p.origin.Y, v.x, v.y)
	}
	return pose2{
		origin: geom2d.Point2{X: nx, Y: ny},
		axisX:  p.axisX,
		axisY:  p.axisY,
		valid:  true,
	}, nil
}

func (p pose2) rotateLocal(a geom2d.Angle) (pose2, error) {
	p.mustBeValid()
	if !isFinite(a.Rad()) {
		return pose2{}, fmt.Errorf("local rotation angle must be finite, got %v", a.Deg())
	}
	c, s := a.Cos(), a.Sin()
	newX := geom2d.Vec2{
		X: p.axisX.X*c + p.axisY.X*s,
		Y: p.axisX.Y*c + p.axisY.Y*s,
	}
	newY := geom2d.Vec2{
		X: -p.axisX.X*s + p.axisY.X*c,
		Y: -p.axisX.Y*s + p.axisY.Y*c,
	}
	out := pose2{
		origin: p.origin,
		axisX:  newX,
		axisY:  newY,
		valid:  true,
	}
	out.mustBeValid()
	return out, nil
}

func (p pose2) transformPoint(v localVec) (geom2d.Point2, error) {
	next, err := p.translateLocal(v)
	if err != nil {
		return geom2d.Point2{}, err
	}
	return next.origin, nil
}
