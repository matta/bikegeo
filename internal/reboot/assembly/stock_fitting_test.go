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

func fenrirCockpitStock() assembly.CockpitStock {
	return assembly.CockpitStock{
		StemLengthStepMm: 10,
		StemAnglesDeg:    []pose.Deg{-17, -6, 0, 6, 17},
		SpacerStepMm:     5,
	}
}

func sqlab311Flx12x15Handlebar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm: 740, CurrentWidthMm: 740, BacksweepDeg: 12, UpsweepDeg: 4,
		StatedRiseMm: 15, RiseMode: assembly.RiseModeStandardKnuckle, BendStartMm: 68.5,
		GripInsetMm: 70, BarRollDeg: 0,
	}
}

func twoBarStock() assembly.CockpitStock {
	stock := fenrirCockpitStock()
	stock.Handlebars = []assembly.HandlebarSetup{persuader800DefaultHandlebar(), sqlab311Flx12x15Handlebar()}
	return stock
}

// fenrirStockForks returns the Q31 configurations in fixture order: Lithic
// rigid; SID at 15, 20, 25% sag; Fox 34 at 15, 20, 25% sag; all on the EC44 12 mm cup.
func fenrirStockForks() []assembly.StockFork {
	lithic := assembly.ForkSpec{StaticA2cMm: 495, TravelMm: 0, RakeMm: 54, Steerer: assembly.SteererTapered}
	sid := assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererTapered}
	fox34 := assembly.ForkSpec{StaticA2cMm: 530, TravelMm: 120, RakeMm: 44, Steerer: assembly.SteererTapered}
	cup := fenrirEc44Cup()
	return []assembly.StockFork{
		{Fork: assembly.ForkSetup{Spec: lithic}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: sid, SagPct: 15}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: sid, SagPct: 20}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: sid, SagPct: 25}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 15}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 20}, LowerCup: cup},
		{Fork: assembly.ForkSetup{Spec: fox34, SagPct: 25}, LowerCup: cup},
	}
}

func fenrirForkStock() assembly.CockpitStock {
	stock := fenrirCockpitStock()
	stock.Forks = fenrirStockForks()
	return stock
}

func illustrativeCockpitPreferences() assembly.CockpitPreferences {
	return assembly.CockpitPreferences{
		GripOval:   assembly.FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60, PointsPerR: 22},
		StemLength: assembly.PenaltyBand{FreeMin: 70, FreeMax: 80, PointsPerUnitBelow: 0.2, PointsPerUnitAbove: 0.2},
		StemAngle:  assembly.PenaltyBand{PointsPerUnitBelow: 0.2, PointsPerUnitAbove: 0.2},
		Spacers:    assembly.PenaltyBand{FreeMax: 20, PointsPerUnitAbove: 0.4},
		BarSweep:   assembly.PenaltyBand{FreeMin: 16, FreeMax: 20, PointsPerUnitBelow: 2, PointsPerUnitAbove: 2},
		ForkSag:    assembly.PenaltyBand{FreeMin: 20, FreeMax: 20, PointsPerUnitBelow: 2, PointsPerUnitAbove: 2},
		BrpMiss:    assembly.PenaltyBand{FreeMax: 1, PointsPerUnitAbove: 2},
	}
}

func mustStockFit(t *testing.T, setup assembly.BikeSetup, target assembly.FitTarget, limit int) assembly.StockFitting {
	t.Helper()
	result, err := assembly.ApplyStockFitting(setup, target, fenrirCockpitStock(), illustrativeCockpitPreferences(), limit)
	if err != nil {
		t.Fatalf("ApplyStockFitting: %v", err)
	}
	return result
}

func TestPenaltyBand(t *testing.T) {
	prefs := illustrativeCockpitPreferences()
	for _, tc := range []struct{ value, points float64 }{{60, 2}, {70, 0}, {75, 0}, {80, 0}, {130, 10}} {
		assertNear(t, "length points", prefs.StemLength.PointsLost(tc.value), tc.points, 1e-12)
	}
	assertNear(t, "spacer points", prefs.Spacers.PointsLost(40), 8, 1e-12)
}

func TestBarSweepBand(t *testing.T) {
	band := illustrativeCockpitPreferences().BarSweep
	for _, tc := range []struct{ sweep, points float64 }{{12, 8}, {16, 0}, {18, 0}, {20, 0}, {22, 4}} {
		if got := band.PointsLost(tc.sweep); got != tc.points {
			t.Errorf("bar sweep %v° loses %v points, want %v", tc.sweep, got, tc.points)
		}
	}
}

func legacyGripRadius(dx, dy float64) float64 {
	sin, cos := math.Sincos(math.Pi / 6)
	return math.Hypot((dx*cos-dy*sin)/10, (dx*sin+dy*cos)/20)
}

func TestFitOval(t *testing.T) {
	oval := illustrativeCockpitPreferences().GripOval
	center := pose.Point{X: 12, Y: -30}
	assertNear(t, "center radius", oval.NormalizedRadius(center, center), 0, 1e-12)
	for _, tc := range []struct{ heading, distance, radius float64 }{{60, 20, 1}, {-30, 10, 1}, {-30, 30, 3}} {
		sin, cos := math.Sincos(tc.heading * math.Pi / 180)
		p := pose.Point{X: center.X + cos*tc.distance, Y: center.Y + sin*tc.distance}
		assertNear(t, "axis radius", oval.NormalizedRadius(center, p), tc.radius, 1e-12)
	}
	assertNear(t, "oval points", oval.PointsLost(3), 44, 1e-12)
	for _, d := range []pose.Point{{X: 23, Y: -4}, {X: -6, Y: 47}, {X: -32, Y: -51}} {
		p := pose.Point{X: center.X + d.X, Y: center.Y + d.Y}
		assertNear(t, "legacy radius", oval.NormalizedRadius(center, p), legacyGripRadius(d.X, d.Y), 1e-12)
	}
}

type stockStem struct {
	angle           pose.Deg
	length, spacers float64
}

func stockStemOf(s assembly.StemSetup) stockStem {
	return stockStem{s.AngleDeg, s.LengthMm, s.SpacersMm}
}

func requireStockStem(t *testing.T, c assembly.StockCandidate, want stockStem) {
	t.Helper()
	if got := stockStemOf(c.Placement.Setup.Stem); got != want {
		t.Fatalf("stem = %+v, want %+v", got, want)
	}
}

func TestStockFittingFreeBuild(t *testing.T) {
	base := fenrirFittingSetup()
	known := base
	known.Stem.AngleDeg, known.Stem.LengthMm, known.Stem.SpacersMm = 0, 70, 10
	grip := gripOf(t, known)
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &grip}, 5)
	if fit.Evaluated != 350 || len(fit.Candidates) != 5 {
		t.Fatalf("evaluated %d, returned %d; want 350 and 5", fit.Evaluated, len(fit.Candidates))
	}
	best := fit.Candidates[0]
	requireStockStem(t, best, stockStem{0, 70, 10})
	if best.GripR >= 1e-9 || best.PointsLost() != 0 || best.Score() != 100 {
		t.Errorf("best build has R %v, points lost %v, score %v", best.GripR, best.PointsLost(), best.Score())
	}
}

func TestStockFittingReferenceBuilds(t *testing.T) {
	base := fenrirFittingSetup()
	grip := gripOf(t, base)
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &grip}, 5)
	for i, want := range []struct {
		stem           stockStem
		radius, points float64
	}{
		{stockStem{0, 70, 20}, 0.615497, 0},
		{stockStem{0, 70, 15}, 0.694523, 0},
		{stockStem{0, 70, 10}, 0.972410, 0},
		{stockStem{6, 70, 10}, 0.598669, 1.2},
	} {
		c := fit.Candidates[i]
		requireStockStem(t, c, want.stem)
		assertNear(t, "default grip radius", c.GripR, want.radius, 1e-5)
		assertNear(t, "default grip points", c.PointsLost(), want.points, 1e-9)
	}

	fit = mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 5)
	for i, want := range []struct {
		stem  stockStem
		score float64
	}{
		{stockStem{-17, 130, 0}, 78.711983},
		{stockStem{-17, 130, 5}, 69.982194},
		{stockStem{-17, 120, 0}, 62.174803},
	} {
		c := fit.Candidates[i]
		requireStockStem(t, c, want.stem)
		assertNear(t, "fit report score", c.Score(), want.score, 1e-5)
	}
	best := fit.Candidates[0]
	assertNear(t, "fit report radius", best.GripR, 1.358546, 1e-5)
	assertNear(t, "fit report miss", best.GripMiss().Norm(), 13.624, 1e-3)
	for i, points := range []float64{7.888017, 10, 3.4, 0, 0, 0, 0} {
		assertNear(t, "fit report penalties", best.Penalties[i].PointsLost, points, 1e-5)
	}
}

func TestStockFittingForwardAndRankOrder(t *testing.T) {
	base := fenrirFittingSetup()
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	if fit.Evaluated != 350 || len(fit.Candidates) != 350 {
		t.Fatalf("evaluated %d, returned %d; want 350 each", fit.Evaluated, len(fit.Candidates))
	}
	wantBands := []assembly.BandName{
		assembly.BandGripOval, assembly.BandStemLength, assembly.BandStemAngle,
		assembly.BandSpacers, assembly.BandBarSweep, assembly.BandForkSag, assembly.BandBrpMiss,
	}
	for i, c := range fit.Candidates {
		p, err := assembly.PlaceBike(c.Placement.Setup)
		if err != nil || !reflect.DeepEqual(c.Placement, p) {
			t.Fatalf("rank %d placement differs from PlaceBike: %v", i+1, err)
		}
		setup := c.Placement.Setup
		setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = base.Stem.AngleDeg, base.Stem.LengthMm, base.Stem.SpacersMm
		if !reflect.DeepEqual(setup, base) {
			t.Errorf("rank %d changed fixed inputs", i+1)
		}
		var bands []assembly.BandName
		for _, line := range c.Penalties {
			bands = append(bands, line.Band)
		}
		if !slices.Equal(bands, wantBands) {
			t.Errorf("rank %d bands = %v, want %v", i+1, bands, wantBands)
		}
		if i > 0 {
			prev := fit.Candidates[i-1]
			if c.PointsLost() < prev.PointsLost() || (c.PointsLost() == prev.PointsLost() && c.GripR < prev.GripR) {
				t.Errorf("rank %d sorts before rank %d", i+1, i)
			}
		}
	}
	if score := fit.Candidates[349].Score(); score >= 0 {
		t.Errorf("last score = %v, want a negative score", score)
	}
}

func TestStockFittingIndependentPenalties(t *testing.T) {
	base := fenrirFittingSetup()
	for _, grip := range []pose.Point{fitReportGrip, gripOf(t, base)} {
		type scoredStem struct {
			stem      stockStem
			points, r float64
		}
		var oracle []scoredStem
		for _, angle := range []pose.Deg{-17, -6, 0, 6, 17} {
			for length := 40.0; length <= 130; length += 10 {
				for spacers := 0.0; spacers <= 30; spacers += 5 {
					setup := base
					setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = angle, length, spacers
					p := gripOf(t, setup)
					r := legacyGripRadius(p.X-grip.X, p.Y-grip.Y)
					points := 22*max(0, r-1) + 0.2*max(0, 70-length, length-80) + 0.2*math.Abs(float64(angle)) + 0.4*max(0, spacers-20)
					oracle = append(oracle, scoredStem{stockStem{angle, length, spacers}, points, r})
				}
			}
		}
		slices.SortStableFunc(oracle, func(a, b scoredStem) int {
			if order := cmp.Compare(a.points, b.points); order != 0 {
				return order
			}
			return cmp.Compare(a.r, b.r)
		})
		fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &grip}, 5)
		requireStockStem(t, fit.Candidates[0], oracle[0].stem)
		for i, c := range fit.Candidates {
			assertNear(t, "oracle points", c.PointsLost(), oracle[i].points, 1e-9)
		}
	}
}

func TestStockFittingFitsSaddleFirst(t *testing.T) {
	base := fenrirFittingSetup()
	saddle := mustFit(t, base, fitReportBRP).Placement.Setup
	both := mustStockFit(t, base, assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}, 350)
	gripOnly := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	for i, c := range both.Candidates {
		got := c.Placement.Setup
		if got.Seatpost.ExtensionMm != saddle.Seatpost.ExtensionMm || got.Saddle.RailOffsetMm != saddle.Saddle.RailOffsetMm {
			t.Errorf("rank %d doesn't carry the saddle fitting", i+1)
		}
		requireStockStem(t, c, stockStemOf(gripOnly.Candidates[i].Placement.Setup.Stem))
	}
}

func TestStockFittingKeepsSubmittedBarByDefault(t *testing.T) {
	setup := fenrirFittingSetup()
	fit := mustStockFit(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	for i, c := range fit.Candidates {
		if c.StockHandlebar || c.Placement.Setup.Handlebar != setup.Handlebar {
			t.Errorf("rank %d did not keep the submitted handlebar", i+1)
		}
	}
}

func TestStockFittingChoosesAmongHandlebars(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	stock := twoBarStock()
	prefs := illustrativeCockpitPreferences()
	full, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 700)
	if err != nil {
		t.Fatal(err)
	}
	if full.Evaluated != 700 || len(full.Candidates) != 700 {
		t.Fatalf("evaluated %d, returned %d, want 700 each", full.Evaluated, len(full.Candidates))
	}
	var counts [2]int
	for i, c := range full.Candidates {
		index := c.HandlebarIndex
		if !c.StockHandlebar || index < 0 || index >= len(stock.Handlebars) {
			t.Fatalf("rank %d handlebar index = %d, want 0 or 1", i+1, index)
		}
		counts[index]++
		if c.Placement.Setup.Handlebar != stock.Handlebars[index] {
			t.Errorf("rank %d did not install stock handlebar %d", i+1, index)
		}
	}
	if counts != [2]int{350, 350} {
		t.Errorf("candidate count by handlebar = %v, want [350 350]", counts)
	}
	fit, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(fit.Candidates) != 5 {
		t.Fatalf("returned %d candidates, want 5", len(fit.Candidates))
	}
	for i, want := range []struct {
		stem          stockStem
		radius, score float64
	}{
		{stockStem{-17, 110, 0}, 0.806233, 82.6},
		{stockStem{-17, 110, 5}, 0.969920, 82.6},
		{stockStem{-6, 130, 0}, 0.919776, 80.8},
		{stockStem{-17, 120, 5}, 0.253407, 80.6},
		{stockStem{-17, 120, 10}, 0.396539, 80.6},
	} {
		c := fit.Candidates[i]
		if !c.StockHandlebar || c.HandlebarIndex != 1 {
			t.Errorf("rank %d handlebar index = %d, want 1", i+1, c.HandlebarIndex)
		}
		requireStockStem(t, c, want.stem)
		assertNear(t, "grip radius", c.GripR, want.radius, 1e-5)
		assertNear(t, "score", c.Score(), want.score, 1e-9)
	}

	setup.Handlebar = sqlab311Flx12x15Handlebar()
	single, err := assembly.ApplyStockFitting(setup, target, fenrirCockpitStock(), prefs, 1)
	if err != nil {
		t.Fatal(err)
	}
	best := fit.Candidates[0]
	requireStockStem(t, best, stockStemOf(single.Candidates[0].Placement.Setup.Stem))
	assertNear(t, "single-bar score", best.Score(), single.Candidates[0].Score(), 1e-12)
}

func TestStockFittingPenalizesSweep(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	fit, err := assembly.ApplyStockFitting(setup, target, twoBarStock(), illustrativeCockpitPreferences(), 700)
	if err != nil {
		t.Fatal(err)
	}
	if len(fit.Candidates) != 700 {
		t.Fatalf("returned %d candidates, want 700", len(fit.Candidates))
	}
	pointsByBar := [...]float64{0, 8}
	for i, c := range fit.Candidates {
		if c.HandlebarIndex < 0 || c.HandlebarIndex >= len(pointsByBar) {
			t.Fatalf("rank %d handlebar index = %d, want 0 or 1", i+1, c.HandlebarIndex)
		}
		if len(c.Penalties) != 7 {
			t.Fatalf("rank %d has %d penalty lines, want 7", i+1, len(c.Penalties))
		}
		want := assembly.Penalty{Band: assembly.BandBarSweep, PointsLost: pointsByBar[c.HandlebarIndex]}
		if got := c.Penalties[4]; got != want {
			t.Errorf("rank %d sweep line = %+v, want %+v", i+1, got, want)
		}
	}
}

func TestStockFittingInstallsEveryBarAtSubmittedRoll(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Handlebar.BarRollDeg = 5
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	stock := twoBarStock()
	prefs := illustrativeCockpitPreferences()
	full, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 700)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range full.Candidates {
		if roll := c.Placement.Setup.Handlebar.BarRollDeg; roll != 5 {
			t.Errorf("rank %d bar roll = %v, want 5", i+1, roll)
		}
	}
	fit, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		index         int
		stem          stockStem
		radius, score float64
	}{
		{0, stockStem{-17, 110, 0}, 0.573433, 82.6},
		{2, stockStem{-17, 120, 5}, 0.055006, 80.6},
	} {
		c := fit.Candidates[want.index]
		if !c.StockHandlebar || c.HandlebarIndex != 1 {
			t.Errorf("rank %d handlebar index = %d, want 1", want.index+1, c.HandlebarIndex)
		}
		requireStockStem(t, c, want.stem)
		assertNear(t, "grip radius at submitted roll", c.GripR, want.radius, 1e-5)
		assertNear(t, "score at submitted roll", c.Score(), want.score, 1e-9)
	}
}

func TestStockFittingCarriesSaddleAcrossBars(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	fit, err := assembly.ApplyStockFitting(setup, target, twoBarStock(), illustrativeCockpitPreferences(), 700)
	if err != nil {
		t.Fatal(err)
	}
	saddle, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &fitReportBRP})
	if err != nil {
		t.Fatal(err)
	}
	want := saddle.Placement.Setup
	for i, c := range fit.Candidates {
		got := c.Placement.Setup
		if got.Seatpost.ExtensionMm != want.Seatpost.ExtensionMm || got.Saddle.RailOffsetMm != want.Saddle.RailOffsetMm {
			t.Errorf("rank %d did not carry the saddle fitting", i+1)
		}
	}
}

func TestStockFittingKeepsSubmittedForkByDefault(t *testing.T) {
	setup := fenrirFittingSetup()
	fit := mustStockFit(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	for i, c := range fit.Candidates {
		if c.StockFork || c.Placement.Setup.Fork != setup.Fork || c.Placement.Setup.LowerCup != setup.LowerCup {
			t.Errorf("rank %d did not keep the submitted fork and lower cup", i+1)
		}
		if _, ok := c.BrpMiss(); ok {
			t.Errorf("rank %d reported a BRP miss without a target", i+1)
		}
		if c.Penalties[5] != (assembly.Penalty{Band: assembly.BandForkSag}) ||
			c.Penalties[6] != (assembly.Penalty{Band: assembly.BandBrpMiss}) {
			t.Errorf("rank %d fork sag and BRP miss penalties = %v, want zero points", i+1, c.Penalties[5:])
		}
	}
}

type stockForkRank struct {
	index         int
	stem          stockStem
	radius, score float64
}

func requireStockForkRanks(t *testing.T, fit assembly.StockFitting, want []stockForkRank) {
	t.Helper()
	if len(fit.Candidates) != len(want) {
		t.Fatalf("returned %d candidates, want %d", len(fit.Candidates), len(want))
	}
	for i, rank := range want {
		c := fit.Candidates[i]
		if !c.StockFork || c.ForkIndex != rank.index {
			t.Errorf("rank %d fork index = %d, want %d", i+1, c.ForkIndex, rank.index)
		}
		requireStockStem(t, c, rank.stem)
		assertNear(t, "grip radius", c.GripR, rank.radius, 1e-5)
		assertNear(t, "score", c.Score(), rank.score, 1e-6)
	}
}

func TestStockFittingChoosesAmongForks(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	stock := fenrirForkStock()
	prefs := illustrativeCockpitPreferences()
	full, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 2450)
	if err != nil {
		t.Fatal(err)
	}
	if full.Evaluated != 2450 || len(full.Candidates) != 2450 {
		t.Fatalf("evaluated %d, returned %d, want 2450 each", full.Evaluated, len(full.Candidates))
	}
	var counts [7]int
	sagPoints := [...]float64{0, 10, 0, 10, 10, 0, 10}
	for i, c := range full.Candidates {
		index := c.ForkIndex
		if !c.StockFork || index < 0 || index >= len(stock.Forks) {
			t.Fatalf("rank %d has invalid stock fork index %d", i+1, index)
		}
		counts[index]++
		if c.Placement.Setup.Fork != stock.Forks[index].Fork || c.Placement.Setup.LowerCup != stock.Forks[index].LowerCup {
			t.Errorf("rank %d did not install fork configuration %d", i+1, index)
		}
		want := assembly.Penalty{Band: assembly.BandForkSag, PointsLost: sagPoints[index]}
		if got := c.Penalties[5]; got != want {
			t.Errorf("rank %d fork sag line = %+v, want %+v", i+1, got, want)
		}
	}
	if counts != [7]int{350, 350, 350, 350, 350, 350, 350} {
		t.Errorf("candidate count by fork = %v, want 350 each", counts)
	}

	fit, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 5)
	if err != nil {
		t.Fatal(err)
	}
	requireStockForkRanks(t, fit, []stockForkRank{
		{2, stockStem{-17, 130, 0}, 1.019761, 86.165257},
		{0, stockStem{-17, 130, 0}, 1.358546, 78.711983},
		{2, stockStem{-17, 130, 5}, 1.412418, 77.526802},
		{3, stockStem{-17, 130, 0}, 0.676521, 76.600000},
		{3, stockStem{-17, 130, 5}, 1.061626, 75.244228},
	})
}

func TestStockFittingRefitsSaddlePerFork(t *testing.T) {
	setup := fenrirFittingSetup()
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	stock := fenrirForkStock()
	prefs := illustrativeCockpitPreferences()
	full, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 2450)
	if err != nil {
		t.Fatal(err)
	}
	if full.Evaluated != 2450 || len(full.Candidates) != 2450 {
		t.Fatalf("evaluated %d, returned %d, want 2450 each", full.Evaluated, len(full.Candidates))
	}
	var saddles [7]assembly.BikeSetup
	for i, fork := range stock.Forks {
		base := setup
		base.Fork, base.LowerCup = fork.Fork, fork.LowerCup
		saddles[i] = mustFit(t, base, fitReportBRP).Placement.Setup
	}
	wantMiss := [...]float64{39.613611, 39.553918, 42.572417, 45.600102, 26.978855, 30.554575, 34.144103}
	for i, c := range full.Candidates {
		index := c.ForkIndex
		if !c.StockFork || index < 0 || index >= len(stock.Forks) {
			t.Fatalf("rank %d has invalid stock fork index %d", i+1, index)
		}
		got, want := c.Placement.Setup, saddles[index]
		if got.Seatpost.ExtensionMm != want.Seatpost.ExtensionMm || got.Saddle.RailOffsetMm != want.Saddle.RailOffsetMm {
			t.Errorf("rank %d did not carry the saddle fitting for fork %d", i+1, index)
		}
		miss, ok := c.BrpMiss()
		if !ok {
			t.Fatalf("rank %d has no BRP miss for its target", i+1)
		}
		assertNear(t, "BRP miss", miss.Norm(), wantMiss[index], 1e-6)
		assertNear(t, "BRP miss points", c.Penalties[6].PointsLost, 2*(miss.Norm()-1), 1e-9)
	}

	fit, err := assembly.ApplyStockFitting(setup, target, stock, prefs, 5)
	if err != nil {
		t.Fatal(err)
	}
	requireStockForkRanks(t, fit, []stockForkRank{
		{2, stockStem{-17, 130, 0}, 1.019761, 3.020423},
		{0, stockStem{-17, 130, 0}, 1.358546, 1.484762},
		{5, stockStem{-17, 130, 0}, 2.401938, -3.351795},
		{2, stockStem{-17, 130, 5}, 1.412418, -5.618032},
		{0, stockStem{-17, 130, 5}, 1.755355, -7.245027},
	})
}

func TestStockFittingPenalizesBrpMissForSubmittedFork(t *testing.T) {
	target := assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}
	fit := mustStockFit(t, fenrirFittingSetup(), target, 5)
	for _, c := range fit.Candidates {
		assertNear(t, "BRP miss points", c.Penalties[6].PointsLost, 77.227222, 1e-6)
	}
	best := fit.Candidates[0]
	requireStockStem(t, best, stockStem{-17, 130, 0})
	assertNear(t, "score", best.Score(), 1.484762, 1e-6)
}

func TestStockFittingZeroSpacers(t *testing.T) {
	base := fenrirFittingSetup()
	base.Stem.SpacersMm, base.Stem.MaxSpacersMm = 0, 0
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 350)
	if fit.Evaluated != 50 || len(fit.Candidates) != 50 {
		t.Fatalf("evaluated %d, returned %d; want 50 each", fit.Evaluated, len(fit.Candidates))
	}
	for _, c := range fit.Candidates {
		if c.Placement.Setup.Stem.SpacersMm != 0 {
			t.Errorf("spacers = %v, want zero", c.Placement.Setup.Stem.SpacersMm)
		}
	}
}

func TestStockFittingStepsAtHardwareLimits(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Stem.MaxLengthMm = 130 - 5e-10
	setup.Stem.MaxSpacersMm = 30 - 5e-10
	fit := mustStockFit(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 351)
	if fit.Evaluated != 350 || len(fit.Candidates) != 350 {
		t.Fatalf("evaluated %d, returned %d; want 350 each", fit.Evaluated, len(fit.Candidates))
	}
	for _, c := range fit.Candidates {
		stem := c.Placement.Setup.Stem
		if stem.LengthMm > setup.Stem.MaxLengthMm || stem.SpacersMm > setup.Stem.MaxSpacersMm {
			t.Errorf("candidate exceeded its hardware limits: %+v", stem)
		}
	}
	if !slices.ContainsFunc(fit.Candidates, func(c assembly.StockCandidate) bool {
		stem := c.Placement.Setup.Stem
		return stem.LengthMm == setup.Stem.MaxLengthMm && stem.SpacersMm == setup.Stem.MaxSpacersMm
	}) {
		t.Error("no candidate landed exactly on both hardware limits")
	}
}

func TestStockFittingErrors(t *testing.T) {
	nan := pose.Point{X: math.NaN()}
	for _, tc := range []struct {
		name   string
		target assembly.FitTarget
		limit  int
		field  assembly.InputField
	}{
		{"missing grip", assembly.FitTarget{}, 5, ""},
		{"invalid grip", assembly.FitTarget{HandGrip: &nan}, 5, assembly.InputFitTargetGrip},
		{"invalid BRP", assembly.FitTarget{HandGrip: &fitReportGrip, BRP: &nan}, 5, assembly.InputFitTargetBRP},
		{"zero limit", assembly.FitTarget{HandGrip: &fitReportGrip}, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := assembly.ApplyStockFitting(fenrirFittingSetup(), tc.target, fenrirCockpitStock(), illustrativeCockpitPreferences(), tc.limit)
			if err == nil {
				t.Fatal("expected stock fitting error")
			}
			if tc.field != "" {
				var input *assembly.InputError
				if !errors.As(err, &input) || input.Field != tc.field {
					t.Errorf("error = %v, want InputError for %q", err, tc.field)
				}
			}
		})
	}

	base := fenrirFittingSetup()
	target := assembly.FitTarget{HandGrip: &fitReportGrip}
	stock, prefs := fenrirCockpitStock(), illustrativeCockpitPreferences()
	base.Stem.MinLengthMm = 0
	if _, err := assembly.ApplyStockFitting(base, target, stock, prefs, 5); err == nil {
		t.Error("accepted invalid stem length range")
	}
	for _, maxSpacers := range []float64{-1, math.Inf(1)} {
		base = fenrirFittingSetup()
		base.Stem.MaxSpacersMm = maxSpacers
		_, want := assembly.PlaceBike(base)
		if _, err := assembly.ApplyStockFitting(base, target, stock, prefs, 5); err == nil || want == nil || err.Error() != want.Error() {
			t.Errorf("invalid base error = %v, want %v", err, want)
		}
	}
}

func TestCockpitStockValidation(t *testing.T) {
	for _, step := range []float64{0, -5, math.NaN(), math.Inf(1)} {
		stock := fenrirCockpitStock()
		stock.StemLengthStepMm = step
		if stock.Validate() == nil {
			t.Errorf("accepted length step %v", step)
		}
		stock = fenrirCockpitStock()
		stock.SpacerStepMm = step
		if stock.Validate() == nil {
			t.Errorf("accepted spacer step %v", step)
		}
	}
	for _, angles := range [][]pose.Deg{nil, {pose.Deg(math.NaN())}} {
		stock := fenrirCockpitStock()
		stock.StemAnglesDeg = angles
		if stock.Validate() == nil {
			t.Errorf("accepted angles %v", angles)
		}
	}
	if err := twoBarStock().Validate(); err != nil {
		t.Errorf("valid handlebars: %v", err)
	}
	stock := twoBarStock()
	stock.Handlebars[0].BarRollDeg = 5
	if err := stock.Validate(); err == nil || !strings.Contains(err.Error(), "BarRollDeg") {
		t.Errorf("expected BarRollDeg error, got %v", err)
	}
	stock = twoBarStock()
	stock.Handlebars[0].GripInsetMm = 0
	_, want := assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0), stock.Handlebars[0])
	if err := stock.Validate(); want == nil || err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Errorf("stock grip inset error = %v, want %v", err, want)
	}
	stock = fenrirForkStock()
	if err := stock.Validate(); err != nil {
		t.Errorf("valid fork configurations: %v", err)
	}
	stock.Forks[1].Fork.SagPct = 150
	if err := stock.Validate(); err == nil || !strings.Contains(err.Error(), "stock fork 1: fork sag") {
		t.Errorf("expected stock fork sag error, got %v", err)
	}
	stock = fenrirForkStock()
	stock.Forks[0].LowerCup.StackMm = 0
	if err := stock.Validate(); err == nil || !strings.Contains(err.Error(), "stock fork 0 lower cup") {
		t.Errorf("expected stock lower cup error, got %v", err)
	}
}

func TestCockpitPreferencesValidation(t *testing.T) {
	for _, edit := range []func(*assembly.CockpitPreferences){
		func(p *assembly.CockpitPreferences) { p.GripOval.SemiMinorMm = 0 },
		func(p *assembly.CockpitPreferences) { p.GripOval.PointsPerR = -1 },
		func(p *assembly.CockpitPreferences) { p.StemLength.PointsPerUnitBelow = -1 },
		func(p *assembly.CockpitPreferences) { p.StemAngle.FreeMin = 1 },
		func(p *assembly.CockpitPreferences) { p.Spacers.FreeMax = math.NaN() },
	} {
		prefs := illustrativeCockpitPreferences()
		edit(&prefs)
		if prefs.Validate() == nil {
			t.Errorf("accepted preferences %+v", prefs)
		}
	}
	prefs := illustrativeCockpitPreferences()
	prefs.BarSweep.FreeMin = 21
	if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "bar sweep") {
		t.Errorf("expected bar sweep error, got %v", err)
	}
	prefs = illustrativeCockpitPreferences()
	prefs.ForkSag.FreeMin = 21
	if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "fork sag") {
		t.Errorf("expected fork sag error, got %v", err)
	}
	prefs = illustrativeCockpitPreferences()
	prefs.StemLength.PointsPerUnitBelow = -1
	if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "stem length band: points per unit must be finite and non-negative") {
		t.Errorf("expected stem length rate error, got %v", err)
	}
	prefs = illustrativeCockpitPreferences()
	prefs.GripOval.PointsPerR = -1
	if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "HandGrip oval: points per R must be finite and non-negative") {
		t.Errorf("expected oval rate error, got %v", err)
	}
}
