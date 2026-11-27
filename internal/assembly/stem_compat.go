package assembly

import "github.com/matta/bikegeo/internal/pose"

// StemSetup specifies the upper headset stack and stem dimensions mounted on
// a steerer axis for the flat *Setup API:
//   - UpperStackMm: axial stack of the upper headset bearing and dust cover.
//   - SpacersMm: axial height of steerer spacers installed below the stem.
//   - ClampHeightMm: axial height of the stem's steerer clamp.
//   - MaxSpacersMm: most spacer stack the steerer allows above the upper cover; 0 means none fit.
//   - MinLengthMm, MaxLengthMm: stem length range fitting may choose; PlaceStem ignores them.
//   - LengthMm: stem extension from steerer axis to bar-clamp center.
//   - AngleDeg: stem angle counter-clockwise from perpendicular to the steerer axis.
type StemSetup struct {
	UpperStackMm  float64
	SpacersMm     float64
	ClampHeightMm float64
	LengthMm      float64
	AngleDeg      pose.Deg

	MaxSpacersMm float64 // most spacer stack the steerer allows above the upper cover; 0 means none fit
	MinLengthMm  float64 // shortest stem length fitting may choose; PlaceStem ignores it
	MaxLengthMm  float64 // longest stem length fitting may choose; PlaceStem ignores it
}

// PlaceStem mounts a stem assembly using steererInWorld (mapping SteererLocal
// at HeadTubeTop with +X pointing up the steerer into the target world
// coordinate system) by delegating to NewSteererPort, NewStem, MountStem, and Place.
func PlaceStem(steererInWorld pose.Transform, setup StemSetup) (StemPlacement, error) {
	// Assert SteererLocal source immediately so passing LowerHeadsetInWorld
	// panics deterministically on CoordinateSystem mismatch.
	headTubeTop := steererInWorld.ApplyPose(pose.OriginPose(SteererLocal))
	if err := validatePose("steererUp", headTubeTop); err != nil {
		return StemPlacement{}, err
	}

	var (
		port          SteererPort
		frameToTarget pose.Transform
		err           error
	)
	if headTubeTop.CoordinateSystem() == FrameLocal {
		port, err = NewSteererPort(steererInWorld, setup.UpperStackMm, setup.MaxSpacersMm)
		frameToTarget = pose.IdentityTransform(FrameLocal)
	} else {
		port, err = NewSteererPort(
			pose.NewTransform(SteererLocal, FrameLocal, pose.Origin(FrameLocal), 0),
			setup.UpperStackMm,
			setup.MaxSpacersMm,
		)
		frameToTarget = headTubeTop.Anchor(FrameLocal)
	}
	if err != nil {
		return StemPlacement{}, err
	}
	stem, err := NewStem(setup.ClampHeightMm, setup.LengthMm, setup.AngleDeg)
	if err != nil {
		return StemPlacement{}, err
	}
	clamped, err := mountStem(port, setup.SpacersMm, stem)
	if err != nil {
		return StemPlacement{}, err
	}
	placed := clamped.Place(frameToTarget)
	if err := validatePlacementPoints("stem", headTubeTop.Origin(), placed.SteererClamp.Origin(), placed.BarClamp.Origin()); err != nil {
		return StemPlacement{}, err
	}
	return placed, nil
}
