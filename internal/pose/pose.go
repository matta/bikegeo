// Package pose provides 2D planar points, displacement vectors, fluent turtle
// poses, and rigid transforms tagged at runtime with an opaque CoordinateSystem.
//
// The types enforce three geometric invariants:
//
//  1. Affine separation (Point vs. Vector): positions and displacement vectors
//     are distinct concrete types. A Vector has no fixed origin and carries
//     signed displacements in millimeters (not unit-normalized unless explicitly
//     constructed with unit norm, such as Along(h, 1)). Subtracting two Points
//     in the same CoordinateSystem yields a Vector; adding a Vector to a Point
//     in the same CoordinateSystem yields a Point; adding two Points is
//     unrepresentable, and Euclidean length (Norm) exists only on Vector.
//
//  2. Coordinate-system isolation (CoordinateSystem): every Point, Vector,
//     Heading, Pose, and Transform carries a non-zero CoordinateSystem
//     registered via NewCoordinateSystem. CoordinateSystem identifies both a
//     coordinate system's datum origin (0, 0) and its (+X, +Y) axes.
//     Uninitialized zero-value structs have an invalid zero CoordinateSystem
//     and panic immediately on first use; operations across mismatched
//     CoordinateSystems panic with both coordinate system names.
//
//  3. Fluent turtle Pose & rigid Transform: a Pose places a turtle at an
//     Origin() in its CoordinateSystem whose Heading() sets its local +X
//     direction (local +Y is 90° counter-clockwise). Advance, TranslateLocal,
//     Turn, and Face chain fluently as methods. Anchor(childSystem) returns a
//     rigid Transform mapping childSystem into the parent Pose's
//     CoordinateSystem.
package pose

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

const radPerDeg = math.Pi / 180.0

var (
	systemMu    sync.RWMutex
	systemNames = []string{"<uninitialized>"}
)

// CoordinateSystem identifies a specific 2D coordinate system (both its datum
// origin and its +X/+Y axes). The zero value is invalid; register coordinate
// systems via NewCoordinateSystem.
type CoordinateSystem struct{ id uint16 }

// NewCoordinateSystem allocates a unique non-zero CoordinateSystem and records
// its human-readable name.
func NewCoordinateSystem(name string) CoordinateSystem {
	if name == "" {
		panic("pose: NewCoordinateSystem requires a non-empty name")
	}
	systemMu.Lock()
	defer systemMu.Unlock()
	if len(systemNames) > math.MaxUint16 {
		panic("pose: CoordinateSystem table overflow")
	}
	id := uint16(len(systemNames))
	systemNames = append(systemNames, name)
	return CoordinateSystem{id: id}
}

// Valid reports whether s is a registered non-zero CoordinateSystem.
func (s CoordinateSystem) Valid() bool {
	return s.id != 0
}

// String returns the coordinate system's registered name, or "<uninitialized>"
// for the zero CoordinateSystem.
func (s CoordinateSystem) String() string {
	if s.id == 0 {
		return "<uninitialized>"
	}
	systemMu.RLock()
	defer systemMu.RUnlock()
	if int(s.id) < len(systemNames) {
		return systemNames[s.id]
	}
	return fmt.Sprintf("CoordinateSystem(%d)", s.id)
}

//go:noinline
func panicUninitializedSystem() {
	panic("pose: uninitialized zero-value CoordinateSystem")
}

//go:noinline
func panicSystemMismatch(a, b CoordinateSystem) {
	if a.id == 0 || b.id == 0 {
		panic("pose: uninitialized zero-value CoordinateSystem")
	}
	panic(fmt.Sprintf("pose: coordinate system mismatch (%s vs %s)", a, b))
}

func requireValidSystem(s CoordinateSystem) {
	if s.id == 0 {
		panicUninitializedSystem()
	}
}

func requireSameSystem(a, b CoordinateSystem) {
	if a.id == 0 || a != b {
		panicSystemMismatch(a, b)
	}
}

// Deg is a scalar angle in degrees with no bound coordinate system, used for
// catalog dimensions, relative turns, tolerances, and user inputs.
type Deg float64

func degFromRad(rad float64) Deg {
	return Deg(rad / radPerDeg)
}

func (d Deg) rad() float64 {
	return float64(d) * radPerDeg
}

func (d Deg) sincos() (sin, cos float64) {
	if d == 0 {
		return 0, 1
	}
	return math.Sincos(d.rad())
}

func (d Deg) tan() float64 {
	return math.Tan(d.rad())
}

// Heading is an absolute orientation angle in degrees within a specific
// coordinate system (0° along +X, 90° along +Y; positive angles rotate
// counter-clockwise).
type Heading struct {
	deg    Deg
	system CoordinateSystem
}

// NewHeading creates a Heading in system with angle deg.
func NewHeading(system CoordinateSystem, deg Deg) Heading {
	requireValidSystem(system)
	return Heading{deg: deg, system: system}
}

// CoordinateSystem returns h's coordinate system.
func (h Heading) CoordinateSystem() CoordinateSystem {
	return h.system
}

// Deg returns h as a raw scalar degree value.
func (h Heading) Deg() Deg {
	requireValidSystem(h.system)
	return h.deg
}

// Turn returns h rotated counter-clockwise by deltaDeg in the same CoordinateSystem.
func (h Heading) Turn(deltaDeg Deg) Heading {
	requireValidSystem(h.system)
	return Heading{deg: h.deg + deltaDeg, system: h.system}
}

// Sub returns the signed counter-clockwise angle from other to h (h − other)
// in the same CoordinateSystem.
func (h Heading) Sub(other Heading) Deg {
	requireSameSystem(h.system, other.system)
	return h.deg - other.deg
}

// Point is a 2D position in millimeters in a specific coordinate system.
// Subtracting two Points yields a Vector; adding a Vector translates a Point.
type Point struct {
	x, y   float64
	system CoordinateSystem
}

// NewPoint creates a Point at (x, y) mm in system.
func NewPoint(system CoordinateSystem, x, y float64) Point {
	requireValidSystem(system)
	return Point{x: x, y: y, system: system}
}

// Origin returns the datum origin (0, 0) in system.
func Origin(system CoordinateSystem) Point {
	requireValidSystem(system)
	return Point{system: system}
}

// X returns p's X coordinate in millimeters.
func (p Point) X() float64 {
	requireValidSystem(p.system)
	return p.x
}

// Y returns p's Y coordinate in millimeters.
func (p Point) Y() float64 {
	requireValidSystem(p.system)
	return p.y
}

// CoordinateSystem returns p's coordinate system.
func (p Point) CoordinateSystem() CoordinateSystem {
	return p.system
}

// Sub returns the displacement vector p − q in the same CoordinateSystem.
func (p Point) Sub(q Point) Vector {
	requireSameSystem(p.system, q.system)
	return Vector{x: p.x - q.x, y: p.y - q.y, system: p.system}
}

// Add returns the position p translated by displacement vector v in the same CoordinateSystem.
func (p Point) Add(v Vector) Point {
	requireSameSystem(p.system, v.system)
	return Point{x: p.x + v.x, y: p.y + v.y, system: p.system}
}

// OffsetTo returns the displacement vector from p to other (other − p) in the same CoordinateSystem.
func (p Point) OffsetTo(other Point) Vector {
	return other.Sub(p)
}

// DistanceTo returns the Euclidean distance in millimeters between p and other.
func (p Point) DistanceTo(other Point) float64 {
	return p.OffsetTo(other).Norm()
}

// VerticalOffsetTo returns the signed +Y offset in millimeters from p to other
// (other.Y() − p.Y()) in the same CoordinateSystem.
func (p Point) VerticalOffsetTo(other Point) float64 {
	requireSameSystem(p.system, other.system)
	return other.y - p.y
}

// AngleTo returns the heading (0° along +X, positive counter-clockwise) from p to other.
func (p Point) AngleTo(other Point) Heading {
	delta := p.OffsetTo(other)
	return Heading{deg: degFromRad(math.Atan2(delta.y, delta.x)), system: p.system}
}

// MarshalJSON serializes p as {"X":..., "Y":...}.
func (p Point) MarshalJSON() ([]byte, error) {
	requireValidSystem(p.system)
	return json.Marshal(struct {
		X float64
		Y float64
	}{
		X: p.x,
		Y: p.y,
	})
}

// Vector is a 2D displacement vector in millimeters in a specific coordinate
// system. It carries no origin and is not unit-normalized unless explicitly
// constructed with unit norm (for example via Along(heading, 1)).
type Vector struct {
	x, y   float64
	system CoordinateSystem
}

// NewVector creates a Vector with signed components (dx, dy) mm in system.
func NewVector(system CoordinateSystem, dx, dy float64) Vector {
	requireValidSystem(system)
	return Vector{x: dx, y: dy, system: system}
}

// X returns v's X component in millimeters.
func (v Vector) X() float64 {
	requireValidSystem(v.system)
	return v.x
}

// Y returns v's Y component in millimeters.
func (v Vector) Y() float64 {
	requireValidSystem(v.system)
	return v.y
}

// CoordinateSystem returns v's coordinate system.
func (v Vector) CoordinateSystem() CoordinateSystem {
	return v.system
}

// Add returns v + w in the same CoordinateSystem.
func (v Vector) Add(w Vector) Vector {
	requireSameSystem(v.system, w.system)
	return Vector{x: v.x + w.x, y: v.y + w.y, system: v.system}
}

// Sub returns v − w in the same CoordinateSystem.
func (v Vector) Sub(w Vector) Vector {
	requireSameSystem(v.system, w.system)
	return Vector{x: v.x - w.x, y: v.y - w.y, system: v.system}
}

// Scale returns v scaled by scalar factor k.
func (v Vector) Scale(k float64) Vector {
	requireValidSystem(v.system)
	return Vector{x: v.x * k, y: v.y * k, system: v.system}
}

// Neg returns −v.
func (v Vector) Neg() Vector {
	requireValidSystem(v.system)
	return Vector{x: -v.x, y: -v.y, system: v.system}
}

// Norm returns the Euclidean length of v in millimeters.
func (v Vector) Norm() float64 {
	requireValidSystem(v.system)
	return math.Hypot(v.x, v.y)
}

// Dot returns the dot product v·other in the same CoordinateSystem.
func (v Vector) Dot(other Vector) float64 {
	requireSameSystem(v.system, other.system)
	return v.x*other.x + v.y*other.y
}

// Pose is an immutable 2D position and heading in a coordinate system whose
// heading sets the turtle's local +X direction (local +Y is 90° counter-clockwise).
type Pose struct {
	origin  Point
	heading Heading
}

// NewPose creates a Pose at origin with headingDeg in origin's CoordinateSystem.
func NewPose(origin Point, headingDeg Deg) Pose {
	requireValidSystem(origin.system)
	return Pose{
		origin:  origin,
		heading: NewHeading(origin.system, headingDeg),
	}
}

// OriginPose creates a Pose at the datum origin (0, 0) of system with heading 0°.
func OriginPose(system CoordinateSystem) Pose {
	return NewPose(Origin(system), 0)
}

// Origin returns the Pose's position in millimeters.
func (p Pose) Origin() Point {
	requireValidSystem(p.origin.system)
	return p.origin
}

// Heading returns the Pose's Heading.
func (p Pose) Heading() Heading {
	requireValidSystem(p.heading.system)
	return p.heading
}

// HeadingDeg returns the Pose's heading as a scalar Deg value.
func (p Pose) HeadingDeg() Deg {
	requireValidSystem(p.heading.system)
	return p.heading.deg
}

// CoordinateSystem returns the Pose's coordinate system.
func (p Pose) CoordinateSystem() CoordinateSystem {
	return p.origin.system
}

// Advance returns a new Pose moved distMm along its local +X direction.
func (p Pose) Advance(distMm float64) Pose {
	requireSameSystem(p.origin.system, p.heading.system)
	sin, cos := p.heading.deg.sincos()
	return Pose{
		origin: Point{
			x:      p.origin.x + distMm*cos,
			y:      p.origin.y + distMm*sin,
			system: p.origin.system,
		},
		heading: p.heading,
	}
}

// TranslateLocal returns a new Pose moved dx along p's local +X and dy along
// p's local +Y (90° CCW) while staying in p's CoordinateSystem.
func (p Pose) TranslateLocal(dx, dy float64) Pose {
	requireSameSystem(p.origin.system, p.heading.system)
	sin, cos := p.heading.deg.sincos()
	return Pose{
		origin: Point{
			x:      p.origin.x + dx*cos - dy*sin,
			y:      p.origin.y + dx*sin + dy*cos,
			system: p.origin.system,
		},
		heading: p.heading,
	}
}

// Turn returns a new Pose at p's origin rotated counter-clockwise by deltaDeg.
func (p Pose) Turn(deltaDeg Deg) Pose {
	requireSameSystem(p.origin.system, p.heading.system)
	return Pose{
		origin:  p.origin,
		heading: p.heading.Turn(deltaDeg),
	}
}

// Face returns a new Pose at p's origin with its heading set directly to worldHeading.
func (p Pose) Face(worldHeading Heading) Pose {
	requireSameSystem(p.origin.system, worldHeading.system)
	return Pose{
		origin:  p.origin,
		heading: worldHeading,
	}
}

// Anchor returns the rigid Transform mapping childSystem (whose (0, 0) sits at
// p.Origin() and +X points along p.Heading()) into p.CoordinateSystem().
func (p Pose) Anchor(childSystem CoordinateSystem) Transform {
	requireSameSystem(p.origin.system, p.heading.system)
	return NewTransform(childSystem, p.origin.system, p.origin, p.heading.deg)
}

// MarshalJSON serializes a Pose's unexported Origin and HeadingDeg for
// encoding/json CLI output.
func (p Pose) MarshalJSON() ([]byte, error) {
	requireSameSystem(p.origin.system, p.heading.system)
	return json.Marshal(struct {
		Origin     Point
		HeadingDeg Deg
	}{
		Origin:     p.origin,
		HeadingDeg: p.heading.deg,
	})
}

// Transform is a 2D rigid transformation mapping points, vectors, and poses
// from coordinate system Src into coordinate system Dst.
type Transform struct {
	src, dst CoordinateSystem
	tx, ty   float64
	cos, sin float64
	rotDeg   Deg
}

// NewTransform constructs a Transform placing src's datum origin (0, 0) at
// originInDst with src's +X oriented at headingInDst in dst.
func NewTransform(src, dst CoordinateSystem, originInDst Point, headingInDst Deg) Transform {
	requireValidSystem(src)
	requireValidSystem(dst)
	requireSameSystem(originInDst.system, dst)
	if src == dst {
		panic(fmt.Sprintf("pose: NewTransform requires distinct coordinate systems (%s == %s); use IdentityTransform", src, dst))
	}
	sin, cos := headingInDst.sincos()
	return Transform{
		src:    src,
		dst:    dst,
		tx:     originInDst.x,
		ty:     originInDst.y,
		cos:    cos,
		sin:    sin,
		rotDeg: headingInDst,
	}
}

// IdentityTransform returns the identity Transform within system.
func IdentityTransform(system CoordinateSystem) Transform {
	requireValidSystem(system)
	return Transform{src: system, dst: system, cos: 1}
}

// Src returns t's source coordinate system.
func (t Transform) Src() CoordinateSystem {
	return t.src
}

// Dst returns t's destination coordinate system.
func (t Transform) Dst() CoordinateSystem {
	return t.dst
}

// ApplyPoint transforms a position from t.Src() into t.Dst() (rotation + translation).
func (t Transform) ApplyPoint(p Point) Point {
	requireValidSystem(t.dst)
	requireSameSystem(p.system, t.src)
	return Point{
		x:      t.tx + p.x*t.cos - p.y*t.sin,
		y:      t.ty + p.x*t.sin + p.y*t.cos,
		system: t.dst,
	}
}

// ApplyVector transforms a displacement vector from t.Src() into t.Dst() (rotation only).
func (t Transform) ApplyVector(v Vector) Vector {
	requireValidSystem(t.dst)
	requireSameSystem(v.system, t.src)
	return Vector{
		x:      v.x*t.cos - v.y*t.sin,
		y:      v.x*t.sin + v.y*t.cos,
		system: t.dst,
	}
}

// ApplyPose transforms a Pose from t.Src() into t.Dst().
func (t Transform) ApplyPose(p Pose) Pose {
	requireValidSystem(t.dst)
	requireSameSystem(p.origin.system, t.src)
	requireSameSystem(p.heading.system, t.src)
	return Pose{
		origin: Point{
			x:      t.tx + p.origin.x*t.cos - p.origin.y*t.sin,
			y:      t.ty + p.origin.x*t.sin + p.origin.y*t.cos,
			system: t.dst,
		},
		heading: Heading{
			deg:    p.heading.deg + t.rotDeg,
			system: t.dst,
		},
	}
}

// OriginInDst returns t.Src()'s datum origin expressed in t.Dst().
func (t Transform) OriginInDst() Point {
	requireValidSystem(t.src)
	requireValidSystem(t.dst)
	return Point{x: t.tx, y: t.ty, system: t.dst}
}

// HeadingInDst returns t.Src()'s +X heading expressed in t.Dst().
func (t Transform) HeadingInDst() Heading {
	requireValidSystem(t.src)
	requireValidSystem(t.dst)
	return Heading{deg: t.rotDeg, system: t.dst}
}

// PoseInDst returns t.Src()'s origin and +X heading as a Pose in t.Dst().
func (t Transform) PoseInDst() Pose {
	return NewPose(t.OriginInDst(), t.HeadingInDst().deg)
}

// Inverse returns the inverse rigid transformation mapping t.Dst() back into t.Src().
func (t Transform) Inverse() Transform {
	requireValidSystem(t.src)
	requireValidSystem(t.dst)
	rot := -t.rotDeg
	if rot == 0 {
		rot = 0
	}
	return Transform{
		src:    t.dst,
		dst:    t.src,
		tx:     -(t.tx*t.cos + t.ty*t.sin),
		ty:     -(-t.tx*t.sin + t.ty*t.cos),
		cos:    t.cos,
		sin:    -t.sin,
		rotDeg: rot,
	}
}

// Compose chains ab (A → B) with bc (B → C) to produce the composite rigid
// transformation (A → C).
func Compose(ab, bc Transform) Transform {
	requireValidSystem(ab.src)
	requireValidSystem(ab.dst)
	requireValidSystem(bc.src)
	requireValidSystem(bc.dst)
	if ab.dst != bc.src {
		panic(fmt.Sprintf("pose: transform compose mismatch (%s → %s cannot compose with %s → %s)", ab.src, ab.dst, bc.src, bc.dst))
	}
	return Transform{
		src:    ab.src,
		dst:    bc.dst,
		tx:     bc.tx + ab.tx*bc.cos - ab.ty*bc.sin,
		ty:     bc.ty + ab.tx*bc.sin + ab.ty*bc.cos,
		cos:    ab.cos*bc.cos - ab.sin*bc.sin,
		sin:    ab.sin*bc.cos + ab.cos*bc.sin,
		rotDeg: ab.rotDeg + bc.rotDeg,
	}
}
