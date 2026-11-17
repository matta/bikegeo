package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// Handlebar represents a validated swept/riser handlebar component in its own
// neutral (roll = 0°) local coordinate frame:
//   - Origin (0, 0, 0): stem bar-clamp center
//   - +X: forward horizontal
//   - +Y: upward vertical
//   - +Z: lateral right from centerline (symmetric for left)
//
// Rise, sweep, width, and grip-inset conventions:
//   - Effective width is CurrentWidthMm if positive, otherwise StockWidthMm.
//   - Lateral hand center is z_hand = EffectiveWidth/2 - gripInsetMm.
//   - Swept span outboard of the bend start is lz = max(0, z_hand - EffectiveBendStartMm).
//   - Base rise at the bend intersection (y_k) is StatedRiseMm for RiseModeStandardKnuckle
//     (or unset), and StatedRiseMm - (StockWidthMm/2 - EffectiveBendStartMm)*tan(UpsweepDeg)
//     for RiseModeBarEndCenter.
//   - Neutral grip coordinates are (-lz*tan(BacksweepDeg), y_k + lz*tan(UpsweepDeg), z_hand).
type Handlebar struct {
	gripLocal localVec
	zHandMm   float64
	valid     bool
}

// NewHandlebar validates handlebar geometry and grip inset and constructs a
// Handlebar component definition in its neutral (roll = 0°) local frame.
func NewHandlebar(spec model.HandlebarSpec, gripInsetMm float64) (Handlebar, error) {
	width := spec.EffectiveWidthMm()
	if !isFinite(width) || width <= 0 {
		return Handlebar{}, fmt.Errorf("handlebar effective width must be finite and positive, got %v mm", width)
	}
	switch spec.RiseMode {
	case "", model.RiseModeStandardKnuckle, model.RiseModeBarEndCenter:
	default:
		return Handlebar{}, fmt.Errorf("unsupported handlebar rise mode %q", spec.RiseMode)
	}
	if spec.RiseMode == model.RiseModeBarEndCenter && (!isFinite(spec.StockWidthMm) || spec.StockWidthMm <= 0) {
		return Handlebar{}, fmt.Errorf("handlebar stock width must be finite and positive for bar_end_center rise mode, got %v mm", spec.StockWidthMm)
	}
	if !isFinite(spec.EffectiveBendStartMm) || spec.EffectiveBendStartMm < 0 {
		return Handlebar{}, fmt.Errorf("handlebar bend start must be finite and non-negative, got %v mm", spec.EffectiveBendStartMm)
	}
	if !isFinite(spec.StatedRiseMm) {
		return Handlebar{}, fmt.Errorf("handlebar stated rise must be finite, got %v mm", spec.StatedRiseMm)
	}
	if !isFinite(float64(spec.BacksweepDeg)) || spec.BacksweepDeg <= -90 || spec.BacksweepDeg >= 90 {
		return Handlebar{}, fmt.Errorf("handlebar backsweep %v° must be in (-90°, 90°)", spec.BacksweepDeg)
	}
	if !isFinite(float64(spec.UpsweepDeg)) || spec.UpsweepDeg <= -90 || spec.UpsweepDeg >= 90 {
		return Handlebar{}, fmt.Errorf("handlebar upsweep %v° must be in (-90°, 90°)", spec.UpsweepDeg)
	}
	if !isFinite(gripInsetMm) || gripInsetMm <= 0 || gripInsetMm >= width/2.0 {
		return Handlebar{}, fmt.Errorf("grip inset %v mm must be in (0, %.1f)", gripInsetMm, width/2.0)
	}

	zHand := width/2.0 - gripInsetMm
	lz := math.Max(0, zHand-spec.EffectiveBendStartMm)
	yk := spec.BaseRiseAtBend()
	dx0 := -lz * spec.BacksweepDeg.Angle().Tan()
	dy0 := yk + lz*spec.UpsweepDeg.Angle().Tan()

	if !isFinite(zHand) || !isFinite(yk) || !isFinite(dx0) || !isFinite(dy0) {
		return Handlebar{}, fmt.Errorf("handlebar local grip calculation overflow")
	}

	return Handlebar{
		gripLocal: localVec{x: dx0, y: dy0},
		zHandMm:   zHand,
		valid:     true,
	}, nil
}

func (h Handlebar) mustBeValid() {
	if !h.valid {
		panic("assembly: uninitialized or invalid Handlebar")
	}
	if !isFinite(h.gripLocal.x) || !isFinite(h.gripLocal.y) || !isFinite(h.zHandMm) {
		panic("assembly: non-finite Handlebar local coordinates")
	}
}

// GripOffset3D returns the 3D offset (ΔX, ΔY, ΔZ) from the bar clamp center to
// the hand grip center when rolled by rollDeg in the clamp.
func (h Handlebar) GripOffset3D(rollDeg model.Deg) (model.Point3D, error) {
	h.mustBeValid()
	if !isFinite(float64(rollDeg)) {
		return model.Point3D{}, fmt.Errorf("bar roll angle must be finite, got %v°", rollDeg)
	}
	rolledPose, err := identityPose2().rotateLocal((-rollDeg).Angle())
	if err != nil {
		return model.Point3D{}, err
	}
	pt, err := rolledPose.transformPoint(h.gripLocal)
	if err != nil {
		return model.Point3D{}, err
	}
	return model.Point3D{X: pt.X, Y: pt.Y, Z: h.zHandMm}, nil
}

// AttachHandlebar mounts bar onto m with clamp roll angle rollDeg and returns
// the immutable HandlebarPlacement.
//
// Roll convention:
//   - rollDeg is measured relative to BarClampMount's reference attitude
//     (leveled bicycle world horizontal +X forward, vertical +Y up).
//   - Positive rollDeg rolls the top of the bar forward (clockwise in the +X/+Y
//     sagittal plane).
func (m BarClampMount) AttachHandlebar(bar Handlebar, rollDeg model.Deg) (HandlebarPlacement, error) {
	m.mustBeValid()
	bar.mustBeValid()
	if !isFinite(float64(rollDeg)) {
		return HandlebarPlacement{}, fmt.Errorf("bar roll angle must be finite, got %v°", rollDeg)
	}

	// Positive rollDeg rolls the top of the bar forward (clockwise = -rollDeg CCW).
	rolledPose, err := m.barPose.rotateLocal((-rollDeg).Angle())
	if err != nil {
		return HandlebarPlacement{}, err
	}
	handGrip, err := rolledPose.transformPoint(bar.gripLocal)
	if err != nil {
		return HandlebarPlacement{}, err
	}
	return HandlebarPlacement{
		handGrip: handGrip,
		valid:    true,
	}, nil
}

// HandlebarPlacement is the immutable solved placement of a handlebar on a stem.
type HandlebarPlacement struct {
	handGrip geom2d.Point2
	valid    bool
}

func (p HandlebarPlacement) mustBeValid() {
	if !p.valid {
		panic("assembly: uninitialized or invalid HandlebarPlacement")
	}
	if !isFinite(p.handGrip.X) || !isFinite(p.handGrip.Y) {
		panic("assembly: non-finite HandlebarPlacement landmarks")
	}
}

// HandGrip returns the world coordinates of the hand-grip center in the sagittal plane.
func (p HandlebarPlacement) HandGrip() geom2d.Point2 {
	p.mustBeValid()
	return p.handGrip
}
