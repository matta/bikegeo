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
) (assembly.StemPlacement, assembly.HandlebarPlacement) {
	t.Helper()
	stemPlace := mustAssembleStem(t, pos, htaDeg, coverMm, spacersMm, clampHeightMm, stemLengthMm, stemAngleDeg)
	bar, err := assembly.NewHandlebar(spec, gripInsetMm)
	if err != nil {
		t.Fatalf("NewHandlebar(%q, %v): %v", spec.Name, gripInsetMm, err)
	}
	barPlace, err := stemPlace.BarClampMount().AttachHandlebar(bar, rollDeg)
	if err != nil {
		t.Fatalf("AttachHandlebar(%v): %v", rollDeg, err)
	}
	return stemPlace, barPlace
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
	stemPlace, barPlace := mustMountCockpit(
		t, geom2d.Point2{X: 400, Y: 600}, 70, 10, 15, 40, 80, -6,
		knuckleSpec, 65, 10,
	)

	dx0 := -250.0 * math.Tan(16.0*math.Pi/180.0)
	dy0 := 30.0 + 250.0*math.Tan(4.0*math.Pi/180.0)
	c10, s10 := math.Cos(10.0*math.Pi/180.0), math.Sin(10.0*math.Pi/180.0)
	wantOff := geom2d.Vec2{X: dx0*c10 + dy0*s10, Y: -dx0*s10 + dy0*c10}
	wantGrip := stemPlace.BarClamp().Add(wantOff)

	assertPointNear(t, "Knuckle HandGrip(+10°)", barPlace.HandGrip(), wantGrip, 1e-11)

	// Reversing roll sign (-10°) must negate the roll contribution on a nonzero-rise/sweep bar.
	_, barNeg := mustMountCockpit(
		t, geom2d.Point2{X: 400, Y: 600}, 70, 10, 15, 40, 80, -6,
		knuckleSpec, 65, -10,
	)
	wantOffNeg := geom2d.Vec2{X: dx0*c10 - dy0*s10, Y: dx0*s10 + dy0*c10}
	assertPointNear(t, "Knuckle HandGrip(-10°)", barNeg.HandGrip(), stemPlace.BarClamp().Add(wantOffNeg), 1e-11)

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
	stemPlace2, barPlace2 := mustMountCockpit(
		t, geom2d.Origin(), 65, 10, 20, 40, 60, 0,
		cutBarEndSpec, 65, 0,
	)
	yk := 50.0 - 265.0*math.Tan(4.0*math.Pi/180.0)
	wantOff2 := geom2d.Vec2{
		X: -180.0 * math.Tan(20.0*math.Pi/180.0),
		Y: yk + 180.0*math.Tan(4.0*math.Pi/180.0),
	}
	assertPointNear(t, "BarEndCenter Cut HandGrip", barPlace2.HandGrip(), stemPlace2.BarClamp().Add(wantOff2), 1e-11)

	// 3. Narrow bar where zHand <= EffectiveBendStartMm => lz clamps to 0 (no sweep/upsweep outboard).
	narrowSpec := knuckleSpec
	narrowSpec.CurrentWidthMm = 280 // zHand = 140 - 65 = 75 < 85
	stemPlace3, barPlace3 := mustMountCockpit(
		t, geom2d.Origin(), 70, 10, 0, 40, 70, 0,
		narrowSpec, 65, 0,
	)
	assertPointNear(t, "Clamped lz HandGrip", barPlace3.HandGrip(), stemPlace3.BarClamp().Add(geom2d.Vec2{X: 0, Y: 30}), 1e-12)
}

func TestHandlebarWorldRelativeRollAcrossAttitudesAndStems(t *testing.T) {
	// Bar roll is defined relative to the leveled bicycle world frame (+X forward, +Y up),
	// so changing frame HTA or stem angle moves BarClamp but preserves the world-frame
	// vector (HandGrip - BarClamp) for a given bar and roll angle.
	barSpec := catalog.HandlebarPresets["sqlab3OX_16_30"]
	const rollDeg = model.Deg(-7.5)

	refStemPlace, refBarPlace := mustMountCockpit(
		t, geom2d.Point2{X: 430, Y: 625}, 70, 11, 15, 40, 80, -6,
		barSpec, model.DefaultGripInsetMm, rollDeg,
	)
	refVec := refBarPlace.HandGrip().Sub(refStemPlace.BarClamp())

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
		sp, bp := mustMountCockpit(
			t, geom2d.Point2{X: 430, Y: 625}, cfg.hta, 11, 15, 40, 80, cfg.stemAng,
			barSpec, model.DefaultGripInsetMm, rollDeg,
		)
		gotVec := bp.HandGrip().Sub(sp.BarClamp())
		if math.Abs(gotVec.X-refVec.X) > 1e-11 || math.Abs(gotVec.Y-refVec.Y) > 1e-11 {
			t.Errorf("HTA=%.1f StemAng=%.1f: grip offset (%.12f, %.12f) != ref (%.12f, %.12f)",
				cfg.hta, cfg.stemAng, gotVec.X, gotVec.Y, refVec.X, refVec.Y)
		}
	}

	// Immutable reuse of StemPlacement.BarClampMount() across different bars and roll angles.
	stemPlace := mustAssembleStem(t, geom2d.Point2{X: 430, Y: 625}, 69.5, 11, 10, 40, 80, -6)
	mount := stemPlace.BarClampMount()
	barA, err := assembly.NewHandlebar(catalog.HandlebarPresets["persuader20_800"], model.DefaultGripInsetMm)
	if err != nil {
		t.Fatal(err)
	}
	barB, err := assembly.NewHandlebar(catalog.HandlebarPresets["sqlab3OX_16_45"], model.DefaultGripInsetMm)
	if err != nil {
		t.Fatal(err)
	}
	placeA, err := mount.AttachHandlebar(barA, 0)
	if err != nil {
		t.Fatal(err)
	}
	placeB, err := mount.AttachHandlebar(barB, 12)
	if err != nil {
		t.Fatal(err)
	}
	if placeA.HandGrip().DistanceTo(placeB.HandGrip()) < 10.0 {
		t.Errorf("expected different bars/rolls on same mount to yield distinct HandGrip positions")
	}
}

func TestHandlebarCatalogAndBoundaryValidation(t *testing.T) {
	for _, b := range catalog.AllHandlebars() {
		if _, err := assembly.NewHandlebar(b, model.DefaultGripInsetMm); err != nil {
			t.Errorf("catalog handlebar %q failed validation: %v", b.ID, err)
		}
	}

	validSpec := catalog.DefaultHandlebar()
	badSpecs := []struct {
		name  string
		mut   func(*model.HandlebarSpec)
		inset float64
	}{
		{"zero width", func(s *model.HandlebarSpec) { s.StockWidthMm = 0; s.CurrentWidthMm = 0 }, 65},
		{"negative width", func(s *model.HandlebarSpec) { s.StockWidthMm = -700; s.CurrentWidthMm = -700 }, 65},
		{"NaN width", func(s *model.HandlebarSpec) { s.StockWidthMm = math.NaN(); s.CurrentWidthMm = 0 }, 65},
		{"unknown rise mode", func(s *model.HandlebarSpec) { s.RiseMode = "bogus" }, 65},
		{"bar_end_center zero stock width", func(s *model.HandlebarSpec) {
			s.RiseMode = model.RiseModeBarEndCenter
			s.StockWidthMm = 0
			s.CurrentWidthMm = 760
		}, 65},
		{"negative bend start", func(s *model.HandlebarSpec) { s.EffectiveBendStartMm = -1 }, 65},
		{"NaN stated rise", func(s *model.HandlebarSpec) { s.StatedRiseMm = math.NaN() }, 65},
		{"backsweep 90", func(s *model.HandlebarSpec) { s.BacksweepDeg = 90 }, 65},
		{"upsweep -90", func(s *model.HandlebarSpec) { s.UpsweepDeg = -90 }, 65},
		{"zero inset", func(s *model.HandlebarSpec) {}, 0},
		{"negative inset", func(s *model.HandlebarSpec) {}, -10},
		{"inset >= half width", func(s *model.HandlebarSpec) {}, validSpec.EffectiveWidthMm() / 2.0},
		{"NaN inset", func(s *model.HandlebarSpec) {}, math.NaN()},
	}
	for _, tc := range badSpecs {
		t.Run(tc.name, func(t *testing.T) {
			s := validSpec
			tc.mut(&s)
			if _, err := assembly.NewHandlebar(s, tc.inset); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}

	t.Run("AttachHandlebar non-finite roll and overflow", func(t *testing.T) {
		stemPlace := mustAssembleStem(t, geom2d.Origin(), 70, 10, 0, 40, 80, 0)
		bar, err := assembly.NewHandlebar(validSpec, model.DefaultGripInsetMm)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stemPlace.BarClampMount().AttachHandlebar(bar, model.Deg(math.NaN())); err == nil {
			t.Errorf("NaN roll should return error")
		}
		if _, err := stemPlace.BarClampMount().AttachHandlebar(bar, model.Deg(math.Inf(1))); err == nil {
			t.Errorf("+Inf roll should return error")
		}

		htMax, err := assembly.NewHeadTubeTop(geom2d.Point2{X: 0, Y: math.MaxFloat64}, 90)
		if err != nil {
			t.Fatal(err)
		}
		sbMax, err := htMax.WithUpperStack(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		stMax, err := assembly.NewStem(40, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		spMax, err := sbMax.AttachStem(stMax)
		if err != nil {
			t.Fatal(err)
		}
		tallBarSpec := validSpec
		tallBarSpec.StatedRiseMm = math.MaxFloat64
		tallBar, err := assembly.NewHandlebar(tallBarSpec, model.DefaultGripInsetMm)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := spMax.BarClampMount().AttachHandlebar(tallBar, 0); err == nil {
			t.Errorf("world Y overflow in AttachHandlebar should return error")
		}
	})
}

func TestHandlebarZeroValueAndIgnoredErrorPanics(t *testing.T) {
	stemPlace := mustAssembleStem(t, geom2d.Origin(), 70, 10, 0, 40, 80, 0)
	validMount := stemPlace.BarClampMount()
	validBar, err := assembly.NewHandlebar(catalog.DefaultHandlebar(), model.DefaultGripInsetMm)
	if err != nil {
		t.Fatal(err)
	}

	assertPanics(t, "zero BarClampMount AttachHandlebar", func() {
		var m assembly.BarClampMount
		_, _ = m.AttachHandlebar(validBar, 0)
	})
	assertPanics(t, "zero Handlebar in AttachHandlebar", func() {
		var b assembly.Handlebar
		_, _ = validMount.AttachHandlebar(b, 0)
	})
	assertPanics(t, "ignored error Handlebar in AttachHandlebar", func() {
		b, _ := assembly.NewHandlebar(catalog.DefaultHandlebar(), -1)
		_, _ = validMount.AttachHandlebar(b, 0)
	})
	assertPanics(t, "zero Handlebar GripOffset3D", func() {
		var b assembly.Handlebar
		_, _ = b.GripOffset3D(0)
	})
	assertPanics(t, "zero HandlebarPlacement HandGrip", func() {
		var p assembly.HandlebarPlacement
		_ = p.HandGrip()
	})
	assertPanics(t, "ignored error HandlebarPlacement HandGrip", func() {
		p, _ := validMount.AttachHandlebar(validBar, model.Deg(math.NaN()))
		_ = p.HandGrip()
	})
}
