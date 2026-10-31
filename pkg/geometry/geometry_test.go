package geometry_test

import (
	"math"
	"testing"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func TestBinaryHavokGeometryParity(t *testing.T) {
	sid110Static := 521.0
	sag20Pct := sid110Static * 0.20
	saggedFork := sid110Static - sag20Pct // 416.8 mm

	binaryHavok := model.BikeGeometry{
		ID:             "binary-havok-m",
		Manufacturer:   "Binary",
		Model:          "Havok",
		Size:           "M",
		Reach:          398.0,
		Stack:          656.0,
		HtaDeg:         69.0,
		StaDeg:         74.0,
		Htl:            150.0,
		BbDrop:         70.0,
		ForkRake:       44.0,
		ForkA2c:        521.0,
		HeadsetCupType: model.CupEC44,
		Wheelbase:      1150.0,
	}

	// Step 1: Geometric A2C
	baseA2c, baseForkA2c := geometry.CalculateGeometricA2c(
		binaryHavok.Stack,
		binaryHavok.BbDrop,
		binaryHavok.Htl,
		binaryHavok.HtaDeg,
		binaryHavok.ForkRake,
		binaryHavok.HeadsetCupType.CupStackMm(),
	)

	if math.Abs(geometry.Round(baseA2c, 2)-494.58) > 0.05 {
		t.Errorf("Expected baseA2c ~494.58, got %.2f", baseA2c)
	}
	if math.Abs(geometry.Round(baseForkA2c, 2)-482.58) > 0.05 {
		t.Errorf("Expected baseForkA2c ~482.58, got %.2f", baseForkA2c)
	}

	// Step 2: Dynamic Geometry
	totalNewA2c := saggedFork + 12.0 // 428.8 mm
	dyn := geometry.CalculateDynamicGeometry(
		binaryHavok.Reach,
		binaryHavok.Stack,
		binaryHavok.HtaDeg,
		binaryHavok.StaDeg,
		binaryHavok.BbDrop,
		baseA2c,
		totalNewA2c,
		binaryHavok.Wheelbase,
		430.0,
	)

	if dyn.NewHta != 72.06 {
		t.Errorf("Expected NewHta 72.06, got %.2f", dyn.NewHta)
	}
	if dyn.NewSta != 77.06 {
		t.Errorf("Expected NewSta 77.06, got %.2f", dyn.NewSta)
	}
	if dyn.NewReach != 432.5 {
		t.Errorf("Expected NewReach 432.5, got %.1f", dyn.NewReach)
	}
	if dyn.NewStack != 633.8 {
		t.Errorf("Expected NewStack 633.8, got %.1f", dyn.NewStack)
	}
	if dyn.NewBbDrop != 92.6 {
		t.Errorf("Expected NewBbDrop 92.6, got %.1f", dyn.NewBbDrop)
	}
	if dyn.DeltaAngleDeg != -3.06 {
		t.Errorf("Expected DeltaAngleDeg -3.06, got %.2f", dyn.DeltaAngleDeg)
	}

	// Step 3: Master AnalyzeBikeFit pipeline
	fit, err := geometry.AnalyzeBikeFit(
		binaryHavok,
		&saggedFork,
		nil,
		model.DefaultRiderProfile(),
	)
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}

	if fit.StructuralA2c != 428.8 {
		t.Errorf("Expected StructuralA2c 428.8, got %.1f", fit.StructuralA2c)
	}
	if fit.ActiveReach != 432.5 {
		t.Errorf("Expected ActiveReach 432.5, got %.1f", fit.ActiveReach)
	}
	if fit.ActiveStack != 633.8 {
		t.Errorf("Expected ActiveStack 633.8, got %.1f", fit.ActiveStack)
	}
	if fit.Aett != 566.2 {
		t.Errorf("Expected AETT 566.2, got %.1f", fit.Aett)
	}
	if fit.ActualSetback != 174.7 {
		t.Errorf("Expected ActualSetback 174.7, got %.1f", fit.ActualSetback)
	}
	if fit.SpareVerticalStack != 126.4 {
		t.Errorf("Expected SpareVerticalStack 126.4, got %.1f", fit.SpareVerticalStack)
	}
	if fit.SpareAxialSteerer != 132.9 {
		t.Errorf("Expected SpareAxialSteerer 132.9, got %.1f", fit.SpareAxialSteerer)
	}
}

func TestCalculateActualSetback(t *testing.T) {
	// 760mm saddle height at 74.0 deg STA -> 760 * cos(74 deg) = 209.484... -> 209.5 mm
	sb := geometry.CalculateActualSetback(74.0, 760.0)
	if sb != 209.5 {
		t.Errorf("Expected ActualSetback 209.5, got %.1f", sb)
	}
}

func TestBiomechanicalPostureAndWeightDistribution(t *testing.T) {
	profile := model.DefaultRiderProfile()
	baselineBike := geometry.GetDefaultBaselineBike()

	posture, err := geometry.CalculateBaselinePosture(baselineBike, profile)
	if err != nil {
		t.Fatalf("CalculateBaselinePosture failed: %v", err)
	}

	// Modeled back angle should be an endurance bikepacking position (~54.5° with 50mm rise bar and 20° sweep)
	if posture.BackAngleDeg < 40.0 || posture.BackAngleDeg > 65.0 {
		t.Errorf("Expected BackAngleDeg between 40° and 65°, got %.1f°", posture.BackAngleDeg)
	}

	// Hip position should be aft of BB
	if posture.HipX >= 0 {
		t.Errorf("Expected HipX < 0, got %.1f", posture.HipX)
	}

	// System CoG should be near the BB
	if math.Abs(posture.SystemCgX) > 100.0 {
		t.Errorf("Expected SystemCgX near BB (within 100mm), got %.1f", posture.SystemCgX)
	}

	// Test weight distribution on Otso Fenrir Large
	fit, err := geometry.AnalyzeBikeFit(baselineBike, nil, nil, profile)
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}
	if fit.WeightDistFrontPct <= 30.0 || fit.WeightDistFrontPct >= 45.0 {
		t.Errorf("Expected WeightDistFrontPct between 30%% and 45%%, got %.1f%%", fit.WeightDistFrontPct)
	}
	if math.Abs((fit.WeightDistFrontPct+fit.WeightDistRearPct)-100.0) > 0.2 {
		t.Errorf("Expected front + rear to equal 100%%, got %.1f%% + %.1f%%", fit.WeightDistFrontPct, fit.WeightDistRearPct)
	}

	// Test chainstay effect: longer chainstay frame should have higher front weight percentage
	shortCsDist := geometry.CalculateWeightDistribution(420, 640, 68, 140, 70, 50, 500, 430, posture)
	longCsDist := geometry.CalculateWeightDistribution(420, 640, 68, 140, 70, 50, 500, 460, posture)

	if longCsDist.FrontPct <= shortCsDist.FrontPct {
		t.Errorf("Expected longer chainstay to shift weight forward (got short: %.1f%%, long: %.1f%%)", shortCsDist.FrontPct, longCsDist.FrontPct)
	}
}

func TestBentSeatTubeElJefe(t *testing.T) {
	actualSta := 73.3
	nominalSta := 75.0

	sizes := []struct {
		name       string
		stack      float64
		expectedOx float64
	}{
		{"Small", 599.0, 19.21},
		{"Medium", 607.0, 19.46},
		{"Large", 615.0, 19.72},
		{"XL", 627.0, 20.10},
	}

	for _, s := range sizes {
		ox := geometry.CalculateBentSeatTubeOffset(s.stack, actualSta, nominalSta)
		if math.Abs(ox-s.expectedOx) > 0.015 {
			t.Errorf("Size %s: expected Ox ~ %.2f, got %.4f", s.name, s.expectedOx, ox)
		}
	}

	// Reference Lookup Table Verification (Section 5 of Specification)
	lookupCases := []struct {
		sizeName    string
		stack       float64
		saddleL     float64
		expectedSta float64
		expectedSb  float64
	}{
		// Small (Ox ~ 19.21mm)
		{"Small", 599.0, 650.0, 74.92, 169.1},
		{"Small", 599.0, 700.0, 74.81, 183.5},
		{"Small", 599.0, 750.0, 74.71, 197.9},
		{"Small", 599.0, 790.0, 74.63, 209.4},
		{"Small", 599.0, 830.0, 74.57, 220.9},

		// Medium (Ox ~ 19.46mm)
		{"Medium", 607.0, 650.0, 74.94, 168.8},
		{"Medium", 607.0, 700.0, 74.83, 183.2},
		{"Medium", 607.0, 750.0, 74.72, 197.6},
		{"Medium", 607.0, 790.0, 74.65, 209.2},
		{"Medium", 607.0, 830.0, 74.59, 220.7},

		// Large (Ox ~ 19.72mm)
		{"Large", 615.0, 650.0, 74.97, 168.6},
		{"Large", 615.0, 700.0, 74.85, 183.0},
		{"Large", 615.0, 750.0, 74.74, 197.4},
		{"Large", 615.0, 790.0, 74.67, 208.9},
		{"Large", 615.0, 830.0, 74.60, 220.5},

		// XL (Ox ~ 20.10mm)
		{"XL", 627.0, 650.0, 75.00, 168.2},
		{"XL", 627.0, 700.0, 74.88, 182.6},
		{"XL", 627.0, 750.0, 74.77, 197.0},
		{"XL", 627.0, 790.0, 74.70, 208.6},
		{"XL", 627.0, 830.0, 74.63, 220.1},
	}

	for _, tc := range lookupCases {
		ox := geometry.CalculateBentSeatTubeOffset(tc.stack, actualSta, nominalSta)
		effSta := geometry.CalculateEffectiveSta(actualSta, ox, tc.saddleL)
		setback := geometry.CalculateActualSetback(effSta, tc.saddleL)

		if math.Abs(effSta-tc.expectedSta) > 0.015 {
			t.Errorf("[%s at L=%.0f] expected effective STA %.2f°, got %.4f°", tc.sizeName, tc.saddleL, tc.expectedSta, effSta)
		}
		if math.Abs(setback-tc.expectedSb) > 0.15 {
			t.Errorf("[%s at L=%.0f] expected setback %.1fmm, got %.1fmm", tc.sizeName, tc.saddleL, tc.expectedSb, setback)
		}
	}

	// Test Revel El Jefe Large in AnalyzeBikeFit pipeline
	elJefeLarge := model.BikeGeometry{
		ID:           "revel-el-jefe-l",
		Manufacturer: "Revel",
		Model:        "El Jefe",
		Size:         "L",
		Reach:        474.0,
		Stack:        615.0,
		HtaDeg:       67.5,
		StaDeg:       75.0,
		ActualStaDeg: 73.3,
		Htl:          106.0,
		BbDrop:       68.0,
		ForkRake:     44.0,
		ForkA2c:      510.0,
		Wheelbase:    1167.0,
	}

	if !elJefeLarge.HasBentSeatTube() {
		t.Fatalf("Expected elJefeLarge.HasBentSeatTube() to be true")
	}

	fit, err := geometry.AnalyzeBikeFit(elJefeLarge, nil, nil, model.DefaultRiderProfile())
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}

	// Effective STA at 780mm saddle height should be ~74.69°
	if math.Abs(fit.ActiveSta-74.69) > 0.02 {
		t.Errorf("Expected ActiveSta ~ 74.69°, got %.2f°", fit.ActiveSta)
	}

	// Setback at 780mm: 780 * cos(74.688°) = 205.998 mm -> 206.0 mm
	if fit.ActualSetback != 206.0 {
		t.Errorf("Expected ActualSetback 206.0mm, got %.1fmm", fit.ActualSetback)
	}
}

func TestAnalyzeBikeFitRequiresPositiveSeatHeight(t *testing.T) {
	bike := geometry.GetDefaultBaselineBike()

	badProf := model.DefaultRiderProfile()
	badProf.SaddleHeightMm = 0
	if _, err := geometry.AnalyzeBikeFit(bike, nil, nil, badProf); err == nil {
		t.Errorf("Expected error for SaddleHeightMm <= 0 in AnalyzeBikeFit, got nil")
	}
	badProf.SaddleHeightMm = -100
	if _, err := geometry.AnalyzeBikeFit(bike, nil, nil, badProf); err == nil {
		t.Errorf("Expected error for negative SaddleHeightMm in AnalyzeBikeFit, got nil")
	}
	if _, err := geometry.CalculateBaselinePosture(bike, badProf); err == nil {
		t.Errorf("Expected error for SaddleHeightMm <= 0 in CalculateBaselinePosture, got nil")
	}
}

func TestAnalyzeBikeFitRequiresPositiveRake(t *testing.T) {
	bike := geometry.GetDefaultBaselineBike()
	prof := model.DefaultRiderProfile()

	// 1. Zero bike ForkRake
	badBike := bike
	badBike.ForkRake = 0
	if _, err := geometry.AnalyzeBikeFit(badBike, nil, nil, prof); err == nil {
		t.Errorf("Expected error for ForkRake <= 0 in AnalyzeBikeFit, got nil")
	}

	// 2. Negative bike ForkRake
	badBike.ForkRake = -44.0
	if _, err := geometry.AnalyzeBikeFit(badBike, nil, nil, prof); err == nil {
		t.Errorf("Expected error for negative ForkRake in AnalyzeBikeFit, got nil")
	}

	// 3. Zero targetRake
	badRake := 0.0
	if _, err := geometry.AnalyzeBikeFit(bike, nil, &badRake, prof); err == nil {
		t.Errorf("Expected error for targetRake <= 0 in AnalyzeBikeFit, got nil")
	}

	// 4. Negative targetRake
	badRake = -44.0
	if _, err := geometry.AnalyzeBikeFit(bike, nil, &badRake, prof); err == nil {
		t.Errorf("Expected error for negative targetRake in AnalyzeBikeFit, got nil")
	}
}

func TestCalculateHBPoint_TrigonometryAndRotation(t *testing.T) {
	reach := 450.0
	stack := 620.0
	seatHeight := 750.0
	staDeg := 74.0

	// Case 1: HTA = 73°, stem -17° (90° - 73° - 17° = 0° => stem extends strictly horizontally forward)
	// spacer = 0 mm => d_steerer = 0 + 40/2 = 20 mm
	// steererX = reach - 20 * cos(73°)
	// steererY = stack + 20 * sin(73°)
	// HbX = steererX + length * cos(0°) = steererX + 100
	// HbY = steererY + length * sin(0°) = steererY
	stem1 := model.StemSpec{LengthMm: 100.0, AngleDeg: -17.0}
	hb1 := geometry.CalculateHBPoint(reach, stack, 73.0, staDeg, seatHeight, stem1, 0.0)

	expectedSteererX1 := reach - 20.0*math.Cos(73.0*math.Pi/180.0)
	expectedSteererY1 := stack + 20.0*math.Sin(73.0*math.Pi/180.0)
	expectedHbX1 := expectedSteererX1 + 100.0
	expectedHbY1 := expectedSteererY1

	if math.Abs(hb1.HbX-expectedHbX1) > 0.1 {
		t.Errorf("hb1.HbX expected %.2f, got %.2f", expectedHbX1, hb1.HbX)
	}
	if math.Abs(hb1.HbY-expectedHbY1) > 0.1 {
		t.Errorf("hb1.HbY expected %.2f, got %.2f", expectedHbY1, hb1.HbY)
	}

	// Verify Seatpost Top coordinates
	seatX := -seatHeight * math.Cos(staDeg*math.Pi/180.0)
	seatY := seatHeight * math.Sin(staDeg*math.Pi/180.0)
	expectedDist1 := math.Hypot(expectedHbX1-seatX, expectedHbY1-seatY)
	if math.Abs(hb1.SaddleHbMm-expectedDist1) > 0.1 {
		t.Errorf("hb1.SaddleHbMm expected %.2f, got %.2f", expectedDist1, hb1.SaddleHbMm)
	}

	// Case 2: HTA = 70°, stem -6° with 10mm spacers
	// d_steerer = 10 + 20 = 30 mm
	// theta_stem = (90 - 70) + (-6) = 14°
	stem2 := model.StemSpec{LengthMm: 80.0, AngleDeg: -6.0}
	hb2 := geometry.CalculateHBPoint(reach, stack, 70.0, staDeg, seatHeight, stem2, 10.0)

	expectedSteererX2 := reach - 30.0*math.Cos(70.0*math.Pi/180.0)
	expectedSteererY2 := stack + 30.0*math.Sin(70.0*math.Pi/180.0)
	expectedHbX2 := expectedSteererX2 + 80.0*math.Cos(14.0*math.Pi/180.0)
	expectedHbY2 := expectedSteererY2 + 80.0*math.Sin(14.0*math.Pi/180.0)
	expectedDist2 := math.Hypot(expectedHbX2-seatX, expectedHbY2-seatY)

	if math.Abs(hb2.HbX-expectedHbX2) > 0.1 {
		t.Errorf("hb2.HbX expected %.2f, got %.2f", expectedHbX2, hb2.HbX)
	}
	if math.Abs(hb2.HbY-expectedHbY2) > 0.1 {
		t.Errorf("hb2.HbY expected %.2f, got %.2f", expectedHbY2, hb2.HbY)
	}
	if math.Abs(hb2.SaddleHbMm-expectedDist2) > 0.1 {
		t.Errorf("hb2.SaddleHbMm expected %.2f, got %.2f", expectedDist2, hb2.SaddleHbMm)
	}
}

func TestMapPossibleHBs_And_SelectBestHB(t *testing.T) {
	reach := 440.0
	stack := 630.0
	htaDeg := 68.0
	staDeg := 74.0
	seatHeight := 750.0

	maxSpacer := 20.0 // 0, 5, 10, 15, 20 mm (5 spacer points)
	canonical := model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0}
	stems := []model.StemSpec{
		{LengthMm: 50.0, AngleDeg: -6.0},
		{LengthMm: 60.0, AngleDeg: -6.0},
		{LengthMm: 70.0, AngleDeg: -6.0},
	}

	bike := model.BikeGeometry{
		Reach:         reach,
		Stack:         stack,
		HtaDeg:        htaDeg,
		StaDeg:        staDeg,
		MaxSpacerMm:   &maxSpacer,
		CanonicalStem: &canonical,
		Stems:         stems,
	}

	points := geometry.MapPossibleHBs(bike, reach, stack, htaDeg, staDeg, seatHeight)
	// 3 stems * 5 spacers = 15 points
	if len(points) != 15 {
		t.Fatalf("Expected 15 possible HB points, got %d", len(points))
	}

	// 1. With nil target, SelectBestHB should select the canonical stem at 0 spacer
	bestNoTarget, ok := geometry.SelectBestHB(points, canonical, nil, nil)
	if !ok {
		t.Fatalf("SelectBestHB returned false")
	}
	if bestNoTarget.Stem.LengthMm != canonical.LengthMm || bestNoTarget.Stem.AngleDeg != canonical.AngleDeg {
		t.Errorf("Expected canonical stem, got %+v", bestNoTarget.Stem)
	}
	if bestNoTarget.SpacerMm != 0 {
		t.Errorf("Expected 0 spacer with no target, got %.1f", bestNoTarget.SpacerMm)
	}

	// 2. Select with target range matching one specific stem/spacer combination
	// Let's find point for 70mm stem @ 10mm spacer:
	var targetPt model.HBPoint
	for _, p := range points {
		if p.Stem.LengthMm == 70.0 && p.SpacerMm == 10.0 {
			targetPt = p
			break
		}
	}
	targetMin := targetPt.SaddleHbMm - 0.5
	targetMax := targetPt.SaddleHbMm + 0.5
	bestTarget, ok := geometry.SelectBestHB(points, canonical, &targetMin, &targetMax)
	if !ok {
		t.Fatalf("SelectBestHB returned false")
	}
	if math.Abs(bestTarget.SaddleHbMm-targetPt.SaddleHbMm) > 0.001 {
		t.Errorf("Expected SaddleHbMm %.1f, got %.1f", targetPt.SaddleHbMm, bestTarget.SaddleHbMm)
	}
	if bestTarget.Stem.LengthMm != 70.0 || bestTarget.SpacerMm != 10.0 {
		t.Errorf("Expected 70mm stem @ 10mm spacer, got %+v @ %.1fmm", bestTarget.Stem, bestTarget.SpacerMm)
	}

	// 3. Test tie breaking: if two points both fall in [targetMin, targetMax], point with canonical stem wins
	// Let target encompass both 60mm stem @ 10mm and 70mm stem @ 0mm
	var ptCanon, ptOther model.HBPoint
	for _, p := range points {
		if p.Stem.LengthMm == 60.0 && p.SpacerMm == 10.0 {
			ptCanon = p
		}
		if p.Stem.LengthMm == 70.0 && p.SpacerMm == 0.0 {
			ptOther = p
		}
	}
	minVal := math.Min(ptCanon.SaddleHbMm, ptOther.SaddleHbMm) - 2.0
	maxVal := math.Max(ptCanon.SaddleHbMm, ptOther.SaddleHbMm) + 2.0
	bestTie, ok := geometry.SelectBestHB(points, canonical, &minVal, &maxVal)
	if !ok {
		t.Fatalf("SelectBestHB returned false")
	}
	// The 60mm stem (canonical length) must win over 50mm or 70mm
	if bestTie.Stem.LengthMm != canonical.LengthMm {
		t.Errorf("Expected canonical stem length %.0fmm to win tie-break, got %.0fmm", canonical.LengthMm, bestTie.Stem.LengthMm)
	}
}
