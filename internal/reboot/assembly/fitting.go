package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// SaddleFitTarget is the BRP target point for saddle fitting, in world
// coordinates (BB at origin, +X forward, +Y up, mm).
type SaddleFitTarget struct {
	BRP pose.Point
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
	Target    SaddleFitTarget
	Placement BikePlacement // PlaceBike(fitted setup); Placement.Setup is the fitted setup
	Outcome   FitOutcome
	LimitedBy []InputField // InputSeatpostExtension and/or InputSaddleRailOffset; nil when reached
}

// BrpMiss returns target − placed BRP (world mm).
func (f Fitting) BrpMiss() pose.Point {
	return f.Target.BRP.Sub(f.Placement.Saddle.BRP.Origin())
}

// BrpMissMm returns |BrpMiss()|.
func (f Fitting) BrpMissMm() float64 {
	return f.BrpMiss().Norm()
}

// ApplyFitting fits Seatpost.ExtensionMm and Saddle.RailOffsetMm so the saddle BRP
// lands on target, within hardware limits. All other setup fields are kept.
// trace, if non-nil, records only the final PlaceBike of the fitted setup and is
// frozen on return.
func ApplyFitting(setup BikeSetup, target SaddleFitTarget, trace *pose.Trace) (Fitting, error) {
	defer trace.Freeze()
	if !isFinitePoint(target.BRP) {
		return Fitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X, target.BRP.Y)
	}
	railLimit, err := setup.RailOffsetLimitMm()
	if err != nil {
		return Fitting{}, err
	}
	eMin := setup.Seatpost.MinExtensionMm
	eMax := math.Inf(1)
	if setup.Seatpost.MaxExtensionMm != 0 {
		eMax = setup.Seatpost.MaxExtensionMm
	}

	// BRP is linear in (extension, rail offset): place once at a probe and
	// decompose the remaining offset along the two fixed directions.
	probeSetup := setup
	probeSetup.Seatpost.ExtensionMm = eMin
	probeSetup.Saddle.RailOffsetMm = 0
	probe, err := PlaceBike(probeSetup, nil)
	if err != nil {
		return Fitting{}, err
	}
	delta := target.BRP.Sub(probe.Saddle.BRP.Origin())
	seat := probe.Frameset.SeatTubeTop.HeadingDeg()
	rail := setup.Saddle.RailPitchDeg + 180
	de, dr, limited, err := fitBox(delta, seat, rail, eMax-eMin, railLimit)
	if err != nil {
		return Fitting{}, err
	}

	fitted := setup
	fitted.Seatpost.ExtensionMm = math.Min(math.Max(eMin+de, eMin), eMax)
	fitted.Saddle.RailOffsetMm = math.Min(math.Max(dr, -railLimit), railLimit)
	placement, err := PlaceBike(fitted, trace)
	if err != nil {
		return Fitting{}, err
	}

	fit := Fitting{Target: target, Placement: placement, Outcome: FitReached}
	if len(limited) > 0 {
		fit.Outcome = FitLimited
		fit.LimitedBy = limited
	}
	if fit.Outcome == FitReached {
		if miss := fit.BrpMissMm(); miss > 1e-6 {
			return Fitting{}, fmt.Errorf("saddle fitting forward check missed target by %v mm", miss)
		}
	}
	return fit, nil
}

// fitBox solves delta ≈ de·dir(seat) + dr·dir(rail) with de in [0, extRange]
// (extRange may be +Inf) and dr in [−railLimit, railLimit]. When the
// unconstrained solution is outside the box, the box minimum lies on an edge;
// the four edges are evaluated in a fixed order and the closest wins, with
// ties keeping the earlier edge. limited lists the extension then rail offset
// when the chosen value sits on a bound; it is nil when delta is reachable.
func fitBox(delta pose.Point, seat, rail pose.Deg, extRange, railLimit float64) (de, dr float64, limited []InputField, err error) {
	de, dr, ok := pose.DecomposeAlong(delta, seat, rail)
	if !ok {
		return 0, 0, nil, fmt.Errorf("saddle rail pitch is parallel to the seat axis; cannot fit the saddle")
	}
	if de >= 0 && de <= extRange && dr >= -railLimit && dr <= railLimit {
		return de, dr, nil, nil
	}

	clamp := func(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }
	miss := func(e, r float64) float64 {
		return delta.Sub(pose.Along(seat, e)).Sub(pose.Along(rail, r)).Norm()
	}
	// Fix extension at e; the best rail offset is the clamped projection.
	fixExt := func(e float64) (float64, float64) {
		r := clamp(pose.ProjectOnto(delta.Sub(pose.Along(seat, e)), rail), -railLimit, railLimit)
		return e, r
	}
	// Fix rail offset at r; the best extension is the clamped projection.
	fixRail := func(r float64) (float64, float64) {
		e := clamp(pose.ProjectOnto(delta.Sub(pose.Along(rail, r)), seat), 0, extRange)
		return e, r
	}

	type candidate struct{ e, r, miss float64 }
	var cands []candidate
	add := func(e, r float64) { cands = append(cands, candidate{e, r, miss(e, r)}) }
	add(fixExt(0))
	if !math.IsInf(extRange, 1) {
		add(fixExt(extRange))
	}
	add(fixRail(-railLimit))
	add(fixRail(railLimit))

	best := cands[0]
	for _, c := range cands[1:] {
		if c.miss < best.miss-1e-9 {
			best = c
		}
	}
	if best.e == 0 || best.e == extRange {
		limited = append(limited, InputSeatpostExtension)
	}
	if best.r == -railLimit || best.r == railLimit {
		limited = append(limited, InputSaddleRailOffset)
	}
	return best.e, best.r, limited, nil
}
