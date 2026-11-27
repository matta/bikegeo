package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// FrameSetup specifies the bare frame dimensions in its unpitched reference
// attitude for the flat *Setup API.
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

func (f FrameSetup) dimensions() FrameDimensions {
	return FrameDimensions{
		ReachMm:             f.ReachMm,
		StackMm:             f.StackMm,
		HeadTubeAngleDeg:    f.HeadTubeAngleDeg,
		HeadTubeLengthMm:    f.HeadTubeLengthMm,
		SeatTubeAngleDeg:    f.SeatTubeAngleDeg,
		SeatTubeLengthMm:    f.SeatTubeLengthMm,
		BbDropMm:            f.BbDropMm,
		ChainstayMm:         f.ChainstayMm,
		LowerBore:           f.LowerBore,
		DefaultLowerCup:     f.DefaultLowerCup,
		MaxPostInsertionMm:  f.MaxPostInsertionMm,
		MinFrameInsertionMm: f.MinFrameInsertionMm,
	}
}

// ForkSpec describes a catalog fork before installation and sag for the flat
// *Setup API.
type ForkSpec struct {
	StaticA2cMm float64
	Travel      ForkTravel
	RakeMm      float64
	Steerer     Steerer
}

// ForkGeometry holds the resolved fixed fork dimensions (effective axle-to-crown
// and rake) for the flat *Setup API.
type ForkGeometry struct {
	A2cMm  float64
	RakeMm float64
}

// ForkSetup holds the physical installation inputs for a fork (its catalog
// ForkSpec and installed SagPct) for the flat *Setup API.
type ForkSetup struct {
	Spec   ForkSpec
	SagPct float64
}

// EffectiveA2cMm returns the fork's effective (sagged) axle-to-crown length.
func (f ForkSetup) EffectiveA2cMm() float64 {
	return f.Spec.StaticA2cMm - f.Spec.Travel.Mm()*(f.SagPct/100.0)
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
	if !f.Spec.Travel.Valid() {
		return fmt.Errorf("unsupported fork travel %d mm", f.Spec.Travel)
	}
	if f.Spec.Steerer == SteererBasis && f.Spec.Travel != TravelRigid {
		return fmt.Errorf("basis fork cannot have travel, got %d mm", f.Spec.Travel)
	}
	if !isFinite(f.SagPct) || f.SagPct < 0 || f.SagPct > 100 {
		return invalidInput(InputForkSag, "fork sag must be in [0%%, 100%%], got %v%%", f.SagPct)
	}
	if f.Spec.Travel == TravelRigid && f.SagPct != 0 {
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

func (f ForkSetup) toForkMounter() (forkMounter, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	switch f.Spec.Steerer {
	case SteererBasis:
		return NewBasisFork(f.Spec.StaticA2cMm, f.Spec.RakeMm)
	case SteererStraight118:
		if f.Spec.Travel.IsSuspension() {
			return NewSuspensionFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, f.Spec.Travel, PhysicalSteererStraight118, f.SagPct)
		}
		return NewRigidFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, PhysicalSteererStraight118)
	case SteererTapered:
		if f.Spec.Travel.IsSuspension() {
			return NewSuspensionFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, f.Spec.Travel, PhysicalSteererTapered, f.SagPct)
		}
		return NewRigidFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, PhysicalSteererTapered)
	default:
		return nil, invalidInput(InputLowerCup, "unknown or missing fork steerer %q", f.Spec.Steerer)
	}
}

// PlaceUnpitchedFrameset builds the frameset in the chart's unpitched attitude
// by delegating to NewFrame, fork constructors, MateFrameset, and Unpitched.
func PlaceUnpitchedFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error) {
	f, err := NewFrame(frame.dimensions())
	if err != nil {
		return FramesetPlacement{}, err
	}
	mounter, err := fork.toForkMounter()
	if err != nil {
		return FramesetPlacement{}, err
	}
	fs, err := mateFrameset(f, lowerCup, DefaultUpperStackMm, mounter, wheels)
	if err != nil {
		return FramesetPlacement{}, err
	}
	return fs.Unpitched(), nil
}

// PlaceFrameset builds a leveled frameset in DynamicWorld in a single pass by
// delegating to NewFrame, fork constructors, MateFrameset, and Place(Dynamic).
func PlaceFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error) {
	f, err := NewFrame(frame.dimensions())
	if err != nil {
		return FramesetPlacement{}, err
	}
	mounter, err := fork.toForkMounter()
	if err != nil {
		return FramesetPlacement{}, err
	}
	fs, err := mateFrameset(f, lowerCup, DefaultUpperStackMm, mounter, wheels)
	if err != nil {
		return FramesetPlacement{}, err
	}
	return fs.Place(Dynamic), nil
}
