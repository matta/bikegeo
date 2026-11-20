package assembly

import (
	"cmp"
	"fmt"
	"slices"

	"bikegeo/internal/reboot/pose"
)

// CockpitStock lists the stock cockpit parts stock fitting may choose from.
// Stem lengths start at Stem.MinLengthMm, and spacer stacks start at zero.
// Both advance in fixed steps up to their hardware limits.
type CockpitStock struct {
	StemLengthStepMm float64
	StemAnglesDeg    []pose.Deg
	SpacerStepMm     float64
}

// Validate checks that steps are positive and finite and the angle list is usable.
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
	return nil
}

// PricedBand is free inside [FreeMin, FreeMax] and charges linearly outside it.
type PricedBand struct {
	FreeMin, FreeMax     float64
	CostBelow, CostAbove float64
}

// PointsLost returns the price of v under the band.
func (b PricedBand) PointsLost(v float64) float64 {
	if v < b.FreeMin {
		return b.CostBelow * (b.FreeMin - v)
	}
	if v > b.FreeMax {
		return b.CostAbove * (v - b.FreeMax)
	}
	return 0
}

func (b PricedBand) validate(name string) error {
	if !isFinite(b.FreeMin) || !isFinite(b.FreeMax) || b.FreeMin > b.FreeMax {
		return fmt.Errorf("%s band: free range must be finite with min <= max, got [%v, %v]", name, b.FreeMin, b.FreeMax)
	}
	if !isFinite(b.CostBelow) || !isFinite(b.CostAbove) || b.CostBelow < 0 || b.CostAbove < 0 {
		return fmt.Errorf("%s band: costs must be finite and non-negative, got %v below and %v above", name, b.CostBelow, b.CostAbove)
	}
	return nil
}

// FitOval is the legacy Area of Reasonable Fit around a HandGrip target (Q17).
// Its major axis follows MajorAxisHeadingDeg in world coordinates. It is free
// inside (R ≤ 1) and charges CostPerR per unit of R beyond the edge.
type FitOval struct {
	SemiMinorMm, SemiMajorMm float64
	MajorAxisHeadingDeg      pose.Deg
	CostPerR                 float64
}

// NormalizedRadius returns R for p around center (R = 1 on the oval's edge).
func (o FitOval) NormalizedRadius(center, p pose.Point) float64 {
	d := p.Sub(center)
	minor := pose.ProjectOnto(d, o.MajorAxisHeadingDeg-90) / o.SemiMinorMm
	major := pose.ProjectOnto(d, o.MajorAxisHeadingDeg) / o.SemiMajorMm
	return pose.Point{X: minor, Y: major}.Norm()
}

// PointsLost returns CostPerR · max(0, r − 1).
func (o FitOval) PointsLost(r float64) float64 {
	return o.CostPerR * max(0, r-1)
}

// CockpitPreferences prices a stock cockpit build on the points-lost ledger.
type CockpitPreferences struct {
	GripOval   FitOval
	StemLength PricedBand // mm
	StemAngle  PricedBand // degrees
	Spacers    PricedBand // mm
}

// Validate checks the oval and each priced band.
func (p CockpitPreferences) Validate() error {
	o := p.GripOval
	if !isFinite(o.SemiMinorMm) || !isFinite(o.SemiMajorMm) || o.SemiMinorMm <= 0 || o.SemiMajorMm <= 0 {
		return fmt.Errorf("HandGrip oval: semi-axes must be positive and finite, got %v and %v mm", o.SemiMinorMm, o.SemiMajorMm)
	}
	if !isFinite(float64(o.MajorAxisHeadingDeg)) {
		return fmt.Errorf("HandGrip oval: major axis heading must be finite, got %v°", o.MajorAxisHeadingDeg)
	}
	if !isFinite(o.CostPerR) || o.CostPerR < 0 {
		return fmt.Errorf("HandGrip oval: cost must be finite and non-negative, got %v", o.CostPerR)
	}
	for _, entry := range []struct {
		name string
		band PricedBand
	}{
		{string(BandStemLength), p.StemLength},
		{string(BandStemAngle), p.StemAngle},
		{string(BandSpacers), p.Spacers},
	} {
		if err := entry.band.validate(entry.name); err != nil {
			return err
		}
	}
	return nil
}

// LedgerBand names one priced line of a stock cockpit ledger.
type LedgerBand string

const (
	BandGripOval   LedgerBand = "HandGrip oval"
	BandStemLength LedgerBand = "stem length"
	BandStemAngle  LedgerBand = "stem angle"
	BandSpacers    LedgerBand = "spacers"
)

// LedgerLine is the points one band loses. The priced values themselves are
// the candidate's GripR and its placed stem setup.
type LedgerLine struct {
	Band       LedgerBand
	PointsLost float64
}

// StockCandidate is one stock cockpit build, placed forward and priced.
type StockCandidate struct {
	Placement BikePlacement
	Target    pose.Point   // HandGrip target
	GripR     float64      // normalized radius of the placed HandGrip
	Ledger    []LedgerLine // oval, stem length, stem angle, spacers
}

// PointsLost returns the sum of the ledger's points lost, in ledger order.
func (c StockCandidate) PointsLost() float64 {
	var total float64
	for _, line := range c.Ledger {
		total += line.PointsLost
	}
	return total
}

// Total returns 100 − PointsLost().
func (c StockCandidate) Total() float64 {
	return 100 - c.PointsLost()
}

// GripMiss returns target − placed HandGrip (world mm).
func (c StockCandidate) GripMiss() pose.Point {
	return c.Target.Sub(c.Placement.Handlebar.HandGrip.Origin())
}

// StockFitting is the result of ApplyStockFitting.
type StockFitting struct {
	Candidates []StockCandidate // cheapest first; at most the requested limit
	Evaluated  int              // stock combinations placed and priced
}

// ApplyStockFitting places every stock stem and spacer combination with
// PlaceBike and returns the cheapest limit candidates. Ties favor smaller
// oval R, then enumeration order. When target.BRP is set, ApplyFitting fits
// the saddle first and every candidate carries those saddle inputs (Q15).
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

	base := setup
	if target.BRP != nil {
		saddle, err := ApplyFitting(setup, FitTarget{BRP: target.BRP}, nil)
		if err != nil {
			return StockFitting{}, err
		}
		base = saddle.Placement.Setup
	}
	if _, err := PlaceBike(base, nil); err != nil {
		return StockFitting{}, err
	}

	grip := *target.HandGrip
	lengths := stockSteps(base.Stem.MinLengthMm, base.Stem.MaxLengthMm, stock.StemLengthStepMm)
	stacks := stockSteps(0, base.Stem.MaxSpacersMm, stock.SpacerStepMm)
	var candidates []StockCandidate
	for _, angle := range stock.StemAnglesDeg {
		for _, length := range lengths {
			for _, spacers := range stacks {
				candidate := base
				candidate.Stem.AngleDeg = angle
				candidate.Stem.LengthMm = length
				candidate.Stem.SpacersMm = spacers
				placement, err := PlaceBike(candidate, nil)
				if err != nil {
					return StockFitting{}, err
				}
				r := prefs.GripOval.NormalizedRadius(grip, placement.Handlebar.HandGrip.Origin())
				candidates = append(candidates, StockCandidate{
					Placement: placement,
					Target:    grip,
					GripR:     r,
					Ledger: []LedgerLine{
						{Band: BandGripOval, PointsLost: prefs.GripOval.PointsLost(r)},
						{Band: BandStemLength, PointsLost: prefs.StemLength.PointsLost(length)},
						{Band: BandStemAngle, PointsLost: prefs.StemAngle.PointsLost(float64(angle))},
						{Band: BandSpacers, PointsLost: prefs.Spacers.PointsLost(spacers)},
					},
				})
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
