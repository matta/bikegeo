package model_test

import (
	"math"
	"testing"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

func TestSaddleInverseAxes(t *testing.T) {
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

func TestSaddleInverseImpossibleGeometry(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
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
}
