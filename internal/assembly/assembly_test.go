package assembly_test

import (
	"math"
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
	placement, err := assembly.PlaceStem(pos, htaDeg, assembly.StemSetup{
		UpperHeadsetCoverMm: coverMm,
		SpacersMm:           spacersMm,
		ClampHeightMm:       clampHeightMm,
		LengthMm:            stemLengthMm,
		AngleDeg:            stemAngleDeg,
	})
	if err != nil {
		t.Fatalf("PlaceStem(%+v, %v): %v", pos, htaDeg, err)
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

	assertPointNear(t, "SteererClamp", p.SteererClamp, wantSteerer, 1e-12)
	assertPointNear(t, "BarClamp", p.BarClamp, wantBar, 1e-12)
}

func TestNonzeroStemAngleAndIndependentFaultSensitivity(t *testing.T) {
	// At HTA = 70°, perpendicular forward is 90° - 70° = 20° above +X.
	// Case A: stem angle = -20° cancels the 20° slope so the stem points strictly +X (horizontal).
	//   dSteerer = 11 (cover) + 15 (spacers) + 20 (40/2 half-clamp) = 46 mm
	//   SteererClamp = (-46*cos(70°), 46*sin(70°))
	//   BarClamp     = SteererClamp + (80, 0)
	pNeg20 := mustAssembleStem(t, geom2d.Origin(), 70.0, 11.0, 15.0, 40.0, 80.0, -20.0)
	sin70 := math.Sin(70.0 * math.Pi / 180.0)
	cos70 := math.Cos(70.0 * math.Pi / 180.0)
	wantSC := geom2d.Point2{X: -46.0 * cos70, Y: 46.0 * sin70}
	wantBCNeg20 := geom2d.Point2{X: wantSC.X + 80.0, Y: wantSC.Y}
	assertPointNear(t, "SteererClamp(70°,-20°)", pNeg20.SteererClamp, wantSC, 1e-11)
	assertPointNear(t, "BarClamp(70°,-20°)", pNeg20.BarClamp, wantBCNeg20, 1e-11)

	// Reversing stem angle (+20° instead of -20°) aims at 40° above +X instead of 0°:
	// detects sign-inversion faults that zero-angle tests cannot see.
	pPos20 := mustAssembleStem(t, geom2d.Origin(), 70.0, 11.0, 15.0, 40.0, 80.0, 20.0)
	cos40 := math.Cos(40.0 * math.Pi / 180.0)
	sin40 := math.Sin(40.0 * math.Pi / 180.0)
	wantBCPos20 := geom2d.Point2{X: wantSC.X + 80.0*cos40, Y: wantSC.Y + 80.0*sin40}
	assertPointNear(t, "BarClamp(70°,+20°)", pPos20.BarClamp, wantBCPos20, 1e-11)
	if pPos20.BarClamp.DistanceTo(pNeg20.BarClamp) < 50.0 {
		t.Fatalf("expected reversed stem angle to diverge significantly")
	}

	// Verify sensitivity to cover and half-clamp height independently:
	pNoCover := mustAssembleStem(t, geom2d.Origin(), 70.0, 0.0, 15.0, 40.0, 80.0, -20.0)
	if d := pNeg20.SteererClamp.DistanceTo(pNoCover.SteererClamp); math.Abs(d-11.0) > 1e-11 {
		t.Errorf("cover contribution = %.12f, want 11.0", d)
	}
	pSmallClamp := mustAssembleStem(t, geom2d.Origin(), 70.0, 11.0, 15.0, 20.0, 80.0, -20.0)
	if d := pNeg20.SteererClamp.DistanceTo(pSmallClamp.SteererClamp); math.Abs(d-10.0) > 1e-11 {
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
			pivot:     geom2d.Origin(),
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

			assertPointNear(t, "SteererClamp", p1.SteererClamp, transform(p0.SteererClamp), 1e-11)
			assertPointNear(t, "BarClamp", p1.BarClamp, transform(p0.BarClamp), 1e-11)
		})
	}
}

func TestDimensionPropertiesAndSpacerTranslation(t *testing.T) {
	pos := geom2d.Point2{X: 430, Y: 625}
	hta := model.Deg(69.5)
	cover := 11.0
	spacers := 10.0
	clampH := 40.0

	placeA := mustAssembleStem(t, pos, hta, cover, spacers, clampH, 70.0, -6.0)
	placeB := mustAssembleStem(t, pos, hta, cover, spacers, clampH, 110.0, 17.0)

	// Both stems share the same steerer clamp center (same clamp height and upper stack).
	assertPointNear(t, "shared SteererClamp", placeA.SteererClamp, placeB.SteererClamp, 1e-12)

	// Clamp separation equals stem length.
	if d := placeA.SteererClamp.DistanceTo(placeA.BarClamp); math.Abs(d-70.0) > 1e-11 {
		t.Errorf("stemA separation = %.12f, want 70.0", d)
	}
	if d := placeB.SteererClamp.DistanceTo(placeB.BarClamp); math.Abs(d-110.0) > 1e-11 {
		t.Errorf("stemB separation = %.12f, want 110.0", d)
	}

	// Distance from head-tube top to steerer clamp equals cover + spacers + clampH/2.
	if d := pos.DistanceTo(placeA.SteererClamp); math.Abs(d-(cover+spacers+clampH/2.0)) > 1e-11 {
		t.Errorf("steerer advance = %.12f, want %.12f", d, cover+spacers+clampH/2.0)
	}

	// Adding delta spacers translates both landmarks along the steerer axis by delta.
	delta := 15.0
	placeA2 := mustAssembleStem(t, pos, hta, cover, spacers+delta, clampH, 70.0, -6.0)

	steerStep := geom2d.FromPolar(delta, (180.0 - hta).Angle())
	assertPointNear(t, "SteererClamp+delta", placeA2.SteererClamp, placeA.SteererClamp.Add(steerStep), 1e-11)
	assertPointNear(t, "BarClamp+delta", placeA2.BarClamp, placeA.BarClamp.Add(steerStep), 1e-11)
}

func TestBoundaryValidation(t *testing.T) {
	basePos := geom2d.Point2{X: 430, Y: 625}
	baseHTA := model.Deg(70.0)
	baseSetup := assembly.StemSetup{
		UpperHeadsetCoverMm: 11.0,
		SpacersMm:           15.0,
		ClampHeightMm:       40.0,
		LengthMm:            80.0,
		AngleDeg:            -6.0,
	}
	// Baseline succeeds with finite landmarks.
	basePlace, err := assembly.PlaceStem(basePos, baseHTA, baseSetup)
	if err != nil {
		t.Fatalf("baseline PlaceStem failed: %v", err)
	}
	if math.IsNaN(basePlace.SteererClamp.X) || math.IsInf(basePlace.SteererClamp.X, 0) ||
		math.IsNaN(basePlace.BarClamp.Y) || math.IsInf(basePlace.BarClamp.Y, 0) {
		t.Fatalf("baseline PlaceStem returned non-finite landmarks: %+v", basePlace)
	}

	// Zero cover and zero spacers, and boundary HTA values (0°, 90°], remain legal.
	for _, hta := range []model.Deg{0.1, 60.0, 90.0} {
		zeroStack := baseSetup
		zeroStack.UpperHeadsetCoverMm = 0
		zeroStack.SpacersMm = 0
		if _, err := assembly.PlaceStem(geom2d.Origin(), hta, zeroStack); err != nil {
			t.Errorf("HTA %v with zero cover/spacers should be valid, got %v", hta, err)
		}
	}

	badCases := []struct {
		name string
		pos  geom2d.Point2
		hta  model.Deg
		mut  func(*assembly.StemSetup)
	}{
		{"NaN X", geom2d.Point2{X: math.NaN(), Y: 600}, baseHTA, nil},
		{"+Inf X", geom2d.Point2{X: math.Inf(1), Y: 600}, baseHTA, nil},
		{"-Inf Y", geom2d.Point2{X: 400, Y: math.Inf(-1)}, baseHTA, nil},
		{"HTA 0", basePos, 0, nil},
		{"HTA negative", basePos, -10, nil},
		{"HTA > 90", basePos, 90.1, nil},
		{"HTA 180", basePos, 180, nil},
		{"HTA NaN", basePos, model.Deg(math.NaN()), nil},
		{"HTA +Inf", basePos, model.Deg(math.Inf(1)), nil},
		{"negative cover", basePos, baseHTA, func(s *assembly.StemSetup) { s.UpperHeadsetCoverMm = -0.1 }},
		{"negative spacers", basePos, baseHTA, func(s *assembly.StemSetup) { s.SpacersMm = -5 }},
		{"NaN cover", basePos, baseHTA, func(s *assembly.StemSetup) { s.UpperHeadsetCoverMm = math.NaN() }},
		{"NaN spacers", basePos, baseHTA, func(s *assembly.StemSetup) { s.SpacersMm = math.NaN() }},
		{"+Inf cover", basePos, baseHTA, func(s *assembly.StemSetup) { s.UpperHeadsetCoverMm = math.Inf(1) }},
		{"-Inf spacers", basePos, baseHTA, func(s *assembly.StemSetup) { s.SpacersMm = math.Inf(-1) }},
		{"zero clamp height", basePos, baseHTA, func(s *assembly.StemSetup) { s.ClampHeightMm = 0 }},
		{"negative clamp height", basePos, baseHTA, func(s *assembly.StemSetup) { s.ClampHeightMm = -10 }},
		{"NaN clamp height", basePos, baseHTA, func(s *assembly.StemSetup) { s.ClampHeightMm = math.NaN() }},
		{"+Inf clamp height", basePos, baseHTA, func(s *assembly.StemSetup) { s.ClampHeightMm = math.Inf(1) }},
		{"zero length", basePos, baseHTA, func(s *assembly.StemSetup) { s.LengthMm = 0 }},
		{"negative length", basePos, baseHTA, func(s *assembly.StemSetup) { s.LengthMm = -50 }},
		{"NaN length", basePos, baseHTA, func(s *assembly.StemSetup) { s.LengthMm = math.NaN() }},
		{"+Inf length", basePos, baseHTA, func(s *assembly.StemSetup) { s.LengthMm = math.Inf(1) }},
		{"NaN angle", basePos, baseHTA, func(s *assembly.StemSetup) { s.AngleDeg = model.Deg(math.NaN()) }},
		{"-Inf angle", basePos, baseHTA, func(s *assembly.StemSetup) { s.AngleDeg = model.Deg(math.Inf(-1)) }},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			s := baseSetup
			if tc.mut != nil {
				tc.mut(&s)
			}
			if _, err := assembly.PlaceStem(tc.pos, tc.hta, s); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}
