package model_test

import (
	"math"
	"testing"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

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
