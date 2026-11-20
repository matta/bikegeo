// Package pose provides 2D planar poses and degree-valued angles for reboot
// component assembly.
package pose

import "math"

const radPerDeg = math.Pi / 180.0

// Deg represents an angle in degrees with no implied coordinate system or plane.
// Coordinate and sign conventions belong to the Pose or component field using it.
type Deg float64

func degFromRad(rad float64) Deg {
	return Deg(rad / radPerDeg)
}

func (d Deg) rad() float64 {
	return float64(d) * radPerDeg
}

func (d Deg) sin() float64 {
	return math.Sin(d.rad())
}

func (d Deg) cos() float64 {
	return math.Cos(d.rad())
}

func (d Deg) tan() float64 {
	return math.Tan(d.rad())
}

// Point is a 2D position in millimeters in the sagittal plane (+X forward, +Y up).
type Point struct {
	X float64
	Y float64
}

// Sub returns p − q.
func (p Point) Sub(q Point) Point {
	return Point{X: p.X - q.X, Y: p.Y - q.Y}
}

// Norm returns the Euclidean length of p as a vector.
func (p Point) Norm() float64 {
	return math.Hypot(p.X, p.Y)
}

// Pose is an immutable 2D sagittal position and heading.
// Headings are measured in degrees in the sagittal world frame (0° along +X forward,
// 90° along +Y up; positive angles rotate counter-clockwise).
type Pose struct {
	origin     Point
	headingDeg Deg
}

// NewPose creates a Pose at origin with sagittal world headingDeg (0° along +X,
// 90° along +Y, positive counter-clockwise).
func NewPose(origin Point, headingDeg Deg) Pose {
	return Pose{origin: origin, headingDeg: headingDeg}
}

// Origin returns the Pose's world position in millimeters.
func (p Pose) Origin() Point {
	return p.origin
}

// HeadingDeg returns the Pose's sagittal world heading in degrees (0° along +X,
// 90° along +Y, positive counter-clockwise).
func (p Pose) HeadingDeg() Deg {
	return p.headingDeg
}

// Advance returns a new Pose moved distMm along the current heading.
func (p Pose) Advance(distMm float64) Pose {
	return Pose{
		origin: Point{
			X: p.origin.X + distMm*p.headingDeg.cos(),
			Y: p.origin.Y + distMm*p.headingDeg.sin(),
		},
		headingDeg: p.headingDeg,
	}
}

// Turn returns a new Pose at the same origin with heading rotated by deltaDeg
// (positive angles rotate counter-clockwise in the sagittal 2D plane).
func (p Pose) Turn(deltaDeg Deg) Pose {
	return Pose{origin: p.origin, headingDeg: p.headingDeg + deltaDeg}
}

// Face returns a new Pose at the same origin with sagittal world heading set
// directly to worldHeadingDeg (0° along +X, 90° along +Y, positive counter-clockwise).
func (p Pose) Face(worldHeadingDeg Deg) Pose {
	return Pose{origin: p.origin, headingDeg: worldHeadingDeg}
}

// Translate returns a new Pose moved by (localX, localY) in the current
// pose's local coordinate frame (+X along heading, +Y 90° counter-clockwise).
func (p Pose) Translate(localX, localY float64) Pose {
	cos := p.headingDeg.cos()
	sin := p.headingDeg.sin()
	return Pose{
		origin: Point{
			X: p.origin.X + localX*cos - localY*sin,
			Y: p.origin.Y + localX*sin + localY*cos,
		},
		headingDeg: p.headingDeg,
	}
}

// AngleTo returns the sagittal world angle in degrees (0° along +X, positive
// counter-clockwise) from p's origin to other's origin.
func (p Pose) AngleTo(other Pose) Deg {
	dx := other.origin.X - p.origin.X
	dy := other.origin.Y - p.origin.Y
	return degFromRad(math.Atan2(dy, dx))
}

// DistanceTo returns the Euclidean distance in millimeters between p and other.
func (p Pose) DistanceTo(other Pose) float64 {
	return math.Hypot(other.origin.X-p.origin.X, other.origin.Y-p.origin.Y)
}

// VerticalOffsetTo returns the signed vertical offset in millimeters from p to other
// (positive when other is above p along world +Y).
func (p Pose) VerticalOffsetTo(other Pose) float64 {
	return other.origin.Y - p.origin.Y
}

// OffsetTo returns other.Origin() − p.Origin() as a world-frame vector.
func (p Pose) OffsetTo(other Pose) Point {
	return other.origin.Sub(p.origin)
}
