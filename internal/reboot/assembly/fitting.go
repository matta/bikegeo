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

// Fit-gate thresholds (Q59a). A build passes the HandGrip gate when
// GripR <= FitGateGripR and passes the BRP gate when |ΔY| <= FitGateBrpYMm
// and |ΔX| <= FitGateBrpXMm.
const (
	FitGateGripR  = 1.0
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

// ApplyFitting fits Seatpost.InsertionMm and Saddle.RailOffsetMm so the saddle BRP
// lands on a BRP target, and Stem.SpacersMm and Stem.LengthMm so the HandGrip
// lands on a HandGrip target, each within hardware limits. All other setup
// fields are kept. LimitedBy lists limited inputs saddle first, then cockpit.
func ApplyFitting(setup BikeSetup, target FitTarget) (Fitting, error) {
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
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", brp.X, brp.Y)
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
	shaft := probe.Seatpost.ShaftTop.HeadingDeg()
	a := fitAxis{shaft, 0, bounds.MaxAllowedInsertionMm - bounds.EffectiveMinInsertionMm, InputSeatpostInsertion}
	b := fitAxis{shaft + 90, 0, math.Inf(1), InputSeatpostSetback} // PlaceSeatpost translates setback along local +Y
	de, ds, limited, ok := fitBox(delta, a, b)
	if !ok {
		return Fitting{}, fmt.Errorf("seatpost setback is parallel to the seat axis; cannot fit the setback")
	}

	fitted := probeSetup
	fitted.Seatpost.InsertionMm = clampFit(bounds.MaxAllowedInsertionMm-de, bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm)
	fitted.Seatpost.Spec.SetbackMm = clampFit(ds, 0, math.Inf(1))
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
	a := fitAxis{probe.Stem.SpacerTop.HeadingDeg(), 0, setup.Stem.MaxSpacersMm, InputStemSpacers}
	b := fitAxis{probe.Stem.BarClamp.HeadingDeg(), 0, maxLen - minLen, InputStemLength}
	ds, dl, limited, ok := fitBox(delta, a, b)
	if !ok {
		return 0, 0, nil, fmt.Errorf("stem axis is parallel to the steerer; cannot fit the cockpit")
	}
	return clampFit(ds, 0, setup.Stem.MaxSpacersMm), clampFit(minLen+dl, minLen, maxLen), limited, nil
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
	a := fitAxis{probe.Seatpost.ShaftTop.HeadingDeg(), 0, bounds.MaxAllowedInsertionMm - bounds.EffectiveMinInsertionMm, InputSeatpostInsertion}
	b := fitAxis{probe.Saddle.RailCenter.HeadingDeg() + 180, -railLimit, railLimit, InputSaddleRailOffset}
	de, dr, limited, ok := fitSaddleBox(delta, a, b)
	if !ok {
		return 0, 0, nil, fmt.Errorf("saddle rail pitch is parallel to the seat axis; cannot fit the saddle")
	}
	return clampFit(bounds.MaxAllowedInsertionMm-de, bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm), clampFit(dr, -railLimit, railLimit), limited, nil
}

func clampFit(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

// fitBoundTolMm allows for round-off at hardware bounds and in saddle fit
// comparisons.
const fitBoundTolMm = 1e-9

// fitAxis is one fitted input: the world heading its Advance moves the target
// landmark along, its bounds relative to the probe value (hi may be +Inf), and
// the input reported when the fitted value sits on a bound.
type fitAxis struct {
	heading pose.Deg
	lo, hi  float64
	field   InputField
}

// fitSaddleBox solves delta ≈ da·dir(a.heading) + db·dir(b.heading) over finite
// bounds. Reachable targets return the exact solution, snapped within
// fitBoundTolMm, with nil limited. Otherwise the optimum lies on a box edge.
// It prefers points passing the BRP gate, then minimizes |ΔY| followed by |ΔX|
// (Q60, Q68, Q74). ok is false when the headings are parallel.
func fitSaddleBox(delta pose.Point, a, b fitAxis) (da, db float64, limited []InputField, ok bool) {
	da, db, ok = pose.DecomposeAlong(delta, a.heading, b.heading)
	if !ok {
		return 0, 0, nil, false
	}
	if da >= a.lo-fitBoundTolMm && da <= a.hi+fitBoundTolMm &&
		db >= b.lo-fitBoundTolMm && db <= b.hi+fitBoundTolMm {
		return clampFit(da, a.lo, a.hi), clampFit(db, b.lo, b.hi), nil, true
	}

	dirA, dirB := pose.Along(a.heading, 1), pose.Along(b.heading, 1)
	type candidate struct {
		da, db, absY, absX float64
		inGate             bool
	}
	candidates := make([]candidate, 0, 8)
	add := func(da, db float64) {
		miss := delta.Sub(pose.Point{X: da*dirA.X + db*dirB.X, Y: da*dirA.Y + db*dirB.Y})
		absY, absX := math.Abs(miss.Y), math.Abs(miss.X)
		inGate := absY <= FitGateBrpYMm+fitBoundTolMm && absX <= FitGateBrpXMm+fitBoundTolMm
		candidates = append(candidates, candidate{da, db, absY, absX, inGate})
	}
	addEdge := func(fixed float64, fixedDir pose.Point, moving fitAxis, dir pose.Point, fixedA bool) {
		base := delta.Sub(pose.Point{X: fixed * fixedDir.X, Y: fixed * fixedDir.Y})
		addAt := func(t float64) {
			if fixedA {
				add(fixed, t)
			} else {
				add(t, fixed)
			}
		}
		lo, hi := moving.lo, moving.hi
		addAt(bestOnSegment(lo, hi, base.X, base.Y, dir.X, dir.Y))
		gateLo, gateHi, ok := clipLinearBound(lo, hi, base.Y, dir.Y, FitGateBrpYMm)
		if !ok {
			return
		}
		gateLo, gateHi, ok = clipLinearBound(gateLo, gateHi, base.X, dir.X, FitGateBrpXMm)
		if ok {
			addAt(clampFit(bestOnSegment(gateLo, gateHi, base.X, base.Y, dir.X, dir.Y), lo, hi))
		}
	}
	addEdge(a.lo, dirA, b, dirB, true)
	addEdge(a.hi, dirA, b, dirB, true)
	addEdge(b.lo, dirB, a, dirA, false)
	addEdge(b.hi, dirB, a, dirA, false)

	best := candidates[0]
	for _, c := range candidates[1:] {
		switch {
		case c.inGate != best.inGate:
			if c.inGate {
				best = c
			}
		case math.Abs(c.absY-best.absY) > fitBoundTolMm:
			if c.absY < best.absY {
				best = c
			}
		case c.absX < best.absX-fitBoundTolMm:
			best = c
		}
	}
	if math.Abs(best.da-a.lo) <= fitBoundTolMm || math.Abs(best.da-a.hi) <= fitBoundTolMm {
		limited = append(limited, a.field)
	}
	if math.Abs(best.db-b.lo) <= fitBoundTolMm || math.Abs(best.db-b.hi) <= fitBoundTolMm {
		limited = append(limited, b.field)
	}
	return clampFit(best.da, a.lo, a.hi), clampFit(best.db, b.lo, b.hi), limited, true
}

// fitDirectionTol treats round-off in a horizontal or vertical direction as zero.
const fitDirectionTol = 1e-12

// clipLinearBound intersects [lo, hi] with the values of t where |c - t*s| <= limit.
func clipLinearBound(lo, hi, c, s, limit float64) (float64, float64, bool) {
	if math.Abs(s) < fitDirectionTol {
		if math.Abs(c) <= limit+fitBoundTolMm {
			return lo, hi, true
		}
		return 0, 0, false
	}
	t1, t2 := (c-limit)/s, (c+limit)/s
	newLo, newHi := max(lo, min(t1, t2)), min(hi, max(t1, t2))
	if newLo > newHi+fitBoundTolMm {
		return 0, 0, false
	}
	if newLo > newHi {
		snapped := clampFit(0.5*(newLo+newHi), lo, hi)
		return snapped, snapped, true
	}
	return newLo, newHi, true
}

// bestOnSegment minimizes |by - t*sy| first and then |bx - t*sx| over [lo, hi].
func bestOnSegment(lo, hi, bx, by, sx, sy float64) float64 {
	if math.Abs(sy) >= fitDirectionTol {
		return clampFit(by/sy, lo, hi)
	}
	if math.Abs(sx) >= fitDirectionTol {
		return clampFit(bx/sx, lo, hi)
	}
	return lo
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
