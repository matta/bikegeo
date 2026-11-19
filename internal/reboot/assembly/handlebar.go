package assembly

import (
	"fmt"

	"bikegeo/internal/reboot/pose"
)

// RiseMeasurementMode identifies how a handlebar's StatedRiseMm is measured.
type RiseMeasurementMode string

const (
	// RiseModeStandardKnuckle measures StatedRiseMm at the bend knuckle (prior to outboard upsweep).
	RiseModeStandardKnuckle RiseMeasurementMode = "standard_knuckle"
	// RiseModeBarEndCenter measures StatedRiseMm at the stock bar-end center (outboard upsweep included).
	RiseModeBarEndCenter RiseMeasurementMode = "bar_end_center"
)

// HandlebarSetup specifies a handlebar's geometry, grip inset, and world-relative roll:
//   - StockWidthMm: uncut handlebar width in mm.
//   - CurrentWidthMm: installed width in mm (0 means uncut, i.e., StockWidthMm;
//     must not exceed StockWidthMm when StockWidthMm > 0).
//   - BacksweepDeg: rearward sweep angle in the top view (outboard of BendStartMm;
//     positive sweeps rearward, in (-90°, 90°)).
//   - UpsweepDeg: upward sweep angle in the front view (outboard of BendStartMm;
//     positive sweeps upward, in (-90°, 90°)).
//   - StatedRiseMm: published rise in mm according to RiseMode.
//   - RiseMode: standard knuckle or bar-end center measurement convention (required).
//   - BendStartMm: lateral half-width from clamp center to the sweep/rise bend intersection.
//   - GripInsetMm: inward distance from the cut bar end to the hand-grip center.
//   - BarRollDeg: handlebar roll in the side view relative to leveled world horizontal (+X);
//     positive rolls the top of the bar forward (clockwise in the sagittal +X/+Y plane).
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

func (h HandlebarSetup) effectiveWidthMm() float64 {
	if h.CurrentWidthMm > 0 {
		return h.CurrentWidthMm
	}
	return h.StockWidthMm
}

func (h HandlebarSetup) handLateralMm() float64 {
	return h.effectiveWidthMm()/2.0 - h.GripInsetMm
}

func (h HandlebarSetup) baseRiseAtBendMm() (float64, error) {
	switch h.RiseMode {
	case RiseModeBarEndCenter:
		if h.StockWidthMm <= 0 {
			return 0, fmt.Errorf("handlebar stock width must be finite and positive for bar_end_center rise mode, got %v mm", h.StockWidthMm)
		}
		if h.BendStartMm > h.StockWidthMm/2.0 {
			return 0, fmt.Errorf("handlebar bend start (%v mm) must not exceed stock half-width (%.1f mm) for bar_end_center rise mode", h.BendStartMm, h.StockWidthMm/2.0)
		}
		stockTipSpan := h.StockWidthMm/2.0 - h.BendStartMm
		return h.StatedRiseMm - pose.DeflectionOverSpan(stockTipSpan, h.UpsweepDeg), nil
	case RiseModeStandardKnuckle:
		return h.StatedRiseMm, nil
	default:
		return 0, fmt.Errorf("unsupported handlebar rise mode %q", h.RiseMode)
	}
}

func (h HandlebarSetup) outboardSweepOffsetMm(zHand float64) (dx, upsweepDy float64) {
	sweptSpan := max(0.0, zHand-h.BendStartMm)
	dx = -pose.DeflectionOverSpan(sweptSpan, h.BacksweepDeg)
	upsweepDy = pose.DeflectionOverSpan(sweptSpan, h.UpsweepDeg)
	return dx, upsweepDy
}

// HandlebarPlacement holds the Pose-valued sagittal landmarks and lateral grip
// station produced by PlaceHandlebar:
//   - BarClamp: incoming stem bar-clamp attachment pose.
//   - BarRollFrame: pose at BarClamp re-oriented to world-relative -BarRollDeg.
//   - HandGrip: sagittal hand-grip projection in the BarRollFrame orientation.
//   - HandLateralMm: lateral grip coordinate (installed width/2 - GripInsetMm)
//     preserved behind the 2D side projection.
type HandlebarPlacement struct {
	BarClamp      pose.Pose
	BarRollFrame  pose.Pose
	HandGrip      pose.Pose
	HandLateralMm float64
}

// PlaceHandlebar mounts a handlebar at barClamp, orients it to world-relative
// -BarRollDeg via Pose.Face, applies the rise at the bend intersection, and
// applies the outboard backsweep/upsweep projection to place HandGrip while
// preserving HandLateralMm.
func PlaceHandlebar(barClamp pose.Pose, setup HandlebarSetup) (HandlebarPlacement, error) {
	if err := validatePose("barClamp", barClamp); err != nil {
		return HandlebarPlacement{}, err
	}
	if !isFinite(setup.StockWidthMm) || setup.StockWidthMm < 0 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar stock width must be finite and non-negative, got %v mm", setup.StockWidthMm)
	}
	if !isFinite(setup.CurrentWidthMm) || setup.CurrentWidthMm < 0 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar current width must be finite and non-negative, got %v mm", setup.CurrentWidthMm)
	}
	if setup.StockWidthMm > 0 && setup.CurrentWidthMm > setup.StockWidthMm {
		return HandlebarPlacement{}, fmt.Errorf("handlebar current width (%v mm) must not exceed stock width (%v mm)", setup.CurrentWidthMm, setup.StockWidthMm)
	}
	width := setup.effectiveWidthMm()
	if !isFinite(width) || width <= 0 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar effective width must be finite and positive, got %v mm", width)
	}
	if !isFinite(setup.BendStartMm) || setup.BendStartMm < 0 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar bend start must be finite and non-negative, got %v mm", setup.BendStartMm)
	}
	if !isFinite(setup.StatedRiseMm) {
		return HandlebarPlacement{}, fmt.Errorf("handlebar stated rise must be finite, got %v mm", setup.StatedRiseMm)
	}
	if !isFinite(float64(setup.BacksweepDeg)) || setup.BacksweepDeg <= -90 || setup.BacksweepDeg >= 90 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar backsweep %v° must be in (-90°, 90°)", setup.BacksweepDeg)
	}
	if !isFinite(float64(setup.UpsweepDeg)) || setup.UpsweepDeg <= -90 || setup.UpsweepDeg >= 90 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar upsweep %v° must be in (-90°, 90°)", setup.UpsweepDeg)
	}
	if !isFinite(setup.GripInsetMm) || setup.GripInsetMm <= 0 || setup.GripInsetMm >= width/2.0 {
		return HandlebarPlacement{}, fmt.Errorf("handlebar grip inset %v mm must be in (0, %.1f)", setup.GripInsetMm, width/2.0)
	}
	if !isFinite(float64(setup.BarRollDeg)) {
		return HandlebarPlacement{}, invalidInput(InputHandlebarRoll, "handlebar roll must be finite, got %v°", setup.BarRollDeg)
	}

	baseRise, err := setup.baseRiseAtBendMm()
	if err != nil {
		return HandlebarPlacement{}, err
	}
	zHand := setup.handLateralMm()
	dx, upsweepDy := setup.outboardSweepOffsetMm(zHand)

	barRollFrame := barClamp.Face("bar roll vs. world horizontal", -setup.BarRollDeg)
	bendRise := barRollFrame.Translate("rise at bend", 0, baseRise)
	handGrip := bendRise.Translate("outboard sweep projection", dx, upsweepDy)

	if err := validatePlacementPoses("handlebar", barClamp, barRollFrame, bendRise, handGrip); err != nil {
		return HandlebarPlacement{}, err
	}

	return HandlebarPlacement{
		BarClamp:      barClamp,
		BarRollFrame:  barRollFrame,
		HandGrip:      handGrip,
		HandLateralMm: zHand,
	}, nil
}
