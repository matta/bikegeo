package assembly

import (
	"cmp"
	"fmt"
	"math"
	"sort"

	"github.com/matta/bikegeo/internal/pose"
)

// ForkOption is one fork configuration build fitting may install: a fork at
// a sag with its lower headset cup.
type ForkOption struct {
	Fork     ForkSetup
	LowerCup LowerCupSetup
}

// PartOptions lists the discrete part options build fitting may choose from.
// Stems are the discrete catalog stems. Spacer stacks start at zero and
// advance in SpacerStepMm steps up to the setup's hardware limit. Handlebars,
// when non-empty, lists the bars build fitting may install; each is installed
// at the submitted setup's BarRollDeg. Empty keeps the submitted bar. Forks,
// when non-empty, lists the fork configurations build fitting may install;
// empty keeps the submitted fork and lower cup.
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
			return fmt.Errorf("handlebar option %d must not set BarRollDeg; build fitting installs every bar at the submitted roll", i)
		}
		if _, err := NewHandlebar(HandlebarDimensions{
			StockWidthMm:   bar.StockWidthMm,
			CurrentWidthMm: bar.CurrentWidthMm,
			BacksweepDeg:   bar.BacksweepDeg,
			UpsweepDeg:     bar.UpsweepDeg,
			StatedRiseMm:   bar.StatedRiseMm,
			RiseMode:       bar.RiseMode,
			BendStartMm:    bar.BendStartMm,
			GripInsetMm:    bar.GripInsetMm,
		}); err != nil {
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
	Placement          Placement
	Target             pose.Point  // HandGrip target
	HasHandlebarOption bool        // true when installed from PartOptions.Handlebars
	HandlebarIndex     int         // index into PartOptions.Handlebars; meaningful only when HasHandlebarOption
	HasForkOption      bool        // true when installed from PartOptions.Forks
	ForkIndex          int         // index into PartOptions.Forks; meaningful only when HasForkOption
	BrpTarget          *pose.Point // saddle BRP target; nil without one
	Score              BuildScore  // 4-tier fit gate, 0–100 weighted total, misses, and 12 attribute scores
}

// GripMiss returns target − placed HandGrip (world mm).
func (b ScoredBuild) GripMiss() pose.Vector {
	return b.Target.Sub(b.Placement.Handlebar.HandGrip.Origin())
}

// BrpMiss returns BrpTarget − placed BRP (world mm), and false without a BRP target.
func (b ScoredBuild) BrpMiss() (pose.Vector, bool) {
	if b.BrpTarget == nil {
		return pose.Vector{}, false
	}
	return b.BrpTarget.Sub(b.Placement.Saddle.BRP.Origin()), true
}

// CompareScoredBuilds orders builds best first: by CompareFitRank, then
// deterministic cockpit tie-breaks (fewer spacers, shorter stem, smaller
// |angle|, + over − angle, handlebar order, fork order).
func CompareScoredBuilds(a, b ScoredBuild) int {
	sa := a.Placement.Setup.Stem
	return compareBuildRank(a.Score, sa.SpacersMm, sa.LengthMm, sa.AngleDeg, a.HandlebarIndex, a.ForkIndex, &b)
}

func compareBuildRank(aScore BuildScore, spacersMm, lengthMm float64, angleDeg pose.Deg, aBarIndex, aForkIndex int, b *ScoredBuild) int {
	if order := CompareFitRank(aScore, b.Score); order != 0 {
		return order
	}
	sb := b.Placement.Setup.Stem
	return cmp.Or(
		cmp.Compare(spacersMm, sb.SpacersMm),
		cmp.Compare(lengthMm, sb.LengthMm),
		cmp.Compare(math.Abs(float64(angleDeg)), math.Abs(float64(sb.AngleDeg))),
		cmp.Compare(float64(sb.AngleDeg), float64(angleDeg)),
		cmp.Compare(aBarIndex, b.HandlebarIndex),
		cmp.Compare(aForkIndex, b.ForkIndex),
	)
}

// BuildRanking is the result of FitBuilds or FitFrontBuilds.
type BuildRanking struct {
	Builds    []ScoredBuild // best first; at most the requested limit
	Evaluated int           // combinations placed and scored
}

// FrontBuildOptions lists the discrete front-cockpit options evaluated on an
// already-seated *SeatedFrameset by FitFrontBuilds.
type FrontBuildOptions struct {
	StemClampHeightMm float64
	Stems             []StemOption
	SpacerStepMm      float64
	Handlebars        []Handlebar
	HasHandlebarOpts  bool // marks ScoredBuild.HasHandlebarOption when true
	DynamicBarRollDeg pose.Deg
}

// preparedStem pairs a validated Stem part with its catalog score and weighted
// score term so inner front-cockpit loops avoid re-validating stems.
type preparedStem struct {
	opt    StemOption
	part   Stem
	weight float64
}

// preparedBar pairs a validated Handlebar part with its backsweep score and
// option tracking metadata.
type preparedBar struct {
	fromOptions bool
	index       int
	part        Handlebar
	sweepScore  float64
	sweepW      float64
}

func prepareStems(clampHeightMm float64, stems []StemOption, weights ScoreWeights) ([]preparedStem, error) {
	if !isFinite(clampHeightMm) || clampHeightMm <= 0 {
		return nil, fmt.Errorf("clamp height must be finite and positive, got %v mm", clampHeightMm)
	}
	out := make([]preparedStem, len(stems))
	for i, opt := range stems {
		part, err := NewStem(clampHeightMm, opt.LengthMm, opt.AngleDeg)
		if err != nil {
			return nil, err
		}
		out[i] = preparedStem{
			opt:    opt,
			part:   part,
			weight: weights.Stem * opt.Score,
		}
	}
	return out, nil
}

// FitFrontBuilds evaluates every discrete (spacers, stem, handlebar)
// combination on seated using MountStem, MountHandlebar, and Build.Place(Dynamic),
// returning the top limit builds ordered by CompareScoredBuilds.
func FitFrontBuilds(
	seated *SeatedFrameset,
	target FitTarget,
	front FrontBuildOptions,
	weights ScoreWeights,
	limit int,
) (BuildRanking, error) {
	seated.requireValid()
	if target.HandGrip == nil {
		return BuildRanking{}, fmt.Errorf("build fitting needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return BuildRanking{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X(), target.HandGrip.Y())
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return BuildRanking{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X(), target.BRP.Y())
	}
	if limit < 1 {
		return BuildRanking{}, fmt.Errorf("build fitting needs a positive build limit, got %d", limit)
	}
	if !isFinite(front.SpacerStepMm) || front.SpacerStepMm <= 0 {
		return BuildRanking{}, fmt.Errorf("spacer step must be positive and finite, got %v mm", front.SpacerStepMm)
	}
	if len(front.Stems) == 0 {
		return BuildRanking{}, fmt.Errorf("build fitting needs at least one stem option")
	}
	if len(front.Handlebars) == 0 {
		return BuildRanking{}, fmt.Errorf("build fitting needs at least one handlebar")
	}
	if !isFinite(float64(front.DynamicBarRollDeg)) {
		return BuildRanking{}, invalidInput(InputHandlebarRoll, "handlebar roll must be finite, got %v°", front.DynamicBarRollDeg)
	}
	if err := weights.Validate(); err != nil {
		return BuildRanking{}, err
	}
	if weights.ActiveTotal(target.BRP != nil) <= 0 {
		return BuildRanking{}, fmt.Errorf("score weights have zero active weight for this target")
	}
	type stemKey struct {
		length float64
		angle  pose.Deg
	}
	seen := make(map[stemKey]bool, len(front.Stems))
	for i, st := range front.Stems {
		if err := st.Validate(); err != nil {
			return BuildRanking{}, fmt.Errorf("stem option %d: %w", i, err)
		}
		k := stemKey{st.LengthMm, st.AngleDeg}
		if seen[k] {
			return BuildRanking{}, fmt.Errorf("stem option %d duplicates %v mm at %v°", i, st.LengthMm, st.AngleDeg)
		}
		seen[k] = true
	}

	stems, err := prepareStems(front.StemClampHeightMm, front.Stems, weights)
	if err != nil {
		return BuildRanking{}, err
	}
	bars := make([]preparedBar, len(front.Handlebars))
	for i, bar := range front.Handlebars {
		bar.requireValid()
		sweepScore := scoreHandlebarSweep(bar.Dimensions().BacksweepDeg)
		bars[i] = preparedBar{
			fromOptions: front.HasHandlebarOpts,
			index:       i,
			part:        bar,
			sweepScore:  sweepScore,
			sweepW:      weights.HandlebarSweep * sweepScore,
		}
	}

	grip := *target.HandGrip
	var brp *pose.Point
	if target.BRP != nil {
		p := *target.BRP
		brp = &p
	}
	oval := newGripOvalProjection(DefaultGripOval())
	best := make([]ScoredBuild, 0, min(limit, maxPreallocatedBuilds))
	evaluated := 0

	maxSpacers := seated.Frameset().Steerer().AllowedSpacers().Max()
	stacks := spacerSteps(0, maxSpacers, front.SpacerStepMm)
	spacerScores := make([]float64, len(stacks))
	spacerWeights := make([]float64, len(stacks))
	for i, s := range stacks {
		sc := scoreSpacers(s)
		spacerScores[i] = sc
		spacerWeights[i] = weights.Spacers * sc
	}

	best, added, err := evalFrontOnSeated(
		seated, brp, grip, oval, weights,
		stacks, spacerScores, spacerWeights,
		stems, bars, front.DynamicBarRollDeg,
		false, 0, 0, 0,
		best, limit,
	)
	if err != nil {
		return BuildRanking{}, err
	}
	evaluated += added
	return BuildRanking{Builds: best, Evaluated: evaluated}, nil
}

// FitBuilds places every combination of fork configuration, handlebar, stem,
// and spacer stack via Frameset -> SeatedFrameset -> Build, scores each with
// named scoring on SeatedFrameset, and returns the best limit builds by
// CompareScoredBuilds. Empty parts.Forks keeps the submitted fork and lower
// cup; empty parts.Handlebars keeps the submitted bar. Every bar is installed
// at setup.Handlebar.BarRollDeg. When target.BRP is set, the saddle is fitted
// first for each fork configuration, and every build on that fork carries
// those saddle inputs. The continuous stem length range on setup.Stem is ignored.
func FitBuilds(setup BikeSetup, target FitTarget, parts PartOptions, weights ScoreWeights, limit int) (BuildRanking, error) {
	if target.HandGrip == nil {
		return BuildRanking{}, fmt.Errorf("build fitting needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return BuildRanking{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X(), target.HandGrip.Y())
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return BuildRanking{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X(), target.BRP.Y())
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

	frame, err := NewFrame(setup.Frame.dimensions())
	if err != nil {
		return BuildRanking{}, err
	}
	post, err := NewRigidPost(
		setup.Seatpost.Spec.TotalLengthMm,
		setup.Seatpost.Spec.MinInsertionMm,
		setup.Seatpost.Spec.MaxInsertionMm,
		setup.Seatpost.Spec.SetbackMm,
		setup.Seatpost.Spec.RailClampLengthMm,
	)
	if err != nil {
		return BuildRanking{}, err
	}
	saddle, err := NewSaddle(SaddleDimensions{
		RailCenterToBrpMm:      setup.Saddle.RailCenterToBrpMm,
		RailCenterToSitBonesMm: setup.Saddle.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      setup.Saddle.RailToBrpHeightMm,
		SitBonesElevationMm:    setup.Saddle.SitBonesElevationMm,
		RailLengthMm:           setup.Saddle.RailLengthMm,
	})
	if err != nil {
		return BuildRanking{}, err
	}
	if !isFinite(float64(setup.Handlebar.BarRollDeg)) {
		return BuildRanking{}, invalidInput(InputHandlebarRoll, "handlebar roll must be finite, got %v°", setup.Handlebar.BarRollDeg)
	}
	stems, err := prepareStems(setup.Stem.ClampHeightMm, parts.Stems, weights)
	if err != nil {
		return BuildRanking{}, err
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

	var bars []preparedBar
	if len(parts.Handlebars) > 0 {
		bars = make([]preparedBar, len(parts.Handlebars))
		for i, barSetup := range parts.Handlebars {
			barPart, err := NewHandlebar(HandlebarDimensions{
				StockWidthMm:   barSetup.StockWidthMm,
				CurrentWidthMm: barSetup.CurrentWidthMm,
				BacksweepDeg:   barSetup.BacksweepDeg,
				UpsweepDeg:     barSetup.UpsweepDeg,
				StatedRiseMm:   barSetup.StatedRiseMm,
				RiseMode:       barSetup.RiseMode,
				BendStartMm:    barSetup.BendStartMm,
				GripInsetMm:    barSetup.GripInsetMm,
			})
			if err != nil {
				return BuildRanking{}, err
			}
			sweepScore := scoreHandlebarSweep(barSetup.BacksweepDeg)
			bars[i] = preparedBar{
				fromOptions: true,
				index:       i,
				part:        barPart,
				sweepScore:  sweepScore,
				sweepW:      weights.HandlebarSweep * sweepScore,
			}
		}
	} else {
		barPart, err := NewHandlebar(HandlebarDimensions{
			StockWidthMm:   setup.Handlebar.StockWidthMm,
			CurrentWidthMm: setup.Handlebar.CurrentWidthMm,
			BacksweepDeg:   setup.Handlebar.BacksweepDeg,
			UpsweepDeg:     setup.Handlebar.UpsweepDeg,
			StatedRiseMm:   setup.Handlebar.StatedRiseMm,
			RiseMode:       setup.Handlebar.RiseMode,
			BendStartMm:    setup.Handlebar.BendStartMm,
			GripInsetMm:    setup.Handlebar.GripInsetMm,
		})
		if err != nil {
			return BuildRanking{}, err
		}
		sweepScore := scoreHandlebarSweep(setup.Handlebar.BacksweepDeg)
		bars = []preparedBar{{
			part:       barPart,
			sweepScore: sweepScore,
			sweepW:     weights.HandlebarSweep * sweepScore,
		}}
	}

	grip := *target.HandGrip
	var brp *pose.Point
	if target.BRP != nil {
		p := *target.BRP
		brp = &p
	}
	oval := newGripOvalProjection(DefaultGripOval())

	var stacks, spacerScores, spacerWeights []float64
	best := make([]ScoredBuild, 0, min(limit, maxPreallocatedBuilds))
	evaluated := 0

	for _, fork := range forks {
		forkM, err := fork.fork.toForkMounter()
		if err != nil {
			return BuildRanking{}, err
		}
		fs, err := MateFrameset(frame, fork.lowerCup, setup.Stem.UpperStackMm, forkM, setup.Wheels)
		if err != nil {
			return BuildRanking{}, err
		}
		if setup.Stem.MaxSpacersMm != DefaultMaxSpacersMm {
			fs, err = fs.WithMaxSpacers(setup.Stem.MaxSpacersMm)
			if err != nil {
				return BuildRanking{}, err
			}
		}
		rear, err := OpenRearCockpit(fs, post, saddle, setup.Saddle.RailPitchDeg)
		if err != nil {
			return BuildRanking{}, err
		}
		var seated *SeatedFrameset
		if brp != nil {
			seated, _, err = rear.FitToBRP(*brp)
		} else {
			seated, err = rear.Clamp(setup.Seatpost.InsertionMm, setup.Saddle.RailOffsetMm)
		}
		if err != nil {
			return BuildRanking{}, err
		}
		if stacks == nil {
			stacks = spacerSteps(0, fs.Steerer().AllowedSpacers().Max(), parts.SpacerStepMm)
			spacerScores = make([]float64, len(stacks))
			spacerWeights = make([]float64, len(stacks))
			for i, spacers := range stacks {
				sc := scoreSpacers(spacers)
				spacerScores[i] = sc
				spacerWeights[i] = weights.Spacers * sc
			}
		}
		var added int
		best, added, err = evalFrontOnSeated(
			seated, brp, grip, oval, weights,
			stacks, spacerScores, spacerWeights,
			stems, bars, setup.Handlebar.BarRollDeg,
			fork.fromOptions, fork.index, setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm,
			best, limit,
		)
		if err != nil {
			return BuildRanking{}, err
		}
		evaluated += added
	}
	return BuildRanking{
		Builds:    best,
		Evaluated: evaluated,
	}, nil
}

func evalFrontOnSeated(
	seated *SeatedFrameset,
	brp *pose.Point,
	grip pose.Point,
	oval gripOvalProjection,
	weights ScoreWeights,
	stacks, spacerScores, spacerWeights []float64,
	stems []preparedStem,
	bars []preparedBar,
	dynamicBarRollDeg pose.Deg,
	hasForkOption bool,
	forkIndex int,
	stemMinLenMm, stemMaxLenMm float64,
	best []ScoredBuild,
	limit int,
) ([]ScoredBuild, int, error) {
	seatedScore, err := scoreSeatedFrameset(seated, brp, weights)
	if err != nil {
		return nil, 0, err
	}
	steererPort := seated.Frameset().Steerer()
	frameToDyn := seated.DynamicFrameset().FrameToWorld
	evaluated := 0

	for si, spacers := range stacks {
		spacersScore := spacerScores[si]
		spacersW := spacerWeights[si]
		for ti := range stems {
			st := stems[ti]
			clampedStem, err := mountStem(steererPort, spacers, st.part)
			if err != nil {
				return nil, evaluated, err
			}
			stemDyn := clampedStem.Place(frameToDyn)
			rollTurn := BarTurnForWorldRoll(stemDyn.BarClampInWorld.HeadingInDst(), dynamicBarRollDeg)
			barClampPort := clampedStem.BarClampInFrame()
			for bi := range bars {
				bar := bars[bi]
				clampedBar, err := MountHandlebar(barClampPort, bar.part, rollTurn)
				if err != nil {
					return nil, evaluated, err
				}
				barDyn := clampedBar.Place(stemDyn.BarClampInWorld)
				gripR := oval.normalizedRadius(grip, barDyn.HandGrip.Origin())
				if !isFinite(gripR) {
					return nil, evaluated, fmt.Errorf("build scoring metric is not finite")
				}
				evaluated++
				rank := seatedScore.rankScore(gripR, st.weight, bar.sweepW, spacersW)
				if len(best) == limit && compareBuildRank(rank, spacers, st.opt.LengthMm, st.opt.AngleDeg, bar.index, forkIndex, &best[len(best)-1]) >= 0 {
					continue
				}
				buildObj := &Build{
					seated:            seated,
					stem:              clampedStem,
					handlebar:         clampedBar,
					dynamicBarRollDeg: dynamicBarRollDeg,
				}
				placement := buildObj.Place(Dynamic)
				placement.Setup.Stem.MinLengthMm = stemMinLenMm
				placement.Setup.Stem.MaxLengthMm = stemMaxLenMm
				fullScore := seatedScore.buildScore(gripR, st.opt.Score, bar.sweepScore, spacersScore)
				scored := ScoredBuild{
					Placement:          placement,
					Target:             grip,
					HasHandlebarOption: bar.fromOptions,
					HandlebarIndex:     bar.index,
					HasForkOption:      hasForkOption,
					ForkIndex:          forkIndex,
					BrpTarget:          brp,
					Score:              fullScore,
				}
				best = keepBest(best, &scored, limit)
			}
		}
	}
	return best, evaluated, nil
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
