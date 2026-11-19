package assembly_test

import (
	"cmp"
	"errors"
	"math"
	"reflect"
	"slices"
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

func illustrativeCockpitPreferences() assembly.CockpitPreferences {
	return assembly.CockpitPreferences{
		GripOval:   assembly.FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60, CostPerR: 22},
		StemLength: assembly.PricedBand{FreeMin: 70, FreeMax: 80, CostBelow: 0.2, CostAbove: 0.2},
		StemAngle:  assembly.PricedBand{CostBelow: 0.2, CostAbove: 0.2},
		Spacers:    assembly.PricedBand{FreeMax: 20, CostAbove: 0.4},
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

func TestPricedBand(t *testing.T) {
	prefs := illustrativeCockpitPreferences()
	for _, tc := range []struct{ value, points float64 }{{60, 2}, {70, 0}, {75, 0}, {80, 0}, {130, 10}} {
		assertNear(t, "length points", prefs.StemLength.PointsLost(tc.value), tc.points, 1e-12)
	}
	assertNear(t, "spacer points", prefs.Spacers.PointsLost(40), 8, 1e-12)
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
	if fit.Evaluated != 450 || len(fit.Candidates) != 5 {
		t.Fatalf("evaluated %d, returned %d; want 450 and 5", fit.Evaluated, len(fit.Candidates))
	}
	best := fit.Candidates[0]
	requireStockStem(t, best, stockStem{0, 70, 10})
	if best.GripR >= 1e-9 || best.PointsLost() != 0 || best.Total() != 100 {
		t.Errorf("best build has R %v, points lost %v, total %v", best.GripR, best.PointsLost(), best.Total())
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
		total float64
	}{
		{stockStem{-17, 130, 0}, 78.711983},
		{stockStem{-17, 130, 5}, 69.982194},
		{stockStem{-17, 120, 0}, 62.174803},
	} {
		c := fit.Candidates[i]
		requireStockStem(t, c, want.stem)
		assertNear(t, "fit report total", c.Total(), want.total, 1e-5)
	}
	best := fit.Candidates[0]
	assertNear(t, "fit report radius", best.GripR, 1.358546, 1e-5)
	assertNear(t, "fit report miss", best.GripMiss().Norm(), 13.624, 1e-3)
	for i, points := range []float64{7.888017, 10, 3.4, 0} {
		assertNear(t, "fit report ledger", best.Ledger[i].PointsLost, points, 1e-5)
	}
}

func TestStockFittingForwardAndRankOrder(t *testing.T) {
	base := fenrirFittingSetup()
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 450)
	if fit.Evaluated != 450 || len(fit.Candidates) != 450 {
		t.Fatalf("evaluated %d, returned %d; want 450 each", fit.Evaluated, len(fit.Candidates))
	}
	wantBands := []assembly.LedgerBand{assembly.BandGripOval, assembly.BandStemLength, assembly.BandStemAngle, assembly.BandSpacers}
	for i, c := range fit.Candidates {
		p, err := assembly.PlaceBike(c.Placement.Setup, nil)
		if err != nil || !reflect.DeepEqual(c.Placement, p) {
			t.Fatalf("rank %d placement differs from PlaceBike: %v", i+1, err)
		}
		setup := c.Placement.Setup
		setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = base.Stem.AngleDeg, base.Stem.LengthMm, base.Stem.SpacersMm
		if !reflect.DeepEqual(setup, base) {
			t.Errorf("rank %d changed fixed inputs", i+1)
		}
		var bands []assembly.LedgerBand
		for _, line := range c.Ledger {
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
	if total := fit.Candidates[449].Total(); total >= 0 {
		t.Errorf("last total = %v, want a negative total", total)
	}
}

func TestStockFittingIndependentLedger(t *testing.T) {
	base := fenrirFittingSetup()
	for _, grip := range []pose.Point{fitReportGrip, gripOf(t, base)} {
		type pricedStem struct {
			stem      stockStem
			points, r float64
		}
		var oracle []pricedStem
		for _, angle := range []pose.Deg{-17, -6, 0, 6, 17} {
			for length := 40.0; length <= 130; length += 10 {
				for spacers := 0.0; spacers <= 40; spacers += 5 {
					setup := base
					setup.Stem.AngleDeg, setup.Stem.LengthMm, setup.Stem.SpacersMm = angle, length, spacers
					p := gripOf(t, setup)
					r := legacyGripRadius(p.X-grip.X, p.Y-grip.Y)
					points := 22*max(0, r-1) + 0.2*max(0, 70-length, length-80) + 0.2*math.Abs(float64(angle)) + 0.4*max(0, spacers-20)
					oracle = append(oracle, pricedStem{stockStem{angle, length, spacers}, points, r})
				}
			}
		}
		slices.SortStableFunc(oracle, func(a, b pricedStem) int {
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
	both := mustStockFit(t, base, assembly.FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}, 450)
	gripOnly := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 450)
	for i, c := range both.Candidates {
		got := c.Placement.Setup
		if got.Seatpost.ExtensionMm != saddle.Seatpost.ExtensionMm || got.Saddle.RailOffsetMm != saddle.Saddle.RailOffsetMm {
			t.Errorf("rank %d doesn't carry the saddle fitting", i+1)
		}
		requireStockStem(t, c, stockStemOf(gripOnly.Candidates[i].Placement.Setup.Stem))
	}
}

func TestStockFittingZeroSpacers(t *testing.T) {
	base := fenrirFittingSetup()
	base.Stem.SpacersMm, base.Stem.MaxSpacersMm = 0, 0
	fit := mustStockFit(t, base, assembly.FitTarget{HandGrip: &fitReportGrip}, 450)
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
	setup.Stem.MaxSpacersMm = 40 - 5e-10
	fit := mustStockFit(t, setup, assembly.FitTarget{HandGrip: &fitReportGrip}, 451)
	if fit.Evaluated != 450 || len(fit.Candidates) != 450 {
		t.Fatalf("evaluated %d, returned %d; want 450 each", fit.Evaluated, len(fit.Candidates))
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
	base = fenrirFittingSetup()
	base.Stem.MaxSpacersMm = -1
	_, want := assembly.PlaceBike(base, nil)
	if _, err := assembly.ApplyStockFitting(base, target, stock, prefs, 5); err == nil || want == nil || err.Error() != want.Error() {
		t.Errorf("invalid base error = %v, want %v", err, want)
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
}

func TestCockpitPreferencesValidation(t *testing.T) {
	for _, edit := range []func(*assembly.CockpitPreferences){
		func(p *assembly.CockpitPreferences) { p.GripOval.SemiMinorMm = 0 },
		func(p *assembly.CockpitPreferences) { p.GripOval.CostPerR = -1 },
		func(p *assembly.CockpitPreferences) { p.StemLength.CostBelow = -1 },
		func(p *assembly.CockpitPreferences) { p.StemAngle.FreeMin = 1 },
		func(p *assembly.CockpitPreferences) { p.Spacers.FreeMax = math.NaN() },
	} {
		prefs := illustrativeCockpitPreferences()
		edit(&prefs)
		if prefs.Validate() == nil {
			t.Errorf("accepted preferences %+v", prefs)
		}
	}
}
