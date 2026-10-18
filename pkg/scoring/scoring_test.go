package scoring

import (
	"math"
	"testing"

	"bikegeo/pkg/model"
)

func TestCurveEval(t *testing.T) {
	c := Curve{
		{X: 10, Y: 0},
		{X: 20, Y: 100},
		{X: 30, Y: 50},
	}

	// Below range: clamped to c[0].Y
	if got := c.Eval(5); got != 0 {
		t.Errorf("Eval(5) expected 0, got %f", got)
	}
	// Exact lower bound
	if got := c.Eval(10); got != 0 {
		t.Errorf("Eval(10) expected 0, got %f", got)
	}
	// Midpoint of first segment
	if got := c.Eval(15); got != 50 {
		t.Errorf("Eval(15) expected 50, got %f", got)
	}
	// Peak
	if got := c.Eval(20); got != 100 {
		t.Errorf("Eval(20) expected 100, got %f", got)
	}
	// Midpoint of second segment
	if got := c.Eval(25); got != 75 {
		t.Errorf("Eval(25) expected 75, got %f", got)
	}
	// Exact upper bound
	if got := c.Eval(30); got != 50 {
		t.Errorf("Eval(30) expected 50, got %f", got)
	}
	// Above range: clamped to c[last].Y
	if got := c.Eval(40); got != 50 {
		t.Errorf("Eval(40) expected 50, got %f", got)
	}
}

func TestDefaultCriteriaCurves(t *testing.T) {
	critMap := make(map[string]Criterion)
	for _, c := range DefaultCriteria {
		critMap[c.ID] = c
	}

	// 1. AETT
	aett := critMap["aett"].Curve
	if aett.Eval(560) != -200 {
		t.Errorf("AETT(560) expected -200, got %f", aett.Eval(560))
	}
	if aett.Eval(584) != 0 {
		t.Errorf("AETT(584) expected 0, got %f", aett.Eval(584))
	}
	if aett.Eval(634) != 100 {
		t.Errorf("AETT(634) expected 100, got %f", aett.Eval(634))
	}
	if aett.Eval(644) != 0 {
		t.Errorf("AETT(644) expected 0, got %f", aett.Eval(644))
	}
	if aett.Eval(654) != -100 {
		t.Errorf("AETT(654) expected -100, got %f", aett.Eval(654))
	}
	if aett.Eval(664) != -200 {
		t.Errorf("AETT(664) expected -200, got %f", aett.Eval(664))
	}
	if aett.Eval(680) != -200 {
		t.Errorf("AETT(680) expected -200, got %f", aett.Eval(680))
	}

	// 2. Setback
	sb := critMap["setback"].Curve
	if sb.Eval(180) != -100 {
		t.Errorf("Setback(180) expected -100, got %f", sb.Eval(180))
	}
	if sb.Eval(205) != 0 {
		t.Errorf("Setback(205) expected 0, got %f", sb.Eval(205))
	}
	if sb.Eval(220) != 100 {
		t.Errorf("Setback(220) expected 100, got %f", sb.Eval(220))
	}
	if sb.Eval(235) != 0 {
		t.Errorf("Setback(235) expected 0, got %f", sb.Eval(235))
	}
	if sb.Eval(250) != -100 {
		t.Errorf("Setback(250) expected -100, got %f", sb.Eval(250))
	}

	// 3. Spare Vert
	sv := critMap["sparevert"].Curve
	if sv.Eval(60) != -100 {
		t.Errorf("SpareVert(60) expected -100, got %f", sv.Eval(60))
	}
	if sv.Eval(85) != 0 {
		t.Errorf("SpareVert(85) expected 0, got %f", sv.Eval(85))
	}
	if sv.Eval(PreferredSpareVertMm) != 100 {
		t.Errorf("SpareVert(%.0f) expected 100, got %f", PreferredSpareVertMm, sv.Eval(PreferredSpareVertMm))
	}
	if sv.Eval(135) != 0 {
		t.Errorf("SpareVert(135) expected 0, got %f", sv.Eval(135))
	}
	if sv.Eval(160) != -100 {
		t.Errorf("SpareVert(160) expected -100, got %f", sv.Eval(160))
	}

	// 4. Chainstay
	cs := critMap["chainstay"].Curve
	if cs.Eval(380) != -100 {
		t.Errorf("Chainstay(380) expected -100, got %f", cs.Eval(380))
	}
	if cs.Eval(420) != 0 {
		t.Errorf("Chainstay(420) expected 0, got %f", cs.Eval(420))
	}
	if cs.Eval(445) != 70 {
		t.Errorf("Chainstay(445) expected 70, got %f", cs.Eval(445))
	}
	if cs.Eval(470) != 100 {
		t.Errorf("Chainstay(470) expected 100, got %f", cs.Eval(470))
	}
	if cs.Eval(480) != 100 {
		t.Errorf("Chainstay(480) expected 100, got %f", cs.Eval(480))
	}

	// 5. BB Drop
	bb := critMap["bbdrop"].Curve
	if bb.Eval(50) != -100 {
		t.Errorf("BBDrop(50) expected -100, got %f", bb.Eval(50))
	}
	if math.Abs(bb.Eval(70)-100) > 0.001 {
		t.Errorf("BBDrop(70) expected 100, got %f", bb.Eval(70))
	}
	if math.Abs(bb.Eval(110)-(-100)) > 0.001 {
		t.Errorf("BBDrop(110) expected -100, got %f", bb.Eval(110))
	}
	// Below 70 loses 15 pts/mm: at 65 should be 100 - 5*15 = 25
	if math.Abs(bb.Eval(65)-25) > 0.001 {
		t.Errorf("BBDrop(65) expected 25, got %f", bb.Eval(65))
	}
	// Above 70 loses 5 pts/mm: at 75 should be 100 - 5*5 = 75
	if math.Abs(bb.Eval(75)-75) > 0.001 {
		t.Errorf("BBDrop(75) expected 75, got %f", bb.Eval(75))
	}

	// 6. HTA
	hta := critMap["hta"].Curve
	if hta.Eval(65) != -100 {
		t.Errorf("HTA(65) expected -100, got %f", hta.Eval(65))
	}
	if hta.Eval(67) != 0 {
		t.Errorf("HTA(67) expected 0, got %f", hta.Eval(67))
	}
	if hta.Eval(68) != 100 {
		t.Errorf("HTA(68) expected 100, got %f", hta.Eval(68))
	}
	if hta.Eval(69) != 0 {
		t.Errorf("HTA(69) expected 0, got %f", hta.Eval(69))
	}
	if hta.Eval(70) != -200 {
		t.Errorf("HTA(70) expected -200, got %f", hta.Eval(70))
	}

	// 7. Front Wt %
	fw := critMap["frontwt"].Curve
	if fw.Eval(25) != -100 {
		t.Errorf("FrontWt(25) expected -100, got %f", fw.Eval(25))
	}
	if fw.Eval(33) != 0 {
		t.Errorf("FrontWt(33) expected 0, got %f", fw.Eval(33))
	}
	if fw.Eval(38) != 100 {
		t.Errorf("FrontWt(38) expected 100, got %f", fw.Eval(38))
	}
	if fw.Eval(45) != 100 {
		t.Errorf("FrontWt(45) expected 100, got %f", fw.Eval(45))
	}
}

func TestScoreAll_MatchesInterviewExpected(t *testing.T) {
	// Analyses from the real comparison matrix
	analyses := []struct {
		name     string
		expected float64 // rounded total expected
		a        model.BikeFitAnalysis
	}{
		{
			name:     "Esker Hayduke (S3)",
			expected: 66,
			a: model.BikeFitAnalysis{
				Aett:               634.9,
				ActualSetback:      213.4,
				SpareVerticalStack: 121.2,
				ActiveChainstay:    454,
				ActiveBbDrop:       65.9,
				ActiveHta:          67.62,
				WeightDistFrontPct: 35.4,
			},
		},
		{
			name:     "Ritchey Ultra (XL)",
			expected: 61,
			a: model.BikeFitAnalysis{
				Aett:               633.3,
				ActualSetback:      219.8,
				SpareVerticalStack: 122.5,
				ActiveChainstay:    439,
				ActiveBbDrop:       59,
				ActiveHta:          68.63,
				WeightDistFrontPct: 35.2,
			},
		},
		{
			name:     "Binary Havok (XL)",
			expected: 40,
			a: model.BikeFitAnalysis{
				Aett:               592.8,
				ActualSetback:      216.3,
				SpareVerticalStack: 78.7,
				ActiveChainstay:    460,
				ActiveBbDrop:       69.3,
				ActiveHta:          68.9,
				WeightDistFrontPct: 37.7,
			},
		},
		{
			name:     "Otso Fenrir (XL)",
			expected: 40,
			a: model.BikeFitAnalysis{
				Aett:               604.3,
				ActualSetback:      206,
				SpareVerticalStack: 88,
				ActiveChainstay:    450,
				ActiveBbDrop:       67.7,
				ActiveHta:          67.69,
				WeightDistFrontPct: 35.5,
			},
		},
		{
			name:     "Otso Fenrir (L)",
			expected: 37,
			a: model.BikeFitAnalysis{
				Aett:               584,
				ActualSetback:      205.7,
				SpareVerticalStack: 106.4,
				ActiveChainstay:    450,
				ActiveBbDrop:       67.8,
				ActiveHta:          67.71,
				WeightDistFrontPct: 36.2,
			},
		},
		{
			name:     "Sour Pasta Party (L)",
			expected: 35,
			a: model.BikeFitAnalysis{
				Aett:               621.2,
				ActualSetback:      202.9,
				SpareVerticalStack: 141.7,
				ActiveChainstay:    435,
				ActiveBbDrop:       71.9,
				ActiveHta:          67.52,
				WeightDistFrontPct: 34.4,
			},
		},
		{
			name:     "Revel El Jefe (L)",
			expected: 35,
			a: model.BikeFitAnalysis{
				Aett:               619.7,
				ActualSetback:      205.3,
				SpareVerticalStack: 135.4,
				ActiveChainstay:    435,
				ActiveBbDrop:       66.1,
				ActiveHta:          67.24,
				WeightDistFrontPct: 34.3,
			},
		},
		{
			name:     "Revel El Jefe (XL)",
			expected: 0,
			a: model.BikeFitAnalysis{
				Aett:               649.1,
				ActualSetback:      203.5,
				SpareVerticalStack: 125,
				ActiveChainstay:    435,
				ActiveBbDrop:       67.1,
				ActiveHta:          67.38,
				WeightDistFrontPct: 33.5,
			},
		},
		{
			name:     "Esker Hayduke (S4)",
			expected: -21,
			a: model.BikeFitAnalysis{
				Aett:               671.8,
				ActualSetback:      215.5,
				SpareVerticalStack: 104.2,
				ActiveChainstay:    454,
				ActiveBbDrop:       64.7,
				ActiveHta:          67.46,
				WeightDistFrontPct: 34.3,
			},
		},
	}

	var input []model.BikeFitAnalysis
	for _, item := range analyses {
		input = append(input, item.a)
	}

	results := ScoreAll(input, DefaultCriteria)
	for i, r := range results {
		rounded := math.Round(r.Total)
		if rounded != analyses[i].expected {
			t.Errorf("%s expected total score %.0f, got %.1f (rounded %.0f)",
				analyses[i].name, analyses[i].expected, r.Total, rounded)
		}
		if r.Rank != i+1 {
			t.Errorf("%s expected rank %d, got %d", analyses[i].name, i+1, r.Rank)
		}
	}
}

func TestExplain_GeneratesReadableOutput(t *testing.T) {
	frames := []ScoredFrame{
		{
			Name: "Esker Hayduke (S3)",
			Score: Score(model.BikeFitAnalysis{
				Aett:               634.9,
				ActualSetback:      213.4,
				SpareVerticalStack: 121.2,
				ActiveChainstay:    454.0,
				ActiveBbDrop:       65.9,
				ActiveHta:          67.62,
				WeightDistFrontPct: 35.4,
			}, DefaultCriteria),
		},
		{
			Name: "Ritchey Ultra (XL)",
			Score: Score(model.BikeFitAnalysis{
				Aett:               633.3,
				ActualSetback:      219.8,
				SpareVerticalStack: 122.5,
				ActiveChainstay:    439.0,
				ActiveBbDrop:       59.0,
				ActiveHta:          68.62,
				WeightDistFrontPct: 35.2,
			}, DefaultCriteria),
		},
	}

	expl := Explain(frames, DefaultCriteria)
	if len(expl.Losses) != 2 {
		t.Fatalf("Expected 2 loss entries, got %d", len(expl.Losses))
	}
	if len(expl.HeadToHead) != 1 {
		t.Fatalf("Expected 1 head-to-head entry, got %d", len(expl.HeadToHead))
	}

	// Verify head-to-head mentions Hayduke beating Ritchey by 5 (rounded 66 vs 61)
	if !contains(expl.HeadToHead[0], "Esker Hayduke (S3) (66) > Ritchey Ultra (XL) (61) by 5") {
		t.Errorf("Unexpected H2H line: %s", expl.HeadToHead[0])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
