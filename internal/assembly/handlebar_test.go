package assembly_test

import (
	"math"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func mustMountCockpit(
	t *testing.T,
	pos geom2d.Point2,
	htaDeg model.Deg,
	coverMm, spacersMm float64,
	clampHeightMm, stemLengthMm float64,
	stemAngleDeg model.Deg,
	spec model.HandlebarSpec,
	gripInsetMm float64,
	rollDeg model.Deg,
) (assembly.StemPlacement, geom2d.Point2) {
	t.Helper()
	stemPlace := mustAssembleStem(t, pos, htaDeg, coverMm, spacersMm, clampHeightMm, stemLengthMm, stemAngleDeg)
	handGrip, err := assembly.PlaceHandlebar(stemPlace.BarClamp, assembly.HandlebarSetup{
		Spec:        spec,
		GripInsetMm: gripInsetMm,
		RollDeg:     rollDeg,
	})
	if err != nil {
		t.Fatalf("PlaceHandlebar(%q, inset=%v, roll=%v): %v", spec.Name, gripInsetMm, rollDeg, err)
	}
	return stemPlace, handGrip
}

func TestHandlebarWorkedExamplesAndRiseModes(t *testing.T) {
	// 1. Standard knuckle mode with nonzero roll:
	//    width=800, inset=65 => zHand = 335, zb = 85 => lz = 250
	//    backsweep=16°, upsweep=4°, statedRise=30 => dx0 = -250*tan(16°), dy0 = 30 + 250*tan(4°)
	//    At roll = +10° (top rolled forward, CW in +X/+Y):
	//      dx = dx0*cos(10°) + dy0*sin(10°)
	//      dy = -dx0*sin(10°) + dy0*cos(10°)
	knuckleSpec := model.HandlebarSpec{
		ID:                   "test-knuckle",
		Name:                 "Test Knuckle",
		StockWidthMm:         800,
		CurrentWidthMm:       800,
		StatedRiseMm:         30,
		BacksweepDeg:         16,
		UpsweepDeg:           4,
		EffectiveBendStartMm: 85,
		RiseMode:             model.RiseModeStandardKnuckle,
	}
	stemPlace, handGrip := mustMountCockpit(
		t, geom2d.Point2{X: 400, Y: 600}, 70, 10, 15, 40, 80, -6,
		knuckleSpec, 65, 10,
	)

	dx0 := -250.0 * math.Tan(16.0*math.Pi/180.0)
	dy0 := 30.0 + 250.0*math.Tan(4.0*math.Pi/180.0)
	c10, s10 := math.Cos(10.0*math.Pi/180.0), math.Sin(10.0*math.Pi/180.0)
	wantOff := geom2d.Vec2{X: dx0*c10 + dy0*s10, Y: -dx0*s10 + dy0*c10}
	wantGrip := stemPlace.BarClamp.Add(wantOff)

	assertPointNear(t, "Knuckle HandGrip(+10°)", handGrip, wantGrip, 1e-11)

	off3D, err := assembly.HandlebarOffset3D(assembly.HandlebarSetup{
		Spec:        knuckleSpec,
		GripInsetMm: 65,
		RollDeg:     10,
	})
	if err != nil {
		t.Fatalf("HandlebarOffset3D: %v", err)
	}
	if math.Abs(off3D.X-wantOff.X) > 1e-11 || math.Abs(off3D.Y-wantOff.Y) > 1e-11 || math.Abs(off3D.Z-335.0) > 1e-12 {
		t.Errorf("HandlebarOffset3D = %+v, want (%.12f, %.12f, 335)", off3D, wantOff.X, wantOff.Y)
	}

	// Reversing roll sign (-10°) must negate the roll contribution on a nonzero-rise/sweep bar.
	_, gripNeg := mustMountCockpit(
		t, geom2d.Point2{X: 400, Y: 600}, 70, 10, 15, 40, 80, -6,
		knuckleSpec, 65, -10,
	)
	wantOffNeg := geom2d.Vec2{X: dx0*c10 - dy0*s10, Y: dx0*s10 + dy0*c10}
	assertPointNear(t, "Knuckle HandGrip(-10°)", gripNeg, stemPlace.BarClamp.Add(wantOffNeg), 1e-11)

	// 2. Bar-end center mode, cut narrower than stock width:
	//    StockWidth=800, CurrentWidth=760, inset=65, zb=135, upsweep=4°, backsweep=20°, statedRise=50
	//    y_k uses stockTipSpan = 400 - 135 = 265 => y_k = 50 - 265*tan(4°)
	//    zHand uses CurrentWidth: 380 - 65 = 315 => lz = 315 - 135 = 180
	cutBarEndSpec := model.HandlebarSpec{
		ID:                   "test-bar-end-cut",
		Name:                 "Test Bar-End Cut",
		StockWidthMm:         800,
		CurrentWidthMm:       760,
		StatedRiseMm:         50,
		BacksweepDeg:         20,
		UpsweepDeg:           4,
		EffectiveBendStartMm: 135,
		RiseMode:             model.RiseModeBarEndCenter,
	}
	stemPlace2, handGrip2 := mustMountCockpit(
		t, geom2d.Origin(), 65, 10, 20, 40, 60, 0,
		cutBarEndSpec, 65, 0,
	)
	yk := 50.0 - 265.0*math.Tan(4.0*math.Pi/180.0)
	wantOff2 := geom2d.Vec2{
		X: -180.0 * math.Tan(20.0*math.Pi/180.0),
		Y: yk + 180.0*math.Tan(4.0*math.Pi/180.0),
	}
	assertPointNear(t, "BarEndCenter Cut HandGrip", handGrip2, stemPlace2.BarClamp.Add(wantOff2), 1e-11)

	// 3. Narrow bar where zHand <= EffectiveBendStartMm => lz clamps to 0 (no sweep/upsweep outboard).
	narrowSpec := knuckleSpec
	narrowSpec.CurrentWidthMm = 280 // zHand = 140 - 65 = 75 < 85
	stemPlace3, handGrip3 := mustMountCockpit(
		t, geom2d.Origin(), 70, 10, 0, 40, 70, 0,
		narrowSpec, 65, 0,
	)
	assertPointNear(t, "Clamped lz HandGrip", handGrip3, stemPlace3.BarClamp.Add(geom2d.Vec2{X: 0, Y: 30}), 1e-12)
}

func TestHandlebarWorldRelativeRollAcrossAttitudesAndStems(t *testing.T) {
	// Bar roll is defined relative to the leveled bicycle world frame (+X forward, +Y up),
	// so changing frame HTA or stem angle moves BarClamp but preserves the world-frame
	// vector (HandGrip - BarClamp) for a given bar and roll angle.
	barSpec := catalog.HandlebarPresets["sqlab3OX_16_30"]
	const rollDeg = model.Deg(-7.5)

	refStemPlace, refGrip := mustMountCockpit(
		t, geom2d.Point2{X: 430, Y: 625}, 70, 11, 15, 40, 80, -6,
		barSpec, model.DefaultGripInsetMm, rollDeg,
	)
	refVec := refGrip.Sub(refStemPlace.BarClamp)

	configs := []struct {
		hta     model.Deg
		stemAng model.Deg
	}{
		{64.0, -17.0},
		{64.0, 17.0},
		{73.5, 0.0},
		{73.5, 25.0},
	}
	for _, cfg := range configs {
		sp, grip := mustMountCockpit(
			t, geom2d.Point2{X: 430, Y: 625}, cfg.hta, 11, 15, 40, 80, cfg.stemAng,
			barSpec, model.DefaultGripInsetMm, rollDeg,
		)
		gotVec := grip.Sub(sp.BarClamp)
		if math.Abs(gotVec.X-refVec.X) > 1e-11 || math.Abs(gotVec.Y-refVec.Y) > 1e-11 {
			t.Errorf("HTA=%.1f StemAng=%.1f: grip offset (%.12f, %.12f) != ref (%.12f, %.12f)",
				cfg.hta, cfg.stemAng, gotVec.X, gotVec.Y, refVec.X, refVec.Y)
		}
	}

	// Placing different bars and roll angles on the same BarClamp yields distinct HandGrip positions.
	stemPlace := mustAssembleStem(t, geom2d.Point2{X: 430, Y: 625}, 69.5, 11, 10, 40, 80, -6)
	gripA, err := assembly.PlaceHandlebar(stemPlace.BarClamp, assembly.HandlebarSetup{
		Spec:        catalog.HandlebarPresets["persuader20_800"],
		GripInsetMm: model.DefaultGripInsetMm,
		RollDeg:     0,
	})
	if err != nil {
		t.Fatal(err)
	}
	gripB, err := assembly.PlaceHandlebar(stemPlace.BarClamp, assembly.HandlebarSetup{
		Spec:        catalog.HandlebarPresets["sqlab3OX_16_45"],
		GripInsetMm: model.DefaultGripInsetMm,
		RollDeg:     12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gripA.DistanceTo(gripB) < 10.0 {
		t.Errorf("expected different bars/rolls on same clamp to yield distinct HandGrip positions")
	}
}

func TestHandlebarCatalogAndBoundaryValidation(t *testing.T) {
	validBarClamp := geom2d.Point2{X: 480, Y: 675}
	for _, b := range catalog.AllHandlebars() {
		setup := assembly.HandlebarSetup{
			Spec:        b,
			GripInsetMm: model.DefaultGripInsetMm,
			RollDeg:     0,
		}
		if _, err := assembly.PlaceHandlebar(validBarClamp, setup); err != nil {
			t.Errorf("catalog handlebar %q failed PlaceHandlebar: %v", b.ID, err)
		}
		if _, err := assembly.HandlebarOffset3D(setup); err != nil {
			t.Errorf("catalog handlebar %q failed HandlebarOffset3D: %v", b.ID, err)
		}
	}

	validSpec := catalog.DefaultHandlebar()
	baseSetup := assembly.HandlebarSetup{
		Spec:        validSpec,
		GripInsetMm: model.DefaultGripInsetMm,
		RollDeg:     0,
	}
	if _, err := assembly.PlaceHandlebar(validBarClamp, baseSetup); err != nil {
		t.Fatalf("baseline PlaceHandlebar failed: %v", err)
	}

	badSpecs := []struct {
		name     string
		barClamp geom2d.Point2
		mut      func(*assembly.HandlebarSetup)
	}{
		{"NaN barClamp X", geom2d.Point2{X: math.NaN(), Y: 600}, nil},
		{"+Inf barClamp Y", geom2d.Point2{X: 400, Y: math.Inf(1)}, nil},
		{"zero width", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.StockWidthMm = 0; s.Spec.CurrentWidthMm = 0 }},
		{"negative width", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.StockWidthMm = -700; s.Spec.CurrentWidthMm = -700 }},
		{"NaN width", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.StockWidthMm = math.NaN(); s.Spec.CurrentWidthMm = 0 }},
		{"unknown rise mode", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.RiseMode = "bogus" }},
		{"bar_end_center zero stock width", validBarClamp, func(s *assembly.HandlebarSetup) {
			s.Spec.RiseMode = model.RiseModeBarEndCenter
			s.Spec.StockWidthMm = 0
			s.Spec.CurrentWidthMm = 760
		}},
		{"negative bend start", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.EffectiveBendStartMm = -1 }},
		{"NaN stated rise", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.StatedRiseMm = math.NaN() }},
		{"backsweep 90", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.BacksweepDeg = 90 }},
		{"upsweep -90", validBarClamp, func(s *assembly.HandlebarSetup) { s.Spec.UpsweepDeg = -90 }},
		{"zero inset", validBarClamp, func(s *assembly.HandlebarSetup) { s.GripInsetMm = 0 }},
		{"negative inset", validBarClamp, func(s *assembly.HandlebarSetup) { s.GripInsetMm = -10 }},
		{"inset >= half width", validBarClamp, func(s *assembly.HandlebarSetup) { s.GripInsetMm = validSpec.EffectiveWidthMm() / 2.0 }},
		{"NaN inset", validBarClamp, func(s *assembly.HandlebarSetup) { s.GripInsetMm = math.NaN() }},
		{"NaN roll", validBarClamp, func(s *assembly.HandlebarSetup) { s.RollDeg = model.Deg(math.NaN()) }},
		{"+Inf roll", validBarClamp, func(s *assembly.HandlebarSetup) { s.RollDeg = model.Deg(math.Inf(1)) }},
		{"world Y overflow", geom2d.Point2{X: 0, Y: math.MaxFloat64}, func(s *assembly.HandlebarSetup) { s.Spec.StatedRiseMm = math.MaxFloat64 }},
	}
	for _, tc := range badSpecs {
		t.Run(tc.name, func(t *testing.T) {
			s := baseSetup
			if tc.mut != nil {
				tc.mut(&s)
			}
			if _, err := assembly.PlaceHandlebar(tc.barClamp, s); err == nil {
				t.Errorf("%s: expected error from PlaceHandlebar, got nil", tc.name)
			}
		})
	}
}
