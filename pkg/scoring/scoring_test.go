package scoring

import (
	"math"
	"strings"
	"testing"

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
	prof, err := ideal.LoadProfile("", "idmatch")
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

	// 2. Zero setups inside oval (steep roll-off + -25 penalty)
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
	// BaseScore = 75 - 1200*(0.1)^2 = 63.0. Final = 63.0 - 25.0 = 38.0
	if math.Abs(resZero.Total-38.0) > 0.5 {
		t.Errorf("Expected score ~38.0 for R=1.10 with zero-fit penalty, got %.1f", resZero.Total)
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
	expected := "Handlebar Clamp Fit: closest stem is 100mm -6° +0mm (R=0.97, reach: 11mm short, stack: 2mm low, 1 valid setup in fit window, -15 stem penalty)"
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
				BestR:          5.37,
				CountInside:    0,
				ZeroFitPenalty: 25.0,
				StemPenalty:    15.0,
			},
		},
	}

	desc := DescribeLossWithRow(spec, meta, -200, 5.37, ideal.ZoneOutside, &row)
	expected := "Handlebar Clamp Fit: 0 valid setups in fit window (closest is 100mm -6° +0mm, R=5.37, reach: 26mm short, stack: 56mm high; -25 penalty, -15 stem penalty)"
	if desc != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, desc)
	}
}
