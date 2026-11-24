package assembly

import (
	"cmp"
	"fmt"
	"math"
	"sort"

	"bikegeo/internal/pose"
)

// ForkOption is one fork configuration build fitting may install: a fork at
// a sag with its lower headset cup (Q31, Q100).
type ForkOption struct {
	Fork     ForkSetup
	LowerCup LowerCupSetup
}

// PartOptions lists the discrete part options build fitting may choose from.
// Stems are the discrete catalog stems (Q97). Spacer stacks start at zero and
// advance in SpacerStepMm steps up to the setup's hardware limit. Handlebars, when
// non-empty, lists the bars build fitting may install. Each is installed at
// the submitted setup's BarRollDeg (Q24). Empty keeps the submitted bar (Q22).
// Forks, when non-empty, lists the fork configurations build fitting may install.
// Empty keeps the submitted fork and lower cup (Q33).
type PartOptions struct {
	Stems        []StemOption
	SpacerStepMm float64
	Handlebars   []HandlebarSetup
	Forks        []ForkOption
}

// Validate checks the stem options, spacer step, handlebars, and fork configurations.
func (p PartOptions) Validate() error {
	if !isFinite(p.SpacerStepMm) || p.SpacerStepMm <= 0 {
		return fmt.Errorf("spacer step must be positive and finite, got %v mm", p.SpacerStepMm)
	}
	if len(p.Stems) == 0 {
		return fmt.Errorf("build fitting needs at least one stem option")
	}
	type stemKey struct {
		length float64
		angle  pose.Deg
	}
	seen := make(map[stemKey]bool, len(p.Stems))
	for i, stem := range p.Stems {
		if err := stem.Validate(); err != nil {
			return fmt.Errorf("stem option %d: %w", i, err)
		}
		key := stemKey{stem.LengthMm, stem.AngleDeg}
		if seen[key] {
			return fmt.Errorf("stem option %d duplicates %v mm at %v°", i, stem.LengthMm, stem.AngleDeg)
		}
		seen[key] = true
	}
	for i, bar := range p.Handlebars {
		if bar.BarRollDeg != 0 {
			return fmt.Errorf("handlebar option %d must not set BarRollDeg; build fitting installs every bar at the submitted roll (Q24)", i)
		}
		if _, err := PlaceHandlebar(pose.NewPose[StemAxis](pose.Point[DynamicWorld]{}, 0), bar); err != nil {
			return fmt.Errorf("handlebar option %d: %w", i, err)
		}
	}
	for i, f := range p.Forks {
		if err := f.Fork.Validate(); err != nil {
			return fmt.Errorf("fork option %d: %w", i, err)
		}
		if err := f.LowerCup.Validate(); err != nil {
			return fmt.Errorf("fork option %d lower cup: %w", i, err)
		}
	}
	return nil
}

// ScoredBuild is one cockpit/fork build, placed forward and scored.
type ScoredBuild struct {
	Placement          BikePlacement
	Target             pose.Point[DynamicWorld]  // HandGrip target
	HasHandlebarOption bool                      // true when installed from PartOptions.Handlebars
	HandlebarIndex     int                       // index into PartOptions.Handlebars; meaningful only when HasHandlebarOption
	HasForkOption      bool                      // true when installed from PartOptions.Forks
	ForkIndex          int                       // index into PartOptions.Forks; meaningful only when HasForkOption
	BrpTarget          *pose.Point[DynamicWorld] // saddle BRP target; nil without one
	Score              BuildScore                // 4-tier fit gate, 0–100 weighted total, misses, and 12 attribute scores
}

// GripMiss returns target − placed HandGrip (world mm).
func (b ScoredBuild) GripMiss() pose.Vec[DynamicWorld] {
	return b.Target.Sub(b.Placement.Handlebar.HandGrip.Origin())
}

// BrpMiss returns BrpTarget − placed BRP (world mm), and false without a BRP target.
func (b ScoredBuild) BrpMiss() (pose.Vec[DynamicWorld], bool) {
	if b.BrpTarget == nil {
		return pose.Vec[DynamicWorld]{}, false
	}
	return b.BrpTarget.Sub(b.Placement.Saddle.BRP.Origin()), true
}

// CompareScoredBuilds orders builds best first: by CompareFitRank, then
// the Q69 deterministic cockpit tie-breaks (fewer spacers, shorter stem,
// smaller |angle|, + over − angle, handlebar order, fork order).
func CompareScoredBuilds(a, b ScoredBuild) int {
	return compareBuildRank(a.Score, a.Placement.Setup.Stem, a.HandlebarIndex, a.ForkIndex, &b)
}

func compareBuildRank(aScore BuildScore, aStem StemSetup, aBarIndex, aForkIndex int, b *ScoredBuild) int {
	if order := CompareFitRank(aScore, b.Score); order != 0 {
		return order
	}
	sb := b.Placement.Setup.Stem
	return cmp.Or(
		cmp.Compare(aStem.SpacersMm, sb.SpacersMm),
		cmp.Compare(aStem.LengthMm, sb.LengthMm),
		cmp.Compare(math.Abs(float64(aStem.AngleDeg)), math.Abs(float64(sb.AngleDeg))),
		cmp.Compare(float64(sb.AngleDeg), float64(aStem.AngleDeg)),
		cmp.Compare(aBarIndex, b.HandlebarIndex),
		cmp.Compare(aForkIndex, b.ForkIndex),
	)
}

// BuildRanking is the result of FitBuilds.
type BuildRanking struct {
	Builds    []ScoredBuild // best first; at most the requested limit
	Evaluated int           // combinations placed and scored
}

// FitBuilds places every combination of fork configuration, handlebar, stem,
// and spacer stack with PlaceBike, scores each with ScoreBuild, and returns
// the best limit builds by CompareScoredBuilds. Empty parts.Forks keeps the
// submitted fork and lower cup; empty parts.Handlebars keeps the submitted
// bar. Every bar is installed at setup.Handlebar.BarRollDeg (Q24). When
// target.BRP is set, ApplyFitting fits the saddle first for each fork
// configuration, and every build on that fork carries those saddle inputs
// (Q15). The continuous stem length range on setup.Stem is ignored (Q97).
func FitBuilds(setup BikeSetup, target FitTarget, parts PartOptions, weights ScoreWeights, limit int) (BuildRanking, error) {
	if target.HandGrip == nil {
		return BuildRanking{}, fmt.Errorf("build fitting needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return BuildRanking{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X, target.HandGrip.Y)
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return BuildRanking{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X, target.BRP.Y)
	}
	if limit < 1 {
		return BuildRanking{}, fmt.Errorf("build fitting needs a positive build limit, got %d", limit)
	}
	if err := parts.Validate(); err != nil {
		return BuildRanking{}, err
	}
	if err := weights.Validate(); err != nil {
		return BuildRanking{}, err
	}
	if weights.ActiveTotal(target.BRP != nil) <= 0 {
		return BuildRanking{}, fmt.Errorf("score weights have zero active weight for this target")
	}

	type forkChoice struct {
		fromOptions bool
		index       int
		fork        ForkSetup
		lowerCup    LowerCupSetup
	}
	forks := []forkChoice{{fork: setup.Fork, lowerCup: setup.LowerCup}}
	if len(parts.Forks) > 0 {
		forks = make([]forkChoice, len(parts.Forks))
		for i, f := range parts.Forks {
			forks[i] = forkChoice{fromOptions: true, index: i, fork: f.Fork, lowerCup: f.LowerCup}
		}
	}

	grip := *target.HandGrip
	var brp *pose.Point[DynamicWorld]
	if target.BRP != nil {
		p := *target.BRP
		brp = &p
	}
	type barChoice struct {
		fromOptions bool
		index       int
		setup       HandlebarSetup
		sweepW      float64
	}
	bars := []barChoice{{
		setup:  setup.Handlebar,
		sweepW: weights.HandlebarSweep * scoreHandlebarSweep(setup.Handlebar.BacksweepDeg),
	}}
	if len(parts.Handlebars) > 0 {
		bars = make([]barChoice, len(parts.Handlebars))
		for i, bar := range parts.Handlebars {
			bar.BarRollDeg = setup.Handlebar.BarRollDeg // Q24
			bars[i] = barChoice{
				fromOptions: true,
				index:       i,
				setup:       bar,
				sweepW:      weights.HandlebarSweep * scoreHandlebarSweep(bar.BacksweepDeg),
			}
		}
	}
	stemWeights := make([]float64, len(parts.Stems))
	for i, stem := range parts.Stems {
		stemWeights[i] = weights.Stem * stem.Score
	}
	oval := newGripOvalProjection(DefaultGripOval())

	var stacks []float64
	var spacerWeights []float64
	best := make([]ScoredBuild, 0, min(limit, maxPreallocatedBuilds))
	evaluated := 0
	buildTarget := FitTarget{HandGrip: &grip, BRP: brp}
	for _, fork := range forks {
		base := setup
		base.Fork, base.LowerCup = fork.fork, fork.lowerCup
		if brp != nil {
			saddle, err := ApplyFitting(base, FitTarget{BRP: brp})
			if err != nil {
				return BuildRanking{}, err
			}
			base = saddle.Placement.Setup
		}
		basePlacement, err := PlaceBike(base)
		if err != nil {
			return BuildRanking{}, err
		}
		baseScore, err := ScoreBuild(basePlacement, buildTarget, 0, weights)
		if err != nil {
			return BuildRanking{}, err
		}
		forkScore := newForkScoreBase(baseScore, weights)
		// Wait until PlaceBike has validated the stem's spacer allowance.
		if stacks == nil {
			stacks = spacerSteps(0, setup.Stem.MaxSpacersMm, parts.SpacerStepMm)
			spacerWeights = make([]float64, len(stacks))
			for i, spacers := range stacks {
				spacerWeights[i] = weights.Spacers * scoreSpacers(spacers)
			}
		}
		for si, spacers := range stacks {
			spacersW := spacerWeights[si]
			for ti, stem := range parts.Stems {
				stemW := stemWeights[ti]
				candidate := base
				candidate.Stem.AngleDeg = stem.AngleDeg
				candidate.Stem.LengthMm = stem.LengthMm
				candidate.Stem.SpacersMm = spacers
				stemPlacement, err := PlaceStem(basePlacement.Frameset.HeadTubeTop, candidate.Stem)
				if err != nil {
					return BuildRanking{}, err
				}
				for _, bar := range bars {
					handlebarPlacement, err := PlaceHandlebar(stemPlacement.BarClamp, bar.setup)
					if err != nil {
						return BuildRanking{}, err
					}
					gripR := oval.normalizedRadius(grip, handlebarPlacement.HandGrip.Origin())
					if !isFinite(gripR) {
						return BuildRanking{}, fmt.Errorf("build scoring metric is not finite")
					}
					evaluated++
					score := forkScore.score(gripR, stemW, bar.sweepW, spacersW)
					if len(best) == limit && compareBuildRank(score, candidate.Stem, bar.index, fork.index, &best[len(best)-1]) >= 0 {
						continue
					}
					candidate.Handlebar = bar.setup
					placement := BikePlacement{
						Setup:     candidate,
						Frameset:  basePlacement.Frameset,
						Seatpost:  basePlacement.Seatpost,
						Saddle:    basePlacement.Saddle,
						Stem:      stemPlacement,
						Handlebar: handlebarPlacement,
					}
					fullScore, err := ScoreBuild(placement, buildTarget, stem.Score, weights)
					if err != nil {
						return BuildRanking{}, err
					}
					build := ScoredBuild{
						Placement:          placement,
						Target:             grip,
						HasHandlebarOption: bar.fromOptions,
						HandlebarIndex:     bar.index,
						HasForkOption:      fork.fromOptions,
						ForkIndex:          fork.index,
						BrpTarget:          brp,
						Score:              fullScore,
					}
					best = keepBest(best, &build, limit)
				}
			}
		}
	}
	return BuildRanking{
		Builds:    best,
		Evaluated: evaluated,
	}, nil
}

// maxPreallocatedBuilds caps FitBuilds' up-front allocation for a large limit;
// the kept builds grow past it as needed.
const maxPreallocatedBuilds = 16

// keepBest adds build to best, which holds at most limit builds in
// CompareScoredBuilds order, dropping the worst build when best is full. A
// build that ties one already kept goes after it, so the first one visited
// stays ahead. FitBuilds keeps only these builds instead of sorting every
// combination it places.
func keepBest(best []ScoredBuild, build *ScoredBuild, limit int) []ScoredBuild {
	if len(best) == limit && CompareScoredBuilds(*build, best[len(best)-1]) >= 0 {
		return best
	}
	i := sort.Search(len(best), func(i int) bool { return CompareScoredBuilds(*build, best[i]) < 0 })
	if len(best) < limit {
		best = append(best, ScoredBuild{})
	}
	copy(best[i+1:], best[i:len(best)-1])
	best[i] = *build
	return best
}

func spacerSteps(lo, hi, step float64) []float64 {
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
