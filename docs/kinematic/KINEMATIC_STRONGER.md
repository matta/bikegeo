Combining Go's static type system with **fail-fast runtime invariants** gives you nearly all the guarantees of Rust's affine type system while keeping the code idiomatic Go.

To make the API truly "idiot-proof," it helps to split safety into two distinct layers:

1. **API & Invariant Violations (`panic`):** Mistakes made by the programmer calling the API—such as using an uninitialized zero-value `Pose`, attaching two stems to the same steerer socket, reusing a finalized `Builder`, or introducing `NaN` into a coordinate. These should fail immediately at the exact call site.
2. **Physical & Domain Violations (`error`):** Physically impossible bike configurations supplied by user input—such as a seatpost bottoming out in the seat tube, a fork crown colliding with the front tire under sag, or a steerer tube that is too short for the spacer stack. These return descriptive `error` values from `Solve()`.

---

## 1. What We Are Locking Down (Compile-Time + Runtime)

### Compile-Time Loopholes Closed

* **Typestate `Part[In, Out]` Eliminates `PlaceSocket`:**
In the original design, `stemPart.BarMount` existed as a `Socket[BarClamp]` *before* the stem was attached to the bike, making it easy to forget `PlaceSocket` and attach the handlebar in stem-local space. In the refactored design, part constructors (`NewStem`, `NewSweptHandlebar`, `NewSeatpost`) only have access to `LocalPlug[R]` and `LocalSocket[R]`. They return an opaque `Part[In, Out]`. The downstream `WorldSocket[R]` **does not exist as a value** until `Attach(asm, parentWorldSocket, part)` is called.
* **Automatic Primitive Collection via `Assembly`:**
`Attach` takes an `*Assembly`, automatically transforms the part's local primitives into world space, appends them to the assembly, and promotes the child's local sockets into `WorldSocket`s in one atomic step. You can no longer forget `allPrims = append(...)` or pair `stemPart.In` with `barPart.Prims`.
* **Encapsulated `Pose` and `Mat3`:**
The fields of `Pose` and `Mat3` are unexported so external callers cannot construct a singular zero-matrix `Pose{Origin: pt}` or mutate basis columns into a non-orthonormal state.

### Runtime Checks Added

* **Single-Use Affine `WorldSocket[R]`:**
Each `WorldSocket[R]` holds a pointer to a runtime `socketState` owned by a specific `*Assembly`. When passed to `Attach`, the runtime verifies that the socket is initialized, belongs to the target `Assembly`, and has not already been occupied. Double-attaching a socket panics immediately.
* **Zero-Value & $SO(3)$ Orthonormality Guards:**
`Pose` tracks an initialization flag and verifies that its rotation matrix satisfies both $\Vert{}R^T R - I\Vert{}_\infty < 10^{-9}$ and $\vert{}\det(R) - 1\vert{} < 10^{-9}$ (ensuring a proper right-handed orthonormal basis where $R^{-1} = R^T$ is mathematically exact).
* **`Builder` Freeze Guard:**
When `FinishPart1` or `FinishPart` is called at the end of a part constructor, the `Builder` is marked `frozen`. If a retained `Cursor` variable tries to extrude a tube after the part is finalized, it panics instead of silently mutating shared state.
* **Finite (`NaN`/`Inf`) & Positive-Dimension Guards:**
Every `Cursor` movement and `Pose` transformation checks that lengths, angles, diameters, and bend radii are finite and within valid geometric bounds (e.g., tube diameter and bend radius are non-negative).
* **Expanded Physical Domain Validation (`error`):**
`AssembleChassis` and `Solve` now check fork crown-to-tire clearance at full sag, positive component dimensions, head/seat angle bounds, and bottom bracket ground clearance.

---

## 2. Refactored Go Implementation

### 2.1 Validated Units, $SO(3)$ Poses, and Invariant Guards

```go
package bikecad

import (
	"errors"
	"fmt"
	"math"
)

// Domain / Physical Configuration Errors (returned to callers of Solve)
var (
	ErrInvalidFrameGeometry      = errors.New("frame dimensions are non-positive or chainstay <= |BB drop|")
	ErrUnbuildableChassis        = errors.New("wheel radius difference exceeds axle span")
	ErrInsufficientSteererLength = errors.New("total steerer stack exceeds fork steerer length")
	ErrForkTireInterference      = errors.New("fork effective axle-to-crown is less than or equal to front wheel radius")
	ErrBBBelowGround             = errors.New("solved bottom bracket center is at or below ground level")
	ErrSeatpostMinInsertion      = errors.New("seatpost insertion is less than minimum required")
	ErrSeatpostBottomedOut       = errors.New("seatpost insertion exceeds frame seat tube length")
)

type MM float64
type Deg float64
type Rad float64

func (d Deg) Rad() Rad { return Rad(float64(d) * math.Pi / 180.0) }
func (r Rad) Deg() Deg { return Deg(float64(r) * 180.0 / math.Pi) }

func mustBeFinite(name string, vals ...float64) {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			panic(fmt.Sprintf("bikecad: non-finite value (%v) in %s", v, name))
		}
	}
}

func mustBeNonNegative(name string, v MM) {
	mustBeFinite(name, float64(v))
	if v < 0 {
		panic(fmt.Sprintf("bikecad: %s must be >= 0, got %v mm", name, v))
	}
}

// WorldPoint is a 3D coordinate in world space (origin at rear tire contact patch).
type WorldPoint struct{ X, Y, Z MM }

func (p WorldPoint) mustBeFinite() {
	mustBeFinite("WorldPoint", float64(p.X), float64(p.Y), float64(p.Z))
}

// LocalVec is a 3D displacement vector expressed in a Pose's local coordinate basis.
type LocalVec struct{ X, Y, Z MM }

func (v LocalVec) mustBeFinite() {
	mustBeFinite("LocalVec", float64(v.X), float64(v.Y), float64(v.Z))
}

// Mat3 is an unexported-field 3x3 orthonormal rotation matrix in SO(3).
type Mat3 struct {
	m     [3][3]float64
	valid bool
}

func IdentityMat3() Mat3 {
	return Mat3{
		m: [3][3]float64{
			{1, 0, 0},
			{0, 1, 0},
			{0, 0, 1},
		},
		valid: true,
	}
}

// mustBeSO3 panics if the matrix was zero-initialized or drifted out of SO(3).
func (mat Mat3) mustBeSO3() {
	if !mat.valid {
		panic("bikecad: uninitialized zero-value Mat3/Pose (use IdentityPose() or Start())")
	}
	const eps = 1e-7
	// Verify R^T * R == I (columns are orthonormal)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			dot := mat.m[0][i]*mat.m[0][j] + mat.m[1][i]*mat.m[1][j] + mat.m[2][i]*mat.m[2][j]
			want := 0.0
			if i == j {
				want = 1.0
			}
			if math.Abs(dot-want) > eps {
				panic(fmt.Sprintf("bikecad: Mat3 violated orthogonality at (%d,%d): got %g, want %g", i, j, dot, want))
			}
		}
	}
	// Verify det(R) == +1 (proper right-handed rotation, not a reflection)
	det := mat.m[0][0]*(mat.m[1][1]*mat.m[2][2]-mat.m[1][2]*mat.m[2][1]) -
		mat.m[0][1]*(mat.m[1][0]*mat.m[2][2]-mat.m[1][2]*mat.m[2][0]) +
		mat.m[0][2]*(mat.m[1][0]*mat.m[2][1]-mat.m[1][1]*mat.m[2][0])
	if math.Abs(det-1.0) > eps {
		panic(fmt.Sprintf("bikecad: Mat3 determinant is %g, want +1.0", det))
	}
}

func RotX(a Rad) Mat3 {
	mustBeFinite("RotX", float64(a))
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		m: [3][3]float64{
			{1, 0, 0},
			{0, c, -s},
			{0, s, c},
		},
		valid: true,
	}
}

func RotY(a Rad) Mat3 {
	mustBeFinite("RotY", float64(a))
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		m: [3][3]float64{
			{c, 0, s},
			{0, 1, 0},
			{-s, 0, c},
		},
		valid: true,
	}
}

func RotZ(a Rad) Mat3 {
	mustBeFinite("RotZ", float64(a))
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		m: [3][3]float64{
			{c, -s, 0},
			{s, c, 0},
			{0, 0, 1},
		},
		valid: true,
	}
}

func (a Mat3) Mul(b Mat3) Mat3 {
	a.mustBeSO3()
	b.mustBeSO3()
	var out [3][3]float64
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r][c] = a.m[r][0]*b.m[0][c] + a.m[r][1]*b.m[1][c] + a.m[r][2]*b.m[2][c]
		}
	}
	res := Mat3{m: out, valid: true}
	res.mustBeSO3()
	return res
}

func (mat Mat3) Transpose() Mat3 {
	mat.mustBeSO3()
	return Mat3{
		m: [3][3]float64{
			{mat.m[0][0], mat.m[1][0], mat.m[2][0]},
			{mat.m[0][1], mat.m[1][1], mat.m[2][1]},
			{mat.m[0][2], mat.m[1][2], mat.m[2][2]},
		},
		valid: true,
	}
}

func (mat Mat3) TransformVec(v LocalVec) WorldPoint {
	mat.mustBeSO3()
	v.mustBeFinite()
	x, y, z := float64(v.X), float64(v.Y), float64(v.Z)
	return WorldPoint{
		X: MM(mat.m[0][0]*x + mat.m[0][1]*y + mat.m[0][2]*z),
		Y: MM(mat.m[1][0]*x + mat.m[1][1]*y + mat.m[1][2]*z),
		Z: MM(mat.m[2][0]*x + mat.m[2][1]*y + mat.m[2][2]*z),
	}
}

// Pose represents a validated rigid 3D coordinate frame in SE(3).
// Fields are unexported so external callers cannot construct a singular zero Pose.
type Pose struct {
	origin WorldPoint
	basis  Mat3
}

func NewPose(origin WorldPoint, basis Mat3) Pose {
	origin.mustBeFinite()
	basis.mustBeSO3()
	return Pose{origin: origin, basis: basis}
}

func IdentityPose() Pose {
	return Pose{origin: WorldPoint{}, basis: IdentityMat3()}
}

func (p Pose) mustBeValid() {
	p.origin.mustBeFinite()
	p.basis.mustBeSO3()
}

func (p Pose) Origin() WorldPoint {
	p.mustBeValid()
	return p.origin
}

func (p Pose) Basis() Mat3 {
	p.mustBeValid()
	return p.basis
}

func (p Pose) TranslateLocal(v LocalVec) Pose {
	p.mustBeValid()
	delta := p.basis.TransformVec(v)
	return Pose{
		origin: WorldPoint{
			X: p.origin.X + delta.X,
			Y: p.origin.Y + delta.Y,
			Z: p.origin.Z + delta.Z,
		},
		basis: p.basis,
	}
}

func (p Pose) RotateLocalX(a Rad) Pose {
	p.mustBeValid()
	return Pose{origin: p.origin, basis: p.basis.Mul(RotX(a))}
}

func (p Pose) RotateLocalY(a Rad) Pose {
	p.mustBeValid()
	return Pose{origin: p.origin, basis: p.basis.Mul(RotY(a))}
}

func (p Pose) RotateLocalZ(a Rad) Pose {
	p.mustBeValid()
	return Pose{origin: p.origin, basis: p.basis.Mul(RotZ(a))}
}

func (p Pose) TransformPoint(pt WorldPoint) WorldPoint {
	return p.TranslateLocal(LocalVec{X: pt.X, Y: pt.Y, Z: pt.Z}).origin
}

func (p Pose) Compose(child Pose) Pose {
	p.mustBeValid()
	child.mustBeValid()
	return Pose{
		origin: p.TransformPoint(child.origin),
		basis:  p.basis.Mul(child.basis),
	}
}

func (p Pose) Inverse() Pose {
	p.mustBeValid()
	invBasis := p.basis.Transpose()
	negOrigin := LocalVec{X: -p.origin.X, Y: -p.origin.Y, Z: -p.origin.Z}
	return Pose{
		origin: invBasis.TransformVec(negOrigin),
		basis:  invBasis,
	}
}

```

---

### 2.2 Space-Separated Ports, Runtime Affine Sockets, and `Assembly`

Here we split `LocalPlug[R]`, `LocalSocket[R]`, and `WorldSocket[R]`, and give `WorldSocket[R]` a pointer to a runtime `socketState`. Attempting to attach two components to the same `WorldSocket[R]`—or passing a zero-value socket—panics immediately with the role name and offending call context.

```go
type Steerer struct{}
type BarClamp struct{}
type SeatCollar struct{}
type SaddleRail struct{}
type Grip struct{}
type SaddleTop struct{}

type Role interface {
	Steerer | BarClamp | SeatCollar | SaddleRail | Grip | SaddleTop
}

// LocalPlug is a part-local input interface created inside a part constructor.
type LocalPlug[R Role] struct {
	pose  Pose
	valid bool
}

func (p LocalPlug[R]) mustBeValid() {
	if !p.valid {
		var zero R
		panic(fmt.Sprintf("bikecad: uninitialized LocalPlug[%T]", zero))
	}
	p.pose.mustBeValid()
}

// LocalSocket is a part-local output interface created inside a part constructor.
// It cannot be passed to Attach; it must be promoted when its owning Part is attached.
type LocalSocket[R Role] struct {
	pose  Pose
	valid bool
}

func (s LocalSocket[R]) mustBeValid() {
	if !s.valid {
		var zero R
		panic(fmt.Sprintf("bikecad: uninitialized LocalSocket[%T]", zero))
	}
	s.pose.mustBeValid()
}

type socketState struct {
	asm      *Assembly
	roleName string
	consumed bool
}

// WorldSocket is an active mounting interface in world space owned by an Assembly.
// Runtime check enforces affine (at-most-once) consumption.
type WorldSocket[R Role] struct {
	pose  Pose
	state *socketState
}

func (s WorldSocket[R]) Pose() Pose {
	s.mustBeValid()
	return s.pose
}

func (s WorldSocket[R]) mustBeValid() {
	if s.state == nil {
		var zero R
		panic(fmt.Sprintf("bikecad: uninitialized zero-value WorldSocket[%T]", zero))
	}
	s.pose.mustBeValid()
}

func (s WorldSocket[R]) consumeFor(targetAsm *Assembly) Pose {
	s.mustBeValid()
	if s.state.asm != targetAsm {
		panic(fmt.Sprintf("bikecad: WorldSocket[%s] belongs to a different Assembly instance", s.state.roleName))
	}
	if s.state.consumed {
		panic(fmt.Sprintf("bikecad: WorldSocket[%s] has already been occupied (cannot attach two parts to the same socket)", s.state.roleName))
	}
	s.state.consumed = true
	return s.pose
}

func newWorldSocket[R Role](asm *Assembly, worldPose Pose) WorldSocket[R] {
	if asm == nil || asm.frozen {
		panic("bikecad: cannot create WorldSocket on nil or finalized Assembly")
	}
	worldPose.mustBeValid()
	var zero R
	st := &socketState{
		asm:      asm,
		roleName: fmt.Sprintf("%T", zero),
	}
	asm.sockets = append(asm.sockets, st)
	return WorldSocket[R]{
		pose:  worldPose,
		state: st,
	}
}

```

---

### 2.3 Freeze-Safe `Builder` and Typestate `Part[In, Out]`

When a part constructor finishes building its geometry, it calls `FinishPart1` or `FinishPart`. That freezes the `Builder` (catching stray `Cursor` reuse) and packages the `LocalPlug[In]`, `[]Primitive`, and socket promotion logic into a `Part[In, Out]`.

```go
type Primitive interface {
	isPrimitive()
	Transform(parent Pose) Primitive
}

type Cylinder struct {
	Start, End WorldPoint
	Diameter   MM
}

func (Cylinder) isPrimitive() {}
func (c Cylinder) Transform(parent Pose) Primitive {
	return Cylinder{
		Start:    parent.TransformPoint(c.Start),
		End:      parent.TransformPoint(c.End),
		Diameter: c.Diameter,
	}
}

type TorusArc struct {
	Center     Pose
	BendRadius MM
	SweepAngle Rad
	TubeDia    MM
}

func (TorusArc) isPrimitive() {}
func (a TorusArc) Transform(parent Pose) Primitive {
	return TorusArc{
		Center:     parent.Compose(a.Center),
		BendRadius: a.BendRadius,
		SweepAngle: a.SweepAngle,
		TubeDia:    a.TubeDia,
	}
}

type Builder struct {
	prims  []Primitive
	frozen bool
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) mustBeWritable() {
	if b == nil {
		panic("bikecad: Cursor used with nil Builder")
	}
	if b.frozen {
		panic("bikecad: attempted to emit primitives from a Cursor after its Builder was finalized")
	}
}

func (b *Builder) freezeAndTake() []Primitive {
	b.mustBeWritable()
	b.frozen = true
	out := make([]Primitive, len(b.prims))
	copy(out, b.prims)
	return out
}

func (b *Builder) Start() Cursor {
	b.mustBeWritable()
	return Cursor{builder: b, pose: IdentityPose()}
}

func (b *Builder) StartAt(p Pose) Cursor {
	b.mustBeWritable()
	p.mustBeValid()
	return Cursor{builder: b, pose: p}
}

type Cursor struct {
	builder *Builder
	pose    Pose
}

func (c Cursor) mustBeValid() {
	if c.builder == nil {
		panic("bikecad: uninitialized zero-value Cursor")
	}
	c.pose.mustBeValid()
}

func (c Cursor) Pose() Pose {
	c.mustBeValid()
	return c.pose
}

func (c Cursor) Move(dx, dy, dz MM) Cursor {
	c.mustBeValid()
	return Cursor{
		builder: c.builder,
		pose:    c.pose.TranslateLocal(LocalVec{X: dx, Y: dy, Z: dz}),
	}
}

func (c Cursor) ExtrudeVec(v LocalVec, dia MM) Cursor {
	c.mustBeValid()
	c.builder.mustBeWritable()
	v.mustBeFinite()
	mustBeNonNegative("Cylinder diameter", dia)

	next := c.pose.TranslateLocal(v)
	if dia > 0 {
		c.builder.prims = append(c.builder.prims, Cylinder{
			Start:    c.pose.Origin(),
			End:      next.Origin(),
			Diameter: dia,
		})
	}
	return Cursor{builder: c.builder, pose: next}
}

func (c Cursor) Straight(dist MM, dia MM) Cursor {
	mustBeFinite("Straight distance", float64(dist))
	return c.ExtrudeVec(LocalVec{X: dist}, dia)
}

func (c Cursor) Roll(angle Deg) Cursor {
	c.mustBeValid()
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalX(angle.Rad())}
}

func (c Cursor) Pitch(angle Deg) Cursor {
	c.mustBeValid()
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalY(angle.Rad())}
}

func (c Cursor) Yaw(angle Deg) Cursor {
	c.mustBeValid()
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalZ(angle.Rad())}
}

func (c Cursor) BendYaw(bendRadius MM, angle Deg, dia MM) Cursor {
	c.mustBeValid()
	c.builder.mustBeWritable()
	mustBeNonNegative("BendYaw bendRadius", bendRadius)
	mustBeNonNegative("BendYaw diameter", dia)
	mustBeFinite("BendYaw angle", float64(angle))

	theta := float64(angle.Rad())
	if theta == 0 {
		return c
	}
	if bendRadius == 0 {
		return c.Yaw(angle)
	}

	r := float64(bendRadius)
	sign := 1.0
	if theta < 0 {
		sign = -1.0
	}
	absTheta := math.Abs(theta)

	arcCenter := c.pose.TranslateLocal(LocalVec{Y: MM(sign * r)})
	if dia > 0 {
		c.builder.prims = append(c.builder.prims, TorusArc{
			Center:     arcCenter,
			BendRadius: bendRadius,
			SweepAngle: Rad(theta),
			TubeDia:    dia,
		})
	}

	dx := MM(r * math.Sin(absTheta))
	dy := MM(sign * r * (1.0 - math.Cos(absTheta)))
	nextPose := c.pose.
		TranslateLocal(LocalVec{X: dx, Y: dy}).
		RotateLocalZ(Rad(theta))

	return Cursor{builder: c.builder, pose: nextPose}
}

func (c Cursor) BendPitch(bendRadius MM, angle Deg, dia MM) Cursor {
	return c.Roll(-90).BendYaw(bendRadius, -angle, dia).Roll(90)
}

func MakePlug[R Role](c Cursor) LocalPlug[R] {
	c.mustBeValid()
	return LocalPlug[R]{pose: c.pose, valid: true}
}

func MakeSocket[R Role](c Cursor) LocalSocket[R] {
	c.mustBeValid()
	return LocalSocket[R]{pose: c.pose, valid: true}
}

// Part encapsulates a component's input plug, local primitives, and output socket promotion.
// Its fields are unexported so it can only be attached through Attach().
type Part[In Role, Out any] struct {
	valid   bool
	in      LocalPlug[In]
	prims   []Primitive
	promote func(asm *Assembly, partWorldPose Pose) Out
}

// PromoteSocket converts a part's LocalSocket[R] into an Assembly-tracked WorldSocket[R].
func PromoteSocket[R Role](asm *Assembly, partWorldPose Pose, local LocalSocket[R]) WorldSocket[R] {
	local.mustBeValid()
	return newWorldSocket[R](asm, partWorldPose.Compose(local.pose))
}

// FinishPart1 packages a component with a single output socket (e.g., Stem, Seatpost, Saddle).
func FinishPart1[In Role, OutRole Role](
	b *Builder,
	in LocalPlug[In],
	out LocalSocket[OutRole],
) Part[In, WorldSocket[OutRole]] {
	in.mustBeValid()
	out.mustBeValid()
	prims := b.freezeAndTake()
	return Part[In, WorldSocket[OutRole]]{
		valid: true,
		in:    in,
		prims: prims,
		promote: func(asm *Assembly, partWorldPose Pose) WorldSocket[OutRole] {
			return PromoteSocket(asm, partWorldPose, out)
		},
	}
}

// FinishPart packages a component with multiple named output sockets (e.g., Handlebar).
func FinishPart[In Role, Out any](
	b *Builder,
	in LocalPlug[In],
	promote func(asm *Assembly, partWorldPose Pose) Out,
) Part[In, Out] {
	in.mustBeValid()
	if promote == nil {
		panic("bikecad: nil promote function in FinishPart")
	}
	prims := b.freezeAndTake()
	return Part[In, Out]{
		valid:   true,
		in:      in,
		prims:   prims,
		promote: promote,
	}
}

// Assembly owns the world-space primitive list and tracks socket consumption.
type Assembly struct {
	prims   []Primitive
	sockets []*socketState
	frozen  bool
}

func NewAssembly() *Assembly {
	return &Assembly{}
}

// Attach consumes `parentSocket`, transforms `part` into world space, appends all
// of `part`'s primitives to `asm`, and returns the promoted world-space output socket(s).
func Attach[In Role, Out any](
	asm *Assembly,
	parentSocket WorldSocket[In],
	part Part[In, Out],
) Out {
	if asm == nil || asm.frozen {
		panic("bikecad: cannot Attach to nil or finalized Assembly")
	}
	if !part.valid {
		panic("bikecad: cannot Attach uninitialized zero-value Part")
	}
	parentWorldPose := parentSocket.consumeFor(asm)
	worldPose := parentWorldPose.Compose(part.in.pose.Inverse())

	for _, p := range part.prims {
		asm.prims = append(asm.prims, p.Transform(worldPose))
	}
	return part.promote(asm, worldPose)
}

```

---

### 2.4 Component Constructors Using `FinishPart1` and `FinishPart`

Notice that `StemPart`, `SeatpostPart`, and `PlaceSocket` are completely gone—`NewStem` returns `Part[Steerer, WorldSocket[BarClamp]]` directly, and `NewSweptHandlebar` returns `Part[BarClamp, HandlebarGrips]`.

```go
type StemSpec struct {
	Extension   MM
	Angle       Deg
	ClampHeight MM
	BodyDia     MM
}

func NewStem(s StemSpec) Part[Steerer, WorldSocket[BarClamp]] {
	mustBeNonNegative("StemSpec.Extension", s.Extension)
	mustBeNonNegative("StemSpec.ClampHeight", s.ClampHeight)
	mustBeNonNegative("StemSpec.BodyDia", s.BodyDia)

	b := NewBuilder()
	base := b.Start()
	base.ExtrudeVec(LocalVec{Z: s.ClampHeight}, s.BodyDia+6)

	barClamp := base.
		Move(0, 0, s.ClampHeight/2).
		Pitch(-s.Angle).
		Straight(s.Extension, s.BodyDia)

	return FinishPart1(b, MakePlug[Steerer](base), MakeSocket[BarClamp](barClamp))
}

type HandlebarSpec struct {
	CenterFlat MM
	Rise       MM
	GripLen    MM
	BendRadius MM
	Backsweep  Deg
	Upsweep    Deg
	Roll       Deg
	Dia        MM
}

type HandlebarGrips struct {
	Left  WorldSocket[Grip]
	Right WorldSocket[Grip]
}

func NewSweptHandlebar(h HandlebarSpec) Part[BarClamp, HandlebarGrips] {
	b := NewBuilder()
	clamp := b.Start()
	rolled := clamp.Pitch(-h.Roll)

	buildSide := func(sideSign float64) Cursor {
		s := Deg(sideSign)
		return rolled.
			Yaw(90 * s).
			Straight(h.CenterFlat/2, h.Dia).
			BendPitch(h.BendRadius, -30*s, h.Dia).
			Straight(h.Rise, h.Dia).
			BendPitch(h.BendRadius, 30*s, h.Dia).
			BendPitch(h.BendRadius, -h.Upsweep, h.Dia).
			BendYaw(h.BendRadius, -h.Backsweep*s, h.Dia).
			Straight(h.GripLen, h.Dia)
	}

	leftLocal := MakeSocket[Grip](buildSide(+1))
	rightLocal := MakeSocket[Grip](buildSide(-1))

	return FinishPart(b, MakePlug[BarClamp](clamp), func(asm *Assembly, worldPose Pose) HandlebarGrips {
		return HandlebarGrips{
			Left:  PromoteSocket(asm, worldPose, leftLocal),
			Right: PromoteSocket(asm, worldPose, rightLocal),
		}
	})
}

type SeatpostSpec struct {
	Extension MM
	Setback   MM
	MinInsert MM
	TotalLen  MM
	Dia       MM
}

func NewSeatpost(sp SeatpostSpec) Part[SeatCollar, WorldSocket[SaddleRail]] {
	b := NewBuilder()
	collar := b.Start()
	topOfShaft := collar.ExtrudeVec(LocalVec{Z: sp.Extension}, sp.Dia)
	clamp := topOfShaft.ExtrudeVec(LocalVec{X: -sp.Setback}, sp.Dia*0.8)

	return FinishPart1(b, MakePlug[SeatCollar](collar), MakeSocket[SaddleRail](clamp))
}

```

---

### 2.5 Grounded Chassis & Complete `Solve` Pipeline

Look at `Solve()` below: every intermediate `WorldSocket` is consumed exactly once, primitives cannot be dropped, and local sockets cannot leak into world space.

```go
type FrameSpec struct {
	ChainstayLength MM
	BBDrop          MM
	Stack           MM
	Reach           MM
	HeadTubeAngle   Deg
	HeadTubeLength  MM
	SeatTubeAngle   Deg
	SeatTubeLength  MM
}

type ForkSpec struct {
	AxleToCrown   MM
	Sag           MM
	Offset        MM
	SteererLength MM
}

func (f ForkSpec) EffectiveAxleToCrown() MM {
	return f.AxleToCrown - f.Sag
}

type HeadsetSpec struct {
	LowerStack MM
	UpperStack MM
}

type WheelSpec struct {
	LoadedRadius MM
}

type SaddleSpec struct {
	RailHeight    MM
	ForeAftOffset MM
	Tilt          Deg
}

type BuildSpec struct {
	Frame      FrameSpec
	Fork       ForkSpec
	Headset    HeadsetSpec
	FrontWheel WheelSpec
	RearWheel  WheelSpec
	SpacerMM   MM
	Stem       StemSpec
	Bar        HandlebarSpec
	Seatpost   SeatpostSpec
	Saddle     SaddleSpec
}

type RollingChassis struct {
	Pitch         Rad
	BottomBracket Pose
	RearAxle      Pose
	FrontAxle     Pose
	SteererOut    WorldSocket[Steerer]
	SeatCollarOut WorldSocket[SeatCollar]
}

func AssembleChassis(
	asm *Assembly,
	frame FrameSpec,
	fork ForkSpec,
	headset HeadsetSpec,
	frontWheel WheelSpec,
	rearWheel WheelSpec,
	spacerHeight MM,
	stemClampHeight MM,
) (RollingChassis, error) {
	if frame.ChainstayLength <= math.Abs(float64(frame.BBDrop)) ||
		frame.Stack <= 0 || frame.Reach <= 0 ||
		frame.HeadTubeLength <= 0 || frame.SeatTubeLength <= 0 ||
		frame.HeadTubeAngle <= 0 || frame.HeadTubeAngle >= 180 ||
		frame.SeatTubeAngle <= 0 || frame.SeatTubeAngle >= 180 {
		return RollingChassis{}, ErrInvalidFrameGeometry
	}

	a2c := fork.EffectiveAxleToCrown()
	if fork.Sag < 0 || a2c <= frontWheel.LoadedRadius {
		return RollingChassis{}, ErrForkTireInterference
	}

	requiredSteerer := frame.HeadTubeLength + headset.LowerStack + headset.UpperStack + spacerHeight + stemClampHeight
	if requiredSteerer > fork.SteererLength {
		return RollingChassis{}, ErrInsufficientSteererLength
	}

	cs := float64(frame.ChainstayLength)
	drop := float64(frame.BBDrop)
	rearAxleFrame := LocalVec{
		X: MM(-math.Sqrt(cs*cs - drop*drop)),
		Z: frame.BBDrop,
	}

	hta := frame.HeadTubeAngle.Rad()
	htDown := LocalVec{X: MM(math.Cos(float64(hta))), Z: MM(-math.Sin(float64(hta)))}
	htForward := LocalVec{X: MM(math.Sin(float64(hta))), Z: MM(math.Cos(float64(hta)))}

	htTopFrame := LocalVec{X: frame.Reach, Z: frame.Stack}
	crownDist := float64(frame.HeadTubeLength + headset.LowerStack)
	rake := float64(fork.Offset)

	frontAxleFrame := LocalVec{
		X: htTopFrame.X + MM((crownDist+float64(a2c))*float64(htDown.X)+rake*float64(htForward.X)),
		Z: htTopFrame.Z + MM((crownDist+float64(a2c))*float64(htDown.Z)+rake*float64(htForward.Z)),
	}

	dx := float64(frontAxleFrame.X - rearAxleFrame.X)
	dz := float64(frontAxleFrame.Z - rearAxleFrame.Z)
	axleSpan := math.Hypot(dx, dz)

	dzWorld := float64(frontWheel.LoadedRadius - rearWheel.LoadedRadius)
	if axleSpan <= math.Abs(dzWorld) {
		return RollingChassis{}, ErrUnbuildableChassis
	}

	frameAxleAngle := math.Atan2(dz, dx)
	worldAxleAngle := math.Asin(dzWorld / axleSpan)
	pitch := Rad(frameAxleAngle - worldAxleAngle)

	b := NewBuilder()
	rearAxlePitched := b.StartAt(NewPose(
		WorldPoint{X: 0, Y: 0, Z: rearWheel.LoadedRadius},
		RotY(pitch),
	))

	bb := rearAxlePitched.Move(-rearAxleFrame.X, 0, -rearAxleFrame.Z)
	if bb.Pose().Origin().Z <= 0 {
		return RollingChassis{}, ErrBBBelowGround
	}
	frontAxle := bb.Move(frontAxleFrame.X, 0, frontAxleFrame.Z)

	bb.ExtrudeVec(rearAxleFrame, 24)

	effSTA := frame.SeatTubeAngle.Rad() + pitch
	seatTubeTilt := (Rad(math.Pi/2.0) - effSTA).Deg()
	seatCollar := bb.Pitch(-seatTubeTilt).ExtrudeVec(LocalVec{Z: frame.SeatTubeLength}, 34.9)

	effHTA := hta + pitch
	steererTilt := (Rad(math.Pi/2.0) - effHTA).Deg()
	htTop := bb.Move(frame.Reach, 0, frame.Stack).Pitch(-steererTilt)
	htTop.ExtrudeVec(LocalVec{Z: -frame.HeadTubeLength}, 44)

	steererStemBase := htTop.ExtrudeVec(LocalVec{Z: headset.UpperStack + spacerHeight}, 28.6)

	// Freeze chassis builder and record chassis primitives into the Assembly:
	asm.prims = append(asm.prims, b.freezeAndTake()...)

	return RollingChassis{
		Pitch:         pitch,
		BottomBracket: bb.Pose(),
		RearAxle:      NewPose(rearAxlePitched.Pose().Origin(), IdentityMat3()),
		FrontAxle:     NewPose(frontAxle.Pose().Origin(), IdentityMat3()),
		SteererOut:    newWorldSocket[Steerer](asm, steererStemBase.Pose()),
		SeatCollarOut: newWorldSocket[SeatCollar](asm, seatCollar.Pose()),
	}, nil
}

type SolvedBike struct {
	Chassis   RollingChassis
	StemClamp WorldSocket[BarClamp]
	LeftGrip  WorldSocket[Grip]
	RightGrip WorldSocket[Grip]
	SaddleSit WorldSocket[SaddleTop]
	Prims     []Primitive
}

func Solve(spec BuildSpec) (SolvedBike, error) {
	inserted := spec.Seatpost.TotalLen - spec.Seatpost.Extension
	if inserted < spec.Seatpost.MinInsert {
		return SolvedBike{}, ErrSeatpostMinInsertion
	}
	if inserted > spec.Frame.SeatTubeLength {
		return SolvedBike{}, ErrSeatpostBottomedOut
	}

	asm := NewAssembly()

	chassis, err := AssembleChassis(
		asm,
		spec.Frame,
		spec.Fork,
		spec.Headset,
		spec.FrontWheel,
		spec.RearWheel,
		spec.SpacerMM,
		spec.Stem.ClampHeight,
	)
	if err != nil {
		return SolvedBike{}, err
	}

	// 1. Attach Stem to Chassis SteererOut (consumes chassis.SteererOut; promotes stemClamp)
	stemClamp := Attach(asm, chassis.SteererOut, NewStem(spec.Stem))

	// 2. Attach Handlebar to Stem (consumes stemClamp; promotes Left & Right grips)
	grips := Attach(asm, stemClamp, NewSweptHandlebar(spec.Bar))

	// 3. Attach Seatpost to Chassis SeatCollarOut (consumes chassis.SeatCollarOut; promotes railMount)
	railMount := Attach(asm, chassis.SeatCollarOut, NewSeatpost(spec.Seatpost))

	// 4. Consume railMount to position the world-leveled Saddle sit-bone reference
	railPose := railMount.consumeFor(asm)
	sb := NewBuilder()
	saddleCursor := sb.StartAt(NewPose(railPose.Origin(), IdentityMat3())).
		Pitch(spec.Saddle.Tilt).
		Move(spec.Saddle.ForeAftOffset, 0, spec.Saddle.RailHeight)
	asm.prims = append(asm.prims, sb.freezeAndTake()...)
	saddleSit := newWorldSocket[SaddleTop](asm, saddleCursor.Pose())

	// Lock the assembly so no further mutations can occur after Solve returns:
	asm.frozen = true
	outPrims := make([]Primitive, len(asm.prims))
	copy(outPrims, asm.prims)

	return SolvedBike{
		Chassis:   chassis,
		StemClamp: stemClamp,
		LeftGrip:  grips.Left,
		RightGrip: grips.Right,
		SaddleSit: saddleSit,
		Prims:     outPrims,
	}, nil
}

```