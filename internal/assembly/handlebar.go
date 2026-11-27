package assembly

import (
	"fmt"

	"github.com/matta/bikegeo/internal/pose"
)

// HandlebarLocal has origin (0, 0) at BarClamp center, +X pointing forward at
// 0° bar roll, and +Y pointing 90° CCW (upward along rise and upsweep).
var HandlebarLocal = pose.NewCoordinateSystem("part:handlebar")

// RiseMeasurementMode identifies how a handlebar's StatedRiseMm is measured.
type RiseMeasurementMode string

const (
	// RiseModeStandardKnuckle measures StatedRiseMm at the bend knuckle (prior to outboard upsweep).
	RiseModeStandardKnuckle RiseMeasurementMode = "standard_knuckle"
	// RiseModeBarEndCenter measures StatedRiseMm at the stock bar-end center (outboard upsweep included).
	RiseModeBarEndCenter RiseMeasurementMode = "bar_end_center"
)

// HandlebarDimensions specifies a handlebar's intrinsic 3D geometry and grip
// station independent of roll or mounting:
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
type HandlebarDimensions struct {
	StockWidthMm   float64
	CurrentWidthMm float64
	BacksweepDeg   pose.Deg
	UpsweepDeg     pose.Deg
	StatedRiseMm   float64
	RiseMode       RiseMeasurementMode
	BendStartMm    float64
	GripInsetMm    float64
}

// Handlebar is a validated handlebar with its 3D-to-2D sagittal grip projection
// precomputed in HandlebarLocal. Construct instances with NewHandlebar.
type Handlebar struct {
	dims             HandlebarDimensions
	effectiveWidthMm float64
	handLateralMm    float64
	baseRiseAtBendMm float64
	gripInBar        pose.Point
	valid            bool
}

// NewHandlebar validates dims and precomputes the analytical 3D-to-2D sagittal
// projection of the hand grip in HandlebarLocal.
func NewHandlebar(dims HandlebarDimensions) (Handlebar, error) {
	if !isFinite(dims.StockWidthMm) || dims.StockWidthMm < 0 {
		return Handlebar{}, fmt.Errorf("handlebar stock width must be finite and non-negative, got %v mm", dims.StockWidthMm)
	}
	if !isFinite(dims.CurrentWidthMm) || dims.CurrentWidthMm < 0 {
		return Handlebar{}, fmt.Errorf("handlebar current width must be finite and non-negative, got %v mm", dims.CurrentWidthMm)
	}
	if dims.StockWidthMm > 0 && dims.CurrentWidthMm > dims.StockWidthMm {
		return Handlebar{}, fmt.Errorf("handlebar current width (%v mm) must not exceed stock width (%v mm)", dims.CurrentWidthMm, dims.StockWidthMm)
	}
	width := dims.StockWidthMm
	if dims.CurrentWidthMm > 0 {
		width = dims.CurrentWidthMm
	}
	if !isFinite(width) || width <= 0 {
		return Handlebar{}, fmt.Errorf("handlebar effective width must be finite and positive, got %v mm", width)
	}
	if !isFinite(dims.BendStartMm) || dims.BendStartMm < 0 {
		return Handlebar{}, fmt.Errorf("handlebar bend start must be finite and non-negative, got %v mm", dims.BendStartMm)
	}
	if !isFinite(dims.StatedRiseMm) {
		return Handlebar{}, fmt.Errorf("handlebar stated rise must be finite, got %v mm", dims.StatedRiseMm)
	}
	if !isFinite(float64(dims.BacksweepDeg)) || dims.BacksweepDeg <= -90 || dims.BacksweepDeg >= 90 {
		return Handlebar{}, fmt.Errorf("handlebar backsweep %v° must be in (-90°, 90°)", dims.BacksweepDeg)
	}
	if !isFinite(float64(dims.UpsweepDeg)) || dims.UpsweepDeg <= -90 || dims.UpsweepDeg >= 90 {
		return Handlebar{}, fmt.Errorf("handlebar upsweep %v° must be in (-90°, 90°)", dims.UpsweepDeg)
	}
	if !isFinite(dims.GripInsetMm) || dims.GripInsetMm <= 0 || dims.GripInsetMm >= width/2.0 {
		return Handlebar{}, fmt.Errorf("handlebar grip inset %v mm must be in (0, %.1f)", dims.GripInsetMm, width/2.0)
	}

	var baseRise float64
	switch dims.RiseMode {
	case RiseModeBarEndCenter:
		if dims.StockWidthMm <= 0 {
			return Handlebar{}, fmt.Errorf("handlebar stock width must be finite and positive for bar_end_center rise mode, got %v mm", dims.StockWidthMm)
		}
		if dims.BendStartMm > dims.StockWidthMm/2.0 {
			return Handlebar{}, fmt.Errorf("handlebar bend start (%v mm) must not exceed stock half-width (%.1f mm) for bar_end_center rise mode", dims.BendStartMm, dims.StockWidthMm/2.0)
		}
		stockTipSpan := dims.StockWidthMm/2.0 - dims.BendStartMm
		baseRise = dims.StatedRiseMm - pose.DeflectionOverSpan(stockTipSpan, dims.UpsweepDeg)
	case RiseModeStandardKnuckle:
		baseRise = dims.StatedRiseMm
	default:
		return Handlebar{}, fmt.Errorf("unsupported handlebar rise mode %q", dims.RiseMode)
	}

	zHand := width/2.0 - dims.GripInsetMm
	sweptSpan := max(0.0, zHand-dims.BendStartMm)
	sweepDX := -pose.DeflectionOverSpan(sweptSpan, dims.BacksweepDeg)
	sweepDY := pose.DeflectionOverSpan(sweptSpan, dims.UpsweepDeg)
	gripInBar := pose.NewPoint(HandlebarLocal, sweepDX, baseRise+sweepDY)
	if !isFinite(baseRise) || !isFinitePoint(gripInBar) {
		return Handlebar{}, fmt.Errorf("handlebar grip projection overflow")
	}

	return Handlebar{
		dims:             dims,
		effectiveWidthMm: width,
		handLateralMm:    zHand,
		baseRiseAtBendMm: baseRise,
		gripInBar:        gripInBar,
		valid:            true,
	}, nil
}

func (h Handlebar) requireValid() {
	if !h.valid {
		panic("assembly: uninitialized zero-value Handlebar")
	}
}

// GripInBar returns the 2D sagittal hand-grip position in HandlebarLocal.
func (h Handlebar) GripInBar() pose.Point {
	h.requireValid()
	return h.gripInBar
}

// HandLateralMm returns the lateral distance from the handlebar centerline to
// the hand-grip station (installed width/2 − GripInsetMm).
func (h Handlebar) HandLateralMm() float64 {
	h.requireValid()
	return h.handLateralMm
}

// Dimensions returns the validated HandlebarDimensions used to construct h.
func (h Handlebar) Dimensions() HandlebarDimensions {
	h.requireValid()
	return h.dims
}

// BarTurnForWorldRoll returns the physical clamp rotation (CCW relative to
// StemLocal's +X extension heading) that sets the handlebar's world roll to
// worldBarRollDeg in stemHeadingInWorld's ambient world coordinate system
// (StaticWorld or DynamicWorld). Positive worldBarRollDeg rolls the top of the
// bar forward (clockwise), corresponding to world heading −worldBarRollDeg.
// It panics deterministically if stemHeadingInWorld is not in StaticWorld or
// DynamicWorld.
func BarTurnForWorldRoll(stemHeadingInWorld pose.Heading, worldBarRollDeg pose.Deg) pose.Deg {
	if _, ok := LoadStateForWorld(stemHeadingInWorld.CoordinateSystem()); !ok {
		panic(fmt.Sprintf("assembly: BarTurnForWorldRoll requires %s or %s heading, got %s",
			StaticWorld, DynamicWorld, stemHeadingInWorld.CoordinateSystem()))
	}
	return pose.NewHeading(stemHeadingInWorld.CoordinateSystem(), -worldBarRollDeg).Sub(stemHeadingInWorld)
}

// ClampedHandlebar is a handlebar locked at a physical roll angle relative to
// StemLocal. Placing it through different barClampInTarget transforms
// (StaticWorld vs. DynamicWorld) moves the bar rigidly with the stem without
// re-leveling to gravity.
type ClampedHandlebar struct {
	bar              Handlebar
	rollTurnFromStem pose.Deg
	barInStem        pose.Transform
}

// MountHandlebar clamps bar onto barClamp at physical rotation rollTurnFromStem
// (CCW from StemLocal's +X axis to HandlebarLocal's +X axis). It panics
// deterministically on a zero Handlebar or wrong-CoordinateSystem BarClampPort,
// and returns an error if barClamp.ClampInParent or rollTurnFromStem is non-finite.
func MountHandlebar(barClamp BarClampPort, bar Handlebar, rollTurnFromStem pose.Deg) (ClampedHandlebar, error) {
	bar.requireValid()
	if !barClamp.ClampInParent.Src().Valid() || !barClamp.ClampInParent.Dst().Valid() {
		panic("assembly: BarClampPort.ClampInParent has uninitialized CoordinateSystem")
	}
	if barClamp.ClampInParent.Src() != StemLocal {
		panic(fmt.Sprintf("assembly: MountHandlebar coordinate system mismatch (want source %s, got %s → %s)",
			StemLocal, barClamp.ClampInParent.Src(), barClamp.ClampInParent.Dst()))
	}
	if !isFinitePoint(barClamp.ClampInParent.OriginInDst()) || !isFinite(float64(barClamp.ClampInParent.HeadingInDst().Deg())) {
		return ClampedHandlebar{}, fmt.Errorf("bar clamp transform must be finite")
	}
	if !isFinite(float64(rollTurnFromStem)) {
		return ClampedHandlebar{}, invalidInput(InputHandlebarRoll, "handlebar roll must be finite, got %v°", rollTurnFromStem)
	}
	barInStem := pose.OriginPose(StemLocal).Turn(rollTurnFromStem).Anchor(HandlebarLocal)
	return ClampedHandlebar{
		bar:              bar,
		rollTurnFromStem: rollTurnFromStem,
		barInStem:        barInStem,
	}, nil
}

func (c ClampedHandlebar) requireValid() {
	if !c.bar.valid {
		panic("assembly: uninitialized zero-value ClampedHandlebar")
	}
}

// Handlebar returns the mounted Handlebar specification.
func (c ClampedHandlebar) Handlebar() Handlebar {
	c.requireValid()
	return c.bar
}

// HandlebarPlacement holds the Pose-valued sagittal landmarks and lateral grip
// station produced for a mounted handlebar in a target coordinate system:
//   - BarClamp: incoming stem bar-clamp attachment pose.
//   - BarCenter: the handlebar's center at BarClamp, turned by the locked clamp
//     angle relative to the stem.
//   - HandGrip: sagittal hand-grip projection in the rolled handlebar orientation.
//   - HandLateralMm: lateral grip coordinate (installed width/2 - GripInsetMm)
//     preserved behind the 2D side projection.
type HandlebarPlacement struct {
	BarClamp      pose.Pose
	BarCenter     pose.Pose
	HandGrip      pose.Pose
	HandLateralMm float64
}

// Place maps the mounted handlebar's landmarks through barClampInTarget
// (StemLocal → target coordinate system, such as StaticWorld, DynamicWorld, or
// FrameLocal). It panics deterministically if barClampInTarget does not have
// StemLocal as its source coordinate system.
func (c ClampedHandlebar) Place(barClampInTarget pose.Transform) HandlebarPlacement {
	c.requireValid()
	barInTarget := pose.Compose(c.barInStem, barClampInTarget)
	barClampPose := barClampInTarget.PoseInDst()
	barCenter := barInTarget.PoseInDst()
	handGrip := barInTarget.ApplyPose(pose.NewPose(c.bar.GripInBar(), 0))
	return HandlebarPlacement{
		BarClamp:      barClampPose,
		BarCenter:     barCenter,
		HandGrip:      handGrip,
		HandLateralMm: c.bar.HandLateralMm(),
	}
}
