package model_test

import (
	"math"
	"testing"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func TestSaddlePlacementIndependentAxes(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
	for _, sta := range []model.Deg{60, 70, 80} {
		for _, pitch := range []model.Deg{model.LevelRailPitchDeg, -7, 12} {
			s.RailPitchDeg = pitch
			for _, offset := range []float64{-15, 0, 15} {
				// Build the landmarks independently from a known axial extension. This
				// isolates rail direction from the STA-dependent post normal.
				const axial, post = 700.0, 11.0
				a, p := sta.Angle(), pitch.Angle()
				clamp := geom2d.Point2{X: -axial*a.Cos() - post*a.Sin(), Y: axial*a.Sin() - post*a.Cos()}
				center := geom2d.Point2{X: clamp.X - offset*p.Cos(), Y: clamp.Y - offset*p.Sin()}
				brp := geom2d.Point2{X: center.X + 32*p.Cos(), Y: center.Y + 32*p.Sin()}
				sit := geom2d.Point2{X: center.X - 48*p.Cos() - 10*p.Sin(), Y: center.Y - 48*p.Sin() + 10*p.Cos()}
				got, err := s.CalculatePoint(sta, brp.DistanceToOrigin(), post, offset)
				if err != nil {
					t.Fatal(err)
				}
				if got.BrpPoint().DistanceTo(brp) > 1e-9 || got.SitBonePoint().DistanceTo(sit) > 1e-9 || got.SeatpostClamp.DistanceTo(clamp) > 1e-9 || got.Transform.RailCenter.DistanceTo(center) > 1e-9 {
					t.Errorf("STA %v pitch %v offset %v: landmarks disagree: %+v", sta, pitch, offset, got)
				}
				if math.Abs(got.SeatAxisDistanceMm-axial) > 1e-9 {
					t.Error("incorrect solved seatpost extension")
				}
				if math.Abs(got.BrpPoint().DistanceToOrigin()-brp.DistanceToOrigin()) > 1e-9 {
					t.Error("BRP radius changed")
				}
				rail := s.RailOffsetForBrpAtSta(sta, -brp.X, brp.DistanceToOrigin(), post)
				if math.IsNaN(rail) || math.Abs(rail-offset) > 1e-9 {
					t.Errorf("rail inverse = %g, want %g", rail, offset)
				}
				angle := s.StaForBrpAndOffset(-brp.X, brp.DistanceToOrigin(), post, offset)
				if math.IsNaN(float64(angle)) || (angle-sta).Abs() > 1e-9 {
					t.Errorf("STA inverse = %v, want %v", angle, sta)
				}
			}
		}
	}
}

func TestSaddleTransformReferenceLandmarks(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
	brp := geom2d.Point2{X: -219, Y: math.Sqrt(746*746 - 219*219)}
	tr := s.TransformFromBrp(brp)
	if got := tr.Point(-48, 10).Sub(tr.Point(32, 0)); got != (geom2d.Vec2{X: -80, Y: 10}) {
		t.Errorf("reference displacement = %+v", got)
	}
	// TIP is +152mm from the rail-centered origin: W70 is 120mm behind
	// TIP, and SIT is 200mm behind TIP. TIP's absolute height is unmeasured.
	if got := tr.Point(152-120, 0); got.DistanceTo(brp) > 1e-9 {
		t.Errorf("TIP-to-R0 origin translation = %+v", got)
	}
	if got := tr.Point(152-200, 10); got.DistanceTo(s.SitBonePointFromBrp(brp)) > 1e-9 {
		t.Errorf("TIP-to-SIT origin translation = %+v", got)
	}
	if got := s.BrpPointFromSitBones(tr.Point(-48, 10)); got.DistanceTo(brp) > 1e-9 {
		t.Error("landmark inverse failed")
	}
}

func TestSaddlePlacementImpossibleGeometry(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
	for _, c := range []struct {
		sta                model.Deg
		height, post, rail float64
		pitch              model.Deg
	}{
		{73, 10, 100, 0, 0}, // no radial intersection
		{73, 0, 0, 0, 0}, {0, 746, 0, 0, 0}, {90, 746, 0, 0, 0},
		{73, 746, 0, math.NaN(), 0}, {73, 746, 0, 0, model.Deg(math.NaN())},
		{73, math.Inf(1), 0, 0, 0},
		{73, 10, 0, 164, -73}, // rail parallel to seat axis; only negative extension reaches the circle
	} {
		s.RailPitchDeg = c.pitch
		if _, err := s.CalculatePoint(c.sta, c.height, c.post, c.rail); err == nil {
			t.Errorf("accepted impossible placement %+v", c)
		}
	}
	s.RailPitchDeg = model.LevelRailPitchDeg
	if got := s.RailOffsetForBrpAtSta(73, 800, 746, 0); !math.IsNaN(got) {
		t.Errorf("impossible rail inverse = %v", got)
	}
	parallel := s
	parallel.RailPitchDeg = -73
	if got := parallel.RailOffsetForBrpAtSta(73, 219, 746, 0); !math.IsNaN(got) {
		t.Errorf("parallel rail inverse = %v", got)
	}
	if got := s.StaForBrpAndOffset(800, 746, 0, 0); !math.IsNaN(float64(got)) {
		t.Errorf("impossible STA inverse = %v", got)
	}
	if got := s.MapPossibleBRPs(73, 10, 100, 30); len(got) != 0 {
		t.Error("impossible candidates survived")
	}
	if got := s.MapPossibleBRPs(73, 746, 0, 61); len(got) != 0 {
		t.Error("overlong clamp produced a candidate")
	}
}

func TestSaddleCandidatesStayWithinFractionalClampWindow(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
	const clampLength = 59.88 // 60 mm marks leave 0.06 mm on each side.
	candidates := s.MapPossibleBRPs(73, 746, 0, clampLength)
	if len(candidates) != 2 {
		t.Fatalf("a window shorter than 1 mm should offer its two endpoints, got %d candidates", len(candidates))
	}
	for i, want := range []float64{-0.06, 0.06} {
		got := candidates[i].RailOffsetMm
		if math.Abs(got-want) > 1e-9 || !s.ClampFitsAt(clampLength, got) {
			t.Errorf("candidate %d = %+.4f mm, want %+.4f mm inside the modeled window", i, got, want)
		}
	}
}
