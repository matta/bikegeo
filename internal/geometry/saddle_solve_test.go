package geometry_test

import (
	"math"
	"strings"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/frames"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/model"
)

func TestSolveSaddleIndependentInverseExample(t *testing.T) {
	// At STA 60°, zero post setback, rail offset +12 mm, level rails, local BRP +32 mm,
	// support (-48, +10) mm, BRP at axial=100 is (-30, 50√3), whose radius is
	// sqrt(900 + 7500) = sqrt(8400) = 20√21 mm.
	spec := model.ReferenceIdmatchSaddle()
	targetRadius := 20.0 * math.Sqrt(21.0)
	p, err := geometry.SolveSaddleForBRPRadius(spec, 60.0, targetRadius, 0.0, 12.0)
	if err != nil {
		t.Fatalf("SolveSaddleForBRPRadius: %v", err)
	}

	if math.Abs(p.AxialDistanceMm()-100.0) > 1e-9 {
		t.Errorf("AxialDistanceMm = %.12f, want 100", p.AxialDistanceMm())
	}
	sqrt3 := math.Sqrt(3)
	for _, tc := range []struct {
		name string
		got  geom2d.Point2
		want geom2d.Point2
	}{
		{"SeatpostClamp", p.SeatpostClamp(), geom2d.Point2{X: -50, Y: 50 * sqrt3}},
		{"RailCenter", p.RailCenter(), geom2d.Point2{X: -62, Y: 50 * sqrt3}},
		{"BRP", p.BRP(), geom2d.Point2{X: -30, Y: 50 * sqrt3}},
		{"SitBones", p.SitBones(), geom2d.Point2{X: -110, Y: 50*sqrt3 + 10}},
	} {
		if d := tc.got.DistanceTo(tc.want); d > 1e-9 {
			t.Errorf("%s = (%.9f, %.9f), want (%.9f, %.9f) (dist=%.3e)", tc.name, tc.got.X, tc.got.Y, tc.want.X, tc.want.Y, d)
		}
	}
}

func TestSolveSaddleForwardInverseConsistency(t *testing.T) {
	for _, spec := range catalog.AllSaddles() {
		for _, sta := range []model.Deg{60, 70, 75, 80} {
			for _, pitch := range []model.Deg{-7, 0, 10} {
				s := spec
				s.RailPitchDeg = pitch
				for _, offset := range []float64{-15, 0, 15} {
					const axial, post = 700.0, 11.0
					axis, err := assembly.NewSeatAxis(geom2d.Origin(), sta)
					if err != nil {
						t.Fatal(err)
					}
					clamp, err := axis.WithSeatpostClamp(axial, post)
					if err != nil {
						t.Fatal(err)
					}
					saddle, err := assembly.NewSaddle(s)
					if err != nil {
						t.Fatal(err)
					}
					fwd, err := clamp.AttachSaddle(saddle, offset)
					if err != nil {
						t.Fatal(err)
					}

					targetRadius := fwd.BRP().DistanceToOrigin()
					inv, err := geometry.SolveSaddleForBRPRadius(s, sta, targetRadius, post, offset)
					if err != nil {
						t.Fatalf("%s STA=%v pitch=%v offset=%v: %v", s.ID, sta, pitch, offset, err)
					}
					if math.Abs(inv.AxialDistanceMm()-axial) > 1e-8 {
						t.Errorf("%s STA=%v pitch=%v offset=%v: axial=%.9f, want %.9f",
							s.ID, sta, pitch, offset, inv.AxialDistanceMm(), axial)
					}
					if d := inv.BRP().DistanceTo(fwd.BRP()); d > 1e-8 {
						t.Errorf("%s STA=%v pitch=%v offset=%v: BRP diff=%.3e", s.ID, sta, pitch, offset, d)
					}
					if d := inv.SitBones().DistanceTo(fwd.SitBones()); d > 1e-8 {
						t.Errorf("%s STA=%v pitch=%v offset=%v: SitBones diff=%.3e", s.ID, sta, pitch, offset, d)
					}
				}
			}
		}
	}
}

func TestSolveSaddleTangentAndImpossibleBranches(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()

	// Tangent case (discriminant == 0):
	// At sta=60°, postSetback=0, railOffset=0, RailCenterToBrpMm=100, pitch=0°:
	// q = (100, 0), perp = -100*cos(30°). Setting targetRadius = 100*cos(30°)
	// hits the tangent (discriminant == 0) with axial = 50 > 0 and BRP = (75, 25√3).
	tangentSpec := model.SaddleSpec{RailCenterToBrpMm: 100}
	tangentRadius := math.Nextafter(100.0*model.Deg(30).Angle().Cos(), math.Inf(1))
	pTan, err := geometry.SolveSaddleForBRPRadius(tangentSpec, 60.0, tangentRadius, 0.0, 0.0)
	if err != nil {
		t.Fatalf("tangent case should succeed: %v", err)
	}
	if math.Abs(pTan.AxialDistanceMm()-50.0) > 1e-5 {
		t.Errorf("tangent axial = %.9f, want 50", pTan.AxialDistanceMm())
	}
	if pTan.BRP().DistanceTo(geom2d.Point2{X: 75, Y: 25 * math.Sqrt(3)}) > 1e-5 {
		t.Errorf("tangent BRP = %+v", pTan.BRP())
	}

	// Just below tangent radius: no intersection (discriminant < 0).
	if _, err := geometry.SolveSaddleForBRPRadius(tangentSpec, 60.0, tangentRadius-1e-3, 0.0, 0.0); err == nil {
		t.Error("expected error for no-intersection (discriminant < 0)")
	}

	// Impossible / rejected cases:
	for _, tc := range []struct {
		name               string
		sta                model.Deg
		height, post, rail float64
		pitch              model.Deg
	}{
		{"no radial intersection", 73, 10, 100, 0, 0},
		{"zero height", 73, 0, 0, 0, 0},
		{"negative height", 73, -746, 0, 0, 0},
		{"NaN height", 73, math.NaN(), 0, 0, 0},
		{"Inf height", 73, math.Inf(1), 0, 0, 0},
		{"overflow radius squared", 73, 1e200, 0, 0, 0},
		{"STA 0", 0, 746, 0, 0, 0},
		{"STA 90", 90, 746, 0, 0, 0},
		{"NaN railOffset", 73, 746, 0, math.NaN(), 0},
		{"NaN pitch", 73, 746, 0, 0, model.Deg(math.NaN())},
		{"negative extension branch only", 73, 10, 0, 164, -73},
	} {
		s := spec
		s.RailPitchDeg = tc.pitch
		if _, err := geometry.SolveSaddleForBRPRadius(s, tc.sta, tc.height, tc.post, tc.rail); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}

	// BRP below BB (Y <= 0): at STA=60°, height=100, postSetback=95, railOffset=32,
	// discriminant > 0 and axial > 0, but BRP Y = -20.46 mm <= 0.
	_, err = geometry.SolveSaddleForBRPRadius(spec, 60, 100, 95, 32)
	if err == nil || !strings.Contains(err.Error(), "not above BB") {
		t.Errorf("BRP below BB: expected 'not above BB' error, got %v", err)
	}
}

func TestMapPossibleBRPs_CandidatesAndFailures(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()

	// Fractional clamp window returns both endpoints.
	const clampLength = 59.88 // 60 mm marks leave 0.06 mm on each side.
	candidates, err := geometry.MapPossibleBRPs(s, 73, 746, 0, clampLength)
	if err != nil {
		t.Fatalf("MapPossibleBRPs: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	for i, want := range []float64{-0.06, 0.06} {
		got := candidates[i].RailOffsetMm
		if math.Abs(got-want) > 1e-9 || !s.ClampFitsAt(clampLength, got) {
			t.Errorf("candidate %d = %+.4f mm, want %+.4f mm", i, got, want)
		}
	}

	// Failing candidate must return an error naming the candidate rather than dropping it.
	_, err = geometry.MapPossibleBRPs(s, 73, 10, 100, 30)
	if err == nil {
		t.Fatal("expected error when candidates fail, got nil")
	}
	if !strings.Contains(err.Error(), "candidate rail offset") {
		t.Errorf("expected error naming failing candidate, got: %v", err)
	}

	// Overlong or non-positive/non-finite clamp lengths must return an error.
	for _, badClamp := range []float64{0, -1, math.NaN(), math.Inf(-1), math.Inf(1), 61} {
		if _, err := geometry.MapPossibleBRPs(s, 73, 746, 0, badClamp); err == nil {
			t.Errorf("expected error for clamp length %v, got nil", badClamp)
		}
	}
}

func TestEndToEndSaddleFailurePropagation(t *testing.T) {
	f, err := frames.GetFrameByID("salsa-fargo")
	if err != nil {
		t.Fatalf("GetFrameByID: %v", err)
	}
	bike, err := geometry.MaterializeBikeGeometry(*f, "L", catalog.ExpandStemSetsFrom(catalog.CommonStemCatalog, f.Stems))
	if err != nil {
		t.Fatalf("MaterializeBikeGeometry: %v", err)
	}
	frame, err := geometry.FrameSizeFromBikeGeometry(bike)
	if err != nil {
		t.Fatalf("FrameSizeFromBikeGeometry: %v", err)
	}
	build := geometry.BuildSpec{
		Fork:               frame.CadFork,
		SagPct:             frame.CadSagPct,
		PhysicalLowerCupMm: frame.CadLowerCupMm,
		WheelRadiusFrontMm: bike.WheelRadiusMm(),
		WheelRadiusRearMm:  bike.WheelRadiusMm(),
	}
	profile := catalog.DefaultRiderProfile()
	goodSaddle := model.ReferenceIdmatchSaddle()
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		Stem:              bike.GetCanonicalStem(),
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         catalog.DefaultHandlebar(),
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := geometry.SeatSpec{
		Saddle:        goodSaddle,
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
	}
	fit := geometry.RiderFit{
		FrameSeatAxisHeightMm: profile.SaddleHeightMm,
		BrpHeightMm:           profile.SaddleHeightMm,
		Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480.9, Y: 726.1}, Ref: geometry.TargetHandGrip},
		ReferenceSaddle:       goodSaddle,
	}

	// Valid baseline must succeed so failures below isolate saddle propagation.
	if _, err := geometry.BuildScene(frame, build, cockpit, seat, fit); err != nil {
		t.Fatalf("valid-saddle BuildScene baseline failed: %v", err)
	}
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, profile, goodSaddle); err != nil {
		t.Fatalf("valid-saddle AnalyzeBikeFitWithSaddle baseline failed: %v", err)
	}

	badSaddle := goodSaddle
	badSaddle.RailCenterToBrpMm = math.NaN()
	seat.Saddle = badSaddle

	// 1. BuildScene must fail on invalid saddle geometry.
	if _, err := geometry.BuildScene(frame, build, cockpit, seat, fit); err == nil || !strings.Contains(err.Error(), "saddle rail-center-to-BRP offset must be finite") {
		t.Errorf("BuildScene: expected finite-offset saddle error, got %v", err)
	}

	// 2. AnalyzeBikeFitWithSaddle must fail when a candidate in MapPossibleBRPs fails
	// (rather than silently dropping infeasible candidates).
	infeasibleWindowSaddle := goodSaddle
	infeasibleWindowSaddle.RailLengthMm = 2000 // yields candidate offsets up to ±985 mm, exceeding BRP radius
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, profile, infeasibleWindowSaddle); err == nil || !strings.Contains(err.Error(), "candidate rail offset") {
		t.Errorf("AnalyzeBikeFitWithSaddle: expected failing candidate error, got %v", err)
	}
}

func TestSolveSaddleForBRPTarget_IndependentExample(t *testing.T) {
	// STA 60°, zero post setback, level reference saddle, radius 20√21 mm, setback 30 mm
	// yields axial distance 100 mm, rail offset +12 mm, and BRP (-30, 50√3) mm.
	spec := model.ReferenceIdmatchSaddle()
	targetRadius := 20.0 * math.Sqrt(21.0)
	sol, err := geometry.SolveSaddleForBRPTarget(spec, 60.0, targetRadius, 30.0, 0.0)
	if err != nil {
		t.Fatalf("SolveSaddleForBRPTarget: %v", err)
	}
	if math.Abs(sol.RailOffsetMm()-12.0) > 1e-9 {
		t.Errorf("RailOffsetMm = %.12f, want 12", sol.RailOffsetMm())
	}
	p := sol.Placement()
	if math.Abs(p.AxialDistanceMm()-100.0) > 1e-9 {
		t.Errorf("AxialDistanceMm = %.12f, want 100", p.AxialDistanceMm())
	}
	sqrt3 := math.Sqrt(3)
	for _, tc := range []struct {
		name string
		got  geom2d.Point2
		want geom2d.Point2
	}{
		{"SeatpostClamp", p.SeatpostClamp(), geom2d.Point2{X: -50, Y: 50 * sqrt3}},
		{"RailCenter", p.RailCenter(), geom2d.Point2{X: -62, Y: 50 * sqrt3}},
		{"BRP", p.BRP(), geom2d.Point2{X: -30, Y: 50 * sqrt3}},
		{"SitBones", p.SitBones(), geom2d.Point2{X: -110, Y: 50*sqrt3 + 10}},
	} {
		if d := tc.got.DistanceTo(tc.want); d > 1e-9 {
			t.Errorf("%s = (%.9f, %.9f), want (%.9f, %.9f) (dist=%.3e)", tc.name, tc.got.X, tc.got.Y, tc.want.X, tc.want.Y, d)
		}
	}
}

func TestSolveSaddleForBRPTarget_IndependentAxesAndRoundTrips(t *testing.T) {
	// Independent trigonometric construction across STA, pitch, signed rail offset, and post setback.
	base := model.ReferenceIdmatchSaddle()
	for _, sta := range []model.Deg{60, 70, 80} {
		for _, pitch := range []model.Deg{-7, 0, 12} {
			s := base
			s.RailPitchDeg = pitch
			for _, post := range []float64{0, 11, 25} {
				for _, offset := range []float64{-15, 0, 15} {
					const axial = 700.0
					a, p := sta.Angle(), pitch.Angle()
					clamp := geom2d.Point2{X: -axial*a.Cos() - post*a.Sin(), Y: axial*a.Sin() - post*a.Cos()}
					center := geom2d.Point2{X: clamp.X - offset*p.Cos(), Y: clamp.Y - offset*p.Sin()}
					brp := geom2d.Point2{X: center.X + s.RailCenterToBrpMm*p.Cos(), Y: center.Y + s.RailCenterToBrpMm*p.Sin()}
					sit := geom2d.Point2{
						X: center.X + s.RailCenterToSitBonesMm*p.Cos() - s.SitBonesElevationMm*p.Sin(),
						Y: center.Y + s.RailCenterToSitBonesMm*p.Sin() + s.SitBonesElevationMm*p.Cos(),
					}

					sol, err := geometry.SolveSaddleForBRPTarget(s, sta, brp.DistanceToOrigin(), -brp.X, post)
					if err != nil {
						t.Fatalf("STA=%v pitch=%v post=%v offset=%v: %v", sta, pitch, post, offset, err)
					}
					if math.Abs(sol.RailOffsetMm()-offset) > 1e-9 {
						t.Errorf("STA=%v pitch=%v post=%v offset=%v: rail=%.9f, want %.9f",
							sta, pitch, post, offset, sol.RailOffsetMm(), offset)
					}
					pl := sol.Placement()
					if math.Abs(pl.AxialDistanceMm()-axial) > 1e-9 {
						t.Errorf("STA=%v pitch=%v post=%v offset=%v: axial=%.9f, want %.9f",
							sta, pitch, post, offset, pl.AxialDistanceMm(), axial)
					}
					if d := pl.SeatpostClamp().DistanceTo(clamp); d > 1e-9 {
						t.Errorf("clamp diff = %.3e", d)
					}
					if d := pl.RailCenter().DistanceTo(center); d > 1e-9 {
						t.Errorf("railCenter diff = %.3e", d)
					}
					if d := pl.BRP().DistanceTo(brp); d > 1e-9 {
						t.Errorf("BRP diff = %.3e", d)
					}
					if d := pl.SitBones().DistanceTo(sit); d > 1e-9 {
						t.Errorf("SitBones diff = %.3e", d)
					}
				}
			}
		}
	}

	// Forward assembly round-trips across all catalog saddles.
	for _, spec := range catalog.AllSaddles() {
		for _, sta := range []model.Deg{60, 70, 75, 80} {
			for _, pitch := range []model.Deg{-7, 0, 10} {
				s := spec
				s.RailPitchDeg = pitch
				for _, offset := range []float64{-15, 0, 15} {
					const axial, post = 700.0, 11.0
					axis, err := assembly.NewSeatAxis(geom2d.Origin(), sta)
					if err != nil {
						t.Fatal(err)
					}
					clamp, err := axis.WithSeatpostClamp(axial, post)
					if err != nil {
						t.Fatal(err)
					}
					saddle, err := assembly.NewSaddle(s)
					if err != nil {
						t.Fatal(err)
					}
					fwd, err := clamp.AttachSaddle(saddle, offset)
					if err != nil {
						t.Fatal(err)
					}
					brp := fwd.BRP()
					sol, err := geometry.SolveSaddleForBRPTarget(s, sta, brp.DistanceToOrigin(), -brp.X, post)
					if err != nil {
						t.Fatalf("%s STA=%v pitch=%v offset=%v: %v", s.ID, sta, pitch, offset, err)
					}
					if math.Abs(sol.RailOffsetMm()-offset) > 1e-8 {
						t.Errorf("%s STA=%v pitch=%v offset=%v: rail=%.9f, want %.9f",
							s.ID, sta, pitch, offset, sol.RailOffsetMm(), offset)
					}
					pl := sol.Placement()
					if math.Abs(pl.AxialDistanceMm()-axial) > 1e-8 {
						t.Errorf("%s STA=%v pitch=%v offset=%v: axial=%.9f, want %.9f",
							s.ID, sta, pitch, offset, pl.AxialDistanceMm(), axial)
					}
					if d := pl.BRP().DistanceTo(brp); d > 1e-8 {
						t.Errorf("%s BRP diff=%.3e", s.ID, d)
					}
				}
			}
		}
	}

	// Forward BRP (negative setback, BRP.X > 0) is allowed if branch constraints hold.
	// At STA=60°, post=0, level reference saddle: BRP=(+10, 50√3) gives axial=100, rail=-28.
	fwdRadius := math.Hypot(10.0, 50.0*math.Sqrt(3))
	fwdSol, err := geometry.SolveSaddleForBRPTarget(base, 60.0, fwdRadius, -10.0, 0.0)
	if err != nil {
		t.Fatalf("forward BRP should succeed: %v", err)
	}
	if math.Abs(fwdSol.RailOffsetMm()-(-28.0)) > 1e-9 {
		t.Errorf("forward BRP rail = %.9f, want -28", fwdSol.RailOffsetMm())
	}
	if math.Abs(fwdSol.Placement().BRP().X-10.0) > 1e-9 {
		t.Errorf("forward BRP X = %.9f, want 10", fwdSol.Placement().BRP().X)
	}
}

func TestSolveSaddleForBRPTarget_SingularAndInvalid(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()

	// Parallel and coincident axes.
	parallel := spec
	parallel.RailPitchDeg = -73
	if _, err := geometry.SolveSaddleForBRPTarget(parallel, 73, 746, 219, 0); err == nil {
		t.Error("expected error for parallel axes")
	}

	// Coincident axes: STA 60°, pitch -60°, post=0, target on the seat axis line.
	coincident := spec
	coincident.RailPitchDeg = -60
	if _, err := geometry.SolveSaddleForBRPTarget(coincident, 60, 100, 50, 0); err == nil {
		t.Error("expected error for coincident axes")
	}

	// Near-parallel axes on either side of det = sin(sta + pitch) = 1e-12.
	belowThresh := spec
	belowThresh.RailCenterToBrpMm = 0
	belowThresh.RailPitchDeg = -60 + model.DegOf(geom2d.FromRad(5e-13))
	if _, err := geometry.SolveSaddleForBRPTarget(belowThresh, 60, 100, 50, 0); err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Errorf("expected parallel-axes error below 1e-12 threshold, got %v", err)
	}
	aboveThresh := spec
	aboveThresh.RailCenterToBrpMm = 0
	aboveThresh.RailPitchDeg = -60 + model.DegOf(geom2d.FromRad(2e-12))
	if _, err := geometry.SolveSaddleForBRPTarget(aboveThresh, 60, 100, 50, 0); err != nil && strings.Contains(err.Error(), "parallel") {
		t.Errorf("expected above-threshold near-parallel axes to pass determinant check, got %v", err)
	}

	for _, tc := range []struct {
		name                  string
		sta                   model.Deg
		height, setback, post float64
		pitch                 model.Deg
	}{
		{"setback equals radius", 73, 746, 746, 0, 0},
		{"negative setback equals radius", 73, 746, -746, 0, 0},
		{"setback exceeds radius", 73, 746, 800, 0, 0},
		{"zero height", 73, 0, 0, 0, 0},
		{"negative height", 73, -746, 0, 0, 0},
		{"NaN height", 73, math.NaN(), 219, 0, 0},
		{"Inf height", 73, math.Inf(1), 219, 0, 0},
		{"NaN setback", 73, 746, math.NaN(), 0, 0},
		{"Inf setback", 73, 746, math.Inf(-1), 0, 0},
		{"negative post", 73, 746, 219, -1, 0},
		{"NaN post", 73, 746, 219, math.NaN(), 0},
		{"overflow radius squared", 73, 1e200, 219, 0, 0},
		{"STA 0", 0, 746, 219, 0, 0},
		{"STA 90", 90, 746, 219, 0, 0},
		{"NaN pitch", 73, 746, 219, 0, model.Deg(math.NaN())},
		// Negative axial extension branch: at STA=60°, pitch=+45°, post=0, d=-10, r=-100
		// gives target = (132/√2 + 5, 132/√2 - 5√3) with target.Y > 0 and target.Dot(u) > 0, but d < 0.
		{"negative axial extension", 60, math.Hypot(132/math.Sqrt2+5, 132/math.Sqrt2-5*math.Sqrt(3)), -(132/math.Sqrt2 + 5), 0, 45},
		// Target behind/below upward seat axis: at STA=60°, u=(-0.5, √3/2).
		// Target=(+100, 10) has dot(u) = -50 + 5√3 < 0.
		{"target behind seat axis", 60, math.Hypot(100, 10), -100, 0, 0},
	} {
		s := spec
		s.RailPitchDeg = tc.pitch
		if _, err := geometry.SolveSaddleForBRPTarget(s, tc.sta, tc.height, tc.setback, tc.post); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestSolveSaddleForBRPTarget_HardwareVsGeometryAndMisuse(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()

	// Out-of-window solution succeeds geometrically and retains negative rail margin.
	// At STA=60°, zero post, radius 20√21, setback 30 gives railOffset = +12 mm.
	// With RailLengthMm = 40 and ClampLengthMm = 30, half window is 5 mm -> margin = -7 mm.
	shortRail := spec
	shortRail.RailLengthMm = 40
	sol, err := geometry.SolveSaddleForBRPTarget(shortRail, 60.0, 20.0*math.Sqrt(21.0), 30.0, 0.0)
	if err != nil {
		t.Fatalf("out-of-window target should succeed geometrically: %v", err)
	}
	if math.Abs(sol.RailOffsetMm()-12.0) > 1e-9 {
		t.Errorf("RailOffsetMm = %.9f, want 12", sol.RailOffsetMm())
	}
	if shortRail.ClampFitsAt(30, sol.RailOffsetMm()) {
		t.Error("expected ClampFitsAt to be false for out-of-window offset")
	}

	// Zero-value and failed SaddleRailSolution panic on access.
	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}
	assertPanics("zero SaddleRailSolution RailOffsetMm", func() {
		var s geometry.SaddleRailSolution
		_ = s.RailOffsetMm()
	})
	assertPanics("zero SaddleRailSolution Placement", func() {
		var s geometry.SaddleRailSolution
		_ = s.Placement()
	})
	assertPanics("failed SaddleRailSolution RailOffsetMm", func() {
		s, _ := geometry.SolveSaddleForBRPTarget(spec, 73, 746, 800, 0)
		_ = s.RailOffsetMm()
	})
	assertPanics("failed SaddleRailSolution Placement", func() {
		s, _ := geometry.SolveSaddleForBRPTarget(spec, 73, 746, 800, 0)
		_ = s.Placement()
	})
}

func TestEndToEndRailOffsetFailurePropagation(t *testing.T) {
	f, err := frames.GetFrameByID("salsa-fargo")
	if err != nil {
		t.Fatalf("GetFrameByID: %v", err)
	}
	bike, err := geometry.MaterializeBikeGeometry(*f, "L", catalog.ExpandStemSetsFrom(catalog.CommonStemCatalog, f.Stems))
	if err != nil {
		t.Fatalf("MaterializeBikeGeometry: %v", err)
	}
	frame, err := geometry.FrameSizeFromBikeGeometry(bike)
	if err != nil {
		t.Fatalf("FrameSizeFromBikeGeometry: %v", err)
	}
	build := geometry.BuildSpec{
		Fork:               frame.CadFork,
		SagPct:             frame.CadSagPct,
		PhysicalLowerCupMm: frame.CadLowerCupMm,
		WheelRadiusFrontMm: bike.WheelRadiusMm(),
		WheelRadiusRearMm:  bike.WheelRadiusMm(),
	}
	profile := catalog.DefaultRiderProfile()
	goodSaddle := model.ReferenceIdmatchSaddle()
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		Stem:              bike.GetCanonicalStem(),
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         catalog.DefaultHandlebar(),
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := geometry.SeatSpec{
		Saddle:        goodSaddle,
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
	}
	fit := geometry.RiderFit{
		FrameSeatAxisHeightMm: profile.SaddleHeightMm,
		BrpHeightMm:           profile.SaddleHeightMm,
		TargetSitBonesXMm:     -model.DefaultTargetSitBoneSetbackMm(),
		Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480.9, Y: 726.1}, Ref: geometry.TargetHandGrip},
		ReferenceSaddle:       goodSaddle,
	}

	// 1. BuildScene baseline and out-of-window negative margin vs impossible target.
	scene, err := geometry.BuildScene(frame, build, cockpit, seat, fit)
	if err != nil {
		t.Fatalf("BuildScene baseline: %v", err)
	}
	shortSeat := seat
	shortSeat.Saddle.RailLengthMm = 32 // half window = 1 mm, smaller than required rail offset
	shortScene, err := geometry.BuildScene(frame, build, cockpit, shortSeat, fit)
	if err != nil {
		t.Fatalf("BuildScene out-of-window should succeed: %v", err)
	}
	wantMargin := shortSeat.HalfRailWindowMm() - math.Abs(scene.RequiredRailOffsetMm)
	if math.Abs(shortScene.RailMarginMm-wantMargin) > 1e-9 || !(shortScene.RailMarginMm < 0) {
		t.Errorf("BuildScene RailMarginMm = %.6f, want %.6f (< 0)", shortScene.RailMarginMm, wantMargin)
	}
	impossibleFit := fit
	impossibleFit.TargetSitBonesXMm = -1000
	if _, err := geometry.BuildScene(frame, build, cockpit, seat, impossibleFit); err == nil {
		t.Error("BuildScene: expected error for unreachable target sit-bones setback")
	}

	// 2. TranslateSaddleFit baseline and failure propagation.
	refPt, err := geometry.SolveBRPPoint(goodSaddle, 73, 746, 0, 0)
	if err != nil {
		t.Fatalf("SolveBRPPoint baseline: %v", err)
	}
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	if _, err := geometry.TranslateSaddleFit(refPt, aspin, 73, model.DefaultSeatpostClampLengthMm); err != nil {
		t.Fatalf("TranslateSaddleFit baseline: %v", err)
	}
	parallelAspin := aspin
	parallelAspin.RailPitchDeg = -73
	if _, err := geometry.TranslateSaddleFit(refPt, parallelAspin, 73, model.DefaultSeatpostClampLengthMm); err == nil {
		t.Error("TranslateSaddleFit: expected error for parallel target rails")
	}

	// 3. AnalyzeSaddleSwap baseline and failure propagation.
	if _, err := geometry.AnalyzeSaddleSwap(goodSaddle, aspin, 746, 299, 0, 30, 700, 13); err != nil {
		t.Fatalf("AnalyzeSaddleSwap baseline: %v", err)
	}
	badTarget := aspin
	badTarget.RailPitchDeg = model.Deg(math.NaN())
	if _, err := geometry.AnalyzeSaddleSwap(goodSaddle, badTarget, 746, 299, 0, 30, 700, 13); err == nil {
		t.Error("AnalyzeSaddleSwap: expected error for NaN target rail pitch")
	}

	// 4. CalculateUniversalCompromiseSTA baseline and failure propagation.
	saddles := catalog.AllSaddles()
	if _, err := geometry.CalculateUniversalCompromiseSTA(saddles, 299, 746, 15, "15mm", 30); err != nil {
		t.Fatalf("CalculateUniversalCompromiseSTA baseline: %v", err)
	}
	badSaddles := append([]model.SaddleSpec(nil), saddles...)
	badSaddles[1].RailPitchDeg = model.Deg(math.NaN())
	if _, err := geometry.CalculateUniversalCompromiseSTA(badSaddles, 299, 746, 15, "15mm", 30); err == nil {
		t.Error("CalculateUniversalCompromiseSTA: expected error for NaN saddle rail pitch")
	}
	for _, badClamp := range []float64{0, -1, math.NaN(), math.Inf(-1), math.Inf(1)} {
		if _, err := geometry.CalculateUniversalCompromiseSTA(saddles, 299, 746, 15, "15mm", badClamp); err == nil {
			t.Errorf("CalculateUniversalCompromiseSTA: expected error for clamp length %v", badClamp)
		}
	}

	// 5. CalculateIdealSeatTubeAngle baseline and failure propagation.
	if _, err := geometry.CalculateIdealSeatTubeAngle(goodSaddle, 299, 746, 15, "15mm", 30); err != nil {
		t.Fatalf("CalculateIdealSeatTubeAngle baseline: %v", err)
	}
	badIdealSaddle := goodSaddle
	badIdealSaddle.RailPitchDeg = model.Deg(math.NaN())
	if _, err := geometry.CalculateIdealSeatTubeAngle(badIdealSaddle, 299, 746, 15, "15mm", 30); err == nil {
		t.Error("CalculateIdealSeatTubeAngle: expected error for NaN saddle rail pitch")
	}
}

func TestSolveSaddleSTAForBRPTarget_IndependentExamples(t *testing.T) {
	ref := model.ReferenceIdmatchSaddle() // RailCenterToBrpMm = 32, level rails

	// 1. Zero post setback: radius 20*sqrt(21) mm, setback 30 mm, rail offset +12 mm -> STA 60°, axial 100 mm, BRP (-30, 50*sqrt(3))
	radius0 := 20 * math.Sqrt(21)
	sol0, err := geometry.SolveSaddleSTAForBRPTarget(ref, radius0, 30, 0, 12)
	if err != nil {
		t.Fatalf("SolveSaddleSTAForBRPTarget(zero post): %v", err)
	}
	if (sol0.EffStaDeg() - 60).Abs() > 1e-9 {
		t.Errorf("zero-post STA = %.12f°, want 60°", sol0.EffStaDeg())
	}
	if math.Abs(sol0.Placement().AxialDistanceMm()-100) > 1e-8 {
		t.Errorf("zero-post axial = %.10f mm, want 100 mm", sol0.Placement().AxialDistanceMm())
	}
	wantBrp0 := geom2d.Point2{X: -30, Y: 50 * math.Sqrt(3)}
	if d := sol0.Placement().BRP().DistanceTo(wantBrp0); d > 1e-8 {
		t.Errorf("zero-post BRP = %+v, want %+v (dist %.3e)", sol0.Placement().BRP(), wantBrp0, d)
	}

	// 2. Nonzero post setback: STA 60°, axial 100 mm, post setback 20 mm, rail offset +12 mm
	// Clamp = (-50 - 10*sqrt(3), 50*sqrt(3) - 10), BRP = (-30 - 10*sqrt(3), 50*sqrt(3) - 10)
	wantClamp := geom2d.Point2{X: -50 - 10*math.Sqrt(3), Y: 50*math.Sqrt(3) - 10}
	wantBrp := geom2d.Point2{X: -30 - 10*math.Sqrt(3), Y: 50*math.Sqrt(3) - 10}
	sol20, err := geometry.SolveSaddleSTAForBRPTarget(ref, wantBrp.DistanceToOrigin(), -wantBrp.X, 20, 12)
	if err != nil {
		t.Fatalf("SolveSaddleSTAForBRPTarget(post=20): %v", err)
	}
	if (sol20.EffStaDeg() - 60).Abs() > 1e-9 {
		t.Errorf("post=20 STA = %.12f°, want 60°", sol20.EffStaDeg())
	}
	if math.Abs(sol20.Placement().AxialDistanceMm()-100) > 1e-8 {
		t.Errorf("post=20 axial = %.10f mm, want 100 mm", sol20.Placement().AxialDistanceMm())
	}
	if d := sol20.Placement().SeatpostClamp().DistanceTo(wantClamp); d > 1e-8 {
		t.Errorf("post=20 clamp = %+v, want %+v (dist %.3e)", sol20.Placement().SeatpostClamp(), wantClamp, d)
	}
	if d := sol20.Placement().BRP().DistanceTo(wantBrp); d > 1e-8 {
		t.Errorf("post=20 BRP = %+v, want %+v (dist %.3e)", sol20.Placement().BRP(), wantBrp, d)
	}
}

func TestSolveSaddleSTAForBRPTarget_OrientationAndReuse(t *testing.T) {
	for _, base := range catalog.AllSaddles() {
		for _, pitch := range []model.Deg{-7, 0, 12} {
			s := base
			s.RailPitchDeg = pitch
			for _, sta := range []model.Deg{60, 72, 80} {
				for _, post := range []float64{0, 15, 25} {
					for _, offset := range []float64{-15, 0, 15} {
						const axial = 700.0
						a, p := sta.Angle(), pitch.Angle()
						clamp := geom2d.Point2{
							X: -axial*a.Cos() - post*a.Sin(),
							Y: axial*a.Sin() - post*a.Cos(),
						}
						center := geom2d.Point2{
							X: clamp.X - offset*p.Cos(),
							Y: clamp.Y - offset*p.Sin(),
						}
						brp := geom2d.Point2{
							X: center.X + s.RailCenterToBrpMm*p.Cos(),
							Y: center.Y + s.RailCenterToBrpMm*p.Sin(),
						}
						sol, err := geometry.SolveSaddleSTAForBRPTarget(s, brp.DistanceToOrigin(), -brp.X, post, offset)
						if err != nil {
							t.Fatalf("%s pitch=%v sta=%v post=%v off=%v: %v", s.ID, pitch, sta, post, offset, err)
						}
						if (sol.EffStaDeg() - sta).Abs() > 1e-9 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: got STA %.12f°", s.ID, pitch, sta, post, offset, sol.EffStaDeg())
						}
						if math.Abs(sol.Placement().AxialDistanceMm()-axial) > 1e-8 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: got axial %.10f mm", s.ID, pitch, sta, post, offset, sol.Placement().AxialDistanceMm())
						}
						if d := sol.Placement().BRP().DistanceTo(brp); d > 1e-8 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: BRP residual %.3e mm", s.ID, pitch, sta, post, offset, d)
						}
					}
				}
			}
		}
	}
}

func TestSolveSaddleSTAForBRPTarget_BranchesAndBoundaries(t *testing.T) {
	ref := model.ReferenceIdmatchSaddle()

	// Zero-radius clamp ambiguity (clamp == (0,0) with post=0): 90° pitched saddle puts BRP at (0, 32) while clamp is at BB.
	pitched90 := ref
	pitched90.RailPitchDeg = 90
	if _, err := geometry.SolveSaddleSTAForBRPTarget(pitched90, 32, 0, 0, 0); err == nil {
		t.Error("expected error for zero clamp radius ambiguity")
	}

	// Post setback exceeding clamp radius.
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, 100, 30, 200, 0); err == nil {
		t.Error("expected error when post setback exceeds clamp radius")
	}

	// Tangent zero distance (rho == p > 0) with legal STA in (0°, 90°) and BRP.Y > 0.
	// With RailPitchDeg=90, RailCenterToBrpMm=32: disp=(0, 32). Target BRP=(-12, 16) (radius=20, setback=12)
	// gives clamp=(-12, -16), rho=20 == postSetback=20, axial=0, STA = atan2(-16, 12) + 90° ≈ 36.87°.
	solT, err := geometry.SolveSaddleSTAForBRPTarget(pitched90, 20, 12, 20, 0)
	if err != nil {
		t.Fatalf("tangent zero-distance legal case failed: %v", err)
	}
	wantTangentSTA := model.DegOf(geom2d.AngleFromSlope(12, -16)) + 90
	if (solT.EffStaDeg()-wantTangentSTA).Abs() > 1e-9 || math.Abs(solT.Placement().AxialDistanceMm()) > 1e-8 {
		t.Errorf("tangent zero-distance: got STA=%.9f° (want %.9f°), axial=%.9f mm", solT.EffStaDeg(), wantTangentSTA, solT.Placement().AxialDistanceMm())
	}

	// Out-of-domain branches: STA = 90° (clamp directly above BB), STA > 90° (clamp forward of BB), STA <= 0°.
	pitched30 := ref
	pitched30.RailPitchDeg = 30
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, math.Hypot(32, 500), -32, 0, 0); err == nil {
		t.Error("expected error for 90° STA endpoint")
	}
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, 500, -100, 0, 0); err == nil {
		t.Error("expected error for STA > 90°")
	}
	if _, err := geometry.SolveSaddleSTAForBRPTarget(pitched30, 10, 0, 0, 0); err == nil {
		t.Error("expected error for STA <= 0°")
	}

	// Invalid targets, NaN, Inf, overflow, negative post setback.
	badCases := []struct {
		name                           string
		radius, setback, post, railOff float64
	}{
		{"zero radius", 0, 0, 0, 0},
		{"negative radius", -746, 200, 0, 0},
		{"unreachable setback==radius", 500, 500, 0, 0},
		{"unreachable setback>radius", 500, 501, 0, 0},
		{"negative post setback", 746, 219, -1, 0},
		{"NaN radius", math.NaN(), 219, 0, 0},
		{"NaN setback", 746, math.NaN(), 0, 0},
		{"NaN post", 746, 219, math.NaN(), 0},
		{"NaN railOffset", 746, 219, 0, math.NaN()},
		{"Inf radius", math.Inf(1), 219, 0, 0},
		{"Inf setback", 746, math.Inf(-1), 0, 0},
		{"Inf post", 746, 219, math.Inf(1), 0},
		{"Inf railOffset", 746, 219, 0, math.Inf(1)},
		{"overflow radius", 1e200, 219, 0, 0},
	}
	for _, tc := range badCases {
		if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, tc.radius, tc.setback, tc.post, tc.railOff); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}

	// Zero SaddleStaSolution must panic on accessors.
	var zero geometry.SaddleStaSolution
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"EffStaDeg", func() { _ = zero.EffStaDeg() }},
		{"Placement", func() { _ = zero.Placement() }},
	} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("zero SaddleStaSolution.%s() did not panic", tc.name)
				}
			}()
			tc.fn()
		}()
	}
}

func TestIdealAndCompromiseSTA_GeometryVsHardware(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	// 1. Invalid clamp or rail dimensions error.
	for _, badClamp := range []float64{0, -5, math.NaN(), math.Inf(1)} {
		if _, err := geometry.CalculateIdealSeatTubeAngle(sqlab, 299, 746, 15, "15mm", badClamp); err == nil {
			t.Errorf("CalculateIdealSeatTubeAngle: expected error for clampLength=%v", badClamp)
		}
	}
	for _, badRail := range []float64{0, -10, math.NaN(), math.Inf(1)} {
		badSaddle := sqlab
		badSaddle.RailLengthMm = badRail
		if _, err := geometry.CalculateIdealSeatTubeAngle(badSaddle, 299, 746, 15, "15mm", 30); err == nil {
			t.Errorf("CalculateIdealSeatTubeAngle: expected error for RailLengthMm=%v", badRail)
		}
		if _, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{badSaddle, aspin}, 299, 746, 15, "15mm", 30); err == nil {
			t.Errorf("CalculateUniversalCompromiseSTA: expected error for RailLengthMm=%v", badRail)
		}
	}

	// 2. Overlong positive clamp reports unavailable hardware range while keeping centered geometric result.
	overlongClamp := sqlab.RailLengthMm + 10
	resOverlong, err := geometry.CalculateIdealSeatTubeAngle(sqlab, 299, 746, 15, "15mm", overlongClamp)
	if err != nil {
		t.Fatalf("overlong clamp should not error for valid centered geometry: %v", err)
	}
	if resOverlong.HasClampWindow {
		t.Error("expected HasClampWindow=false for overlong clamp")
	}
	if (resOverlong.IdealStaDeg - 71.75).Abs() > 0.01 {
		t.Errorf("expected centered IdealStaDeg ~ 71.75°, got %.2f°", resOverlong.IdealStaDeg)
	}

	// 3. Impossible endpoint when hardware fits must error (naming saddle and endpoint), not set HasClampWindow=false.
	// With sitBoneSetback=51, saddleHeight=746, post=0, centered STA is < 90° (clamp.X=-3), while +5 mm margin
	// pushes clamp forward of BB (clamp.X=+2, STA > 90°).
	longRailSaddle := sqlab
	longRailSaddle.RailLengthMm = 300
	if _, err := geometry.CalculateIdealSeatTubeAngle(longRailSaddle, 51, 746, 0, "0mm", 30); err == nil || !strings.Contains(err.Error(), longRailSaddle.ID) {
		t.Errorf("expected contextual error naming saddle %q for impossible endpoint, got %v", longRailSaddle.ID, err)
	}
	if _, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{longRailSaddle, aspin}, 51, 746, 0, "0mm", 30); err == nil || !strings.Contains(err.Error(), longRailSaddle.ID) {
		t.Errorf("expected contextual compromise error naming saddle %q, got %v", longRailSaddle.ID, err)
	}

	// 4. Disjoint valid margin intervals remain a legitimate HasUniversalMargin=false result.
	compDisjoint, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{sqlab, aspin, slr}, 299, 746, 15, "15mm", 30)
	if err != nil {
		t.Fatalf("disjoint valid intervals should not error: %v", err)
	}
	if compDisjoint.HasUniversalMargin {
		t.Error("expected HasUniversalMargin=false for disjoint 3-saddle margin intervals")
	}
}

func TestSolveSaddleSTAForBRPTarget_IndependentExamples(t *testing.T) {
	ref := model.ReferenceIdmatchSaddle() // RailCenterToBrpMm = 32, level rails

	// 1. Zero post setback: radius 20*sqrt(21) mm, setback 30 mm, rail offset +12 mm -> STA 60°, axial 100 mm, BRP (-30, 50*sqrt(3))
	radius0 := 20 * math.Sqrt(21)
	sol0, err := geometry.SolveSaddleSTAForBRPTarget(ref, radius0, 30, 0, 12)
	if err != nil {
		t.Fatalf("SolveSaddleSTAForBRPTarget(zero post): %v", err)
	}
	if (sol0.EffStaDeg() - 60).Abs() > 1e-9 {
		t.Errorf("zero-post STA = %.12f°, want 60°", sol0.EffStaDeg())
	}
	if math.Abs(sol0.Placement().AxialDistanceMm()-100) > 1e-8 {
		t.Errorf("zero-post axial = %.10f mm, want 100 mm", sol0.Placement().AxialDistanceMm())
	}
	wantBrp0 := geom2d.Point2{X: -30, Y: 50 * math.Sqrt(3)}
	if d := sol0.Placement().BRP().DistanceTo(wantBrp0); d > 1e-8 {
		t.Errorf("zero-post BRP = %+v, want %+v (dist %.3e)", sol0.Placement().BRP(), wantBrp0, d)
	}

	// 2. Nonzero post setback: STA 60°, axial 100 mm, post setback 20 mm, rail offset +12 mm
	// Clamp = (-50 - 10*sqrt(3), 50*sqrt(3) - 10), BRP = (-30 - 10*sqrt(3), 50*sqrt(3) - 10)
	wantClamp := geom2d.Point2{X: -50 - 10*math.Sqrt(3), Y: 50*math.Sqrt(3) - 10}
	wantBrp := geom2d.Point2{X: -30 - 10*math.Sqrt(3), Y: 50*math.Sqrt(3) - 10}
	sol20, err := geometry.SolveSaddleSTAForBRPTarget(ref, wantBrp.DistanceToOrigin(), -wantBrp.X, 20, 12)
	if err != nil {
		t.Fatalf("SolveSaddleSTAForBRPTarget(post=20): %v", err)
	}
	if (sol20.EffStaDeg() - 60).Abs() > 1e-9 {
		t.Errorf("post=20 STA = %.12f°, want 60°", sol20.EffStaDeg())
	}
	if math.Abs(sol20.Placement().AxialDistanceMm()-100) > 1e-8 {
		t.Errorf("post=20 axial = %.10f mm, want 100 mm", sol20.Placement().AxialDistanceMm())
	}
	if d := sol20.Placement().SeatpostClamp().DistanceTo(wantClamp); d > 1e-8 {
		t.Errorf("post=20 clamp = %+v, want %+v (dist %.3e)", sol20.Placement().SeatpostClamp(), wantClamp, d)
	}
	if d := sol20.Placement().BRP().DistanceTo(wantBrp); d > 1e-8 {
		t.Errorf("post=20 BRP = %+v, want %+v (dist %.3e)", sol20.Placement().BRP(), wantBrp, d)
	}
}

func TestSolveSaddleSTAForBRPTarget_OrientationAndReuse(t *testing.T) {
	for _, base := range catalog.AllSaddles() {
		for _, pitch := range []model.Deg{-7, 0, 12} {
			s := base
			s.RailPitchDeg = pitch
			for _, sta := range []model.Deg{60, 72, 80} {
				for _, post := range []float64{0, 15, 25} {
					for _, offset := range []float64{-15, 0, 15} {
						const axial = 700.0
						a, p := sta.Angle(), pitch.Angle()
						clamp := geom2d.Point2{
							X: -axial*a.Cos() - post*a.Sin(),
							Y: axial*a.Sin() - post*a.Cos(),
						}
						center := geom2d.Point2{
							X: clamp.X - offset*p.Cos(),
							Y: clamp.Y - offset*p.Sin(),
						}
						brp := geom2d.Point2{
							X: center.X + s.RailCenterToBrpMm*p.Cos(),
							Y: center.Y + s.RailCenterToBrpMm*p.Sin(),
						}
						sol, err := geometry.SolveSaddleSTAForBRPTarget(s, brp.DistanceToOrigin(), -brp.X, post, offset)
						if err != nil {
							t.Fatalf("%s pitch=%v sta=%v post=%v off=%v: %v", s.ID, pitch, sta, post, offset, err)
						}
						if (sol.EffStaDeg() - sta).Abs() > 1e-9 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: got STA %.12f°", s.ID, pitch, sta, post, offset, sol.EffStaDeg())
						}
						if math.Abs(sol.Placement().AxialDistanceMm()-axial) > 1e-8 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: got axial %.10f mm", s.ID, pitch, sta, post, offset, sol.Placement().AxialDistanceMm())
						}
						if d := sol.Placement().BRP().DistanceTo(brp); d > 1e-8 {
							t.Errorf("%s pitch=%v sta=%v post=%v off=%v: BRP residual %.3e mm", s.ID, pitch, sta, post, offset, d)
						}
					}
				}
			}
		}
	}
}

func TestSolveSaddleSTAForBRPTarget_BranchesAndBoundaries(t *testing.T) {
	ref := model.ReferenceIdmatchSaddle()

	// Zero-radius clamp ambiguity (clamp == (0,0) with post=0): 90° pitched saddle puts BRP at (0, 32) while clamp is at BB.
	pitched90 := ref
	pitched90.RailPitchDeg = 90
	if _, err := geometry.SolveSaddleSTAForBRPTarget(pitched90, 32, 0, 0, 0); err == nil {
		t.Error("expected error for zero clamp radius ambiguity")
	}

	// Post setback exceeding clamp radius.
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, 100, 30, 200, 0); err == nil {
		t.Error("expected error when post setback exceeds clamp radius")
	}

	// Tangent zero distance (rho == p > 0) with legal STA in (0°, 90°) and BRP.Y > 0.
	// With RailPitchDeg=90, RailCenterToBrpMm=32: disp=(0, 32). Target BRP=(-12, 16) (radius=20, setback=12)
	// gives clamp=(-12, -16), rho=20 == postSetback=20, axial=0, STA = atan2(-16, 12) + 90° ≈ 36.87°.
	solT, err := geometry.SolveSaddleSTAForBRPTarget(pitched90, 20, 12, 20, 0)
	if err != nil {
		t.Fatalf("tangent zero-distance legal case failed: %v", err)
	}
	wantTangentSTA := model.DegOf(geom2d.AngleFromSlope(12, -16)) + 90
	if (solT.EffStaDeg()-wantTangentSTA).Abs() > 1e-9 || math.Abs(solT.Placement().AxialDistanceMm()) > 1e-8 {
		t.Errorf("tangent zero-distance: got STA=%.9f° (want %.9f°), axial=%.9f mm", solT.EffStaDeg(), wantTangentSTA, solT.Placement().AxialDistanceMm())
	}

	// Out-of-domain branches: STA = 90° (clamp directly above BB), STA > 90° (clamp forward of BB), STA <= 0°.
	pitched30 := ref
	pitched30.RailPitchDeg = 30
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, math.Hypot(32, 500), -32, 0, 0); err == nil {
		t.Error("expected error for 90° STA endpoint")
	}
	if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, 500, -100, 0, 0); err == nil {
		t.Error("expected error for STA > 90°")
	}
	if _, err := geometry.SolveSaddleSTAForBRPTarget(pitched30, 10, 0, 0, 0); err == nil {
		t.Error("expected error for STA <= 0°")
	}

	// Invalid targets, NaN, Inf, overflow, negative post setback.
	badCases := []struct {
		name                           string
		radius, setback, post, railOff float64
	}{
		{"zero radius", 0, 0, 0, 0},
		{"negative radius", -746, 200, 0, 0},
		{"unreachable setback==radius", 500, 500, 0, 0},
		{"unreachable setback>radius", 500, 501, 0, 0},
		{"negative post setback", 746, 219, -1, 0},
		{"NaN radius", math.NaN(), 219, 0, 0},
		{"NaN setback", 746, math.NaN(), 0, 0},
		{"NaN post", 746, 219, math.NaN(), 0},
		{"NaN railOffset", 746, 219, 0, math.NaN()},
		{"Inf radius", math.Inf(1), 219, 0, 0},
		{"Inf setback", 746, math.Inf(-1), 0, 0},
		{"Inf post", 746, 219, math.Inf(1), 0},
		{"Inf railOffset", 746, 219, 0, math.Inf(1)},
		{"overflow radius", 1e200, 219, 0, 0},
	}
	for _, tc := range badCases {
		if _, err := geometry.SolveSaddleSTAForBRPTarget(ref, tc.radius, tc.setback, tc.post, tc.railOff); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}

	// Zero SaddleStaSolution must panic on accessors.
	var zero geometry.SaddleStaSolution
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"EffStaDeg", func() { _ = zero.EffStaDeg() }},
		{"Placement", func() { _ = zero.Placement() }},
	} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("zero SaddleStaSolution.%s() did not panic", tc.name)
				}
			}()
			tc.fn()
		}()
	}
}

func TestIdealAndCompromiseSTA_GeometryVsHardware(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	// 1. Invalid clamp or rail dimensions error.
	for _, badClamp := range []float64{0, -5, math.NaN(), math.Inf(1)} {
		if _, err := geometry.CalculateIdealSeatTubeAngle(sqlab, 299, 746, 15, "15mm", badClamp); err == nil {
			t.Errorf("CalculateIdealSeatTubeAngle: expected error for clampLength=%v", badClamp)
		}
	}
	for _, badRail := range []float64{0, -10, math.NaN(), math.Inf(1)} {
		badSaddle := sqlab
		badSaddle.RailLengthMm = badRail
		if _, err := geometry.CalculateIdealSeatTubeAngle(badSaddle, 299, 746, 15, "15mm", 30); err == nil {
			t.Errorf("CalculateIdealSeatTubeAngle: expected error for RailLengthMm=%v", badRail)
		}
		if _, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{badSaddle, aspin}, 299, 746, 15, "15mm", 30); err == nil {
			t.Errorf("CalculateUniversalCompromiseSTA: expected error for RailLengthMm=%v", badRail)
		}
	}

	// 2. Overlong positive clamp reports unavailable hardware range while keeping centered geometric result.
	overlongClamp := sqlab.RailLengthMm + 10
	resOverlong, err := geometry.CalculateIdealSeatTubeAngle(sqlab, 299, 746, 15, "15mm", overlongClamp)
	if err != nil {
		t.Fatalf("overlong clamp should not error for valid centered geometry: %v", err)
	}
	if resOverlong.HasClampWindow {
		t.Error("expected HasClampWindow=false for overlong clamp")
	}
	if (resOverlong.IdealStaDeg - 71.75).Abs() > 0.01 {
		t.Errorf("expected centered IdealStaDeg ~ 71.75°, got %.2f°", resOverlong.IdealStaDeg)
	}

	// 3. Impossible endpoint when hardware fits must error (naming saddle and endpoint), not set HasClampWindow=false.
	// With sitBoneSetback=51, saddleHeight=746, post=0, centered STA is < 90° (clamp.X=-3), while +5 mm margin
	// pushes clamp forward of BB (clamp.X=+2, STA > 90°).
	longRailSaddle := sqlab
	longRailSaddle.RailLengthMm = 300
	if _, err := geometry.CalculateIdealSeatTubeAngle(longRailSaddle, 51, 746, 0, "0mm", 30); err == nil || !strings.Contains(err.Error(), longRailSaddle.ID) {
		t.Errorf("expected contextual error naming saddle %q for impossible endpoint, got %v", longRailSaddle.ID, err)
	}
	if _, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{longRailSaddle, aspin}, 51, 746, 0, "0mm", 30); err == nil || !strings.Contains(err.Error(), longRailSaddle.ID) {
		t.Errorf("expected contextual compromise error naming saddle %q, got %v", longRailSaddle.ID, err)
	}

	// 4. Disjoint valid margin intervals remain a legitimate HasUniversalMargin=false result.
	compDisjoint, err := geometry.CalculateUniversalCompromiseSTA([]model.SaddleSpec{sqlab, aspin, slr}, 299, 746, 15, "15mm", 30)
	if err != nil {
		t.Fatalf("disjoint valid intervals should not error: %v", err)
	}
	if compDisjoint.HasUniversalMargin {
		t.Error("expected HasUniversalMargin=false for disjoint 3-saddle margin intervals")
	}
}
