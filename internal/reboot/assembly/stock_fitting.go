package assembly

import (
	"cmp"
	"fmt"
	"slices"

	"bikegeo/internal/reboot/pose"
)

// StockFork is one fork configuration stock fitting may install: a fork at a
// sag with its lower headset cup (Q31).
type StockFork struct {
	Fork     ForkSetup
	LowerCup LowerCupSetup
}

// CockpitStock lists the stock cockpit parts stock fitting may choose from.
// Stem lengths start at Stem.MinLengthMm, and spacer stacks start at zero.
// Both advance in fixed steps up to their hardware limits. Handlebars, when
// non-empty, lists the bars stock fitting may install. Each is installed at
// the submitted setup's BarRollDeg (Q24). Empty keeps the submitted bar (Q22).
// Forks, when non-empty, lists the fork configurations stock fitting may install.
// Empty keeps the submitted fork and lower cup (Q33).
type CockpitStock struct {
	StemLengthStepMm float64
	StemAnglesDeg    []pose.Deg
	SpacerStepMm     float64
	Handlebars       []HandlebarSetup
	Forks            []StockFork
}

// Validate checks the stock steps, stem angles, handlebars, and fork configurations.
func (s CockpitStock) Validate() error {
	if !isFinite(s.StemLengthStepMm) || s.StemLengthStepMm <= 0 {
		return fmt.Errorf("stock stem length step must be positive and finite, got %v mm", s.StemLengthStepMm)
	}
	if !isFinite(s.SpacerStepMm) || s.SpacerStepMm <= 0 {
		return fmt.Errorf("stock spacer step must be positive and finite, got %v mm", s.SpacerStepMm)
	}
	if len(s.StemAnglesDeg) == 0 {
		return fmt.Errorf("stock fitting needs at least one stem angle")
	}
	for _, angle := range s.StemAnglesDeg {
		if !isFinite(float64(angle)) {
			return fmt.Errorf("stock stem angle must be finite, got %v°", angle)
		}
	}
	for i, bar := range s.Handlebars {
		if bar.BarRollDeg != 0 {
			return fmt.Errorf("stock handlebar %d must not set BarRollDeg; stock fitting installs every bar at the submitted roll (Q24)", i)
		}
		if _, err := PlaceHandlebar(pose.NewPose(pose.Point{}, 0, nil), bar); err != nil {
			return fmt.Errorf("stock handlebar %d: %w", i, err)
		}
	}
	for i, f := range s.Forks {
		if err := f.Fork.Validate(); err != nil {
			return fmt.Errorf("stock fork %d: %w", i, err)
		}
		if err := f.LowerCup.Validate(); err != nil {
			return fmt.Errorf("stock fork %d lower cup: %w", i, err)
		}
	}
	return nil
}

// PenaltyBand is free inside [FreeMin, FreeMax] and penalizes linearly outside it:
// PointsPerUnitBelow points per unit below FreeMin, PointsPerUnitAbove per unit above FreeMax.
type PenaltyBand struct {
	FreeMin, FreeMax                       float64
	PointsPerUnitBelow, PointsPerUnitAbove float64
}

// PointsLost returns the penalty of v under the band.
func (b PenaltyBand) PointsLost(v float64) float64 {
	if v < b.FreeMin {
		return b.PointsPerUnitBelow * (b.FreeMin - v)
	}
	if v > b.FreeMax {
		return b.PointsPerUnitAbove * (v - b.FreeMax)
	}
	return 0
}

func (b PenaltyBand) validate(name string) error {
	if !isFinite(b.FreeMin) || !isFinite(b.FreeMax) || b.FreeMin > b.FreeMax {
		return fmt.Errorf("%s band: free range must be finite with min <= max, got [%v, %v]", name, b.FreeMin, b.FreeMax)
	}
	if !isFinite(b.PointsPerUnitBelow) || !isFinite(b.PointsPerUnitAbove) || b.PointsPerUnitBelow < 0 || b.PointsPerUnitAbove < 0 {
		return fmt.Errorf("%s band: points per unit must be finite and non-negative, got %v below and %v above", name, b.PointsPerUnitBelow, b.PointsPerUnitAbove)
	}
	return nil
}

// FitOval is the legacy Area of Reasonable Fit around a HandGrip target (Q17).
// Its major axis follows MajorAxisHeadingDeg in world coordinates. It is free
// inside (R ≤ 1) and penalizes PointsPerR points per unit of R beyond the edge.
type FitOval struct {
	SemiMinorMm, SemiMajorMm float64
	MajorAxisHeadingDeg      pose.Deg
	PointsPerR               float64
}

// NormalizedRadius returns R for p around center (R = 1 on the oval's edge).
func (o FitOval) NormalizedRadius(center, p pose.Point) float64 {
	d := p.Sub(center)
	minor := pose.ProjectOnto(d, o.MajorAxisHeadingDeg-90) / o.SemiMinorMm
	major := pose.ProjectOnto(d, o.MajorAxisHeadingDeg) / o.SemiMajorMm
	return pose.Point{X: minor, Y: major}.Norm()
}

// PointsLost returns PointsPerR · max(0, r − 1).
func (o FitOval) PointsLost(r float64) float64 {
	return o.PointsPerR * max(0, r-1)
}

// CockpitPreferences holds the bands that score a stock cockpit build (score = 100 − points lost).
type CockpitPreferences struct {
	GripOval   FitOval
	StemLength PenaltyBand // mm
	StemAngle  PenaltyBand // degrees
	Spacers    PenaltyBand // mm
	BarSweep   PenaltyBand // degrees of backsweep (Q30)
	ForkSag    PenaltyBand // percent sag; penalizes suspension forks only (Q32)
	BrpMiss    PenaltyBand // mm of remaining saddle BRP miss, for any cause; counts whenever there is a BRP target (Q32, Q34)
}

// Validate checks the oval and each penalty band.
func (p CockpitPreferences) Validate() error {
	o := p.GripOval
	if !isFinite(o.SemiMinorMm) || !isFinite(o.SemiMajorMm) || o.SemiMinorMm <= 0 || o.SemiMajorMm <= 0 {
		return fmt.Errorf("HandGrip oval: semi-axes must be positive and finite, got %v and %v mm", o.SemiMinorMm, o.SemiMajorMm)
	}
	if !isFinite(float64(o.MajorAxisHeadingDeg)) {
		return fmt.Errorf("HandGrip oval: major axis heading must be finite, got %v°", o.MajorAxisHeadingDeg)
	}
	if !isFinite(o.PointsPerR) || o.PointsPerR < 0 {
		return fmt.Errorf("HandGrip oval: points per R must be finite and non-negative, got %v", o.PointsPerR)
	}
	for _, entry := range []struct {
		name string
		band PenaltyBand
	}{
		{string(BandStemLength), p.StemLength},
		{string(BandStemAngle), p.StemAngle},
		{string(BandSpacers), p.Spacers},
		{string(BandBarSweep), p.BarSweep},
		{string(BandForkSag), p.ForkSag},
		{string(BandBrpMiss), p.BrpMiss},
	} {
		if err := entry.band.validate(entry.name); err != nil {
			return err
		}
	}
	return nil
}

// BandName names one penalty line of a stock cockpit build.
type BandName string

const (
	BandGripOval   BandName = "HandGrip oval"
	BandStemLength BandName = "stem length"
	BandStemAngle  BandName = "stem angle"
	BandSpacers    BandName = "spacers"
	BandBarSweep   BandName = "bar sweep"
	BandForkSag    BandName = "fork sag"
	BandBrpMiss    BandName = "BRP miss"
)

// Penalty is the points one band takes off a build's score.
type Penalty struct {
	Band       BandName
	PointsLost float64
}

// StockCandidate is one stock cockpit build, placed forward and scored.
type StockCandidate struct {
	Placement      BikePlacement
	Target         pose.Point  // HandGrip target
	StockHandlebar bool        // the bar came from CockpitStock.Handlebars; false when the submitted bar was kept
	HandlebarIndex int         // index into CockpitStock.Handlebars of the installed bar; meaningful only when StockHandlebar
	StockFork      bool        // the fork came from CockpitStock.Forks; false when the submitted fork was kept
	ForkIndex      int         // index into CockpitStock.Forks; meaningful only when StockFork
	BrpTarget      *pose.Point // saddle BRP target; nil without one
	GripR          float64     // normalized radius of the placed HandGrip
	Penalties      []Penalty   // oval, stem length, stem angle, spacers, bar sweep, fork sag, BRP miss
}

// PointsLost returns the sum of the penalties' points lost, in penalty order.
func (c StockCandidate) PointsLost() float64 {
	var lost float64
	for _, p := range c.Penalties {
		lost += p.PointsLost
	}
	return lost
}

// Score returns 100 − PointsLost().
func (c StockCandidate) Score() float64 {
	return 100 - c.PointsLost()
}

// GripMiss returns target − placed HandGrip (world mm).
func (c StockCandidate) GripMiss() pose.Point {
	return c.Target.Sub(c.Placement.Handlebar.HandGrip.Origin())
}

// BrpMiss returns BrpTarget − placed BRP (world mm), and false without a BRP target.
func (c StockCandidate) BrpMiss() (pose.Point, bool) {
	if c.BrpTarget == nil {
		return pose.Point{}, false
	}
	return c.BrpTarget.Sub(c.Placement.Saddle.BRP.Origin()), true
}

// StockFitting is the result of ApplyStockFitting.
type StockFitting struct {
	Candidates []StockCandidate // best-scoring first; at most the requested limit
	Evaluated  int              // stock combinations placed and scored
}

// ApplyStockFitting places every stock combination of fork configuration,
// handlebar, stem angle, stem length, and spacer stack with PlaceBike and
// returns the best-scoring limit candidates. Empty stock.Forks keeps the submitted
// fork and lower cup; empty stock.Handlebars keeps the submitted bar. Every bar
// is installed at setup.Handlebar.BarRollDeg (Q24). When target.BRP is set,
// ApplyFitting fits the saddle first for each fork configuration, and every
// candidate on that fork carries those saddle inputs (Q15). Ties favor smaller
// oval R, then enumeration order (forks, bars, angles, lengths, spacers).
func ApplyStockFitting(setup BikeSetup, target FitTarget, stock CockpitStock, prefs CockpitPreferences, limit int) (StockFitting, error) {
	if target.HandGrip == nil {
		return StockFitting{}, fmt.Errorf("stock fitting needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return StockFitting{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X, target.HandGrip.Y)
	}
	if limit < 1 {
		return StockFitting{}, fmt.Errorf("stock fitting needs a positive candidate limit, got %d", limit)
	}
	if err := stock.Validate(); err != nil {
		return StockFitting{}, err
	}
	if err := prefs.Validate(); err != nil {
		return StockFitting{}, err
	}
	if err := validateStemLengthRange(setup.Stem); err != nil {
		return StockFitting{}, err
	}

	type stockForkChoice struct {
		stock    bool
		index    int
		fork     ForkSetup
		lowerCup LowerCupSetup
	}
	forks := []stockForkChoice{{fork: setup.Fork, lowerCup: setup.LowerCup}}
	if len(stock.Forks) > 0 {
		forks = make([]stockForkChoice, len(stock.Forks))
		for i, f := range stock.Forks {
			forks[i] = stockForkChoice{stock: true, index: i, fork: f.Fork, lowerCup: f.LowerCup}
		}
	}

	grip := *target.HandGrip
	var brp *pose.Point
	if target.BRP != nil {
		p := *target.BRP
		brp = &p
	}
	type stockBar struct {
		stock bool
		index int
		setup HandlebarSetup
	}
	bars := []stockBar{{setup: setup.Handlebar}}
	if len(stock.Handlebars) > 0 {
		bars = make([]stockBar, len(stock.Handlebars))
		for i, bar := range stock.Handlebars {
			bar.BarRollDeg = setup.Handlebar.BarRollDeg // Q24
			bars[i] = stockBar{stock: true, index: i, setup: bar}
		}
	}
	var lengths, stacks []float64
	var candidates []StockCandidate
	for _, fork := range forks {
		base := setup
		base.Fork, base.LowerCup = fork.fork, fork.lowerCup
		if brp != nil {
			saddle, err := ApplyFitting(base, FitTarget{BRP: brp}, nil)
			if err != nil {
				return StockFitting{}, err
			}
			base = saddle.Placement.Setup
		}
		if _, err := PlaceBike(base, nil); err != nil {
			return StockFitting{}, err
		}
		// Wait until PlaceBike has validated the stem's spacer allowance.
		if lengths == nil {
			lengths = stockSteps(setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm, stock.StemLengthStepMm)
			stacks = stockSteps(0, setup.Stem.MaxSpacersMm, stock.SpacerStepMm)
		}
		for _, bar := range bars {
			for _, angle := range stock.StemAnglesDeg {
				for _, length := range lengths {
					for _, spacers := range stacks {
						candidate := base
						candidate.Stem.AngleDeg = angle
						candidate.Stem.LengthMm = length
						candidate.Stem.SpacersMm = spacers
						candidate.Handlebar = bar.setup
						placement, err := PlaceBike(candidate, nil)
						if err != nil {
							return StockFitting{}, err
						}
						r := prefs.GripOval.NormalizedRadius(grip, placement.Handlebar.HandGrip.Origin())
						result := StockCandidate{
							Placement:      placement,
							Target:         grip,
							StockHandlebar: bar.stock,
							HandlebarIndex: bar.index,
							StockFork:      fork.stock,
							ForkIndex:      fork.index,
							BrpTarget:      brp,
							GripR:          r,
						}
						result.Penalties = []Penalty{
							{Band: BandGripOval, PointsLost: prefs.GripOval.PointsLost(r)},
							{Band: BandStemLength, PointsLost: prefs.StemLength.PointsLost(length)},
							{Band: BandStemAngle, PointsLost: prefs.StemAngle.PointsLost(float64(angle))},
							{Band: BandSpacers, PointsLost: prefs.Spacers.PointsLost(spacers)},
							{Band: BandBarSweep, PointsLost: prefs.BarSweep.PointsLost(float64(bar.setup.BacksweepDeg))},
							{Band: BandForkSag, PointsLost: forkSagPoints(prefs.ForkSag, candidate.Fork)},
							{Band: BandBrpMiss, PointsLost: brpMissPoints(prefs.BrpMiss, result)},
						}
						candidates = append(candidates, result)
					}
				}
			}
		}
	}
	slices.SortStableFunc(candidates, func(a, b StockCandidate) int {
		if order := cmp.Compare(a.PointsLost(), b.PointsLost()); order != 0 {
			return order
		}
		return cmp.Compare(a.GripR, b.GripR)
	})
	return StockFitting{
		Candidates: candidates[:min(limit, len(candidates))],
		Evaluated:  len(candidates),
	}, nil
}

// forkSagPoints penalizes suspension sag. A rigid fork has no sag penalty (Q32).
func forkSagPoints(band PenaltyBand, fork ForkSetup) float64 {
	if fork.Spec.TravelMm == 0 {
		return 0
	}
	return band.PointsLost(fork.SagPct)
}

// brpMissPoints penalizes the remaining saddle BRP miss when there is a target (Q32, Q34).
func brpMissPoints(band PenaltyBand, candidate StockCandidate) float64 {
	miss, ok := candidate.BrpMiss()
	if !ok {
		return 0
	}
	return band.PointsLost(miss.Norm())
}

func stockSteps(lo, hi, step float64) []float64 {
	if lo == hi {
		return []float64{lo}
	}
	var values []float64
	for i := 0; ; i++ {
		v := lo + float64(i)*step
		if v > hi+1e-9 {
			break
		}
		values = append(values, min(v, hi))
	}
	return values
}
