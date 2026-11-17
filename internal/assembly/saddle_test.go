package assembly_test

import (
	"math"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func mustAssembleSaddle(
	t *testing.T,
	bb geom2d.Point2,
	staDeg model.Deg,
	axialMm, postSetbackMm, railOffsetMm float64,
	spec model.SaddleSpec,
) assembly.SaddlePlacement {
	t.Helper()
	axis, err := assembly.NewSeatAxis(bb, staDeg)
	if err != nil {
		t.Fatalf("NewSeatAxis(%+v, %v): %v", bb, staDeg, err)
	}
	clamp, err := axis.WithSeatpostClamp(axialMm, postSetbackMm)
	if err != nil {
		t.Fatalf("WithSeatpostClamp(%v, %v): %v", axialMm, postSetbackMm, err)
	}
	saddle, err := assembly.NewSaddle(spec)
	if err != nil {
		t.Fatalf("NewSaddle(%+v): %v", spec, err)
	}
	placement, err := clamp.AttachSaddle(saddle, railOffsetMm)
	if err != nil {
		t.Fatalf("AttachSaddle(%v): %v", railOffsetMm, err)
	}
	return placement
}

func TestSaddleIndependent60DegForwardExample(t *testing.T) {
	// At STA 60°, axial distance 100 mm, zero post setback, rail offset +12 mm,
	// level rails, local BRP +32 mm and support point (-48, +10) mm:
	//   clamp       = (-50, 50√3)
	//   rail center = (-62, 50√3)
	//   BRP         = (-30, 50√3)
	//   sit bones   = (-110, 50√3 + 10)
	spec := model.ReferenceIdmatchSaddle()
	p := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 12.0, spec)

	sqrt3 := math.Sqrt(3)
	assertPointNear(t, "SeatpostClamp", p.SeatpostClamp(), geom2d.Point2{X: -50, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "RailCenter", p.RailCenter(), geom2d.Point2{X: -62, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "BRP", p.BRP(), geom2d.Point2{X: -30, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "SitBones", p.SitBones(), geom2d.Point2{X: -110, Y: 50*sqrt3 + 10}, 1e-9)
	if math.Abs(p.AxialDistanceMm()-100.0) > 1e-12 {
		t.Errorf("AxialDistanceMm = %.12f, want 100", p.AxialDistanceMm())
	}
}

func TestSaddleOffsetOwnershipAndSigns(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()
	base := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 12.0, spec)
	withSetback := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 20.0, 12.0, spec)

	// At STA 60°, adding 20 mm post setback translates every landmark by (-10√3, -10) mm.
	wantShift := geom2d.Vec2{X: -10.0 * math.Sqrt(3), Y: -10.0}
	for _, tc := range []struct {
		name string
		a, b geom2d.Point2
	}{
		{"SeatpostClamp", base.SeatpostClamp(), withSetback.SeatpostClamp()},
		{"RailCenter", base.RailCenter(), withSetback.RailCenter()},
		{"BRP", base.BRP(), withSetback.BRP()},
		{"SitBones", base.SitBones(), withSetback.SitBones()},
	} {
		gotShift := tc.b.Sub(tc.a)
		if d := gotShift.Sub(wantShift).Norm(); d > 1e-9 {
			t.Errorf("%s setback shift = (%v, %v), want (%v, %v)", tc.name, gotShift.X, gotShift.Y, wantShift.X, wantShift.Y)
		}
	}

	// Increasing rail offset moves saddle landmarks backward along the rails while leaving clamp fixed.
	moreRailOffset := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 22.0, spec)
	assertPointNear(t, "SeatpostClamp unchanged", moreRailOffset.SeatpostClamp(), base.SeatpostClamp(), 1e-12)
	assertPointNear(t, "RailCenter shifted -10", moreRailOffset.RailCenter(), base.RailCenter().Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)
	assertPointNear(t, "BRP shifted -10", moreRailOffset.BRP(), base.BRP().Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)
	assertPointNear(t, "SitBones shifted -10", moreRailOffset.SitBones(), base.SitBones().Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)
}

func TestSaddleNonzeroPitchAndFrameAttitude(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()
	spec.RailPitchDeg = 10.0 // Positive pitch raises the nose (CCW).

	// Evaluate at two different STAs with the same clamp world position? Better:
	// compare the saddle-local offset vectors (BRP - RailCenter) and (SitBones - RailCenter)
	// across different STAs; they must remain identical and strictly world-pitch-relative.
	p60 := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 5.0, spec)
	p75 := mustAssembleSaddle(t, geom2d.Origin(), 75.0, 100.0, 0.0, 5.0, spec)

	// Nose (BRP is +32 mm along rails) must be higher than RailCenter when pitch > 0.
	if p60.BRP().Y <= p60.RailCenter().Y {
		t.Errorf("positive rail pitch should raise BRP above rail center: BRP.Y=%.4f, RailCenter.Y=%.4f", p60.BRP().Y, p60.RailCenter().Y)
	}
	brpVec60 := p60.BRP().Sub(p60.RailCenter())
	brpVec75 := p75.BRP().Sub(p75.RailCenter())
	if brpVec60.Sub(brpVec75).Norm() > 1e-12 {
		t.Errorf("rail pitch must be world-relative, not STA-dependent: 60°=%+v, 75°=%+v", brpVec60, brpVec75)
	}
	sitVec60 := p60.SitBones().Sub(p60.RailCenter())
	sitVec75 := p75.SitBones().Sub(p75.RailCenter())
	if sitVec60.Sub(sitVec75).Norm() > 1e-12 {
		t.Errorf("sit-bone offset must be world-relative, not STA-dependent: 60°=%+v, 75°=%+v", sitVec60, sitVec75)
	}
}

func TestSaddleRigidTranslationProperty(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()
	spec.RailPitchDeg = -4.0
	shift := geom2d.Vec2{X: 120.0, Y: -35.0}
	base := mustAssembleSaddle(t, geom2d.Origin(), 73.0, 700.0, 15.0, -8.0, spec)
	shifted := mustAssembleSaddle(t, geom2d.Origin().Add(shift), 73.0, 700.0, 15.0, -8.0, spec)

	assertPointNear(t, "SeatpostClamp", shifted.SeatpostClamp(), base.SeatpostClamp().Add(shift), 1e-9)
	assertPointNear(t, "RailCenter", shifted.RailCenter(), base.RailCenter().Add(shift), 1e-9)
	assertPointNear(t, "BRP", shifted.BRP(), base.BRP().Add(shift), 1e-9)
	assertPointNear(t, "SitBones", shifted.SitBones(), base.SitBones().Add(shift), 1e-9)
}

func TestSaddleAssemblyBoundaryTable(t *testing.T) {
	// Zero axial distance and zero post setback are legal.
	spec := model.ReferenceIdmatchSaddle()
	pZero := mustAssembleSaddle(t, geom2d.Origin(), 73.0, 0.0, 0.0, 0.0, spec)
	assertPointNear(t, "zero clamp", pZero.SeatpostClamp(), geom2d.Origin(), 1e-12)

	// SeatAxis invalid inputs
	for _, tc := range []struct {
		name string
		bb   geom2d.Point2
		sta  model.Deg
	}{
		{"NaN bb.X", geom2d.Point2{X: math.NaN()}, 73},
		{"Inf bb.Y", geom2d.Point2{Y: math.Inf(1)}, 73},
		{"STA 0", geom2d.Origin(), 0},
		{"STA negative", geom2d.Origin(), -10},
		{"STA 90", geom2d.Origin(), 90},
		{"STA > 90", geom2d.Origin(), 95},
		{"STA NaN", geom2d.Origin(), model.Deg(math.NaN())},
		{"STA Inf", geom2d.Origin(), model.Deg(math.Inf(1))},
	} {
		if _, err := assembly.NewSeatAxis(tc.bb, tc.sta); err == nil {
			t.Errorf("NewSeatAxis(%s): expected error, got nil", tc.name)
		}
	}

	validAxis, err := assembly.NewSeatAxis(geom2d.Origin(), 73)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		axial, setback float64
	}{
		{"negative axial", -1, 0},
		{"NaN axial", math.NaN(), 0},
		{"Inf axial", math.Inf(1), 0},
		{"negative setback", 100, -1},
		{"NaN setback", 100, math.NaN()},
		{"Inf setback", 100, math.Inf(1)},
	} {
		if _, err := validAxis.WithSeatpostClamp(tc.axial, tc.setback); err == nil {
			t.Errorf("WithSeatpostClamp(%s): expected error, got nil", tc.name)
		}
	}

	// Overflow during clamp translation
	hugeAxis, _ := assembly.NewSeatAxis(geom2d.Point2{X: -math.MaxFloat64, Y: 0}, 45)
	if _, err := hugeAxis.WithSeatpostClamp(math.MaxFloat64, 0); err == nil {
		t.Errorf("WithSeatpostClamp overflow: expected error, got nil")
	}

	// Saddle invalid inputs
	for _, tc := range []struct {
		name string
		mut  func(*model.SaddleSpec)
	}{
		{"NaN RailCenterToBrpMm", func(s *model.SaddleSpec) { s.RailCenterToBrpMm = math.NaN() }},
		{"Inf RailCenterToSitBonesMm", func(s *model.SaddleSpec) { s.RailCenterToSitBonesMm = math.Inf(1) }},
		{"NaN SitBonesElevationMm", func(s *model.SaddleSpec) { s.SitBonesElevationMm = math.NaN() }},
		{"Inf RailPitchDeg", func(s *model.SaddleSpec) { s.RailPitchDeg = model.Deg(math.Inf(-1)) }},
	} {
		s := spec
		tc.mut(&s)
		if _, err := assembly.NewSaddle(s); err == nil {
			t.Errorf("NewSaddle(%s): expected error, got nil", tc.name)
		}
	}

	// AttachSaddle invalid railOffset and overflow
	validClamp, _ := validAxis.WithSeatpostClamp(700, 0)
	validSaddle, _ := assembly.NewSaddle(spec)
	if _, err := validClamp.AttachSaddle(validSaddle, math.NaN()); err == nil {
		t.Errorf("AttachSaddle(NaN): expected error, got nil")
	}
	if _, err := validClamp.AttachSaddle(validSaddle, math.Inf(1)); err == nil {
		t.Errorf("AttachSaddle(Inf): expected error, got nil")
	}
	hugeClamp, _ := validAxis.WithSeatpostClamp(math.MaxFloat64, 0)
	if _, err := hugeClamp.AttachSaddle(validSaddle, math.MaxFloat64); err == nil {
		t.Errorf("AttachSaddle overflow: expected error, got nil")
	}
}

func TestSaddleZeroValueAndIgnoredErrorPanics(t *testing.T) {
	validSaddle, _ := assembly.NewSaddle(model.ReferenceIdmatchSaddle())

	assertPanics(t, "zero SeatAxis AxisDirection", func() {
		var sa assembly.SeatAxis
		_ = sa.AxisDirection()
	})
	assertPanics(t, "zero SeatAxis WithSeatpostClamp", func() {
		var sa assembly.SeatAxis
		_, _ = sa.WithSeatpostClamp(100, 0)
	})
	assertPanics(t, "ignored error SeatAxis", func() {
		sa, _ := assembly.NewSeatAxis(geom2d.Origin(), 0)
		_, _ = sa.WithSeatpostClamp(100, 0)
	})
	assertPanics(t, "zero SeatpostClampMount AttachSaddle", func() {
		var m assembly.SeatpostClampMount
		_, _ = m.AttachSaddle(validSaddle, 0)
	})
	assertPanics(t, "ignored error SeatpostClampMount", func() {
		sa, _ := assembly.NewSeatAxis(geom2d.Origin(), 73)
		m, _ := sa.WithSeatpostClamp(-1, 0)
		_, _ = m.AttachSaddle(validSaddle, 0)
	})
	assertPanics(t, "zero Saddle AttachSaddle", func() {
		sa, _ := assembly.NewSeatAxis(geom2d.Origin(), 73)
		m, _ := sa.WithSeatpostClamp(700, 0)
		var s assembly.Saddle
		_, _ = m.AttachSaddle(s, 0)
	})
	assertPanics(t, "zero SaddlePlacement BRP", func() {
		var p assembly.SaddlePlacement
		_ = p.BRP()
	})
	assertPanics(t, "zero SaddlePlacement SeatpostClamp", func() {
		var p assembly.SaddlePlacement
		_ = p.SeatpostClamp()
	})
	assertPanics(t, "zero SaddlePlacement RailCenter", func() {
		var p assembly.SaddlePlacement
		_ = p.RailCenter()
	})
	assertPanics(t, "zero SaddlePlacement SitBones", func() {
		var p assembly.SaddlePlacement
		_ = p.SitBones()
	})
	assertPanics(t, "zero SaddlePlacement AxialDistanceMm", func() {
		var p assembly.SaddlePlacement
		_ = p.AxialDistanceMm()
	})
	assertPanics(t, "ignored error SaddlePlacement", func() {
		sa, _ := assembly.NewSeatAxis(geom2d.Origin(), 73)
		m, _ := sa.WithSeatpostClamp(700, 0)
		p, _ := m.AttachSaddle(validSaddle, math.NaN())
		_ = p.BRP()
	})
}
