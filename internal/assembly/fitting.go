package assembly

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

// FitTarget holds the landmark targets for ApplyFitting, in world coordinates
// (BB at origin, +X forward, +Y up, mm). A nil landmark is not fitted, and the
// inputs that would fit it are kept from the setup.
type FitTarget struct {
	BRP      *pose.Point // saddle BRP (Q7)
	HandGrip *pose.Point // handlebar HandGrip (Q11)
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

// Fitting is the result of ApplyFitting: the fitted setup, its forward
// placement (the final check), and which limits stopped it.
type Fitting struct {
	Target    FitTarget     // copies of the requested landmark targets
	Placement BikePlacement // PlaceBike(fitted setup); Placement.Setup is the fitted setup
	Outcome   FitOutcome
	// LimitedBy lists the fitted inputs that sit on a hardware bound, in the
	// order InputSeatpostInsertion, InputSaddleRailOffset, InputStemSpacers,
	// InputStemLength; nil when every target was reached.
	LimitedBy []InputField
}

// BrpMiss returns target − placed BRP (world mm), and false when the target has no BRP.
func (f Fitting) BrpMiss() (pose.Vec, bool) {
	if f.Target.BRP == nil {
		return pose.Vec{}, false
	}
	return f.Target.BRP.Sub(f.Placement.Saddle.BRP.Origin()), true
}

// GripMiss returns target − placed HandGrip (world mm), and false when the target has no HandGrip.
func (f Fitting) GripMiss() (pose.Vec, bool) {
	if f.Target.HandGrip == nil {
		return pose.Vec{}, false
	}
	return f.Target.HandGrip.Sub(f.Placement.Handlebar.HandGrip.Origin()), true
}

// ApplyFitting fits Seatpost.InsertionMm and Saddle.RailOffsetMm so the saddle BRP
// lands on a BRP target, and Stem.SpacersMm and Stem.LengthMm so the HandGrip
// lands on a HandGrip target, each within hardware limits. All other setup
// fields are kept. LimitedBy lists limited inputs saddle first, then cockpit.
func ApplyFitting(setup BikeSetup, target FitTarget) (Fitting, error) {
	if target.BRP == nil && target.HandGrip == nil {
		return Fitting{}, fmt.Errorf("fit target has no landmarks")
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X(), target.BRP.Y())
	}
	if target.HandGrip != nil && !isFinitePoint(*target.HandGrip) {
		return Fitting{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X(), target.HandGrip.Y())
	}

	// Each landmark is linear in its two fitted inputs: place once at a probe
	// and decompose the remaining offset along the two fixed directions.
	probeSetup := setup
	var bounds InsertionBounds
	if target.BRP != nil {
		var err error
		bounds, err = SeatpostInsertionBounds(setup.Frame, setup.Seatpost.Spec)
		if err != nil {
			return Fitting{}, err
		}
		// Probe at maximum insertion (shortest exposure): the fit then raises the
		// saddle by pulling the post out.
		probeSetup.Seatpost.InsertionMm = bounds.MaxAllowedInsertionMm
		probeSetup.Saddle.RailOffsetMm = 0
	}
	if target.HandGrip != nil {
		if err := validateStemLengthRange(setup.Stem); err != nil {
			return Fitting{}, err
		}
		probeSetup.Stem.SpacersMm = 0
		probeSetup.Stem.LengthMm = setup.Stem.MinLengthMm
	}
	probe, err := PlaceBike(probeSetup)
	if err != nil {
		return Fitting{}, err
	}

	fitted := setup
	var limitedBy []InputField
	var saddleLimited, cockpitLimited bool
	if target.BRP != nil {
		insertion, rail, limited, err := fitSaddle(setup, bounds, probe, *target.BRP)
		if err != nil {
			return Fitting{}, err
		}
		fitted.Seatpost.InsertionMm, fitted.Saddle.RailOffsetMm = insertion, rail
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

	placement, err := PlaceBike(fitted)
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

// FitPostSetback fits Seatpost.Spec.SetbackMm and Seatpost.InsertionMm so the
// saddle BRP lands on brp, with Saddle.RailOffsetMm held at railOffsetMm. It
// explains the post setback a BRP target needs (Q29), and callers never
// install its build. Insertion keeps the frame and post limits. Setback is at least 0
// and has no upper bound. LimitedBy lists InputSeatpostInsertion, then
// InputSeatpostSetback, for the inputs that sit on a bound. The submitted
// insertion, setback, and rail offset are ignored; every other setup field is kept.
func FitPostSetback(setup BikeSetup, brp pose.Point, railOffsetMm float64) (Fitting, error) {
	if !isFinitePoint(brp) {
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", brp.X(), brp.Y())
	}
	bounds, err := SeatpostInsertionBounds(setup.Frame, setup.Seatpost.Spec)
	if err != nil {
		return Fitting{}, err
	}
	// Probe a copy of the post at maximum insertion with no setback.
	probeSetup := setup
	probeSetup.Seatpost.InsertionMm = bounds.MaxAllowedInsertionMm
	probeSetup.Seatpost.Spec.SetbackMm = 0
	probeSetup.Saddle.RailOffsetMm = railOffsetMm
	probe, err := PlaceBike(probeSetup)
	if err != nil {
		return Fitting{}, err
	}

	delta := brp.Sub(probe.Saddle.BRP.Origin())
	shaft := probe.Seatpost.ShaftTop.Heading()
	a := bounds.extensionAdjustment(shaft)
	// Setback has no upper bound: the result describes a hypothetical post
	// (Q29), so no installed hardware limits it.
	b := fitAdjustment{shaft.Turn(90.0), fitInterval{0, math.Inf(1)}, InputSeatpostSetback}
	de, ds, limited, ok := fitBox(delta, a, b, nearestPoint)
	if !ok {
		return Fitting{}, fmt.Errorf("seatpost setback is parallel to the seat axis; cannot fit the setback")
	}

	fitted := probeSetup
	fitted.Seatpost.InsertionMm = bounds.insertionAfterExtension(de)
	fitted.Seatpost.Spec.SetbackMm = ds
	placement, err := PlaceBike(fitted)
	if err != nil {
		return Fitting{}, err
	}

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

// validateStemLengthRange checks the stem length range fitting may choose.
func validateStemLengthRange(stem StemSetup) error {
	minLen, maxLen := stem.MinLengthMm, stem.MaxLengthMm
	if !isFinite(minLen) || !isFinite(maxLen) || minLen <= 0 || minLen > maxLen {
		return fmt.Errorf("stem length range must be finite with 0 < min <= max, got [%v, %v] mm", minLen, maxLen)
	}
	return nil
}

// fitCockpit fits stem spacers and stem length to a HandGrip target, using
// directions read from the probe placement (spacers 0, minimum stem length).
func fitCockpit(setup BikeSetup, probe BikePlacement, grip pose.Point) (spacers, length float64, limited []InputField, err error) {
	minLen, maxLen := setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm
	delta := grip.Sub(probe.Handlebar.HandGrip.Origin())
	a := fitAdjustment{probe.Stem.SpacerTop.Heading(), fitInterval{0, setup.Stem.MaxSpacersMm}, InputStemSpacers}
	b := fitAdjustment{probe.Stem.BarClamp.Heading(), fitInterval{0, maxLen - minLen}, InputStemLength}
	ds, dl, limited, ok := fitBox(delta, a, b, nearestPoint)
	if !ok {
		return 0, 0, nil, fmt.Errorf("stem axis is parallel to the steerer; cannot fit the cockpit")
	}
	return ds, (fitInterval{minLen, maxLen}).clamp(minLen + dl), limited, nil
}

// fitSaddle fits seatpost insertion and saddle rail offset to a BRP target,
// using directions read from the probe placement, which sits at maximum
// insertion. Pulling the post out by the solved travel lowers the insertion.
func fitSaddle(setup BikeSetup, bounds InsertionBounds, probe BikePlacement, brp pose.Point) (insertion, rail float64, limited []InputField, err error) {
	railLimit, err := setup.RailOffsetLimitMm()
	if err != nil {
		return 0, 0, nil, err
	}
	delta := brp.Sub(probe.Saddle.BRP.Origin())
	a := bounds.extensionAdjustment(probe.Seatpost.ShaftTop.Heading())
	b := fitAdjustment{probe.Saddle.RailCenter.Heading().Turn(180), fitInterval{-railLimit, railLimit}, InputSaddleRailOffset}
	de, dr, limited, ok := fitBox(delta, a, b, inRectThenYFirst)
	if !ok {
		return 0, 0, nil, fmt.Errorf("saddle rail pitch is parallel to the seat axis; cannot fit the saddle")
	}
	return bounds.insertionAfterExtension(de), dr, limited, nil
}

// extensionAdjustment returns the seatpost insertion adjustment, with travel
// measured as extension out along the shaft from maximum insertion.
func (bounds InsertionBounds) extensionAdjustment(heading pose.Heading) fitAdjustment {
	return fitAdjustment{heading, fitInterval{0, bounds.MaxAllowedInsertionMm - bounds.EffectiveMinInsertionMm}, InputSeatpostInsertion}
}

func (bounds InsertionBounds) insertionAfterExtension(extension float64) float64 {
	return (fitInterval{bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm}).clamp(bounds.MaxAllowedInsertionMm - extension)
}

const (
	fitBoundTolMm   = 1e-9
	fitDirectionTol = 1e-12 // treat round-off in a horizontal or vertical direction as zero
)

// fitInterval is a closed range of adjustment values in millimeters.
type fitInterval struct{ lo, hi float64 }

func (r fitInterval) clamp(value float64) float64 { return min(max(value, r.lo), r.hi) }

func (r fitInterval) contains(value float64) bool {
	return value >= r.lo-fitBoundTolMm && value <= r.hi+fitBoundTolMm
}

// clip clips the horizontal interval to the portion where the line with
// intercept c and slope -s lies between heights -limit and +limit.
// It returns that interval and true, or an empty result and false. For example,
// clipping [0, 10] with c=6, s=2, limit=2 yields [2, 4].
func (r fitInterval) clip(c, s, limit float64) (fitInterval, bool) {
	if math.Abs(s) < fitDirectionTol {
		if math.Abs(c) <= limit+fitBoundTolMm {
			return r, true
		}
		return fitInterval{}, false
	}
	left, right := (c-limit)/s, (c+limit)/s
	lo, hi := max(r.lo, min(left, right)), min(r.hi, max(left, right))
	if lo > hi+fitBoundTolMm {
		return fitInterval{}, false
	}
	if lo > hi {
		touch := r.clamp((lo + hi) / 2)
		return fitInterval{touch, touch}, true
	}
	return fitInterval{lo, hi}, true
}

// heightFirst returns the multiplier in r (in mm) that brings travel*direction
// closest to target in Y, breaking ties by distance in X. direction is a
// unit-length direction vector in DynamicWorld.
func (r fitInterval) heightFirst(target, direction pose.Vec) float64 {
	if math.Abs(direction.Y()) >= fitDirectionTol {
		return r.clamp(target.Y() / direction.Y())
	}
	if math.Abs(direction.X()) >= fitDirectionTol {
		return r.clamp(target.X() / direction.X())
	}
	return r.lo
}

// fitAdjustment is one fitting input as the solver sees it: the direction it
// moves its landmark (BRP or HandGrip), how far it may move it, and which
// input it is. Travel is in mm from where the landmark sits in the probe
// build.
type fitAdjustment struct {
	heading pose.Heading
	fitInterval
	field InputField
}

type fitPriority int

const (
	nearestPoint fitPriority = iota
	// inRectThenYFirst prefers points inside the target's allowed error
	// rectangle, then minimizes absolute Y error followed by absolute X error.
	inRectThenYFirst
)

// A fitEdge holds one input at a bound and lets the other input vary.
type fitEdge struct {
	fixed, moving fitAdjustment
	bound         float64
}

// candidates returns signed travel distances (in mm) along the moving direction.
// Each distance, paired with the fixed direction's bound, identifies a point on
// the edge relative to the probe landmark. nearestPoint projects delta onto the
// moving direction and clamps to the edge. inRectThenYFirst minimizes absolute
// Y error over the whole edge, breaking ties by absolute X error. If the edge
// intersects the allowed error rectangle around delta, it also returns the best
// distance within that intersection using the same ordering.
func (edge fitEdge) candidates(delta pose.Vec, priority fitPriority) []float64 {
	if priority == nearestPoint {
		target := delta.Sub(pose.Along(edge.fixed.heading, edge.bound))
		return []float64{edge.moving.clamp(pose.ProjectOnto(target, edge.moving.heading))}
	}
	fixedDir := pose.Along(edge.fixed.heading, 1) // unit vector along fixed direction
	target := delta.Sub(pose.Vc(delta.Frame(), edge.bound*fixedDir.X(), edge.bound*fixedDir.Y()))
	dir := pose.Along(edge.moving.heading, 1) // unit vector along moving direction
	travel := edge.moving.heightFirst(target, dir)
	gate, ok := edge.moving.clip(target.Y(), dir.Y(), FitGateBrpYMm)
	if ok {
		if gate, ok = gate.clip(target.X(), dir.X(), FitGateBrpXMm); ok {
			return []float64{travel, gate.heightFirst(target, dir)}
		}
	}
	return []float64{travel}
}

// miss returns the vector from the resulting point to the target delta.
// Each travel[i] is a signed distance in mm along adjustments[i].heading. Together
// the two displacements locate the point relative to the probe landmark.
// nearestPoint subtracts each displacement from delta in turn. The other
// priority subtracts their sum using unit direction vectors dirA and dirB.
// Floating-point rounding at a tolerance boundary can make the evaluation
// order significant.
func (p fitPriority) miss(delta pose.Vec, adjustments [2]fitAdjustment, travel [2]float64) pose.Vec {
	if p == nearestPoint {
		return delta.Sub(pose.Along(adjustments[0].heading, travel[0])).Sub(pose.Along(adjustments[1].heading, travel[1]))
	}
	dirA, dirB := pose.Along(adjustments[0].heading, 1), pose.Along(adjustments[1].heading, 1)
	return delta.Sub(pose.Vc(delta.Frame(), travel[0]*dirA.X()+travel[1]*dirB.X(), travel[0]*dirA.Y()+travel[1]*dirB.Y()))
}

// prefers compares target misses. Ties keep the first candidate visited.
func (p fitPriority) prefers(miss, best pose.Vec) bool {
	if p == nearestPoint {
		return miss.Norm() < best.Norm()-fitBoundTolMm
	}
	x, y := math.Abs(miss.X()), math.Abs(miss.Y())
	bestX, bestY := math.Abs(best.X()), math.Abs(best.Y())
	inGate := func(x, y float64) bool {
		return y <= FitGateBrpYMm+fitBoundTolMm && x <= FitGateBrpXMm+fitBoundTolMm
	}
	if pass := inGate(x, y); pass != inGate(bestX, bestY) {
		return pass
	}
	if math.Abs(y-bestY) > fitBoundTolMm {
		return y < bestY
	}
	return x < bestX-fitBoundTolMm
}

// fitBox returns travel (in mm) along directions a and b to reach delta. For a
// reachable target, limited is nil, including at a bound within round-off.
// Otherwise it fixes each input at its finite bounds and reports the winning
// point's limiting inputs in (a, b) order. Parallel directions return ok=false.
func fitBox(delta pose.Vec, a, b fitAdjustment, priority fitPriority) (da, db float64, limited []InputField, ok bool) {
	da, db, ok = pose.DecomposeAlong(delta, a.heading, b.heading)
	if !ok {
		return 0, 0, nil, false
	}
	if a.contains(da) && b.contains(db) {
		return a.clamp(da), b.clamp(db), nil, true
	}

	adjustments := [2]fitAdjustment{a, b}
	var best [2]float64
	var bestMiss pose.Vec
	haveBest := false
	for fixed, adj := range adjustments {
		moving := 1 - fixed
		for _, bound := range [...]float64{adj.lo, adj.hi} {
			if math.IsInf(bound, 1) {
				continue
			}
			edge := fitEdge{fixed: adj, moving: adjustments[moving], bound: bound}
			for _, travel := range edge.candidates(delta, priority) {
				var setting [2]float64
				setting[fixed], setting[moving] = bound, travel
				miss := priority.miss(delta, adjustments, setting)
				if !haveBest || priority.prefers(miss, bestMiss) {
					best, bestMiss, haveBest = setting, miss, true
				}
			}
		}
	}

	// Clipped error-rectangle intersections can reach a bound within round-off.
	boundTol := 0.0
	if priority == inRectThenYFirst {
		boundTol = fitBoundTolMm
	}
	for i, adj := range adjustments {
		if math.Abs(best[i]-adj.lo) <= boundTol || math.Abs(best[i]-adj.hi) <= boundTol {
			limited = append(limited, adj.field)
		}
	}
	return best[0], best[1], limited, true
}
