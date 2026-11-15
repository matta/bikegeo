package geometry

import (
	"testing"

	"bikegeo/pkg/model"
)

func TestCanonicalHB_PicksCanonicalStemWithFewestSpacers(t *testing.T) {
	canonical := model.StemSpec{LengthMm: 60, AngleDeg: -6}
	var points []model.HBPoint
	for _, l := range []float64{50, 60, 70} {
		for _, sp := range []float64{0, 5, 10} {
			points = append(points, model.HBPoint{Stem: model.StemSpec{LengthMm: l, AngleDeg: -6}, SpacerMm: sp})
		}
	}
	best, ok := canonicalHB(points, canonical)
	if !ok || best.Stem.LengthMm != 60 || best.SpacerMm != 0 {
		t.Errorf("canonicalHB = %+v (ok=%v), want the 60mm stem at 0 spacer", best, ok)
	}
	if _, ok := canonicalHB(nil, canonical); ok {
		t.Error("no candidates must report ok=false")
	}
}
