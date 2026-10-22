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

func TestQuadraticCurveEval(t *testing.T) {
	qc := QuadraticCurve{
		Low:   625,
		High:  636,
		WLow:  41,
		WHigh: 24,
		Floor: -200,
		Peak:  100,
	}

	// Inside plateau
	for _, x := range []float64{625, 630, 634, 636} {
		if val := qc.Eval(x); val != 100 {
			t.Errorf("Eval(%f) inside plateau expected 100, got %f", x, val)
		}
	}

	// Slight deviation above High (Hayduke at 637mm: 1mm over)
	if val := qc.Eval(637); val < 99.8 || val > 100.0 {
		t.Errorf("Eval(637) expected ~99.83, got %f", val)
	}

	// Zero crossing high
	if math.Abs(qc.Eval(660)-0) > 0.001 {
		t.Errorf("Eval(660) expected 0, got %f", qc.Eval(660))
	}

	// Zero crossing low
	if math.Abs(qc.Eval(584)-0) > 0.001 {
		t.Errorf("Eval(584) expected 0, got %f", qc.Eval(584))
	}

	// Floor clamping
	if val := qc.Eval(700); val != -200 {
		t.Errorf("Eval(700) expected -200 (clamped), got %f", val)
	}
	if val := qc.Eval(500); val != -200 {
		t.Errorf("Eval(500) expected -200 (clamped), got %f", val)
	}
}

func TestDefaultCriteriaCurves(t *testing.T) {
	critMap := make(map[string]Criterion)
	for _, c := range DefaultCriteria {
		critMap[c.ID] = c
	}

	// 1. AETT
	aett := critMap["aett"].Curve
	if aett.Eval(500) != -200 {
		t.Errorf("AETT(500) expected -200, got %f", aett.Eval(500))
	}
	if math.Abs(aett.Eval(584)-0) > 0.001 {
		t.Errorf("AETT(584) expected 0, got %f", aett.Eval(584))
	}
	if aett.Eval(625) != 100 {
		t.Errorf("AETT(625) expected 100, got %f", aett.Eval(625))
	}
	if aett.Eval(634) != 100 {
		t.Errorf("AETT(634) expected 100, got %f", aett.Eval(634))
	}
	if aett.Eval(636) != 100 {
		t.Errorf("AETT(636) expected 100, got %f", aett.Eval(636))
	}
	// 3mm past 634 is 637mm: 1mm past 636 sweet spot -> > 99.8
	if aett.Eval(637) < 99.8 {
		t.Errorf("AETT(637) expected > 99.8, got %f", aett.Eval(637))
	}
	if math.Abs(aett.Eval(660)-0) > 0.001 {
		t.Errorf("AETT(660) expected 0, got %f", aett.Eval(660))
	}
	if aett.Eval(680) != -200 {
		t.Errorf("AETT(680) expected -200, got %f", aett.Eval(680))
	}

	// 2. Setback: peak 230mm, 220–240mm band (225/235 = 95, 220/240 = 80), 210/250 = 0, floor -200 at <=200 / >=260
	sb := critMap["setback"].Curve
	if sb.Eval(180) != -200 {
		t.Errorf("Setback(180) expected -200, got %f", sb.Eval(180))
	}
	if sb.Eval(200) != -200 {
		t.Errorf("Setback(200) expected -200, got %f", sb.Eval(200))
	}
	if math.Abs(sb.Eval(210)-0) > 0.001 {
		t.Errorf("Setback(210) expected 0, got %f", sb.Eval(210))
	}
	if sb.Eval(215) != 40 {
		t.Errorf("Setback(215) expected 40, got %f", sb.Eval(215))
	}
	if sb.Eval(220) != 80 {
		t.Errorf("Setback(220) expected 80, got %f", sb.Eval(220))
	}
	if sb.Eval(225) != 95 {
		t.Errorf("Setback(225) expected 95, got %f", sb.Eval(225))
	}
	if sb.Eval(230) != 100 {
		t.Errorf("Setback(230) expected 100, got %f", sb.Eval(230))
	}
	if sb.Eval(235) != 95 {
		t.Errorf("Setback(235) expected 95, got %f", sb.Eval(235))
	}
	if sb.Eval(240) != 80 {
		t.Errorf("Setback(240) expected 80, got %f", sb.Eval(240))
	}
	if math.Abs(sb.Eval(250)-0) > 0.001 {
		t.Errorf("Setback(250) expected 0, got %f", sb.Eval(250))
	}
	if sb.Eval(260) != -200 {
		t.Errorf("Setback(260) expected -200, got %f", sb.Eval(260))
	}

	// 3. Spare Vert: sweet spot [70, 100], WLow=15, WHigh=35, Floor=-100
	sv := critMap["sparevert"].Curve
	if sv.Eval(45) != -100 {
		t.Errorf("SpareVert(45) expected -100, got %f", sv.Eval(45))
	}
	if math.Abs(sv.Eval(55)-0) > 0.001 {
		t.Errorf("SpareVert(55) expected 0, got %f", sv.Eval(55))
	}
	if sv.Eval(70) != 100 {
		t.Errorf("SpareVert(70) expected 100, got %f", sv.Eval(70))
	}
	if sv.Eval(85) != 100 {
		t.Errorf("SpareVert(85) expected 100, got %f", sv.Eval(85))
	}
	if sv.Eval(PreferredSpareVertMaxMm) != 100 {
		t.Errorf("SpareVert(%.0f) expected 100, got %f", PreferredSpareVertMaxMm, sv.Eval(PreferredSpareVertMaxMm))
	}
	if math.Abs(sv.Eval(135)-0) > 0.001 {
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
	if math.Abs(bb.Eval(50)-0) > 0.001 {
		t.Errorf("BBDrop(50) expected 0, got %f", bb.Eval(50))
	}
	if math.Abs(bb.Eval(70)-100) > 0.001 {
		t.Errorf("BBDrop(70) expected 100, got %f", bb.Eval(70))
	}
	if math.Abs(bb.Eval(90)-0) > 0.001 {
		t.Errorf("BBDrop(90) expected 0, got %f", bb.Eval(90))
	}
	if bb.Eval(110) != -100 {
		t.Errorf("BBDrop(110) expected -100, got %f", bb.Eval(110))
	}
	// At 65 (5mm higher BB, W=20): 100 - 100*(5/20)^2 = 93.75
	if math.Abs(bb.Eval(65)-93.75) > 0.001 {
		t.Errorf("BBDrop(65) expected 93.75, got %f", bb.Eval(65))
	}
	// At 75 (5mm lower BB, W=20): 100 - 100*(5/20)^2 = 93.75
	if math.Abs(bb.Eval(75)-93.75) > 0.001 {
		t.Errorf("BBDrop(75) expected 93.75, got %f", bb.Eval(75))
	}

	// 6. HTA
	hta := critMap["hta"].Curve
	if hta.Eval(65) != -200 {
		t.Errorf("HTA(65) expected -200, got %f", hta.Eval(65))
	}
	if math.Abs(hta.Eval(67)-0) > 0.001 {
		t.Errorf("HTA(67) expected 0, got %f", hta.Eval(67))
	}
	if hta.Eval(68) != 100 {
		t.Errorf("HTA(68) expected 100, got %f", hta.Eval(68))
	}
	if math.Abs(hta.Eval(69)-0) > 0.001 {
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
			name:     "Ritchey Ultra (XL)",
			expected: 76,
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
			name:     "Esker Hayduke (S3)",
			expected: 69,
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
			name:     "Binary Havok (XL)",
			expected: 62,
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
			expected: 36,
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
			name:     "Revel El Jefe (L)",
			expected: 17,
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
			name:     "Esker Hayduke (S4)",
			expected: 14,
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
		{
			name:     "Otso Fenrir (L)",
			expected: 13,
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
			name:     "Revel El Jefe (XL)",
			expected: 7,
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
			name:     "Sour Pasta Party (L)",
			expected: 0,
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

	// Verify head-to-head mentions Ritchey beating Hayduke by 7 (rounded 76 vs 69)
	if !contains(expl.HeadToHead[0], "Ritchey Ultra (XL) (76) > Esker Hayduke (S3) (69) by 7") {
		t.Errorf("Unexpected H2H line: %s", expl.HeadToHead[0])
	}
}

func TestScore_PointsLost(t *testing.T) {
	// AETT = 630 (ideal sweet spot 625-636), subscore = 100, points lost should be 0
	// Spare vert = 119mm (19mm past 100mm sweet spot), subscore = 70.5, weight = 5, totalWeight = 31
	// pointsLost = 5 * (100 - 70.5) / 31 ≈ 4.75 -> rounds to 5
	analysis := model.BikeFitAnalysis{
		Aett:               630.0,
		ActualSetback:      230.0,
		SpareVerticalStack: 119.0,
		ActiveChainstay:    470.0,
		ActiveBbDrop:       70.0,
		ActiveHta:          68.0,
		WeightDistFrontPct: 38.0,
	}
	res := Score(analysis, DefaultCriteria)
	for _, c := range res.Criteria {
		if c.ID == "aett" {
			if c.PointsLost != 0 {
				t.Errorf("Expected AETT points lost 0, got %f", c.PointsLost)
			}
		}
		if c.ID == "setback" {
			if c.PointsLost != 0 {
				t.Errorf("Expected Setback points lost 0, got %f", c.PointsLost)
			}
		}
		if c.ID == "sparevert" {
			if math.Round(c.PointsLost) != 4 {
				t.Errorf("Expected Spare Vert points lost ~4, got %f", c.PointsLost)
			}
		}
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
