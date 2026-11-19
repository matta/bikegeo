package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// FitTarget holds the landmark targets for ApplyFitting, in world coordinates
// (BB at origin, +X forward, +Y up, mm). A nil landmark is not fitted, and the
// inputs that would fit it are kept from the setup.
type FitTarget struct {
	BRP      *pose.Point // saddle BRP (Q7)
	HandGrip *pose.Point // handlebar HandGrip (Q11)
}

// FitOutcome says whether a Fitting reached its target or stopped at a hardware limit.
type FitOutcome string

const (
	FitReached FitOutcome = "reached"
	FitLimited FitOutcome = "limited"
)

// Fitting is the result of ApplyFitting: the fitted setup, its forward
// placement (the final check), and which limits stopped it.
type Fitting struct {
	Target    FitTarget     // copies of the requested landmark targets
	Placement BikePlacement // PlaceBike(fitted setup); Placement.Setup is the fitted setup
	Outcome   FitOutcome
	// LimitedBy lists the fitted inputs that sit on a hardware bound, in the
	// order InputSeatpostExtension, InputSaddleRailOffset, InputStemSpacers,
	// InputStemLength; nil when every target was reached.
	LimitedBy []InputField
}

// BrpMiss returns target − placed BRP (world mm), and false when the target has no BRP.
func (f Fitting) BrpMiss() (pose.Point, bool) {
	if f.Target.BRP == nil {
		return pose.Point{}, false
	}
	return f.Target.BRP.Sub(f.Placement.Saddle.BRP.Origin()), true
}

// GripMiss returns target − placed HandGrip (world mm), and false when the target has no HandGrip.
func (f Fitting) GripMiss() (pose.Point, bool) {
	if f.Target.HandGrip == nil {
		return pose.Point{}, false
	}
	return f.Target.HandGrip.Sub(f.Placement.Handlebar.HandGrip.Origin()), true
}

// ApplyFitting fits Seatpost.ExtensionMm and Saddle.RailOffsetMm so the saddle BRP
// lands on a BRP target, and Stem.SpacersMm and Stem.LengthMm so the HandGrip
// lands on a HandGrip target, each within hardware limits. All other setup
// fields are kept. LimitedBy lists limited inputs saddle first, then cockpit.
// trace, if non-nil, records only the final PlaceBike of the fitted setup and is
// frozen on return.
func ApplyFitting(setup BikeSetup, target FitTarget, trace *pose.Trace) (Fitting, error) {
	defer trace.Freeze()
	if target.BRP == nil && target.HandGrip == nil {
		return Fitting{}, fmt.Errorf("fit target has no landmarks")
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X, target.BRP.Y)
	}
	if target.HandGrip != nil && !isFinitePoint(*target.HandGrip) {
		return Fitting{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X, target.HandGrip.Y)
	}

	// Each landmark is linear in its two fitted inputs: place once at a probe
	// and decompose the remaining offset along the two fixed directions.
	probeSetup := setup
	if target.BRP != nil {
		probeSetup.Seatpost.ExtensionMm = setup.Seatpost.MinExtensionMm
		probeSetup.Saddle.RailOffsetMm = 0
	}
	if target.HandGrip != nil {
		minLen, maxLen := setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm
		if !isFinite(minLen) || !isFinite(maxLen) || minLen <= 0 || minLen > maxLen {
			return Fitting{}, fmt.Errorf("stem length range must be finite with 0 < min <= max, got [%v, %v] mm", minLen, maxLen)
		}
		probeSetup.Stem.SpacersMm = 0
		probeSetup.Stem.LengthMm = minLen
	}
	probe, err := PlaceBike(probeSetup, nil)
	if err != nil {
		return Fitting{}, err
	}

	fitted := setup
	var limitedBy []InputField
	var saddleLimited, cockpitLimited bool
	if target.BRP != nil {
		ext, rail, limited, err := fitSaddle(setup, probe, *target.BRP)
		if err != nil {
			return Fitting{}, err
		}
		fitted.Seatpost.ExtensionMm, fitted.Saddle.RailOffsetMm = ext, rail
		saddleLimited = len(limited) > 0
		limitedBy = append(limitedBy, limited...)
	}
	if target.HandGrip != nil {
		spacers, length, limited, err := fitCockpit(setup, probe, *target.HandGrip)
		if err != nil {
			return Fitting{}, err
		}
		fitted.Stem.SpacersMm, fitted.Stem.LengthMm = spacers, length
		cockpitLimited = len(limited) > 0
		limitedBy = append(limitedBy, limited...)
	}

	placement, err := PlaceBike(fitted, trace)
	if err != nil {
		return Fitting{}, err
	}

	fit := Fitting{Placement: placement, Outcome: FitReached}
	if target.BRP != nil {
		p := *target.BRP
		fit.Target.BRP = &p
	}
	if target.HandGrip != nil {
		p := *target.HandGrip
		fit.Target.HandGrip = &p
	}
	if len(limitedBy) > 0 {
		fit.Outcome = FitLimited
		fit.LimitedBy = limitedBy
	}
	if miss, ok := fit.BrpMiss(); ok && !saddleLimited && miss.Norm() > 1e-6 {
		return Fitting{}, fmt.Errorf("saddle fitting forward check missed target by %v mm", miss.Norm())
	}
	if miss, ok := fit.GripMiss(); ok && !cockpitLimited && miss.Norm() > 1e-6 {
		return Fitting{}, fmt.Errorf("cockpit fitting forward check missed target by %v mm", miss.Norm())
	}
	return fit, nil
}

// fitCockpit fits stem spacers and stem length to a HandGrip target, using
// directions read from the probe placement (spacers 0, minimum stem length).
func fitCockpit(setup BikeSetup, probe BikePlacement, grip pose.Point) (spacers, length float64, limited []InputField, err error) {
	minLen, maxLen := setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm
	delta := grip.Sub(probe.Handlebar.HandGrip.Origin())
	a := fitAxis{probe.Stem.SpacerTop.HeadingDeg(), 0, setup.Stem.MaxSpacersMm, InputStemSpacers}
	b := fitAxis{probe.Stem.BarClamp.HeadingDeg(), 0, maxLen - minLen, InputStemLength}
	ds, dl, limited, ok := fitBox(delta, a, b)
	if !ok {
		return 0, 0, nil, fmt.Errorf("stem axis is parallel to the steerer; cannot fit the cockpit")
	}
	return clampFit(ds, 0, setup.Stem.MaxSpacersMm), clampFit(minLen+dl, minLen, maxLen), limited, nil
}

// fitSaddle fits seatpost extension and saddle rail offset to a BRP target,
// using directions read from the probe placement.
func fitSaddle(setup BikeSetup, probe BikePlacement, brp pose.Point) (ext, rail float64, limited []InputField, err error) {
	railLimit, err := setup.RailOffsetLimitMm()
	if err != nil {
		return 0, 0, nil, err
	}
	eMin := setup.Seatpost.MinExtensionMm
	eMax := math.Inf(1)
	if setup.Seatpost.MaxExtensionMm != 0 {
		eMax = setup.Seatpost.MaxExtensionMm
	}
	delta := brp.Sub(probe.Saddle.BRP.Origin())
	a := fitAxis{probe.Seatpost.ShaftTop.HeadingDeg(), 0, eMax - eMin, InputSeatpostExtension}
	b := fitAxis{probe.Saddle.RailCenter.HeadingDeg() + 180, -railLimit, railLimit, InputSaddleRailOffset}
	de, dr, limited, ok := fitBox(delta, a, b)
	if !ok {
		return 0, 0, nil, fmt.Errorf("saddle rail pitch is parallel to the seat axis; cannot fit the saddle")
	}
	return clampFit(eMin+de, eMin, eMax), clampFit(dr, -railLimit, railLimit), limited, nil
}

func clampFit(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

// fitBoundTolMm is how far outside a hardware bound an unconstrained fitting
// value may land from round-off and still count as reached.
const fitBoundTolMm = 1e-9

// fitAxis is one fitted input: the world heading its Advance moves the target
// landmark along, its bounds relative to the probe value (hi may be +Inf), and
// the input reported when the fitted value sits on a bound.
type fitAxis struct {
	heading pose.Deg
	lo, hi  float64
	field   InputField
}

// fitBox solves delta ≈ da·dir(a.heading) + db·dir(b.heading) with da in
// [a.lo, a.hi] and db in [b.lo, b.hi]. When the unconstrained solution is
// outside the box, the box minimum lies on an edge; the four edges are
// evaluated in a fixed order and the closest wins, with ties keeping the
// earlier edge. limited lists a.field then b.field when the chosen value sits
// on a bound; it is nil when delta is reachable. An unconstrained solution
// within fitBoundTolMm of a bound counts as reachable and is snapped onto the
// bound, so round-off in the decomposition cannot turn an exactly reachable
// target into a limited one. ok is false when the headings are parallel.
func fitBox(delta pose.Point, a, b fitAxis) (da, db float64, limited []InputField, ok bool) {
	da, db, ok = pose.DecomposeAlong(delta, a.heading, b.heading)
	if !ok {
		return 0, 0, nil, false
	}
	if da >= a.lo-fitBoundTolMm && da <= a.hi+fitBoundTolMm &&
		db >= b.lo-fitBoundTolMm && db <= b.hi+fitBoundTolMm {
		return clampFit(da, a.lo, a.hi), clampFit(db, b.lo, b.hi), nil, true
	}

	miss := func(x, y float64) float64 {
		return delta.Sub(pose.Along(a.heading, x)).Sub(pose.Along(b.heading, y)).Norm()
	}
	// Fix a at x; the best b is the clamped projection.
	fixA := func(x float64) (float64, float64) {
		y := clampFit(pose.ProjectOnto(delta.Sub(pose.Along(a.heading, x)), b.heading), b.lo, b.hi)
		return x, y
	}
	// Fix b at y; the best a is the clamped projection.
	fixB := func(y float64) (float64, float64) {
		x := clampFit(pose.ProjectOnto(delta.Sub(pose.Along(b.heading, y)), a.heading), a.lo, a.hi)
		return x, y
	}

	type candidate struct{ x, y, miss float64 }
	var cands []candidate
	add := func(x, y float64) { cands = append(cands, candidate{x, y, miss(x, y)}) }
	add(fixA(a.lo))
	if !math.IsInf(a.hi, 1) {
		add(fixA(a.hi))
	}
	add(fixB(b.lo))
	if !math.IsInf(b.hi, 1) {
		add(fixB(b.hi))
	}

	best := cands[0]
	for _, c := range cands[1:] {
		if c.miss < best.miss-1e-9 {
			best = c
		}
	}
	if best.x == a.lo || best.x == a.hi {
		limited = append(limited, a.field)
	}
	if best.y == b.lo || best.y == b.hi {
		limited = append(limited, b.field)
	}
	return best.x, best.y, limited, true
}
