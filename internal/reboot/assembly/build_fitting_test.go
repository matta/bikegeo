package assembly_test

import (
	"cmp"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func fenrirStemOptions() []assembly.StemOption {
	return []assembly.StemOption{
		{LengthMm: 60, AngleDeg: 0, Score: 95.5},
		{LengthMm: 70, AngleDeg: -6, Score: 100.0},
		{LengthMm: 70, AngleDeg: 0, Score: 100.0},
		{LengthMm: 70, AngleDeg: 6, Score: 100.0},
		{LengthMm: 90, AngleDeg: -17, Score: 62.0},
		{LengthMm: 110, AngleDeg: -17, Score: 36.25},
		{LengthMm: 120, AngleDeg: -30, Score: 0.0},
		{LengthMm: 120, AngleDeg: -20, Score: 0.0},
		{LengthMm: 120, AngleDeg: -17, Score: 24.0},
		{LengthMm: 120, AngleDeg: -10, Score: 34.5},
	}
}

func fenrirPartOptions() assembly.PartOptions {
	return assembly.PartOptions{Stems: fenrirStemOptions(), SpacerStepMm: 5}
}

func sqlab311Flx12x15Handlebar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm: 740, CurrentWidthMm: 740, BacksweepDeg: 12, UpsweepDeg: 4,
		StatedRiseMm: 15, RiseMode: assembly.RiseModeStandardKnuckle, BendStartMm: 68.5,
		GripInsetMm: 70, BarRollDeg: 0,
	}
}

func twoBarOptions() assembly.PartOptions {
	parts := fenrirPartOptions()
	parts.Handlebars = []assembly.HandlebarSetup{persuader800DefaultHandlebar(), sqlab311Flx12x15Handlebar()}
	return parts
}

// fenrirForkOptions returns the Q31/Q129 configurations in fixture order: Lithic
// rigid; SID at 15, 20% sag; Fox 34 at 15, 20, 25% sag; all on the EC44 12 mm cup.
func fenrirForkOptions() []assembly.ForkOption {
	lithic := assembly.ForkSpec{StaticA2cMm: 495, Travel: assembly.TravelRigid, RakeMm: 54, Steerer: assembly.SteererTapered}
	sid := assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererTapered}
	fox34 := assembly.ForkSpec{StaticA2cMm: 530, Travel: assembly.Travel120, RakeMm: 44, Steerer: assembly.SteererTapered}
	cup := fenrirEc44Cup()
	return []assembly.ForkOption{
		{Fork: assembly.ForkSetup{Spec: lithic}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: sid, SagPct: 15}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: sid, SagPct: 20}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 15}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 20}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 25}, LowerCup: cup},
	}
}

func fenrirMultiForkOptions() assembly.PartOptions {
	parts := fenrirPartOptions()
	parts.Forks = fenrirForkOptions()
	return parts
}

func mustFitBuildsWith(t *testing.T, setup assembly.BikeSetup, target assembly.FitTarget, parts assembly.PartOptions, limit int) assembly.BuildRanking {
	t.Helper()
	result, err := assembly.FitBuilds(setup, target, parts, testDefaultScoreWeights(), limit)
	if err != nil {
		t.Fatalf("FitBuilds: %v", err)
	}
	return result
}

func mustFitBuilds(t *testing.T, setup assembly.BikeSetup, target assembly.FitTarget, limit int) assembly.BuildRanking {
	t.Helper()
	return mustFitBuildsWith(t, setup, target, fenrirPartOptions(), limit)
}

func legacyGripRadius(dx, dy float64) float64 {
	sin, cos := math.Sincos(math.Pi / 6)
	return math.Hypot((dx*cos-dy*sin)/10, (dx*sin+dy*cos)/20)
}

func TestFitOval(t *testing.T) {
	oval := assembly.DefaultGripOval()
	if err := oval.Validate(); err != nil {
		t.Fatal(err)
	}
	center := pose.Point{X: 12, Y: -30}
	assertNear(t, "center radius", oval.NormalizedRadius(center, center), 0, 1e-12)
	for _, tc := range []struct{ heading, distance, radius float64 }{{60, 20, 1}, {-30, 10, 1}, {-30, 30, 3}} {
		sin, cos := math.Sincos(tc.heading * math.Pi / 180)
		p := pose.Point{X: center.X + cos*tc.distance, Y: center.Y + sin*tc.distance}
		assertNear(t, "axis radius", oval.NormalizedRadius(center, p), tc.radius, 1e-12)
	}
	for _, d := range []pose.Point{{X: 23, Y: -4}, {X: -6, Y: 47}, {X: -32, Y: -51}} {
		p := pose.Point{X: center.X + d.X, Y: center.Y + d.Y}
		assertNear(t, "legacy radius", oval.NormalizedRadius(center, p), legacyGripRadius(d.X, d.Y), 1e-12)
	}
	for _, bad := range []assembly.FitOval{
		{SemiMinorMm: 0, SemiMajorMm: 20}, {SemiMinorMm: 10, SemiMajorMm: -1}, {SemiMinorMm: math.NaN(), SemiMajorMm: 20},
		{SemiMinorMm: 10, SemiMajorMm: math.Inf(1)}, {SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: pose.Deg(math.NaN())},
	} {
		if bad.Validate() == nil {
			t.Errorf("accepted oval %+v", bad)
		}
	}
}

type stemConfig struct {
	angle           pose.Deg
	length, spacers float64
}

func stemConfigOf(s assembly.StemSetup) stemConfig {
	return stemConfig{s.AngleDeg, s.LengthMm, s.SpacersMm}
}

func requireStemConfig(t *testing.T, c assembly.ScoredBuild, want stemConfig) {
	t.Helper()
	if got := stemConfigOf(c.Placement.Setup.Stem); got != want {
		t.Fatalf("stem = %+v, want %+v", got, want)
	}
}

type buildRank struct {
	tier  assembly.FitTier
	total float64
	stem  stemConfig
	r     float64
	bar   int // -1 skips the check
	fork  int // -1 skips the check
}

func rank(tier assembly.FitTier, total float64, angle pose.Deg, length, spacers, r float64) buildRank {
	return buildRank{tier: tier, total: total, stem: stemConfig{angle, length, spacers}, r: r, bar: -1, fork: -1}
}

func (r buildRank) onBar(i int) buildRank  { r.bar = i; return r }
func (r buildRank) onFork(i int) buildRank { r.fork = i; return r }

func requireRanks(t *testing.T, fit assembly.BuildRanking, want ...buildRank) {
	t.Helper()
	if len(fit.Builds) < len(want) {
		t.Fatalf("returned %d candidates, want at least %d", len(fit.Builds), len(want))
	}
	for i, w := range want {
		c := fit.Builds[i]
		if c.Score.Tier != w.tier {
			t.Errorf("rank %d tier = %v, want %v", i+1, c.Score.Tier, w.tier)
		}
		assertNear(t, "total", c.Score.Total, w.total, 1e-5)
		requireStemConfig(t, c, w.stem)
		assertNear(t, "grip radius", c.Score.GripR, w.r, 1e-5)
		if w.bar >= 0 && (!c.HasHandlebarOption || c.HandlebarIndex != w.bar) {
			t.Errorf("rank %d bar = %d, want %d", i+1, c.HandlebarIndex, w.bar)
		}
		if w.fork >= 0 && (!c.HasForkOption || c.ForkIndex != w.fork) {
			t.Errorf("rank %d fork = %d, want %d", i+1, c.ForkIndex, w.fork)
		}
	}
}

func TestFitBuildsFreeBuild(t *testing.T) {
	known := fenrirFittingSetup()
	known.Stem.AngleDeg, known.Stem.LengthMm, known.Stem.SpacersMm = 0, 70, 10

	sid := known
	sid.Fork = sidFork(20)
	p := gripPlacement(t, sid)
	grip, brp := p.Handlebar.HandGrip.Origin(), p.Saddle.BRP.Origin()
	sidBase := sid
	sidBase.Stem.AngleDeg, sidBase.Stem.LengthMm, sidBase.Stem.SpacersMm = 0, 60, 0
	fit := mustFitBuilds(t, sidBase, assembly.FitTarget{HandGrip: &grip, BRP: &brp}, 5)
	if fit.Evaluated != 70 || len(fit.Builds) != 5 {
		t.Fatalf("evaluated %d, returned %d; want 70 and 5", fit.Evaluated, len(fit.Builds))
	}
	requireRanks(t, fit, rank(assembly.FitsAll, 100, 0, 70, 10, 0))

	lithicGrip := gripOf(t, known)
	fit = mustFitBuilds(t, fenrirFittingSetup(), assembly.FitTarget{HandGrip: &lithicGrip}, 5)
	if fit.Evaluated != 70 {
		t.Fatalf("evaluated %d, want 70", fit.Evaluated)
	}
	requireRanks(t, fit, rank(assembly.FitsGrip, 99.157237, 0, 70, 10, 0))
	if fit.Builds[0].Score.GripR >= 1e-9 {
		t.Errorf("best R = %v", fit.Builds[0].Score.GripR)
	}
	for _, a := range fit.Builds[0].Score.Attributes {
		saddle := slices.Contains([]assembly.AttributeName{assembly.AttrSaddleBrpY, assembly.AttrSaddleBrpX, assembly.AttrSaddleRailOffset, assembly.AttrSeatpostInsertion}, a.Attribute)
		if a.Active == saddle {
			t.Errorf("%s Active = %v without BRP", a.Attribute, a.Active)
		}
	}
}

func TestFitBuildsReferenceBuilds(t *testing.T) {
	base := fenrirFittingSetup()
	grip := gripOf(t, base)
	fit := mustFitBuilds(t, base, assembly.FitTarget{HandGrip: &grip}, 5)
	if fit.Evaluated != 70 {
		t.Fatalf("evaluated %d, want 70", fit.Evaluated)
	}
	requireRanks(t, fit,
		rank(assembly.FitsGrip, 97.929120, 0, 60, 15, 0.095778),
		rank(assembly.FitsGrip, 97.595491, 6, 70, 10, 0.598669),
		rank(assembly.FitsGrip, 97.551593, 0, 70, 20, 0.615497),
		rank(assembly.FitsGrip, 97.435741, 6, 70, 15, 0.659907),
	)

	fit = mustFitBuilds(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 5)
	requireRanks(t, fit,
		rank(assembly.FitsNeither, 56.854583, -30, 120, 0, 1.764743),
		rank(assembly.FitsNeither, 54.312823, -30, 120, 5, 1.873003),
		rank(assembly.FitsNeither, 53.138838, -20, 120, 0, 1.923006),
	)
	assertNear(t, "fit report miss", fit.Builds[0].GripMiss().Norm(), 31.956, 1e-3)
}

func TestFitBuildsForwardAndRankOrder(t *testing.T) {
	base := fenrirFittingSetup()
	fit := mustFitBuilds(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 70)
	if fit.Evaluated != 70 || len(fit.Builds) != 70 {
		t.Fatalf("evaluated %d, returned %d; want 70 each", fit.Evaluated, len(fit.Builds))
	}
	wantOrder := []assembly.AttributeName{
		assembly.AttrHandGrip, assembly.AttrSaddleBrpY, assembly.AttrSaddleBrpX, assembly.AttrStem,
		assembly.AttrHandlebarSweep, assembly.AttrForkSag, assembly.AttrSpacers, assembly.AttrSaddleRailOffset,
		assembly.AttrRcWbRatio, assembly.AttrSeatpostInsertion, assembly.AttrLeveledBbDrop, assembly.AttrTrail,
	}
	for i, c := range fit.Builds {
		p, err := assembly.PlaceBike(c.Placement.Setup)
		if err != nil || !reflect.DeepEqual(c.Placement, p) {
			t.Fatalf("rank %d placement differs from PlaceBike: %v", i+1, err)
		}
		setup := c.Placement.Setup
		setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = base.Stem.AngleDeg, base.Stem.LengthMm, base.Stem.SpacersMm
		if !reflect.DeepEqual(setup, base) {
			t.Errorf("rank %d changed fixed inputs", i+1)
		}
		if len(c.Score.Attributes) != 12 {
			t.Fatalf("rank %d has %d attributes", i+1, len(c.Score.Attributes))
		}
		for j, a := range c.Score.Attributes {
			if a.Attribute != wantOrder[j] {
				t.Errorf("rank %d attribute %d = %s, want %s", i+1, j, a.Attribute, wantOrder[j])
			}
			if saddle := j == 1 || j == 2 || j == 7 || j == 9; a.Active == saddle {
				t.Errorf("rank %d %s Active = %v", i+1, a.Attribute, a.Active)
			}
		}
		if i > 0 && assembly.CompareScoredBuilds(fit.Builds[i-1], c) > 0 {
			t.Errorf("rank %d sorts before rank %d", i+1, i)
		}
	}
}

// interp is piecewise-linear interpolation through (xs, ys), clamped at the ends.
func interp(x float64, xs, ys []float64) float64 {
	if x <= xs[0] {
		return ys[0]
	}
	for i := 1; i < len(xs); i++ {
		if x <= xs[i] {
			return ys[i-1] + (ys[i]-ys[i-1])*(x-xs[i-1])/(xs[i]-xs[i-1])
		}
	}
	return ys[len(ys)-1]
}

func TestFitBuildsIndependentOracle(t *testing.T) {
	base := fenrirFittingSetup()
	for _, grip := range []pose.Point{fitReportGrip, gripOf(t, base)} {
		type built struct {
			stem     stemConfig
			total, r float64
			passGrip bool
		}
		var oracle []built
		for _, stem := range fenrirStemOptions() {
			for spacers := 0.0; spacers <= 30; spacers += 5 {
				setup := base
				setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = stem.AngleDeg, stem.LengthMm, spacers
				p := gripPlacement(t, setup)
				g := p.Handlebar.HandGrip.Origin()
				r := legacyGripRadius(g.X-grip.X, g.Y-grip.Y)
				gripScore := interp(r, []float64{0, 1, 2}, []float64{100, 90, 0})
				sweep := interp(float64(setup.Handlebar.BacksweepDeg), []float64{10, 16, 20, 26}, []float64{0, 100, 100, 0})
				spacerScore := interp(spacers, []float64{20, 30}, []float64{100, 0})
				rcwb := interp(p.Frameset.RcWbRatioPct(), []float64{34, 37, 39, 42}, []float64{0, 100, 100, 0})
				bb := interp(p.Frameset.LeveledBbDropMm(), []float64{45, 65, 75, 95}, []float64{0, 100, 100, 0})
				trail := interp(p.Frameset.TrailMm(), []float64{78, 98, 106, 126}, []float64{0, 100, 100, 0})
				total := (6*gripScore + 5*stem.Score + 3*sweep + 3*100 + 2*spacerScore + 2*rcwb + bb + trail) / 23
				oracle = append(oracle, built{stemConfig{stem.AngleDeg, stem.LengthMm, spacers}, total, r, r <= 1})
			}
		}
		slices.SortStableFunc(oracle, func(a, b built) int {
			if a.passGrip != b.passGrip {
				if a.passGrip {
					return -1
				}
				return 1
			}
			if !a.passGrip {
				if o := cmp.Compare(a.r, b.r); o != 0 {
					return o
				}
			}
			return cmp.Compare(b.total, a.total)
		})
		fit := mustFitBuilds(t, base, assembly.FitTarget{HandGrip: &grip}, 5)
		requireStemConfig(t, fit.Builds[0], oracle[0].stem)
		for i, c := range fit.Builds {
			assertNear(t, "oracle total", c.Score.Total, oracle[i].total, 1e-9)
		}
	}
}

func TestFitBuildsKeepsSubmittedBarByDefault(t *testing.T) {
	setup := fenrirFittingSetup()
	fit := mustFitBuilds(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 70)
	for i, c := range fit.Builds {
		if c.HasHandlebarOption || c.Placement.Setup.Handlebar != setup.Handlebar {
			t.Errorf("rank %d did not keep the submitted handlebar", i+1)
		}
	}
}

func TestFitBuildsChoosesAmongHandlebars(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	parts := twoBarOptions()
	full := mustFitBuildsWith(t, setup, target, parts, 140)
	if full.Evaluated != 140 || len(full.Builds) != 140 {
		t.Fatalf("evaluated %d, returned %d, want 140 each", full.Evaluated, len(full.Builds))
	}
	var counts [2]int
	for i, c := range full.Builds {
		index := c.HandlebarIndex
		if !c.HasHandlebarOption || index < 0 || index >= len(parts.Handlebars) {
			t.Fatalf("rank %d handlebar index = %d, want 0 or 1", i+1, index)
		}
		counts[index]++
		if c.Placement.Setup.Handlebar != parts.Handlebars[index] {
			t.Errorf("rank %d did not install handlebar option %d", i+1, index)
		}
	}
	if counts != [2]int{70, 70} {
		t.Errorf("candidate count by handlebar = %v, want [70 70]", counts)
	}
	requireRanks(t, full,
		rank(assembly.FitsGrip, 74.823105, -10, 120, 0, 0.536417).onBar(1),
		rank(assembly.FitsGrip, 74.499673, -17, 110, 0, 0.806233).onBar(1),
		rank(assembly.FitsGrip, 74.072663, -17, 110, 5, 0.969920).onBar(1),
		rank(assembly.FitsGrip, 73.717781, -10, 120, 5, 0.960125).onBar(1),
		rank(assembly.FitsGrip, 73.278784, -17, 120, 5, 0.253407).onBar(1),
	)

	setup.Handlebar = sqlab311Flx12x15Handlebar()
	single := mustFitBuilds(t, setup, target, 1)
	best := slices.IndexFunc(full.Builds, func(c assembly.ScoredBuild) bool { return c.HandlebarIndex == 1 })
	requireStemConfig(t, single.Builds[0], stemConfigOf(full.Builds[best].Placement.Setup.Stem))
	assertNear(t, "single-bar score", single.Builds[0].Score.Total, full.Builds[best].Score.Total, 1e-12)
}

func TestFitBuildsScoresBarSweep(t *testing.T) {
	fit := mustFitBuildsWith(t, fenrirFittingSetup(), assembly.FitTarget{HandGrip: &fitReportGrip}, twoBarOptions(), 140)
	if fit.Evaluated != 140 {
		t.Fatalf("evaluated %d, want 140", fit.Evaluated)
	}
	want := [...]float64{100, 33.333333}
	for i, c := range fit.Builds {
		a := c.Score.Attributes[4]
		if a.Attribute != assembly.AttrHandlebarSweep {
			t.Fatalf("attribute 4 = %s", a.Attribute)
		}
		assertNear(t, "sweep score", a.Score, want[c.HandlebarIndex], 1e-5)
		_ = i
	}
}

func TestFitBuildsInstallsEveryBarAtSubmittedRoll(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Handlebar.BarRollDeg = 5
	fit := mustFitBuildsWith(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, twoBarOptions(), 140)
	if fit.Evaluated != 140 {
		t.Fatalf("evaluated %d, want 140", fit.Evaluated)
	}
	for i, c := range fit.Builds {
		if roll := c.Placement.Setup.Handlebar.BarRollDeg; roll != 5 {
			t.Errorf("rank %d bar roll = %v, want 5", i+1, roll)
		}
	}
	requireRanks(t, fit,
		rank(assembly.FitsGrip, 75.106976, -17, 110, 0, 0.573433).onBar(1),
		rank(assembly.FitsGrip, 74.586581, -10, 120, 0, 0.627085).onBar(1),
		rank(assembly.FitsGrip, 74.478949, -17, 110, 5, 0.814177).onBar(1),
		rank(assembly.FitsGrip, 73.796352, -17, 120, 5, 0.055006).onBar(1),
		rank(assembly.FitsGrip, 72.965927, -17, 120, 10, 0.373335).onBar(1),
	)
}

func TestFitBuildsCarriesSaddleAcrossBars(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	fit := mustFitBuildsWith(t, setup, target, twoBarOptions(), 140)
	saddle, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &fitReportBRP})
	if err != nil {
		t.Fatal(err)
	}
	want := saddle.Placement.Setup
	for i, c := range fit.Builds {
		got := c.Placement.Setup
		if got.Seatpost.InsertionMm != want.Seatpost.InsertionMm || got.Saddle.RailOffsetMm != want.Saddle.RailOffsetMm {
			t.Errorf("rank %d did not carry the saddle fitting", i+1)
		}
	}
}

func TestFitBuildsKeepsSubmittedForkByDefault(t *testing.T) {
	setup := fenrirFittingSetup()
	fit := mustFitBuilds(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 70)
	for i, c := range fit.Builds {
		if c.HasForkOption || c.Placement.Setup.Fork != setup.Fork || c.Placement.Setup.LowerCup != setup.LowerCup {
			t.Errorf("rank %d did not keep the submitted fork and lower cup", i+1)
		}
		if _, ok := c.BrpMiss(); ok {
			t.Errorf("rank %d reported a BRP miss without a target", i+1)
		}
		if c.Score.HasBrp {
			t.Errorf("rank %d scored a BRP without a target", i+1)
		}
	}
}

func TestFitBuildsChoosesAmongForks(t *testing.T) {
	setup := fenrirFittingSetup()
	parts := fenrirMultiForkOptions()
	full := mustFitBuildsWith(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, parts, 420)
	if full.Evaluated != 420 || len(full.Builds) != 420 {
		t.Fatalf("evaluated %d, returned %d, want 420 each", full.Evaluated, len(full.Builds))
	}
	var counts [6]int
	for i, c := range full.Builds {
		index := c.ForkIndex
		if !c.HasForkOption || index < 0 || index >= len(parts.Forks) {
			t.Fatalf("rank %d has invalid fork option index %d", i+1, index)
		}
		counts[index]++
		if c.Placement.Setup.Fork != parts.Forks[index].Fork || c.Placement.Setup.LowerCup != parts.Forks[index].LowerCup {
			t.Errorf("rank %d did not install fork configuration %d", i+1, index)
		}
	}
	if counts != [6]int{70, 70, 70, 70, 70, 70} {
		t.Errorf("candidate count by fork = %v, want 70 each", counts)
	}
	requireRanks(t, full,
		rank(assembly.FitsNeither, 61.179441, -20, 120, 0, 1.616431).onFork(2),
		rank(assembly.FitsNeither, 60.317274, -30, 120, 0, 1.653153).onFork(2),
		rank(assembly.FitsNeither, 59.467269, -30, 120, 5, 1.689357).onFork(2),
		rank(assembly.FitsNeither, 56.854583, -30, 120, 0, 1.764743).onFork(0),
		rank(assembly.FitsNeither, 56.326779, -30, 120, 0, 1.767563).onFork(1),
	)
}

func TestFitBuildsRefitsSaddlePerFork(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	parts := fenrirMultiForkOptions()
	full := mustFitBuildsWith(t, setup, target, parts, 420)
	if full.Evaluated != 420 || len(full.Builds) != 420 {
		t.Fatalf("evaluated %d, returned %d, want 420 each", full.Evaluated, len(full.Builds))
	}
	var saddles [6]assembly.BikeSetup
	for i, fork := range parts.Forks {
		base := setup
		base.Fork, base.LowerCup = fork.Fork, fork.LowerCup
		saddles[i] = mustFit(t, base, fitReportBRP).Placement.Setup
	}
	wantMiss := [...]float64{53.368222, 53.311080, 56.197798, 41.221376, 44.669901, 48.122963}
	wantInsertion := [...]float64{197.387571, 197.372502, 198.128305, 194.085981, 195.043299, 195.986030}
	for i, c := range full.Builds {
		index := c.ForkIndex
		got, want := c.Placement.Setup, saddles[index]
		if got.Seatpost.InsertionMm != want.Seatpost.InsertionMm || got.Saddle.RailOffsetMm != want.Saddle.RailOffsetMm {
			t.Errorf("rank %d did not carry the saddle fitting for fork %d", i+1, index)
		}
		miss, ok := c.BrpMiss()
		if !ok || !c.Score.HasBrp {
			t.Fatalf("rank %d has no BRP miss for its target", i+1)
		}
		assertNear(t, "BRP miss", miss.Norm(), wantMiss[index], 1e-6)
		assertNear(t, "scored miss", math.Hypot(c.Score.BrpMissXMm, c.Score.BrpMissYMm), wantMiss[index], 1e-6)
		assertNear(t, "fitted insertion", got.Seatpost.InsertionMm, wantInsertion[index], 1e-6)
		assertNear(t, "fitted rail offset", got.Saddle.RailOffsetMm, 15, 1e-9)
	}
	requireRanks(t, full,
		rank(assembly.FitsNeither, 58.531310, -20, 120, 0, 1.616431).onFork(2),
		rank(assembly.FitsNeither, 57.980481, -30, 120, 0, 1.653153).onFork(2),
		rank(assembly.FitsNeither, 57.437422, -30, 120, 5, 1.689357).onFork(2),
		rank(assembly.FitsNeither, 55.768206, -30, 120, 0, 1.764743).onFork(0),
		rank(assembly.FitsNeither, 55.430998, -30, 120, 0, 1.767563).onFork(1),
	)
	miss, _ := full.Builds[0].BrpMiss()
	assertNear(t, "rank 1 miss", miss.Norm(), 56.197798, 1e-6)
}

func TestFitBuildsScoresBrpMissForSubmittedFork(t *testing.T) {
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	fit := mustFitBuilds(t, fenrirFittingSetup(), target, 5)
	if fit.Evaluated != 70 {
		t.Fatalf("evaluated %d, want 70", fit.Evaluated)
	}
	requireRanks(t, fit, rank(assembly.FitsNeither, 55.768206, -30, 120, 0, 1.764743))
	best := fit.Builds[0]
	for name, want := range map[assembly.AttributeName]float64{
		assembly.AttrSaddleBrpY: 100, assembly.AttrSaddleBrpX: 0, assembly.AttrSaddleRailOffset: 0, assembly.AttrSeatpostInsertion: 100,
	} {
		assertNear(t, string(name), attrScore(t, best.Score, name).Score, want, 1e-5)
	}
}

func TestFitBuildsFourTiersAndBasisFork(t *testing.T) {
	setback := fenrirFittingSetup()
	setback.Seatpost.Spec = genericPostSpec(25)
	brp := pose.Point{X: -189, Y: 713.1}
	target := assembly.FitTarget{BRP: &brp, HandGrip: &fitReportGrip}

	fit := mustFitBuilds(t, setback, target, 5)
	if fit.Evaluated != 70 {
		t.Fatalf("evaluated %d, want 70", fit.Evaluated)
	}
	requireRanks(t, fit, rank(assembly.FitsBRP, 68.295440, -30, 120, 0, 1.764743))
	assertNear(t, "rail offset score", attrScore(t, fit.Builds[0].Score, assembly.AttrSaddleRailOffset).Score, 25.490222, 1e-5)

	two := mustFitBuildsWith(t, setback, target, twoBarOptions(), 5)
	if two.Evaluated != 140 {
		t.Fatalf("evaluated %d, want 140", two.Evaluated)
	}
	requireRanks(t, two, rank(assembly.FitsAll, 79.775329, -10, 120, 0, 0.536417).onBar(1))

	basis := mustFitBuilds(t, basisSetup(), assembly.FitTarget{HandGrip: &fitReportGrip}, 5)
	requireRanks(t, basis, rank(assembly.FitsNeither, 69.082428, -20, 120, 0, 1.207581))
	assertNear(t, "basis fork sag", attrScore(t, basis.Builds[0].Score, assembly.AttrForkSag).Score, 100, 1e-9)
	assertNear(t, "basis trail", attrScore(t, basis.Builds[0].Score, assembly.AttrTrail).Score, 60.989388, 1e-5)
}

func TestCompareScoredBuildsTieBreaks(t *testing.T) {
	score := assembly.BuildScore{Tier: assembly.FitsGrip, Total: 90, GripR: 0.5}
	mk := func(spacers, length float64, angle pose.Deg, bar, fork int) assembly.ScoredBuild {
		var c assembly.ScoredBuild
		c.Score = score
		c.Placement.Setup.Stem = assembly.StemSetup{SpacersMm: spacers, LengthMm: length, AngleDeg: angle}
		c.HandlebarIndex, c.ForkIndex = bar, fork
		return c
	}
	base := mk(10, 70, 6, 1, 1)
	for _, tc := range []struct {
		name   string
		better assembly.ScoredBuild
		worse  assembly.ScoredBuild
	}{
		{"fewer spacers", mk(5, 120, 17, 5, 5), base},
		{"shorter stem", mk(10, 60, 17, 5, 5), base},
		{"smaller |angle|", mk(10, 70, 0, 5, 5), base},
		{"+angle over -angle", base, mk(10, 70, -6, 0, 0)},
		{"handlebar order", mk(10, 70, 6, 0, 5), base},
		{"fork order", mk(10, 70, 6, 1, 0), base},
	} {
		if got := assembly.CompareScoredBuilds(tc.better, tc.worse); got >= 0 {
			t.Errorf("%s: Compare(better, worse) = %d", tc.name, got)
		}
		if got := assembly.CompareScoredBuilds(tc.worse, tc.better); got <= 0 {
			t.Errorf("%s: Compare(worse, better) = %d", tc.name, got)
		}
	}
	if got := assembly.CompareScoredBuilds(base, base); got != 0 {
		t.Errorf("Compare(x, x) = %d", got)
	}
	better := base
	better.Score.Tier = assembly.FitsAll
	if assembly.CompareScoredBuilds(better, mk(0, 40, 0, 0, 0)) >= 0 {
		t.Error("fit rank must dominate cockpit tie-breaks")
	}

	setup := fenrirFittingSetup()
	forward := mustFitBuildsWith(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, twoBarOptions(), 140)
	reversedOptions := twoBarOptions()
	slices.Reverse(reversedOptions.Stems)
	reversed := mustFitBuildsWith(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, reversedOptions, 140)
	for i := range forward.Builds {
		a, b := forward.Builds[i], reversed.Builds[i]
		if stemConfigOf(a.Placement.Setup.Stem) != stemConfigOf(b.Placement.Setup.Stem) || a.HandlebarIndex != b.HandlebarIndex {
			t.Fatalf("rank %d differs after reversing stems", i+1)
		}
	}
}

func TestFitBuildsZeroSpacers(t *testing.T) {
	base := fenrirFittingSetup()
	base.Stem.SpacersMm, base.Stem.MaxSpacersMm = 0, 0
	fit := mustFitBuilds(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	if fit.Evaluated != 10 || len(fit.Builds) != 10 {
		t.Fatalf("evaluated %d, returned %d; want 10 each", fit.Evaluated, len(fit.Builds))
	}
	for _, c := range fit.Builds {
		if c.Placement.Setup.Stem.SpacersMm != 0 {
			t.Errorf("spacers = %v, want zero", c.Placement.Setup.Stem.SpacersMm)
		}
	}
}

func TestFitBuildsStepsAtHardwareLimits(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Stem.MaxSpacersMm = 30 - 5e-10
	fit := mustFitBuilds(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 71)
	if fit.Evaluated != 70 || len(fit.Builds) != 70 {
		t.Fatalf("evaluated %d, returned %d; want 70 each", fit.Evaluated, len(fit.Builds))
	}
	for _, c := range fit.Builds {
		if c.Placement.Setup.Stem.SpacersMm > setup.Stem.MaxSpacersMm {
			t.Errorf("candidate exceeded the spacer limit: %+v", c.Placement.Setup.Stem)
		}
	}
	if !slices.ContainsFunc(fit.Builds, func(c assembly.ScoredBuild) bool {
		return c.Placement.Setup.Stem.SpacersMm == setup.Stem.MaxSpacersMm
	}) {
		t.Error("no candidate landed exactly on the spacer limit")
	}
}

func TestFitBuildsIgnoresContinuousStemLengthRange(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Stem.MinLengthMm = 0
	fit := mustFitBuilds(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 5)
	if fit.Evaluated != 70 {
		t.Fatalf("evaluated %d, want 70", fit.Evaluated)
	}
}

func TestFitBuildsErrors(t *testing.T) {
	nan := pose.Point{X: math.NaN()}
	w := testDefaultScoreWeights()
	for _, tc := range []struct {
		name    string
		target  assembly.FitTarget
		limit   int
		weights assembly.ScoreWeights
		field   assembly.InputField
	}{
		{"missing grip", assembly.FitTarget{}, 5, w, ""},
		{"invalid grip", assembly.FitTarget{HandGrip: &nan}, 5, w, assembly.InputFitTargetGrip},
		{"invalid BRP", assembly.FitTarget{HandGrip: &fitReportGrip, BRP: &nan}, 5, w, assembly.InputFitTargetBRP},
		{"zero limit", assembly.FitTarget{HandGrip: &fitReportGrip}, 0, w, ""},
		{"invalid weights", assembly.FitTarget{HandGrip: &fitReportGrip}, 5, assembly.ScoreWeights{Stem: -1}, ""},
		{"saddle-only weights", assembly.FitTarget{HandGrip: &fitReportGrip}, 5, assembly.ScoreWeights{SaddleBrpY: 1}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := assembly.FitBuilds(fenrirFittingSetup(), tc.target, fenrirPartOptions(), tc.weights, tc.limit)
			if err == nil {
				t.Fatal("expected build fitting error")
			}
			if tc.field != "" {
				var input *assembly.InputError
				if !errors.As(err, &input) || input.Field != tc.field {
					t.Errorf("error = %v, want InputError for %q", err, tc.field)
				}
			}
		})
	}

	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	for _, maxSpacers := range []float64{-1, math.Inf(1)} {
		base := fenrirFittingSetup()
		base.Stem.MaxSpacersMm = maxSpacers
		_, want := assembly.PlaceBike(base)
		if _, err := assembly.FitBuilds(base, target, fenrirPartOptions(), w, 5); err == nil || want == nil || err.Error() != want.Error() {
			t.Errorf("invalid base error = %v, want %v", err, want)
		}
	}
}

func TestPartOptionsValidation(t *testing.T) {
	for _, step := range []float64{0, -5, math.NaN(), math.Inf(1)} {
		parts := fenrirPartOptions()
		parts.SpacerStepMm = step
		if parts.Validate() == nil {
			t.Errorf("accepted spacer step %v", step)
		}
	}
	parts := fenrirPartOptions()
	parts.Stems = nil
	if parts.Validate() == nil {
		t.Error("accepted empty stems")
	}
	parts = fenrirPartOptions()
	parts.Stems = append(parts.Stems, assembly.StemOption{LengthMm: 70, AngleDeg: 0, Score: 50})
	if err := parts.Validate(); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Errorf("expected duplicate stem error, got %v", err)
	}
	parts = fenrirPartOptions()
	parts.Stems[0].Score = 101
	if parts.Validate() == nil {
		t.Error("accepted invalid stem")
	}
	if err := twoBarOptions().Validate(); err != nil {
		t.Errorf("valid handlebars: %v", err)
	}
	parts = twoBarOptions()
	parts.Handlebars[0].BarRollDeg = 5
	if err := parts.Validate(); err == nil || !strings.Contains(err.Error(), "BarRollDeg") {
		t.Errorf("expected BarRollDeg error, got %v", err)
	}
	parts = twoBarOptions()
	parts.Handlebars[0].GripInsetMm = 0
	_, want := assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0), parts.Handlebars[0])
	if err := parts.Validate(); want == nil || err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Errorf("handlebar option grip inset error = %v, want %v", err, want)
	}
	parts = fenrirMultiForkOptions()
	if err := parts.Validate(); err != nil {
		t.Errorf("valid fork configurations: %v", err)
	}
	parts.Forks[1].Fork.SagPct = 150
	if err := parts.Validate(); err == nil || !strings.Contains(err.Error(), "fork option 1: fork sag") {
		t.Errorf("expected fork option sag error, got %v", err)
	}
	parts = fenrirMultiForkOptions()
	parts.Forks[0].LowerCup.StackMm = 0
	if err := parts.Validate(); err == nil || !strings.Contains(err.Error(), "fork option 0 lower cup") {
		t.Errorf("expected fork option lower cup error, got %v", err)
	}
}
