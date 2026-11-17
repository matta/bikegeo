package assembly

import (
	"fmt"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// HeadTubeTop represents the validated top of the head tube on a leveled frame.
// Its local basis has +Y pointing upward along the steerer axis and +X pointing
// forward perpendicular to the steerer axis.
type HeadTubeTop struct {
	pose  pose2
	valid bool
}

// NewHeadTubeTop is the root adapter that validates an active head-tube top position
// and head-tube angle (in (0°, 90°]) and constructs a checked HeadTubeTop mount.
func NewHeadTubeTop(pos geom2d.Point2, htaDeg model.Deg) (HeadTubeTop, error) {
	if !isFinite(pos.X) || !isFinite(pos.Y) {
		return HeadTubeTop{}, fmt.Errorf("head-tube top position must be finite, got (%v, %v)", pos.X, pos.Y)
	}
	if !isFinite(float64(htaDeg)) || htaDeg <= 0 || htaDeg > 90 {
		return HeadTubeTop{}, fmt.Errorf("active HTA %.2f° must be in (0°, 90°]", htaDeg)
	}
	// Steerer upward axis is at polar angle (180° - HTA); its forward normal (+X)
	// is 90° clockwise from the steerer, at polar angle (90° - HTA).
	p, err := newPose2(pos, (90.0 - htaDeg).Angle())
	if err != nil {
		return HeadTubeTop{}, err
	}
	return HeadTubeTop{pose: p, valid: true}, nil
}

func (h HeadTubeTop) mustBeValid() {
	if !h.valid {
		panic("assembly: uninitialized or invalid HeadTubeTop")
	}
	h.pose.mustBeValid()
}

// WithUpperStack advances up the steerer axis from the head-tube top by the
// stack sitting below the stem (upperHeadsetCoverMm + spacersMm), producing the
// StemBase mount where the bottom of the stem clamp rests:
//
//   - upperHeadsetCoverMm: axial stack height above the head tube of the upper
//     headset bearing and its bearing cover (dust cap), beneath any spacers.
//   - spacersMm: axial height of the steerer spacers installed below the stem
//     (between the upper headset cover and the bottom of the stem). Spacers
//     or a preload top cap placed above the stem do not affect stem placement.
func (h HeadTubeTop) WithUpperStack(upperHeadsetCoverMm, spacersMm float64) (StemBase, error) {
	h.mustBeValid()
	if !isFinite(upperHeadsetCoverMm) || upperHeadsetCoverMm < 0 {
		return StemBase{}, fmt.Errorf("upper headset cover stack must be finite and non-negative, got %v mm", upperHeadsetCoverMm)
	}
	if !isFinite(spacersMm) || spacersMm < 0 {
		return StemBase{}, fmt.Errorf("spacers must be finite and non-negative, got %v mm", spacersMm)
	}
	advance := upperHeadsetCoverMm + spacersMm
	if !isFinite(advance) {
		return StemBase{}, fmt.Errorf("upper stack advance overflow (%v + %v)", upperHeadsetCoverMm, spacersMm)
	}
	nextPose, err := h.pose.translateLocal(localVec{x: 0, y: advance})
	if err != nil {
		return StemBase{}, err
	}
	return StemBase{mountPose: nextPose, valid: true}, nil
}

// StemBase represents the checked steerer mount above the upper headset cover
// and spacer stack, ready to accept a Stem.
type StemBase struct {
	// A distinct field name prevents conversion to or from HeadTubeTop.
	mountPose pose2
	valid     bool
}

func (b StemBase) mustBeValid() {
	if !b.valid {
		panic("assembly: uninitialized or invalid StemBase")
	}
	b.mountPose.mustBeValid()
}

// Stem represents a validated stem component in its own local coordinate frame
// (origin at steerer clamp bottom, +Y up steerer, +X forward perpendicular to steerer).
type Stem struct {
	steererClampLocal localVec
	barClampLocal     localVec
	valid             bool
}

// NewStem validates stem dimensions and constructs a Stem component definition.
// clampHeightMm and lengthMm must be finite and positive; angleDeg must be finite
// (measured counter-clockwise from perpendicular to the steerer axis).
func NewStem(clampHeightMm, lengthMm float64, angleDeg model.Deg) (Stem, error) {
	if !isFinite(clampHeightMm) || clampHeightMm <= 0 {
		return Stem{}, fmt.Errorf("stem clamp height must be finite and positive, got %v mm", clampHeightMm)
	}
	if !isFinite(lengthMm) || lengthMm <= 0 {
		return Stem{}, fmt.Errorf("stem length must be finite and positive, got %v mm", lengthMm)
	}
	if !isFinite(float64(angleDeg)) {
		return Stem{}, fmt.Errorf("stem angle must be finite, got %v°", angleDeg)
	}

	steererClampPose, err := identityPose2().translateLocal(localVec{x: 0, y: clampHeightMm / 2.0})
	if err != nil {
		return Stem{}, err
	}
	angledPose, err := steererClampPose.rotateLocal(angleDeg.Angle())
	if err != nil {
		return Stem{}, err
	}
	barClampPose, err := angledPose.translateLocal(localVec{x: lengthMm, y: 0})
	if err != nil {
		return Stem{}, err
	}

	return Stem{
		steererClampLocal: localVec{x: steererClampPose.origin.X, y: steererClampPose.origin.Y},
		barClampLocal:     localVec{x: barClampPose.origin.X, y: barClampPose.origin.Y},
		valid:             true,
	}, nil
}

func (s Stem) mustBeValid() {
	if !s.valid {
		panic("assembly: uninitialized or invalid Stem")
	}
	if !isFinite(s.steererClampLocal.x) || !isFinite(s.steererClampLocal.y) ||
		!isFinite(s.barClampLocal.x) || !isFinite(s.barClampLocal.y) {
		panic("assembly: non-finite Stem local coordinates")
	}
}

// AttachStem mounts stem onto b and returns the immutable StemPlacement.
func (b StemBase) AttachStem(stem Stem) (StemPlacement, error) {
	b.mustBeValid()
	stem.mustBeValid()

	steererClamp, err := b.mountPose.transformPoint(stem.steererClampLocal)
	if err != nil {
		return StemPlacement{}, err
	}
	barClamp, err := b.mountPose.transformPoint(stem.barClampLocal)
	if err != nil {
		return StemPlacement{}, err
	}
	return StemPlacement{
		steererClamp: steererClamp,
		barClamp:     barClamp,
		valid:        true,
	}, nil
}

// StemPlacement is the immutable solved placement of the upper headset stack and stem.
type StemPlacement struct {
	steererClamp geom2d.Point2
	barClamp     geom2d.Point2
	valid        bool
}

func (p StemPlacement) mustBeValid() {
	if !p.valid {
		panic("assembly: uninitialized or invalid StemPlacement")
	}
	if !isFinite(p.steererClamp.X) || !isFinite(p.steererClamp.Y) ||
		!isFinite(p.barClamp.X) || !isFinite(p.barClamp.Y) {
		panic("assembly: non-finite StemPlacement landmarks")
	}
}

// SteererClamp returns the world coordinates of the stem's steerer-clamp center.
func (p StemPlacement) SteererClamp() geom2d.Point2 {
	p.mustBeValid()
	return p.steererClamp
}

// BarClamp returns the world coordinates of the stem's handlebar-clamp center.
func (p StemPlacement) BarClamp() geom2d.Point2 {
	p.mustBeValid()
	return p.barClamp
}
