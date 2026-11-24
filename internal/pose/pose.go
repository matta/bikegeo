// Package pose provides 2D points, displacement vectors, turtle poses, and
// degree-valued angles for component assembly.
package pose

import (
	"encoding/json"
	"math"
)

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
// Point-to-point measurements depend only on position, so a landmark's origin
// can be compared directly with another origin or a target.
type Point struct {
	X float64
	Y float64
}

// Sub returns the displacement p − q.
func (p Point) Sub(q Point) Vec {
	return Vec{X: p.X - q.X, Y: p.Y - q.Y}
}

// Add returns p translated by the displacement v.
func (p Point) Add(v Vec) Point {
	return Point{X: p.X + v.X, Y: p.Y + v.Y}
}

// AngleTo returns the world angle in degrees from p to other (0° along +X,
// positive counter-clockwise).
func (p Point) AngleTo(other Point) Deg {
	delta := p.OffsetTo(other)
	return degFromRad(math.Atan2(delta.Y, delta.X))
}

// DistanceTo returns the Euclidean distance in millimeters between p and other.
func (p Point) DistanceTo(other Point) float64 {
	return p.OffsetTo(other).Norm()
}

// VerticalOffsetTo returns the signed vertical offset in millimeters from p to
// other (positive when other is above p along world +Y).
func (p Point) VerticalOffsetTo(other Point) float64 {
	return other.Y - p.Y
}

// OffsetTo returns the world-frame displacement from p to other.
func (p Point) OffsetTo(other Point) Vec {
	return other.Sub(p)
}

// Vec is a 2D displacement in millimeters.
type Vec struct {
	X float64
	Y float64
}

// Add returns v + w.
func (v Vec) Add(w Vec) Vec {
	return Vec{X: v.X + w.X, Y: v.Y + w.Y}
}

// Sub returns v − w.
func (v Vec) Sub(w Vec) Vec {
	return Vec{X: v.X - w.X, Y: v.Y - w.Y}
}

// Scale returns v multiplied by factor.
func (v Vec) Scale(factor float64) Vec {
	return Vec{X: v.X * factor, Y: v.Y * factor}
}

// Neg returns −v.
func (v Vec) Neg() Vec {
	return Vec{X: -v.X, Y: -v.Y}
}

// Norm returns the Euclidean length of v in millimeters.
func (v Vec) Norm() float64 {
	return math.Hypot(v.X, v.Y)
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

// MarshalJSON serializes a Pose's unexported Origin and HeadingDeg for
// encoding/json CLI output (Q49a, Q88).
func (p Pose) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Origin     Point
		HeadingDeg Deg
	}{
		Origin:     p.origin,
		HeadingDeg: p.headingDeg,
	})
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
