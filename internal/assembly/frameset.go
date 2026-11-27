package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

// WheelSetup specifies the wheel radius used to locate tire-ground contact
// points below the leveled axles.
type WheelSetup struct {
	RadiusMm float64
}

func validateWheels(wheels WheelSetup) error {
	if !isFinite(wheels.RadiusMm) || wheels.RadiusMm <= 0 {
		return fmt.Errorf("wheel radius must be finite and positive, got %v mm", wheels.RadiusMm)
	}
	return nil
}

// forkMounter is the thin consumer-side interface MateFrameset requires from a
// concrete fork implementation (RigidFork, SuspensionFork, or BasisFork).
type forkMounter interface {
	CheckHeadset(bore LowerBore, cup LowerCupSetup) error
	AxleInFork(state LoadState) pose.Point
}

// Frameset is an immutable mated frame, lower headset cup, fork, and wheelset.
// Fork axle geometry is snapshotted by value at mating time so mutating a
// caller-owned fork pointer cannot alter the frameset. Place(Static) and
// Place(Dynamic) level the frameset into StaticWorld or DynamicWorld in a
// single pass from FrameLocal.
type Frameset struct {
	frame              Frame
	lowerCup           LowerCupSetup
	wheels             WheelSetup
	steererPort        SteererPort
	forkInFrame        pose.Transform
	staticAxleInFork   pose.Point
	dynamicAxleInFork  pose.Point
	staticAxleInFrame  pose.Point
	dynamicAxleInFrame pose.Point
	forkSpec           ForkSpec
	forkSagPct         float64
	valid              bool
}

func forkMetadata(fork forkMounter, staticAxle pose.Point) (ForkSpec, float64) {
	switch f := fork.(type) {
	case RigidFork:
		return ForkSpec{StaticA2cMm: f.staticA2cMm, Travel: TravelRigid, RakeMm: f.rakeMm, Steerer: f.steerer.Steerer()}, 0
	case *RigidFork:
		return ForkSpec{StaticA2cMm: f.staticA2cMm, Travel: TravelRigid, RakeMm: f.rakeMm, Steerer: f.steerer.Steerer()}, 0
	case SuspensionFork:
		return ForkSpec{StaticA2cMm: f.staticA2cMm, Travel: f.travel, RakeMm: f.rakeMm, Steerer: f.steerer.Steerer()}, f.sagPct
	case *SuspensionFork:
		return ForkSpec{StaticA2cMm: f.staticA2cMm, Travel: f.travel, RakeMm: f.rakeMm, Steerer: f.steerer.Steerer()}, f.sagPct
	case BasisFork:
		return ForkSpec{StaticA2cMm: f.basisA2cMm, Travel: TravelRigid, RakeMm: f.rakeMm, Steerer: SteererBasis}, 0
	case *BasisFork:
		return ForkSpec{StaticA2cMm: f.basisA2cMm, Travel: TravelRigid, RakeMm: f.rakeMm, Steerer: SteererBasis}, 0
	default:
		return ForkSpec{StaticA2cMm: staticAxle.X(), Travel: TravelRigid, RakeMm: staticAxle.Y(), Steerer: SteererBasis}, 0
	}
}

func validateForkAxle(label string, p pose.Point) error {
	if !p.CoordinateSystem().Valid() {
		panic(fmt.Sprintf("assembly: %s fork axle has uninitialized CoordinateSystem", label))
	}
	if p.CoordinateSystem() != ForkLocal {
		panic(fmt.Sprintf("assembly: %s fork axle coordinate system mismatch (want %s, got %s)",
			label, ForkLocal, p.CoordinateSystem()))
	}
	if !isFinitePoint(p) || p.X() <= 0 {
		return invalidInput(InputForkA2c, "fork %s effective A2C must be finite and positive, got (%v, %v) mm", label, p.X(), p.Y())
	}
	return nil
}

// MateFrameset validates SHIS compatibility between frame, lowerCup, and fork,
// builds the frame's upper SteererPort (with upperStackMm and
// DefaultMaxSpacersMm), snapshots the fork's Static and Dynamic axle positions
// in ForkLocal by value, and precomputes the fork-to-frame transform so
// Unpitched and Place(Static|Dynamic) level in one pass.
func MateFrameset(frame Frame, lowerCup LowerCupSetup, upperStackMm float64, fork forkMounter, wheels WheelSetup) (*Frameset, error) {
	fs, err := mateFrameset(frame, lowerCup, upperStackMm, fork, wheels)
	if err != nil {
		return nil, err
	}
	return &fs, nil
}

func mateFrameset(frame Frame, lowerCup LowerCupSetup, upperStackMm float64, fork forkMounter, wheels WheelSetup) (Frameset, error) {
	frame.requireValid()
	if fork == nil {
		return Frameset{}, fmt.Errorf("fork must not be nil")
	}
	if err := fork.CheckHeadset(frame.LowerBore(), lowerCup); err != nil {
		return Frameset{}, err
	}
	if err := validateWheels(wheels); err != nil {
		return Frameset{}, err
	}
	steererPort, err := NewSteererPort(frame.HeadTube().SteererInFrame(), upperStackMm, DefaultMaxSpacersMm)
	if err != nil {
		return Frameset{}, err
	}

	staticAxle := fork.AxleInFork(Static)
	dynamicAxle := fork.AxleInFork(Dynamic)
	if err := validateForkAxle("static", staticAxle); err != nil {
		return Frameset{}, err
	}
	if err := validateForkAxle("dynamic", dynamicAxle); err != nil {
		return Frameset{}, err
	}

	// LowerHeadsetLocal sits at HeadTubeBottom with +X down the steerer.
	// Advancing lowerCup.StackMm reaches LowerCupBottom (the crown race seat),
	// where ForkLocal's (0, 0) attaches with +X down the steerer and +Y in the rake direction.
	lowerCupBottomLocal := pose.OriginPose(LowerHeadsetLocal).Advance(lowerCup.StackMm)
	forkInFrame := pose.Compose(lowerCupBottomLocal.Anchor(ForkLocal), frame.HeadTube().LowerHeadsetInFrame())

	staticAxleInFrame := forkInFrame.ApplyPoint(staticAxle)
	dynamicAxleInFrame := forkInFrame.ApplyPoint(dynamicAxle)
	if err := validatePlacementPoints("frameset", frame.RearAxle(), forkInFrame.OriginInDst(), staticAxleInFrame, dynamicAxleInFrame); err != nil {
		return Frameset{}, err
	}

	forkSpec, forkSagPct := forkMetadata(fork, staticAxle)

	return Frameset{
		frame:              frame,
		lowerCup:           lowerCup,
		wheels:             wheels,
		steererPort:        steererPort,
		forkInFrame:        forkInFrame,
		staticAxleInFork:   staticAxle,
		dynamicAxleInFork:  dynamicAxle,
		staticAxleInFrame:  staticAxleInFrame,
		dynamicAxleInFrame: dynamicAxleInFrame,
		forkSpec:           forkSpec,
		forkSagPct:         forkSagPct,
		valid:              true,
	}, nil
}

func (fs *Frameset) requireValid() {
	if fs == nil || !fs.valid {
		panic("assembly: uninitialized Frameset")
	}
}

// Frame returns the mated bare Frame.
func (fs *Frameset) Frame() Frame {
	fs.requireValid()
	return fs.frame
}

// LowerCup returns the mated lower headset cup setup.
func (fs *Frameset) LowerCup() LowerCupSetup {
	fs.requireValid()
	return fs.lowerCup
}

// Wheels returns the mated wheel setup.
func (fs *Frameset) Wheels() WheelSetup {
	fs.requireValid()
	return fs.wheels
}

// SeatTube returns the frame's SeatTubeSocket in FrameLocal.
func (fs *Frameset) SeatTube() SeatTubeSocket {
	fs.requireValid()
	return fs.frame.SeatTube()
}

// Steerer returns the frameset's sealed upper SteererPort in FrameLocal.
func (fs *Frameset) Steerer() SteererPort {
	fs.requireValid()
	return fs.steererPort
}

// WithMaxSpacers returns a copy of fs whose upper SteererPort caps steerer
// spacers at maxSpacersMm (0 <= maxSpacersMm <= DefaultMaxSpacersMm).
func (fs *Frameset) WithMaxSpacers(maxSpacersMm float64) (*Frameset, error) {
	fs.requireValid()
	port, err := NewSteererPort(fs.steererPort.SteererInFrame(), fs.steererPort.UpperStackMm(), maxSpacersMm)
	if err != nil {
		return nil, err
	}
	copyFS := *fs
	copyFS.steererPort = port
	return &copyFS, nil
}

// ForkTravel returns the mated fork's suspension travel class (TravelRigid for
// rigid and basis forks).
func (fs *Frameset) ForkTravel() ForkTravel {
	fs.requireValid()
	return fs.forkSpec.Travel
}

// ForkSagPct returns the mated fork's seated sag percentage (0 for rigid and
// basis forks).
func (fs *Frameset) ForkSagPct() float64 {
	fs.requireValid()
	return fs.forkSagPct
}

func (fs *Frameset) forkSetupSnapshot() ForkSetup {
	return ForkSetup{Spec: fs.forkSpec, SagPct: fs.forkSagPct}
}

func (fs *Frameset) frameSetupSnapshot() FrameSetup {
	d := fs.Frame().Dimensions()
	return FrameSetup{
		ReachMm:             d.ReachMm,
		StackMm:             d.StackMm,
		HeadTubeAngleDeg:    d.HeadTubeAngleDeg,
		HeadTubeLengthMm:    d.HeadTubeLengthMm,
		SeatTubeAngleDeg:    d.SeatTubeAngleDeg,
		SeatTubeLengthMm:    d.SeatTubeLengthMm,
		BbDropMm:            d.BbDropMm,
		ChainstayMm:         d.ChainstayMm,
		LowerBore:           d.LowerBore,
		DefaultLowerCup:     d.DefaultLowerCup,
		MaxPostInsertionMm:  d.MaxPostInsertionMm,
		MinFrameInsertionMm: d.MinFrameInsertionMm,
	}
}

func (fs *Frameset) axleForState(state LoadState) (inFork, inFrame pose.Point) {
	switch state {
	case Static:
		return fs.staticAxleInFork, fs.staticAxleInFrame
	case Dynamic:
		return fs.dynamicAxleInFork, fs.dynamicAxleInFrame
	default:
		panic(fmt.Sprintf("assembly: invalid LoadState %q", state))
	}
}

// FrameToWorld returns the single-pass rigid transform mapping FrameLocal into
// state's leveled world coordinate system (StaticWorld or DynamicWorld) with
// RearAxle.Y == FrontAxle.Y.
func (fs *Frameset) FrameToWorld(state LoadState) pose.Transform {
	fs.requireValid()
	_, frontInFrame := fs.axleForState(state)
	// Add +0° to normalize -0° when the axle line is already level.
	pitch := 0 - fs.frame.RearAxle().AngleTo(frontInFrame).Deg()
	world := state.WorldSystem()
	return pose.NewTransform(FrameLocal, world, pose.Origin(world), pitch)
}

// Unpitched places the frameset in UnpitchedChart at 0° frame pitch using the
// fork's dynamic axle position (identical to static for rigid and basis forks).
func (fs *Frameset) Unpitched() FramesetPlacement {
	fs.requireValid()
	frameToChart := pose.NewTransform(FrameLocal, UnpitchedChart, pose.Origin(UnpitchedChart), 0)
	return fs.placeWithTransform(frameToChart, fs.dynamicAxleInFork)
}

// Place levels the frameset into state's world coordinate system (StaticWorld
// or DynamicWorld) in a single pass from FrameLocal.
func (fs *Frameset) Place(state LoadState) FramesetPlacement {
	fs.requireValid()
	axleInFork, _ := fs.axleForState(state)
	return fs.placeWithTransform(fs.FrameToWorld(state), axleInFork)
}

func (fs *Frameset) placeWithTransform(frameToWorld pose.Transform, axleInFork pose.Point) FramesetPlacement {
	world := frameToWorld.Dst()
	bb := frameToWorld.PoseInDst()
	rearAxle := frameToWorld.ApplyPose(pose.NewPose(fs.frame.RearAxle(), 0))

	seatAxis := frameToWorld.ApplyPose(fs.frame.SeatAxis())
	seatTubeTop := pose.Compose(fs.frame.SeatTube().CollarInFrame, frameToWorld).PoseInDst()

	headTubeTop := pose.Compose(fs.frame.HeadTube().SteererInFrame(), frameToWorld).PoseInDst()
	headTubeBottom := pose.Compose(fs.frame.HeadTube().LowerHeadsetInFrame(), frameToWorld).PoseInDst()

	forkInWorld := pose.Compose(fs.forkInFrame, frameToWorld)
	lowerCupBottom := forkInWorld.PoseInDst()
	forkSteererBase := forkInWorld.ApplyPose(pose.NewPose(pose.NewPoint(ForkLocal, axleInFork.X(), 0), 0))
	// Turning 90° from steerer-down (+X in ForkLocal) points along +Y: the fork rake direction.
	frontAxle := forkInWorld.ApplyPose(pose.NewPose(axleInFork, 90.0))

	// Face straight down (-90°), then advance one wheel radius to the ground.
	plumbDown := pose.NewHeading(world, -90.0)
	rearTireContact := rearAxle.Face(plumbDown).Advance(fs.wheels.RadiusMm)
	frontTireContact := frontAxle.Face(plumbDown).Advance(fs.wheels.RadiusMm)

	return FramesetPlacement{
		BB:               bb,
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
		FrameToWorld:     frameToWorld,
	}
}

// FramesetPlacement holds frame, fork, lower headset cup, and wheel-ground
// contact landmark poses in an ambient world coordinate system (UnpitchedChart,
// StaticWorld, or DynamicWorld), anchored with BB at (0, 0) (+X forward, +Y up).
//
// Each landmark carries the Pose along its construction path:
//   - BB and RearAxle carry the leveled frame heading (world PitchDeg).
//   - SeatAxis (anchored at BB) and SeatTubeTop point up and rearward along
//     the seat axis (180° − SeatTubeAngleDeg from the frame).
//   - HeadTubeTop points up and rearward along the steerer (180° −
//     HeadTubeAngleDeg); HeadTubeBottom, LowerCupBottom, and ForkSteererBase
//     point down the steerer (−HeadTubeAngleDeg).
//   - FrontAxle points forward and up perpendicular to the steerer (90° −
//     HeadTubeAngleDeg).
//   - RearTireContact and FrontTireContact point straight down (−90°).
//   - FrameToWorld maps FrameLocal into the placement's world coordinate system
//     (omitted from JSON snapshots).
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
	FrameToWorld     pose.Transform `json:"-"`
}

// SteererInWorld returns the rigid transform mapping SteererLocal
// ((0, 0) at HeadTubeTop, +X up the steerer) into HeadTubeTop's world coordinate system.
func (f FramesetPlacement) SteererInWorld() pose.Transform {
	return f.HeadTubeTop.Anchor(SteererLocal)
}

// LowerHeadsetInWorld returns the rigid transform mapping LowerHeadsetLocal
// ((0, 0) at HeadTubeBottom, +X down the steerer) into HeadTubeBottom's world coordinate system.
func (f FramesetPlacement) LowerHeadsetInWorld() pose.Transform {
	return f.HeadTubeBottom.Anchor(LowerHeadsetLocal)
}

// PitchDeg returns the counter-clockwise frame leveling pitch applied at BB.
func (f FramesetPlacement) PitchDeg() pose.Deg {
	return f.BB.HeadingDeg()
}

// LevelingPitchDeg returns the counter-clockwise rotation about BB that
// would put the axles level: nonzero on an unpitched placement whose fork
// and cup differ from the chart's basis, and 0 (to rounding) on a leveled
// placement.
func (f FramesetPlacement) LevelingPitchDeg() pose.Deg {
	return -f.RearAxle.Origin().AngleTo(f.FrontAxle.Origin()).Deg()
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
	return f.RearAxle.Origin().DistanceTo(f.FrontAxle.Origin())
}

// LeveledBbDropMm returns the vertical drop from the leveled axle line to BB.
func (f FramesetPlacement) LeveledBbDropMm() float64 {
	return f.BB.Origin().VerticalOffsetTo(f.RearAxle.Origin())
}

// TrailMm returns the ground trail: the horizontal distance on the ground from
// the front tire contact back to where the leveled steering axis meets the
// ground. Positive when the contact sits behind that point.
func (f FramesetPlacement) TrailMm() float64 {
	// Split HeadTubeTop → FrontTireContact into a run down the steering axis
	// and a horizontal remainder. The steering axis meets the ground (the
	// contact's height) at HeadTubeTop plus the run, so the remainder runs
	// from that point to the contact, and trail is its negation.
	steererDown := f.HeadTubeTop.Turn(180.0).Heading()
	worldHoriz := pose.NewHeading(f.HeadTubeTop.CoordinateSystem(), 0)
	_, ahead, ok := pose.DecomposeAlong(f.HeadTubeTop.Origin().OffsetTo(f.FrontTireContact.Origin()), steererDown, worldHoriz)
	if !ok {
		return math.NaN() // a horizontal steering axis never meets the ground
	}
	return -ahead
}

// RearCenterMm returns the horizontal distance from the rear axle to BB on the
// leveled bike (the RC in RC / WB).
func (f FramesetPlacement) RearCenterMm() float64 {
	return f.RearAxle.Origin().OffsetTo(f.BB.Origin()).X()
}

// RcWbRatioPct returns RearCenterMm as a percentage of WheelbaseMm.
func (f FramesetPlacement) RcWbRatioPct() float64 {
	return 100.0 * f.RearCenterMm() / f.WheelbaseMm()
}

// LeveledReachMm returns the horizontal distance from BB to HeadTubeTop on the
// leveled bike. It differs from the chart's reach by the leveling pitch.
func (f FramesetPlacement) LeveledReachMm() float64 {
	return f.BB.Origin().OffsetTo(f.HeadTubeTop.Origin()).X()
}

// LeveledStackMm returns the vertical distance from BB to HeadTubeTop on the
// leveled bike.
func (f FramesetPlacement) LeveledStackMm() float64 {
	return f.BB.Origin().VerticalOffsetTo(f.HeadTubeTop.Origin())
}

// EttMm returns the effective top tube: the horizontal distance, at
// HeadTubeTop's height, from the leveled seat axis through BB to HeadTubeTop.
func (f FramesetPlacement) EttMm() float64 {
	// Split BB → HeadTubeTop into a run up the seat axis and a horizontal
	// remainder; the remainder is the ETT.
	worldHoriz := pose.NewHeading(f.SeatAxis.CoordinateSystem(), 0)
	_, ett, ok := pose.DecomposeAlong(f.SeatAxis.Origin().OffsetTo(f.HeadTubeTop.Origin()), f.SeatAxis.Heading(), worldHoriz)
	if !ok {
		return math.NaN() // a horizontal seat axis never reaches HeadTubeTop's height
	}
	return ett
}
