package assembly

import (
	"cmp"
	"fmt"
	"math"
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
// Stems are the discrete catalog stems (Q97). Spacer stacks start at zero and
// advance in SpacerStepMm steps up to the setup's hardware limit. Handlebars, when
// non-empty, lists the bars stock fitting may install. Each is installed at
// the submitted setup's BarRollDeg (Q24). Empty keeps the submitted bar (Q22).
// Forks, when non-empty, lists the fork configurations stock fitting may install.
// Empty keeps the submitted fork and lower cup (Q33).
type CockpitStock struct {
	Stems        []StockStem
	SpacerStepMm float64
	Handlebars   []HandlebarSetup
	Forks        []StockFork
}

// Validate checks the stock stems, spacer step, handlebars, and fork configurations.
func (s CockpitStock) Validate() error {
	if !isFinite(s.SpacerStepMm) || s.SpacerStepMm <= 0 {
		return fmt.Errorf("stock spacer step must be positive and finite, got %v mm", s.SpacerStepMm)
	}
	if len(s.Stems) == 0 {
		return fmt.Errorf("stock fitting needs at least one stem")
	}
	type stemKey struct {
		length float64
		angle  pose.Deg
	}
	seen := make(map[stemKey]bool, len(s.Stems))
	for i, stem := range s.Stems {
		if err := stem.Validate(); err != nil {
			return fmt.Errorf("stock stem %d: %w", i, err)
		}
		key := stemKey{stem.LengthMm, stem.AngleDeg}
		if seen[key] {
			return fmt.Errorf("stock stem %d duplicates %v mm at %v°", i, stem.LengthMm, stem.AngleDeg)
		}
		seen[key] = true
	}
	for i, bar := range s.Handlebars {
		if bar.BarRollDeg != 0 {
			return fmt.Errorf("stock handlebar %d must not set BarRollDeg; stock fitting installs every bar at the submitted roll (Q24)", i)
		}
		if _, err := PlaceHandlebar(pose.NewPose(pose.Point{}, 0), bar); err != nil {
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

// StockCandidate is one stock cockpit build, placed forward and scored.
type StockCandidate struct {
	Placement      BikePlacement
	Target         pose.Point  // HandGrip target
	StockHandlebar bool        // true when installed from CockpitStock.Handlebars
	HandlebarIndex int         // index into CockpitStock.Handlebars; meaningful only when StockHandlebar
	StockFork      bool        // true when installed from CockpitStock.Forks
	ForkIndex      int         // index into CockpitStock.Forks; meaningful only when StockFork
	BrpTarget      *pose.Point // saddle BRP target; nil without one
	Score          BuildScore  // 4-tier fit gate, 0–100 weighted total, misses, and 12 attribute scores
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

// CompareStockCandidates orders candidates best first: by CompareFitRank, then
// the Q69 deterministic cockpit tie-breaks (fewer spacers, shorter stem,
// smaller |angle|, + over − angle, handlebar order, fork order).
func CompareStockCandidates(a, b StockCandidate) int {
	if order := CompareFitRank(a.Score, b.Score); order != 0 {
		return order
	}
	sa, sb := a.Placement.Setup.Stem, b.Placement.Setup.Stem
	return cmp.Or(
		cmp.Compare(sa.SpacersMm, sb.SpacersMm),
		cmp.Compare(sa.LengthMm, sb.LengthMm),
		cmp.Compare(math.Abs(float64(sa.AngleDeg)), math.Abs(float64(sb.AngleDeg))),
		cmp.Compare(float64(sb.AngleDeg), float64(sa.AngleDeg)),
		cmp.Compare(a.HandlebarIndex, b.HandlebarIndex),
		cmp.Compare(a.ForkIndex, b.ForkIndex),
	)
}

// StockFitting is the result of ApplyStockFitting.
type StockFitting struct {
	Candidates []StockCandidate // best first; at most the requested limit
	Evaluated  int              // stock combinations placed and scored
}

// ApplyStockFitting places every stock combination of fork configuration,
// handlebar, stem, and spacer stack with PlaceBike, scores each with ScoreBuild,
// and returns the best limit candidates by CompareStockCandidates. Empty
// stock.Forks keeps the submitted fork and lower cup; empty stock.Handlebars
// keeps the submitted bar. Every bar is installed at setup.Handlebar.BarRollDeg
// (Q24). When target.BRP is set, ApplyFitting fits the saddle first for each
// fork configuration, and every candidate on that fork carries those saddle
// inputs (Q15). The continuous stem length range on setup.Stem is ignored (Q97).
func ApplyStockFitting(setup BikeSetup, target FitTarget, stock CockpitStock, weights ScoreWeights, limit int) (StockFitting, error) {
	if target.HandGrip == nil {
		return StockFitting{}, fmt.Errorf("stock fitting needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return StockFitting{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X, target.HandGrip.Y)
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return StockFitting{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X, target.BRP.Y)
	}
	if limit < 1 {
		return StockFitting{}, fmt.Errorf("stock fitting needs a positive candidate limit, got %d", limit)
	}
	if err := stock.Validate(); err != nil {
		return StockFitting{}, err
	}
	if err := weights.Validate(); err != nil {
		return StockFitting{}, err
	}
	if weights.ActiveTotal(target.BRP != nil) <= 0 {
		return StockFitting{}, fmt.Errorf("score weights have zero active weight for this target")
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
	var stacks []float64
	var candidates []StockCandidate
	for _, fork := range forks {
		base := setup
		base.Fork, base.LowerCup = fork.fork, fork.lowerCup
		if brp != nil {
			saddle, err := ApplyFitting(base, FitTarget{BRP: brp})
			if err != nil {
				return StockFitting{}, err
			}
			base = saddle.Placement.Setup
		}
		if _, err := PlaceBike(base); err != nil {
			return StockFitting{}, err
		}
		// Wait until PlaceBike has validated the stem's spacer allowance.
		if stacks == nil {
			stacks = stockSteps(0, setup.Stem.MaxSpacersMm, stock.SpacerStepMm)
		}
		candidateTarget := FitTarget{HandGrip: &grip, BRP: brp}
		for _, bar := range bars {
			for _, stem := range stock.Stems {
				for _, spacers := range stacks {
					candidate := base
					candidate.Stem.AngleDeg = stem.AngleDeg
					candidate.Stem.LengthMm = stem.LengthMm
					candidate.Stem.SpacersMm = spacers
					candidate.Handlebar = bar.setup
					placement, err := PlaceBike(candidate)
					if err != nil {
						return StockFitting{}, err
					}
					score, err := ScoreBuild(placement, candidateTarget, stem.Score, weights)
					if err != nil {
						return StockFitting{}, err
					}
					candidates = append(candidates, StockCandidate{
						Placement:      placement,
						Target:         grip,
						StockHandlebar: bar.stock,
						HandlebarIndex: bar.index,
						StockFork:      fork.stock,
						ForkIndex:      fork.index,
						BrpTarget:      brp,
						Score:          score,
					})
				}
			}
		}
	}
	slices.SortFunc(candidates, CompareStockCandidates)
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
