package scoring

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

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

func TestScoreRow_MissingDataRenormalization(t *testing.T) {
	prof := &ideal.Profile{
		Name: "test-missing",
		Attributes: []ideal.AttributeSpec{
			{
				ID:        "setback",
				Weight:    10,
				Ideal:     &ideal.Range{Min: new(float64(230)), Max: new(float64(230))},
				EdgeScore: new(float64(80)),
				Falloff:   &ideal.Falloff{Below: new(float64(10)), Above: new(float64(10))},
			},
			{
				ID:        "standover",
				Weight:    10,
				Ideal:     &ideal.Range{Min: new(float64(700)), Max: new(float64(750))},
				EdgeScore: new(float64(80)),
				Falloff:   &ideal.Falloff{Below: new(float64(50)), Above: new(float64(50))},
			},
		},
	}

	// Frame A has both setback=230 (100) and standover=720 (100) -> total 100
	rowA := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			StandoverHeight: 720,
		},
		Canonical: model.BikeFitAnalysis{
			ActualSetback: 230,
		},
	}
	resA := ScoreRow(rowA, prof)
	if math.Round(resA.Total) != 100 {
		t.Errorf("Frame A expected total 100, got %.1f", resA.Total)
	}

	// Frame B has setback=230 (100) and standover=0 (missing)
	// Active weight should renormalize to 10/10 -> total 100
	rowB := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			StandoverHeight: 0,
		},
		Canonical: model.BikeFitAnalysis{
			ActualSetback: 230,
		},
	}
	resB := ScoreRow(rowB, prof)
	if math.Round(resB.Total) != 100 {
		t.Errorf("Frame B expected total 100 with renormalized weight, got %.1f", resB.Total)
	}
	if !resB.Criteria[1].Missing {
		t.Errorf("Expected standover criterion on Frame B to have Missing=true")
	}
	if resB.Criteria[1].FormattedCell != "— (no data)" {
		t.Errorf("Expected '— (no data)', got %q", resB.Criteria[1].FormattedCell)
	}

	// Test CollectMissingDataNotes
	frames := []ScoredFrame{
		{Name: "Bike A", Score: resA},
		{Name: "Bike B", Score: resB},
	}
	notes := CollectMissingDataNotes(frames)
	if len(notes) != 1 || !contains(notes[0], "Standover not scored for: Bike B") {
		t.Errorf("Unexpected missing notes: %v", notes)
	}
}

func TestExplainProfile_LossesAndH2H(t *testing.T) {
	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		t.Fatalf("Failed to load idmatch profile: %v", err)
	}

	rowRitchey := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: model.BRPPoint{
				BrpSetbackMm: 219.0,
			},
			SelectedBRP: &model.BRPPoint{
				BrpSetbackMm: 219.0,
			},
			CockpitFitResult: &model.CockpitFitResult{
				HasFit:        true,
				FinalSubScore: 100,
				CountInside:   4,
			},
			ActiveBbDrop:       70.0,
			WeightDistFrontPct: 37.0,
			TrailMm:            102.0,
		},
	}
	rowHayduke := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: model.BRPPoint{
				BrpSetbackMm: 195.0,
			},
			SelectedBRP: &model.BRPPoint{
				BrpSetbackMm: 195.0,
			},
			CockpitFitResult: &model.CockpitFitResult{
				HasFit:        true,
				FinalSubScore: 100,
				CountInside:   4,
			},
			ActiveBbDrop:       70.0,
			WeightDistFrontPct: 37.0,
			TrailMm:            102.0,
		},
	}

	scoreRitchey := ScoreRow(rowRitchey, prof)
	scoreHayduke := ScoreRow(rowHayduke, prof)

	frames := []ScoredFrame{
		{Name: "Ritchey Ultra (XL)", Score: scoreRitchey, Row: rowRitchey},
		{Name: "Esker Hayduke (S3)", Score: scoreHayduke, Row: rowHayduke},
	}

	expl := ExplainProfile(frames, prof)
	if len(expl.Losses) != 2 {
		t.Fatalf("Expected 2 losses, got %d", len(expl.Losses))
	}
	if len(expl.HeadToHead) != 1 {
		t.Fatalf("Expected 1 head-to-head, got %d", len(expl.HeadToHead))
	}

	// Verify Loss text mentions BRP Setback for Hayduke
	if !contains(expl.Losses[1], "Esker Hayduke (S3)") || !contains(expl.Losses[1], "BRP Setback") {
		t.Errorf("Expected BRP Setback loss in Hayduke explanation: %s", expl.Losses[1])
	}
}

func TestScoreRow_SaddleHb(t *testing.T) {
	prof := &ideal.Profile{
		Name: "test-saddle-hb",
		Attributes: []ideal.AttributeSpec{
			{
				ID:        "saddle_hb",
				Weight:    10,
				Ideal:     &ideal.Range{Min: new(float64(695)), Max: new(float64(705))},
				EdgeScore: new(float64(80)),
				Falloff:   &ideal.Falloff{Below: new(float64(50)), Above: new(float64(50))},
			},
		},
	}

	canonicalStem := model.StemSpec{LengthMm: 60, AngleDeg: -6}
	possibleHBs := []model.HBPoint{
		{
			Stem:       model.StemSpec{LengthMm: 50, AngleDeg: -6},
			SpacerMm:   10,
			HbX:        500,
			HbY:        650,
			SaddleHbMm: 680,
		},
		{
			Stem:       model.StemSpec{LengthMm: 70, AngleDeg: -6},
			SpacerMm:   20,
			HbX:        520,
			HbY:        660,
			SaddleHbMm: 700, // Exact target match
		},
		{
			Stem:       model.StemSpec{LengthMm: 90, AngleDeg: 0},
			SpacerMm:   0,
			HbX:        540,
			HbY:        670,
			SaddleHbMm: 730,
		},
	}

	row := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			CanonicalStem: canonicalStem,
			Stems: []model.StemSpec{
				{LengthMm: 50, AngleDeg: -6},
				{LengthMm: 70, AngleDeg: -6},
				{LengthMm: 90, AngleDeg: 0},
			},
		},
		Canonical: model.BikeFitAnalysis{
			PossibleHBs: possibleHBs,
			SelectedHB:  &possibleHBs[0], // Initially non-target
			SaddleHbMm:  possibleHBs[0].SaddleHbMm,
		},
	}

	res := ScoreRow(row, prof)
	if math.Round(res.Total) != 100 {
		t.Errorf("Expected total score 100 for Saddle-HB in target range [695, 705], got %.1f", res.Total)
	}

	if len(res.Criteria) != 1 {
		t.Fatalf("Expected 1 criterion, got %d", len(res.Criteria))
	}

	crit := res.Criteria[0]
	if crit.ID != "saddle_hb" {
		t.Errorf("Expected criterion ID 'saddle_hb', got %s", crit.ID)
	}
	if crit.FormattedCell != "700 ✓" {
		t.Errorf("Expected formatted cell '700 ✓', got %q", crit.FormattedCell)
	}
}

func TestScoreRow_IdmatchFit(t *testing.T) {
	// Ideal profile with BRP Setback (target 219mm), Saddle-HB (target 700mm), and Saddle-HB Drop (target 13mm)
	prof := &ideal.Profile{
		Name:           "test-idmatch",
		SaddleHeightMm: new(float64(746)),
		Attributes: []ideal.AttributeSpec{
			{
				ID:         "brp_setback",
				Weight:     10,
				Ideal:      &ideal.Range{Min: new(float64(219)), Max: new(float64(219))}, // target 219mm
				Acceptable: &ideal.Range{Min: new(float64(214)), Max: new(float64(224))}, // within ±5mm of rail center
				EdgeScore:  new(float64(75)),
				Falloff:    &ideal.Falloff{Below: new(float64(20)), Above: new(float64(20))},
			},
			{
				ID:         "saddle_hb",
				Weight:     10,
				Ideal:      &ideal.Range{Min: new(float64(695)), Max: new(float64(705))},
				Acceptable: &ideal.Range{Min: new(float64(685)), Max: new(float64(715))},
				EdgeScore:  new(float64(75)),
				Falloff:    &ideal.Falloff{Below: new(float64(15)), Above: new(float64(15))},
			},
			{
				ID:         "saddle_hb_drop",
				Weight:     5,
				Ideal:      &ideal.Range{Min: new(float64(-2)), Max: new(float64(8))},
				Acceptable: &ideal.Range{Min: new(float64(-10)), Max: new(float64(16))},
				EdgeScore:  new(float64(75)),
				Falloff:    &ideal.Falloff{Below: new(float64(10)), Above: new(float64(10))},
			},
		},
	}

	// Frame geometry with ideal 72.0° STA at saddle height 746mm
	staDeg := 72.0
	seatHeight := 746.0
	saddle := model.ReferenceIdmatchSaddle()
	deadCenterBRP := saddle.CalculatePoint(staDeg, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	possibleBRPs := saddle.MapPossibleBRPs(staDeg, seatHeight, model.DefaultSeatpostSetbackMm, model.DefaultSeatpostClampLengthMm)

	canonicalStem := model.StemSpec{LengthMm: 60, AngleDeg: -6}
	maxSpacer := 30.0

	bike := model.BikeGeometry{
		Reach:         490.0,
		Stack:         625.0,
		HtaDeg:        68.0,
		StaDeg:        staDeg,
		MaxSpacerMm:   maxSpacer,
		CanonicalStem: canonicalStem,
		Stems: []model.StemSpec{
			{LengthMm: 50, AngleDeg: -6},
			{LengthMm: 60, AngleDeg: -6},
			{LengthMm: 70, AngleDeg: -6},
			{LengthMm: 80, AngleDeg: -6},
		},
	}

	row := model.CanonicalComparisonRow{
		Bike: bike,
		Canonical: model.BikeFitAnalysis{
			ActiveReach:   490.0,
			ActiveStack:   625.0,
			ActiveHta:     68.0,
			ActiveSta:     staDeg,
			Saddle:        saddle,
			DeadCenterBRP: deadCenterBRP,
			PossibleBRPs:  possibleBRPs,
		},
	}

	res := ScoreRow(row, prof)
	if res.Total < 90.0 {
		t.Errorf("Expected score > 90 for 72.0° frame (ideal setback match), got %.1f", res.Total)
	}

	// Verify criteria results for 72.0° frame
	for _, c := range res.Criteria {
		if c.ID == "brp_setback" {
			if c.FormattedCell != "219 ✓" {
				t.Errorf("Expected brp_setback cell '219 ✓', got %q", c.FormattedCell)
			}
		}
		if c.ID == "saddle_hb" {
			if c.SubScore < 95.0 {
				t.Errorf("Expected saddle_hb subscore >= 95, got %.1f (%q)", c.SubScore, c.FormattedCell)
			}
		}
	}

	// Verify that steep STA = 74.53° (Otso Fenrir) is properly penalized without a Drop Best
	fenrirSTA := 74.53
	fenrirDeadCenter := saddle.CalculatePoint(fenrirSTA, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	fenrirPossible := saddle.MapPossibleBRPs(fenrirSTA, seatHeight, model.DefaultSeatpostSetbackMm, model.DefaultSeatpostClampLengthMm)
	fenrirBike := bike
	fenrirBike.StaDeg = fenrirSTA
	fenrirRow := model.CanonicalComparisonRow{
		Bike: fenrirBike,
		Canonical: model.BikeFitAnalysis{
			ActiveReach:   465.0,
			ActiveStack:   625.0,
			ActiveHta:     68.0,
			ActiveSta:     fenrirSTA,
			Saddle:        saddle,
			DeadCenterBRP: fenrirDeadCenter,
			PossibleBRPs:  fenrirPossible,
		},
	}
	fenrirRes := ScoreRow(fenrirRow, prof)
	if fenrirRes.Total > 50.0 {
		t.Errorf("Expected score < 50 for Otso Fenrir on Redshift post alone (cannot reach 219mm), got %.1f", fenrirRes.Total)
	}
}

func TestScoreRow_SeatpostExtension(t *testing.T) {
	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		t.Fatalf("Failed to load idmatch profile: %v", err)
	}

	buildRow := func(seatTubeLength float64) model.CanonicalComparisonRow {
		seatHeight := 746.0
		ext := geometry.CalculateSeatpostExtension(seatHeight, seatTubeLength)
		return model.CanonicalComparisonRow{
			Bike: model.BikeGeometry{
				SeatTubeLength: seatTubeLength,
			},
			Canonical: model.BikeFitAnalysis{
				SeatpostExtensionMm: ext,
			},
		}
	}

	// 1. Ideal extension: seat tube 466mm -> extension = 280mm (within ideal [250, 310])
	rowIdeal := buildRow(466.0)
	resIdeal := ScoreRow(rowIdeal, prof)
	var critIdeal *CriterionResult
	for i := range resIdeal.Criteria {
		if resIdeal.Criteria[i].ID == "seatpost_extension" {
			critIdeal = &resIdeal.Criteria[i]
			break
		}
	}
	if critIdeal == nil {
		t.Fatalf("seatpost_extension criterion not found in idmatch results")
	}
	if critIdeal.SubScore != 100.0 {
		t.Errorf("Expected subscore 100.0 for 280mm extension, got %.1f", critIdeal.SubScore)
	}
	if critIdeal.Zone != string(ideal.ZoneIdeal) {
		t.Errorf("Expected ZoneIdeal for 280mm extension, got %s", critIdeal.Zone)
	}

	// 2. Acceptable low extension: seat tube 526mm -> extension = 220mm (within acceptable [210, 340], ideal min 250)
	// Ratio: (220 - 210) / (250 - 210) = 10 / 40 = 0.25 -> 75 + 25*0.25 = 81.25
	rowAcc := buildRow(526.0)
	resAcc := ScoreRow(rowAcc, prof)
	var critAcc *CriterionResult
	for i := range resAcc.Criteria {
		if resAcc.Criteria[i].ID == "seatpost_extension" {
			critAcc = &resAcc.Criteria[i]
			break
		}
	}
	if critAcc == nil || math.Abs(critAcc.SubScore-81.25) > 0.5 {
		t.Errorf("Expected subscore ~81.25 for 220mm extension, got %+v", critAcc)
	}
	if critAcc.Zone != string(ideal.ZoneAcceptable) {
		t.Errorf("Expected ZoneAcceptable for 220mm extension, got %s", critAcc.Zone)
	}

	// 3. Severely short extension: Fargo XL (seat tube 558.8mm) -> extension = 187.2mm
	// 22.8mm below acceptable min (210mm) -> with 25mm falloff, slope 75/25=3 -> subscore = 75 - 22.8*3 = 6.6
	rowFargoXL := buildRow(558.8)
	resFargoXL := ScoreRow(rowFargoXL, prof)
	var critFargoXL *CriterionResult
	for i := range resFargoXL.Criteria {
		if resFargoXL.Criteria[i].ID == "seatpost_extension" {
			critFargoXL = &resFargoXL.Criteria[i]
			break
		}
	}
	if critFargoXL == nil || critFargoXL.SubScore > 15.0 {
		t.Errorf("Expected severe penalty (< 15 subscore) for Fargo XL 187.2mm extension, got %+v", critFargoXL)
	}
	if critFargoXL.Zone != string(ideal.ZoneOutside) {
		t.Errorf("Expected ZoneOutside for Fargo XL 187.2mm extension, got %s", critFargoXL.Zone)
	}
}

func TestScoreRow_HandlebarClampFit(t *testing.T) {
	// Profile with handlebar_clamp_fit, saddle_hb target (700mm), and saddle_hb_drop target (13mm)
	prof := &ideal.Profile{
		Name:           "test-clamp-fit",
		SaddleHeightMm: new(float64(746)),
		Attributes: []ideal.AttributeSpec{
			{
				ID:         "handlebar_clamp_fit",
				Weight:     15,
				Ideal:      ideal.AtLeast(100),
				Acceptable: ideal.AtLeast(75),
				EdgeScore:  new(float64(75)),
				Falloff:    ideal.FalloffBelow(75),
				Floor:      new(float64(-200)),
			},
			ideal.DisplayOnlyWithTarget("saddle_hb", ideal.Between(695, 705), ideal.Between(685, 715), new(float64(75)), ideal.FalloffBoth(15, 15)),
			ideal.DisplayOnlyWithTarget("saddle_hb_drop", ideal.Between(8, 18), ideal.Between(0, 26), new(float64(75)), ideal.FalloffBoth(10, 10)),
		},
	}

	brp := model.BRPPoint{
		BrpX:         -219.0,
		BrpY:         713.0,
		BrpSetbackMm: 219.0,
	}

	canonicalStem := model.StemSpec{LengthMm: 70, AngleDeg: -6}

	// 1. Exact Center & Multiple Setups inside oval
	// Ideal center:
	// dx0 = sqrt(700^2 - 13^2) = 699.879 -> Target HbX = -219 + 699.879 = 480.879
	// dy0 = 13 -> Target HbY = 713 + 13 = 726.0
	targetHbX := -219.0 + math.Sqrt(700.0*700.0-13.0*13.0)
	targetHbY := 713.0 + 13.0

	possibleHBs := []model.HBPoint{
		{
			Stem:           canonicalStem,
			SpacerMm:       10,
			HbX:            targetHbX,
			HbY:            targetHbY,
			SaddleHbMm:     700.0,
			SaddleHbDropMm: 13.0,
		},
		{
			Stem:           model.StemSpec{LengthMm: 80, AngleDeg: -6},
			SpacerMm:       15,
			HbX:            targetHbX + 5.0,
			HbY:            targetHbY + 5.0,
			SaddleHbMm:     707.0,
			SaddleHbDropMm: 18.0,
		},
		{
			Stem:           model.StemSpec{LengthMm: 60, AngleDeg: -6},
			SpacerMm:       5,
			HbX:            targetHbX - 5.0,
			HbY:            targetHbY - 5.0,
			SaddleHbMm:     693.0,
			SaddleHbDropMm: 8.0,
		},
	}

	rowMulti := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			CanonicalStem: canonicalStem,
		},
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: brp,
			PossibleHBs:   possibleHBs,
		},
	}

	resMulti := ScoreRow(rowMulti, prof)
	if resMulti.Total != 100.0 {
		t.Errorf("Expected total score 100 for exact center with multi-setup bonus, got %.1f", resMulti.Total)
	}

	var critClamp *CriterionResult
	for i := range resMulti.Criteria {
		if resMulti.Criteria[i].ID == "handlebar_clamp_fit" {
			critClamp = &resMulti.Criteria[i]
			break
		}
	}
	if critClamp == nil {
		t.Fatalf("Expected handlebar_clamp_fit criterion")
	}
	if critClamp.FormattedCell != "3 ✓" {
		t.Errorf("Expected cell to show '3 ✓', got %q", critClamp.FormattedCell)
	}

	// 2. Zero setups inside oval (continuous roll-off past the oval edge)
	// Place single stem outside oval at R = 1.10 along minor axis
	dx := 11.0 * math.Cos(30.0*math.Pi/180.0)
	dy := -11.0 * math.Sin(30.0*math.Pi/180.0)
	possibleZero := []model.HBPoint{
		{
			Stem:           model.StemSpec{LengthMm: 90, AngleDeg: 0},
			SpacerMm:       0,
			HbX:            targetHbX + dx,
			HbY:            targetHbY + dy,
			SaddleHbMm:     720.0,
			SaddleHbDropMm: 7.5,
		},
	}
	rowZero := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			CanonicalStem: canonicalStem,
		},
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: brp,
			PossibleHBs:   possibleZero,
		},
	}
	resZero := ScoreRow(rowZero, prof)
	// BaseScore at R=1.10 is ~65.1 (continuous past the oval edge, no flat penalty).
	if math.Abs(resZero.Total-65.1) > 0.5 {
		t.Errorf("Expected score ~65.1 for R=1.10, got %.1f", resZero.Total)
	}
}

func TestScoreRow_HandlebarClampFit_StemExtremityPenalty(t *testing.T) {
	prof := &ideal.Profile{
		Name:           "test-stem-extreme",
		SaddleHeightMm: new(float64(746)),
		Attributes: []ideal.AttributeSpec{
			{
				ID:         "handlebar_clamp_fit",
				Weight:     15,
				Ideal:      ideal.AtLeast(100),
				Acceptable: ideal.AtLeast(75),
				EdgeScore:  new(float64(75)),
				Falloff:    ideal.FalloffBelow(75),
				Floor:      new(float64(-200)),
			},
			ideal.DisplayOnlyWithTarget("saddle_hb", ideal.Between(695, 705), ideal.Between(685, 715), new(float64(75)), ideal.FalloffBoth(15, 15)),
			ideal.DisplayOnlyWithTarget("saddle_hb_drop", ideal.Between(8, 18), ideal.Between(0, 26), new(float64(75)), ideal.FalloffBoth(10, 10)),
		},
	}

	brp := model.BRPPoint{
		BrpX:         -219.0,
		BrpY:         713.0,
		BrpSetbackMm: 219.0,
	}
	targetHbX := -219.0 + math.Sqrt(700.0*700.0-13.0*13.0)
	targetHbY := 713.0 + 13.0

	// Single setup inside oval at exact center, but using 100mm stem on frame expecting 60-100mm (midpoint 80mm)
	possible100 := []model.HBPoint{
		{
			Stem:           model.StemSpec{LengthMm: 100, AngleDeg: -6},
			SpacerMm:       10,
			HbX:            targetHbX,
			HbY:            targetHbY,
			SaddleHbMm:     700.0,
			SaddleHbDropMm: 13.0,
		},
	}

	row100 := model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			CanonicalStem:       model.StemSpec{LengthMm: 80, AngleDeg: -6},
			ExpectedStemRangeMm: []float64{60.0, 100.0},
		},
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: brp,
			PossibleHBs:   possible100,
		},
	}

	distVal, dropVal := 700.0, 13.0
	geometry.ApplyCockpitFit(&row100.Canonical, row100.Bike, row100.Bike.GetCanonicalStem(), &distVal, &distVal, &dropVal, &dropVal)

	res100 := ScoreRow(row100, prof)
	// BaseScore = 100.0, bonus = 0 (1 setup). StemPenalty = 15.0 -> SubScore = 85.0
	var critClamp *CriterionResult
	for i := range res100.Criteria {
		if res100.Criteria[i].ID == "handlebar_clamp_fit" {
			critClamp = &res100.Criteria[i]
			break
		}
	}
	if critClamp == nil {
		t.Fatalf("Expected handlebar_clamp_fit criterion")
	}
	if math.Abs(critClamp.SubScore-85.0) > 0.1 {
		t.Errorf("Expected subscore 85.0 due to 15pt stem penalty, got %.1f", critClamp.SubScore)
	}

	// Verify loss explanation mentions stem penalty and omits reach/stack offset when R=0.00
	meta, _ := ideal.LookupAttribute("handlebar_clamp_fit")
	spec := prof.Attributes[0]
	lossDesc := DescribeLossWithRow(spec, meta, critClamp.Value, critClamp.DistanceToIdeal, ideal.ZoneAcceptable, &row100)
	if !strings.Contains(lossDesc, "-15 stem penalty") {
		t.Errorf("Expected loss explanation to mention '-15 stem penalty', got: %s", lossDesc)
	}
	if strings.Contains(lossDesc, "reach:") || strings.Contains(lossDesc, "stack:") {
		t.Errorf("Expected loss explanation to omit reach and stack offsets when R=0.0, got: %s", lossDesc)
	}
}

func TestDescribeLoss_CockpitFitOffset_InsideOval(t *testing.T) {
	spec := ideal.AttributeSpec{ID: "handlebar_clamp_fit"}
	meta, _ := ideal.LookupAttribute("handlebar_clamp_fit")

	row := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			CockpitFitResult: &model.CockpitFitResult{
				TargetOval: model.FitOval{
					CenterX: 500.0,
					CenterY: 700.0,
				},
				SelectedHB: model.HBPoint{
					Stem:     model.StemSpec{LengthMm: 100, AngleDeg: -6},
					SpacerMm: 0,
					HbX:      489.0, // 11mm short
					HbY:      698.0, // 2mm low
				},
				BestR:       0.97,
				CountInside: 1,
				StemPenalty: 15.0,
			},
		},
	}

	desc := DescribeLossWithRow(spec, meta, 85, 0.97, ideal.ZoneAcceptable, &row)
	expected := "Oval Fits: closest stem is 100mm -6° +0mm (R=0.97, reach: 11mm short, stack: 2mm low, 1 valid setup in fit window, -15 stem penalty)"
	if desc != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, desc)
	}
}

func TestDescribeLoss_CockpitFitOffset_ZeroFits(t *testing.T) {
	spec := ideal.AttributeSpec{ID: "handlebar_clamp_fit"}
	meta, _ := ideal.LookupAttribute("handlebar_clamp_fit")

	row := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			CockpitFitResult: &model.CockpitFitResult{
				TargetOval: model.FitOval{
					CenterX: 500.0,
					CenterY: 700.0,
				},
				SelectedHB: model.HBPoint{
					Stem:     model.StemSpec{LengthMm: 100, AngleDeg: -6},
					SpacerMm: 0,
					HbX:      474.0, // 26mm short
					HbY:      756.0, // 56mm high
				},
				BestR:       5.37,
				CountInside: 0,
				StemPenalty: 15.0,
			},
		},
	}

	desc := DescribeLossWithRow(spec, meta, -200, 5.37, ideal.ZoneOutside, &row)
	expected := "Oval Fits: 0 valid setups in fit window (closest is 100mm -6° +0mm, R=5.37, reach: 26mm short, stack: 56mm high, -15 stem penalty)"
	if desc != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, desc)
	}
}

// Regression: three setups identical except for how far short the bars fall must
// rank by that distance, however far outside the fit oval they all are. A hard
// floor on the cockpit curve once made them tie, letting minor criteria decide.
func TestScoreAllRows_FarOutsideCockpitStillRanksByDistance(t *testing.T) {
	prof := &ideal.Profile{
		Name:           "test-far-cockpit",
		SaddleHeightMm: new(float64(746)),
		Attributes: []ideal.AttributeSpec{
			{
				ID:         "handlebar_clamp_fit",
				Weight:     15,
				Ideal:      ideal.AtLeast(100),
				Acceptable: ideal.AtLeast(75),
				EdgeScore:  new(float64(75)),
				Falloff:    ideal.FalloffBelow(75),
				Floor:      new(float64(-200)),
			},
			ideal.DisplayOnlyWithTarget("saddle_hb", ideal.Between(695, 705), ideal.Between(685, 715), new(float64(75)), ideal.FalloffBoth(15, 15)),
			ideal.DisplayOnlyWithTarget("saddle_hb_drop", ideal.Between(8, 18), ideal.Between(0, 26), new(float64(75)), ideal.FalloffBoth(10, 10)),
		},
	}

	brp := model.BRPPoint{BrpX: -219.0, BrpY: 713.0, BrpSetbackMm: 219.0}
	stem := model.StemSpec{LengthMm: 100, AngleDeg: -6}
	targetHbX := -219.0 + math.Sqrt(700.0*700.0-13.0*13.0)
	targetHbY := 713.0 + 13.0

	// Reach shortfalls (mm) from the target, listed from worst to best.
	shortfalls := []float64{103, 87, 74, 63, 50}
	var rows []model.CanonicalComparisonRow
	for _, short := range shortfalls {
		rows = append(rows, model.CanonicalComparisonRow{
			Bike: model.BikeGeometry{CanonicalStem: stem},
			Canonical: model.BikeFitAnalysis{
				DeadCenterBRP: brp,
				PossibleHBs: []model.HBPoint{
					{Stem: stem, HbX: targetHbX - short, HbY: targetHbY, SaddleHbMm: 700 - short, SaddleHbDropMm: 13},
				},
			},
		})
	}

	results := ScoreAllRows(rows, prof)
	for i := 1; i < len(results); i++ {
		if !(results[i].Total > results[i-1].Total) {
			t.Errorf("shortfall %.0fmm scored %.3f, not above shortfall %.0fmm at %.3f",
				shortfalls[i], results[i].Total, shortfalls[i-1], results[i-1].Total)
		}
		if results[i].Rank >= results[i-1].Rank {
			t.Errorf("shortfall %.0fmm ranked %d, not better than shortfall %.0fmm at %d",
				shortfalls[i], results[i].Rank, shortfalls[i-1], results[i-1].Rank)
		}
	}
}

func TestLossDeltaMatchesCell(t *testing.T) {
	prof := ideal.IdmatchProfile()
	specs := []ideal.AttributeSpec{}
	for _, a := range prof.Attributes {
		if a.Ideal == nil || a.ID == "handlebar_clamp_fit" {
			continue
		}
		specs = append(specs, a)
	}
	// Include saddle-translated BRP setback and BRP-HB specs, plus synthetic HTA/STA specs.
	berthoud, ok := catalog.GetSaddle("berthoud-aspin-open")
	if !ok {
		t.Fatal("missing berthoud-aspin-open saddle")
	}
	for _, a := range prof.Attributes {
		if a.ID == "brp_setback" || a.ID == "saddle_hb" {
			specs = append(specs, translateAttributeSpecForSaddle(a, berthoud))
		}
	}
	specs = append(specs,
		ideal.AttributeSpec{
			ID:         "hta",
			Weight:     2,
			Ideal:      ideal.Between(67.5, 68.5),
			Acceptable: ideal.Between(66.5, 69.5),
			EdgeScore:  new(75.0),
			Falloff:    ideal.FalloffBoth(2.0, 2.0),
			Floor:      new(-100.0),
		},
		ideal.AttributeSpec{
			ID:         "sta",
			Weight:     2,
			Ideal:      ideal.Between(73.0, 74.5),
			Acceptable: ideal.Between(72.0, 75.5),
			EdgeScore:  new(75.0),
			Falloff:    ideal.FalloffBoth(2.0, 2.0),
			Floor:      new(-100.0),
		},
	)

	numUnitRe := regexp.MustCompile(`(-?[0-9]+(?:\.[0-9]+)?)(?:mm|%|°)`)

	for _, spec := range specs {
		meta, ok := ideal.LookupAttribute(spec.ID)
		if !ok {
			t.Fatalf("unknown attribute %q", spec.ID)
		}

		// Probe points around lower and upper bounds including .5 and .05 ties.
		var testVals []float64
		if spec.Ideal.Min != nil {
			base := math.Floor(*spec.Ideal.Min) - 35
			for i := 0; i <= 360; i++ {
				testVals = append(testVals, math.Round((base+float64(i)*0.1)*1000)/1000)
			}
			if meta.Decimals > 0 {
				for i := -30; i <= 10; i++ {
					testVals = append(testVals, math.Round((*spec.Ideal.Min+float64(i)*0.05)*1000)/1000)
				}
			}
		}
		if spec.Ideal.Max != nil {
			base := math.Ceil(*spec.Ideal.Max)
			for i := 0; i <= 360; i++ {
				testVals = append(testVals, math.Round((base+float64(i)*0.1)*1000)/1000)
			}
			if meta.Decimals > 0 {
				for i := -10; i <= 30; i++ {
					testVals = append(testVals, math.Round((*spec.Ideal.Max+float64(i)*0.05)*1000)/1000)
				}
			}
		}

		for _, v := range testVals {
			ev := ideal.Evaluate(spec, meta, v, nil)
			fields := strings.Fields(ev.FormattedCell)
			if len(fields) != 4 {
				continue // ideal (no delta)
			}
			cellVal := fields[0]
			cellDelta := strings.TrimSuffix(fields[2], meta.Unit)

			// Test both printed DistanceToIdeal and unrounded raw distance.
			rawDist := 0.0
			if spec.Ideal.Min != nil && v < *spec.Ideal.Min {
				rawDist = v - *spec.Ideal.Min
			} else if spec.Ideal.Max != nil && v > *spec.Ideal.Max {
				rawDist = v - *spec.Ideal.Max
			}
			for _, distArg := range []float64{ev.DistanceToIdeal, rawDist} {
				desc := DescribeLossWithRow(spec, meta, v, distArg, ev.Zone, nil)
				matches := numUnitRe.FindAllStringSubmatch(desc, -1)
				if len(matches) < 2 {
					t.Fatalf("%s(%.3f): unexpected DescribeLossWithRow %q", spec.ID, v, desc)
				}
				lossVal := matches[0][1]
				lossDelta := matches[1][1]
				if lossVal != cellVal || lossDelta != cellDelta {
					t.Fatalf("%s(%.3f) dist=%.3f: cell=%q (val=%s delta=%s) vs loss=%q (val=%s delta=%s)",
						spec.ID, v, distArg, ev.FormattedCell, cellVal, cellDelta, desc, lossVal, lossDelta)
				}
			}
		}
	}

	// Verify ExplainProfile on the Pilot .5-tie cases (183.5 -> 184 & 35, 188.5 -> 188 & 31).
	buildTieRow := func(brpSetback float64) model.CanonicalComparisonRow {
		return model.CanonicalComparisonRow{
			Canonical: model.BikeFitAnalysis{
				DeadCenterBRP: model.BRPPoint{BrpSetbackMm: brpSetback},
				SelectedBRP:   &model.BRPPoint{BrpSetbackMm: brpSetback, RailOffsetMm: 15},
				CockpitFitResult: &model.CockpitFitResult{
					HasFit:        true,
					FinalSubScore: 100,
					CountInside:   4,
				},
				ActiveBbDrop:       70.0,
				WeightDistFrontPct: 37.0,
				TrailMm:            102.0,
			},
		}
	}
	rowElJefe := buildTieRow(183.5)
	rowVenn := buildTieRow(188.5)
	expl := ExplainProfile([]ScoredFrame{
		{Name: "El Jefe (XL)", Row: rowElJefe, Score: ScoreRow(rowElJefe, prof)},
		{Name: "Venn Ti (L)", Row: rowVenn, Score: ScoreRow(rowVenn, prof)},
	}, prof)
	joined := strings.Join(expl.Losses, "\n")
	if !strings.Contains(joined, "184mm is 35mm forward of the 219mm ideal") {
		t.Errorf("El Jefe loss text missing expected 184/35: %s", joined)
	}
	if !strings.Contains(joined, "188mm is 31mm forward of the 219mm ideal") {
		t.Errorf("Venn Ti loss text missing expected 188/31: %s", joined)
	}
}

func TestScoreRow_DisplayOnlyWithTargetCarriesZone(t *testing.T) {
	prof := ideal.IdmatchProfile()
	row := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: model.BRPPoint{BrpSetbackMm: 219},
			PossibleHBs: []model.HBPoint{
				{
					Stem:           model.StemSpec{LengthMm: 90, AngleDeg: -6},
					SaddleHbMm:     685.0, // acceptable for [695,705] / [685,715]
					SaddleHbDropMm: 2.0,   // ideal for [-2,8]
				},
			},
			ActiveReach:        465,
			ActiveStack:        630,
			ActiveBbDrop:       70,
			WeightDistFrontPct: 37.0,
			TrailMm:            102,
		},
	}
	res := ScoreRow(row, prof)
	byID := map[string]CriterionResult{}
	for _, c := range res.Criteria {
		byID[c.ID] = c
	}
	if c := byID["saddle_hb"]; !c.IsDisplayOnly || c.Zone != string(ideal.ZoneAcceptable) {
		t.Errorf("saddle_hb = %+v, want IsDisplayOnly=true Zone=acceptable", c)
	}
	if c := byID["saddle_hb_drop"]; !c.IsDisplayOnly || c.Zone != string(ideal.ZoneIdeal) {
		t.Errorf("saddle_hb_drop = %+v, want IsDisplayOnly=true Zone=ideal", c)
	}
	if c := byID["reach"]; !c.IsDisplayOnly || c.Zone != "" {
		t.Errorf("reach = %+v, want IsDisplayOnly=true Zone=\"\"", c)
	}
}

func TestLossAndHeadToHeadUseRegistryLabel(t *testing.T) {
	prof := ideal.IdmatchProfile()
	meta, ok := ideal.LookupAttribute("handlebar_clamp_fit")
	if !ok {
		t.Fatal("missing handlebar_clamp_fit meta")
	}

	makeRow := func(name string, bestR float64, count int) ScoredFrame {
		row := model.CanonicalComparisonRow{
			Canonical: model.BikeFitAnalysis{
				DeadCenterBRP: model.BRPPoint{BrpSetbackMm: 219},
				SelectedHB:    &model.HBPoint{Stem: model.StemSpec{LengthMm: 90, AngleDeg: -6}, SaddleHbMm: 690, SaddleHbDropMm: 3},
				CockpitFitResult: &model.CockpitFitResult{
					TargetOval:    model.FitOval{CenterX: 500, CenterY: 700},
					SelectedHB:    model.HBPoint{Stem: model.StemSpec{LengthMm: 90, AngleDeg: -6}, HbX: 480, HbY: 700},
					BestR:         bestR,
					CountInside:   count,
					FinalSubScore: 50 - bestR*10,
				},
				ActiveReach:        460,
				ActiveStack:        620,
				ActiveBbDrop:       70,
				WeightDistFrontPct: 37,
				TrailMm:            102,
			},
		}
		return ScoredFrame{Name: name, Row: row, Score: ScoreRow(row, prof)}
	}

	fa := makeRow("Frame A", 1.2, 0)
	fb := makeRow("Frame B", 3.5, 0)
	expl := ExplainProfile([]ScoredFrame{fa, fb}, prof)

	lossesJoined := strings.Join(expl.Losses, "\n")
	if strings.Contains(lossesJoined, "Handlebar Clamp Fit") {
		t.Errorf("losses still use stale name 'Handlebar Clamp Fit': %s", lossesJoined)
	}
	if !strings.Contains(lossesJoined, meta.Label+":") {
		t.Errorf("losses missing registry label %q: %s", meta.Label, lossesJoined)
	}

	h2hJoined := strings.Join(expl.HeadToHead, "\n")
	if !strings.Contains(h2hJoined, meta.Label+" sub-score ") {
		t.Errorf("head-to-head should label Oval Fits values as sub-score: %s", h2hJoined)
	}
}

func TestBrpSetbackExtractMatchesSelectedBRP(t *testing.T) {
	meta, ok := ideal.LookupAttribute("brp_setback")
	if !ok {
		t.Fatal("missing brp_setback attribute")
	}
	row := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: model.BRPPoint{BrpSetbackMm: 185.0},
			SelectedBRP:   &model.BRPPoint{BrpSetbackMm: 200.0, RailOffsetMm: 15.0},
		},
	}
	val, valid := meta.Extract(row)
	if !valid || val != 200.0 {
		t.Errorf("brp_setback Extract = (%.1f, %v), want (200.0, true)", val, valid)
	}
}

func TestExplainProfile_HeadToHeadReconcilesAllFactors(t *testing.T) {
	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		t.Fatalf("GetProfile(idmatch): %v", err)
	}

	// Build two frames differing in 3 positive factors (+5, +4, +2), 2 negative
	// factors (-3, -2), and a <1 pt factor (+0.4): total gap = 65 - 59 = 6.
	makeCrit := func(id, label, unit string, val, pts float64) CriterionResult {
		return CriterionResult{ID: id, Label: label, Unit: unit, Value: val, Points: pts, Weight: 1}
	}
	fa := ScoredFrame{
		Name: "Frame A (L)",
		Score: Result{
			Total: 65.4,
			Criteria: []CriterionResult{
				makeCrit("brp_setback", "BRP Setback", "mm", 215, 25),
				makeCrit("handlebar_clamp_fit", "Oval Fits", "", 80, 34),
				makeCrit("seatpost_extension", "Seatpost Ext", "mm", 270, 12),
				makeCrit("frontwt", "Front Wt", "%", 35.0, 5),
				makeCrit("bbdrop", "BB Drop", "mm", 62, 5),
				makeCrit("trail", "Trail", "mm", 102, 10.4),
			},
		},
	}
	fb := ScoredFrame{
		Name: "Frame B (L)",
		Score: Result{
			Total: 59.0,
			Criteria: []CriterionResult{
				makeCrit("brp_setback", "BRP Setback", "mm", 205, 20),
				makeCrit("handlebar_clamp_fit", "Oval Fits", "", 65, 30),
				makeCrit("seatpost_extension", "Seatpost Ext", "mm", 250, 10),
				makeCrit("frontwt", "Front Wt", "%", 37.0, 8),
				makeCrit("bbdrop", "BB Drop", "mm", 70, 7),
				makeCrit("trail", "Trail", "mm", 101, 10.0),
			},
		},
	}

	expl := ExplainProfile([]ScoredFrame{fa, fb}, prof)
	if len(expl.HeadToHead) != 1 {
		t.Fatalf("HeadToHead count = %d, want 1", len(expl.HeadToHead))
	}
	line := expl.HeadToHead[0]
	for _, want := range []string{
		"BRP Setback 215mm vs 205mm (+5)",
		"Oval Fits sub-score 80 vs 65 (+4)",
		"Seatpost Ext 270mm vs 250mm (+2)",
		"Front Wt 35.0% vs 37.0% (-3)",
		"BB Drop 62mm vs 70mm (-2)",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("H2H line missing %q:\n%s", want, line)
		}
	}
}
