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
}

// LowerCupStandard identifies the SHIS lower headset cup interface marrying a
// frame's head tube to a fork steerer (or BareHeadTube for an uncupped CAD chart).
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

// ForkSpec describes a catalog fork before installation and sag.
type ForkSpec struct {
	StaticA2cMm float64
	TravelMm    float64 // 0 for rigid forks
	RakeMm      float64
}

// ForkGeometry holds the resolved fixed fork dimensions (effective axle-to-crown
// and rake) used by the frameset Pose recipe and FrameChart.CadFork.
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

// FramesetPlacement holds the leveled Pose-valued landmarks for a grounded
// frameset (frame, fork, lower headset cup, and wheel-ground contacts), anchored
// with BB at (0, 0) (+X forward, +Y up) and a horizontal axle line
// (RearAxle.Origin().Y == FrontAxle.Origin().Y). RearTireContact and
// FrontTireContact sit vertically below the leveled axles on the horizontal ground
// plane (RearTireContact.Origin().Y == FrontTireContact.Origin().Y).
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

// FrameChart groups a bare frame's unpitched reference dimensions, its CAD
// lower headset cup, its fixed CAD baseline fork geometry, and its published wheelbase.
type FrameChart struct {
	Frame                FrameSetup
	CadLowerCup          LowerCupSetup
	CadFork              ForkGeometry
	PublishedWheelbaseMm float64
	Precision            ChartPrecision // rounding half-widths of the published columns (Q43)
}

// ChartBaseline holds the leveled CAD baseline fork frameset placement.
type ChartBaseline struct {
	Placement FramesetPlacement
}

// DeltaPitchDeg returns the pitch change of installed relative to the CAD baseline fork.
func (b ChartBaseline) DeltaPitchDeg(installed FramesetPlacement) pose.Deg {
	return installed.PitchDeg() - b.Placement.PitchDeg()
}

// PlaceCadBaselineFork runs the unpitched (0°) frameset recipe once to measure
// the leveling pitch, then builds the leveled CAD baseline fork frameset.
func PlaceCadBaselineFork(chart FrameChart, wheels WheelSetup) (ChartBaseline, error) {
	if err := validateFrameGeometry(chart.Frame); err != nil {
		return ChartBaseline{}, err
	}
	if err := validateForkGeometry(chart.CadFork); err != nil {
		return ChartBaseline{}, err
	}
	if err := chart.CadLowerCup.Validate(); err != nil {
		return ChartBaseline{}, err
	}
	if err := validateWheels(wheels); err != nil {
		return ChartBaseline{}, err
	}

	_, pitchDeg := measureUnpitchedFrameset(chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork)

	return ChartBaseline{
		Placement: placeLeveledFrameset(chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork, wheels, pitchDeg, nil),
	}, nil
}

// PlaceFrameset constructs a leveled frameset (frame, lower headset cup, fork,
// and wheel-ground contacts) using a two-pass Pose recipe: Pass 1 measures the
// axle-leveling pitch at 0° frame pitch, and Pass 2 builds the leveled frameset
// rooted at BB rotated by that pitch and locates the wheel-ground contact
// landmarks directly below each axle.
func PlaceFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup, trace *pose.Trace) (FramesetPlacement, error) {
	if err := validateFramesetInputs(frame, lowerCup, fork, wheels); err != nil {
		return FramesetPlacement{}, err
	}

	geom := fork.Geometry()
	_, pitchDeg := measureUnpitchedFrameset(frame, lowerCup.StackMm, geom)
	return placeLeveledFrameset(frame, lowerCup.StackMm, geom, wheels, pitchDeg, trace), nil
}

func measureUnpitchedFrameset(frame FrameSetup, lowerCupMm float64, fork ForkGeometry) (FramesetPlacement, pose.Deg) {
	unpitched := placeFramesetAtPitch(pose.NewPose(pose.Point{}, 0, nil), frame, lowerCupMm, fork)
	return unpitched, -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)
}

func placeLeveledFrameset(frame FrameSetup, lowerCupMm float64, fork ForkGeometry, wheels WheelSetup, pitchDeg pose.Deg, trace *pose.Trace) FramesetPlacement {
	frameRoot := pose.NewPose(pose.Point{}, 0, trace).Turn("frame pitch to level axles", pitchDeg)
	leveled := placeFramesetAtPitch(frameRoot, frame, lowerCupMm, fork)
	leveled.RearTireContact = leveled.RearAxle.Face("down to ground", -90.0).Advance("rear wheel radius", wheels.RadiusMm)
	leveled.FrontTireContact = leveled.FrontAxle.Face("down to ground", -90.0).Advance("front wheel radius", wheels.RadiusMm)
	return leveled
}

func placeFramesetAtPitch(frameRoot pose.Pose, frame FrameSetup, lowerCupMm float64, fork ForkGeometry) FramesetPlacement {
	rearX := -pose.RightTriangleLeg(frame.ChainstayMm, frame.BbDropMm)
	rearAxle := frameRoot.Translate("rear axle", rearX, frame.BbDropMm)

	headTubeTop := frameRoot.Translate("head tube top (reach, stack)", frame.ReachMm, frame.StackMm).
		Turn("steerer axis (180° - HTA)", 180.0-frame.HeadTubeAngleDeg)
	steererDown := headTubeTop.Turn("down steerer", 180.0)
	headTubeBottom := steererDown.Advance("head tube length", frame.HeadTubeLengthMm)
	lowerCupBottom := headTubeBottom.Advance("lower headset cup", lowerCupMm)
	forkSteererBase := lowerCupBottom.Advance("fork A2C", fork.A2cMm)
	frontAxle := forkSteererBase.Turn("fork rake direction", 90.0).Advance("fork rake", fork.RakeMm)

	seatAxis := frameRoot.Turn("seat tube axis (180° - STA)", 180.0-frame.SeatTubeAngleDeg)
	seatTubeTop := seatAxis.Advance("seat tube length", frame.SeatTubeLengthMm)

	return FramesetPlacement{
		BB:              frameRoot,
		RearAxle:        rearAxle,
		SeatAxis:        seatAxis,
		SeatTubeTop:     seatTubeTop,
		HeadTubeTop:     headTubeTop,
		HeadTubeBottom:  headTubeBottom,
		LowerCupBottom:  lowerCupBottom,
		ForkSteererBase: forkSteererBase,
		FrontAxle:       frontAxle,
	}
}

func validateForkGeometry(fork ForkGeometry) error {
	if !isFinite(fork.A2cMm) || fork.A2cMm <= 0 {
		return invalidInput(InputForkA2c, "fork A2C must be finite and positive, got %v mm", fork.A2cMm)
	}
	if !isFinite(fork.RakeMm) {
		return fmt.Errorf("fork rake must be finite, got %v mm", fork.RakeMm)
	}
	return nil
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
	return nil
}

func validateFramesetInputs(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) error {
	if err := validateFrameGeometry(frame); err != nil {
		return err
	}
	if err := fork.Validate(); err != nil {
		return err
	}
	if err := lowerCup.Validate(); err != nil {
		return err
	}
	if err := validateWheels(wheels); err != nil {
		return err
	}
	return nil
}
