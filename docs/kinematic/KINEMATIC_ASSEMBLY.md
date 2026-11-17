# Principled Bicycle Geometry & Kinematic Assembly in Go

## 1. Why Transition Away from Ad-Hoc Geometry Code?

In geometric modeling, ad-hoc coordinate manipulation breaks down the moment **mutations stop being local**—when changing one component (such as fork axle-to-crown length, fork sag, or lower headset cup stack) silently invalidates derived coordinates elsewhere in the system.

Four failure modes signal that an ad-hoc approach has outlived its usefulness:

1. **The $N \times M$ Audit Problem:** Adding a single new physical detail—such as an external lower headset cup (`EC44`), suspension fork sag, or handlebar roll—forces an inspection of every existing calculation to see which ones forgot to account for it.
2. **Order-of-Operations Drift:** Mutating derived state incrementally (e.g., calling `SwapFork()` followed by `SetSaddleHeight()` versus the reverse) can produce different coordinates or stale state when intermediate values are patched in place rather than derived from canonical inputs.
3. **Coordinate-Space Mixing:** Representing everything as raw `float64` values mixes incompatible reference frames—world $X/Z$ relative to the ground or bottom bracket, distance along the seat tube axis, distance along the steerer axis, and stem extension—making it easy to add two numbers that do not share a basis.
4. **Missing-Piece & Invalid-Assembly Bugs:** Nothing in an untyped setup prevents attaching a stem directly to a frame without accounting for the upper headset cover, or clamping a component onto an incompatible interface.

---

## 2. Core Architectural Principles

A bicycle setup does not require a heavy numerical constraint solver (such as a CAD sketch solver that iterates over arbitrary intersecting lines). A bicycle is a **directed acyclic kinematic tree** anchored to the ground by two wheels. Four design choices eliminate ad-hoc geometry bugs while keeping the implementation fast and readable:

1. **Pure Forward Kinematics (No Incremental Mutation):**
   Store **only intrinsic component specifications** (frame dimensions, fork specs, headset stacks, spacer height, stem specs, bar geometry, seatpost specs, saddle specs). Compute all world coordinates and 3D primitives in a single, deterministic pass (`Solve(BuildSpec) -> SolvedBike`). Recomputing ~15 rigid-body transforms takes microseconds and completely eliminates stale state.
2. **Rigid-Body Poses in $SE(3)$:**
   Use the **ISO 8855 vehicle coordinate convention**:
   * $+X$: Forward (rear-to-front in world space; along tube tangent in local space)
   * $+Y$: Rider-left
   * $+Z$: Vertically upward
   * World Origin $(0, 0, 0)$: Rear tire ground contact patch
   * Right-hand rule: Positive rotation around $+Y$ pitches nose-down.
   Every attachment point is represented as a `Pose` (3D origin + $3 \times 3$ orthonormal orientation basis). When fork length changes frame pitch, downstream components simply compose onto their parent `Pose` without needing to know *why* the parent tilted.
3. **Reusable Geometric Primitives via a 3D `Cursor` Builder:**
   Instead of scattering `math.Sin` and `math.Cos` across every component, parts are constructed using a 3D turtle/CNC tube-bender `Cursor` (`Straight`, `ExtrudeVec`, `Pitch`, `Yaw`, `Roll`, `BendYaw`, `BendPitch`). The `Cursor` walks through 3D space, automatically emitting renderable/collision-checkable primitives (`Cylinder`, `TorusArc`) and computing endpoint poses from a single source of truth.
4. **Phantom-Typed Mating Ports (`Plug[R]` and `Socket[R]`):**
   Even when parts are built from identical geometric primitives (`Cylinder`, `TorusArc`), their attachment endpoints are wrapped in generic `Plug[R]` and `Socket[R]` types parameterized by zero-byte role markers (`Steerer`, `BarClamp`, `SeatCollar`, `SaddleRail`, `Grip`). Invalid assemblies fail to compile.

---

## 3. Modern Go Language Features Used (Go 1.18+)

If you are coming from earlier versions of Go, this architecture relies on four language and design features:

1. **Type Parameters / Generics (Go 1.18):**
   Types and functions can take compile-time type arguments in square brackets, such as `Socket[R Role]` and `func Attach[R Role](...)`. Unlike `interface{}`, generics are verified at compile time and incur zero runtime boxing or reflection overhead.
2. **Zero-Byte Phantom Types (`struct{}`):**
   An empty struct (`type Steerer struct{}`) occupies 0 bytes of memory at runtime. When used as a type parameter (`Socket[Steerer]`), it acts purely as a compile-time tag that distinguishes a `Socket[Steerer]` from a `Socket[BarClamp]` despite both having the exact same underlying memory layout (`Pose`).
3. **Union Type Sets in Interfaces (Go 1.18):**
   Interfaces can restrict which types may satisfy a generic parameter using the union operator (`|`):
   ```go
   type Role interface {
       Steerer | BarClamp | SeatCollar | SaddleRail | Grip | SaddleTop
   }
   ```
   This prevents instantiating `Socket[int]` or `Plug[string]`—only the listed physical interfaces are permitted.
4. **Call-Site Type Inference (Go 1.18–1.21) & Value-Semantic Branching:**
   * When calling `Attach(parentSocket, childPlug, childPrims)`, the compiler infers the `Role` type parameter automatically from the arguments, so you never have to write `Attach[BarClamp](...)` at call sites.
   * Go does not allow *methods* on a struct to introduce new type parameters not already declared on the receiver struct. Because `Cursor` is non-generic, converting a `Cursor` into a generic `Plug[R]` or `Socket[R]` uses a package-level function (`MakePlug[Steerer](cursor)`) rather than a method.
   * `Cursor` is passed **by value** while holding a pointer to a shared `Builder`. Branching a component in 3D space (such as building the left and right sides of a handlebar from the center clamp) requires no state stack—reusing the `center` variable naturally forks from its saved `Pose`.

---

## 4. Reference Implementation in Go

### 4.1 Units, Coordinate Spaces, and $SE(3)$ Poses

Separating `WorldPoint` from `LocalVec` prevents accidentally adding a local component offset directly to a world coordinate without rotating it through the component's basis.

```go
package bikecad

import (
	"errors"
	"math"
)

var (
	ErrInvalidFrameGeometry      = errors.New("chainstay length must exceed absolute BB drop")
	ErrUnbuildableChassis        = errors.New("wheel radius difference exceeds axle span")
	ErrInsufficientSteererLength = errors.New("steerer stack exceeds fork steerer length")
	ErrSeatpostMinInsertion      = errors.New("seatpost insertion is less than minimum required")
	ErrSeatpostBottomedOut       = errors.New("seatpost insertion exceeds frame seat tube length")
)

type MM float64
type Deg float64
type Rad float64

func (d Deg) Rad() Rad { return Rad(float64(d) * math.Pi / 180.0) }
func (r Rad) Deg() Deg { return Deg(float64(r) * 180.0 / math.Pi) }

// WorldPoint is a 3D coordinate in world space (origin at rear tire contact patch).
type WorldPoint struct{ X, Y, Z MM }

// LocalVec is a 3D displacement vector expressed in a Pose's local coordinate basis.
type LocalVec struct{ X, Y, Z MM }

// Mat3 is a 3x3 orthonormal rotation matrix whose columns are the local X, Y, Z axes in world space.
type Mat3 [3][3]float64

func IdentityMat3() Mat3 {
	return Mat3{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	}
}

func RotX(a Rad) Mat3 {
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		{1, 0, 0},
		{0, c, -s},
		{0, s, c},
	}
}

func RotY(a Rad) Mat3 {
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		{c, 0, s},
		{0, 1, 0},
		{-s, 0, c},
	}
}

func RotZ(a Rad) Mat3 {
	c, s := math.Cos(float64(a)), math.Sin(float64(a))
	return Mat3{
		{c, -s, 0},
		{s, c, 0},
		{0, 0, 1},
	}
}

func (a Mat3) Mul(b Mat3) Mat3 {
	var out Mat3
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r][c] = a[r][0]*b[0][c] + a[r][1]*b[1][c] + a[r][2]*b[2][c]
		}
	}
	return out
}

// Transpose returns the inverse of an orthonormal rotation matrix (R^-1 == R^T).
func (m Mat3) Transpose() Mat3 {
	return Mat3{
		{m[0][0], m[1][0], m[2][0]},
		{m[0][1], m[1][1], m[2][1]},
		{m[0][2], m[1][2], m[2][2]},
	}
}

func (m Mat3) TransformVec(v LocalVec) WorldPoint {
	x, y, z := float64(v.X), float64(v.Y), float64(v.Z)
	return WorldPoint{
		X: MM(m[0][0]*x + m[0][1]*y + m[0][2]*z),
		Y: MM(m[1][0]*x + m[1][1]*y + m[1][2]*z),
		Z: MM(m[2][0]*x + m[2][1]*y + m[2][2]*z),
	}
}

// Pose represents a rigid 3D coordinate frame (origin + orientation basis).
type Pose struct {
	Origin WorldPoint
	Basis  Mat3
}

func IdentityPose() Pose {
	return Pose{Origin: WorldPoint{}, Basis: IdentityMat3()}
}

func (p Pose) TranslateLocal(v LocalVec) Pose {
	delta := p.Basis.TransformVec(v)
	return Pose{
		Origin: WorldPoint{
			X: p.Origin.X + delta.X,
			Y: p.Origin.Y + delta.Y,
			Z: p.Origin.Z + delta.Z,
		},
		Basis: p.Basis,
	}
}

func (p Pose) RotateLocalX(a Rad) Pose { return Pose{Origin: p.Origin, Basis: p.Basis.Mul(RotX(a))} }
func (p Pose) RotateLocalY(a Rad) Pose { return Pose{Origin: p.Origin, Basis: p.Basis.Mul(RotY(a))} }
func (p Pose) RotateLocalZ(a Rad) Pose { return Pose{Origin: p.Origin, Basis: p.Basis.Mul(RotZ(a))} }

func (p Pose) TransformPoint(pt WorldPoint) WorldPoint {
	return p.TranslateLocal(LocalVec{X: pt.X, Y: pt.Y, Z: pt.Z}).Origin
}

// Compose multiplies two rigid transforms: T_world = T_p * T_child.
func (p Pose) Compose(child Pose) Pose {
	return Pose{
		Origin: p.TransformPoint(child.Origin),
		Basis:  p.Basis.Mul(child.Basis),
	}
}

// Inverse returns the rigid transform T^-1 such that p.Compose(p.Inverse()) == IdentityPose().
func (p Pose) Inverse() Pose {
	invBasis := p.Basis.Transpose()
	negOrigin := LocalVec{X: -p.Origin.X, Y: -p.Origin.Y, Z: -p.Origin.Z}
	return Pose{
		Origin: invBasis.TransformVec(negOrigin),
		Basis:  invBasis,
	}
}
```

---

### 4.2 Phantom Roles, Typed Mating Ports, and 3D Primitives

```go
// Zero-byte phantom marker types representing mechanical mating interfaces.
type Steerer struct{}
type BarClamp struct{}
type SeatCollar struct{}
type SaddleRail struct{}
type Grip struct{}
type SaddleTop struct{}

// Role restricts generic Plug[R] and Socket[R] types to valid mechanical interfaces.
type Role interface {
	Steerer | BarClamp | SeatCollar | SaddleRail | Grip | SaddleTop
}

// Plug represents an input mounting interface on a child component.
type Plug[R Role] struct {
	pose Pose
}

func (p Plug[R]) Pose() Pose { return p.pose }

// Socket represents an output mounting interface on a parent component.
type Socket[R Role] struct {
	pose Pose
}

func (s Socket[R]) Pose() Pose { return s.pose }

// Primitive represents a 3D geometric shape for visualization and clearance checks.
type Primitive interface {
	Transform(parent Pose) Primitive
}

// Cylinder represents a straight tube segment between two 3D points.
type Cylinder struct {
	Start, End WorldPoint
	Diameter   MM
}

func (c Cylinder) Transform(parent Pose) Primitive {
	return Cylinder{
		Start:    parent.TransformPoint(c.Start),
		End:      parent.TransformPoint(c.End),
		Diameter: c.Diameter,
	}
}

// TorusArc represents a constant-radius pipe bend in Center's local X-Y plane.
type TorusArc struct {
	Center     Pose
	BendRadius MM
	SweepAngle Rad
	TubeDia    MM
}

func (a TorusArc) Transform(parent Pose) Primitive {
	return TorusArc{
		Center:     parent.Compose(a.Center),
		BendRadius: a.BendRadius,
		SweepAngle: a.SweepAngle,
		TubeDia:    a.TubeDia,
	}
}
```

---

### 4.3 The Fluent 3D `Cursor` Builder

All trigonometric projections and bend sweeps live inside `Cursor`. Individual bike parts are constructed by chaining `Cursor` movements and stamping endpoints with `MakePlug` and `MakeSocket`.

```go
type Builder struct {
	prims []Primitive
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Primitives() []Primitive {
	out := make([]Primitive, len(b.prims))
	copy(out, b.prims)
	return out
}

func (b *Builder) Start() Cursor {
	return Cursor{
		builder: b,
		pose:    IdentityPose(),
	}
}

func (b *Builder) StartAt(p Pose) Cursor {
	return Cursor{
		builder: b,
		pose:    p,
	}
}

// Cursor is a value-type 3D toolhead that emits primitives into Builder as it moves.
// Convention: Cursor's local +X axis is the primary tube-extrusion tangent direction.
type Cursor struct {
	builder *Builder
	pose    Pose
}

func (c Cursor) Pose() Pose { return c.pose }

// Move translates the cursor in its local basis without emitting a primitive.
func (c Cursor) Move(dx, dy, dz MM) Cursor {
	return Cursor{
		builder: c.builder,
		pose:    c.pose.TranslateLocal(LocalVec{X: dx, Y: dy, Z: dz}),
	}
}

// ExtrudeVec draws a Cylinder along local vector v and advances the cursor to its endpoint.
func (c Cursor) ExtrudeVec(v LocalVec, dia MM) Cursor {
	next := c.pose.TranslateLocal(v)
	if dia > 0 {
		c.builder.prims = append(c.builder.prims, Cylinder{
			Start:    c.pose.Origin,
			End:      next.Origin,
			Diameter: dia,
		})
	}
	return Cursor{builder: c.builder, pose: next}
}

// Straight extrudes a Cylinder along the cursor's local +X tangent.
func (c Cursor) Straight(dist MM, dia MM) Cursor {
	return c.ExtrudeVec(LocalVec{X: dist}, dia)
}

func (c Cursor) Roll(angle Deg) Cursor {
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalX(angle.Rad())}
}

func (c Cursor) Pitch(angle Deg) Cursor {
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalY(angle.Rad())}
}

func (c Cursor) Yaw(angle Deg) Cursor {
	return Cursor{builder: c.builder, pose: c.pose.RotateLocalZ(angle.Rad())}
}

// BendYaw sweeps the tube through `angle` in the cursor's local X-Y plane with radius `bendRadius`.
// Positive angle bends toward local +Y; negative angle bends toward local -Y.
func (c Cursor) BendYaw(bendRadius MM, angle Deg, dia MM) Cursor {
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

// BendPitch sweeps the tube in the cursor's local X-Z plane.
// Positive angle pitches the tube downward (-Z); negative angle pitches upward (+Z).
func (c Cursor) BendPitch(bendRadius MM, angle Deg, dia MM) Cursor {
	return c.Roll(-90).BendYaw(bendRadius, -angle, dia).Roll(90)
}

// MakePlug stamps a Cursor's current pose as a typed input Plug[R].
func MakePlug[R Role](c Cursor) Plug[R] {
	return Plug[R]{pose: c.pose}
}

// MakeSocket stamps a Cursor's current pose as a typed output Socket[R].
func MakeSocket[R Role](c Cursor) Socket[R] {
	return Socket[R]{pose: c.pose}
}
```

---

### 4.4 Component Constructors Built from Primitives

None of the component constructors below use raw `math.Sin` or `math.Cos`; each chains `Cursor` steps to generate both typed attachment ports and 3D primitives simultaneously.

```go
type StemSpec struct {
	Extension   MM  // Center-to-center length along stem body
	Angle       Deg // Rise relative to perpendicular to steerer (e.g., +6, 0, -17)
	ClampHeight MM  // Steerer clamp stack height
	BodyDia     MM  // Visual tube diameter
}

type StemPart struct {
	In       Plug[Steerer]
	BarMount Socket[BarClamp]
	Prims    []Primitive
}

func NewStem(s StemSpec) StemPart {
	b := NewBuilder()
	base := b.Start() // Origin at bottom of steerer clamp bore; +Z up steerer, +X forward

	// 1. Extrude the steerer clamp bore cylinder along local +Z.
	base.ExtrudeVec(LocalVec{Z: s.ClampHeight}, s.BodyDia+6)

	// 2. Move halfway up the steerer clamp, pitch by -Angle, and extrude the stem body along +X.
	barClamp := base.
		Move(0, 0, s.ClampHeight/2).
		Pitch(-s.Angle).
		Straight(s.Extension, s.BodyDia)

	return StemPart{
		In:       MakePlug[Steerer](base),
		BarMount: MakeSocket[BarClamp](barClamp),
		Prims:    b.Primitives(),
	}
}

type HandlebarSpec struct {
	CenterFlat MM  // Unswept center section width
	Rise       MM  // Riser segment length
	GripLen    MM  // Straight grip section length
	BendRadius MM  // Tube-bender centerline radius
	Backsweep  Deg // Rearward sweep angle
	Upsweep    Deg // Upward sweep angle
	Roll       Deg // Bar roll angle in the stem clamp
	Dia        MM  // Tube diameter
}

type HandlebarPart struct {
	In        Plug[BarClamp]
	LeftGrip  Socket[Grip]
	RightGrip Socket[Grip]
	Prims     []Primitive
}

func NewSweptHandlebar(h HandlebarSpec) HandlebarPart {
	b := NewBuilder()
	clamp := b.Start() // At stem clamp center: +X forward, +Y left, +Z up
	rolled := clamp.Pitch(-h.Roll)

	// Value semantics let us branch left (+1) and right (-1) cleanly from `rolled`:
	buildSide := func(sideSign float64) Cursor {
		s := Deg(sideSign)
		return rolled.
			Yaw(90 * s).                              // Aim cursor +X along the bar (+Y left or -Y right)
			Straight(h.CenterFlat/2, h.Dia).          // Unswept center clamp section
			BendPitch(h.BendRadius, -30*s, h.Dia).    // Bend upward for rise
			Straight(h.Rise, h.Dia).                  // Riser segment
			BendPitch(h.BendRadius, 30*s, h.Dia).     // Level out from rise
			BendPitch(h.BendRadius, -h.Upsweep, h.Dia). // Apply upsweep
			BendYaw(h.BendRadius, -h.Backsweep*s, h.Dia). // Apply backsweep toward rear (-X)
			Straight(h.GripLen, h.Dia)                // Grip section
	}

	leftEnd := buildSide(+1)
	rightEnd := buildSide(-1)

	return HandlebarPart{
		In:        MakePlug[BarClamp](clamp),
		LeftGrip:  MakeSocket[Grip](leftEnd),
		RightGrip: MakeSocket[Grip](rightEnd),
		Prims:     b.Primitives(),
	}
}

type SeatpostSpec struct {
	Extension MM // Seat collar to rail clamp center along post axis
	Setback   MM // Aft offset perpendicular to post axis
	MinInsert MM // Minimum required insertion inside frame
	TotalLen  MM // Total shaft length below rail clamp
	Dia       MM // Post diameter (e.g., 27.2, 30.9, 31.6)
}

type SeatpostPart struct {
	In        Plug[SeatCollar]
	RailMount Socket[SaddleRail]
	Prims     []Primitive
}

func NewSeatpost(sp SeatpostSpec) SeatpostPart {
	b := NewBuilder()
	collar := b.Start() // Origin at seat collar; +Z up seat tube axis, +X forward perpendicular to post

	topOfShaft := collar.ExtrudeVec(LocalVec{Z: sp.Extension}, sp.Dia)
	clamp := topOfShaft.ExtrudeVec(LocalVec{X: -sp.Setback}, sp.Dia*0.8)

	return SeatpostPart{
		In:        MakePlug[SeatCollar](collar),
		RailMount: MakeSocket[SaddleRail](clamp),
		Prims:     b.Primitives(),
	}
}
```

---

### 4.5 Closed-Form Grounded Chassis Pitch Solver

Because frame pitch depends simultaneously on fork axle-to-crown, sag, fork offset, lower headset cup stack, and both wheel radii, `AssembleChassis` solves the ground-contact pitch angle $\theta_{\text{pitch}}$ in closed form before exposing typed `Steerer` and `SeatCollar` sockets.

```go
type FrameSpec struct {
	ChainstayLength MM
	BBDrop          MM  // Positive when BB is below axle line in design geometry
	Stack           MM  // Design stack (axles level)
	Reach           MM  // Design reach (axles level)
	HeadTubeAngle   Deg // Design HTA (axles level)
	HeadTubeLength  MM
	SeatTubeAngle   Deg // Design STA (axles level)
	SeatTubeLength  MM  // BB center to top of seat collar
}

type ForkSpec struct {
	AxleToCrown   MM // Unloaded axle-to-crown
	Sag           MM // Static compression under rider weight (0 for rigid)
	Offset        MM // Rake perpendicular to steerer axis
	SteererLength MM // Total steerer length above crown race seat
}

func (f ForkSpec) EffectiveAxleToCrown() MM {
	return f.AxleToCrown - f.Sag
}

type HeadsetSpec struct {
	LowerStack MM // e.g., 12mm for EC44, 0-2mm for IS/ZS
	UpperStack MM // Bearing cover + top cap stack above head tube
}

type WheelSpec struct {
	LoadedRadius MM // Tire outer radius under load (e.g., ~368mm for 29x2.25)
}

type RollingChassis struct {
	Pitch         Rad
	BottomBracket Pose
	RearAxle      Pose
	FrontAxle     Pose
	SteererOut    Socket[Steerer]
	SeatCollarOut Socket[SeatCollar]
	Prims         []Primitive
}

func AssembleChassis(
	frame FrameSpec,
	fork ForkSpec,
	headset HeadsetSpec,
	frontWheel WheelSpec,
	rearWheel WheelSpec,
	spacerHeight MM,
	stemClampHeight MM,
) (RollingChassis, error) {
	requiredSteerer := frame.HeadTubeLength + headset.LowerStack + headset.UpperStack + spacerHeight + stemClampHeight
	if requiredSteerer > fork.SteererLength {
		return RollingChassis{}, ErrInsufficientSteererLength
	}

	// 1. Compute rear and front axle coordinates in frame-design space (BB at 0,0; design axles level).
	cs := float64(frame.ChainstayLength)
	drop := float64(frame.BBDrop)
	if cs <= math.Abs(drop) {
		return RollingChassis{}, ErrInvalidFrameGeometry
	}
	rearAxleFrame := LocalVec{
		X: MM(-math.Sqrt(cs*cs - drop*drop)),
		Z: frame.BBDrop,
	}

	hta := frame.HeadTubeAngle.Rad()
	htDown := LocalVec{X: MM(math.Cos(float64(hta))), Z: MM(-math.Sin(float64(hta)))}
	htForward := LocalVec{X: MM(math.Sin(float64(hta))), Z: MM(math.Cos(float64(hta)))}

	htTopFrame := LocalVec{X: frame.Reach, Z: frame.Stack}
	crownDist := float64(frame.HeadTubeLength + headset.LowerStack)
	a2c := float64(fork.EffectiveAxleToCrown())
	rake := float64(fork.Offset)

	frontAxleFrame := LocalVec{
		X: htTopFrame.X + MM((crownDist+a2c)*float64(htDown.X)+rake*float64(htForward.X)),
		Z: htTopFrame.Z + MM((crownDist+a2c)*float64(htDown.Z)+rake*float64(htForward.Z)),
	}

	// 2. Solve nose-down pitch angle so both tires rest on Z = 0 simultaneously.
	dx := float64(frontAxleFrame.X - rearAxleFrame.X)
	dz := float64(frontAxleFrame.Z - rearAxleFrame.Z)
	axleSpan := math.Hypot(dx, dz)

	dzWorld := float64(frontWheel.LoadedRadius - rearWheel.LoadedRadius)
	if axleSpan <= math.Abs(dzWorld) {
		return RollingChassis{}, ErrUnbuildableChassis
	}

	frameAxleAngle := math.Atan2(dz, dx)
	worldAxleAngle := math.Asin(dzWorld / axleSpan)
	pitch := Rad(frameAxleAngle - worldAxleAngle) // Positive = nose-down rotation around +Y

	// 3. Place rear axle at (0, 0, rearWheel.LoadedRadius) and orient the pitched frame.
	b := NewBuilder()
	rearAxlePitched := b.StartAt(Pose{
		Origin: WorldPoint{X: 0, Y: 0, Z: rearWheel.LoadedRadius},
		Basis:  RotY(pitch),
	})

	bb := rearAxlePitched.Move(-rearAxleFrame.X, 0, -rearAxleFrame.Z)
	frontAxle := bb.Move(frontAxleFrame.X, 0, frontAxleFrame.Z)

	// Emit primary frame & fork tubes using the Cursor builder:
	bb.ExtrudeVec(rearAxleFrame, 24) // Chainstay centerline

	effSTA := frame.SeatTubeAngle.Rad() + pitch
	seatTubeTilt := (Rad(math.Pi/2.0) - effSTA).Deg()
	seatCollar := bb.Pitch(-seatTubeTilt).ExtrudeVec(LocalVec{Z: frame.SeatTubeLength}, 34.9)

	effHTA := hta + pitch
	steererTilt := (Rad(math.Pi/2.0) - effHTA).Deg()
	htTop := bb.Move(frame.Reach, 0, frame.Stack).Pitch(-steererTilt)
	htTop.ExtrudeVec(LocalVec{Z: -frame.HeadTubeLength}, 44) // Head tube

	// Advance up the steerer through upper headset cover and spacers to the stem mounting base:
	steererStemBase := htTop.ExtrudeVec(LocalVec{Z: headset.UpperStack + spacerHeight}, 28.6)

	return RollingChassis{
		Pitch:         pitch,
		BottomBracket: bb.Pose(),
		RearAxle:      Pose{Origin: rearAxlePitched.Pose().Origin, Basis: IdentityMat3()},
		FrontAxle:     Pose{Origin: frontAxle.Pose().Origin, Basis: IdentityMat3()},
		SteererOut:    MakeSocket[Steerer](steererStemBase),
		SeatCollarOut: MakeSocket[SeatCollar](seatCollar),
		Prims:         b.Primitives(),
	}, nil
}
```

---

### 4.6 Type-Safe Assembly (`Attach`) and Complete `Solve` Entry Point

The generic `Attach` function mates any child `Plug[R]` to a parent `Socket[R]` in world space, transforming all of the child's 3D primitives automatically. Attempting to pass mismatched roles (such as attaching a `HandlebarPart` directly to `SteererOut`) is rejected by the Go compiler.

```go
// Attach mates a child part's local Plug[R] to a parent's world-space Socket[R],
// returning the child part's world Pose and its transformed 3D primitives.
func Attach[R Role](
	parentWorldSocket Socket[R],
	childLocalPlug Plug[R],
	childPrims []Primitive,
) (Pose, []Primitive) {
	// T_world_child = T_world_socket * (T_child_plug)^-1
	worldPose := parentWorldSocket.Pose().Compose(childLocalPlug.Pose().Inverse())

	placed := make([]Primitive, len(childPrims))
	for i, p := range childPrims {
		placed[i] = p.Transform(worldPose)
	}
	return worldPose, placed
}

// PlaceSocket transforms a part-local Socket[R] into world space once its owning part is attached.
func PlaceSocket[R Role](partWorldPose Pose, localSocket Socket[R]) Socket[R] {
	return Socket[R]{pose: partWorldPose.Compose(localSocket.Pose())}
}

type SaddleSpec struct {
	RailHeight    MM  // Rail center to sit-bone contact surface
	ForeAftOffset MM  // Clamp position relative to sit-bone reference (+ = saddle pushed forward)
	Tilt          Deg // Nose-down tilt angle relative to world horizontal
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

type SolvedBike struct {
	Chassis   RollingChassis
	StemClamp Socket[BarClamp]
	LeftGrip  Socket[Grip]
	RightGrip Socket[Grip]
	SaddleSit Socket[SaddleTop]
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

	chassis, err := AssembleChassis(
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

	allPrims := append([]Primitive(nil), chassis.Prims...)

	// 1. Attach Stem (Plug[Steerer]) to Chassis (Socket[Steerer])
	stemPart := NewStem(spec.Stem)
	stemPose, stemPrims := Attach(chassis.SteererOut, stemPart.In, stemPart.Prims)
	allPrims = append(allPrims, stemPrims...)
	worldBarClamp := PlaceSocket(stemPose, stemPart.BarMount)

	// 2. Attach Handlebar (Plug[BarClamp]) to Stem (Socket[BarClamp])
	barPart := NewSweptHandlebar(spec.Bar)
	barPose, barPrims := Attach(worldBarClamp, barPart.In, barPart.Prims)
	allPrims = append(allPrims, barPrims...)
	leftGrip := PlaceSocket(barPose, barPart.LeftGrip)
	rightGrip := PlaceSocket(barPose, barPart.RightGrip)

	// 3. Attach Seatpost (Plug[SeatCollar]) to Chassis (Socket[SeatCollar])
	postPart := NewSeatpost(spec.Seatpost)
	postPose, postPrims := Attach(chassis.SeatCollarOut, postPart.In, postPart.Prims)
	allPrims = append(allPrims, postPrims...)
	worldRailMount := PlaceSocket(postPose, postPart.RailMount)

	// 4. Resolve Saddle Sit-Bone Pose (leveled to world horizontal before applying Saddle.Tilt)
	b := NewBuilder()
	saddleSit := b.StartAt(Pose{Origin: worldRailMount.Pose().Origin, Basis: IdentityMat3()}).
		Pitch(spec.Saddle.Tilt).
		Move(spec.Saddle.ForeAftOffset, 0, spec.Saddle.RailHeight)

	return SolvedBike{
		Chassis:   chassis,
		StemClamp: worldBarClamp,
		LeftGrip:  leftGrip,
		RightGrip: rightGrip,
		SaddleSit: MakeSocket[SaddleTop](saddleSit),
		Prims:     allPrims,
	}, nil
}

// Derived World-Space Fit & Handling Metrics:

func (b SolvedBike) Wheelbase() MM {
	return b.Chassis.FrontAxle.Origin.X - b.Chassis.RearAxle.Origin.X
}

func (b SolvedBike) SaddleToGripReach() MM {
	return b.LeftGrip.Pose().Origin.X - b.SaddleSit.Pose().Origin.X
}

func (b SolvedBike) SaddleToGripDrop() MM {
	return b.SaddleSit.Pose().Origin.Z - b.LeftGrip.Pose().Origin.Z
}
```