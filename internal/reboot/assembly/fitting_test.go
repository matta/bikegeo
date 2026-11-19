package assembly_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

// fenrirFittingSetup is the default Fenrir build (Lithic Carbon Mountain fork,
// EC44 cup). Numbers copied from internal/reboot/web/fixture.go; assembly tests
// must not import the web fixture.
func fenrirFittingSetup() assembly.BikeSetup {
	return assembly.BikeSetup{
		Frame:    fenrirSizeLFrame(),
		LowerCup: fenrirEc44Cup(),
		Fork: assembly.ForkSetup{
			Spec:   assembly.ForkSpec{StaticA2cMm: 495.0, TravelMm: 0.0, RakeMm: 54.0},
			SagPct: 0.0,
		},
		Wheels: fenrirWheels(),
		Seatpost: assembly.SeatpostSetup{
			ExtensionMm:    263.5,
			MinExtensionMm: 110.0,
			MaxExtensionMm: 300.0,
			PostSetbackMm:  0.0,
			ClampLengthMm:  30.0,
		},
		Saddle:    sqlab610DefaultSaddle(),
		Stem:      fenrirDefaultStem(),
		Handlebar: persuader800DefaultHandlebar(),
	}
}

// fitReportBRP is Matt's idmatch fit report BRP (docs/RIDER_FIT.md).
var fitReportBRP = pose.Point{X: -219, Y: 713.1}

func brpOf(t *testing.T, setup assembly.BikeSetup) pose.Point {
	t.Helper()
	p, err := assembly.PlaceBike(setup, nil)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	return p.Saddle.BRP.Origin()
}

func mustFit(t *testing.T, setup assembly.BikeSetup, target pose.Point) assembly.Fitting {
	t.Helper()
	fit, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &target}, nil)
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
	return f.Placement.Setup.Seatpost.ExtensionMm, f.Placement.Setup.Saddle.RailOffsetMm
}

func TestApplyFittingRoundTripReached(t *testing.T) {
	known := fenrirFittingSetup()
	known.Seatpost.ExtensionMm = 200
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
		known.Seatpost.ExtensionMm = c.ext
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
	assertNear(t, "extension", ext, 260.092, 0.01)
	miss, _ := fit.BrpMiss()
	assertNear(t, "miss.X", miss.X, -38.212, 0.01)
	assertNear(t, "miss.Y", miss.Y, -10.446, 0.01)
	assertNear(t, "|miss|", brpMissMm(t, fit), 39.614, 0.01)

	// Setback is perpendicular to the seat axis: same extension, smaller miss.
	setback := fenrirFittingSetup()
	setback.Seatpost.PostSetbackMm = 27
	fit27 := mustFit(t, setback, fitReportBRP)
	ext27, rail27 := fittedValues(fit27)
	if want := []assembly.InputField{assembly.InputSaddleRailOffset}; !reflect.DeepEqual(fit27.LimitedBy, want) {
		t.Fatalf("setback LimitedBy = %v, want %v", fit27.LimitedBy, want)
	}
	assertNear(t, "setback rail offset", rail27, 15, 0)
	assertNear(t, "setback extension", ext27, ext, 1e-9)
	assertNear(t, "setback |miss|", brpMissMm(t, fit27), 12.614, 0.01)
}

func TestApplyFittingExtensionLimits(t *testing.T) {
	base := fenrirFittingSetup()
	baseBRP := brpOf(t, base)
	// The default Fenrir seat axis heading is 105.289°.
	seat := 105.289 * math.Pi / 180

	above := mustFit(t, base, pose.Point{X: baseBRP.X + 200*math.Cos(seat), Y: baseBRP.Y + 200*math.Sin(seat)})
	ext, _ := fittedValues(above)
	if len(above.LimitedBy) == 0 || above.LimitedBy[0] != assembly.InputSeatpostExtension {
		t.Fatalf("above LimitedBy = %v, want it to start with the seatpost extension", above.LimitedBy)
	}
	assertNear(t, "above extension", ext, 300, 0)

	below := mustFit(t, base, pose.Point{X: baseBRP.X - 200*math.Cos(seat), Y: baseBRP.Y - 200*math.Sin(seat)})
	ext, _ = fittedValues(below)
	if len(below.LimitedBy) == 0 || below.LimitedBy[0] != assembly.InputSeatpostExtension {
		t.Fatalf("below LimitedBy = %v, want it to start with the seatpost extension", below.LimitedBy)
	}
	assertNear(t, "below extension", ext, 110, 0)
}

func TestApplyFittingBothLimitsInOrder(t *testing.T) {
	base := fenrirFittingSetup()
	baseBRP := brpOf(t, base)
	seat := 105.289 * math.Pi / 180
	// 200 mm up the seat axis plus 50 mm of rail offset (level rails: BRP moves -X).
	target := pose.Point{X: baseBRP.X + 200*math.Cos(seat) - 50, Y: baseBRP.Y + 200*math.Sin(seat)}
	fit := mustFit(t, base, target)
	want := []assembly.InputField{assembly.InputSeatpostExtension, assembly.InputSaddleRailOffset}
	if !reflect.DeepEqual(fit.LimitedBy, want) {
		t.Fatalf("LimitedBy = %v, want %v", fit.LimitedBy, want)
	}
	ext, rail := fittedValues(fit)
	assertNear(t, "extension", ext, 300, 0)
	assertNear(t, "rail offset", rail, 15, 0)
}

func TestApplyFittingUnboundedMaxExtension(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Seatpost.MaxExtensionMm = 0
	baseBRP := brpOf(t, setup)
	seat := 105.289 * math.Pi / 180
	fit := mustFit(t, setup, pose.Point{X: baseBRP.X + 200*math.Cos(seat), Y: baseBRP.Y + 200*math.Sin(seat)})
	if fit.Outcome != assembly.FitReached {
		t.Fatalf("Outcome = %v, want reached", fit.Outcome)
	}
	ext, _ := fittedValues(fit)
	assertNear(t, "extension", ext, 463.5, 0.01)
}

func TestApplyFittingTraceEquivalence(t *testing.T) {
	tr := pose.NewTrace()
	brp := fitReportBRP
	target := assembly.FitTarget{BRP: &brp}
	traced, err := assembly.ApplyFitting(fenrirFittingSetup(), target, tr)
	if err != nil {
		t.Fatalf("traced: %v", err)
	}
	untraced, err := assembly.ApplyFitting(fenrirFittingSetup(), target, nil)
	if err != nil {
		t.Fatalf("untraced: %v", err)
	}
	if traced.Outcome != untraced.Outcome || !reflect.DeepEqual(traced.LimitedBy, untraced.LimitedBy) {
		t.Fatalf("outcome differs: %v %v vs %v %v", traced.Outcome, traced.LimitedBy, untraced.Outcome, untraced.LimitedBy)
	}
	tp, up := traced.Placement, untraced.Placement
	if tp.Setup != up.Setup ||
		tp.Saddle.BRP.Origin() != up.Saddle.BRP.Origin() ||
		tp.Saddle.SitBones.Origin() != up.Saddle.SitBones.Origin() ||
		tp.Handlebar.HandGrip.Origin() != up.Handlebar.HandGrip.Origin() {
		t.Fatalf("placements differ with tracing on vs off")
	}
	n := len(tr.Steps())
	if n == 0 {
		t.Fatal("trace is empty")
	}
	tr.Freeze()
	pose.NewPose(pose.Point{}, 0, tr).Advance("after freeze", 1)
	if len(tr.Steps()) != n {
		t.Fatal("trace was not frozen by ApplyFitting")
	}
}

func TestApplyFittingErrors(t *testing.T) {
	var inputErr *assembly.InputError
	if _, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{}, nil); err == nil {
		t.Fatal("empty FitTarget: want error")
	}
	for _, bad := range []pose.Point{{X: math.NaN(), Y: 700}, {X: -200, Y: math.Inf(1)}} {
		_, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{BRP: &bad}, nil)
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetBRP {
			t.Fatalf("target %v: err = %v, want InputFitTargetBRP", bad, err)
		}
	}

	longClamp := fenrirFittingSetup()
	longClamp.Seatpost.ClampLengthMm = 70
	if _, err := assembly.ApplyFitting(longClamp, assembly.FitTarget{BRP: &fitReportBRP}, nil); err == nil {
		t.Fatal("clamp longer than rails: want error")
	}

	badFrame := fenrirFittingSetup()
	badFrame.Frame.ReachMm = math.NaN()
	_, fitErr := assembly.ApplyFitting(badFrame, assembly.FitTarget{BRP: &fitReportBRP}, nil)
	_, placeErr := assembly.PlaceBike(badFrame, nil)
	if fitErr == nil || placeErr == nil || fitErr.Error() != placeErr.Error() {
		t.Fatalf("fit error %v, want PlaceBike error %v", fitErr, placeErr)
	}
}

func TestApplyFittingIsMinimalOverGrid(t *testing.T) {
	setup := fenrirFittingSetup()
	fit := mustFit(t, setup, fitReportBRP)
	if fit.Outcome != assembly.FitLimited {
		t.Fatalf("Outcome = %v, want limited", fit.Outcome)
	}
	best := brpMissMm(t, fit)
	for ext := 110.0; ext <= 300; ext++ {
		for rail := -15.0; rail <= 15; rail++ {
			s := setup
			s.Seatpost.ExtensionMm = ext
			s.Saddle.RailOffsetMm = rail
			brp := brpOf(t, s)
			miss := math.Hypot(fitReportBRP.X-brp.X, fitReportBRP.Y-brp.Y)
			if miss < best-1e-6 {
				t.Fatalf("grid point ext %v rail %v misses by %v, beating the fitting's %v", ext, rail, miss, best)
			}
		}
	}
}

func TestApplyFittingCopiesTarget(t *testing.T) {
	target := fitReportBRP
	fit := mustFit(t, fenrirFittingSetup(), target)
	if fit.Target.BRP == &target {
		t.Fatal("Fitting.Target.BRP aliases the caller's point")
	}
	target.X = 1000
	if *fit.Target.BRP != fitReportBRP {
		t.Fatalf("Target.BRP = %v after caller mutation, want %v", *fit.Target.BRP, fitReportBRP)
	}
}

// fitReportGrip is the idmatch fit report handlebar point (docs/RIDER_FIT.md).
// Whether the report point is HandGrip or the stem clamp is unresolved.
var fitReportGrip = pose.Point{X: 481, Y: 726}

func gripPlacement(t *testing.T, setup assembly.BikeSetup) assembly.BikePlacement {
	t.Helper()
	p, err := assembly.PlaceBike(setup, nil)
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
	fit, err := assembly.ApplyFitting(setup, assembly.FitTarget{HandGrip: &grip}, nil)
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
	for _, c := range []struct{ spacers, length float64 }{{0, 40}, {40, 130}, {0, 130}} {
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
		assertNear(t, "miss.X", miss.X, c.x, 0.01)
		assertNear(t, "miss.Y", miss.Y, c.y, 0.01)
		assertNear(t, "|miss|", miss.Norm(), c.absMiss, 0.01)
	}
}

// unitAt returns the unit vector at a heading, using independent math.
func unitAt(h pose.Deg) pose.Point {
	r := float64(h) * math.Pi / 180
	return pose.Point{X: math.Cos(r), Y: math.Sin(r)}
}

func TestApplyFittingGripSpacersLimitOnly(t *testing.T) {
	base := withStem(fenrirFittingSetup(), 0, 100)
	u := unitAt(gripPlacement(t, base).Stem.SpacerTop.HeadingDeg())
	g := gripOf(t, base)
	target := pose.Point{X: g.X - 20*u.X, Y: g.Y - 20*u.Y}
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
	for sp := 0.0; sp <= 40; sp++ {
		for ln := 40.0; ln <= 130; ln++ {
			g := gripOf(t, withStem(setup, sp, ln))
			if miss := math.Hypot(target.X-g.X, target.Y-g.Y); miss < best-1e-6 {
				t.Fatalf("grid spacers %v length %v misses by %v, beating the fitting's %v", sp, ln, miss, best)
			}
		}
	}
}

func TestApplyFittingGripLengthLimitOnly(t *testing.T) {
	base := withStem(fenrirFittingSetup(), 20, 130)
	v := unitAt(gripPlacement(t, base).Stem.BarClamp.HeadingDeg())
	g := gripOf(t, base)
	target := pose.Point{X: g.X + 15*v.X, Y: g.Y + 15*v.Y}
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
	if got.Stem.AngleDeg != in.Stem.AngleDeg || got.Stem.UpperCoverMm != in.Stem.UpperCoverMm ||
		got.Stem.ClampHeightMm != in.Stem.ClampHeightMm || got.Handlebar != in.Handlebar {
		t.Fatalf("fixed cockpit inputs changed: %+v", got.Stem)
	}
}

func TestApplyFittingBothLandmarksMatchSingleFits(t *testing.T) {
	setup := fenrirFittingSetup()
	brp, grip := fitReportBRP, fitReportGrip
	both, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &brp, HandGrip: &grip}, nil)
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

func TestApplyFittingBothLandmarksTraceEquivalence(t *testing.T) {
	brp, grip := fitReportBRP, fitReportGrip
	target := assembly.FitTarget{BRP: &brp, HandGrip: &grip}
	tr := pose.NewTrace()
	traced, err := assembly.ApplyFitting(fenrirFittingSetup(), target, tr)
	if err != nil {
		t.Fatal(err)
	}
	untraced, err := assembly.ApplyFitting(fenrirFittingSetup(), target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if traced.Placement.Setup != untraced.Placement.Setup ||
		traced.Placement.Saddle.BRP.Origin() != untraced.Placement.Saddle.BRP.Origin() ||
		traced.Placement.Handlebar.HandGrip.Origin() != untraced.Placement.Handlebar.HandGrip.Origin() ||
		!reflect.DeepEqual(traced.LimitedBy, untraced.LimitedBy) {
		t.Fatal("placements differ with tracing on vs off")
	}
	if len(tr.Steps()) == 0 {
		t.Fatal("trace is empty")
	}
}

func TestApplyFittingGripErrors(t *testing.T) {
	var inputErr *assembly.InputError
	for _, bad := range []pose.Point{{X: math.NaN(), Y: 700}, {X: 400, Y: math.Inf(-1)}} {
		_, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{HandGrip: &bad}, nil)
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
		if _, err := assembly.ApplyFitting(setup, assembly.FitTarget{HandGrip: &grip}, nil); err == nil {
			t.Errorf("%s with grip target: want error", name)
		}
		if _, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &brp}, nil); err != nil {
			t.Errorf("%s with only BRP target: %v", name, err)
		}
	}

	bad := fenrirFittingSetup()
	bad.Stem.MaxSpacersMm = -1
	_, fitErr := assembly.ApplyFitting(bad, assembly.FitTarget{HandGrip: &grip}, nil)
	_, placeErr := assembly.PlaceBike(bad, nil)
	if fitErr == nil || placeErr == nil || fitErr.Error() != placeErr.Error() {
		t.Fatalf("fit error %v, want PlaceBike error %v", fitErr, placeErr)
	}
}
