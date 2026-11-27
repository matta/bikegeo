package assembly_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// fenrirFittingSetup is the default Fenrir build (Lithic Carbon Mountain fork,
// EC44 cup). Numbers copied from the web fixture it replaced; assembly tests
// must not import web code.
func fenrirFittingSetup() assembly.BikeSetup {
	return assembly.BikeSetup{
		Frame:    fenrirSizeLFrame(),
		LowerCup: fenrirEc44Cup(),
		Fork: assembly.ForkSetup{
			Spec:   assembly.ForkSpec{StaticA2cMm: 495.0, Travel: assembly.TravelRigid, RakeMm: 54.0, Steerer: assembly.SteererTapered},
			SagPct: 0.0,
		},
		Wheels:    fenrirWheels(),
		Seatpost:  assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 180.576},
		Saddle:    sqlab610DefaultSaddle(),
		Stem:      fenrirDefaultStem(),
		Handlebar: persuader800DefaultHandlebar(),
	}
}

// fitReportBRP is Matt's idmatch fit report BRP (docs/RIDER_FIT.md).
var fitReportBRP = pose.NewPoint(assembly.DynamicWorld, -219, 713.1)

func brpOf(t *testing.T, setup assembly.BikeSetup) pose.Point {
	t.Helper()
	p, err := assembly.PlaceBike(setup)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	return p.Saddle.BRP.Origin()
}

func mustFit(t *testing.T, setup assembly.BikeSetup, target pose.Point) assembly.Fitting {
	t.Helper()
	fit, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &target})
	if err != nil {
		t.Fatalf("ApplyFitting: %v", err)
	}
	return fit
}

// brpMissMm returns |BrpMiss()| and fails when the fitting has no BRP target.
func brpMissMm(t *testing.T, f assembly.Fitting) float64 {
	t.Helper()
	miss, ok := f.BrpMiss()
	if !ok {
		t.Fatal("fitting has no BRP target")
	}
	return miss.Norm()
}

func fittedValues(f assembly.Fitting) (ext, rail float64) {
	return f.Placement.Setup.Seatpost.ExposedMm(), f.Placement.Setup.Saddle.RailOffsetMm
}

func TestApplyFittingRoundTripReached(t *testing.T) {
	known := fenrirFittingSetup()
	known.Seatpost.InsertionMm = fenrirPostTotalMm - (200)
	known.Saddle.RailOffsetMm = 5
	target := brpOf(t, known)

	fit := mustFit(t, fenrirFittingSetup(), target)
	ext, rail := fittedValues(fit)
	if fit.Outcome != assembly.FitReached {
		t.Fatalf("Outcome = %v, want reached", fit.Outcome)
	}
	assertNear(t, "extension", ext, 200, 1e-9)
	assertNear(t, "rail offset", rail, 5, 1e-9)
	assertNear(t, "miss", brpMissMm(t, fit), 0, 1e-9)
	if fit.LimitedBy != nil {
		t.Fatalf("LimitedBy = %v, want nil", fit.LimitedBy)
	}
}

// A target placed by a build whose settings sit exactly on hardware bounds is
// reachable: round-off in the decomposition must not report it as limited.
func TestApplyFittingReachesTargetsOnBounds(t *testing.T) {
	for _, c := range []struct{ ext, rail float64 }{
		{300, 15}, {110, 15}, {300, -15}, {110, -15}, {300, 0}, {110, 0}, {200, 15}, {200, -15},
	} {
		known := fenrirFittingSetup()
		known.Seatpost.InsertionMm = fenrirPostTotalMm - (c.ext)
		known.Saddle.RailOffsetMm = c.rail
		fit := mustFit(t, fenrirFittingSetup(), brpOf(t, known))
		ext, rail := fittedValues(fit)
		if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
			t.Errorf("ext %v rail %v: Outcome = %v, LimitedBy = %v, want reached", c.ext, c.rail, fit.Outcome, fit.LimitedBy)
		}
		assertNear(t, "extension", ext, c.ext, 1e-9)
		assertNear(t, "rail offset", rail, c.rail, 1e-9)
		assertNear(t, "miss", brpMissMm(t, fit), 0, 1e-9)
	}
}

func TestApplyFittingFitReportBRPLimitedByRailOffset(t *testing.T) {
	fit := mustFit(t, fenrirFittingSetup(), fitReportBRP)
	ext, rail := fittedValues(fit)
	if fit.Outcome != assembly.FitLimited {
		t.Fatalf("Outcome = %v, want limited", fit.Outcome)
	}
	if want := []assembly.InputField{assembly.InputSaddleRailOffset}; !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	assertNear(t, "rail offset", rail, 15, 0)
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 197.387571, 1e-6)
	assertNear(t, "extension", ext, fenrirPostTotalMm-197.387571, 1e-6)
	miss, _ := fit.BrpMiss()
	assertNear(t, "miss.X", miss.X(), -53.368222, 1e-6)
	assertNear(t, "miss.Y", miss.Y(), 0, 1e-9)
	assertNear(t, "|miss|", brpMissMm(t, fit), 53.368222, 1e-6)

	// Setback moves the cradle along local +Y (195.29°), lowering it.
	// Preserving BRP height pulls the post out to 190.006853 mm insertion.
	setback := fenrirFittingSetup()
	setback.Seatpost.Spec.SetbackMm = 27
	fit27 := mustFit(t, setback, fitReportBRP)
	_, rail27 := fittedValues(fit27)
	if want := []assembly.InputField{assembly.InputSaddleRailOffset}; !reflect.DeepEqual(fit27.LimitedBy, want) {
		t.Fatalf("setback LimitedBy = %v, want %v", fit27.LimitedBy, want)
	}
	assertNear(t, "setback rail offset", rail27, 15, 0)
	assertNear(t, "setback insertion", fit27.Placement.Setup.Seatpost.InsertionMm, 190.006853, 1e-6)
	miss27, _ := fit27.BrpMiss()
	assertNear(t, "setback miss.Y", miss27.Y(), 0, 1e-9)
	assertNear(t, "setback |miss|", brpMissMm(t, fit27), 25.377598, 1e-6)
}

func TestApplyFittingExtensionLimits(t *testing.T) {
	base := fenrirFittingSetup()
	baseBRP := brpOf(t, base)
	// The default Fenrir seat axis heading is 105.289°.
	seat := 105.289 * math.Pi / 180

	above := mustFit(t, base, pose.NewPoint(assembly.DynamicWorld, baseBRP.X()+200*math.Cos(seat), baseBRP.Y()+200*math.Sin(seat)))
	ext, _ := fittedValues(above)
	if len(above.LimitedBy) == 0 || above.LimitedBy[0] != assembly.InputSeatpostInsertion {
		t.Fatalf("above LimitedBy = %v, want it to start with the seatpost extension", above.LimitedBy)
	}
	assertNear(t, "above extension", ext, 300, 0)

	below := mustFit(t, base, pose.NewPoint(assembly.DynamicWorld, baseBRP.X()-200*math.Cos(seat), baseBRP.Y()-200*math.Sin(seat)))
	ext, _ = fittedValues(below)
	if len(below.LimitedBy) == 0 || below.LimitedBy[0] != assembly.InputSeatpostInsertion {
		t.Fatalf("below LimitedBy = %v, want it to start with the seatpost extension", below.LimitedBy)
	}
	assertNear(t, "below extension", ext, 110, 0)
}

func TestApplyFittingBothLimitsInOrder(t *testing.T) {
	base := fenrirFittingSetup()
	baseBRP := brpOf(t, base)
	seat := 105.289 * math.Pi / 180
	// 200 mm up the seat axis plus 50 mm of rail offset (level rails: BRP moves -X).
	target := pose.NewPoint(assembly.DynamicWorld, baseBRP.X()+200*math.Cos(seat)-50, baseBRP.Y()+200*math.Sin(seat))
	fit := mustFit(t, base, target)
	want := []assembly.InputField{assembly.InputSeatpostInsertion, assembly.InputSaddleRailOffset}
	if !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	ext, rail := fittedValues(fit)
	assertNear(t, "extension", ext, 300, 0)
	assertNear(t, "rail offset", rail, 15, 0)
}

func TestApplyFittingErrors(t *testing.T) {
	var inputErr *assembly.InputError
	if _, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{}); err == nil {
		t.Fatal("empty FitTarget: want error")
	}
	for _, bad := range []pose.Point{pose.NewPoint(assembly.DynamicWorld, math.NaN(), 700), pose.NewPoint(assembly.DynamicWorld, -200, math.Inf(1))} {
		_, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{BRP: &bad})
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetBRP {
			t.Fatalf("target %v: err = %v, want InputFitTargetBRP", bad, err)
		}
	}

	longClamp := fenrirFittingSetup()
	longClamp.Seatpost.Spec.RailClampLengthMm = 70
	if _, err := assembly.ApplyFitting(longClamp, assembly.FitTarget{BRP: &fitReportBRP}); err == nil {
		t.Fatal("clamp longer than rails: want error")
	}

	badFrame := fenrirFittingSetup()
	badFrame.Frame.ReachMm = math.NaN()
	_, fitErr := assembly.ApplyFitting(badFrame, assembly.FitTarget{BRP: &fitReportBRP})
	_, placeErr := assembly.PlaceBike(badFrame)
	if fitErr == nil || placeErr == nil || fitErr.Error() != placeErr.Error() {
		t.Fatalf("fit error %v, want PlaceBike error %v", fitErr, placeErr)
	}
}

var (
	gateTradeOffBRP     = pose.NewPoint(assembly.DynamicWorld, -175.242919, 710.580028)
	pitchedInsertionBRP = pose.NewPoint(assembly.DynamicWorld, -180.355237, 819.658647)
)

func TestApplyFittingIsMinimalOverGrid(t *testing.T) {
	const tol = 1e-6
	inGate := func(miss pose.Vector) bool {
		return math.Abs(miss.Y()) <= assembly.FitGateBrpYMm+1e-9 && math.Abs(miss.X()) <= assembly.FitGateBrpXMm+1e-9
	}
	for _, tc := range []struct {
		name   string
		target pose.Point
		pitch  pose.Deg
	}{
		{"fit report outside gate", fitReportBRP, 0},
		{"gate trade-off", gateTradeOffBRP, 0},
		{"pitched rails at insertion bound", pitchedInsertionBRP, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setup := fenrirFittingSetup()
			setup.Saddle.RailPitchDeg = tc.pitch
			fit := mustFit(t, setup, tc.target)
			if fit.Outcome != assembly.FitLimited {
				t.Fatalf("Outcome = %v, want limited", fit.Outcome)
			}
			best, _ := fit.BrpMiss()
			bestInGate := inGate(best)
			for ext := 110.0; ext <= 300; ext++ {
				for rail := -15.0; rail <= 15; rail++ {
					setup.Seatpost.InsertionMm = fenrirPostTotalMm - ext
					setup.Saddle.RailOffsetMm = rail
					miss := tc.target.Sub(brpOf(t, setup))
					dy := math.Abs(miss.Y()) - math.Abs(best.Y())
					dx := math.Abs(miss.X()) - math.Abs(best.X())
					gridInGate := inGate(miss)
					if (gridInGate && !bestInGate) || (gridInGate == bestInGate && (dy < -tol || (math.Abs(dy) <= tol && dx < -tol))) {
						t.Fatalf("grid point ext %v rail %v misses by %v, beating the fitting's %v", ext, rail, miss, best)
					}
				}
			}
		})
	}
}

func TestApplyFittingGateAwareTradeOff(t *testing.T) {
	// At 200 mm insertion and +15 mm rail offset, BRP is (-164.942919, 710.580028).
	// A target 10.3 mm behind that point permits a small height trade-off.
	// At 10.6 mm behind, entering the gate would require over 2 mm of height miss.
	for _, tc := range []struct {
		name      string
		target    pose.Point
		insertion float64
		missX     float64
		missY     float64
		inGate    bool
	}{
		{"gate passing", gateTradeOffBRP, 198.862281, -10, -1.097454, true},
		{"outside gate", pose.NewPoint(assembly.DynamicWorld, -175.542919, 710.580028), 200, -10.6, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fit := mustFit(t, fenrirFittingSetup(), tc.target)
			want := []assembly.InputField{assembly.InputSaddleRailOffset}
			if fit.Outcome != assembly.FitLimited || !reflect.DeepEqual(fit.LimitedBy, want) {
				t.Fatalf("Outcome = %v, LimitedBy = %v, want limited by %v", fit.Outcome, fit.LimitedBy, want)
			}
			assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, tc.insertion, 1e-5)
			assertNear(t, "rail offset", fit.Placement.Setup.Saddle.RailOffsetMm, 15, 0)
			miss, _ := fit.BrpMiss()
			if tc.inGate {
				assertNear(t, "miss.X", miss.X(), tc.missX, 1e-6)
				assertNear(t, "miss.Y", miss.Y(), tc.missY, 1e-5)
				if math.Abs(miss.Y()) > assembly.FitGateBrpYMm || math.Abs(miss.X()) > assembly.FitGateBrpXMm+1e-9 {
					t.Errorf("miss %v fails the BRP gate", miss)
				}
			} else {
				assertNear(t, "miss.X", miss.X(), tc.missX, 1e-5)
				assertNear(t, "miss.Y", miss.Y(), tc.missY, 1e-9)
			}
		})
	}
}

func TestApplyFittingPitchedRailsInsertionBound(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Saddle.RailPitchDeg = 5
	// At minimum insertion, moving the pitched rails forward raises the BRP
	// toward the target while increasing the horizontal miss.
	fit := mustFit(t, setup, pitchedInsertionBRP)
	want := []assembly.InputField{assembly.InputSeatpostInsertion, assembly.InputSaddleRailOffset}
	if fit.Outcome != assembly.FitLimited || !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want limited by %v", fit.Outcome, fit.LimitedBy, want)
	}
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 100, 0)
	assertNear(t, "rail offset", fit.Placement.Setup.Saddle.RailOffsetMm, -15, 0)
	miss, _ := fit.BrpMiss()
	assertNear(t, "miss.X", miss.X(), -14.942920, 1e-5)
	assertNear(t, "miss.Y", miss.Y(), 8.692664, 1e-5)
}

func TestApplyFittingDegenerateBox(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Frame.MaxPostInsertionMm = 100
	setup.Seatpost.InsertionMm = 100
	setup.Saddle.RailLengthMm = 30
	setup.Seatpost.Spec.RailClampLengthMm = 30
	brp := brpOf(t, setup)

	fit := mustFit(t, setup, pose.NewPoint(assembly.DynamicWorld, brp.X()-20, brp.Y()+5))
	want := []assembly.InputField{assembly.InputSeatpostInsertion, assembly.InputSaddleRailOffset}
	if fit.Outcome != assembly.FitLimited || !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want limited by %v", fit.Outcome, fit.LimitedBy, want)
	}
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 100, 0)
	assertNear(t, "rail offset", fit.Placement.Setup.Saddle.RailOffsetMm, 0, 0)
	miss := brpMissMm(t, fit)
	if math.IsNaN(miss) || math.IsInf(miss, 0) {
		t.Fatalf("miss = %v, want finite", miss)
	}

	fit = mustFit(t, setup, brp)
	if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "reachable miss", brpMissMm(t, fit), 0, 1e-9)
}

func TestFitGateConstants(t *testing.T) {
	assertNear(t, "HandGrip normalized radius", assembly.FitGateGripNormalizedRadius, 1, 0)
	assertNear(t, "BRP Y", assembly.FitGateBrpYMm, 2, 0)
	assertNear(t, "BRP X", assembly.FitGateBrpXMm, 10, 0)
}

func TestApplyFittingCopiesTarget(t *testing.T) {
	target := fitReportBRP
	fit := mustFit(t, fenrirFittingSetup(), target)
	if fit.Target.BRP == &target {
		t.Fatal("Fitting.Target.BRP aliases the caller's point")
	}
	target = pose.NewPoint(assembly.DynamicWorld, 1000, target.Y())
	if *fit.Target.BRP != fitReportBRP {
		t.Fatalf("Target.BRP = %v after caller mutation, want %v", *fit.Target.BRP, fitReportBRP)
	}
}

// fitReportGrip is the idmatch fit report handlebar point (docs/RIDER_FIT.md).
// Whether the report point is HandGrip or the stem clamp is unresolved.
var fitReportGrip = pose.NewPoint(assembly.DynamicWorld, 481, 726)

func gripPlacement(t *testing.T, setup assembly.BikeSetup) assembly.BikePlacement {
	t.Helper()
	p, err := assembly.PlaceBike(setup)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	return p
}

func gripOf(t *testing.T, setup assembly.BikeSetup) pose.Point {
	t.Helper()
	return gripPlacement(t, setup).Handlebar.HandGrip.Origin()
}

func withStem(setup assembly.BikeSetup, spacers, length float64) assembly.BikeSetup {
	setup.Stem.SpacersMm = spacers
	setup.Stem.LengthMm = length
	return setup
}

func mustFitGrip(t *testing.T, setup assembly.BikeSetup, grip pose.Point) assembly.Fitting {
	t.Helper()
	fit, err := assembly.ApplyFitting(setup, assembly.FitTarget{HandGrip: &grip})
	if err != nil {
		t.Fatalf("ApplyFitting: %v", err)
	}
	return fit
}

func gripMissMm(t *testing.T, f assembly.Fitting) float64 {
	t.Helper()
	miss, ok := f.GripMiss()
	if !ok {
		t.Fatal("fitting has no HandGrip target")
	}
	return miss.Norm()
}

func stemValues(f assembly.Fitting) (spacers, length float64) {
	return f.Placement.Setup.Stem.SpacersMm, f.Placement.Setup.Stem.LengthMm
}

func TestApplyFittingGripRoundTripReached(t *testing.T) {
	known := withStem(fenrirFittingSetup(), 10, 90)
	fit := mustFitGrip(t, fenrirFittingSetup(), gripOf(t, known))
	spacers, length := stemValues(fit)
	if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "spacers", spacers, 10, 1e-9)
	assertNear(t, "length", length, 90, 1e-9)
	assertNear(t, "miss", gripMissMm(t, fit), 0, 1e-9)
	in := fenrirFittingSetup()
	if fit.Placement.Setup.Seatpost != in.Seatpost || fit.Placement.Setup.Saddle != in.Saddle {
		t.Fatal("seatpost or saddle changed without a BRP target")
	}
}

func TestApplyFittingGripReachesTargetsOnBounds(t *testing.T) {
	for _, c := range []struct{ spacers, length float64 }{{0, 40}, {30, 130}, {0, 130}} {
		fit := mustFitGrip(t, fenrirFittingSetup(), gripOf(t, withStem(fenrirFittingSetup(), c.spacers, c.length)))
		spacers, length := stemValues(fit)
		if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
			t.Errorf("%v: Outcome = %v, LimitedBy = %v, want reached", c, fit.Outcome, fit.LimitedBy)
		}
		assertNear(t, "spacers", spacers, c.spacers, 1e-9)
		assertNear(t, "length", length, c.length, 1e-9)
	}
}

func TestApplyFittingFitReportGripLimitedByBoth(t *testing.T) {
	for _, c := range []struct {
		angle         pose.Deg
		x, y, absMiss float64
	}{
		{-6, 17.007, -30.248, 34.701},
		{-17, 12.342, -5.769, 13.624},
	} {
		setup := fenrirFittingSetup()
		setup.Stem.AngleDeg = c.angle
		fit := mustFitGrip(t, setup, fitReportGrip)
		want := []assembly.InputField{assembly.InputStemSpacers, assembly.InputStemLength}
		if fit.Outcome != assembly.FitLimited || !reflect.DeepEqual(fit.LimitedBy, want) {
			t.Fatalf("angle %v: Outcome = %v, LimitedBy = %v, want %v", c.angle, fit.Outcome, fit.LimitedBy, want)
		}
		spacers, length := stemValues(fit)
		assertNear(t, "spacers", spacers, 0, 0)
		assertNear(t, "length", length, 130, 0)
		miss, _ := fit.GripMiss()
		assertNear(t, "miss.X", miss.X(), c.x, 0.01)
		assertNear(t, "miss.Y", miss.Y(), c.y, 0.01)
		assertNear(t, "|miss|", miss.Norm(), c.absMiss, 0.01)
	}
}

// unitAt returns the unit vector at a heading, using independent math.
func unitAt(h pose.Deg) pose.Vector {
	r := float64(h) * math.Pi / 180
	return pose.NewVector(assembly.DynamicWorld, math.Cos(r), math.Sin(r))
}

func TestApplyFittingGripSpacersLimitOnly(t *testing.T) {
	base := withStem(fenrirFittingSetup(), 0, 100)
	u := unitAt(gripPlacement(t, base).Stem.SpacerTop.HeadingDeg())
	g := gripOf(t, base)
	target := pose.NewPoint(assembly.DynamicWorld, g.X()-20*u.X(), g.Y()-20*u.Y())
	fit := mustFitGrip(t, fenrirFittingSetup(), target)
	if want := []assembly.InputField{assembly.InputStemSpacers}; !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	spacers, length := stemValues(fit)
	assertNear(t, "spacers", spacers, 0, 0)
	assertNear(t, "length", length, 100-20*math.Cos(96*math.Pi/180), 1e-6)
	assertNear(t, "|miss|", gripMissMm(t, fit), 20*math.Sin(96*math.Pi/180), 1e-6)

	// Minimality over a 1 mm forward-placement grid.
	best := gripMissMm(t, fit)
	setup := fenrirFittingSetup()
	for sp := 0.0; sp <= 30; sp++ {
		for ln := 40.0; ln <= 130; ln++ {
			g := gripOf(t, withStem(setup, sp, ln))
			if miss := math.Hypot(target.X()-g.X(), target.Y()-g.Y()); miss < best-1e-6 {
				t.Fatalf("grid spacers %v length %v misses by %v, beating the fitting's %v", sp, ln, miss, best)
			}
		}
	}
}

func TestApplyFittingGripLengthLimitOnly(t *testing.T) {
	base := withStem(fenrirFittingSetup(), 20, 130)
	v := unitAt(gripPlacement(t, base).Stem.BarClamp.HeadingDeg())
	g := gripOf(t, base)
	target := pose.NewPoint(assembly.DynamicWorld, g.X()+15*v.X(), g.Y()+15*v.Y())
	fit := mustFitGrip(t, fenrirFittingSetup(), target)
	if want := []assembly.InputField{assembly.InputStemLength}; !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	spacers, length := stemValues(fit)
	assertNear(t, "length", length, 130, 0)
	assertNear(t, "spacers", spacers, 20+15*math.Cos(96*math.Pi/180), 1e-6)
	assertNear(t, "|miss|", gripMissMm(t, fit), 15*math.Sin(96*math.Pi/180), 1e-6)
}

func TestApplyFittingGripKeepsFixedInputs(t *testing.T) {
	in := fenrirFittingSetup()
	fit := mustFitGrip(t, in, fitReportGrip)
	got := fit.Placement.Setup
	if got.Stem.AngleDeg != in.Stem.AngleDeg || got.Stem.UpperStackMm != in.Stem.UpperStackMm ||
		got.Stem.ClampHeightMm != in.Stem.ClampHeightMm || got.Handlebar != in.Handlebar {
		t.Fatalf("fixed cockpit inputs changed: %+v", got.Stem)
	}
}

func TestApplyFittingBothLandmarksMatchSingleFits(t *testing.T) {
	setup := fenrirFittingSetup()
	brp, grip := fitReportBRP, fitReportGrip
	both, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &brp, HandGrip: &grip})
	if err != nil {
		t.Fatal(err)
	}
	want := []assembly.InputField{assembly.InputSaddleRailOffset, assembly.InputStemSpacers, assembly.InputStemLength}
	if !reflect.DeepEqual(both.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", both.LimitedBy, want)
	}
	saddle := mustFit(t, setup, fitReportBRP)
	cockpit := mustFitGrip(t, setup, fitReportGrip)
	ext, rail := fittedValues(both)
	sExt, sRail := fittedValues(saddle)
	spacers, length := stemValues(both)
	cSpacers, cLength := stemValues(cockpit)
	if ext != sExt || rail != sRail || spacers != cSpacers || length != cLength {
		t.Fatalf("combined (%v %v %v %v) differs from single fits (%v %v %v %v)", ext, rail, spacers, length, sExt, sRail, cSpacers, cLength)
	}
}

func TestApplyFittingGripErrors(t *testing.T) {
	var inputErr *assembly.InputError
	for _, bad := range []pose.Point{pose.NewPoint(assembly.DynamicWorld, math.NaN(), 700), pose.NewPoint(assembly.DynamicWorld, 400, math.Inf(-1))} {
		_, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{HandGrip: &bad})
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetGrip {
			t.Fatalf("target %v: err = %v, want InputFitTargetGrip", bad, err)
		}
	}

	grip, brp := fitReportGrip, fitReportBRP
	for name, mutate := range map[string]func(*assembly.StemSetup){
		"zero min":  func(s *assembly.StemSetup) { s.MinLengthMm = 0 },
		"min > max": func(s *assembly.StemSetup) { s.MinLengthMm, s.MaxLengthMm = 100, 90 },
		"NaN max":   func(s *assembly.StemSetup) { s.MaxLengthMm = math.NaN() },
	} {
		setup := fenrirFittingSetup()
		mutate(&setup.Stem)
		if _, err := assembly.ApplyFitting(setup, assembly.FitTarget{HandGrip: &grip}); err == nil {
			t.Errorf("%s with grip target: want error", name)
		}
		if _, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &brp}); err != nil {
			t.Errorf("%s with only BRP target: %v", name, err)
		}
	}

	bad := fenrirFittingSetup()
	bad.Stem.MaxSpacersMm = -1
	_, fitErr := assembly.ApplyFitting(bad, assembly.FitTarget{HandGrip: &grip})
	_, placeErr := assembly.PlaceBike(bad)
	if fitErr == nil || placeErr == nil || fitErr.Error() != placeErr.Error() {
		t.Fatalf("fit error %v, want PlaceBike error %v", fitErr, placeErr)
	}
}

func TestFitPostSetbackCentersRails(t *testing.T) {
	fit, err := assembly.FitPostSetback(fenrirFittingSetup(), fitReportBRP, 0)
	if err != nil {
		t.Fatalf("FitPostSetback: %v", err)
	}
	if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "setback", fit.Placement.Setup.Seatpost.Spec.SetbackMm, 65.948584, 1e-6)
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 179.359870, 1e-6)
	assertNear(t, "miss", brpMissMm(t, fit), 0, 1e-9)
	if fit.Placement.Setup.Saddle.RailOffsetMm != 0 {
		t.Fatalf("RailOffsetMm = %v, want 0", fit.Placement.Setup.Saddle.RailOffsetMm)
	}
}

func TestFitPostSetbackAtRailLimit(t *testing.T) {
	fit, err := assembly.FitPostSetback(fenrirFittingSetup(), fitReportBRP, 15)
	if err != nil {
		t.Fatalf("FitPostSetback: %v", err)
	}
	if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "setback", fit.Placement.Setup.Seatpost.Spec.SetbackMm, 51.479453, 1e-6)
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 183.315151, 1e-6)
	assertNear(t, "miss", brpMissMm(t, fit), 0, 1e-9)
}

func TestFitPostSetbackIgnoresSubmittedSeatInputs(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Seatpost.Spec.SetbackMm = 25
	setup.Seatpost.InsertionMm = fenrirPostTotalMm - (200)
	setup.Saddle.RailOffsetMm = -10

	got, err := assembly.FitPostSetback(setup, fitReportBRP, 0)
	if err != nil {
		t.Fatalf("FitPostSetback: %v", err)
	}
	want, err := assembly.FitPostSetback(fenrirFittingSetup(), fitReportBRP, 0)
	if err != nil {
		t.Fatalf("FitPostSetback default: %v", err)
	}
	assertNear(t, "setback", got.Placement.Setup.Seatpost.Spec.SetbackMm, want.Placement.Setup.Seatpost.Spec.SetbackMm, 1e-9)
	assertNear(t, "extension", got.Placement.Setup.Seatpost.ExposedMm(), want.Placement.Setup.Seatpost.ExposedMm(), 1e-9)
}

func TestFitPostSetbackStopsAtZeroSetback(t *testing.T) {
	target := pose.NewPoint(assembly.DynamicWorld, -150, 758.1) // the original 713.1 target, translated up the 45 mm BRP height
	fit, err := assembly.FitPostSetback(fenrirFittingSetup(), target, 0)
	if err != nil {
		t.Fatalf("FitPostSetback: %v", err)
	}
	if fit.Outcome != assembly.FitLimited {
		t.Fatalf("Outcome = %v, want limited", fit.Outcome)
	}
	if want := []assembly.InputField{assembly.InputSeatpostSetback}; !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	if fit.Placement.Setup.Seatpost.Spec.SetbackMm != 0 {
		t.Fatalf("PostSetbackMm = %v, want 0", fit.Placement.Setup.Seatpost.Spec.SetbackMm)
	}
	assertNear(t, "extension", fit.Placement.Setup.Seatpost.ExposedMm(), 245.853232, 1e-5)
	assertNear(t, "miss", brpMissMm(t, fit), 12.475261, 1e-5)
}

func TestFitPostSetbackForwardTarget(t *testing.T) {
	target := pose.NewPoint(assembly.DynamicWorld, -150, 758.1) // the original 713.1 target, translated up the 45 mm BRP height
	fit, err := assembly.FitPostSetback(fenrirFittingSetup(), target, -15)
	if err != nil {
		t.Fatalf("FitPostSetback: %v", err)
	}
	if fit.Outcome != assembly.FitReached || fit.LimitedBy != nil {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "setback", fit.Placement.Setup.Seatpost.Spec.SetbackMm, 1.993870, 1e-6)
	assertNear(t, "extension", fit.Placement.Setup.Seatpost.ExposedMm(), 249.808512, 1e-6)
	assertNear(t, "miss", brpMissMm(t, fit), 0, 1e-9)
}

func TestFitPostSetbackStopsAtExtension(t *testing.T) {
	target := pose.NewPoint(assembly.DynamicWorld, -219, 945) // the original 900 target, translated up the 45 mm BRP height

	fit0, err := assembly.FitPostSetback(fenrirFittingSetup(), target, 0)
	if err != nil {
		t.Fatalf("FitPostSetback rail 0: %v", err)
	}
	if fit0.Outcome != assembly.FitLimited {
		t.Fatalf("rail 0 Outcome = %v, want limited", fit0.Outcome)
	}
	if want := []assembly.InputField{assembly.InputSeatpostInsertion}; !reflect.DeepEqual(fit0.LimitedBy, want) {
		t.Fatalf("rail 0 LimitedBy = %v, want %v", fit0.LimitedBy, want)
	}
	if fit0.Placement.Setup.Seatpost.ExposedMm() != 300 {
		t.Fatalf("rail 0 ExtensionMm = %v, want 300", fit0.Placement.Setup.Seatpost.ExposedMm())
	}
	assertNear(t, "rail 0 setback", fit0.Placement.Setup.Seatpost.Spec.SetbackMm, 4.799945, 1e-5)
	assertNear(t, "rail 0 miss", brpMissMm(t, fit0), 144.332896, 1e-5)

	fit15, err := assembly.FitPostSetback(fenrirFittingSetup(), target, 15)
	if err != nil {
		t.Fatalf("FitPostSetback rail 15: %v", err)
	}
	if fit15.Outcome != assembly.FitLimited {
		t.Fatalf("rail 15 Outcome = %v, want limited", fit15.Outcome)
	}
	if want := []assembly.InputField{assembly.InputSeatpostInsertion, assembly.InputSeatpostSetback}; !reflect.DeepEqual(fit15.LimitedBy, want) {
		t.Fatalf("rail 15 LimitedBy = %v, want %v", fit15.LimitedBy, want)
	}
	if fit15.Placement.Setup.Seatpost.ExposedMm() != 300 || fit15.Placement.Setup.Seatpost.Spec.SetbackMm != 0 {
		t.Fatalf("rail 15 got ext=%v setback=%v, want 300, 0", fit15.Placement.Setup.Seatpost.ExposedMm(), fit15.Placement.Setup.Seatpost.Spec.SetbackMm)
	}
	assertNear(t, "rail 15 miss", brpMissMm(t, fit15), 140.710227, 1e-6)
	saddleFit := mustFit(t, fenrirFittingSetup(), target)
	assertNear(t, "rail 15 cross-check miss", brpMissMm(t, fit15), brpMissMm(t, saddleFit), 1e-9)
}

func TestFitPostSetbackRejectsRailBeyondClamp(t *testing.T) {
	var inputErr *assembly.InputError
	_, err := assembly.FitPostSetback(fenrirFittingSetup(), fitReportBRP, 15.5)
	if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSaddleRailOffset {
		t.Fatalf("rail 15.5: err = %v, want InputSaddleRailOffset", err)
	}

	_, err = assembly.FitPostSetback(fenrirFittingSetup(), pose.NewPoint(assembly.DynamicWorld, math.NaN(), 713.1), 0)
	if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetBRP {
		t.Fatalf("NaN BRP: err = %v, want InputFitTargetBRP", err)
	}
}

// Independent reference values for the 45 mm raised BRP (Fenrir L, Lithic,
// level rails, target (-219, 713.1)).
func TestApplyFittingReferencePosts(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		spec                     assembly.SeatpostSpec
		insertion, setback, miss float64
	}{
		{"generic 0 mm", genericPostSpec(0), 197.387571, 0, 53.368222},
		{"generic with synthetic 27 mm setback", genericPostSpec(27), 190.006853, 27, 25.377598},
		{"Redshift", redshiftPostSpec(), 144.107252, 12, 40.927945},
	} {
		setup := fenrirFittingSetup()
		setup.Seatpost = assembly.SeatpostSetup{Spec: tc.spec, InsertionMm: 150}
		fit := mustFit(t, setup, fitReportBRP)
		got := fit.Placement.Setup
		assertNear(t, tc.name+" insertion", got.Seatpost.InsertionMm, tc.insertion, 1e-6)
		assertNear(t, tc.name+" rail offset", got.Saddle.RailOffsetMm, 15, 1e-9)
		assertNear(t, tc.name+" setback", got.Seatpost.Spec.SetbackMm, tc.setback, 1e-9)
		assertNear(t, tc.name+" miss", brpMissMm(t, fit), tc.miss, 1e-6)
		miss, _ := fit.BrpMiss()
		assertNear(t, tc.name+" miss.Y", miss.Y(), 0, 1e-9)
	}
}

func TestApplyFitting25mmPostReachesForwardTarget(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Seatpost = assembly.SeatpostSetup{Spec: genericPostSpec(25), InsertionMm: 150}
	fit := mustFit(t, setup, pose.NewPoint(assembly.DynamicWorld, -189, 713.1))
	if fit.Outcome != assembly.FitReached {
		t.Fatalf("Outcome = %v, LimitedBy = %v, want reached", fit.Outcome, fit.LimitedBy)
	}
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 190.553573, 1e-6)
	assertNear(t, "rail offset", fit.Placement.Setup.Saddle.RailOffsetMm, 12.450978, 1e-6)
}

func TestFitPostSetbackRedshiftForwardLimit(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Seatpost = assembly.SeatpostSetup{Spec: redshiftPostSpec(), InsertionMm: 150}
	target := pose.NewPoint(assembly.DynamicWorld, -150, 758.1)
	fit := mustFit(t, setup, target)
	assertNear(t, "rail offset", fit.Placement.Setup.Saddle.RailOffsetMm, -15, 1e-9)
	assertNear(t, "insertion", fit.Placement.Setup.Seatpost.InsertionMm, 100, 1e-9)
	want := []assembly.InputField{assembly.InputSeatpostInsertion, assembly.InputSaddleRailOffset}
	if !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	miss, _ := fit.BrpMiss()
	assertNear(t, "miss.X", miss.X(), 9.702493, 1e-6)
	assertNear(t, "miss.Y", miss.Y(), 2.453759, 1e-6)

	centered, err := assembly.FitPostSetback(setup, target, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "centered setback", centered.Placement.Setup.Seatpost.Spec.SetbackMm, 0, 1e-9)
	assertNear(t, "centered miss", brpMissMm(t, centered), 12.475261, 1e-5)

	held, err := assembly.FitPostSetback(setup, target, -15)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "held setback", held.Placement.Setup.Seatpost.Spec.SetbackMm, 1.993870, 1e-6)
	assertNear(t, "held insertion", held.Placement.Setup.Seatpost.InsertionMm, 100.191488, 1e-6)
}
