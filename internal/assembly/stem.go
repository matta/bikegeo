package assembly

import (
	"fmt"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// StemSetup specifies the upper headset stack and stem dimensions mounted on a head-tube top:
//   - UpperHeadsetCoverMm: axial stack of the upper headset bearing and dust cover beneath spacers.
//   - SpacersMm: axial height of steerer spacers installed below the stem.
//   - ClampHeightMm: axial height of the stem's steerer clamp.
//   - LengthMm: stem extension from steerer axis to bar-clamp center.
//   - AngleDeg: stem angle counter-clockwise from perpendicular to the steerer axis.
type StemSetup struct {
	UpperHeadsetCoverMm float64
	SpacersMm           float64
	ClampHeightMm       float64
	LengthMm            float64
	AngleDeg            model.Deg
}

// StemPlacement holds the world coordinates of a placed stem's clamp centers.
// Callers must check the error returned by PlaceStem before using these fields.
type StemPlacement struct {
	SteererClamp geom2d.Point2
	BarClamp     geom2d.Point2
}

// PlaceStem places a stem on the leveled head-tube top at activeHtaDeg and returns
// its steerer-clamp and bar-clamp centers.
// Callers must check the returned error before using the placement.
func PlaceStem(headTubeTop geom2d.Point2, activeHtaDeg model.Deg, setup StemSetup) (StemPlacement, error) {
	if !isFinitePoint(headTubeTop) {
		return StemPlacement{}, fmt.Errorf("head-tube top position must be finite, got (%v, %v)", headTubeTop.X, headTubeTop.Y)
	}
	if !isFinite(float64(activeHtaDeg)) || activeHtaDeg <= 0 || activeHtaDeg > 90 {
		return StemPlacement{}, fmt.Errorf("active HTA %.2f° must be in (0°, 90°]", activeHtaDeg)
	}
	if !isFinite(setup.UpperHeadsetCoverMm) || setup.UpperHeadsetCoverMm < 0 {
		return StemPlacement{}, fmt.Errorf("upper headset cover stack must be finite and non-negative, got %v mm", setup.UpperHeadsetCoverMm)
	}
	if !isFinite(setup.SpacersMm) || setup.SpacersMm < 0 {
		return StemPlacement{}, fmt.Errorf("spacers must be finite and non-negative, got %v mm", setup.SpacersMm)
	}
	if !isFinite(setup.ClampHeightMm) || setup.ClampHeightMm <= 0 {
		return StemPlacement{}, fmt.Errorf("stem clamp height must be finite and positive, got %v mm", setup.ClampHeightMm)
	}
	if !isFinite(setup.LengthMm) || setup.LengthMm <= 0 {
		return StemPlacement{}, fmt.Errorf("stem length must be finite and positive, got %v mm", setup.LengthMm)
	}
	if !isFinite(float64(setup.AngleDeg)) {
		return StemPlacement{}, fmt.Errorf("stem angle must be finite, got %v°", setup.AngleDeg)
	}

	steererHeading := (180.0 - activeHtaDeg).Angle()
	upperStackMm := setup.UpperHeadsetCoverMm + setup.SpacersMm
	halfClampMm := setup.ClampHeightMm / 2.0
	stemExtensionTurn := (-90.0 + setup.AngleDeg).Angle()

	// Start at the head-tube top, oriented up the steerer.
	p := newPose2(headTubeTop, steererHeading)
	p = p.advance(upperStackMm)
	p = p.advance(halfClampMm)
	steererClamp := p.origin

	p = p.turn(stemExtensionTurn)
	p = p.advance(setup.LengthMm)
	barClamp := p.origin

	if !isFinitePoint(steererClamp) || !isFinitePoint(barClamp) {
		return StemPlacement{}, fmt.Errorf("stem placement overflow from head-tube top (%v, %v)", headTubeTop.X, headTubeTop.Y)
	}
	return StemPlacement{
		SteererClamp: steererClamp,
		BarClamp:     barClamp,
	}, nil
}
