package assembly

import (
	"math"

	"bikegeo/internal/geom2d"
)

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isFinitePoint(p geom2d.Point2) bool {
	return isFinite(p.X) && isFinite(p.Y)
}

// pose2 is a 2D right-handed rigid coordinate frame in the sagittal plane
// (+X forward, +Y up, counter-clockwise positive rotation).
type pose2 struct {
	origin geom2d.Point2
	axisX  geom2d.Vec2
}

func identityPose2() pose2 {
	return pose2{
		origin: geom2d.Origin(),
		axisX:  geom2d.Vec2{X: 1},
	}
}

// newPose2 creates a right-handed 2D pose at origin whose local +X axis points
// along heading (counter-clockwise from world +X).
func newPose2(origin geom2d.Point2, heading geom2d.Angle) pose2 {
	return pose2{
		origin: origin,
		axisX:  geom2d.UnitFromPolar(heading),
	}
}

func (p pose2) axisY() geom2d.Vec2 {
	return p.axisX.Perpendicular(geom2d.CounterClockwise)
}

func (p pose2) advance(dist float64) pose2 {
	return p.translate(dist, 0)
}

func (p pose2) translate(dx, dy float64) pose2 {
	return pose2{
		origin: p.origin.Add(p.axisX.Scale(dx)).Add(p.axisY().Scale(dy)),
		axisX:  p.axisX,
	}
}

func (p pose2) turn(angle geom2d.Angle) pose2 {
	return pose2{
		origin: p.origin,
		axisX:  p.axisX.Rotate(angle),
	}
}

func (p pose2) point(dx, dy float64) geom2d.Point2 {
	return p.translate(dx, dy).origin
}
