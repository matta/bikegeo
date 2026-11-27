package assembly

import "github.com/matta/bikegeo/internal/pose"

// FrameSetup is the exported bare-frame geometry snapshot carried inside
// Placement.Setup for JSON output and display readouts.
type FrameSetup struct {
	ReachMm             float64
	StackMm             float64
	HeadTubeAngleDeg    pose.Deg
	HeadTubeLengthMm    float64
	SeatTubeAngleDeg    pose.Deg
	SeatTubeLengthMm    float64
	BbDropMm            float64
	ChainstayMm         float64
	LowerBore           LowerBore
	DefaultLowerCup     LowerCupSetup
	MaxPostInsertionMm  float64
	MinFrameInsertionMm float64
}

// ForkSpec is the exported fork specification snapshot carried inside
// Placement.Setup.Fork.
type ForkSpec struct {
	StaticA2cMm float64
	Travel      ForkTravel
	RakeMm      float64
	Steerer     Steerer
}

// ForkSetup is the exported fork installation snapshot carried inside
// Placement.Setup.
type ForkSetup struct {
	Spec   ForkSpec
	SagPct float64
}

// EffectiveA2cMm returns the fork's effective (sagged) axle-to-crown length.
func (f ForkSetup) EffectiveA2cMm() float64 {
	return f.Spec.StaticA2cMm - f.Spec.Travel.Mm()*(f.SagPct/100.0)
}

// SeatpostSpec is the exported seatpost geometry snapshot carried inside
// Placement.Setup.Seatpost.
type SeatpostSpec struct {
	TotalLengthMm, MaxInsertionMm, MinInsertionMm float64
	SetbackMm, RailClampLengthMm                  float64
}

// SeatpostSetup is the exported seatpost installation snapshot carried inside
// Placement.Setup.
type SeatpostSetup struct {
	Spec        SeatpostSpec
	InsertionMm float64
}

// ExposedMm returns the post length above the seat tube top.
func (s SeatpostSetup) ExposedMm() float64 {
	return s.Spec.TotalLengthMm - s.InsertionMm
}

// SaddleSetup is the exported saddle geometry and fitted-position snapshot
// carried inside Placement.Setup.
type SaddleSetup struct {
	RailCenterToBrpMm      float64
	RailCenterToSitBonesMm float64
	RailToBrpHeightMm      float64
	SitBonesElevationMm    float64
	RailLengthMm           float64
	RailPitchDeg           pose.Deg
	RailOffsetMm           float64
}

// StemSetup is the exported headset-stack and stem snapshot carried inside
// Placement.Setup.
type StemSetup struct {
	UpperStackMm  float64
	SpacersMm     float64
	ClampHeightMm float64
	LengthMm      float64
	AngleDeg      pose.Deg
	MaxSpacersMm  float64
	MinLengthMm   float64
	MaxLengthMm   float64
}

// HandlebarSetup is the exported handlebar geometry and roll snapshot carried
// inside Placement.Setup.
type HandlebarSetup struct {
	StockWidthMm   float64
	CurrentWidthMm float64
	BacksweepDeg   pose.Deg
	UpsweepDeg     pose.Deg
	StatedRiseMm   float64
	RiseMode       RiseMeasurementMode
	BendStartMm    float64
	GripInsetMm    float64
	BarRollDeg     pose.Deg
}

// BikeSetup is the exported complete-build input snapshot carried inside
// Placement.Setup for CLI JSON output, inspect-link reconstruction, and web
// readouts.
type BikeSetup struct {
	Frame     FrameSetup
	LowerCup  LowerCupSetup
	Fork      ForkSetup
	Wheels    WheelSetup
	Seatpost  SeatpostSetup
	Saddle    SaddleSetup
	Stem      StemSetup
	Handlebar HandlebarSetup
}
