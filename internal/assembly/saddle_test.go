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
	bbToClampAlongSeatAxisMm, postSetbackMm, railOffsetMm float64,
	spec model.SaddleSpec,
) assembly.SaddlePlacement {
	t.Helper()
	placement, err := assembly.PlaceSaddle(bb, staDeg, assembly.SaddleSetup{
		Saddle:                   spec,
		BBToClampAlongSeatAxisMm: bbToClampAlongSeatAxisMm,
		PostSetbackMm:            postSetbackMm,
		RailOffsetMm:             railOffsetMm,
	})
	if err != nil {
		t.Fatalf("PlaceSaddle(%+v, %v): %v", bb, staDeg, err)
	}
	return placement
}

func TestSaddleIndependent60DegForwardExample(t *testing.T) {
	// At STA 60°, BB-to-clamp along seat axis 100 mm, zero post setback, rail offset +12 mm,
	// level rails, local BRP +32 mm and support point (-48, +10) mm:
	//   clamp       = (-50, 50√3)
	//   rail center = (-62, 50√3)
	//   BRP         = (-30, 50√3)
	//   sit bones   = (-110, 50√3 + 10)
	spec := model.ReferenceIdmatchSaddle()
	p := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 12.0, spec)

	sqrt3 := math.Sqrt(3)
	assertPointNear(t, "SeatpostClamp", p.SeatpostClamp, geom2d.Point2{X: -50, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "RailCenter", p.RailCenter, geom2d.Point2{X: -62, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "BRP", p.BRP, geom2d.Point2{X: -30, Y: 50 * sqrt3}, 1e-9)
	assertPointNear(t, "SitBones", p.SitBones, geom2d.Point2{X: -110, Y: 50*sqrt3 + 10}, 1e-9)
	if math.Abs(p.BBToClampAlongSeatAxisMm-100.0) > 1e-12 {
		t.Errorf("BBToClampAlongSeatAxisMm = %.12f, want 100", p.BBToClampAlongSeatAxisMm)
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
		{"SeatpostClamp", base.SeatpostClamp, withSetback.SeatpostClamp},
		{"RailCenter", base.RailCenter, withSetback.RailCenter},
		{"BRP", base.BRP, withSetback.BRP},
		{"SitBones", base.SitBones, withSetback.SitBones},
	} {
		gotShift := tc.b.Sub(tc.a)
		if d := gotShift.Sub(wantShift).Norm(); d > 1e-9 {
			t.Errorf("%s setback shift = (%v, %v), want (%v, %v)", tc.name, gotShift.X, gotShift.Y, wantShift.X, wantShift.Y)
		}
	}

	// Increasing rail offset moves saddle landmarks backward along the rails while leaving clamp fixed.
	moreRailOffset := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 22.0, spec)
	assertPointNear(t, "SeatpostClamp unchanged", moreRailOffset.SeatpostClamp, base.SeatpostClamp, 1e-12)
	assertPointNear(t, "RailCenter shifted -10", moreRailOffset.RailCenter, base.RailCenter.Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)
	assertPointNear(t, "BRP shifted -10", moreRailOffset.BRP, base.BRP.Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)
	assertPointNear(t, "SitBones shifted -10", moreRailOffset.SitBones, base.SitBones.Add(geom2d.Vec2{X: -10, Y: 0}), 1e-12)

	// Negative rail offset moves saddle landmarks forward along the rails while leaving clamp fixed.
	negRailOffset := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, -8.0, spec)
	assertPointNear(t, "SeatpostClamp unchanged (neg)", negRailOffset.SeatpostClamp, base.SeatpostClamp, 1e-12)
	assertPointNear(t, "RailCenter shifted +20", negRailOffset.RailCenter, base.RailCenter.Add(geom2d.Vec2{X: 20, Y: 0}), 1e-12)
	assertPointNear(t, "BRP shifted +20", negRailOffset.BRP, base.BRP.Add(geom2d.Vec2{X: 20, Y: 0}), 1e-12)
}

func TestSaddleNonzeroPitchAndFrameAttitude(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()
	spec.RailPitchDeg = 10.0 // Positive pitch raises the nose (CCW).

	// Compare saddle-local offset vectors (BRP - RailCenter) and (SitBones - RailCenter)
	// across different STAs; they must remain identical and strictly world-pitch-relative.
	p60 := mustAssembleSaddle(t, geom2d.Origin(), 60.0, 100.0, 0.0, 5.0, spec)
	p75 := mustAssembleSaddle(t, geom2d.Origin(), 75.0, 100.0, 0.0, 5.0, spec)

	// Nose (BRP is +32 mm along rails) must be higher than RailCenter when pitch > 0.
	if p60.BRP.Y <= p60.RailCenter.Y {
		t.Errorf("positive rail pitch should raise BRP above rail center: BRP.Y=%.4f, RailCenter.Y=%.4f", p60.BRP.Y, p60.RailCenter.Y)
	}
	brpVec60 := p60.BRP.Sub(p60.RailCenter)
	brpVec75 := p75.BRP.Sub(p75.RailCenter)
	if brpVec60.Sub(brpVec75).Norm() > 1e-12 {
		t.Errorf("rail pitch must be world-relative, not STA-dependent: 60°=%+v, 75°=%+v", brpVec60, brpVec75)
	}
	sitVec60 := p60.SitBones.Sub(p60.RailCenter)
	sitVec75 := p75.SitBones.Sub(p75.RailCenter)
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

	assertPointNear(t, "SeatpostClamp", shifted.SeatpostClamp, base.SeatpostClamp.Add(shift), 1e-9)
	assertPointNear(t, "RailCenter", shifted.RailCenter, base.RailCenter.Add(shift), 1e-9)
	assertPointNear(t, "BRP", shifted.BRP, base.BRP.Add(shift), 1e-9)
	assertPointNear(t, "SitBones", shifted.SitBones, base.SitBones.Add(shift), 1e-9)
}

func TestSaddleAssemblyBoundaryTable(t *testing.T) {
	// Zero BB-to-clamp distance along seat axis and zero post setback are legal.
	spec := model.ReferenceIdmatchSaddle()
	pZero := mustAssembleSaddle(t, geom2d.Origin(), 73.0, 0.0, 0.0, 0.0, spec)
	assertPointNear(t, "zero clamp", pZero.SeatpostClamp, geom2d.Origin(), 1e-12)

	baseSetup := assembly.SaddleSetup{
		Saddle:                   spec,
		BBToClampAlongSeatAxisMm: 700,
		PostSetbackMm:            15,
		RailOffsetMm:             5,
	}

	badCases := []struct {
		name string
		bb   geom2d.Point2
		sta  model.Deg
		mut  func(*assembly.SaddleSetup)
	}{
		{"NaN bb.X", geom2d.Point2{X: math.NaN()}, 73, nil},
		{"Inf bb.Y", geom2d.Point2{Y: math.Inf(1)}, 73, nil},
		{"STA 0", geom2d.Origin(), 0, nil},
		{"STA negative", geom2d.Origin(), -10, nil},
		{"STA 90", geom2d.Origin(), 90, nil},
		{"STA > 90", geom2d.Origin(), 95, nil},
		{"STA NaN", geom2d.Origin(), model.Deg(math.NaN()), nil},
		{"STA Inf", geom2d.Origin(), model.Deg(math.Inf(1)), nil},
		{"negative BBToClampAlongSeatAxisMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.BBToClampAlongSeatAxisMm = -1 }},
		{"NaN BBToClampAlongSeatAxisMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.BBToClampAlongSeatAxisMm = math.NaN() }},
		{"Inf BBToClampAlongSeatAxisMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.BBToClampAlongSeatAxisMm = math.Inf(1) }},
		{"negative setback", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.PostSetbackMm = -1 }},
		{"NaN setback", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.PostSetbackMm = math.NaN() }},
		{"Inf setback", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.PostSetbackMm = math.Inf(1) }},
		{"NaN RailCenterToBrpMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.Saddle.RailCenterToBrpMm = math.NaN() }},
		{"Inf RailCenterToSitBonesMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.Saddle.RailCenterToSitBonesMm = math.Inf(1) }},
		{"NaN SitBonesElevationMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.Saddle.SitBonesElevationMm = math.NaN() }},
		{"Inf RailPitchDeg", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.Saddle.RailPitchDeg = model.Deg(math.Inf(-1)) }},
		{"NaN RailOffsetMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.RailOffsetMm = math.NaN() }},
		{"Inf RailOffsetMm", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) { s.RailOffsetMm = math.Inf(1) }},
		{"clamp translation overflow", geom2d.Point2{X: -math.MaxFloat64, Y: 0}, 45, func(s *assembly.SaddleSetup) { s.BBToClampAlongSeatAxisMm = math.MaxFloat64; s.PostSetbackMm = 0 }},
		{"rail placement overflow", geom2d.Origin(), 73, func(s *assembly.SaddleSetup) {
			s.BBToClampAlongSeatAxisMm = math.MaxFloat64
			s.PostSetbackMm = 0
			s.RailOffsetMm = math.MaxFloat64
		}},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			s := baseSetup
			if tc.mut != nil {
				tc.mut(&s)
			}
			if _, err := assembly.PlaceSaddle(tc.bb, tc.sta, s); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

func TestSaddlePlacementCopiesSyntheticSpec(t *testing.T) {
	// Synthetic geometric fixture without catalog ID or rail span metadata.
	spec := model.SaddleSpec{
		RailCenterToBrpMm:      32,
		RailCenterToSitBonesMm: -48,
		SitBonesElevationMm:    12,
		RailPitchDeg:           5,
	}
	setup := assembly.SaddleSetup{
		Saddle:                   spec,
		BBToClampAlongSeatAxisMm: 700,
		PostSetbackMm:            15,
		RailOffsetMm:             -8,
	}
	p, err := assembly.PlaceSaddle(geom2d.Origin(), 73, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle: %v", err)
	}

	// Mutating the caller's spec after placement must not affect the stored copy.
	setup.Saddle.RailPitchDeg = -25
	setup.Saddle.RailCenterToBrpMm = 999
	setup.Saddle.ID = "mutated"

	if p.Saddle.RailPitchDeg != 5 || p.Saddle.RailCenterToBrpMm != 32 || p.Saddle.ID != "" {
		t.Errorf("placement Saddle = %+v, want unmutated synthetic spec", p.Saddle)
	}
	if p.PostSetbackMm != 15 || p.RailOffsetMm != -8 {
		t.Errorf("placement adjustments = (post=%.1f, rail=%.1f), want (15, -8)", p.PostSetbackMm, p.RailOffsetMm)
	}
}

func TestSaddleQueriesAgreeWithForwardPlacement(t *testing.T) {
	spec := model.ReferenceIdmatchSaddle()
	spec.RailPitchDeg = 30
	v, err := assembly.SaddleRailOffsetDirection(spec.RailPitchDeg)
	if err != nil {
		t.Fatalf("SaddleRailOffsetDirection: %v", err)
	}
	// Positive pitch (+30° nose up) means rearward along rails points at 210°: (-√3/2, -1/2).
	wantRailDir := geom2d.Vec2{X: -math.Sqrt(3) / 2, Y: -0.5}
	if math.Abs(v.X-wantRailDir.X) > 1e-12 || math.Abs(v.Y-wantRailDir.Y) > 1e-12 {
		t.Errorf("SaddleRailOffsetDirection(+30°) = (%.12f, %.12f), want (%.12f, %.12f)", v.X, v.Y, wantRailDir.X, wantRailDir.Y)
	}

	// Queries must agree with forward movements across STAs, pitches, and offsets.
	for _, sta := range []model.Deg{60, 73, 82} {
		u, err := assembly.SeatAxisDirection(sta)
		if err != nil {
			t.Fatalf("SeatAxisDirection(%v): %v", sta, err)
		}
		for _, pitch := range []model.Deg{-7, 0, 12} {
			s := spec
			s.RailPitchDeg = pitch
			railDir, err := assembly.SaddleRailOffsetDirection(pitch)
			if err != nil {
				t.Fatalf("SaddleRailOffsetDirection(%v): %v", pitch, err)
			}
			for _, offset := range []float64{-14, 0, 11} {
				p0 := mustAssembleSaddle(t, geom2d.Origin(), sta, 500, 15, offset, s)
				pSeatAxis := mustAssembleSaddle(t, geom2d.Origin(), sta, 500+25, 15, offset, s)
				pRail := mustAssembleSaddle(t, geom2d.Origin(), sta, 500, 15, offset+10, s)

				gotSeatAxisStep := pSeatAxis.SeatpostClamp.Sub(p0.SeatpostClamp)
				if d := gotSeatAxisStep.Sub(u.Scale(25)).Norm(); d > 1e-11 {
					t.Errorf("STA=%v: BB-to-clamp along seat axis step mismatch %.3e", sta, d)
				}
				gotRailStep := pRail.BRP.Sub(p0.BRP)
				if d := gotRailStep.Sub(railDir.Scale(10)).Norm(); d > 1e-11 {
					t.Errorf("pitch=%v: rail step mismatch %.3e", pitch, d)
				}
				disp, err := assembly.SaddleClampToBRP(s, offset)
				if err != nil {
					t.Fatalf("SaddleClampToBRP: %v", err)
				}
				if d := p0.BRP.Sub(p0.SeatpostClamp).Sub(disp).Norm(); d > 1e-11 {
					t.Errorf("pitch=%v offset=%v: ClampToBRP mismatch %.3e", pitch, offset, d)
				}
			}
		}
	}

	// Query boundary failures.
	for _, badSTA := range []model.Deg{0, -10, 90, 95, model.Deg(math.NaN()), model.Deg(math.Inf(1))} {
		if _, err := assembly.SeatAxisDirection(badSTA); err == nil {
			t.Errorf("SeatAxisDirection(%v): expected error", badSTA)
		}
	}
	for _, badPitch := range []model.Deg{model.Deg(math.NaN()), model.Deg(math.Inf(-1))} {
		if _, err := assembly.SaddleRailOffsetDirection(badPitch); err == nil {
			t.Errorf("SaddleRailOffsetDirection(%v): expected error", badPitch)
		}
	}
	if _, err := assembly.SaddleClampToBRP(spec, math.NaN()); err == nil {
		t.Error("SaddleClampToBRP(NaN): expected error")
	}
	badSpec := spec
	badSpec.RailCenterToBrpMm = math.NaN()
	if _, err := assembly.SaddleClampToBRP(badSpec, 0); err == nil {
		t.Error("SaddleClampToBRP(NaN BRP): expected error")
	}
}
