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
	badSaddle := model.ReferenceIdmatchSaddle()
	badSaddle.RailCenterToBrpMm = math.NaN()

	// 1. BuildScene must fail.
	if _, err := geometry.BuildScene(
		frame,
		build,
		geometry.CockpitSpec{
			HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
			Stem:              bike.GetCanonicalStem(),
			StemClampHeightMm: model.StemThicknessMm,
			Handlebar:         catalog.DefaultHandlebar(),
			GripInsetMm:       model.DefaultGripInsetMm,
		},
		geometry.SeatSpec{
			Saddle:        badSaddle,
			PostSetbackMm: model.DefaultSeatpostSetbackMm,
			ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		},
		geometry.RiderFit{BrpHeightMm: profile.SaddleHeightMm},
	); err == nil {
		t.Error("BuildScene accepted NaN saddle geometry, want error")
	}

	// 2. AnalyzeBikeFitWithSaddle must fail.
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, profile, badSaddle); err == nil {
		t.Error("AnalyzeBikeFitWithSaddle accepted NaN saddle geometry, want error")
	}
}
