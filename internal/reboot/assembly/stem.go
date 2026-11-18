// Package assembly constructs physical bicycle components and assemblies
// from Pose operations.
package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// StemSetup specifies the upper headset stack and stem dimensions mounted on
// a steerer axis:
//   - UpperCoverMm: axial stack of the upper headset bearing and dust cover.
//   - SpacersMm: axial height of steerer spacers installed below the stem.
//   - ClampHeightMm: axial height of the stem's steerer clamp.
//   - LengthMm: stem extension from steerer axis to bar-clamp center.
//   - AngleDeg: stem angle counter-clockwise from perpendicular to the steerer axis.
type StemSetup struct {
	UpperCoverMm  float64
	SpacersMm     float64
	ClampHeightMm float64
	LengthMm      float64
	AngleDeg      pose.Deg
}

// StemPlacement holds the Pose-valued landmarks produced by PlaceStem.
// Each landmark retains its local coordinate frame orientation.
type StemPlacement struct {
	HeadTubeTop  pose.Pose
	CoverTop     pose.Pose
	SpacerTop    pose.Pose
	SteererClamp pose.Pose
	BarClamp     pose.Pose
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func isFinitePoint(p pose.Point) bool {
	return isFinite(p.X) && isFinite(p.Y)
}

// PlaceStem mounts a stem assembly starting at steererUp (positioned at the
// head-tube top and oriented up the steerer axis).
func PlaceStem(steererUp pose.Pose, setup StemSetup) (StemPlacement, error) {
	if !isFinitePoint(steererUp.Origin()) || !isFinite(float64(steererUp.HeadingDeg())) {
		return StemPlacement{}, fmt.Errorf("steererUp pose must be finite, got (%v, %v) at %v°",
			steererUp.Origin().X, steererUp.Origin().Y, steererUp.HeadingDeg())
	}
	if !isFinite(setup.UpperCoverMm) || setup.UpperCoverMm < 0 {
		return StemPlacement{}, fmt.Errorf("upper cover must be finite and non-negative, got %v mm", setup.UpperCoverMm)
	}
	if !isFinite(setup.SpacersMm) || setup.SpacersMm < 0 {
		return StemPlacement{}, fmt.Errorf("spacers must be finite and non-negative, got %v mm", setup.SpacersMm)
	}
	if !isFinite(setup.ClampHeightMm) || setup.ClampHeightMm <= 0 {
		return StemPlacement{}, fmt.Errorf("clamp height must be finite and positive, got %v mm", setup.ClampHeightMm)
	}
	if !isFinite(setup.LengthMm) || setup.LengthMm <= 0 {
		return StemPlacement{}, fmt.Errorf("stem length must be finite and positive, got %v mm", setup.LengthMm)
	}
	if !isFinite(float64(setup.AngleDeg)) {
		return StemPlacement{}, fmt.Errorf("stem angle must be finite, got %v°", setup.AngleDeg)
	}

	coverTop := steererUp.Advance("upper headset cover", setup.UpperCoverMm)
	spacerTop := coverTop.Advance("spacers", setup.SpacersMm)
	steererClamp := spacerTop.Advance("half stem clamp height", setup.ClampHeightMm/2.0)
	stemAxis := steererClamp.Turn("perpendicular", -90.0).Turn("stem axis", setup.AngleDeg)
	barClamp := stemAxis.Advance("stem length", setup.LengthMm)

	if !isFinitePoint(steererClamp.Origin()) || !isFinitePoint(barClamp.Origin()) {
		return StemPlacement{}, fmt.Errorf("stem placement overflow from (%v, %v)",
			steererUp.Origin().X, steererUp.Origin().Y)
	}

	return StemPlacement{
		HeadTubeTop:  steererUp,
		CoverTop:     coverTop,
		SpacerTop:    spacerTop,
		SteererClamp: steererClamp,
		BarClamp:     barClamp,
	}, nil
}
