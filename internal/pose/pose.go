// Package pose provides 2D planar points, displacement vectors, fluent turtle
// poses, and rigid transforms tagged at runtime with an opaque FrameID.
//
// The types enforce three geometric invariants:
//
//  1. Affine separation (Point vs. Vec): positions and displacement vectors are
//     distinct concrete types. A Vec has no fixed origin and carries signed
//     displacements in millimeters (not unit-normalized unless explicitly
//     constructed with unit norm, such as Along(h, 1)). Subtracting two Points
//     in the same FrameID yields a Vec; adding a Vec to a Point in the same
//     FrameID yields a Point; adding two Points is unrepresentable, and
//     Euclidean length (Norm) exists only on Vec.
//
//  2. Coordinate-frame isolation (FrameID): every Point, Vec, Heading, Pose,
//     and Transform carries a non-zero FrameID registered via NewFrame. FrameID
//     identifies both a coordinate frame's datum origin (0, 0) and its (+X, +Y)
//     axes. Uninitialized zero-value structs have an invalid zero FrameID and
//     panic immediately on first use; operations across mismatched FrameIDs
//     panic with both frame names.
//
//  3. Fluent turtle Pose & rigid Transform: a Pose places a turtle at an
//     Origin() in its FrameID whose Heading() sets its local +X direction
//     (local +Y is 90° counter-clockwise). Advance, TranslateLocal, Turn, and
//     Face chain fluently as methods. Anchor(childFrame) returns a rigid
//     Transform mapping childFrame into the parent Pose's FrameID.
package pose

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

const radPerDeg = math.Pi / 180.0

var (
	frameMu    sync.RWMutex
	frameNames = []string{"<uninitialized>"}
)

// FrameID identifies a specific 2D coordinate frame (both its datum origin and
// its +X/+Y axes). The zero value is invalid; register frames via NewFrame.
type FrameID struct{ id uint16 }

// NewFrame allocates a unique non-zero FrameID and records its human-readable name.
func NewFrame(name string) FrameID {
	if name == "" {
		panic("pose: NewFrame requires a non-empty name")
	}
	frameMu.Lock()
	defer frameMu.Unlock()
	if len(frameNames) > math.MaxUint16 {
		panic("pose: FrameID table overflow")
	}
	id := uint16(len(frameNames))
	frameNames = append(frameNames, name)
	return FrameID{id: id}
}

// Valid reports whether f is a registered non-zero FrameID.
func (f FrameID) Valid() bool {
	return f.id != 0
}

// String returns the frame's registered name, or "<uninitialized>" for the zero FrameID.
func (f FrameID) String() string {
	if f.id == 0 {
		return "<uninitialized>"
	}
	frameMu.RLock()
	defer frameMu.RUnlock()
	if int(f.id) < len(frameNames) {
		return frameNames[f.id]
	}
	return fmt.Sprintf("FrameID(%d)", f.id)
}

//go:noinline
func panicUninitializedFrame() {
	panic("pose: uninitialized zero-value FrameID")
}

//go:noinline
func panicFrameMismatch(a, b FrameID) {
	if a.id == 0 || b.id == 0 {
		panic("pose: uninitialized zero-value FrameID")
	}
	panic(fmt.Sprintf("pose: coordinate frame mismatch (%s vs %s)", a, b))
}

func requireValidFrame(f FrameID) {
	if f.id == 0 {
		panicUninitializedFrame()
	}
}

func requireSameFrame(a, b FrameID) {
	if a.id == 0 || a != b {
		panicFrameMismatch(a, b)
	}
}

// Deg is a scalar angle in degrees with no bound coordinate frame, used for
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
// coordinate frame (0° along +X, 90° along +Y; positive angles rotate
// counter-clockwise).
type Heading struct {
	deg   Deg
	frame FrameID
}

// NewHeading creates a Heading in frame with angle deg.
func NewHeading(frame FrameID, deg Deg) Heading {
	requireValidFrame(frame)
	return Heading{deg: deg, frame: frame}
}

// Frame returns h's coordinate frame.
func (h Heading) Frame() FrameID {
	return h.frame
}

// Deg returns h as a raw scalar degree value.
func (h Heading) Deg() Deg {
	requireValidFrame(h.frame)
	return h.deg
}

// Turn returns h rotated counter-clockwise by deltaDeg in the same FrameID.
func (h Heading) Turn(deltaDeg Deg) Heading {
	requireValidFrame(h.frame)
	return Heading{deg: h.deg + deltaDeg, frame: h.frame}
}

// Sub returns the signed counter-clockwise angle from other to h (h − other)
// in the same FrameID.
func (h Heading) Sub(other Heading) Deg {
	requireSameFrame(h.frame, other.frame)
	return h.deg - other.deg
}

// Point is a 2D position in millimeters in a specific coordinate frame.
// Subtracting two Points yields a Vec; adding a Vec translates a Point.
type Point struct {
	x, y  float64
	frame FrameID
}

// Pt creates a Point at (x, y) mm in frame.
func Pt(frame FrameID, x, y float64) Point {
	requireValidFrame(frame)
	return Point{x: x, y: y, frame: frame}
}

// Origin returns the datum origin (0, 0) in frame.
func Origin(frame FrameID) Point {
	requireValidFrame(frame)
	return Point{frame: frame}
}

// X returns p's X coordinate in millimeters.
func (p Point) X() float64 {
	requireValidFrame(p.frame)
	return p.x
}

// Y returns p's Y coordinate in millimeters.
func (p Point) Y() float64 {
	requireValidFrame(p.frame)
	return p.y
}

// Frame returns p's coordinate frame.
func (p Point) Frame() FrameID {
	return p.frame
}

// Sub returns the displacement vector p − q in the same FrameID.
func (p Point) Sub(q Point) Vec {
	requireSameFrame(p.frame, q.frame)
	return Vec{x: p.x - q.x, y: p.y - q.y, frame: p.frame}
}

// Add returns the position p translated by displacement vector v in the same FrameID.
func (p Point) Add(v Vec) Point {
	requireSameFrame(p.frame, v.frame)
	return Point{x: p.x + v.x, y: p.y + v.y, frame: p.frame}
}

// OffsetTo returns the displacement vector from p to other (other − p) in the same FrameID.
func (p Point) OffsetTo(other Point) Vec {
	return other.Sub(p)
}

// DistanceTo returns the Euclidean distance in millimeters between p and other.
func (p Point) DistanceTo(other Point) float64 {
	return p.OffsetTo(other).Norm()
}

// VerticalOffsetTo returns the signed +Y offset in millimeters from p to other
// (other.Y() − p.Y()) in the same FrameID.
func (p Point) VerticalOffsetTo(other Point) float64 {
	requireSameFrame(p.frame, other.frame)
	return other.y - p.y
}

// AngleTo returns the heading (0° along +X, positive counter-clockwise) from p to other.
func (p Point) AngleTo(other Point) Heading {
	delta := p.OffsetTo(other)
	return Heading{deg: degFromRad(math.Atan2(delta.y, delta.x)), frame: p.frame}
}

// MarshalJSON serializes p as {"X":..., "Y":...}.
func (p Point) MarshalJSON() ([]byte, error) {
	requireValidFrame(p.frame)
	return json.Marshal(struct {
		X float64
		Y float64
	}{
		X: p.x,
		Y: p.y,
	})
}

// Vec is a 2D displacement vector in millimeters in a specific coordinate frame.
// It carries no origin and is not unit-normalized unless explicitly
// constructed with unit norm (for example via Along(heading, 1)).
type Vec struct {
	x, y  float64
	frame FrameID
}

// Vc creates a Vec with components (x, y) mm in frame.
func Vc(frame FrameID, x, y float64) Vec {
	requireValidFrame(frame)
	return Vec{x: x, y: y, frame: frame}
}

// X returns v's X component in millimeters.
func (v Vec) X() float64 {
	requireValidFrame(v.frame)
	return v.x
}

// Y returns v's Y component in millimeters.
func (v Vec) Y() float64 {
	requireValidFrame(v.frame)
	return v.y
}

// Frame returns v's coordinate frame.
func (v Vec) Frame() FrameID {
	return v.frame
}

// Add returns v + w in the same FrameID.
func (v Vec) Add(w Vec) Vec {
	requireSameFrame(v.frame, w.frame)
	return Vec{x: v.x + w.x, y: v.y + w.y, frame: v.frame}
}

// Sub returns v − w in the same FrameID.
func (v Vec) Sub(w Vec) Vec {
	requireSameFrame(v.frame, w.frame)
	return Vec{x: v.x - w.x, y: v.y - w.y, frame: v.frame}
}

// Scale returns v scaled by scalar factor k.
func (v Vec) Scale(k float64) Vec {
	requireValidFrame(v.frame)
	return Vec{x: v.x * k, y: v.y * k, frame: v.frame}
}

// Neg returns −v.
func (v Vec) Neg() Vec {
	requireValidFrame(v.frame)
	return Vec{x: -v.x, y: -v.y, frame: v.frame}
}

// Norm returns the Euclidean length of v in millimeters.
func (v Vec) Norm() float64 {
	requireValidFrame(v.frame)
	return math.Hypot(v.x, v.y)
}

// Dot returns the dot product v·other in the same FrameID.
func (v Vec) Dot(other Vec) float64 {
	requireSameFrame(v.frame, other.frame)
	return v.x*other.x + v.y*other.y
}

// Pose is an immutable 2D position and heading in a coordinate frame whose
// heading sets the turtle's local +X direction (local +Y is 90° counter-clockwise).
type Pose struct {
	origin  Point
	heading Heading
}

// NewPose creates a Pose at origin with headingDeg in origin's FrameID.
func NewPose(origin Point, headingDeg Deg) Pose {
	requireValidFrame(origin.frame)
	return Pose{
		origin:  origin,
		heading: NewHeading(origin.frame, headingDeg),
	}
}

// OriginPose creates a Pose at the datum origin (0, 0) of frame with heading 0°.
func OriginPose(frame FrameID) Pose {
	return NewPose(Origin(frame), 0)
}

// Origin returns the Pose's position in millimeters.
func (p Pose) Origin() Point {
	requireValidFrame(p.origin.frame)
	return p.origin
}

// Heading returns the Pose's Heading.
func (p Pose) Heading() Heading {
	requireValidFrame(p.heading.frame)
	return p.heading
}

// HeadingDeg returns the Pose's heading as a scalar Deg value.
func (p Pose) HeadingDeg() Deg {
	requireValidFrame(p.heading.frame)
	return p.heading.deg
}

// Frame returns the Pose's coordinate frame.
func (p Pose) Frame() FrameID {
	return p.origin.frame
}

// Advance returns a new Pose moved distMm along its local +X direction.
func (p Pose) Advance(distMm float64) Pose {
	requireSameFrame(p.origin.frame, p.heading.frame)
	sin, cos := p.heading.deg.sincos()
	return Pose{
		origin: Point{
			x:     p.origin.x + distMm*cos,
			y:     p.origin.y + distMm*sin,
			frame: p.origin.frame,
		},
		heading: p.heading,
	}
}

// TranslateLocal returns a new Pose moved dx along p's local +X and dy along
// p's local +Y (90° CCW) while staying in p's FrameID.
func (p Pose) TranslateLocal(dx, dy float64) Pose {
	requireSameFrame(p.origin.frame, p.heading.frame)
	sin, cos := p.heading.deg.sincos()
	return Pose{
		origin: Point{
			x:     p.origin.x + dx*cos - dy*sin,
			y:     p.origin.y + dx*sin + dy*cos,
			frame: p.origin.frame,
		},
		heading: p.heading,
	}
}

// Turn returns a new Pose at p's origin rotated counter-clockwise by deltaDeg.
func (p Pose) Turn(deltaDeg Deg) Pose {
	requireSameFrame(p.origin.frame, p.heading.frame)
	return Pose{
		origin:  p.origin,
		heading: p.heading.Turn(deltaDeg),
	}
}

// Face returns a new Pose at p's origin with its heading set directly to worldHeading.
func (p Pose) Face(worldHeading Heading) Pose {
	requireSameFrame(p.origin.frame, worldHeading.frame)
	return Pose{
		origin:  p.origin,
		heading: worldHeading,
	}
}

// Anchor returns the rigid Transform mapping childFrame (whose (0, 0) sits at
// p.Origin() and +X points along p.Heading()) into p.Frame().
func (p Pose) Anchor(childFrame FrameID) Transform {
	requireSameFrame(p.origin.frame, p.heading.frame)
	return NewTransform(childFrame, p.origin.frame, p.origin, p.heading.deg)
}

// MarshalJSON serializes a Pose's unexported Origin and HeadingDeg for
// encoding/json CLI output.
func (p Pose) MarshalJSON() ([]byte, error) {
	requireSameFrame(p.origin.frame, p.heading.frame)
	return json.Marshal(struct {
		Origin     Point
		HeadingDeg Deg
	}{
		Origin:     p.origin,
		HeadingDeg: p.heading.deg,
	})
}

// Transform is a 2D rigid transformation mapping points, vectors, and poses
// from coordinate frame Src into coordinate frame Dst.
type Transform struct {
	src, dst FrameID
	tx, ty   float64
	cos, sin float64
	rotDeg   Deg
}

// NewTransform constructs a Transform placing src's datum origin (0, 0) at
// originInDst with src's +X oriented at headingInDst in dst.
func NewTransform(src, dst FrameID, originInDst Point, headingInDst Deg) Transform {
	requireValidFrame(src)
	requireValidFrame(dst)
	requireSameFrame(originInDst.frame, dst)
	if src == dst {
		panic(fmt.Sprintf("pose: NewTransform requires distinct frames (%s == %s); use IdentityTransform", src, dst))
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

// IdentityTransform returns the identity Transform within frame.
func IdentityTransform(frame FrameID) Transform {
	requireValidFrame(frame)
	return Transform{src: frame, dst: frame, cos: 1}
}

// Src returns t's source coordinate frame.
func (t Transform) Src() FrameID {
	return t.src
}

// Dst returns t's destination coordinate frame.
func (t Transform) Dst() FrameID {
	return t.dst
}

// ApplyPoint transforms a position from t.Src() into t.Dst() (rotation + translation).
func (t Transform) ApplyPoint(p Point) Point {
	requireValidFrame(t.dst)
	requireSameFrame(p.frame, t.src)
	return Point{
		x:     t.tx + p.x*t.cos - p.y*t.sin,
		y:     t.ty + p.x*t.sin + p.y*t.cos,
		frame: t.dst,
	}
}

// ApplyVec transforms a displacement vector from t.Src() into t.Dst() (rotation only).
func (t Transform) ApplyVec(v Vec) Vec {
	requireValidFrame(t.dst)
	requireSameFrame(v.frame, t.src)
	return Vec{
		x:     v.x*t.cos - v.y*t.sin,
		y:     v.x*t.sin + v.y*t.cos,
		frame: t.dst,
	}
}

// ApplyPose transforms a Pose from t.Src() into t.Dst().
func (t Transform) ApplyPose(p Pose) Pose {
	requireValidFrame(t.dst)
	requireSameFrame(p.origin.frame, t.src)
	requireSameFrame(p.heading.frame, t.src)
	return Pose{
		origin: Point{
			x:     t.tx + p.origin.x*t.cos - p.origin.y*t.sin,
			y:     t.ty + p.origin.x*t.sin + p.origin.y*t.cos,
			frame: t.dst,
		},
		heading: Heading{
			deg:   p.heading.deg + t.rotDeg,
			frame: t.dst,
		},
	}
}

// OriginInDst returns t.Src()'s datum origin expressed in t.Dst().
func (t Transform) OriginInDst() Point {
	requireValidFrame(t.src)
	requireValidFrame(t.dst)
	return Point{x: t.tx, y: t.ty, frame: t.dst}
}

// HeadingInDst returns t.Src()'s +X heading expressed in t.Dst().
func (t Transform) HeadingInDst() Heading {
	requireValidFrame(t.src)
	requireValidFrame(t.dst)
	return Heading{deg: t.rotDeg, frame: t.dst}
}

// PoseInDst returns t.Src()'s origin and +X heading as a Pose in t.Dst().
func (t Transform) PoseInDst() Pose {
	return NewPose(t.OriginInDst(), t.HeadingInDst().deg)
}

// Inverse returns the inverse rigid transformation mapping t.Dst() back into t.Src().
func (t Transform) Inverse() Transform {
	requireValidFrame(t.src)
	requireValidFrame(t.dst)
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
	requireValidFrame(ab.src)
	requireValidFrame(ab.dst)
	requireValidFrame(bc.src)
	requireValidFrame(bc.dst)
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
