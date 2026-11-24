# Slice 23: Compile-Time Coordinate-Space, Axis, and Affine Type Safety in `pose` and `assembly`

Status: Partly implemented 2026-11-23; stopped after Commit 1
Implementer: Medium (mid-tier) — behavior-preserving, compiler-guided migration to the generic `pose` and `assembly` types specified in this plan across 4 staged commits.
Review: required — architectural and geometric type-system change across `pose` and `assembly`.

## Implementation handoff

- **Who & when** Codex (GPT-6), 2026-11-23.
- **Commit** `:/^Separate geometry points from displacement vectors` resolves to the commit introducing this handoff.
- **Verification** `go vet ./...`, `go test ./...`, and `make pre-commit` passed. `go fix` and `go fmt` left no changes.
- **Next** Commit 2 parameterizes the pose types. No deviations from Commit 1's implementation scope.

## Goal

Upgrade [`internal/pose`](../internal/pose/pose.go) and [`internal/assembly`](../internal/assembly/bike.go) per **Q173** ([`DESIGN.md`](DESIGN.md)) so the Go compiler enforces coordinate-system boundaries, component orientation axes, and affine point-vs-vector distinction at compile time with zero memory overhead and zero change to numerical outputs, URL query parameters, or CLI JSON serialization — laying a type-checked foundation before Phase 2 (`Q159–Q172`) implements static/dynamic build transfers and saddle-to-cockpit coupling.

---

## Rationale

In the current codebase, [`pose.Point`](../internal/pose/pose.go), [`pose.Pose`](../internal/pose/pose.go), and [`pose.Deg`](../internal/pose/pose.go) carry no compile-time information about:

1. **Affine role (position vs. displacement):**
   - `pose.Point` is used for both positions relative to `BB = (0, 0)` (`Pose.Origin()`, `FitTarget.BRP`, `FitTarget.HandGrip`) and displacement vectors (`Point.Sub(Point)`, `Pose.OffsetTo(Pose)`, `pose.Along`, `Fitting.BrpMiss()`, `fitBox` deltas).
   - Every `.Norm()` call in the codebase operates on a displacement vector, never on a position.
   - Because `Point.Sub(Point)` and `Pose.OffsetTo(Pose)` both return `pose.Point` with opposite argument orders (`p.Sub(q) = p − q` vs. `p.OffsetTo(q) = q − p`), callers in `internal/web` currently wrap target points in dummy `pose.NewPose(target, 0)` values just to call `.OffsetTo` or `.DistanceTo`.
   - For upcoming `Q172` (`brpToHbOffset = HB_target − BRP_target`, `coupledHbTarget = BRP_dynamic + brpToHbOffset`), separating `Point[S]` and `Vec[S]` allows `Point[S].Sub(Point[S]) -> Vec[S]` and `Point[S].Add(Vec[S]) -> Point[S]` while making `Point + Point` unrepresentable.

2. **Ambient world space (`UnpitchedChart` vs. `StaticWorld` vs. `DynamicWorld`):**
   - `PlaceUnpitchedFrameset` places landmarks in the chart's unpitched attitude (`0°` frame pitch, axles not level unless fork and cup match the chart basis), whereas `PlaceFrameset` and `PlaceBike` place landmarks in the leveled dynamic world (`BB = (0, 0)`, `+X` along the leveled axle line, `+Y` up).
   - Phase 2 (`Q159–Q172`) adds static workshop placements and measurements (`0%` suspension sag with clamped saddle and handlebar rotations) alongside dynamic placements (at seated suspension sag).
   - Parameterizing points, vectors, headings, poses, and frameset placements by a zero-sized world-space marker `W` prevents mixing unpitched chart coordinates, static workshop coordinates, and dynamic fit coordinates without an explicit transform.

3. **Local component orientation axis (`Axis`) during turtle construction and component transfers:**
   - Every bicycle landmark pose has a physical local orientation (`FrameAxis`, `SeatTubeAxis`, `SaddleRailAxis`, `SteererUpAxis`, `SteererDownAxis`, `ForkRakeAxis`, `StemAxis`, `HandlebarAxis`, `PlumbDownAxis`), already documented in prose on `FramesetPlacement`, `SeatpostPlacement`, `SaddlePlacement`, `StemPlacement`, and `HandlebarPlacement`.
   - With an untyped `pose.Pose` (or a single-parameter `Pose[W]`), all 18 landmarks share one Go type. Nothing prevents:
     - Calling `.Translate(dx, dy)` with a seatpost-shaft displacement (`(-6, +5)` at 20% ShockStop sag) on `SaddlePlacement.BRP` (whose heading is the `+5°` saddle rail rather than the `~105°` seatpost shaft — an `11.97 mm` silent error).
     - Turning `+90°` for fork rake from `HeadTubeTop` (`SteererUpAxis`) instead of `ForkSteererBase` (`SteererDownAxis`).
     - Mounting a stem on `HeadTubeBottom` or `SeatTubeTop` instead of `HeadTubeTop`.
     - Getting the sign of a clamped post-to-rail or stem-to-bar turn backwards (`from − to` vs. `to − from`) when unloading a dynamic build to the static state.
   - **Why `Pose[Axis, W]` keeps turtle chains concise:** In 2D turtle graphics, `.Advance(distMm)` and `.Translate(v Vec[Axis])` move the turtle's origin in `W` while keeping its local orientation `Axis` invariant. Because neither `Axis` nor `W` changes during `Advance` or `Translate`, they remain ordinary dot-chained methods (`p.Advance(...).Translate(...)`). Only `Turn` and `Face` change `Axis`. By typing directed relative turns as `TurnDeg[FromAxis, ToAxis]`, `pose.Turn(p, turnDeg)` and `pose.MeasureTurn(fromPose, toPose)` infer all three type parameters (`FromAxis`, `ToAxis`, `W`) from their arguments with **zero `[...]` type brackets at the call site**.

---

## Instructions for Commentary in `internal/pose`

Add the rationale above directly into [`internal/pose/pose.go`](../internal/pose/pose.go) as package and type documentation so future readers and agents understand why the type parameters exist and how to use them:

1. **Package doc comment on `package pose` (`internal/pose/pose.go`):**
   - Explain the two-level phantom type design:
     - Ambient coordinate spaces `S` / `W` (such as `DynamicWorld`, `StaticWorld`, `UnpitchedChart`) distinguish 2D coordinate systems so positions (`Point[S]`), displacements (`Vec[S]`), and absolute headings (`Heading[W]`) from different load states or reference attitudes cannot be mixed without an explicit `Transform[Src, Dst]`.
     - Local orientation axes `Axis` (such as `SeatTubeAxis`, `SaddleRailAxis`, `SteererUpAxis`) tag which component's invariant local directions (`local +X` along heading, `local +Y` `90°` counter-clockwise) a `Pose[Axis, W]` currently faces.
   - Explain why `Advance` and `Translate` are methods on `Pose[Axis, W]` (translation preserves axis orientation `Axis`), while `Turn` and `Face` are package-level functions that transition `Pose[OldAxis, W]` to `Pose[NewAxis, W]` (Go methods cannot introduce a new type parameter `NewAxis`).
   - Note that phantom marker types are zero-sized empty structs (`struct{}`), adding zero bytes to `Point[S]` (16 bytes), `Vec[S]` (16 bytes), or `Pose[Axis, W]` (24 bytes), and preserving flat `encoding/json` output. Warn callers always to declare marker types as distinct defined types (`type DynamicWorld struct{}`), never type aliases (`type DynamicWorld = struct{}`).
2. **Type and function doc comments in `internal/pose`:**
   - On `Point[S]` and `Vec[S]`: state the affine law (`Point − Point → Vec`, `Point + Vec → Point`, `Vec ± Vec → Vec`; `Point + Point` does not exist, and Euclidean length `Norm()` belongs only to `Vec[S]`). Explain why point-to-point world measurements (`DistanceTo`, `VerticalOffsetTo`, `OffsetTo`, `AngleTo`) live on `Point[S]` rather than `Pose[Axis, W]`: they depend only on the two origins in `S` and not on either landmark's local orientation `Axis`, allowing direct comparison between landmarks with different local axes (such as `BB` in `FrameAxis` and `BRP` in `SaddleRailAxis`) or between a placed landmark origin and a target `Point[S]`.
   - On `Deg`, `Heading[W]`, and `TurnDeg[FromAxis, ToAxis]`: distinguish frame-independent scalar angles in degrees (`Deg`, used for catalog specifications and input fields), absolute orientation headings in world `W` (`Heading[W]`, consumed by `Face`), and directed relative rotations from `FromAxis` to `ToAxis` (`TurnDeg[FromAxis, ToAxis]`, produced by `MeasureTurn` or component angle helpers and consumed by `Turn`). Explain how `MeasureTurn(from, to)` and `Turn(p, delta)` prevent `from − to` vs. `to − from` sign inversions by encoding the turn direction in `[FromAxis, ToAxis]`.
   - On `Transform[Src, Dst]`, `Inverse()`, `Compose()`, and `Pose[Axis, W].Transform()`: state the 2D rigid transformation law (`p_dst = R(heading_in_dst) * p_src + origin_in_dst` for points; rotation-only `v_dst = R(heading_in_dst) * v_src` for vectors) and explain why `p.Transform()` is a zero-argument method returning `Transform[Axis, W]` rather than a free-form cast to an arbitrary local frame.

---

## Full Implementation Sketches

### 1. `internal/pose/pose.go`

```go
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
//     alias `= struct{}`). Values from different spaces cannot be combined
//     unless mapped through an explicit Transform[Src, Dst].
//
//  3. Turtle orientation-axis tracking (Pose[Axis, W]): a Pose[Axis, W] places
//     a turtle at an Origin() in ambient space W whose HeadingDeg() defines the
//     local orientation Axis (+X along HeadingDeg(), +Y 90° counter-clockwise).
//     Moving the turtle via Advance(distMm) or Translate(Vec[Axis]) changes its
//     origin in W while keeping its local orientation Axis invariant, so both
//     remain dot-chained methods on Pose[Axis, W] and Translate rejects vectors
//     from any other component's local frame. Only Turn and Face change Axis;
//     because Go methods cannot introduce a new type parameter, Turn and Face
//     are package-level functions returning Pose[NewAxis, W].
//
// Because empty struct markers occupy 0 bytes, Point[S] (16 bytes), Vec[S]
// (16 bytes), and Pose[Axis, W] (24 bytes) have zero memory overhead and
// serialize to the same flat JSON as untyped structs.
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
// type parameters without brackets and makes reversing the sign of a measured
// turn (from − to vs. to − from) a compile-time error.
type TurnDeg[FromAxis, ToAxis any] float64

// Deg returns d as a raw scalar degree value.
func (d TurnDeg[FromAxis, ToAxis]) Deg() Deg {
	return Deg(d)
}

// Point is a 2D position in millimeters in coordinate space S.
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
	return math.Hypot(other.X-p.X, other.Y-p.Y)
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
	return Heading[S](degFromRad(math.Atan2(other.Y-p.Y, other.X-p.X)))
}

// Vec is a 2D displacement vector in millimeters in coordinate space S.
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

// MeasureTurn returns the directed relative rotation from `from` (FromAxis) to
// `to` (ToAxis) in W (`to.Heading() − from.Heading()`).
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
func (p Pose[Axis, W]) Transform() Transform[Axis, W] {
	return NewRigidTransform[Axis, W](p.origin, p.headingDeg)
}

// Transform is a 2D rigid transformation mapping points and vectors from
// coordinate space Src into coordinate space Dst.
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
```

### 2. `internal/pose/primitives.go`

```go
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

// Along returns the displacement vector in W of length lengthMm at heading.
func Along[W any](heading Heading[W], lengthMm float64) Vec[W] {
	return Vec[W]{X: lengthMm * heading.cos(), Y: lengthMm * heading.sin()}
}

// ProjectOnto returns the signed length of v's component along the unit vector
// at heading in W.
func ProjectOnto[W any](v Vec[W], heading Heading[W]) float64 {
	return v.X*heading.cos() + v.Y*heading.sin()
}

// DecomposeAlong returns the coefficients (a, b) with
// v = a·dir(headingA) + b·dir(headingB), where dir(h) is the unit vector at
// heading h in W. ok is false when the headings are parallel or antiparallel
// (|sin(headingB − headingA)| < 1e-12).
func DecomposeAlong[W any](v Vec[W], headingA, headingB Heading[W]) (a, b float64, ok bool) {
	dA := Along(headingA, 1)
	dB := Along(headingB, 1)
	den := cross(dA, dB)
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	return cross(v, dB) / den, cross(dA, v) / den, true
}

func cross[W any](p, q Vec[W]) float64 {
	return p.X*q.Y - p.Y*q.X
}
```

### 3. `internal/assembly` Coordinate Spaces, Orientation Axes, and Component Sketches

#### A. Phantom Spaces and Orientation Axes (in `internal/assembly/frameset.go` or `internal/assembly/spaces.go`)

```go
package assembly

import "bikegeo/internal/pose"

// Ambient 2D world coordinate spaces (0 bytes each; BB = (0, 0)):
//   - UnpitchedChart: chart reference attitude (Reach along +X, Stack along +Y, 0° pitch).
//   - StaticWorld: bike on level ground with suspension at 0% sag (+X along leveled axles, +Y up).
//   - DynamicWorld: bike on level ground with suspension at seated sag (+X along leveled axles, +Y up).
type (
	UnpitchedChart struct{}
	StaticWorld    struct{}
	DynamicWorld   struct{}
)

// Component orientation axes (0 bytes each; local +X along pose heading, local +Y 90° CCW):
//   - FrameAxis: frame reference axis (chart horizontal, pitched by PitchDeg in W).
//   - SeatTubeAxis: up and rearward along the seat tube / seatpost shaft (+X up shaft, +Y rearward perp).
//   - SaddleRailAxis: forward along the saddle rails (+X toward nose, +Y upward perp to rails).
//   - SteererUpAxis: up and rearward along the steerer axis (+X up steerer, +Y rearward perp).
//   - SteererDownAxis: down and forward along the steerer axis (+X down steerer, +Y forward perp).
//   - ForkRakeAxis: forward and up perpendicular to the steerer in the fork rake direction.
//   - StemAxis: along the stem extension from steerer clamp center to bar clamp center.
//   - HandlebarAxis: unrolled handlebar side-view plane at bar clamp (+X forward, +Y along rise/upsweep).
//   - PlumbDownAxis: straight down (-90° in W) from an axle to its tire-ground contact.
type (
	FrameAxis       struct{}
	SeatTubeAxis    struct{}
	SaddleRailAxis  struct{}
	SteererUpAxis   struct{}
	SteererDownAxis struct{}
	ForkRakeAxis    struct{}
	StemAxis        struct{}
	HandlebarAxis   struct{}
	PlumbDownAxis   struct{}
)

// Fixed geometric axis transitions inside a frameset.
const (
	steererUpToDown       pose.TurnDeg[SteererUpAxis, SteererDownAxis] = 180.0
	steererDownToForkRake pose.TurnDeg[SteererDownAxis, ForkRakeAxis]  = 90.0
	steererDownFromUp     pose.TurnDeg[SteererUpAxis, SteererDownAxis] = 180.0
)

func (f FrameSetup) steererUpTurn() pose.TurnDeg[FrameAxis, SteererUpAxis] {
	return pose.TurnDeg[FrameAxis, SteererUpAxis](180.0 - f.HeadTubeAngleDeg)
}

func (f FrameSetup) seatTubeTurn() pose.TurnDeg[FrameAxis, SeatTubeAxis] {
	return pose.TurnDeg[FrameAxis, SeatTubeAxis](180.0 - f.SeatTubeAngleDeg)
}
```

#### B. `FramesetPlacement[W]` and `placeFramesetAtPitch[W]` (`internal/assembly/frameset.go`)

```go
type FramesetPlacement[W any] struct {
	BB               pose.Pose[FrameAxis, W]
	RearAxle         pose.Pose[FrameAxis, W]
	RearTireContact  pose.Pose[PlumbDownAxis, W]
	SeatAxis         pose.Pose[SeatTubeAxis, W]
	SeatTubeTop      pose.Pose[SeatTubeAxis, W]
	HeadTubeTop      pose.Pose[SteererUpAxis, W]
	HeadTubeBottom   pose.Pose[SteererDownAxis, W]
	LowerCupBottom   pose.Pose[SteererDownAxis, W]
	ForkSteererBase  pose.Pose[SteererDownAxis, W]
	FrontAxle        pose.Pose[ForkRakeAxis, W]
	FrontTireContact pose.Pose[PlumbDownAxis, W]
}

func (f FramesetPlacement[W]) PitchDeg() pose.Deg {
	return f.BB.HeadingDeg()
}

func (f FramesetPlacement[W]) LevelingPitchDeg() pose.Deg {
	return -f.RearAxle.Origin().AngleTo(f.FrontAxle.Origin()).Deg()
}

func (f FramesetPlacement[W]) PitchDeltaDeg(reference FramesetPlacement[W]) pose.Deg {
	return f.PitchDeg() - reference.PitchDeg()
}

func (f FramesetPlacement[W]) LeveledHeadTubeAngleDeg() pose.Deg {
	return 180.0 - f.HeadTubeTop.HeadingDeg()
}

func (f FramesetPlacement[W]) LeveledSeatTubeAngleDeg() pose.Deg {
	return 180.0 - f.SeatAxis.HeadingDeg()
}

func (f FramesetPlacement[W]) WheelbaseMm() float64 {
	return f.RearAxle.Origin().DistanceTo(f.FrontAxle.Origin())
}

func (f FramesetPlacement[W]) LeveledBbDropMm() float64 {
	return f.BB.Origin().VerticalOffsetTo(f.RearAxle.Origin())
}

func (f FramesetPlacement[W]) TrailMm() float64 {
	steererDown := pose.Turn(f.HeadTubeTop, steererDownFromUp).Heading()
	_, ahead, ok := pose.DecomposeAlong(f.HeadTubeTop.Origin().OffsetTo(f.FrontTireContact.Origin()), steererDown, 0)
	if !ok {
		return math.NaN()
	}
	return -ahead
}

func (f FramesetPlacement[W]) RearCenterMm() float64 {
	return f.RearAxle.Origin().OffsetTo(f.BB.Origin()).X
}

func (f FramesetPlacement[W]) RcWbRatioPct() float64 {
	return 100.0 * f.RearCenterMm() / f.WheelbaseMm()
}

func (f FramesetPlacement[W]) LeveledReachMm() float64 {
	return f.BB.Origin().OffsetTo(f.HeadTubeTop.Origin()).X
}

func (f FramesetPlacement[W]) LeveledStackMm() float64 {
	return f.BB.Origin().VerticalOffsetTo(f.HeadTubeTop.Origin())
}

func (f FramesetPlacement[W]) EttMm() float64 {
	_, ett, ok := pose.DecomposeAlong(f.SeatAxis.Origin().OffsetTo(f.HeadTubeTop.Origin()), f.SeatAxis.Heading(), 0)
	if !ok {
		return math.NaN()
	}
	return ett
}

func PlaceUnpitchedFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement[UnpitchedChart], error) {
	if err := validateFramesetInputs(frame, lowerCup, fork, wheels); err != nil {
		return FramesetPlacement[UnpitchedChart]{}, err
	}
	return placeFramesetAtPitch[UnpitchedChart](0, frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
}

func PlaceFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement[DynamicWorld], error) {
	unpitched, err := PlaceUnpitchedFrameset(frame, lowerCup, fork, wheels)
	if err != nil {
		return FramesetPlacement[DynamicWorld]{}, err
	}
	pitch := pose.Heading[DynamicWorld](unpitched.LevelingPitchDeg())
	return placeFramesetAtPitch[DynamicWorld](pitch, frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
}

func placeFramesetAtPitch[W any](pitchDeg pose.Heading[W], frame FrameSetup, lowerCupMm float64, fork ForkGeometry, wheels WheelSetup) FramesetPlacement[W] {
	frameRoot := pose.NewPose[FrameAxis](pose.Point[W]{}, pitchDeg)
	rearX := -pose.RightTriangleLeg(frame.ChainstayMm, frame.BbDropMm)
	rearAxle := frameRoot.Translate(pose.Vec[FrameAxis]{X: rearX, Y: frame.BbDropMm})

	headTubeTop := pose.Turn(
		frameRoot.Translate(pose.Vec[FrameAxis]{X: frame.ReachMm, Y: frame.StackMm}),
		frame.steererUpTurn(),
	)
	steererDown := pose.Turn(headTubeTop, steererUpToDown)
	headTubeBottom := steererDown.Advance(frame.HeadTubeLengthMm)
	lowerCupBottom := headTubeBottom.Advance(lowerCupMm)
	forkSteererBase := lowerCupBottom.Advance(fork.A2cMm)
	frontAxle := pose.Turn(forkSteererBase, steererDownToForkRake).Advance(fork.RakeMm)

	seatAxis := pose.Turn(frameRoot, frame.seatTubeTurn())
	seatTubeTop := seatAxis.Advance(frame.SeatTubeLengthMm)

	rearTireContact := pose.Face[PlumbDownAxis](rearAxle, -90.0).Advance(wheels.RadiusMm)
	frontTireContact := pose.Face[PlumbDownAxis](frontAxle, -90.0).Advance(wheels.RadiusMm)

	return FramesetPlacement[W]{
		BB:               frameRoot,
		RearAxle:         rearAxle,
		RearTireContact:  rearTireContact,
		SeatAxis:         seatAxis,
		SeatTubeTop:      seatTubeTop,
		HeadTubeTop:      headTubeTop,
		HeadTubeBottom:   headTubeBottom,
		LowerCupBottom:   lowerCupBottom,
		ForkSteererBase:  forkSteererBase,
		FrontAxle:        frontAxle,
		FrontTireContact: frontTireContact,
	}
}
```

#### C. `PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, `PlaceHandlebar`, and `BikePlacement` (`seatpost.go`, `saddle.go`, `stem.go`, `handlebar.go`, `bike.go`)

```go
// --- seatpost.go ---
func (p SeatpostSpec) setbackOffset() pose.Vec[SeatTubeAxis] {
	return pose.Vec[SeatTubeAxis]{X: 0, Y: p.SetbackMm}
}

type SeatpostPlacement struct {
	SeatTubeTop pose.Pose[SeatTubeAxis, DynamicWorld]
	ShaftTop    pose.Pose[SeatTubeAxis, DynamicWorld]
	RailCradle  pose.Pose[SeatTubeAxis, DynamicWorld]
}

func PlaceSeatpost(seatTubeTop pose.Pose[SeatTubeAxis, DynamicWorld], frame FrameSetup, setup SeatpostSetup) (SeatpostPlacement, error) {
	// ... existing validation ...
	shaftTop := seatTubeTop.Advance(setup.ExposedMm())
	railCradle := shaftTop.Translate(setup.Spec.setbackOffset())
	// ...
}

// --- saddle.go ---
func (s SaddleSetup) brpOffset() pose.Vec[SaddleRailAxis] {
	return pose.Vec[SaddleRailAxis]{X: s.RailCenterToBrpMm, Y: s.RailToBrpHeightMm}
}

func (s SaddleSetup) sitBonesOffset() pose.Vec[SaddleRailAxis] {
	return pose.Vec[SaddleRailAxis]{X: s.RailCenterToSitBonesMm, Y: s.RailToBrpHeightMm + s.SitBonesElevationMm}
}

type SaddlePlacement struct {
	RailCradle pose.Pose[SeatTubeAxis, DynamicWorld]
	RailFrame  pose.Pose[SaddleRailAxis, DynamicWorld]
	RailCenter pose.Pose[SaddleRailAxis, DynamicWorld]
	BRP        pose.Pose[SaddleRailAxis, DynamicWorld]
	SitBones   pose.Pose[SaddleRailAxis, DynamicWorld]
}

func PlaceSaddle(railCradle pose.Pose[SeatTubeAxis, DynamicWorld], clampLengthMm float64, setup SaddleSetup) (SaddlePlacement, error) {
	// ... existing validation ...
	railFrame := pose.Face[SaddleRailAxis](railCradle, pose.Heading[DynamicWorld](setup.RailPitchDeg))
	railCenter := railFrame.Advance(-setup.RailOffsetMm)
	brp := railCenter.Translate(setup.brpOffset())
	sitBones := railCenter.Translate(setup.sitBonesOffset())
	// ...
}

// --- stem.go ---
func (s StemSetup) steererToStemTurn() pose.TurnDeg[SteererUpAxis, StemAxis] {
	return pose.TurnDeg[SteererUpAxis, StemAxis](-90.0 + s.AngleDeg)
}

type StemPlacement struct {
	HeadTubeTop  pose.Pose[SteererUpAxis, DynamicWorld]
	CoverTop     pose.Pose[SteererUpAxis, DynamicWorld]
	SpacerTop    pose.Pose[SteererUpAxis, DynamicWorld]
	SteererClamp pose.Pose[SteererUpAxis, DynamicWorld]
	BarClamp     pose.Pose[StemAxis, DynamicWorld]
}

func PlaceStem(steererUp pose.Pose[SteererUpAxis, DynamicWorld], setup StemSetup) (StemPlacement, error) {
	// ... existing validation ...
	coverTop := steererUp.Advance(setup.UpperStackMm)
	spacerTop := coverTop.Advance(setup.SpacersMm)
	steererClamp := spacerTop.Advance(setup.ClampHeightMm / 2.0)
	barClamp := pose.Turn(steererClamp, setup.steererToStemTurn()).Advance(setup.LengthMm)
	// ...
}

// --- handlebar.go ---
func (h HandlebarSetup) outboardSweepOffset(zHand float64) pose.Vec[HandlebarAxis] {
	sweptSpan := max(0.0, zHand-h.BendStartMm)
	return pose.Vec[HandlebarAxis]{
		X: -pose.DeflectionOverSpan(sweptSpan, h.BacksweepDeg),
		Y:  pose.DeflectionOverSpan(sweptSpan, h.UpsweepDeg),
	}
}

type HandlebarPlacement struct {
	BarClamp      pose.Pose[StemAxis, DynamicWorld]
	BarRollFrame  pose.Pose[HandlebarAxis, DynamicWorld]
	HandGrip      pose.Pose[HandlebarAxis, DynamicWorld]
	HandLateralMm float64
}

func PlaceHandlebar(barClamp pose.Pose[StemAxis, DynamicWorld], setup HandlebarSetup) (HandlebarPlacement, error) {
	// ... existing validation ...
	barRollFrame := pose.Face[HandlebarAxis](barClamp, pose.Heading[DynamicWorld](-setup.BarRollDeg))
	bendRise := barRollFrame.Translate(pose.Vec[HandlebarAxis]{X: 0, Y: baseRise})
	handGrip := bendRise.Translate(setup.outboardSweepOffset(zHand))
	// ...
}

// --- bike.go ---
type BikePlacement struct {
	Setup     BikeSetup
	Frameset  FramesetPlacement[DynamicWorld]
	Seatpost  SeatpostPlacement
	Saddle    SaddlePlacement
	Stem      StemPlacement
	Handlebar HandlebarPlacement
}

func (b BikePlacement) SeatAxisToPostTopMm() float64 {
	return b.Frameset.SeatAxis.Origin().DistanceTo(b.Seatpost.ShaftTop.Origin())
}

func (b BikePlacement) SaddleHeightMm() float64 {
	return b.Frameset.BB.Origin().DistanceTo(b.Saddle.BRP.Origin())
}

func (b BikePlacement) BrpToGripDistMm() float64 {
	return b.Saddle.BRP.Origin().DistanceTo(b.Handlebar.HandGrip.Origin())
}

func (b BikePlacement) SitBonesToGripRiseMm() float64 {
	return b.Saddle.SitBones.Origin().VerticalOffsetTo(b.Handlebar.HandGrip.Origin())
}
```

#### D. `fitting.go`, `scoring.go`, and `build_fitting.go` Sketches (`Point[DynamicWorld]` vs. `Vec[DynamicWorld]`)

```go
// --- fitting.go ---
type FitTarget struct {
	BRP      *pose.Point[DynamicWorld]
	HandGrip *pose.Point[DynamicWorld]
}

func (f Fitting) BrpMiss() (pose.Vec[DynamicWorld], bool) {
	if f.Target.BRP == nil {
		return pose.Vec[DynamicWorld]{}, false
	}
	return f.Target.BRP.Sub(f.Placement.Saddle.BRP.Origin()), true
}

func (f Fitting) GripMiss() (pose.Vec[DynamicWorld], bool) {
	if f.Target.HandGrip == nil {
		return pose.Vec[DynamicWorld]{}, false
	}
	return f.Target.HandGrip.Sub(f.Placement.Handlebar.HandGrip.Origin()), true
}

// fitAxis, fitEdge, fitPriority, and fitBox operate on pose.Vec[DynamicWorld] and pose.Heading[DynamicWorld]:
type fitAxis struct {
	heading pose.Heading[DynamicWorld]
	fitInterval
	field InputField
}

func (r fitInterval) heightFirst(target, direction pose.Vec[DynamicWorld]) float64 { ... }
func (edge fitEdge) candidates(delta pose.Vec[DynamicWorld], priority fitPriority) []float64 { ... }
func (p fitPriority) miss(delta pose.Vec[DynamicWorld], axes [2]fitAxis, travel [2]float64) pose.Vec[DynamicWorld] { ... }
func (p fitPriority) prefers(miss, best pose.Vec[DynamicWorld]) bool { ... }
func fitBox(delta pose.Vec[DynamicWorld], a, b fitAxis, priority fitPriority) (da, db float64, limited []InputField, ok bool) { ... }

// --- scoring.go ---
func (o FitOval) NormalizedRadius(center, p pose.Point[DynamicWorld]) float64 {
	return newGripOvalProjection(o).normalizedRadius(center, p)
}

type gripOvalProjection struct {
	minorDir, majorDir       pose.Vec[DynamicWorld]
	semiMinorMm, semiMajorMm float64
}

func newGripOvalProjection(o FitOval) gripOvalProjection {
	h := pose.Heading[DynamicWorld](o.MajorAxisHeadingDeg)
	return gripOvalProjection{
		minorDir:    pose.Along(h.Turn(-90), 1),
		majorDir:    pose.Along(h, 1),
		semiMinorMm: o.SemiMinorMm,
		semiMajorMm: o.SemiMajorMm,
	}
}

func (p gripOvalProjection) normalizedRadius(center, pt pose.Point[DynamicWorld]) float64 {
	d := pt.Sub(center)
	minor := (d.X*p.minorDir.X + d.Y*p.minorDir.Y) / p.semiMinorMm
	major := (d.X*p.majorDir.X + d.Y*p.majorDir.Y) / p.semiMajorMm
	return pose.Vec[DynamicWorld]{X: minor, Y: major}.Norm()
}

// --- build_fitting.go ---
type ScoredBuild struct {
	Placement          BikePlacement
	Target             pose.Point[DynamicWorld]
	HasHandlebarOption bool
	HandlebarIndex     int
	HasForkOption      bool
	ForkIndex          int
	BrpTarget          *pose.Point[DynamicWorld]
	Score              BuildScore
}

func (b ScoredBuild) GripMiss() pose.Vec[DynamicWorld] {
	return b.Target.Sub(b.Placement.Handlebar.HandGrip.Origin())
}

func (b ScoredBuild) BrpMiss() (pose.Vec[DynamicWorld], bool) {
	if b.BrpTarget == nil {
		return pose.Vec[DynamicWorld]{}, false
	}
	return b.BrpTarget.Sub(b.Placement.Saddle.BRP.Origin()), true
}
```

#### E. Outer Package Sketches (`internal/catalog`, `internal/query`, `internal/web`)

- **[`internal/catalog/catalog.go`](../internal/catalog/catalog.go):**
  ```go
  func DefaultBrpTarget() pose.Point[assembly.DynamicWorld]  { return pose.Point[assembly.DynamicWorld]{X: -219.0, Y: 713.1} }
  func DefaultGripTarget() pose.Point[assembly.DynamicWorld] { return pose.Point[assembly.DynamicWorld]{X: 481.0, Y: 726.0} }
  ```
- **[`internal/catalog/chart_consistency_test.go`](../internal/catalog/chart_consistency_test.go):**
  ```go
  func chartMismatches(unpitched assembly.FramesetPlacement[assembly.UnpitchedChart], publishedWheelbaseMm float64) [2]float64 {
  	return [2]float64{
  		unpitched.RearAxle.Origin().VerticalOffsetTo(unpitched.FrontAxle.Origin()),
  		(unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - publishedWheelbaseMm,
  	}
  }
  ```
- **[`internal/web/bike_page.go`](../internal/web/bike_page.go) (lines 523, 539):**
  Replace `miss := bike.Saddle.BRP.OffsetTo(pose.NewPose(*brp, 0))` with direct `miss := bike.Saddle.BRP.Origin().OffsetTo(*brp)` (or `brp.Sub(bike.Saddle.BRP.Origin())`), deleting the dummy `pose.NewPose(..., 0)` allocation.
- **[`internal/web/diagram.go`](../internal/web/diagram.go):**
  - `toSVG(p pose.Point[assembly.DynamicWorld]) svgPoint`
  - `poseOutline` precomputes its `svgRect` and 4 `[]svgPoint` corners via a generic constructor `newPoseOutline[Axis any](name string, p pose.Pose[Axis, assembly.DynamicWorld], startMm, lengthMm, thicknessMm float64) poseOutline` (using `p.Translate(pose.Vec[Axis]{X: along, Y: across}).Origin()`), allowing `stemParts` (`SteererUpAxis` + `StemAxis`) and `seatParts` (`SaddleRailAxis`) to live in uniform `[]poseOutline` slices without erasing `Axis` in `assembly`.
  - `makeSegment` and `bikeLandmarkSpec` take `pose.Point[assembly.DynamicWorld]` (each landmark's `.Origin()`) rather than `pose.Pose`, since neither segments nor dot markers use the landmark's heading.
  - Screen-pixel distances in `placeDiagramLabels` use a local `screenSpace struct{}` marker (`pose.Vec[screenSpace]{X: nx - px, Y: ny - py}.Norm()`) so screen-pixel coordinates are also type-isolated from physical millimeter `DynamicWorld` vectors.

---

## Staged Commit Plan (with Temporary Escape Hatches)

Each commit below compiles cleanly and passes `make pre-commit` (`go fix`, `go fmt`, `go vet`, `make boundary`, `exhaustive`, `go test ./...`).

- **One-commit-per-session rule:** Stop after completing and committing **one** commit from the staged plan below (or after landing an isolated bug-fix commit). In that commit, update `SLICE23.md`'s header to `Status: Partly implemented <YYYY-MM-DD>; stopped after Commit <k>` (or `Status: Implemented <YYYY-MM-DD>; awaiting review` on Commit 4) and update `## Implementation handoff` with the commit hash and verification status, then stop and wait for Matt before starting the next commit.
- **Bug-isolation rule:** Every type-safety commit in this slice must be strictly behavior-preserving. If applying the new types exposes a real bug in existing code or tests (for example, a cross-frame or wrong-axis operation, a reversed offset sign, or an unpitched vs. leveled mix-up), **do not fix the bug inside the type-safety commit**. Instead, land the bug fix and its regression test in an isolated standalone commit first (passing `make pre-commit`), and then apply the pure type-safety commit on top.

### Commit 1: Separate Affine `pose.Point` (Position) and `pose.Vec` (Displacement), and Move Point-to-Point Measurements onto `Point`
- **Scope:** [`internal/pose`](../internal/pose/pose.go), [`internal/assembly`](../internal/assembly/fitting.go), [`internal/web`](../internal/web/diagram.go), and their tests (unparameterized `Point` and `Vec` first).
- **Changes:**
  - Add `pose.Vec` (`Add`, `Sub`, `Scale`, `Neg`, `Norm`); change `Point.Sub(Point)` to return `Vec`; add `Point.Add(Vec) Point`; remove `Point.Norm()`.
  - Move `DistanceTo`, `VerticalOffsetTo`, `OffsetTo(other Point) Vec`, and `AngleTo(other Point) Deg` onto `pose.Point` (removing them from `pose.Pose` and updating callers to call `.Origin().DistanceTo(...)`, `.Origin().VerticalOffsetTo(...)`, etc., which also removes the fake `pose.NewPose(target, 0)` wrappers in `bike_page.go` and `diagram.go`).
  - Change `pose.Along(...) -> Vec`, `pose.ProjectOnto(v Vec, ...)`, and `pose.DecomposeAlong(v Vec, ...)`.
  - Update `Fitting.BrpMiss()`, `Fitting.GripMiss()`, `ScoredBuild.GripMiss()`, `ScoredBuild.BrpMiss()`, `fitBox`, and `gripOvalProjection` to use `pose.Vec`.
- **Verification:** `make pre-commit`.

### Commit 2: Parameterize `internal/pose` (`Point[S]`, `Vec[S]`, `Heading[W]`, `TurnDeg[From, To]`, `Pose[Axis, W]`, `Transform[Src, Dst]`) with Temporary Same-Axis Bridges
- **Scope:** [`internal/pose`](../internal/pose/pose.go) (+ commentary and `pose_test.go`), plus mechanical type-argument wiring in `assembly`, `catalog`, `query`, and `web`.
- **Changes:**
  - Parameterize `Point[S]`, `Vec[S]`, `Heading[W]`, `TurnDeg[FromAxis, ToAxis]`, `Pose[Axis, W]`, and add `Transform[Src, Dst]` (`NewRigidTransform`, `ApplyPoint`, `ApplyVec`, `Inverse`, `Compose`, and `Pose[Axis, W].Transform()`).
  - Add the full package and type doc commentary in `internal/pose/pose.go` and `internal/pose/primitives.go`.
  - **Temporary escape hatches on `Pose[Axis, W]`** (marked `// Temporary Slice 23 bridge; removed in Commit 4`):
    - `func (p Pose[Axis, W]) TurnSameAxis(delta Deg) Pose[Axis, W]`
    - `func (p Pose[Axis, W]) FaceSameAxis(worldHeading Deg) Pose[Axis, W]`
  - In `internal/assembly`, define `UnpitchedChart`, `StaticWorld`, `DynamicWorld`, and `FrameAxis`, and temporarily keep all placement poses at `pose.Pose[FrameAxis, DynamicWorld]` using `TurnSameAxis` / `FaceSameAxis` while migrating all `Point` and `Vec` references across `assembly`, `catalog`, `query`, and `web` to `pose.Point[assembly.DynamicWorld]` and `pose.Vec[assembly.DynamicWorld]`.
  - Expand `internal/pose/pose_test.go` to test zero memory overhead (`unsafe.Sizeof`), `Turn`/`MeasureTurn`/`Face`, `Transform`/`Inverse`/`Compose` round-trips, and `MarshalJSON`.
- **Verification:** `make pre-commit`.

### Commit 3: Migrate Component Builders (`PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, `PlaceHandlebar`) and `diagram.go` to Typed Orientation Axes
- **Scope:** [`internal/assembly/seatpost.go`](../internal/assembly/seatpost.go), [`saddle.go`](../internal/assembly/saddle.go), [`stem.go`](../internal/assembly/stem.go), [`handlebar.go`](../internal/assembly/handlebar.go), [`internal/web/diagram.go`](../internal/web/diagram.go), and component tests.
- **Changes:**
  - Define `SeatTubeAxis`, `SaddleRailAxis`, `SteererUpAxis`, `SteererDownAxis`, `ForkRakeAxis`, `StemAxis`, `HandlebarAxis`, and `PlumbDownAxis` in `internal/assembly`.
  - Update `SeatpostPlacement`, `SaddlePlacement`, `StemPlacement`, and `HandlebarPlacement` fields and `PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, and `PlaceHandlebar` signatures to use their exact `pose.Pose[Axis, DynamicWorld]` types.
  - Replace `TurnSameAxis` and `FaceSameAxis` in those four builders with `pose.Turn(steererClamp, setup.steererToStemTurn())` and `pose.Face[SaddleRailAxis]` / `pose.Face[HandlebarAxis]`, and use typed local offset helpers (`Vec[SeatTubeAxis]`, `Vec[SaddleRailAxis]`, `Vec[HandlebarAxis]`).
  - Update `internal/web/diagram.go` (`newPoseOutline[Axis]`, point-based `makeSegment` and `bikeLandmarkSpec`).
- **Verification:** `make pre-commit`.

### Commit 4: Migrate `FramesetPlacement[W]` (`UnpitchedChart` vs. `DynamicWorld`), `placeFramesetAtPitch[W]`, and Delete All Temporary Escape Hatches
- **Scope:** [`internal/assembly/frameset.go`](../internal/assembly/frameset.go), [`validate.go`](../internal/assembly/validate.go), [`internal/catalog/chart_consistency_test.go`](../internal/catalog/chart_consistency_test.go), [`internal/pose/pose.go`](../internal/pose/pose.go), and [`docs/SLICE23.md`](SLICE23.md).
- **Changes:**
  - Parameterize `FramesetPlacement[W]` over `W` (`PlaceUnpitchedFrameset` returns `FramesetPlacement[UnpitchedChart]`; `PlaceFrameset` returns `FramesetPlacement[DynamicWorld]`), with typed landmark axes (`FrameAxis`, `SeatTubeAxis`, `SteererUpAxis`, `SteererDownAxis`, `ForkRakeAxis`, `PlumbDownAxis`).
  - Update `placeFramesetAtPitch[W]` to use `pose.Turn` with `frame.steererUpTurn()`, `steererUpToDown`, `steererDownToForkRake`, `frame.seatTubeTurn()`, and `pose.Face[PlumbDownAxis]`.
  - Delete the temporary escape hatches (`TurnSameAxis`, `FaceSameAxis`) from `internal/pose/pose.go`.
  - Fold the 3–5 line `## Implementation handoff` into `docs/SLICE23.md`.
- **Verification:** `make pre-commit`.

---

## Behavior-Preserving Test Cases

1. **Zero memory overhead (`internal/pose/pose_test.go`):**
   - Compare `unsafe.Sizeof(pose.Point[testWorld]{})` and `unsafe.Sizeof(pose.Vec[testWorld]{})` against `unsafe.Sizeof(struct{ X, Y float64 }{})`.
   - Compare `unsafe.Sizeof(pose.Pose[testAxis, testWorld]{})` against `unsafe.Sizeof(struct{ origin struct{ X, Y float64 }; headingDeg pose.Deg }{})`.
2. **Affine `Point[S]` and `Vec[S]` operations (`internal/pose/pose_test.go`):**
   - `p.Sub(q)` returns `Vec[S]`; `q.Add(p.Sub(q))` recovers `p`.
   - `Vec[S]` addition, subtraction, scalar multiplication (`Scale`), negation (`Neg`), and `Norm()`.
   - `Point[S].DistanceTo`, `VerticalOffsetTo`, `OffsetTo`, and `AngleTo` match the previous `Pose` methods to `1e-12`.
3. **Typed turtle transitions and `MeasureTurn` / `Turn` round-trip (`internal/pose/pose_test.go`):**
   - `pose.Turn(p, delta)` transitions `Pose[FromAxis, W]` to `Pose[ToAxis, W]` without explicit type brackets.
   - `pose.Face[NewAxis](p, heading)` sets the absolute heading in `W` and transitions to `Pose[NewAxis, W]`.
   - `clamped := pose.MeasureTurn(fromPose, toPose)` followed by `pose.Turn(otherFromPose, clamped)` reproduces the exact relative angle delta across a frame pitch change.
4. **Rigid `Transform[Src, Dst]`, `Inverse()`, `Compose()`, and `Pose.Transform()` (`internal/pose/pose_test.go`):**
   - `p.Transform().ApplyPoint(Point[Axis]{X: dx, Y: dy})` equals `p.Translate(Vec[Axis]{X: dx, Y: dy}).Origin()` to `< 1e-12 mm`.
   - `t.Inverse().ApplyPoint(t.ApplyPoint(pt))` and `t.Inverse().ApplyVec(t.ApplyVec(v))` recover `pt` and `v` to `< 1e-12 mm`.
   - 3-space composition `Compose(railToPost, postToWorld).ApplyPoint(brpInRail)` matches the turtle chain `seatTubeTop.Advance(...).Translate(...).Face(...).Advance(...).Translate(...)` to `< 1e-12 mm`.
   - Cross-state saddle transfer `Compose(dynamicCradle.Transform().Inverse(), staticCradle.Transform())` matches walking the static bike with `pose.Turn(staticCradle, clampedRailTurn)` to `< 1e-12 mm`.
5. **JSON serialization parity (`internal/pose/pose_test.go`, `cmd/bikegeo/main_test.go`):**
   - `json.Marshal` on `Pose[Axis, W]` produces `{"Origin":{"X":-219,"Y":713.1},"HeadingDeg":104.75}` and still rejects `NaN` and `Inf` origin coordinates or headings.
   - `bikegeo compare` and `bikegeo bike` CLI tests pass with zero JSON output changes.
6. **Full repository regression suite (`make pre-commit`):**
   - Every existing test in `internal/pose`, `internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, `internal/web`, and `cmd/bikegeo` passes without changing any expected coordinate, fit outcome, tier, or score.

---

## Out of Scope

- Phase 2 physical and fitting model changes (`Q159–Q172`: Redshift ShockStop static/dynamic split, seated fork sag per travel class, fixed dynamic handlebar roll, SQlab 610 `+5°` rail pitch, static/dynamic target conversion UI, workshop proximity warnings, and `coupledHbTarget` saddle-to-cockpit coupling).
- Any changes to URL query parameters, HTML/SVG output, or CLI JSON schema.
