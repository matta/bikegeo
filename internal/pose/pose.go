// Package pose provides type-safe 2D planar points, displacement vectors,
// turtle poses, and rigid transforms parameterized by zero-sized phantom
// coordinate-space and orientation-axis marker types.
//
// The type parameters enforce three geometric invariants at compile time:
//
//  1. Affine separation (Point[S] vs. Vec[S]): positions and displacement
//     vectors in coordinate space S are distinct types. Subtracting two
//     Point[S] values yields a Vec[S]; adding a Vec[S] to a Point[S] yields a
//     Point[S]; adding two Point[S] values is unrepresentable, and Euclidean
//     length (Norm) exists only on Vec[S]. Point-to-point measurements in S
//     (DistanceTo, VerticalOffsetTo, OffsetTo, AngleTo) live on Point[S]
//     because they depend only on position in S, allowing landmarks with
//     different local orientation axes to be compared directly in the same
//     world space.
//
//  2. Ambient coordinate-space isolation (W / S): world states and coordinate
//     frames (such as unpitched chart coordinates, static workshop coordinates,
//     and dynamic riding coordinates) are represented by distinct defined empty
//     struct types (for example, `type DynamicWorld struct{}`; never use a type
//     alias `= struct{}`). Geometry operations require matching spaces. Use
//     Transform[Src, Dst] to map coordinates between spaces.
//
//  3. Turtle orientation-axis tracking (Pose[Axis, W]): a Pose[Axis, W] places
//     a turtle at an Origin() in ambient space W whose Heading() defines the
//     local orientation Axis (+X along Heading(), +Y 90° counter-clockwise).
//     Moving the turtle via Advance(distMm) or Translate(Vec[Axis]) changes its
//     origin in W while keeping its local orientation Axis invariant, so both
//     remain dot-chained methods on Pose[Axis, W] and Translate rejects vectors
//     from any other component's local frame. Only Turn and Face change Axis;
//     because Go methods cannot introduce a new type parameter, Turn and Face
//     are package-level functions returning Pose[NewAxis, W].
//
// Marker types carry no runtime data, so Point[S], Vec[S], and Pose[Axis, W]
// add no memory overhead and serialize to flat JSON.
package pose

import (
	"encoding/json"
	"math"
)

const radPerDeg = math.Pi / 180.0

// Deg is a scalar angle in degrees with no bound coordinate space or axis pair,
// used for catalog dimensions, tolerances, and user inputs.
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

// Heading is an absolute orientation angle in degrees in ambient coordinate
// space W (0° along W's +X axis, 90° along W's +Y axis; positive angles rotate
// counter-clockwise).
type Heading[W any] float64

// Deg returns h as a raw scalar degree value.
func (h Heading[W]) Deg() Deg {
	return Deg(h)
}

// Turn returns h rotated counter-clockwise by deltaDeg in W.
func (h Heading[W]) Turn(deltaDeg Deg) Heading[W] {
	return h + Heading[W](deltaDeg)
}

// Sub returns the signed counter-clockwise angle from other to h (h − other).
func (h Heading[W]) Sub(other Heading[W]) Deg {
	return Deg(h - other)
}

func (h Heading[W]) sin() float64 { return Deg(h).sin() }
func (h Heading[W]) cos() float64 { return Deg(h).cos() }

// TurnDeg is a directed relative rotation angle in degrees from FromAxis's +X
// direction to ToAxis's +X direction (positive counter-clockwise). Encoding the
// source and destination axes in the type allows Turn(p, delta) to infer all
// type parameters without brackets. Swapping the poses in MeasureTurn
// reverses both the sign and the type's axis order, so the resulting turn
// cannot be applied to a pose with the original source axis.
type TurnDeg[FromAxis, ToAxis any] float64

// Deg returns d as a raw scalar degree value.
func (d TurnDeg[FromAxis, ToAxis]) Deg() Deg {
	return Deg(d)
}

// Point is a 2D position in millimeters in coordinate space S. Subtracting
// positions yields a Vec[S], and adding a Vec[S] translates a position.
// Comparing two points requires only their coordinates in S, so landmarks
// with different pose axes and measured targets share the same measurements.
type Point[S any] struct {
	X float64
	Y float64
}

// Sub returns the displacement vector p − q in coordinate space S.
func (p Point[S]) Sub(q Point[S]) Vec[S] {
	return Vec[S]{X: p.X - q.X, Y: p.Y - q.Y}
}

// Add returns the position p translated by displacement vector v in S.
func (p Point[S]) Add(v Vec[S]) Point[S] {
	return Point[S]{X: p.X + v.X, Y: p.Y + v.Y}
}

// DistanceTo returns the Euclidean distance in millimeters between p and other in S.
func (p Point[S]) DistanceTo(other Point[S]) float64 {
	return p.OffsetTo(other).Norm()
}

// VerticalOffsetTo returns the signed +Y offset in millimeters from p to other
// in S (other.Y − p.Y).
func (p Point[S]) VerticalOffsetTo(other Point[S]) float64 {
	return other.Y - p.Y
}

// OffsetTo returns the displacement vector from p to other in S (other − p).
func (p Point[S]) OffsetTo(other Point[S]) Vec[S] {
	return other.Sub(p)
}

// AngleTo returns the heading in S (0° along +X, positive counter-clockwise)
// from p to other.
func (p Point[S]) AngleTo(other Point[S]) Heading[S] {
	delta := p.OffsetTo(other)
	return Heading[S](degFromRad(math.Atan2(delta.Y, delta.X)))
}

// Vec is a 2D displacement vector in millimeters in coordinate space S.
// Adding or subtracting displacements yields another Vec[S]. Norm measures
// the length of a displacement.
type Vec[S any] struct {
	X float64
	Y float64
}

// Add returns v + w in S.
func (v Vec[S]) Add(w Vec[S]) Vec[S] {
	return Vec[S]{X: v.X + w.X, Y: v.Y + w.Y}
}

// Sub returns v − w in S.
func (v Vec[S]) Sub(w Vec[S]) Vec[S] {
	return Vec[S]{X: v.X - w.X, Y: v.Y - w.Y}
}

// Scale returns v scaled by scalar factor k in S.
func (v Vec[S]) Scale(k float64) Vec[S] {
	return Vec[S]{X: v.X * k, Y: v.Y * k}
}

// Neg returns −v in S.
func (v Vec[S]) Neg() Vec[S] {
	return Vec[S]{X: -v.X, Y: -v.Y}
}

// Norm returns the Euclidean length of v in millimeters.
func (v Vec[S]) Norm() float64 {
	return math.Hypot(v.X, v.Y)
}

// Pose is an immutable 2D position and heading in ambient coordinate space W
// whose heading defines the local orientation Axis (local +X along heading,
// local +Y 90° counter-clockwise).
type Pose[Axis, W any] struct {
	origin     Point[W]
	headingDeg Heading[W]
}

// NewPose creates a Pose[Axis, W] at origin with headingDeg in W.
func NewPose[Axis, W any](origin Point[W], headingDeg Heading[W]) Pose[Axis, W] {
	return Pose[Axis, W]{origin: origin, headingDeg: headingDeg}
}

// Origin returns the Pose's position in W in millimeters.
func (p Pose[Axis, W]) Origin() Point[W] {
	return p.origin
}

// Heading returns the Pose's typed heading in W.
func (p Pose[Axis, W]) Heading() Heading[W] {
	return p.headingDeg
}

// HeadingDeg returns the Pose's heading in W as a scalar Deg value.
func (p Pose[Axis, W]) HeadingDeg() Deg {
	return Deg(p.headingDeg)
}

// MarshalJSON serializes a Pose's unexported Origin and HeadingDeg for
// encoding/json CLI output (Q49a, Q88).
func (p Pose[Axis, W]) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Origin     Point[W]
		HeadingDeg Deg
	}{
		Origin:     p.origin,
		HeadingDeg: Deg(p.headingDeg),
	})
}

// Advance returns a new Pose[Axis, W] moved distMm along Axis's +X direction.
func (p Pose[Axis, W]) Advance(distMm float64) Pose[Axis, W] {
	return Pose[Axis, W]{
		origin: Point[W]{
			X: p.origin.X + distMm*p.headingDeg.cos(),
			Y: p.origin.Y + distMm*p.headingDeg.sin(),
		},
		headingDeg: p.headingDeg,
	}
}

// Translate returns a new Pose[Axis, W] moved by local displacement v in Axis's
// coordinate frame (+X along heading, +Y 90° counter-clockwise).
func (p Pose[Axis, W]) Translate(v Vec[Axis]) Pose[Axis, W] {
	cos := p.headingDeg.cos()
	sin := p.headingDeg.sin()
	return Pose[Axis, W]{
		origin: Point[W]{
			X: p.origin.X + v.X*cos - v.Y*sin,
			Y: p.origin.Y + v.X*sin + v.Y*cos,
		},
		headingDeg: p.headingDeg,
	}
}

// Turn returns a new Pose[ToAxis, W] at p's origin rotated from FromAxis into
// ToAxis by delta. All type parameters (FromAxis, ToAxis, W) are inferred from
// the arguments without explicit type brackets at the call site.
func Turn[FromAxis, ToAxis, W any](p Pose[FromAxis, W], delta TurnDeg[FromAxis, ToAxis]) Pose[ToAxis, W] {
	return Pose[ToAxis, W]{
		origin:     p.origin,
		headingDeg: p.headingDeg.Turn(Deg(delta)),
	}
}

// MeasureTurn returns the directed relative rotation from FromAxis to ToAxis
// in W by subtracting from.Heading() from to.Heading().
func MeasureTurn[FromAxis, ToAxis, W any](from Pose[FromAxis, W], to Pose[ToAxis, W]) TurnDeg[FromAxis, ToAxis] {
	return TurnDeg[FromAxis, ToAxis](to.headingDeg.Sub(from.headingDeg))
}

// Face returns a new Pose[NewAxis, W] at p's origin with its heading in W set
// directly to worldHeading, establishing NewAxis. Because NewAxis is first in
// the type parameter list, callers pass only Face[NewAxis](p, worldHeading) and
// Go infers OldAxis and W.
func Face[NewAxis, OldAxis, W any](p Pose[OldAxis, W], worldHeading Heading[W]) Pose[NewAxis, W] {
	return Pose[NewAxis, W]{
		origin:     p.origin,
		headingDeg: worldHeading,
	}
}

// Transform returns the 2D rigid transformation mapping local coordinates in
// Axis (anchored at p.Origin() with +X along p.Heading()) into ambient space W.
// The pose determines both spaces and the placement of the local frame.
func (p Pose[Axis, W]) Transform() Transform[Axis, W] {
	return NewRigidTransform[Axis, W](p.origin, p.headingDeg)
}

// Transform is a 2D rigid transformation mapping points and vectors from
// coordinate space Src into coordinate space Dst. It rotates both points
// and vectors into Dst and then adds the destination-space origin to points.
// Construct one with NewRigidTransform or Pose.Transform.
type Transform[Src, Dst any] struct {
	tx, ty float64
	cos    float64
	sin    float64
}

// NewRigidTransform constructs a Transform[Src, Dst] placing Src's origin at
// originInDst with Src's +X axis oriented at headingInDst in Dst.
func NewRigidTransform[Src, Dst any](originInDst Point[Dst], headingInDst Heading[Dst]) Transform[Src, Dst] {
	return Transform[Src, Dst]{
		tx:  originInDst.X,
		ty:  originInDst.Y,
		cos: headingInDst.cos(),
		sin: headingInDst.sin(),
	}
}

// ApplyPoint transforms a position from Src into Dst (rotation + translation).
func (t Transform[Src, Dst]) ApplyPoint(p Point[Src]) Point[Dst] {
	return Point[Dst]{
		X: t.tx + p.X*t.cos - p.Y*t.sin,
		Y: t.ty + p.X*t.sin + p.Y*t.cos,
	}
}

// ApplyVec transforms a displacement vector from Src into Dst (rotation only).
func (t Transform[Src, Dst]) ApplyVec(v Vec[Src]) Vec[Dst] {
	return Vec[Dst]{
		X: v.X*t.cos - v.Y*t.sin,
		Y: v.X*t.sin + v.Y*t.cos,
	}
}

// Inverse returns the inverse rigid transformation mapping Dst back into Src.
func (t Transform[Src, Dst]) Inverse() Transform[Dst, Src] {
	return Transform[Dst, Src]{
		tx:  -(t.tx*t.cos + t.ty*t.sin),
		ty:  -(-t.tx*t.sin + t.ty*t.cos),
		cos: t.cos,
		sin: -t.sin,
	}
}

// Compose chains ab (A → B) with bc (B → C) to produce the composite rigid
// transformation (A → C).
func Compose[A, B, C any](ab Transform[A, B], bc Transform[B, C]) Transform[A, C] {
	return Transform[A, C]{
		tx:  bc.tx + ab.tx*bc.cos - ab.ty*bc.sin,
		ty:  bc.ty + ab.tx*bc.sin + ab.ty*bc.cos,
		cos: ab.cos*bc.cos - ab.sin*bc.sin,
		sin: ab.sin*bc.cos + ab.cos*bc.sin,
	}
}
