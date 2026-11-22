package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// FrameSetup specifies the bare frame dimensions in its unpitched reference
// attitude (where reference Reach is horizontal and reference Stack is vertical).
type FrameSetup struct {
	ReachMm          float64
	StackMm          float64
	HeadTubeAngleDeg pose.Deg
	HeadTubeLengthMm float64
	SeatTubeAngleDeg pose.Deg
	SeatTubeLengthMm float64
	BbDropMm         float64
	ChainstayMm      float64

	// LowerBore is the head tube's lower bore. DefaultLowerCup is the frame's
	// stock physical lower cup, which must fit LowerBore.
	LowerBore       LowerBore
	DefaultLowerCup LowerCupSetup

	// MaxPostInsertionMm is the frame's published seatpost insertion cap; zero
	// means undeclared, which caps insertion at the seat tube's C-T length less
	// 20 mm. MinFrameInsertionMm is the frame's minimum insertion; zero means none.
	MaxPostInsertionMm  float64
	MinFrameInsertionMm float64
}

// LowerBore identifies the head tube's lower bore, which sets the lower
// headset cup standards the frame accepts.
type LowerBore string

const (
	Bore34Pressed    LowerBore = "34_pressed"
	Bore44Pressed    LowerBore = "44_pressed"
	Bore56Pressed    LowerBore = "56_pressed"
	Bore42Integrated LowerBore = "42_integrated"
	Bore52Integrated LowerBore = "52_integrated"
)

// Steerer identifies a fork's steerer tube type. SteererBasis is the sentinel for
// the basis fork, which has no physical steerer.
type Steerer string

const (
	SteererStraight118 Steerer = "straight_118"
	SteererTapered     Steerer = "tapered"
	SteererBasis       Steerer = "basis"
)

// LowerCupStandard identifies the SHIS lower headset cup interface marrying a
// frame's head tube to a fork steerer (or BareHeadTube for an uncupped basis chart).
type LowerCupStandard string

const (
	LowerCupEC34         LowerCupStandard = "EC34"
	LowerCupEC44         LowerCupStandard = "EC44"
	LowerCupZS44         LowerCupStandard = "ZS44"
	LowerCupZS56         LowerCupStandard = "ZS56"
	LowerCupIS42         LowerCupStandard = "IS42"
	LowerCupIS52         LowerCupStandard = "IS52"
	LowerCupBareHeadTube LowerCupStandard = "bare_head_tube"
)

// LowerCupSetup specifies the lower headset cup standard and axial stack height
// marrying a frame's head tube to a fork steerer.
type LowerCupSetup struct {
	Standard LowerCupStandard
	StackMm  float64
}

// Validate checks that the lower headset cup standard is recognized and its
// stack height satisfies the physical constraint for that standard.
func (c LowerCupSetup) Validate() error {
	if !isFinite(c.StackMm) {
		return invalidInput(InputLowerCup, "lower headset cup must be finite, got %v mm", c.StackMm)
	}
	switch c.Standard {
	case LowerCupEC34, LowerCupEC44:
		if c.StackMm <= 0 {
			return invalidInput(InputLowerCup, "%s external lower headset cup must have positive stack, got %v mm", c.Standard, c.StackMm)
		}
		return nil
	case LowerCupZS44, LowerCupZS56, LowerCupIS42, LowerCupIS52:
		if c.StackMm < 0 {
			return invalidInput(InputLowerCup, "%s lower headset cup must be non-negative, got %v mm", c.Standard, c.StackMm)
		}
		return nil
	case LowerCupBareHeadTube:
		if c.StackMm != 0 {
			return invalidInput(InputLowerCup, "bare head tube has no lower headset cup (must be 0 mm), got %v mm", c.StackMm)
		}
		return nil
	default:
		return invalidInput(InputLowerCup, "unknown or missing lower headset cup standard %q", c.Standard)
	}
}

// cupBore returns the head tube bore a physical lower cup standard fits, and
// false for the bare head tube (which fits any bore).
func cupBore(std LowerCupStandard) (LowerBore, bool) {
	switch std {
	case LowerCupEC34:
		return Bore34Pressed, true
	case LowerCupEC44, LowerCupZS44:
		return Bore44Pressed, true
	case LowerCupZS56:
		return Bore56Pressed, true
	case LowerCupIS42:
		return Bore42Integrated, true
	case LowerCupIS52:
		return Bore52Integrated, true
	case LowerCupBareHeadTube:
		return "", false
	default:
		return "", false
	}
}

// cupTakesTaperedSteerer reports whether a physical cup standard accepts a
// tapered steerer (every standard accepts a straight one).
func cupTakesTaperedSteerer(std LowerCupStandard) bool {
	switch std {
	case LowerCupEC44, LowerCupZS56, LowerCupIS52:
		return true
	case LowerCupEC34, LowerCupZS44, LowerCupIS42, LowerCupBareHeadTube:
		return false
	default:
		return false
	}
}

// validateHeadset checks that the lower cup is valid, fits the head tube's
// lower bore, and accepts the fork's steerer. A bare head tube is a basis
// hypothesis: it fits any recognized bore but only the basis steerer, and a
// physical fork always requires a physical cup. SteererBasis fits any valid cup
// on its matching bore.
func validateHeadset(bore LowerBore, cup LowerCupSetup, steerer Steerer) error {
	switch bore {
	case Bore34Pressed, Bore44Pressed, Bore56Pressed, Bore42Integrated, Bore52Integrated:
	default:
		return invalidInput(InputLowerCup, "unknown or missing head tube lower bore %q", bore)
	}
	switch steerer {
	case SteererStraight118, SteererTapered, SteererBasis:
	default:
		return invalidInput(InputLowerCup, "unknown or missing fork steerer %q", steerer)
	}
	if err := cup.Validate(); err != nil {
		return err
	}
	cupFits, physical := cupBore(cup.Standard)
	if !physical {
		if steerer != SteererBasis {
			return invalidInput(InputLowerCup, "a %s steerer fork needs a physical lower cup, not %s", steerer, cup.Standard)
		}
		return nil
	}
	if cupFits != bore {
		return invalidInput(InputLowerCup, "%s lower cup needs a %s head tube bore, but the frame has %s", cup.Standard, cupFits, bore)
	}
	switch steerer {
	case SteererStraight118, SteererBasis:
		return nil
	case SteererTapered:
		if !cupTakesTaperedSteerer(cup.Standard) {
			return invalidInput(InputLowerCup, "%s lower cup does not fit a tapered steerer", cup.Standard)
		}
		return nil
	default:
		return nil
	}
}

// validateDefaultLowerCup checks the frame's declared default as a physical
// cup of the frame's bore.
func validateDefaultLowerCup(frame FrameSetup) error {
	if frame.DefaultLowerCup.Standard == LowerCupBareHeadTube {
		return invalidInput(InputLowerCup, "frame default lower cup must be a physical cup, not %s", frame.DefaultLowerCup.Standard)
	}
	return validateHeadset(frame.LowerBore, frame.DefaultLowerCup, SteererBasis)
}

// ForkSpec describes a catalog fork before installation and sag.
type ForkSpec struct {
	StaticA2cMm float64
	TravelMm    float64 // 0 for rigid forks
	RakeMm      float64
	Steerer     Steerer
}

// ForkGeometry holds the resolved fixed fork dimensions (effective axle-to-crown
// and rake) used by the frameset Pose recipe.
type ForkGeometry struct {
	A2cMm  float64
	RakeMm float64
}

// ForkSetup holds the physical installation inputs for a fork (its catalog
// ForkSpec and installed SagPct) that PlaceFrameset resolves and validates.
type ForkSetup struct {
	Spec   ForkSpec
	SagPct float64
}

// EffectiveA2cMm returns the fork's effective (sagged) axle-to-crown length.
func (f ForkSetup) EffectiveA2cMm() float64 {
	return f.Spec.StaticA2cMm - f.Spec.TravelMm*(f.SagPct/100.0)
}

// Geometry resolves the fork setup's effective A2C and rake into ForkGeometry.
func (f ForkSetup) Geometry() ForkGeometry {
	return ForkGeometry{
		A2cMm:  f.EffectiveA2cMm(),
		RakeMm: f.Spec.RakeMm,
	}
}

// Validate checks that the fork specification, sag percentage, and resulting
// effective A2C are valid physical inputs.
func (f ForkSetup) Validate() error {
	if !isFinite(f.Spec.StaticA2cMm) || f.Spec.StaticA2cMm <= 0 {
		return invalidInput(InputForkA2c, "fork static A2C must be finite and positive, got %v mm", f.Spec.StaticA2cMm)
	}
	if !isFinite(f.Spec.TravelMm) || f.Spec.TravelMm < 0 {
		return fmt.Errorf("fork travel must be finite and non-negative, got %v mm", f.Spec.TravelMm)
	}
	if f.Spec.Steerer == SteererBasis && f.Spec.TravelMm != 0 {
		return fmt.Errorf("basis fork cannot have travel, got %v mm", f.Spec.TravelMm)
	}
	if !isFinite(f.SagPct) || f.SagPct < 0 || f.SagPct > 100 {
		return invalidInput(InputForkSag, "fork sag must be in [0%%, 100%%], got %v%%", f.SagPct)
	}
	if f.Spec.TravelMm == 0 && f.SagPct != 0 {
		return invalidInput(InputForkSag, "rigid fork (0 mm travel) cannot have non-zero sag, got %v%%", f.SagPct)
	}
	eff := f.EffectiveA2cMm()
	if !isFinite(eff) || eff <= 0 {
		return invalidInput(InputForkA2c, "fork effective A2C must be finite and positive, got %v mm", eff)
	}
	if !isFinite(f.Spec.RakeMm) {
		return fmt.Errorf("fork rake must be finite, got %v mm", f.Spec.RakeMm)
	}
	return nil
}

// WheelSetup specifies the wheel radius used to locate tire-ground contact
// points below the leveled axles.
type WheelSetup struct {
	RadiusMm float64
}

// FramesetPlacement holds the Pose-valued landmarks for a frameset (frame,
// fork, lower headset cup, and wheel-ground contacts), anchored with BB at
// (0, 0) (+X forward, +Y up). PlaceFrameset returns it leveled (horizontal
// axle line, RearAxle.Origin().Y == FrontAxle.Origin().Y, with RearTireContact
// and FrontTireContact on one horizontal ground line); PlaceUnpitchedFrameset
// returns the same landmarks at 0° pitch (each tire contact one wheel radius
// straight below its axle).
//
// Each landmark is the Pose produced at that point as the turtle constructs
// ("draws") the frameset from BB, so its local +X heading is a direct
// consequence of the construction path:
//   - BB and RearAxle carry the leveled frame reference axis (world PitchDeg).
//   - SeatAxis (at BB) and SeatTubeTop point up and rearward along the leveled
//     seat tube axis (180° - SeatTubeAngleDeg).
//   - HeadTubeTop points up and rearward along the leveled steerer axis
//     (180° - HeadTubeAngleDeg), ready for PlaceStem; HeadTubeBottom,
//     LowerCupBottom, and ForkSteererBase point down the steerer (-HeadTubeAngleDeg).
//   - FrontAxle points forward and up along the fork rake direction (90° -
//     HeadTubeAngleDeg, perpendicular to the steerer).
//   - RearTireContact and FrontTireContact point straight down (-90°) to the ground.
type FramesetPlacement struct {
	BB               pose.Pose
	RearAxle         pose.Pose
	RearTireContact  pose.Pose
	SeatAxis         pose.Pose
	SeatTubeTop      pose.Pose
	HeadTubeTop      pose.Pose
	HeadTubeBottom   pose.Pose
	LowerCupBottom   pose.Pose
	ForkSteererBase  pose.Pose
	FrontAxle        pose.Pose
	FrontTireContact pose.Pose
}

// PitchDeg returns the counter-clockwise frame leveling pitch applied at BB.
func (f FramesetPlacement) PitchDeg() pose.Deg {
	return f.BB.HeadingDeg()
}

// LevelingPitchDeg returns the counter-clockwise rotation about BB that
// would put the axles level: nonzero on an unpitched placement whose fork
// and cup differ from the chart's basis, and 0 (to rounding) on a
// placement PlaceFrameset returned.
func (f FramesetPlacement) LevelingPitchDeg() pose.Deg {
	return -f.RearAxle.AngleTo(f.FrontAxle)
}

// PitchDeltaDeg returns f's leveling pitch minus reference's: positive when
// f's front end sits higher, as an installed fork does against the basis fork.
func (f FramesetPlacement) PitchDeltaDeg(reference FramesetPlacement) pose.Deg {
	return f.PitchDeg() - reference.PitchDeg()
}

// LeveledHeadTubeAngleDeg returns the leveled head tube angle in degrees.
func (f FramesetPlacement) LeveledHeadTubeAngleDeg() pose.Deg {
	return 180.0 - f.HeadTubeTop.HeadingDeg()
}

// LeveledSeatTubeAngleDeg returns the leveled seat tube angle in degrees.
func (f FramesetPlacement) LeveledSeatTubeAngleDeg() pose.Deg {
	return 180.0 - f.SeatAxis.HeadingDeg()
}

// WheelbaseMm returns the horizontal distance between the leveled rear and front axles.
func (f FramesetPlacement) WheelbaseMm() float64 {
	return f.RearAxle.DistanceTo(f.FrontAxle)
}

// LeveledBbDropMm returns the vertical drop from the leveled axle line to BB.
func (f FramesetPlacement) LeveledBbDropMm() float64 {
	return f.BB.VerticalOffsetTo(f.RearAxle)
}

// TrailMm returns the ground trail: the horizontal distance on the ground from
// the front tire contact back to where the leveled steering axis meets the
// ground. Positive when the contact sits behind that point.
func (f FramesetPlacement) TrailMm() float64 {
	// Split HeadTubeTop → FrontTireContact into a run down the steering axis
	// and a horizontal remainder. The steering axis meets the ground (the
	// contact's height) at HeadTubeTop plus the run, so the remainder runs
	// from that point to the contact, and trail is its negation.
	steererDown := f.HeadTubeTop.HeadingDeg() + 180.0
	_, ahead, ok := pose.DecomposeAlong(f.HeadTubeTop.OffsetTo(f.FrontTireContact), steererDown, 0)
	if !ok {
		return math.NaN() // a horizontal steering axis never meets the ground
	}
	return -ahead
}

// RearCenterMm returns the horizontal distance from the rear axle to BB on the
// leveled bike (the RC in RC / WB).
func (f FramesetPlacement) RearCenterMm() float64 {
	return f.RearAxle.OffsetTo(f.BB).X
}

// RcWbRatioPct returns RearCenterMm as a percentage of WheelbaseMm.
func (f FramesetPlacement) RcWbRatioPct() float64 {
	return 100.0 * f.RearCenterMm() / f.WheelbaseMm()
}

// LeveledReachMm returns the horizontal distance from BB to HeadTubeTop on the
// leveled bike. It differs from the chart's reach by the leveling pitch (Q105).
func (f FramesetPlacement) LeveledReachMm() float64 {
	return f.BB.OffsetTo(f.HeadTubeTop).X
}

// LeveledStackMm returns the vertical distance from BB to HeadTubeTop on the
// leveled bike (Q105).
func (f FramesetPlacement) LeveledStackMm() float64 {
	return f.BB.VerticalOffsetTo(f.HeadTubeTop)
}

// EttMm returns the effective top tube: the horizontal distance, at
// HeadTubeTop's height, from the leveled seat axis through BB to HeadTubeTop
// (legacy BikeScene.EttMm, Q105).
func (f FramesetPlacement) EttMm() float64 {
	// Split BB → HeadTubeTop into a run up the seat axis and a horizontal
	// remainder; the remainder is the ETT.
	_, ett, ok := pose.DecomposeAlong(f.SeatAxis.OffsetTo(f.HeadTubeTop), f.SeatAxis.HeadingDeg(), 0)
	if !ok {
		return math.NaN() // a horizontal seat axis never reaches HeadTubeTop's height
	}
	return ett
}

// PlaceUnpitchedFrameset builds the frameset in the chart's own orientation:
// BB at the origin, frame reference axis horizontal (0° pitch). The axles are
// level only when the fork and cup reproduce the chart's basis, and
// LevelingPitchDeg measures how far off they are. Tire contacts sit one wheel
// radius straight below each axle.
func PlaceUnpitchedFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error) {
	if err := validateFramesetInputs(frame, lowerCup, fork, wheels); err != nil {
		return FramesetPlacement{}, err
	}
	return placeFramesetAtPitch(0, frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
}

// PlaceFrameset constructs a leveled frameset (frame, lower headset cup, fork,
// and wheel-ground contacts) using a two-pass Pose recipe: Pass 1 calls
// PlaceUnpitchedFrameset to measure the axle-leveling pitch at 0° frame pitch,
// and Pass 2 builds the leveled frameset rooted at BB rotated by that pitch and
// locates the wheel-ground contact landmarks directly below each axle.
func PlaceFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error) {
	unpitched, err := PlaceUnpitchedFrameset(frame, lowerCup, fork, wheels)
	if err != nil {
		return FramesetPlacement{}, err
	}
	return placeFramesetAtPitch(unpitched.LevelingPitchDeg(), frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
}

func placeFramesetAtPitch(pitchDeg pose.Deg, frame FrameSetup, lowerCupMm float64, fork ForkGeometry, wheels WheelSetup) FramesetPlacement {
	frameRoot := pose.NewPose(pose.Point{}, 0).Turn(pitchDeg)
	rearX := -pose.RightTriangleLeg(frame.ChainstayMm, frame.BbDropMm)
	rearAxle := frameRoot.Translate(rearX, frame.BbDropMm)

	headTubeTop := frameRoot.Translate(frame.ReachMm, frame.StackMm).
		Turn(180.0 - frame.HeadTubeAngleDeg) // heading along the steerer axis, pointing up
	steererDown := headTubeTop.Turn(180.0)
	headTubeBottom := steererDown.Advance(frame.HeadTubeLengthMm)
	lowerCupBottom := headTubeBottom.Advance(lowerCupMm)
	forkSteererBase := lowerCupBottom.Advance(fork.A2cMm)
	// Turning 90° from steerer-down points forward: the fork rake direction.
	frontAxle := forkSteererBase.Turn(90.0).Advance(fork.RakeMm)

	// The seat tube axis points up and back: 180° minus the seat tube angle.
	seatAxis := frameRoot.Turn(180.0 - frame.SeatTubeAngleDeg)
	seatTubeTop := seatAxis.Advance(frame.SeatTubeLengthMm)

	// Face straight down (-90°), then advance one wheel radius to the ground.
	rearTireContact := rearAxle.Face(-90.0).Advance(wheels.RadiusMm)
	frontTireContact := frontAxle.Face(-90.0).Advance(wheels.RadiusMm)

	return FramesetPlacement{
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

func validateWheels(wheels WheelSetup) error {
	if !isFinite(wheels.RadiusMm) || wheels.RadiusMm <= 0 {
		return fmt.Errorf("wheel radius must be finite and positive, got %v mm", wheels.RadiusMm)
	}
	return nil
}

func validateFrameGeometry(frame FrameSetup) error {
	if !isFinite(frame.ReachMm) || frame.ReachMm <= 0 {
		return fmt.Errorf("reach must be finite and positive, got %v mm", frame.ReachMm)
	}
	if !isFinite(frame.StackMm) || frame.StackMm <= 0 {
		return fmt.Errorf("stack must be finite and positive, got %v mm", frame.StackMm)
	}
	if !isFinite(float64(frame.HeadTubeAngleDeg)) || frame.HeadTubeAngleDeg <= 0 || frame.HeadTubeAngleDeg >= 180 {
		return fmt.Errorf("head tube angle must be in (0°, 180°), got %v°", frame.HeadTubeAngleDeg)
	}
	if !isFinite(float64(frame.SeatTubeAngleDeg)) || frame.SeatTubeAngleDeg <= 0 || frame.SeatTubeAngleDeg >= 180 {
		return fmt.Errorf("seat tube angle must be in (0°, 180°), got %v°", frame.SeatTubeAngleDeg)
	}
	if !isFinite(frame.HeadTubeLengthMm) || frame.HeadTubeLengthMm <= 0 {
		return fmt.Errorf("head tube length must be finite and positive, got %v mm", frame.HeadTubeLengthMm)
	}
	if !isFinite(frame.BbDropMm) || !isFinite(frame.ChainstayMm) || frame.ChainstayMm <= math.Abs(frame.BbDropMm) {
		return fmt.Errorf("chainstay (%v mm) must exceed |BB drop| (%v mm)", frame.ChainstayMm, frame.BbDropMm)
	}
	if !isFinite(frame.SeatTubeLengthMm) || frame.SeatTubeLengthMm <= 0 {
		return fmt.Errorf("seat tube length must be finite and positive, got %v mm", frame.SeatTubeLengthMm)
	}
	return validateDefaultLowerCup(frame)
}

func validateFramesetInputs(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) error {
	if err := validateFrameGeometry(frame); err != nil {
		return err
	}
	if err := fork.Validate(); err != nil {
		return err
	}
	if err := validateHeadset(frame.LowerBore, lowerCup, fork.Spec.Steerer); err != nil {
		return err
	}
	if err := validateWheels(wheels); err != nil {
		return err
	}
	return nil
}
