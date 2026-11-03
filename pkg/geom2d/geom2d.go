package geom2d

import (
	"fmt"
	"math"
)

// Angle represents an angular quantity stored in radians.
type Angle float64

// FromRad creates an Angle from radians.
func FromRad(rad float64) Angle {
	return Angle(rad)
}

// FromDeg creates an Angle from degrees.
func FromDeg(deg float64) Angle {
	return Angle(deg * math.Pi / 180.0)
}

// Deg returns the angle in degrees.
func (a Angle) Deg() float64 {
	return float64(a) * 180.0 / math.Pi
}

// Rad returns the angle in radians.
func (a Angle) Rad() float64 {
	return float64(a)
}

// Cos returns the cosine of the angle.
func (a Angle) Cos() float64 {
	return math.Cos(float64(a))
}

// Sin returns the sine of the angle.
func (a Angle) Sin() float64 {
	return math.Sin(float64(a))
}

// Tan returns the tangent of the angle.
func (a Angle) Tan() float64 {
	return math.Tan(float64(a))
}

// Add adds two angles.
func (a Angle) Add(b Angle) Angle {
	return Angle(float64(a) + float64(b))
}

// Sub subtracts angle b from a.
func (a Angle) Sub(b Angle) Angle {
	return Angle(float64(a) - float64(b))
}

// Neg returns the negation of the angle.
func (a Angle) Neg() Angle {
	return Angle(-float64(a))
}

// AngleFromSlope returns the angle of a 2D displacement (dy/dx).
func AngleFromSlope(dx, dy float64) Angle {
	return Angle(math.Atan2(dy, dx))
}

// AngleFromLift returns the rotation angle of a lever of length 'arm' that causes vertical lift 'dy'.
// sin(theta) = dy / arm. Clamps dy/arm to [-1, 1] for numerical safety.
func AngleFromLift(arm, dy float64) (Angle, error) {
	if arm <= 0 {
		return 0, fmt.Errorf("lever arm must be positive, got %.1f", arm)
	}
	ratio := math.Max(-1.0, math.Min(1.0, dy/arm))
	return Angle(math.Asin(ratio)), nil
}

// SolveTriangleSSS solves for the angle opposite to side c in a triangle with sides a, b, c
// using the Law of Cosines: c^2 = a^2 + b^2 - 2ab*cos(C) => cos(C) = (a^2 + b^2 - c^2) / (2ab).
func SolveTriangleSSS(a, b, c float64) (Angle, error) {
	if a <= 0 || b <= 0 {
		return 0, fmt.Errorf("triangle sides a and b must be positive, got a=%.1f b=%.1f", a, b)
	}
	cosC := (a*a + b*b - c*c) / (2.0 * a * b)
	cosC = math.Max(-1.0, math.Min(1.0, cosC))
	return Angle(math.Acos(cosC)), nil
}

// UnitFromPolar returns a normalized unit vector directed along polar angle (counter-clockwise from +X).
func UnitFromPolar(angle Angle) Vec2 {
	return FromPolar(1.0, angle)
}

// TurnDirection specifies a rotational direction in 2D.
type TurnDirection int

const (
	CounterClockwise TurnDirection = 1
	Clockwise        TurnDirection = -1
)

// Vec2 represents a 2D vector / displacement.
type Vec2 struct {
	X, Y float64
}

// Origin is the (0, 0) point.
var Origin = Point2{X: 0, Y: 0}

// UnitX is a unit vector pointing along the positive X-axis (forward).
var UnitX = Vec2{X: 1, Y: 0}

// UnitY is a unit vector pointing along the positive Y-axis (upward).
var UnitY = Vec2{X: 0, Y: 1}

// FromPolar creates a Vec2 from polar coordinates (length r and angle theta).
func FromPolar(r float64, theta Angle) Vec2 {
	return Vec2{
		X: r * theta.Cos(),
		Y: r * theta.Sin(),
	}
}

// Add returns v + w.
func (v Vec2) Add(w Vec2) Vec2 {
	return Vec2{X: v.X + w.X, Y: v.Y + w.Y}
}

// Sub returns v - w.
func (v Vec2) Sub(w Vec2) Vec2 {
	return Vec2{X: v.X - w.X, Y: v.Y - w.Y}
}

// Scale returns v multiplied by scalar s.
func (v Vec2) Scale(s float64) Vec2 {
	return Vec2{X: v.X * s, Y: v.Y * s}
}

// Dot returns the dot product of v and w.
func (v Vec2) Dot(w Vec2) float64 {
	return v.X*w.X + v.Y*w.Y
}

// Cross returns the 2D scalar cross product (v.X*w.Y - v.Y*w.X).
func (v Vec2) Cross(w Vec2) float64 {
	return v.X*w.Y - v.Y*w.X
}

// Norm returns the Euclidean length of v.
func (v Vec2) Norm() float64 {
	return math.Hypot(v.X, v.Y)
}

// NormSq returns the squared length of v.
func (v Vec2) NormSq() float64 {
	return v.X*v.X + v.Y*v.Y
}

// Unit returns the normalized unit vector in direction of v.
// Returns zero vector if norm is 0.
func (v Vec2) Unit() Vec2 {
	n := v.Norm()
	if n == 0 {
		return Vec2{}
	}
	return Vec2{X: v.X / n, Y: v.Y / n}
}

// Rotate rotates the vector by angle a (counter-clockwise if positive).
func (v Vec2) Rotate(a Angle) Vec2 {
	c := a.Cos()
	s := a.Sin()
	return Vec2{
		X: v.X*c - v.Y*s,
		Y: v.X*s + v.Y*c,
	}
}

// RotateDeg rotates the vector by deg degrees.
func (v Vec2) RotateDeg(deg float64) Vec2 {
	return v.Rotate(FromDeg(deg))
}

// Perpendicular returns a vector rotated 90 degrees in the given TurnDirection.
func (v Vec2) Perpendicular(turn TurnDirection) Vec2 {
	if turn == Clockwise {
		// (x, y) -> (y, -x)
		return Vec2{X: v.Y, Y: -v.X}
	}
	// CounterClockwise: (x, y) -> (-y, x)
	return Vec2{X: -v.Y, Y: v.X}
}

// Angle returns the polar angle of v from the +X axis.
func (v Vec2) Angle() Angle {
	return Angle(math.Atan2(v.Y, v.X))
}

// Point2 represents a 2D position in Cartesian space.
type Point2 struct {
	X, Y float64
}

// Add returns p translated by displacement vector v.
func (p Point2) Add(v Vec2) Point2 {
	return Point2{X: p.X + v.X, Y: p.Y + v.Y}
}

// Sub returns the displacement vector from q to p (p - q).
func (p Point2) Sub(q Point2) Vec2 {
	return Vec2{X: p.X - q.X, Y: p.Y - q.Y}
}

// SubVec returns p translated by -v.
func (p Point2) SubVec(v Vec2) Point2 {
	return Point2{X: p.X - v.X, Y: p.Y - v.Y}
}

// DistanceTo returns the Euclidean distance between p and q.
func (p Point2) DistanceTo(q Point2) float64 {
	return math.Hypot(p.X-q.X, p.Y-q.Y)
}

// DistanceToOrigin returns the distance from p to (0, 0).
func (p Point2) DistanceToOrigin() float64 {
	return math.Hypot(p.X, p.Y)
}

// PolarAngle returns the angle from (0,0) to p relative to the +X axis.
func (p Point2) PolarAngle() Angle {
	return Angle(math.Atan2(p.Y, p.X))
}

// PolarAngleDeg returns the angle from (0,0) to p in degrees.
func (p Point2) PolarAngleDeg() float64 {
	return p.PolarAngle().Deg()
}

// VecFromOrigin returns the position as a vector from the origin.
func (p Point2) VecFromOrigin() Vec2 {
	return Vec2{X: p.X, Y: p.Y}
}

// Axis2 represents a directed line or ray with an anchor origin and a normalized unit direction.
type Axis2 struct {
	Origin    Point2
	Direction Vec2 // normalized unit vector
}

// NewAxis creates an Axis2 and normalizes its direction vector.
func NewAxis(origin Point2, dir Vec2) Axis2 {
	return Axis2{
		Origin:    origin,
		Direction: dir.Unit(),
	}
}

// NewAxisFromPolar creates an Axis2 starting at origin directed along standard polar angle (counter-clockwise from +X).
func NewAxisFromPolar(origin Point2, angle Angle) Axis2 {
	return Axis2{
		Origin:    origin,
		Direction: UnitFromPolar(angle),
	}
}

// Normal returns the unit normal vector perpendicular to the axis in turn direction.
func (a Axis2) Normal(turn TurnDirection) Vec2 {
	return a.Direction.Perpendicular(turn)
}

// PointAt returns a point at axial distance along the axis from origin.
func (a Axis2) PointAt(axialDist float64) Point2 {
	return a.Origin.Add(a.Direction.Scale(axialDist))
}

// PointWithOffset returns a point at axial distance along the axis, then offset by normalDist along the axis normal.
func (a Axis2) PointWithOffset(axialDist, normalDist float64, turn TurnDirection) Point2 {
	return a.PointAt(axialDist).Add(a.Normal(turn).Scale(normalDist))
}

// IntersectHorizontalLine finds the intersection of the axis with the horizontal line Y = y.
// Returns false if the axis is parallel to the horizontal line (Direction.Y == 0).
func (a Axis2) IntersectHorizontalLine(y float64) (Point2, bool) {
	if math.Abs(a.Direction.Y) < 1e-12 {
		return Point2{}, false
	}
	t := (y - a.Origin.Y) / a.Direction.Y
	return a.PointAt(t), true
}

// Intersect finds the intersection of two axes.
// Returns false if the axes are parallel.
func (a Axis2) Intersect(other Axis2) (Point2, bool) {
	cross := a.Direction.Cross(other.Direction)
	if math.Abs(cross) < 1e-12 {
		return Point2{}, false
	}
	dp := other.Origin.Sub(a.Origin)
	t := dp.Cross(other.Direction) / cross
	return a.PointAt(t), true
}

// Transform2 represents a 2D isometric rigid-body transform (rotation around a pivot point).
type Transform2 struct {
	Pivot    Point2
	Rotation Angle // counter-clockwise if positive
}

// NewRotation creates a Transform2 rotating around pivot by angle.
func NewRotation(pivot Point2, angle Angle) Transform2 {
	return Transform2{Pivot: pivot, Rotation: angle}
}

// RotationForLift returns a Transform2 pivoting around 'pivot' that vertically lifts 'point' by dy.
// When point is to the right (+X) of pivot (like front axle relative to rear axle),
// lifting (+dy) pitches the bike counter-clockwise around the rear axle.
func RotationForLift(pivot, point Point2, dy float64) (Transform2, error) {
	arm := pivot.DistanceTo(point)
	ang, err := AngleFromLift(arm, dy)
	if err != nil {
		return Transform2{}, err
	}
	// If point is to the right of pivot (dx > 0), positive dy lifts the right side -> counter-clockwise rotation (+ang)
	// If point is to the left of pivot (dx < 0), positive dy lifts the left side -> clockwise rotation (-ang)
	dx := point.X - pivot.X
	if dx < 0 {
		ang = ang.Neg()
	}
	return Transform2{Pivot: pivot, Rotation: ang}, nil
}

// Apply transforms point p by rotating around t.Pivot by t.Rotation.
func (t Transform2) Apply(p Point2) Point2 {
	rel := p.Sub(t.Pivot).Rotate(t.Rotation)
	return t.Pivot.Add(rel)
}

// ApplyVec rotates vector v by t.Rotation.
func (t Transform2) ApplyVec(v Vec2) Vec2 {
	return v.Rotate(t.Rotation)
}

// Inverse returns the inverse transformation.
func (t Transform2) Inverse() Transform2 {
	return Transform2{Pivot: t.Pivot, Rotation: t.Rotation.Neg()}
}

// Ellipse2 represents a 2D oriented ellipse for cockpit fit evaluation.
type Ellipse2 struct {
	Center     Point2
	SemiMinorA float64 // semi-minor radius (horizontal before rotation)
	SemiMajorB float64 // semi-major radius (vertical before rotation)
	Angle      Angle   // clockwise rotation from vertical/major axis
}

// Evaluate returns the normalized elliptical distance and whether point p lies inside or on the ellipse.
// In the bike fit model, AngleDeg rotates clockwise (major axis tilts from +Y into +X).
func (e Ellipse2) Evaluate(p Point2) (normalizedRadius float64, isInside bool) {
	dx := p.X - e.Center.X
	dy := p.Y - e.Center.Y

	rad := e.Angle.Rad()
	sinT := math.Sin(rad)
	cosT := math.Cos(rad)

	// Rotated projections: clockwise rotation by theta tilts major axis (+dy) into (+dx)
	pMajor := dx*sinT + dy*cosT
	pMinor := dx*cosT - dy*sinT

	a := e.SemiMinorA
	if a <= 0 {
		a = 10.0
	}
	b := e.SemiMajorB
	if b <= 0 {
		b = 20.0
	}

	r := math.Hypot(pMinor/a, pMajor/b)
	return r, r <= 1.0+1e-9
}
