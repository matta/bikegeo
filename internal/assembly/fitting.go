package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

// FitTarget holds the landmark targets for fitting and scoring, in ambient
// world coordinates (BB at origin, +X forward, +Y up, mm). A nil landmark is
// not fitted, and the inputs that would fit it are kept from the setup.
type FitTarget struct {
	BRP      *pose.Point // saddle BRP target in DynamicWorld
	HandGrip *pose.Point // handlebar HandGrip target in DynamicWorld
}

// FitGateGripNormalizedRadius is the largest normalized radius that passes
// the HandGrip gate. A radius of 1 lies on the edge of the fit oval centered
// on the HandGrip target.
const FitGateGripNormalizedRadius = 1.0

// The BRP gate requires the absolute vertical and horizontal misses to be
// within their respective limits in millimeters.
const (
	FitGateBrpYMm = 2.0
	FitGateBrpXMm = 10.0
)

// FitOutcome says whether a Fitting reached its target or stopped at a hardware limit.
type FitOutcome string

const (
	FitReached FitOutcome = "reached"
	FitLimited FitOutcome = "limited"
)

// Fitting is the result of ApplyFitting or FitPostSetback: the fitted setup,
// its forward placement (the final check), and which limits stopped it.
type Fitting struct {
	Target    FitTarget // copies of the requested landmark targets
	Placement Placement // forward placement of the fitted build
	Outcome   FitOutcome
	// LimitedBy lists the fitted inputs that sit on a hardware bound, in the
	// order InputSeatpostInsertion, InputSaddleRailOffset, InputStemSpacers,
	// InputStemLength (or InputSeatpostInsertion, InputSeatpostSetback for
	// FitPostSetback); nil when every target was reached.
	LimitedBy []InputField
}

// BrpMiss returns target − placed BRP (world mm), and false when the target has no BRP.
func (f Fitting) BrpMiss() (pose.Vector, bool) {
	if f.Target.BRP == nil {
		return pose.Vector{}, false
	}
	return f.Target.BRP.Sub(f.Placement.Saddle.BRP.Origin()), true
}

// GripMiss returns target − placed HandGrip (world mm), and false when the target has no HandGrip.
func (f Fitting) GripMiss() (pose.Vector, bool) {
	if f.Target.HandGrip == nil {
		return pose.Vector{}, false
	}
	return f.Target.HandGrip.Sub(f.Placement.Handlebar.HandGrip.Origin()), true
}

// FrontCockpitDoF represents a SeatedFrameset paired with a stem angle, clamp
// height, stem length range, handlebar, and dynamic bar roll whose steerer
// spacer stack and continuous stem length are still open.
type FrontCockpitDoF struct {
	seated            *SeatedFrameset
	stemAngleDeg      pose.Deg
	stemClampHeightMm float64
	stemLenRange      BoundedRange
	bar               Handlebar
	dynamicBarRollDeg pose.Deg
	probeBuild        *Build
	probePlacement    Placement
}

// OpenFrontCockpit validates the continuous front-cockpit parameters on seated
// and probes once at (0 mm spacers, stemLenRange.Min()) to construct the
// 2-DoF affine HandGrip sheet in DynamicWorld.
func OpenFrontCockpit(
	seated *SeatedFrameset,
	stemAngleDeg pose.Deg,
	stemClampHeightMm float64,
	stemLenRange BoundedRange,
	bar Handlebar,
	dynamicBarRollDeg pose.Deg,
) (*FrontCockpitDoF, error) {
	seated.requireValid()
	bar.requireValid()
	if !stemLenRange.Valid() || !isFinite(stemLenRange.Min()) || !isFinite(stemLenRange.Max()) || stemLenRange.Min() <= 0 {
		return nil, fmt.Errorf("stem length range must be finite with 0 < min <= max")
	}
	if !isFinite(float64(dynamicBarRollDeg)) {
		return nil, invalidInput(InputHandlebarRoll, "handlebar roll must be finite, got %v°", dynamicBarRollDeg)
	}
	probeStem, err := NewStem(stemClampHeightMm, stemLenRange.Min(), stemAngleDeg)
	if err != nil {
		return nil, err
	}
	probeBuild, err := seated.AttachFront(0, probeStem, bar, dynamicBarRollDeg)
	if err != nil {
		return nil, err
	}
	probePlacement := probeBuild.Place(Dynamic)
	return &FrontCockpitDoF{
		seated:            seated,
		stemAngleDeg:      stemAngleDeg,
		stemClampHeightMm: stemClampHeightMm,
		stemLenRange:      stemLenRange,
		bar:               bar,
		dynamicBarRollDeg: dynamicBarRollDeg,
		probeBuild:        probeBuild,
		probePlacement:    probePlacement,
	}, nil
}

func (d *FrontCockpitDoF) requireValid() {
	if d == nil || d.seated == nil || d.probeBuild == nil {
		panic("assembly: uninitialized FrontCockpitDoF")
	}
}

// AllowedSpacers returns the frameset steerer's permitted spacer interval.
func (d *FrontCockpitDoF) AllowedSpacers() BoundedRange {
	d.requireValid()
	return d.seated.Frameset().Steerer().AllowedSpacers()
}

// Planar2DoF returns the 2-DoF affine HandGrip surface in DynamicWorld
// parameterized by (StemSpacers, StemLength).
func (d *FrontCockpitDoF) Planar2DoF() Planar2DoF {
	d.requireValid()
	return Planar2DoF{
		Origin: d.probePlacement.Handlebar.HandGrip.Origin(),
		AxisA: LinearDoF{
			Field:   InputStemSpacers,
			Heading: d.probePlacement.Stem.SpacerTop.Heading(),
			Range:   d.AllowedSpacers(),
			Probe:   0,
			Sign:    +1,
		},
		AxisB: LinearDoF{
			Field:   InputStemLength,
			Heading: d.probePlacement.Stem.BarClamp.Heading(),
			Range:   d.stemLenRange,
			Probe:   d.stemLenRange.Min(),
			Sign:    +1,
		},
	}
}

// Clamp locks the front cockpit at spacersMm and stemLengthMm, returning an
// immutable Build.
func (d *FrontCockpitDoF) Clamp(spacersMm, stemLengthMm float64) (*Build, error) {
	d.requireValid()
	if _, err := d.stemLenRange.Bind(InputStemLength, stemLengthMm); err != nil {
		return nil, err
	}
	stem, err := NewStem(d.stemClampHeightMm, stemLengthMm, d.stemAngleDeg)
	if err != nil {
		return nil, err
	}
	return d.seated.AttachFront(spacersMm, stem, d.bar, d.dynamicBarRollDeg)
}

// FitToGrip solves the front cockpit's Planar2DoF for target with
// PriorityNearest and clamps the resulting Build.
func (d *FrontCockpitDoF) FitToGrip(target pose.Point) (*Build, []InputField, error) {
	d.requireValid()
	if !isFinitePoint(target) {
		return nil, nil, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.X(), target.Y())
	}
	spacers, length, limited, ok := SolvePlanar2DoF(d.Planar2DoF(), target, PriorityNearest)
	if !ok {
		return nil, nil, fmt.Errorf("stem axis is parallel to the steerer; cannot fit the cockpit")
	}
	build, err := d.Clamp(spacers, length)
	if err != nil {
		return nil, nil, err
	}
	if len(limited) == 0 {
		if miss := target.Sub(build.Place(Dynamic).Handlebar.HandGrip.Origin()); miss.Norm() > 1e-6 {
			return nil, nil, fmt.Errorf("cockpit fitting forward check missed target by %v mm", miss.Norm())
		}
	}
	return build, limited, nil
}

// FitPostSetback fits a hypothetical rigid seatpost's SetbackMm and
// InsertionMm on b so the saddle BRP lands on brp, with saddle rail offset
// held at railOffsetMm. It explains the post setback a BRP target needs, and
// callers never install its build. Insertion keeps the frame and post limits.
// Setback is at least 0 and has no upper bound. LimitedBy lists
// InputSeatpostInsertion, then InputSeatpostSetback, for inputs on a bound.
func (b *Build) FitPostSetback(brp pose.Point, railOffsetMm float64) (Fitting, error) {
	b.requireValid()
	if !isFinitePoint(brp) {
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", brp.X(), brp.Y())
	}
	fs := b.seated.frameset
	postSnap := b.seated.seatpost.post
	saddle := b.seated.saddle.Saddle()
	railPitch := b.seated.dynamicRailPitchDeg
	spacersMm := b.stem.Spacers().Value
	stem := b.stem.Stem()
	bar := b.handlebar.Handlebar()
	barRoll := b.dynamicBarRollDeg

	// Probe a zero-setback rigid post with the same length, insertion, and clamp
	// geometry at maximum insertion and railOffsetMm.
	zeroSetbackPost, err := NewRigidPost(
		postSnap.totalLengthMm,
		postSnap.minInsertionMm,
		postSnap.maxInsertionMm,
		0,
		postSnap.dynamicClamp.ClampLengthMm,
	)
	if err != nil {
		return Fitting{}, err
	}
	probeRear, err := OpenRearCockpit(fs, zeroSetbackPost, saddle, railPitch)
	if err != nil {
		return Fitting{}, err
	}
	insRange := probeRear.AllowedInsertion()
	probeSeated, err := probeRear.Clamp(insRange.Max(), railOffsetMm)
	if err != nil {
		return Fitting{}, err
	}

	setbackRange, err := NewBoundedRange(0, math.Inf(1), "min seatpost setback", "")
	if err != nil {
		return Fitting{}, err
	}
	shaftHeading := probeSeated.DynamicSeatpost().ShaftTop.Heading()
	sheet := Planar2DoF{
		Origin: probeSeated.DynamicSaddle().BRP.Origin(),
		AxisA: LinearDoF{
			Field:   InputSeatpostInsertion,
			Heading: shaftHeading,
			Range:   insRange,
			Probe:   insRange.Max(),
			Sign:    -1,
		},
		AxisB: LinearDoF{
			Field:   InputSeatpostSetback,
			Heading: shaftHeading.Turn(90.0),
			Range:   setbackRange,
			Probe:   0,
			Sign:    +1,
		},
	}
	ins, setback, limited, ok := SolvePlanar2DoF(sheet, brp, PriorityNearest)
	if !ok {
		return Fitting{}, fmt.Errorf("seatpost setback is parallel to the seat axis; cannot fit the setback")
	}

	fittedPost, err := NewRigidPost(
		postSnap.totalLengthMm,
		postSnap.minInsertionMm,
		postSnap.maxInsertionMm,
		setback,
		postSnap.dynamicClamp.ClampLengthMm,
	)
	if err != nil {
		return Fitting{}, err
	}
	fittedRear, err := OpenRearCockpit(fs, fittedPost, saddle, railPitch)
	if err != nil {
		return Fitting{}, err
	}
	fittedSeated, err := fittedRear.Clamp(ins, railOffsetMm)
	if err != nil {
		return Fitting{}, err
	}
	fittedBuild, err := fittedSeated.AttachFront(spacersMm, stem, bar, barRoll)
	if err != nil {
		return Fitting{}, err
	}
	placement := fittedBuild.Place(Dynamic)

	p := brp
	fit := Fitting{
		Target:    FitTarget{BRP: &p},
		Placement: placement,
		Outcome:   FitReached,
	}
	if len(limited) > 0 {
		fit.Outcome = FitLimited
		fit.LimitedBy = limited
	} else if miss, ok := fit.BrpMiss(); ok && miss.Norm() > 1e-6 {
		return Fitting{}, fmt.Errorf("post setback fitting forward check missed target by %v mm", miss.Norm())
	}
	return fit, nil
}
