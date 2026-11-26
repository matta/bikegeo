package assembly_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// referenceFitBuilds is the original exhaustive FitBuilds without a limit: it
// places and scores every fork, handlebar, stem, and spacer combination and
// returns all of them sorted by CompareScoredBuilds, so FitBuilds at any limit
// must return a prefix of it. It skips input validation, so callers pass valid
// inputs only. TestFitBuildsMatchesReference holds FitBuilds to it.
func referenceFitBuilds(t *testing.T, setup assembly.BikeSetup, target assembly.FitTarget, parts assembly.PartOptions, weights assembly.ScoreWeights) []assembly.ScoredBuild {
	t.Helper()
	type forkChoice struct {
		fromOptions bool
		index       int
		option      assembly.ForkOption
	}
	forks := []forkChoice{{option: assembly.ForkOption{Fork: setup.Fork, LowerCup: setup.LowerCup}}}
	if len(parts.Forks) > 0 {
		forks = forks[:0]
		for i, f := range parts.Forks {
			forks = append(forks, forkChoice{fromOptions: true, index: i, option: f})
		}
	}
	type barChoice struct {
		fromOptions bool
		index       int
		setup       assembly.HandlebarSetup
	}
	bars := []barChoice{{setup: setup.Handlebar}}
	if len(parts.Handlebars) > 0 {
		bars = bars[:0]
		for i, bar := range parts.Handlebars {
			bar.BarRollDeg = setup.Handlebar.BarRollDeg
			bars = append(bars, barChoice{fromOptions: true, index: i, setup: bar})
		}
	}
	var stacks []float64
	for spacers := 0.0; spacers <= setup.Stem.MaxSpacersMm+1e-9; spacers += parts.SpacerStepMm {
		stacks = append(stacks, min(spacers, setup.Stem.MaxSpacersMm))
	}

	grip := *target.HandGrip
	var brp *pose.Point[assembly.DynamicWorld]
	if target.BRP != nil {
		p := *target.BRP
		brp = &p
	}
	var builds []assembly.ScoredBuild
	for _, fork := range forks {
		base := setup
		base.Fork, base.LowerCup = fork.option.Fork, fork.option.LowerCup
		if brp != nil {
			saddle, err := assembly.ApplyFitting(base, assembly.FitTarget{BRP: brp})
			if err != nil {
				t.Fatalf("reference ApplyFitting: %v", err)
			}
			base = saddle.Placement.Setup
		}
		buildTarget := assembly.FitTarget{HandGrip: &grip, BRP: brp}
		for _, bar := range bars {
			for _, stem := range parts.Stems {
				for _, spacers := range stacks {
					candidate := base
					candidate.Stem.AngleDeg = stem.AngleDeg
					candidate.Stem.LengthMm = stem.LengthMm
					candidate.Stem.SpacersMm = spacers
					candidate.Handlebar = bar.setup
					placement, err := assembly.PlaceBike(candidate)
					if err != nil {
						t.Fatalf("reference PlaceBike: %v", err)
					}
					score, err := assembly.ScoreBuild(placement, buildTarget, stem.Score, weights)
					if err != nil {
						t.Fatalf("reference ScoreBuild: %v", err)
					}
					builds = append(builds, assembly.ScoredBuild{
						Placement:          placement,
						Target:             grip,
						HasHandlebarOption: bar.fromOptions,
						HandlebarIndex:     bar.index,
						HasForkOption:      fork.fromOptions,
						ForkIndex:          fork.index,
						BrpTarget:          brp,
						Score:              score,
					})
				}
			}
		}
	}
	slices.SortFunc(builds, assembly.CompareScoredBuilds)
	return builds
}

// TestFitBuildsMatchesReference checks that FitBuilds returns exactly the
// builds, in exactly the order, that the exhaustive sort returns, across
// submitted and offered forks and bars, targets that land in every fit tier,
// and limits from one build to every build.
func TestFitBuildsMatchesReference(t *testing.T) {
	setback := fenrirFittingSetup()
	setback.Seatpost.Spec = genericPostSpec(25)
	setups := map[string]assembly.BikeSetup{"fenrir": fenrirFittingSetup(), "setback": setback}

	multi := twoBarOptions()
	multi.Forks = fenrirForkOptions()
	partSets := map[string]assembly.PartOptions{
		"stems":          fenrirPartOptions(),
		"bars":           twoBarOptions(),
		"forks":          fenrirMultiForkOptions(),
		"bars_and_forks": multi,
	}

	setbackBRP := pose.Point[assembly.DynamicWorld]{X: -189, Y: 713.1}
	farBRP := pose.Point[assembly.DynamicWorld]{X: -260, Y: 640}
	brps := map[string]*pose.Point[assembly.DynamicWorld]{"no_brp": nil, "report_brp": &fitReportBRP, "setback_brp": &setbackBRP, "far_brp": &farBRP}

	grips := []pose.Point[assembly.DynamicWorld]{
		{X: fitReportGrip.X - 60, Y: fitReportGrip.Y - 60},
		fitReportGrip,
		{X: fitReportGrip.X + 60, Y: fitReportGrip.Y + 60},
	}

	const everyBuild = 1 << 20
	defaultLimits := []int{1, 5}
	fullLimits := []int{1, 2, 5, 23, everyBuild}

	weights := testDefaultScoreWeights()
	winningTiers := map[assembly.FitTier]bool{}
	for setupName, setup := range setups {
		for partsName, parts := range partSets {
			for brpName, brp := range brps {
				for gi, grip := range grips {
					target := assembly.FitTarget{HandGrip: &grip, BRP: brp}
					all := referenceFitBuilds(t, setup, target, parts, weights)
					winningTiers[all[0].Score.Tier] = true
					limits := defaultLimits
					if setupName == "fenrir" && (brpName == "no_brp" || brpName == "report_brp") && gi == 1 {
						limits = fullLimits
					}
					for _, limit := range limits {
						got, err := assembly.FitBuilds(setup, target, parts, weights, limit)
						if err != nil {
							t.Fatalf("%s/%s/%s grip %v limit %d: FitBuilds: %v", setupName, partsName, brpName, grip, limit, err)
						}
						if got.Evaluated != len(all) {
							t.Fatalf("%s/%s/%s grip %v limit %d: Evaluated = %d, want %d", setupName, partsName, brpName, grip, limit, got.Evaluated, len(all))
						}
						if want := all[:min(limit, len(all))]; !reflect.DeepEqual(got.Builds, want) {
							t.Fatalf("%s/%s/%s grip %v limit %d: builds differ from the exhaustive sort", setupName, partsName, brpName, grip, limit)
						}
					}
				}
			}
		}
	}
	for _, tier := range []assembly.FitTier{assembly.FitsAll, assembly.FitsBRP, assembly.FitsGrip, assembly.FitsNeither} {
		if !winningTiers[tier] {
			t.Errorf("no case's winning build is in tier %v; widen the targets so every tie-break chain decides a winner", tier)
		}
	}
}
