package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// FrameSetup specifies the frame dimensions in its unpitched reference attitude
// (where reference Reach is horizontal and reference Stack is vertical).
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

// ForkSetup holds the resolved physical dimensions (effective axle-to-crown,
// rake, and lower headset cup stack) that PlaceChassis draws.
type ForkSetup struct {
	A2cMm      float64
	RakeMm     float64
	LowerCupMm float64
}

// ForkSpec describes a catalog fork before installation and sag.
type ForkSpec struct {
	StaticA2cMm float64
	TravelMm    float64 // 0 for rigid forks
	RakeMm      float64
}

// Setup resolves the fork specification at sagPct with lowerCupMm into a
// ForkSetup ready for PlaceChassis.
func (s ForkSpec) Setup(sagPct, lowerCupMm float64) (ForkSetup, error) {
	if !isFinite(s.StaticA2cMm) || s.StaticA2cMm <= 0 {
		return ForkSetup{}, invalidInput(InputForkA2c, "fork static A2C must be finite and positive, got %v mm", s.StaticA2cMm)
	}
	if !isFinite(s.TravelMm) || s.TravelMm < 0 {
		return ForkSetup{}, fmt.Errorf("fork travel must be finite and non-negative, got %v mm", s.TravelMm)
	}
	if !isFinite(sagPct) || sagPct < 0 || sagPct > 100 {
		return ForkSetup{}, invalidInput(InputForkSag, "fork sag must be in [0%%, 100%%], got %v%%", sagPct)
	}
	if s.TravelMm == 0 && sagPct != 0 {
		return ForkSetup{}, invalidInput(InputForkSag, "rigid fork (0 mm travel) cannot have non-zero sag, got %v%%", sagPct)
	}
	setup := ForkSetup{
		A2cMm:      s.StaticA2cMm - s.TravelMm*(sagPct/100.0),
		RakeMm:     s.RakeMm,
		LowerCupMm: lowerCupMm,
	}
	if err := validateForkSetup(setup); err != nil {
		return ForkSetup{}, err
	}
	return setup, nil
}

// WheelSetup specifies the wheel radius used to locate tire-ground contact
// points below the leveled axles.
type WheelSetup struct {
	RadiusMm float64
}

// ChassisPlacement holds the leveled Pose-valued landmarks for a frame, fork,
// and wheels, anchored with BB at (0, 0) (+X forward, +Y up) and a horizontal
// axle line (RearAxle.Origin().Y == FrontAxle.Origin().Y). RearTireContact and
// FrontTireContact sit vertically below the leveled axles on the horizontal ground
// plane (RearTireContact.Origin().Y == FrontTireContact.Origin().Y).
//
// Each landmark is the Pose produced at that point as the turtle constructs
// ("draws") the chassis from BB, so its local +X heading is a direct
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
type ChassisPlacement struct {
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
func (c ChassisPlacement) PitchDeg() pose.Deg {
	return c.BB.HeadingDeg()
}

// LeveledHeadTubeAngleDeg returns the leveled head tube angle in degrees.
func (c ChassisPlacement) LeveledHeadTubeAngleDeg() pose.Deg {
	return 180.0 - c.HeadTubeTop.HeadingDeg()
}

// LeveledSeatTubeAngleDeg returns the leveled seat tube angle in degrees.
func (c ChassisPlacement) LeveledSeatTubeAngleDeg() pose.Deg {
	return 180.0 - c.SeatAxis.HeadingDeg()
}

// WheelbaseMm returns the horizontal distance between the leveled rear and front axles.
func (c ChassisPlacement) WheelbaseMm() float64 {
	return c.RearAxle.DistanceTo(c.FrontAxle)
}

// LeveledBbDropMm returns the vertical drop from the leveled axle line to BB.
func (c ChassisPlacement) LeveledBbDropMm() float64 {
	return c.BB.VerticalOffsetTo(c.RearAxle)
}

// FrameChart groups a frame's unpitched reference dimensions with its CAD
// baseline fork and published wheelbase.
type FrameChart struct {
	Frame                FrameSetup
	CadFork              ForkSetup
	PublishedWheelbaseMm float64
}

// ChartResiduals holds the raw 0°-pitch chart-closure residuals when a
// FrameChart is evaluated with its CAD baseline fork before leveling.
type ChartResiduals struct {
	AxleHeightDiffMm float64 // FrontAxle.Y - RearAxle.Y at 0° pitch
	WheelbaseDiffMm  float64 // (FrontAxle.X - RearAxle.X) - publishedWheelbaseMm at 0° pitch
}

// ChartBaseline holds the leveled CAD-baseline chassis placement and the raw
// 0°-pitch chart-closure residuals computed from a single unpitched pass.
type ChartBaseline struct {
	Placement ChassisPlacement
	Residuals ChartResiduals
}

// DeltaPitchDeg returns the pitch change of installed relative to the CAD baseline.
func (b ChartBaseline) DeltaPitchDeg(installed ChassisPlacement) pose.Deg {
	return installed.PitchDeg() - b.Placement.PitchDeg()
}

// PlaceCadBaseline runs the unpitched (0°) chassis recipe once to measure both
// the raw chart-closure residuals and the leveling pitch, then builds the
// leveled CAD-baseline chassis.
func PlaceCadBaseline(chart FrameChart, wheels WheelSetup) (ChartBaseline, error) {
	if err := validateChassisInputs(chart.Frame, chart.CadFork, wheels); err != nil {
		return ChartBaseline{}, err
	}
	if !isFinite(chart.PublishedWheelbaseMm) || chart.PublishedWheelbaseMm <= 0 {
		return ChartBaseline{}, fmt.Errorf("published wheelbase must be finite and positive, got %v mm", chart.PublishedWheelbaseMm)
	}

	unpitched, pitchDeg := measureUnpitchedChassis(chart.Frame, chart.CadFork)
	residuals := ChartResiduals{
		AxleHeightDiffMm: unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle),
		WheelbaseDiffMm:  (unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - chart.PublishedWheelbaseMm,
	}

	return ChartBaseline{
		Placement: placeLeveledChassis(chart.Frame, chart.CadFork, wheels, pitchDeg, nil),
		Residuals: residuals,
	}, nil
}

// PlaceChassis constructs a leveled frame-and-fork chassis using a two-pass
// Pose recipe: Pass 1 measures the axle-leveling pitch at 0° frame pitch, and
// Pass 2 builds the leveled chassis rooted at BB rotated by that pitch and
// locates the wheel-ground contact landmarks directly below each axle.
func PlaceChassis(frame FrameSetup, fork ForkSetup, wheels WheelSetup, trace *pose.Trace) (ChassisPlacement, error) {
	if err := validateChassisInputs(frame, fork, wheels); err != nil {
		return ChassisPlacement{}, err
	}

	_, pitchDeg := measureUnpitchedChassis(frame, fork)
	return placeLeveledChassis(frame, fork, wheels, pitchDeg, trace), nil
}

func measureUnpitchedChassis(frame FrameSetup, fork ForkSetup) (ChassisPlacement, pose.Deg) {
	unpitched := placeChassisAtPitch(pose.NewPose(pose.Point{}, 0, nil), frame, fork)
	return unpitched, -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)
}

func placeLeveledChassis(frame FrameSetup, fork ForkSetup, wheels WheelSetup, pitchDeg pose.Deg, trace *pose.Trace) ChassisPlacement {
	frameRoot := pose.NewPose(pose.Point{}, 0, trace).Turn("frame pitch to level axles", pitchDeg)
	leveled := placeChassisAtPitch(frameRoot, frame, fork)
	leveled.RearTireContact = leveled.RearAxle.Face("down to ground", -90.0).Advance("rear wheel radius", wheels.RadiusMm)
	leveled.FrontTireContact = leveled.FrontAxle.Face("down to ground", -90.0).Advance("front wheel radius", wheels.RadiusMm)
	return leveled
}

func placeChassisAtPitch(frameRoot pose.Pose, frame FrameSetup, fork ForkSetup) ChassisPlacement {
	rearX := -pose.RightTriangleLeg(frame.ChainstayMm, frame.BbDropMm)
	rearAxle := frameRoot.Translate("rear axle", rearX, frame.BbDropMm)

	headTubeTop := frameRoot.Translate("head tube top (reach, stack)", frame.ReachMm, frame.StackMm).
		Turn("steerer axis (180° - HTA)", 180.0-frame.HeadTubeAngleDeg)
	steererDown := headTubeTop.Turn("down steerer", 180.0)
	headTubeBottom := steererDown.Advance("head tube length", frame.HeadTubeLengthMm)
	lowerCupBottom := headTubeBottom.Advance("lower headset cup", fork.LowerCupMm)
	forkSteererBase := lowerCupBottom.Advance("fork A2C", fork.A2cMm)
	frontAxle := forkSteererBase.Turn("fork rake direction", 90.0).Advance("fork rake", fork.RakeMm)

	seatAxis := frameRoot.Turn("seat tube axis (180° - STA)", 180.0-frame.SeatTubeAngleDeg)
	seatTubeTop := seatAxis.Advance("seat tube length", frame.SeatTubeLengthMm)

	return ChassisPlacement{
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

func validateForkSetup(fork ForkSetup) error {
	if !isFinite(fork.A2cMm) || fork.A2cMm <= 0 {
		return invalidInput(InputForkA2c, "fork A2C must be finite and positive, got %v mm", fork.A2cMm)
	}
	if !isFinite(fork.RakeMm) {
		return fmt.Errorf("fork rake must be finite, got %v mm", fork.RakeMm)
	}
	if !isFinite(fork.LowerCupMm) || fork.LowerCupMm < 0 {
		return invalidInput(InputLowerCup, "lower headset cup must be finite and non-negative, got %v mm", fork.LowerCupMm)
	}
	return nil
}

func validateChassisInputs(frame FrameSetup, fork ForkSetup, wheels WheelSetup) error {
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
	if err := validateForkSetup(fork); err != nil {
		return err
	}
	if !isFinite(wheels.RadiusMm) || wheels.RadiusMm <= 0 {
		return fmt.Errorf("wheel radius must be finite and positive, got %v mm", wheels.RadiusMm)
	}
	return nil
}
