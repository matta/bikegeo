package scoring

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"bikegeo/internal/canonicalizer"
	"bikegeo/internal/catalog"
	"bikegeo/internal/geometry"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
)

func scoreRowWithTargets(row model.CanonicalComparisonRow, prof *ideal.Profile) Result {
	rows := []model.CanonicalComparisonRow{row}
	if err := canonicalizer.ApplyIdealCockpitTargets(rows, prof); err != nil {
		panic(err)
	}
	return ScoreRow(rows[0], prof)
}

func scoreAllRowsWithTargets(rows []model.CanonicalComparisonRow, prof *ideal.Profile) []Result {
	if err := canonicalizer.ApplyIdealCockpitTargets(rows, prof); err != nil {
		panic(err)
	}
	return ScoreAllRows(rows, prof)
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

func TestScoreRow_MissingDataRenormalization(t *testing.T) {
	prof := &ideal.Profile{
		Name: "test-missing",
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "setback",
				Cost:  ideal.CostEach(1),
				Ideal: &ideal.Range{Min: model.Some(230.0), Max: model.Some(230.0)},
			},
			{
				ID:    "standover",
				Cost:  ideal.CostEach(1),
				Ideal: &ideal.Range{Min: model.Some(700.0), Max: model.Some(750.0)},
			},
		},
	}

	// Frame A has both setback=230 and standover=720 inside their free ranges -> total 100
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

	// Frame B has setback=230 (free) and standover=0 (missing)
	// A missing band loses no points -> total 100
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
		t.Errorf("Frame B expected total 100 (missing band loses nothing), got %.1f", resB.Total)
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
				HasFit:      true,
				CountInside: 4,
			},
			ActiveBbDrop: 70.0,
			TrailMm:      102.0,
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
				HasFit:      true,
				CountInside: 4,
			},
			ActiveBbDrop: 70.0,
			TrailMm:      102.0,
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
		Name:      "test-saddle-hb",
		TargetRef: model.TargetHandGrip,
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "saddle_hb",
				Cost:  ideal.CostEach(1),
				Ideal: &ideal.Range{Min: model.Some(695.0), Max: model.Some(705.0)},
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
			SelectedHB:  &possibleHBs[1], // the on-target build
			SaddleHbMm:  possibleHBs[1].SaddleHbMm,
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
	// Ideal profile with BRP Setback (target 219mm) and Oval Fits on a 700mm / 13mm cockpit target
	prof := &ideal.Profile{
		Name:                "test-idmatch",
		SaddleHeightMm:      model.Some(746.0),
		TargetRef:           model.TargetHandGrip,
		CockpitTargetDistMm: 700,
		CockpitTargetRiseMm: 13,
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "brp_setback",
				Cost:  ideal.CostEach(1),
				Ideal: &ideal.Range{Min: model.Some(219.0), Max: model.Some(219.0)}, // target 219mm
			},
			{
				ID:    "handlebar_clamp_fit",
				Cost:  &ideal.Cost{Above: ideal.OvalCostPerR},
				Ideal: ideal.AtLeast(1),
			},
			ideal.DisplayOnly("saddle_hb"),
			ideal.DisplayOnly("saddle_hb_drop"),
		},
	}

	// Synthetic frame whose 71.5° STA can reach the report target at 746mm height.
	staDeg := model.Deg(71.5)
	seatHeight := 746.0
	saddle := model.ReferenceIdmatchSaddle()
	deadCenterBRP, placementErr := saddle.CalculatePoint(staDeg, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	if placementErr != nil {
		t.Fatal(placementErr)
	}
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
			Handlebar:     catalog.DefaultHandlebar(),
			DeadCenterBRP: deadCenterBRP,
			PossibleBRPs:  possibleBRPs,
		},
	}

	res := scoreRowWithTargets(row, prof)
	if res.Total < 90.0 {
		t.Errorf("Expected score > 90 for 71.5° frame (ideal setback match), got %.1f", res.Total)
	}

	// Verify criteria results for 71.5° frame
	for _, c := range res.Criteria {
		if c.ID == "brp_setback" {
			if c.FormattedCell != "219 ✓" {
				t.Errorf("Expected brp_setback cell '219 ✓', got %q", c.FormattedCell)
			}
		}
		if c.ID == "handlebar_clamp_fit" {
			if c.PointsLost > 2.0 {
				t.Errorf("Expected the oval within reach (<= 2 points lost), got %.1f (%q)", c.PointsLost, c.FormattedCell)
			}
		}
	}

	// Verify that steep STA = 74.53° (Otso Fenrir) is properly penalized without a Drop Best
	fenrirSTA := model.Deg(74.53)
	fenrirDeadCenter, placementErr := saddle.CalculatePoint(fenrirSTA, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	if placementErr != nil {
		t.Fatal(placementErr)
	}
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
			Handlebar:     catalog.DefaultHandlebar(),
			DeadCenterBRP: fenrirDeadCenter,
			PossibleBRPs:  fenrirPossible,
		},
	}
	fenrirRes := scoreRowWithTargets(fenrirRow, prof)
	if fenrirRes.Total > 50.0 {
		t.Errorf("Expected score < 50 for Otso Fenrir on the zero-setback dropper (cannot reach 219mm), got %.1f", fenrirRes.Total)
	}
}

func TestScoreRow_SeatpostExtension(t *testing.T) {
	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		t.Fatalf("Failed to load idmatch profile: %v", err)
	}

	buildRow := func(seatTubeLength float64) model.CanonicalComparisonRow {
		seatHeight := 746.0
		ext := 0.0
		if seatTubeLength > 0 {
			ext = seatHeight - seatTubeLength
		}
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
	if critIdeal.PointsLost != 0 {
		t.Errorf("Expected 0 points lost for 280mm extension, got %.1f", critIdeal.PointsLost)
	}
	if critIdeal.Zone != string(ideal.ZoneIdeal) {
		t.Errorf("Expected ZoneIdeal for 280mm extension, got %s", critIdeal.Zone)
	}

	// 2. Acceptable low extension: seat tube 526mm -> extension = 220mm (within acceptable [210, 340], ideal min 250)
	// 30mm below the 250mm free bound at 0.18 points/mm -> 5.4 points lost
	rowAcc := buildRow(526.0)
	resAcc := ScoreRow(rowAcc, prof)
	var critAcc *CriterionResult
	for i := range resAcc.Criteria {
		if resAcc.Criteria[i].ID == "seatpost_extension" {
			critAcc = &resAcc.Criteria[i]
			break
		}
	}
	if critAcc == nil || math.Abs(critAcc.PointsLost-5.4) > 0.05 {
		t.Errorf("Expected ~5.4 points lost for 220mm extension, got %+v", critAcc)
	}
	if critAcc.Zone != string(ideal.ZoneOutside) {
		t.Errorf("Expected ZoneOutside (not free) for 220mm extension, got %s", critAcc.Zone)
	}

	// 3. Severely short extension: Fargo XL (seat tube 558.8mm) -> extension = 187.2mm
	// 62.8mm below the 250mm free bound at 0.18 points/mm -> 11.3 points lost
	rowFargoXL := buildRow(558.8)
	resFargoXL := ScoreRow(rowFargoXL, prof)
	var critFargoXL *CriterionResult
	for i := range resFargoXL.Criteria {
		if resFargoXL.Criteria[i].ID == "seatpost_extension" {
			critFargoXL = &resFargoXL.Criteria[i]
			break
		}
	}
	if critFargoXL == nil || math.Abs(critFargoXL.PointsLost-11.3) > 0.05 {
		t.Errorf("Expected ~11.3 points lost for Fargo XL 187.2mm extension, got %+v", critFargoXL)
	}
	if critFargoXL.Zone != string(ideal.ZoneOutside) {
		t.Errorf("Expected ZoneOutside for Fargo XL 187.2mm extension, got %s", critFargoXL.Zone)
	}
}

func TestScoreRow_HandlebarClampFit(t *testing.T) {
	// Profile with handlebar_clamp_fit, saddle_hb target (700mm), and saddle_hb_drop target (13mm)
	prof := &ideal.Profile{
		CockpitTargetDistMm: 700,
		CockpitTargetRiseMm: 13,
		Name:                "test-clamp-fit",
		SaddleHeightMm:      model.Some(746.0),
		TargetRef:           model.TargetHandGrip,
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "handlebar_clamp_fit",
				Cost:  &ideal.Cost{Above: ideal.OvalCostPerR},
				Ideal: ideal.AtLeast(100),
			},
			ideal.DisplayOnly("saddle_hb"),
			ideal.DisplayOnly("saddle_hb_drop"),
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

	resMulti := scoreRowWithTargets(rowMulti, prof)
	if resMulti.Total != 100.0 {
		t.Errorf("Expected total score 100 for exact center (free inside the oval), got %.1f", resMulti.Total)
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
	resZero := scoreRowWithTargets(rowZero, prof)
	// R=1.10 is 0.10 past the oval edge at 22 points per unit R -> 2.2 points lost.
	if math.Abs(resZero.Total-97.8) > 0.05 {
		t.Errorf("Expected score ~97.8 for R=1.10, got %.1f", resZero.Total)
	}
}

func TestScoreRow_HandlebarClampFit_StemExtremityPenalty(t *testing.T) {
	prof := &ideal.Profile{
		CockpitTargetDistMm: 700,
		CockpitTargetRiseMm: 13,
		Name:                "test-stem-extreme",
		SaddleHeightMm:      model.Some(746.0),
		TargetRef:           model.TargetHandGrip,
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "handlebar_clamp_fit",
				Cost:  &ideal.Cost{Above: ideal.OvalCostPerR},
				Ideal: ideal.AtLeast(100),
			},
			ideal.DisplayOnly("saddle_hb"),
			ideal.DisplayOnly("saddle_hb_drop"),
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

	ovalCost := func(_ model.HBPoint, r float64) float64 {
		return ideal.OvalPointsLost(&model.CockpitFitResult{BestR: r}, nil)
	}
	geometry.ApplyCockpitFitByCost(&row100.Canonical, 700.0, 13.0, geometry.TargetHandGrip, model.Opt[model.HandlebarSpec]{}, ovalCost)

	res100 := ScoreRow(row100, prof)
	// R = 0 (free inside the oval). A 100mm stem on a 60-100mm frame costs nothing under Oval Fits.
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
	if critClamp.PointsLost != 0 {
		t.Errorf("Expected 0 points lost inside the oval, got %.2f", critClamp.PointsLost)
	}

	// Verify loss explanation omits the reach/stack offset when R=0.00
	meta, _ := ideal.LookupAttribute("handlebar_clamp_fit")
	spec := prof.Attributes[0]
	lossDesc := DescribeLossWithRow(spec, meta, critClamp.Value, critClamp.DistanceToIdeal, ideal.ZoneIdeal, &row100)
	if strings.Contains(lossDesc, "stem penalty") {
		t.Errorf("Expected loss explanation not to mention the unpriced stem penalty, got: %s", lossDesc)
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
			},
		},
	}

	desc := DescribeLossWithRow(spec, meta, 85, 0.97, ideal.ZoneIdeal, &row)
	expected := "Oval Fits: closest stem is 100mm -6° +0mm (R=0.97, reach: 11mm short, stack: 2mm low, 1 valid setup in fit window)"
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
			},
		},
	}

	desc := DescribeLossWithRow(spec, meta, -200, 5.37, ideal.ZoneOutside, &row)
	expected := "Oval Fits: 0 valid setups in fit window (closest is 100mm -6° +0mm, R=5.37, reach: 26mm short, stack: 56mm high)"
	if desc != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, desc)
	}
}

// Regression: three setups identical except for how far short the bars fall must
// rank by that distance, however far outside the fit oval they all are. A hard
// floor on the cockpit curve once made them tie, letting minor criteria decide.
func TestScoreAllRows_FarOutsideCockpitStillRanksByDistance(t *testing.T) {
	prof := &ideal.Profile{
		CockpitTargetDistMm: 700,
		CockpitTargetRiseMm: 13,
		Name:                "test-far-cockpit",
		SaddleHeightMm:      model.Some(746.0),
		TargetRef:           model.TargetHandGrip,
		Attributes: []ideal.AttributeSpec{
			{
				ID:    "handlebar_clamp_fit",
				Cost:  &ideal.Cost{Above: ideal.OvalCostPerR},
				Ideal: ideal.AtLeast(100),
			},
			ideal.DisplayOnly("saddle_hb"),
			ideal.DisplayOnly("saddle_hb_drop"),
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

	results := scoreAllRowsWithTargets(rows, prof)
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
	// Include the saddle-translated BRP setback and a display-only BRP-HB spec, plus synthetic HTA/STA specs.
	berthoud, ok := catalog.GetSaddle("berthoud-aspin-open")
	if !ok {
		t.Fatal("missing berthoud-aspin-open saddle")
	}
	for _, a := range prof.ForSaddle(berthoud).Attributes {
		if a.ID == "brp_setback" {
			specs = append(specs, a)
		}
	}
	// A display-only band with a free range, centred on the Berthoud cockpit target.
	specs = append(specs, ideal.AttributeSpec{ID: "saddle_hb", Ideal: ideal.Between(679.9, 689.9)})
	specs = append(specs,
		ideal.AttributeSpec{
			ID:    "hta",
			Cost:  ideal.CostEach(1),
			Ideal: ideal.Between(67.5, 68.5),
		},
		ideal.AttributeSpec{
			ID:    "sta",
			Cost:  ideal.CostEach(1),
			Ideal: ideal.Between(73.0, 74.5),
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
		if minVal, ok := spec.Ideal.Min.Get(); ok {
			base := math.Floor(minVal) - 35
			for i := 0; i <= 360; i++ {
				testVals = append(testVals, math.Round((base+float64(i)*0.1)*1000)/1000)
			}
			if meta.Decimals > 0 {
				for i := -30; i <= 10; i++ {
					testVals = append(testVals, math.Round((minVal+float64(i)*0.05)*1000)/1000)
				}
			}
		}
		if maxVal, ok := spec.Ideal.Max.Get(); ok {
			base := math.Ceil(maxVal)
			for i := 0; i <= 360; i++ {
				testVals = append(testVals, math.Round((base+float64(i)*0.1)*1000)/1000)
			}
			if meta.Decimals > 0 {
				for i := -10; i <= 30; i++ {
					testVals = append(testVals, math.Round((maxVal+float64(i)*0.05)*1000)/1000)
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
			if minVal, ok := spec.Ideal.Min.Get(); ok && v < minVal {
				rawDist = v - minVal
			} else if maxVal, ok := spec.Ideal.Max.Get(); ok && v > maxVal {
				rawDist = v - maxVal
			}
			for _, distArg := range []float64{ev.DistanceToIdeal, rawDist} {
				desc := DescribeLossWithRow(spec, meta, v, distArg, ev.Zone, nil)
				matches := numUnitRe.FindAllStringSubmatch(desc, -1)
				if len(matches) < 2 {
					t.Fatalf("%s(%.3f): unexpected DescribeLossWithRow %q", spec.ID, v, desc)
				}
				lossVal := matches[0][1]
				lossDelta := matches[1][1]
				if spec.IsScored() && spec.LimitPoints == 0 {
					// A scored cell shows points lost; the sentence still names the distance.
					if lossVal != cellVal {
						t.Fatalf("%s(%.3f): cell=%q vs loss=%q", spec.ID, v, ev.FormattedCell, desc)
					}
					continue
				}
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
					HasFit:      true,
					CountInside: 4,
				},
				ActiveBbDrop: 70.0,
				TrailMm:      102.0,
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

func TestScoreRow_DisplayOnlyBandsCarryNoZone(t *testing.T) {
	prof := ideal.IdmatchProfile()
	row := model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			DeadCenterBRP: model.BRPPoint{BrpSetbackMm: 219},
			PossibleHBs: []model.HBPoint{
				{
					Stem:           model.StemSpec{LengthMm: 90, AngleDeg: -6},
					SaddleHbMm:     685.0, // outside the free [695,705]
					SaddleHbDropMm: 2.0,   // ideal for [-2,8]
				},
			},
			ActiveReach:  465,
			ActiveStack:  630,
			ActiveBbDrop: 70,
			TrailMm:      102,
		},
	}
	res := scoreRowWithTargets(row, prof)
	byID := map[ideal.MetricID]CriterionResult{}
	for _, c := range res.Criteria {
		byID[c.ID] = c
	}
	// The cockpit target is explicit on the profile; saddle_hb and saddle_hb_drop carry no range.
	if c := byID["saddle_hb"]; !c.IsDisplayOnly || c.Zone != "" {
		t.Errorf("saddle_hb = %+v, want IsDisplayOnly=true Zone=\"\"", c)
	}
	if c := byID["saddle_hb_drop"]; !c.IsDisplayOnly || c.Zone != "" {
		t.Errorf("saddle_hb_drop = %+v, want IsDisplayOnly=true Zone=\"\"", c)
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
					TargetOval:  model.FitOval{CenterX: 500, CenterY: 700},
					SelectedHB:  model.HBPoint{Stem: model.StemSpec{LengthMm: 90, AngleDeg: -6}, HbX: 480, HbY: 700},
					BestR:       bestR,
					CountInside: count,
				},
				ActiveReach:  460,
				ActiveStack:  620,
				ActiveBbDrop: 70,
				TrailMm:      102,
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
	if !strings.Contains(h2hJoined, meta.Label+" R=1.20 vs R=3.50") {
		t.Errorf("head-to-head should compare Oval Fits by oval radius R: %s", h2hJoined)
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

	// Build two frames differing in 3 positive factors (+5, +4, +2), 1 negative
	// factor (-2), and a <1 pt factor (+0.4): total gap = 65 - 59 = 6.
	// Each band's points lost is 40 minus its listed points, so A gains
	// (B lost − A lost) = (A pts − B pts) on it.
	makeCrit := func(id ideal.MetricID, label, unit string, val, pts float64) CriterionResult {
		return CriterionResult{ID: id, Label: label, Unit: unit, Value: val, PointsLost: 40 - pts}
	}
	withR := func(c CriterionResult, r float64) CriterionResult {
		c.DistanceToIdeal = r
		return c
	}
	fa := ScoredFrame{
		Name: "Frame A (L)",
		Score: Result{
			Total: 65.4,
			Criteria: []CriterionResult{
				makeCrit("brp_setback", "BRP Setback", "mm", 215, 25),
				withR(makeCrit("handlebar_clamp_fit", "Oval Fits", "", 80, 34), 1.2),
				makeCrit("seatpost_extension", "Seatpost Ext", "mm", 270, 12),
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
				withR(makeCrit("handlebar_clamp_fit", "Oval Fits", "", 65, 30), 1.5),
				makeCrit("seatpost_extension", "Seatpost Ext", "mm", 250, 10),
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
		"Oval Fits R=1.20 vs R=1.50 (+4)",
		"Seatpost Ext 270mm vs 250mm (+2)",
		"BB Drop 62mm vs 70mm (-2)",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("H2H line missing %q:\n%s", want, line)
		}
	}
}

func TestScoreRow_CockpitTargetRefModes(t *testing.T) {
	bar := catalog.DefaultHandlebar()
	barOff, err := geometry.GetHandlebarOffset(bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	flatBar := model.HandlebarSpec{
		ID:             "flat-zero-bar",
		Name:           "Flat Zero",
		StockWidthMm:   700,
		CurrentWidthMm: 700,
	}

	brp := model.BRPPoint{
		BrpX:     -219.0,
		BrpY:     713.0,
		SitBoneX: -219.0,
		SitBoneY: 713.0,
	}
	targetDist := 700.0
	targetDrop := 13.0
	targetY := brp.SitBoneY + targetDrop
	targetX := brp.BrpX + math.Sqrt(targetDist*targetDist-math.Pow(targetY-brp.BrpY, 2))

	// candA places HandGrip dead-center at (targetX, targetY).
	candA := model.HBPoint{
		Stem:           model.StemSpec{LengthMm: 60, AngleDeg: 0},
		ClampX:         targetX - barOff.X,
		ClampY:         targetY - barOff.Y,
		HbX:            targetX,
		HbY:            targetY,
		SaddleHbMm:     targetDist,
		SaddleHbDropMm: targetDrop,
	}
	// candB places StemClamp dead-center at (targetX, targetY), so its physical HandGrip
	// sits offset by barOff and has distinct SaddleHbMm / SaddleHbDropMm.
	gripBX := targetX + barOff.X
	gripBY := targetY + barOff.Y
	candB := model.HBPoint{
		Stem:           model.StemSpec{LengthMm: 80, AngleDeg: 0},
		ClampX:         targetX,
		ClampY:         targetY,
		HbX:            gripBX,
		HbY:            gripBY,
		SaddleHbMm:     math.Hypot(gripBX-brp.BrpX, gripBY-brp.BrpY),
		SaddleHbDropMm: gripBY - brp.SitBoneY,
	}

	makeRow := func() model.CanonicalComparisonRow {
		return model.CanonicalComparisonRow{
			Bike: model.BikeGeometry{
				CanonicalStem: model.StemSpec{LengthMm: 70, AngleDeg: 0},
			},
			Canonical: model.BikeFitAnalysis{
				DeadCenterBRP: brp,
				PossibleHBs:   []model.HBPoint{candA, candB},
			},
		}
	}

	makeProf := func(ref model.TargetRef, refBar model.Opt[model.HandlebarSpec]) *ideal.Profile {
		return &ideal.Profile{
			Name:           "test-target-ref",
			SaddleHeightMm: model.Some(746.0),
			TargetRef:      ref,
			ReferenceBar:   refBar,

			CockpitTargetDistMm: targetDist,
			CockpitTargetRiseMm: targetDrop,
			Attributes: []ideal.AttributeSpec{
				{
					ID:    "handlebar_clamp_fit",
					Cost:  &ideal.Cost{Above: ideal.OvalCostPerR},
					Ideal: &ideal.Range{Min: model.Some(1.0)},
				},
				ideal.DisplayOnly("saddle_hb"),
				ideal.DisplayOnly("saddle_hb_drop"),
				ideal.DisplayOnly("best_stem"),
			},
		}
	}

	bestStemLen := func(res Result) float64 {
		for _, c := range res.Criteria {
			if c.ID == "best_stem" {
				return c.Value
			}
		}
		return 0
	}

	// 1. TargetHandGrip selects candA (60mm).
	resHand := scoreRowWithTargets(makeRow(), makeProf(model.TargetHandGrip, model.Opt[model.HandlebarSpec]{}))
	if got := bestStemLen(resHand); got != 60 {
		t.Errorf("TargetHandGrip selected stem = %.0fmm, want 60mm", got)
	}
	if math.Round(resHand.Total) != 100 {
		t.Errorf("TargetHandGrip total = %.2f, want ~100", resHand.Total)
	}

	// 2. TargetStemClamp selects candB (80mm).
	profClamp := makeProf(model.TargetStemClamp, model.Opt[model.HandlebarSpec]{})
	resClamp := scoreRowWithTargets(makeRow(), profClamp)
	if got := bestStemLen(resClamp); got != 80 {
		t.Errorf("TargetStemClamp selected stem = %.0fmm, want 80mm", got)
	}
	if math.Round(resClamp.Total) != 100 {
		t.Errorf("TargetStemClamp total = %.2f, want ~100", resClamp.Total)
	}

	// Verify ApplyCockpitFitByCost preserves physical HandGrip SaddleHbMm/DropMm on SelectedHB
	// and DescribeLossWithRow measures offset from StemClamp rather than HandGrip.
	rowClamp := makeRow()
	ovalCost := func(_ model.HBPoint, r float64) float64 {
		return ideal.OvalPointsLost(&model.CockpitFitResult{BestR: r}, nil)
	}
	geometry.ApplyCockpitFitByCost(&rowClamp.Canonical, targetDist, targetDrop,
		model.TargetStemClamp, model.Opt[model.HandlebarSpec]{}, ovalCost)
	if rowClamp.Canonical.CockpitFitResult == nil || rowClamp.Canonical.CockpitFitResult.TargetRef != model.TargetStemClamp {
		t.Fatalf("ApplyCockpitFitByCost TargetRef = %+v, want TargetStemClamp", rowClamp.Canonical.CockpitFitResult)
	}
	if math.Abs(rowClamp.Canonical.SelectedHB.SaddleHbMm-candB.SaddleHbMm) > 1e-9 {
		t.Errorf("SelectedHB.SaddleHbMm = %.3f, want physical HandGrip %.3f", rowClamp.Canonical.SelectedHB.SaddleHbMm, candB.SaddleHbMm)
	}
	shiftedOval := rowClamp.Canonical.CockpitFitResult.TargetOval
	shiftedOval.CenterX += 25.0
	fitOut := geometry.EvaluateCockpitFitByCost([]model.HBPoint{candB}, shiftedOval,
		model.TargetStemClamp, model.Opt[model.HandlebarSpec]{}, ovalCost)
	rowOut := makeRow()
	rowOut.Canonical.CockpitFitResult = &fitOut
	rowOut.Canonical.SelectedHB = &fitOut.SelectedHB
	lossMsg := DescribeLossWithRow(profClamp.Attributes[0], ideal.AttributeMeta{Label: "Oval Fits"}, float64(fitOut.CountInside), 0, ideal.ZoneOutside, &rowOut)
	if !strings.Contains(lossMsg, "reach: 25mm short, stack: 0mm") {
		t.Errorf("DescribeLossWithRow TargetStemClamp offset = %q, want StemClamp delta (25mm short, stack: 0mm)", lossMsg)
	}

	// 3. TargetReferenceBarClamp with flat zero-offset bar matches TargetHandGrip.
	resRefFlat := scoreRowWithTargets(makeRow(), makeProf(model.TargetReferenceBarClamp, model.Some(flatBar)))
	if got := bestStemLen(resRefFlat); got != 60 {
		t.Errorf("TargetReferenceBarClamp(flatBar) selected stem = %.0fmm, want 60mm", got)
	}
	if math.Abs(resRefFlat.Total-resHand.Total) > 1e-6 {
		t.Errorf("TargetReferenceBarClamp(flatBar) total = %.6f, want %.6f (matching TargetHandGrip)", resRefFlat.Total, resHand.Total)
	}

	// 4. TargetReferenceBarClamp with ReferenceBar == Handlebar matches TargetStemClamp.
	resRefSame := scoreRowWithTargets(makeRow(), makeProf(model.TargetReferenceBarClamp, model.Some(bar)))
	if got := bestStemLen(resRefSame); got != 80 {
		t.Errorf("TargetReferenceBarClamp(sameBar) selected stem = %.0fmm, want 80mm", got)
	}
	if math.Abs(resRefSame.Total-resClamp.Total) > 1e-6 {
		t.Errorf("TargetReferenceBarClamp(sameBar) total = %.6f, want %.6f (matching TargetStemClamp)", resRefSame.Total, resClamp.Total)
	}

}

func gateRow(brp float64, countInside int, bestR float64) model.CanonicalComparisonRow {
	return model.CanonicalComparisonRow{
		Canonical: model.BikeFitAnalysis{
			SelectedBRP:      &model.BRPPoint{BrpSetbackMm: brp},
			CockpitFitResult: &model.CockpitFitResult{CountInside: countInside, BestR: bestR},
		},
	}
}

func TestFits_Gate(t *testing.T) {
	prof := ideal.IdmatchProfile()
	for _, tc := range []struct {
		name string
		row  model.CanonicalComparisonRow
		want bool
	}{
		{"hands in the oval and BRP on target", gateRow(219, 3, 0.4), true},
		{"hands inside via the selected build only", gateRow(219, 0, 1.0), true},
		{"BRP exactly 10mm short is still fit", gateRow(209, 1, 0.9), true},
		{"BRP exactly 10mm long is still fit", gateRow(229, 1, 0.9), true},
		{"BRP 11mm short is out", gateRow(208, 1, 0.9), false},
		{"BRP 11mm long is out", gateRow(230, 1, 0.9), false},
		{"no build puts the hands inside the oval", gateRow(219, 0, 1.2), false},
		{"no cockpit result", model.CanonicalComparisonRow{Canonical: model.BikeFitAnalysis{SelectedBRP: &model.BRPPoint{BrpSetbackMm: 219}}}, false},
		{"no selected BRP", model.CanonicalComparisonRow{Canonical: model.BikeFitAnalysis{CockpitFitResult: &model.CockpitFitResult{CountInside: 2, BestR: 0.5}}}, false},
	} {
		if got := Fits(tc.row, prof); got != tc.want {
			t.Errorf("%s: Fits = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScoreAllRows_FittingSetupsRankAboveHigherScoringMisses(t *testing.T) {
	prof := ideal.IdmatchProfile()
	// The first row misses the gate but is the cheaper build on points; the second fits.
	miss := gateRow(219, 0, 1.05)
	miss.Canonical.ActiveBbDrop, miss.Canonical.TrailMm = 70, 102
	fit := gateRow(212, 2, 0.8)
	fit.Canonical.ActiveBbDrop, fit.Canonical.TrailMm = 80, 130
	res := ScoreAllRows([]model.CanonicalComparisonRow{miss, fit}, prof)
	if !(res[0].Total > res[1].Total) {
		t.Fatalf("fixture must have the miss outscoring the fit: %.1f vs %.1f", res[0].Total, res[1].Total)
	}
	if res[0].Fits || !res[1].Fits {
		t.Fatalf("Fits = %v/%v, want false/true", res[0].Fits, res[1].Fits)
	}
	if res[1].Rank != 1 || res[0].Rank != 2 {
		t.Errorf("ranks = %d/%d, want the fitting setup #1 and the miss #2", res[1].Rank, res[0].Rank)
	}
	if !Better(res[1], res[0]) || Better(res[0], res[1]) {
		t.Error("Better must put the fitting setup first whatever its points")
	}
}

func TestScoreRowUsesOneSaddleTranslationForFitGate(t *testing.T) {
	p := ideal.IdmatchProfile()
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	for _, tc := range []struct {
		brp, radius float64
		want        bool
	}{
		{220, 0.9, true},     // 9 mm from the Aspin target of 211, but 17 mm from a twice-shifted target.
		{194, 0.9, false},    // 17 mm from the Aspin target, but only 9 mm from a twice-shifted target.
		{211, 1.0002, false}, // A near miss is still outside the oval.
	} {
		row := model.CanonicalComparisonRow{Canonical: model.BikeFitAnalysis{
			Saddle: aspin, SelectedBRP: &model.BRPPoint{BrpSetbackMm: tc.brp},
			CockpitFitResult: &model.CockpitFitResult{BestR: tc.radius},
		}}
		got := ScoreRow(row, p)
		if got.Fits != tc.want || Fits(row, p) != got.Fits {
			t.Errorf("Aspin BRP %.1f, R %.4f: score gate %t, direct gate %t, want %t", tc.brp, tc.radius, got.Fits, Fits(row, p), tc.want)
		}
	}
}
