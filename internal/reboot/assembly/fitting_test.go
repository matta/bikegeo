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
	fit, err := assembly.ApplyFitting(setup, assembly.SaddleFitTarget{BRP: target}, nil)
	if err != nil {
		t.Fatalf("ApplyFitting: %v", err)
	}
	return fit
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
	assertNear(t, "miss", fit.BrpMissMm(), 0, 1e-9)
	if fit.LimitedBy != nil {
		t.Fatalf("LimitedBy = %v, want nil", fit.LimitedBy)
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
	miss := fit.BrpMiss()
	assertNear(t, "miss.X", miss.X, -38.212, 0.01)
	assertNear(t, "miss.Y", miss.Y, -10.446, 0.01)
	assertNear(t, "|miss|", fit.BrpMissMm(), 39.614, 0.01)

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
	assertNear(t, "setback |miss|", fit27.BrpMissMm(), 12.614, 0.01)
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
	target := assembly.SaddleFitTarget{BRP: fitReportBRP}
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
	for _, bad := range []pose.Point{{X: math.NaN(), Y: 700}, {X: -200, Y: math.Inf(1)}} {
		_, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.SaddleFitTarget{BRP: bad}, nil)
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetBRP {
			t.Fatalf("target %v: err = %v, want InputFitTargetBRP", bad, err)
		}
	}

	longClamp := fenrirFittingSetup()
	longClamp.Seatpost.ClampLengthMm = 70
	if _, err := assembly.ApplyFitting(longClamp, assembly.SaddleFitTarget{BRP: fitReportBRP}, nil); err == nil {
		t.Fatal("clamp longer than rails: want error")
	}

	badFrame := fenrirFittingSetup()
	badFrame.Frame.ReachMm = math.NaN()
	_, fitErr := assembly.ApplyFitting(badFrame, assembly.SaddleFitTarget{BRP: fitReportBRP}, nil)
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
	best := fit.BrpMissMm()
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
