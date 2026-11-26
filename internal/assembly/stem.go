// Package assembly constructs physical bicycle components and assemblies
// from Pose operations.
package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// Stem constants (Q82). DefaultUpperStackMm is illustrative (legacy
// DefaultHeadsetTopStackMm). DefaultMaxSpacersMm is the conservative spacer
// ceiling every physical build must respect.
const (
	DefaultUpperStackMm = 11.0
	DefaultMaxSpacersMm = 30.0
)

// StemSetup specifies the upper headset stack and stem dimensions mounted on
// a steerer axis:
//   - UpperStackMm: axial stack of the upper headset bearing and dust cover.
//   - SpacersMm: axial height of steerer spacers installed below the stem.
//   - ClampHeightMm: axial height of the stem's steerer clamp.
//   - MaxSpacersMm: most spacer stack the steerer allows above the upper cover; 0 means none fit.
//   - MinLengthMm, MaxLengthMm: stem length range fitting may choose; PlaceStem ignores them.
//   - LengthMm: stem extension from steerer axis to bar-clamp center.
//   - AngleDeg: stem angle counter-clockwise from perpendicular to the steerer axis.
type StemSetup struct {
	// TODO: Move UpperStackMm into a headset setup alongside LowerCupSetup.
	UpperStackMm  float64
	SpacersMm     float64
	ClampHeightMm float64
	LengthMm      float64
	AngleDeg      pose.Deg

	MaxSpacersMm float64 // most spacer stack the steerer allows above the upper cover; 0 means none fit
	MinLengthMm  float64 // shortest stem length fitting may choose; PlaceStem ignores it
	MaxLengthMm  float64 // longest stem length fitting may choose; PlaceStem ignores it
}

// steererPerpDir is the 0° stem datum direction: forward, perpendicular to the
// steerer (-90° from SteererUpDir).
type steererPerpDir struct{}

const steererUpToPerp pose.TurnDeg[SteererUpDir, steererPerpDir] = -90.0

func (s StemSetup) angleTurn() pose.TurnDeg[steererPerpDir, StemDir] {
	return pose.TurnDeg[steererPerpDir, StemDir](s.AngleDeg)
}

// StemPlacement holds the Pose-valued landmarks produced by PlaceStem.
// Each landmark carries its local orientation anchored at its Origin().
type StemPlacement struct {
	HeadTubeTop  pose.Pose[SteererUpDir, DynamicWorld]
	CoverTop     pose.Pose[SteererUpDir, DynamicWorld]
	SpacerTop    pose.Pose[SteererUpDir, DynamicWorld]
	SteererClamp pose.Pose[SteererUpDir, DynamicWorld]
	BarClamp     pose.Pose[StemDir, DynamicWorld]
}

// PlaceStem mounts a stem assembly starting at steererUp (positioned at the
// head-tube top and oriented up the steerer direction).
func PlaceStem(steererUp pose.Pose[SteererUpDir, DynamicWorld], setup StemSetup) (StemPlacement, error) {
	if err := validatePose("steererUp", steererUp); err != nil {
		return StemPlacement{}, err
	}
	if !isFinite(setup.UpperStackMm) || setup.UpperStackMm < 0 {
		return StemPlacement{}, fmt.Errorf("upper cover must be finite and non-negative, got %v mm", setup.UpperStackMm)
	}
	if !isFinite(setup.SpacersMm) || setup.SpacersMm < 0 {
		return StemPlacement{}, invalidInput(InputStemSpacers, "spacers must be finite and non-negative, got %v mm", setup.SpacersMm)
	}
	if !isFinite(setup.MaxSpacersMm) || setup.MaxSpacersMm < 0 {
		return StemPlacement{}, fmt.Errorf("max spacers must be finite and non-negative, got %v mm", setup.MaxSpacersMm)
	}
	if setup.MaxSpacersMm > DefaultMaxSpacersMm {
		return StemPlacement{}, fmt.Errorf("max spacers must be at most %v mm, got %v mm", DefaultMaxSpacersMm, setup.MaxSpacersMm)
	}
	if setup.SpacersMm > setup.MaxSpacersMm {
		return StemPlacement{}, invalidInput(InputStemSpacers, "spacers must be at most %v mm (steerer allowance), got %v mm", setup.MaxSpacersMm, setup.SpacersMm)
	}
	if !isFinite(setup.ClampHeightMm) || setup.ClampHeightMm <= 0 {
		return StemPlacement{}, fmt.Errorf("clamp height must be finite and positive, got %v mm", setup.ClampHeightMm)
	}
	if !isFinite(setup.LengthMm) || setup.LengthMm <= 0 {
		return StemPlacement{}, invalidInput(InputStemLength, "stem length must be finite and positive, got %v mm", setup.LengthMm)
	}
	if !isFinite(float64(setup.AngleDeg)) {
		return StemPlacement{}, invalidInput(InputStemAngle, "stem angle must be finite, got %v°", setup.AngleDeg)
	}

	coverTop := steererUp.Advance(setup.UpperStackMm)
	spacerTop := coverTop.Advance(setup.SpacersMm)
	steererClamp := spacerTop.Advance(setup.ClampHeightMm / 2.0)
	// Turn -90° to the 0° stem datum perpendicular to the steerer, then rotate by the stem angle.
	steererPerp := pose.Turn(steererClamp, steererUpToPerp)
	barClamp := pose.Turn(steererPerp, setup.angleTurn()).Advance(setup.LengthMm)

	if err := validatePlacementPoints("stem", steererUp.Origin(), steererClamp.Origin(), barClamp.Origin()); err != nil {
		return StemPlacement{}, err
	}

	return StemPlacement{
		HeadTubeTop:  steererUp,
		CoverTop:     coverTop,
		SpacerTop:    spacerTop,
		SteererClamp: steererClamp,
		BarClamp:     barClamp,
	}, nil
}
