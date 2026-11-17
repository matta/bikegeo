package assembly_test

import (
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func mustAssembleStem(
	t *testing.T,
	pos geom2d.Point2,
	htaDeg model.Deg,
	coverMm, spacersMm float64,
	clampHeightMm, stemLengthMm float64,
	stemAngleDeg model.Deg,
) assembly.StemPlacement {
	t.Helper()
	ht, err := assembly.NewHeadTubeTop(pos, htaDeg)
	if err != nil {
		t.Fatalf("NewHeadTubeTop(%+v, %v): %v", pos, htaDeg, err)
	}
	base, err := ht.WithUpperStack(coverMm, spacersMm)
	if err != nil {
		t.Fatalf("WithUpperStack(%v, %v): %v", coverMm, spacersMm, err)
	}
	stem, err := assembly.NewStem(clampHeightMm, stemLengthMm, stemAngleDeg)
	if err != nil {
		t.Fatalf("NewStem(%v, %v, %v): %v", clampHeightMm, stemLengthMm, stemAngleDeg, err)
	}
	placement, err := base.AttachStem(stem)
	if err != nil {
		t.Fatalf("AttachStem: %v", err)
	}
	return placement
}

func assertPointNear(t *testing.T, label string, got, want geom2d.Point2, tol float64) {
	t.Helper()
	if d := got.DistanceTo(want); d > tol || math.IsNaN(d) {
		t.Errorf("%s = (%.12f, %.12f), want (%.12f, %.12f) (dist=%.3e > tol=%.3e)",
			label, got.X, got.Y, want.X, want.Y, d, tol)
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic, succeeded silently", name)
			}
		}()
		fn()
	})
}

func TestIndependent60DegWorkedExample(t *testing.T) {
	// With head-tube top (0, 0), cover 10, spacers 20, clamp height 40,
	// stem length 100 and angle 0° at 60° HTA:
	//   steerer advance = 10 + 20 + 40/2 = 50 mm along (-cos 60°, sin 60°) = (-1/2, √3/2)
	//   => SteererClamp = (-25, 25√3)
	//   stem direction = perpendicular forward (sin 60°, cos 60°) = (√3/2, 1/2)
	//   => BarClamp = (50√3 - 25, 50 + 25√3)
	p := mustAssembleStem(t, geom2d.Point2{X: 0, Y: 0}, 60.0, 10.0, 20.0, 40.0, 100.0, 0.0)

	wantSteerer := geom2d.Point2{X: -25.0, Y: 25.0 * math.Sqrt(3)}
	wantBar := geom2d.Point2{X: 50.0*math.Sqrt(3) - 25.0, Y: 50.0 + 25.0*math.Sqrt(3)}

	assertPointNear(t, "SteererClamp", p.SteererClamp(), wantSteerer, 1e-12)
	assertPointNear(t, "BarClamp", p.BarClamp(), wantBar, 1e-12)
}

func TestStemLocalFrameIndependentOfGeom2DGlobals(t *testing.T) {
	origin, unitX, unitY := geom2d.Origin, geom2d.UnitX, geom2d.UnitY
	t.Cleanup(func() { geom2d.Origin, geom2d.UnitX, geom2d.UnitY = origin, unitX, unitY })
	// This shifted, rotated basis is still a valid pose, so a general pose check
	// would accept it. The stem's local identity must be fixed independently.
	geom2d.Origin = geom2d.Point2{X: 100}
	geom2d.UnitX = geom2d.Vec2{Y: 1}
	geom2d.UnitY = geom2d.Vec2{X: -1}

	p := mustAssembleStem(t, geom2d.Point2{}, 90, 10, 20, 40, 80, 0)
	assertPointNear(t, "SteererClamp", p.SteererClamp(), geom2d.Point2{Y: 50}, 1e-12)
	assertPointNear(t, "BarClamp", p.BarClamp(), geom2d.Point2{X: 80, Y: 50}, 1e-12)
}

func TestNonzeroStemAngleAndIndependentFaultSensitivity(t *testing.T) {
	// At HTA = 70°, perpendicular forward is 90° - 70° = 20° above +X.
	// Case A: stem angle = -20° cancels the 20° slope so the stem points strictly +X (horizontal).
	//   dSteerer = 11 (cover) + 15 (spacers) + 20 (40/2 half-clamp) = 46 mm
	//   SteererClamp = (-46*cos(70°), 46*sin(70°))
	//   BarClamp     = SteererClamp + (80, 0)
	pNeg20 := mustAssembleStem(t, geom2d.Origin, 70.0, 11.0, 15.0, 40.0, 80.0, -20.0)
	sin70 := math.Sin(70.0 * math.Pi / 180.0)
	cos70 := math.Cos(70.0 * math.Pi / 180.0)
	wantSC := geom2d.Point2{X: -46.0 * cos70, Y: 46.0 * sin70}
	wantBCNeg20 := geom2d.Point2{X: wantSC.X + 80.0, Y: wantSC.Y}
	assertPointNear(t, "SteererClamp(70°,-20°)", pNeg20.SteererClamp(), wantSC, 1e-11)
	assertPointNear(t, "BarClamp(70°,-20°)", pNeg20.BarClamp(), wantBCNeg20, 1e-11)

	// Reversing stem angle (+20° instead of -20°) aims at 40° above +X instead of 0°:
	// detects sign-inversion faults that zero-angle tests cannot see.
	pPos20 := mustAssembleStem(t, geom2d.Origin, 70.0, 11.0, 15.0, 40.0, 80.0, 20.0)
	cos40 := math.Cos(40.0 * math.Pi / 180.0)
	sin40 := math.Sin(40.0 * math.Pi / 180.0)
	wantBCPos20 := geom2d.Point2{X: wantSC.X + 80.0*cos40, Y: wantSC.Y + 80.0*sin40}
	assertPointNear(t, "BarClamp(70°,+20°)", pPos20.BarClamp(), wantBCPos20, 1e-11)
	if pPos20.BarClamp().DistanceTo(pNeg20.BarClamp()) < 50.0 {
		t.Fatalf("expected reversed stem angle to diverge significantly")
	}

	// Verify sensitivity to cover and half-clamp height independently:
	pNoCover := mustAssembleStem(t, geom2d.Origin, 70.0, 0.0, 15.0, 40.0, 80.0, -20.0)
	if d := pNeg20.SteererClamp().DistanceTo(pNoCover.SteererClamp()); math.Abs(d-11.0) > 1e-11 {
		t.Errorf("cover contribution = %.12f, want 11.0", d)
	}
	pSmallClamp := mustAssembleStem(t, geom2d.Origin, 70.0, 11.0, 15.0, 20.0, 80.0, -20.0)
	if d := pNeg20.SteererClamp().DistanceTo(pSmallClamp.SteererClamp()); math.Abs(d-10.0) > 1e-11 {
		t.Errorf("half-clamp contribution delta = %.12f, want 10.0", d)
	}
}

func TestRigidTransformationProperty(t *testing.T) {
	// Rotating and translating the parent head-tube top must transform both
	// SteererClamp and BarClamp once under the same rigid motion.
	// Any duplicated or omitted parent attitude fails this test.
	cases := []struct {
		name      string
		pos       geom2d.Point2
		hta       model.Deg
		cover     float64
		spacers   float64
		clampH    float64
		stemLen   float64
		stemAng   model.Deg
		pivot     geom2d.Point2
		rotCCWDeg model.Deg
		trans     geom2d.Vec2
	}{
		{
			name:      "CCW rotation around BB with negative stem angle",
			pos:       geom2d.Point2{X: 424, Y: 631},
			hta:       70.0,
			cover:     11.0,
			spacers:   15.0,
			clampH:    40.0,
			stemLen:   80.0,
			stemAng:   -17.0,
			pivot:     geom2d.Origin,
			rotCCWDeg: 12.0, // new HTA = 58°
			trans:     geom2d.Vec2{X: -35.0, Y: 42.0},
		},
		{
			name:      "CW rotation around rear axle with positive stem angle",
			pos:       geom2d.Point2{X: 450, Y: 610},
			hta:       68.5,
			cover:     8.0,
			spacers:   25.0,
			clampH:    42.0,
			stemLen:   90.0,
			stemAng:   10.0,
			pivot:     geom2d.Point2{X: -435, Y: 65},
			rotCCWDeg: -14.0, // new HTA = 82.5°
			trans:     geom2d.Vec2{X: 18.0, Y: -27.5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p0 := mustAssembleStem(t, tc.pos, tc.hta, tc.cover, tc.spacers, tc.clampH, tc.stemLen, tc.stemAng)

			rot := geom2d.NewRotation(tc.pivot, tc.rotCCWDeg.Angle())
			transform := func(pt geom2d.Point2) geom2d.Point2 {
				return rot.Apply(pt).Add(tc.trans)
			}

			ht1Pos := transform(tc.pos)
			ht1Hta := tc.hta - tc.rotCCWDeg
			p1 := mustAssembleStem(t, ht1Pos, ht1Hta, tc.cover, tc.spacers, tc.clampH, tc.stemLen, tc.stemAng)

			assertPointNear(t, "SteererClamp", p1.SteererClamp(), transform(p0.SteererClamp()), 1e-11)
			assertPointNear(t, "BarClamp", p1.BarClamp(), transform(p0.BarClamp()), 1e-11)
		})
	}
}

func TestDimensionPropertiesAndImmutableReuse(t *testing.T) {
	pos := geom2d.Point2{X: 430, Y: 625}
	hta := model.Deg(69.5)
	cover := 11.0
	spacers := 10.0
	clampH := 40.0

	ht, err := assembly.NewHeadTubeTop(pos, hta)
	if err != nil {
		t.Fatal(err)
	}
	base1, err := ht.WithUpperStack(cover, spacers)
	if err != nil {
		t.Fatal(err)
	}

	// Reusing the immutable StemBase for two different stems must yield independent placements.
	stemA, err := assembly.NewStem(clampH, 70.0, -6.0)
	if err != nil {
		t.Fatal(err)
	}
	stemB, err := assembly.NewStem(clampH, 110.0, 17.0)
	if err != nil {
		t.Fatal(err)
	}
	placeA, err := base1.AttachStem(stemA)
	if err != nil {
		t.Fatal(err)
	}

	placeB, err := base1.AttachStem(stemB)
	if err != nil {
		t.Fatal(err)
	}

	// Both stems share the same steerer clamp center (same clamp height on same base).
	assertPointNear(t, "shared SteererClamp", placeA.SteererClamp(), placeB.SteererClamp(), 1e-12)

	// Clamp separation equals stem length.
	if d := placeA.SteererClamp().DistanceTo(placeA.BarClamp()); math.Abs(d-70.0) > 1e-11 {
		t.Errorf("stemA separation = %.12f, want 70.0", d)
	}
	if d := placeB.SteererClamp().DistanceTo(placeB.BarClamp()); math.Abs(d-110.0) > 1e-11 {
		t.Errorf("stemB separation = %.12f, want 110.0", d)
	}

	// Distance from head-tube top to steerer clamp equals cover + spacers + clampH/2.
	if d := pos.DistanceTo(placeA.SteererClamp()); math.Abs(d-(cover+spacers+clampH/2.0)) > 1e-11 {
		t.Errorf("steerer advance = %.12f, want %.12f", d, cover+spacers+clampH/2.0)
	}

	// Adding delta spacers translates both landmarks along the steerer axis by delta.
	delta := 15.0
	base2, err := ht.WithUpperStack(cover, spacers+delta)
	if err != nil {
		t.Fatal(err)
	}
	placeA2, err := base2.AttachStem(stemA)
	if err != nil {
		t.Fatal(err)
	}

	steerStep := geom2d.FromPolar(delta, (180.0 - hta).Angle())
	assertPointNear(t, "SteererClamp+delta", placeA2.SteererClamp(), placeA.SteererClamp().Add(steerStep), 1e-11)
	assertPointNear(t, "BarClamp+delta", placeA2.BarClamp(), placeA.BarClamp().Add(steerStep), 1e-11)
}

func TestBoundaryValidation(t *testing.T) {
	t.Run("NewHeadTubeTop", func(t *testing.T) {
		badCases := []struct {
			name string
			pos  geom2d.Point2
			hta  model.Deg
		}{
			{"NaN X", geom2d.Point2{X: math.NaN(), Y: 600}, 70},
			{"+Inf X", geom2d.Point2{X: math.Inf(1), Y: 600}, 70},
			{"-Inf Y", geom2d.Point2{X: 400, Y: math.Inf(-1)}, 70},
			{"HTA 0", geom2d.Origin, 0},
			{"HTA negative", geom2d.Origin, -10},
			{"HTA > 90", geom2d.Origin, 90.1},
			{"HTA 180", geom2d.Origin, 180},
			{"HTA NaN", geom2d.Origin, model.Deg(math.NaN())},
			{"HTA +Inf", geom2d.Origin, model.Deg(math.Inf(1))},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := assembly.NewHeadTubeTop(tc.pos, tc.hta); err == nil {
					t.Errorf("%s: expected error, got nil", tc.name)
				}
			})
		}
		for _, hta := range []model.Deg{0.1, 60.0, 90.0} {
			if _, err := assembly.NewHeadTubeTop(geom2d.Origin, hta); err != nil {
				t.Errorf("HTA %v should be valid, got %v", hta, err)
			}
		}
	})

	t.Run("WithUpperStack", func(t *testing.T) {
		ht, err := assembly.NewHeadTubeTop(geom2d.Origin, 70)
		if err != nil {
			t.Fatal(err)
		}
		badStacks := []struct {
			name    string
			cover   float64
			spacers float64
		}{
			{"negative cover", -0.1, 0},
			{"negative spacers", 0, -5},
			{"NaN cover", math.NaN(), 0},
			{"NaN spacers", 0, math.NaN()},
			{"+Inf cover", math.Inf(1), 0},
			{"-Inf spacers", 0, math.Inf(-1)},
			{"sum overflow", math.MaxFloat64, math.MaxFloat64},
		}
		for _, tc := range badStacks {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := ht.WithUpperStack(tc.cover, tc.spacers); err == nil {
					t.Errorf("%s: expected error, got nil", tc.name)
				}
			})
		}
		// Zero cover and zero spacers remain legal.
		if _, err := ht.WithUpperStack(0, 0); err != nil {
			t.Errorf("zero cover and spacers should be legal, got %v", err)
		}
		// World translation overflow on finite inputs.
		htHigh, err := assembly.NewHeadTubeTop(geom2d.Point2{X: 0, Y: math.MaxFloat64}, 90)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := htHigh.WithUpperStack(math.MaxFloat64, 0); err == nil {
			t.Errorf("translation overflow should return error, got nil")
		}
	})

	t.Run("NewStem", func(t *testing.T) {
		badStems := []struct {
			name   string
			clampH float64
			lenMm  float64
			ang    model.Deg
		}{
			{"zero clamp height", 0, 80, 0},
			{"negative clamp height", -10, 80, 0},
			{"NaN clamp height", math.NaN(), 80, 0},
			{"+Inf clamp height", math.Inf(1), 80, 0},
			{"zero length", 40, 0, 0},
			{"negative length", 40, -50, 0},
			{"NaN length", 40, math.NaN(), 0},
			{"+Inf length", 40, math.Inf(1), 0},
			{"NaN angle", 40, 80, model.Deg(math.NaN())},
			{"-Inf angle", 40, 80, model.Deg(math.Inf(-1))},
			{"local Y overflow", math.MaxFloat64, math.MaxFloat64, 90},
		}
		for _, tc := range badStems {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := assembly.NewStem(tc.clampH, tc.lenMm, tc.ang); err == nil {
					t.Errorf("%s: expected error, got nil", tc.name)
				}
			})
		}
	})

	t.Run("AttachStem overflow", func(t *testing.T) {
		ht, err := assembly.NewHeadTubeTop(geom2d.Point2{X: math.MaxFloat64, Y: 0}, 90)
		if err != nil {
			t.Fatal(err)
		}
		sb, err := ht.WithUpperStack(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		stem, err := assembly.NewStem(40, math.MaxFloat64, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sb.AttachStem(stem); err == nil {
			t.Errorf("world X overflow in AttachStem should return error, got nil")
		}
	})
}

func TestZeroValueAndIgnoredErrorMisusePanics(t *testing.T) {
	validHT, err := assembly.NewHeadTubeTop(geom2d.Origin, 70)
	if err != nil {
		t.Fatal(err)
	}
	validSB, err := validHT.WithUpperStack(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	validStem, err := assembly.NewStem(40, 80, -6)
	if err != nil {
		t.Fatal(err)
	}

	assertPanics(t, "zero HeadTubeTop WithUpperStack", func() {
		var ht assembly.HeadTubeTop
		_, _ = ht.WithUpperStack(10, 0)
	})
	assertPanics(t, "ignored error HeadTubeTop", func() {
		ht, _ := assembly.NewHeadTubeTop(geom2d.Point2{X: math.NaN()}, 70)
		_, _ = ht.WithUpperStack(10, 0)
	})
	assertPanics(t, "zero StemBase AttachStem", func() {
		var sb assembly.StemBase
		_, _ = sb.AttachStem(validStem)
	})
	assertPanics(t, "ignored error StemBase", func() {
		sb, _ := validHT.WithUpperStack(-1, 0)
		_, _ = sb.AttachStem(validStem)
	})
	assertPanics(t, "zero Stem in AttachStem", func() {
		var st assembly.Stem
		_, _ = validSB.AttachStem(st)
	})
	assertPanics(t, "ignored error Stem in AttachStem", func() {
		st, _ := assembly.NewStem(0, 80, 0)
		_, _ = validSB.AttachStem(st)
	})
	assertPanics(t, "zero StemPlacement SteererClamp", func() {
		var p assembly.StemPlacement
		_ = p.SteererClamp()
	})
	assertPanics(t, "zero StemPlacement BarClamp", func() {
		var p assembly.StemPlacement
		_ = p.BarClamp()
	})
	assertPanics(t, "ignored error StemPlacement SteererClamp", func() {
		htMax, _ := assembly.NewHeadTubeTop(geom2d.Point2{X: math.MaxFloat64, Y: 0}, 90)
		sbMax, _ := htMax.WithUpperStack(0, 0)
		stMax, _ := assembly.NewStem(40, math.MaxFloat64, 0)
		p, _ := sbMax.AttachStem(stMax)
		_ = p.SteererClamp()
	})
	assertPanics(t, "ignored error StemPlacement BarClamp", func() {
		htMax, _ := assembly.NewHeadTubeTop(geom2d.Point2{X: math.MaxFloat64, Y: 0}, 90)
		sbMax, _ := htMax.WithUpperStack(0, 0)
		stMax, _ := assembly.NewStem(40, math.MaxFloat64, 0)
		p, _ := sbMax.AttachStem(stMax)
		_ = p.BarClamp()
	})
}

func TestExternalCompileNegativeFixtures(t *testing.T) {
	cases := []struct {
		dir      string
		wantSub1 string
		wantSub2 string
	}{
		{
			dir:      "headtube_attach_stem",
			wantSub1: "HeadTubeTop",
			wantSub2: "AttachStem",
		},
		{
			dir:      "stembase_with_upper_stack",
			wantSub1: "StemBase",
			wantSub2: "WithUpperStack",
		},
		{
			dir:      "headtube_to_stembase",
			wantSub1: "cannot convert headTube",
			wantSub2: "to type assembly.StemBase",
		},
		{
			dir:      "stembase_to_headtube",
			wantSub1: "cannot convert stemBase",
			wantSub2: "to type assembly.HeadTubeTop",
		},
		{
			dir:      "local_offset_as_placed_mount",
			wantSub1: "cannot use",
			wantSub2: "assembly.StemBase",
		},
		{
			dir:      "unexported_field_forge",
			wantSub1: "cannot refer to unexported field valid",
			wantSub2: "assembly.HeadTubeTop",
		},
		{
			dir:      "stembase_attach_handlebar",
			wantSub1: "StemBase",
			wantSub2: "AttachHandlebar",
		},
		{
			dir:      "barclampmount_attach_stem",
			wantSub1: "BarClampMount",
			wantSub2: "AttachStem",
		},
		{
			dir:      "stembase_to_barclampmount",
			wantSub1: "cannot convert stemBase",
			wantSub2: "to type assembly.BarClampMount",
		},
		{
			dir:      "unexported_barclampmount_forge",
			wantSub1: "cannot refer to unexported field valid",
			wantSub2: "assembly.BarClampMount",
		},
		{
			dir:      "seataxis_attach_saddle",
			wantSub1: "SeatAxis",
			wantSub2: "AttachSaddle",
		},
		{
			dir:      "seataxis_to_seatpostclampmount",
			wantSub1: "cannot convert sa",
			wantSub2: "to type assembly.SeatpostClampMount",
		},
		{
			dir:      "seatpostclampmount_to_seataxis",
			wantSub1: "cannot convert clamp",
			wantSub2: "to type assembly.SeatAxis",
		},
		{
			dir:      "seatpostclampmount_to_barclampmount",
			wantSub1: "cannot convert clamp",
			wantSub2: "to type assembly.BarClampMount",
		},
		{
			dir:      "unexported_seataxis_forge",
			wantSub1: "cannot refer to unexported field valid",
			wantSub2: "assembly.SeatAxis",
		},
		{
			dir:      "unexported_seatpostclampmount_forge",
			wantSub1: "cannot refer to unexported field valid",
			wantSub2: "assembly.SeatpostClampMount",
		},
	}

	tmpOut := filepath.Join(t.TempDir(), "compile_neg_out")
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			pkgPath := "./" + filepath.ToSlash(filepath.Join("testdata", "compile_negative", tc.dir))
			cmd := exec.Command("go", "build", "-o", tmpOut, pkgPath)
			outBytes, err := cmd.CombinedOutput()
			out := string(outBytes)
			if err == nil {
				t.Fatalf("expected compile error for %s, but go build succeeded", tc.dir)
			}
			if !strings.Contains(out, tc.wantSub1) || !strings.Contains(out, tc.wantSub2) {
				t.Errorf("compile error for %s did not match expected type-check failure (%q, %q):\n%s",
					tc.dir, tc.wantSub1, tc.wantSub2, out)
			}
		})
	}
}
