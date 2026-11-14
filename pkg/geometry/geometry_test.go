package geometry_test

import (
	"math"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func TestBinaryHavokGeometryParity(t *testing.T) {
	sid110Static := 521.0
	sag20Pct := sid110Static * 0.20
	saggedFork := sid110Static - sag20Pct // 416.8 mm

	binaryHavok := model.BikeGeometry{
		ID:                 "binary-havok-m",
		Manufacturer:       "Binary",
		Model:              "Havok",
		Size:               "M",
		Reach:              398.0,
		Stack:              656.0,
		HtaDeg:             69.0,
		StaDeg:             74.0,
		Htl:                150.0,
		BbDrop:             70.0,
		ForkRake:           44.0,
		ForkA2c:            521.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: 12.0,
		Wheelbase:          1150.0,
	}

	// Step 1: Geometric A2C
	baseA2c, err := geometry.CalculateGeometricA2c(
		binaryHavok.Stack,
		binaryHavok.BbDrop,
		binaryHavok.Htl,
		binaryHavok.HtaDeg,
		binaryHavok.ForkRake,
	)
	if err != nil {
		t.Fatalf("unexpected error in CalculateGeometricA2c: %v", err)
	}
	baseForkA2c := baseA2c - binaryHavok.HeadsetCupType.CupStackMm()

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

	if geometry.Round(dyn.NewHta, 2) != 72.12 {
		t.Errorf("Expected NewHta 72.12, got %.2f", dyn.NewHta)
	}
	if geometry.Round(dyn.NewSta, 2) != 77.12 {
		t.Errorf("Expected NewSta 77.12, got %.2f", dyn.NewSta)
	}
	if geometry.Round(dyn.NewReach, 1) != 433.1 {
		t.Errorf("Expected NewReach 433.1, got %.1f", dyn.NewReach)
	}
	if geometry.Round(dyn.NewStack, 1) != 633.4 {
		t.Errorf("Expected NewStack 633.4, got %.1f", dyn.NewStack)
	}
	if geometry.Round(dyn.NewBbDrop, 1) != 93.0 {
		t.Errorf("Expected NewBbDrop 93.0, got %.1f", dyn.NewBbDrop)
	}
	if geometry.Round(dyn.DeltaAngleDeg, 2) != -3.12 {
		t.Errorf("Expected DeltaAngleDeg -3.12, got %.2f", dyn.DeltaAngleDeg)
	}

	// Step 3: Master AnalyzeBikeFit pipeline
	fit, err := geometry.AnalyzeBikeFit(
		binaryHavok,
		model.Some(saggedFork),
		model.Opt[float64]{},
		catalog.DefaultRiderProfile(),
	)
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}

	if geometry.Round(fit.StructuralA2c, 1) != 428.8 {
		t.Errorf("Expected StructuralA2c 428.8, got %.1f", fit.StructuralA2c)
	}
	if geometry.Round(fit.ActiveReach, 1) != 433.1 {
		t.Errorf("Expected ActiveReach 433.1, got %.1f", fit.ActiveReach)
	}
	if geometry.Round(fit.ActiveStack, 1) != 633.4 {
		t.Errorf("Expected ActiveStack 633.4, got %.1f", fit.ActiveStack)
	}
	if geometry.Round(fit.Aett, 1) != 566.0 {
		t.Errorf("Expected AETT 566.0, got %.1f", fit.Aett)
	}
	if geometry.Round(fit.ActualSetback, 1) != 173.9 {
		t.Errorf("Expected ActualSetback 173.9, got %.1f", fit.ActualSetback)
	}
	if geometry.Round(fit.SpareVerticalStack, 1) != 127.0 {
		t.Errorf("Expected SpareVerticalStack 127.0, got %.1f", fit.SpareVerticalStack)
	}
	if geometry.Round(fit.SpareAxialSteerer, 1) != 133.5 {
		t.Errorf("Expected SpareAxialSteerer 133.5, got %.1f", fit.SpareAxialSteerer)
	}
}

func TestCalculateActualSetback(t *testing.T) {
	// 760mm saddle height at 74.0 deg STA -> 760 * cos(74 deg) = 209.484... -> 209.5 mm
	sb := geometry.CalculateActualSetback(74.0, 760.0)
	if geometry.Round(sb, 1) != 209.5 {
		t.Errorf("Expected ActualSetback 209.5, got %.1f", sb)
	}
}

func TestBiomechanicalPostureAndWeightDistribution(t *testing.T) {
	profile := catalog.DefaultRiderProfile()
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
	fit, err := geometry.AnalyzeBikeFit(baselineBike, model.Opt[float64]{}, model.Opt[float64]{}, profile)
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
	actualSta := model.Deg(73.3)
	nominalSta := model.Deg(75.0)

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
		expectedSta model.Deg
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

		if (effSta - tc.expectedSta).Abs() > 0.015 {
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

	fit, err := geometry.AnalyzeBikeFit(elJefeLarge, model.Opt[float64]{}, model.Opt[float64]{}, catalog.DefaultRiderProfile())
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}

	// Effective STA at 780mm saddle height should be ~74.69°
	if (fit.ActiveSta - 74.69).Abs() > 0.02 {
		t.Errorf("Expected ActiveSta ~ 74.69°, got %.2f°", fit.ActiveSta)
	}

	// Setback at 780mm: 780 * cos(74.688°) = 205.998 mm -> 206.0 mm
	if geometry.Round(fit.ActualSetback, 1) != 206.0 {
		t.Errorf("Expected ActualSetback 206.0mm, got %.1fmm", fit.ActualSetback)
	}
}

func TestAnalyzeBikeFitRequiresPositiveSeatHeight(t *testing.T) {
	bike := geometry.GetDefaultBaselineBike()

	badProf := catalog.DefaultRiderProfile()
	badProf.SaddleHeightMm = 0
	if _, err := geometry.AnalyzeBikeFit(bike, model.Opt[float64]{}, model.Opt[float64]{}, badProf); err == nil {
		t.Errorf("Expected error for SaddleHeightMm <= 0 in AnalyzeBikeFit, got nil")
	}
	badProf.SaddleHeightMm = -100
	if _, err := geometry.AnalyzeBikeFit(bike, model.Opt[float64]{}, model.Opt[float64]{}, badProf); err == nil {
		t.Errorf("Expected error for negative SaddleHeightMm in AnalyzeBikeFit, got nil")
	}
	if _, err := geometry.CalculateBaselinePosture(bike, badProf); err == nil {
		t.Errorf("Expected error for SaddleHeightMm <= 0 in CalculateBaselinePosture, got nil")
	}
}

func TestAnalyzeBikeFitRequiresPositiveRake(t *testing.T) {
	bike := geometry.GetDefaultBaselineBike()
	prof := catalog.DefaultRiderProfile()

	// 1. Zero bike ForkRake
	badBike := bike
	badBike.ForkRake = 0
	if _, err := geometry.AnalyzeBikeFit(badBike, model.Opt[float64]{}, model.Opt[float64]{}, prof); err == nil {
		t.Errorf("Expected error for ForkRake <= 0 in AnalyzeBikeFit, got nil")
	}

	// 2. Negative bike ForkRake
	badBike.ForkRake = -44.0
	if _, err := geometry.AnalyzeBikeFit(badBike, model.Opt[float64]{}, model.Opt[float64]{}, prof); err == nil {
		t.Errorf("Expected error for negative ForkRake in AnalyzeBikeFit, got nil")
	}

	// 3. Zero targetRake
	if _, err := geometry.AnalyzeBikeFit(bike, model.Opt[float64]{}, model.Some(0.0), prof); err == nil {
		t.Errorf("Expected error for targetRake <= 0 in AnalyzeBikeFit, got nil")
	}

	// 4. Negative targetRake
	if _, err := geometry.AnalyzeBikeFit(bike, model.Opt[float64]{}, model.Some(-44.0), prof); err == nil {
		t.Errorf("Expected error for negative targetRake in AnalyzeBikeFit, got nil")
	}
}

func TestCalculateHandlebarOffset(t *testing.T) {
	offset := geometry.DefaultHandlebarOffset()
	// Tumbleweed Persuader 20° (800mm):
	// zb = 135mm, zHand = 400 - 65 = 335mm, Lz = 200mm.
	// OffsetX = -200 * tan(20°) = -72.8 mm
	// OffsetY = 50 - (400 - 135)*tan(4°) + 200*tan(4°) = +45.5 mm
	if math.Abs(offset.X-(-72.8)) > 0.05 {
		t.Errorf("Expected offset.X ~ -72.8mm, got %.2fmm", offset.X)
	}
	if math.Abs(offset.Y-45.5) > 0.05 {
		t.Errorf("Expected offset.Y ~ 45.5mm, got %.2fmm", offset.Y)
	}
}

func TestCalculateHBPoint_TrigonometryAndRotation(t *testing.T) {
	reach := 450.0
	stack := 620.0
	seatHeight := 750.0
	staDeg := model.Deg(74.0)

	// BRP point with 0 offset matches bare seat tube top coordinates
	brp := geometry.CalculateBRPPoint(staDeg, seatHeight, 0.0, 0.0, 0.0)
	barOffset := geometry.DefaultHandlebarOffset()

	// Case 1: HTA = 73°, stem -17° (90° - 73° - 17° = 0° => stem extends strictly horizontally forward)
	// spacer = 0 mm => d_steerer = 0 + 40/2 = 20 mm
	// steererX = reach - 20 * cos(73°)
	// steererY = stack + 20 * sin(73°)
	// ClampX = steererX + length * cos(0°) = steererX + 100
	// ClampY = steererY + length * sin(0°) = steererY
	// HbX = ClampX + barOffset.X
	// HbY = ClampY + barOffset.Y
	stem1 := model.StemSpec{LengthMm: 100.0, AngleDeg: -17.0}
	hb1 := geometry.CalculateHBPoint(reach, stack, 73.0, brp, stem1, 0.0)

	expectedSteererX1 := reach - 20.0*math.Cos(73.0*math.Pi/180.0)
	expectedSteererY1 := stack + 20.0*math.Sin(73.0*math.Pi/180.0)
	expectedClampX1 := expectedSteererX1 + 100.0
	expectedClampY1 := expectedSteererY1
	expectedHbX1 := expectedClampX1 + barOffset.X
	expectedHbY1 := expectedClampY1 + barOffset.Y

	if math.Abs(hb1.ClampX-expectedClampX1) > 0.1 {
		t.Errorf("hb1.ClampX expected %.2f, got %.2f", expectedClampX1, hb1.ClampX)
	}
	if math.Abs(hb1.ClampY-expectedClampY1) > 0.1 {
		t.Errorf("hb1.ClampY expected %.2f, got %.2f", expectedClampY1, hb1.ClampY)
	}
	if math.Abs(hb1.HbX-expectedHbX1) > 0.1 {
		t.Errorf("hb1.HbX expected %.2f, got %.2f", expectedHbX1, hb1.HbX)
	}
	if math.Abs(hb1.HbY-expectedHbY1) > 0.1 {
		t.Errorf("hb1.HbY expected %.2f, got %.2f", expectedHbY1, hb1.HbY)
	}

	// Verify Seatpost Top coordinates
	seatX := -seatHeight * math.Cos(float64(staDeg)*math.Pi/180.0)
	seatY := seatHeight * math.Sin(float64(staDeg)*math.Pi/180.0)
	expectedDist1 := math.Hypot(expectedHbX1-seatX, expectedHbY1-seatY)
	if math.Abs(hb1.SaddleHbMm-expectedDist1) > 0.1 {
		t.Errorf("hb1.SaddleHbMm expected %.2f, got %.2f", expectedDist1, hb1.SaddleHbMm)
	}

	// Case 2: HTA = 70°, stem -6° with 10mm spacers
	// d_steerer = 10 + 20 = 30 mm
	// theta_stem = (90 - 70) + (-6) = 14°
	stem2 := model.StemSpec{LengthMm: 80.0, AngleDeg: -6.0}
	hb2 := geometry.CalculateHBPoint(reach, stack, 70.0, brp, stem2, 10.0)

	expectedSteererX2 := reach - 30.0*math.Cos(70.0*math.Pi/180.0)
	expectedSteererY2 := stack + 30.0*math.Sin(70.0*math.Pi/180.0)
	expectedClampX2 := expectedSteererX2 + 80.0*math.Cos(14.0*math.Pi/180.0)
	expectedClampY2 := expectedSteererY2 + 80.0*math.Sin(14.0*math.Pi/180.0)
	expectedHbX2 := expectedClampX2 + barOffset.X
	expectedHbY2 := expectedClampY2 + barOffset.Y
	expectedDist2 := math.Hypot(expectedHbX2-seatX, expectedHbY2-seatY)

	if math.Abs(hb2.ClampX-expectedClampX2) > 0.1 {
		t.Errorf("hb2.ClampX expected %.2f, got %.2f", expectedClampX2, hb2.ClampX)
	}
	if math.Abs(hb2.ClampY-expectedClampY2) > 0.1 {
		t.Errorf("hb2.ClampY expected %.2f, got %.2f", expectedClampY2, hb2.ClampY)
	}
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
	htaDeg := model.Deg(68.0)
	staDeg := model.Deg(74.0)
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
		MaxSpacerMm:   maxSpacer,
		CanonicalStem: canonical,
		Stems:         stems,
	}

	brp := geometry.CalculateBRPPoint(staDeg, seatHeight, 0.0, 0.0, 0.0)
	points := geometry.MapPossibleHBs(bike, reach, stack, htaDeg, brp)
	// 3 stems * 5 spacers = 15 points
	if len(points) != 15 {
		t.Fatalf("Expected 15 possible HB points, got %d", len(points))
	}

	// 1. With unset target, SelectBestHB should select the canonical stem at 0 spacer
	bestNoTarget, ok := geometry.SelectBestHB(points, canonical, model.Opt[float64]{}, model.Opt[float64]{})
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
	bestTarget, ok := geometry.SelectBestHB(points, canonical, model.Some(targetMin), model.Some(targetMax))
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
	bestTie, ok := geometry.SelectBestHB(points, canonical, model.Some(minVal), model.Some(maxVal))
	if !ok {
		t.Fatalf("SelectBestHB returned false")
	}
	// The 60mm stem (canonical length) must win over 50mm or 70mm
	if bestTie.Stem.LengthMm != canonical.LengthMm {
		t.Errorf("Expected canonical stem length %.0fmm to win tie-break, got %.0fmm", canonical.LengthMm, bestTie.Stem.LengthMm)
	}
}

func TestCalculateBRPPoint_IdmatchPDFMath(t *testing.T) {
	// idmatch report: Saddle height = 746mm (exact hypot(219, 713) = 745.88mm)
	// BRP: X = -219.0mm, Z = 713.0mm => angle from BB = 72.93°
	seatHeight := 746.0

	// Test 1: On a frame with STA = 72.9255° and 0 offset, BRP is exactly (-219.0, 713.0)
	brpZero := geometry.CalculateBRPPoint(72.9255, seatHeight, 0.0, 0.0, 0.0)
	if math.Abs(brpZero.BrpX-(-219.0)) > 0.5 {
		t.Errorf("Expected BrpX ~ -219.0, got %.1f", brpZero.BrpX)
	}
	if math.Abs(brpZero.BrpY-713.0) > 0.5 {
		t.Errorf("Expected BrpY ~ 713.0, got %.1f", brpZero.BrpY)
	}
	if geometry.Round(brpZero.BrpSetbackMm, 1) != 219.0 {
		t.Errorf("Expected BrpSetbackMm 219.0, got %.1f", brpZero.BrpSetbackMm)
	}
	if (brpZero.BrpAngleFromBbDeg - 72.9).Abs() > 0.1 {
		t.Errorf("Expected BrpAngleFromBbDeg 72.9°, got %.1f°", brpZero.BrpAngleFromBbDeg)
	}

	// Test 2: Radial distance invariance hypot(BrpX, BrpY) == seatHeight across various offsets
	for _, railOffset := range []float64{-20.0, -10.0, 0.0, 5.0, 15.0, 20.0} {
		brp := geometry.CalculateBRPPoint(74.0, seatHeight, model.DefaultSeatpostSetbackMm, model.DefaultSaddleBrpOffsetMm, railOffset)
		r := math.Hypot(brp.BrpX, brp.BrpY)
		if math.Abs(r-seatHeight) > 0.15 {
			t.Errorf("Radial distance violated for railOffset=%.1f: expected %.1f, got %.2f", railOffset, seatHeight, r)
		}
	}

	// Test 3: CalculateHBPoint reproduces idmatch Saddle-Handlebar distance d=700mm and drop c=+13mm
	// In the report, BRP = (-219, 713) and HB = (481, 726)
	brpReport := model.BRPPoint{
		BrpX:         -219.0,
		BrpY:         713.0,
		BrpSetbackMm: 219.0,
	}
	// To achieve target hand position (HbX=481.0, HbY=726.0) with a 50mm rise / 20° sweep bar,
	// the stem clamp center must sit downward and forward:
	// ClampX = HbX - OffsetX = 481.0 - (-47.88) = 528.88
	// ClampY = HbY - OffsetY = 726.0 - 50.00 = 676.00
	// For HTA=90°, steererX = 400: stem length = 528.88 - 400 = 128.88, stack = 676 - 20 = 656
	barOffset := geometry.DefaultHandlebarOffset()
	clampX := 481.0 - barOffset.X
	clampY := 726.0 - barOffset.Y

	reach := 400.0 + 20.0*math.Cos(90.0*math.Pi/180.0) // 400
	stack := clampY - 20.0*math.Sin(90.0*math.Pi/180.0)
	stem := model.StemSpec{LengthMm: clampX - 400.0, AngleDeg: 0.0}
	hb := geometry.CalculateHBPoint(reach, stack, 90.0, brpReport, stem, 0.0)

	if math.Abs(hb.HbX-481.0) > 0.1 {
		t.Errorf("Expected HbX ~ 481.0, got %.1f", hb.HbX)
	}
	if math.Abs(hb.HbY-726.0) > 0.1 {
		t.Errorf("Expected HbY ~ 726.0, got %.1f", hb.HbY)
	}
	if math.Abs(hb.SaddleHbMm-700.1) > 0.2 {
		t.Errorf("Expected SaddleHbMm ~ 700.1mm, got %.1fmm", hb.SaddleHbMm)
	}
	if math.Abs(hb.SaddleHbDropMm-13.0) > 0.1 {
		t.Errorf("Expected SaddleHbDropMm 13.0mm, got %.1fmm", hb.SaddleHbDropMm)
	}
}

func TestMapPossibleBRPs_And_SelectBestBRP(t *testing.T) {
	staDeg := model.Deg(72.0)
	seatHeight := 746.0

	points := geometry.MapPossibleBRPs(staDeg, seatHeight)
	// SQlab 610 with 30mm clamp: straight rails 60mm -> clamp window [-15, +15] in 1mm steps = 31 points
	if len(points) != 31 {
		t.Fatalf("Expected 31 candidate BRP points, got %d", len(points))
	}

	// 1. Without target, should select dead-center (RailOffsetMm = 0)
	bestNoTarget, ok := geometry.SelectBestBRP(points, model.Opt[float64]{}, model.Opt[float64]{})
	if !ok {
		t.Fatalf("SelectBestBRP returned false")
	}
	if math.Abs(bestNoTarget.RailOffsetMm) > 0.001 {
		t.Errorf("Expected 0 rail offset without target, got %.1f", bestNoTarget.RailOffsetMm)
	}

	// 2. Target 219mm on STA=72.0° with Redshift sagged 11mm: dead-center setback is 219.1mm
	tMin := 218.5
	tMax := 219.5
	bestTarget, ok := geometry.SelectBestBRP(points, model.Some(tMin), model.Some(tMax))
	if !ok {
		t.Fatalf("SelectBestBRP returned false")
	}
	if math.Abs(bestTarget.BrpSetbackMm-219.0) > 0.5 {
		t.Errorf("Expected BrpSetbackMm ~ 219.0, got %.1f", bestTarget.BrpSetbackMm)
	}
	if math.Abs(bestTarget.RailOffsetMm) > 1.0 {
		t.Errorf("Expected RailOffsetMm near 0mm on 72.0° STA, got %.1f", bestTarget.RailOffsetMm)
	}

	// 3. Target 219mm on steep STA=75.0° (Binary Kinetik): dead center is ~181.5mm, requires +37.5mm (clamps to +15mm)
	steepPoints := geometry.MapPossibleBRPs(75.0, seatHeight)
	bestClamped, ok := geometry.SelectBestBRP(steepPoints, model.Some(tMin), model.Some(tMax))
	if !ok {
		t.Fatalf("SelectBestBRP returned false for steep points")
	}
	if bestClamped.RailOffsetMm != 15.0 {
		t.Errorf("Expected clamp to max +15mm rail offset, got %.1f", bestClamped.RailOffsetMm)
	}
}

func TestSelectBestHBWithDrop_Hierarchy(t *testing.T) {
	canonicalStem := model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0}

	// ptA: SaddleHbMm = 700 (inside distance [695, 705]), SaddleHbDropMm = 25 (12mm away from drop target 13)
	// ptB: SaddleHbMm = 700 (inside distance [695, 705]), SaddleHbDropMm = 14 (1mm away from drop target 13)
	// ptC: SaddleHbMm = 708 (3mm away from distance), SaddleHbDropMm = 13 (0mm away from drop target)
	ptA := model.HBPoint{
		Stem:           model.StemSpec{LengthMm: 60, AngleDeg: 0},
		SpacerMm:       20,
		SaddleHbMm:     700.0,
		SaddleHbDropMm: 25.0,
	}
	ptB := model.HBPoint{
		Stem:           model.StemSpec{LengthMm: 60, AngleDeg: -6},
		SpacerMm:       10,
		SaddleHbMm:     700.0,
		SaddleHbDropMm: 14.0,
	}
	ptC := model.HBPoint{
		Stem:           model.StemSpec{LengthMm: 70, AngleDeg: -6},
		SpacerMm:       10,
		SaddleHbMm:     708.0,
		SaddleHbDropMm: 13.0,
	}

	pts := []model.HBPoint{ptA, ptC, ptB}
	distMin := 695.0
	distMax := 705.0
	dropMin := 8.0
	dropMax := 18.0

	// ptB must win over ptA because both are in distance target range [695, 705], but ptB is in drop target range [8, 18].
	// ptB must win over ptC because distance is PRIMARY (ptC is outside distance range [695, 705]).
	best, ok := geometry.SelectBestHBWithDrop(pts, canonicalStem, model.Some(distMin), model.Some(distMax), model.Some(dropMin), model.Some(dropMax))
	if !ok {
		t.Fatalf("SelectBestHBWithDrop returned false")
	}
	if best.SaddleHbMm != 700.0 || best.SaddleHbDropMm != 14.0 {
		t.Errorf("Expected ptB (700mm, drop 14mm) to win primary distance / secondary drop hierarchy, got dist=%.1f, drop=%.1f",
			best.SaddleHbMm, best.SaddleHbDropMm)
	}
}

func TestCalculateSeatpostExtension(t *testing.T) {
	tests := []struct {
		name           string
		seatHeight     float64
		seatTubeLength float64
		expected       float64
	}{
		{"Valid 746mm saddle with 450mm seat tube", 746.0, 450.0, 296.0},
		{"Valid 746mm saddle with 558.8mm seat tube (Fargo XL)", 746.0, 558.8, 187.2},
		{"Valid 780mm saddle with 480mm seat tube", 780.0, 480.0, 300.0},
		{"Zero seat height", 0.0, 450.0, 0.0},
		{"Negative seat height", -50.0, 450.0, 0.0},
		{"Zero seat tube length", 746.0, 0.0, 0.0},
		{"Negative seat tube length", 746.0, -400.0, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := geometry.CalculateSeatpostExtension(tc.seatHeight, tc.seatTubeLength)
			if math.Abs(got-tc.expected) > 0.05 {
				t.Errorf("CalculateSeatpostExtension(%.1f, %.1f) = %.1f; expected %.1f",
					tc.seatHeight, tc.seatTubeLength, got, tc.expected)
			}
		})
	}
}

func TestFitOval_Geometry(t *testing.T) {
	centerX := 500.0
	centerY := 700.0
	oval := geometry.DefaultFitOval(centerX, centerY)

	// 1. Center point
	rCenter, inCenter := geometry.EvaluateFitOval(oval, centerX, centerY)
	if math.Abs(rCenter) > 1e-6 || !inCenter {
		t.Errorf("Center expected R=0 and inside=true, got R=%.4f, in=%v", rCenter, inCenter)
	}

	// 2. Major axis boundary (tilted 30° clockwise: +X=20*sin(30°)=10, +Y=20*cos(30°)=17.3205)
	xMajor := centerX + 20.0*math.Sin(30.0*math.Pi/180.0)
	yMajor := centerY + 20.0*math.Cos(30.0*math.Pi/180.0)
	rMajor, inMajor := geometry.EvaluateFitOval(oval, xMajor, yMajor)
	if math.Abs(rMajor-1.0) > 0.001 || !inMajor {
		t.Errorf("Major axis boundary expected R=1.0 and inside=true, got R=%.4f, in=%v", rMajor, inMajor)
	}

	// 3. Minor axis boundary (+X=10*cos(30°)=8.6603, -Y=10*sin(30°)=-5.0)
	xMinor := centerX + 10.0*math.Cos(30.0*math.Pi/180.0)
	yMinor := centerY - 10.0*math.Sin(30.0*math.Pi/180.0)
	rMinor, inMinor := geometry.EvaluateFitOval(oval, xMinor, yMinor)
	if math.Abs(rMinor-1.0) > 0.001 || !inMinor {
		t.Errorf("Minor axis boundary expected R=1.0 and inside=true, got R=%.4f, in=%v", rMinor, inMinor)
	}

	// 4. Outside point
	xOut := centerX + 30.0
	yOut := centerY
	rOut, inOut := geometry.EvaluateFitOval(oval, xOut, yOut)
	if rOut <= 1.0 || inOut {
		t.Errorf("Outside point expected R > 1.0 and inside=false, got R=%.4f, in=%v", rOut, inOut)
	}
}

func TestCockpitBaseScore_RollOff(t *testing.T) {
	checks := []struct{ r, want float64 }{
		{0.0, 100.0}, // center
		{0.5, 87.5},  // mid-oval: linear between 100 and 75
		{1.0, 75.0},  // oval edge
		{2.0, 0.0},   // one oval radius past the edge
	}
	for _, c := range checks {
		if s := geometry.CalculateCockpitBaseScore(c.r); math.Abs(s-c.want) > 1e-9 {
			t.Errorf("R=%.2f expected %.2f, got %.4f", c.r, c.want, s)
		}
	}
}

// The cockpit proximity score must never go flat or jump, however far the bars are.
func TestCockpitBaseScore_StrictlyDecreasingAndContinuous(t *testing.T) {
	prev := geometry.CalculateCockpitBaseScore(0)
	for i := 1; i <= 100000; i++ {
		r := float64(i) * 0.001
		cur := geometry.CalculateCockpitBaseScore(r)
		if !(cur < prev) {
			t.Fatalf("not strictly decreasing at R=%.3f: %.6f then %.6f", r, prev, cur)
		}
		if !(cur > -200.0) {
			t.Fatalf("R=%.3f reached the -200 asymptote: %.6f", r, cur)
		}
		if prev-cur > 0.5 {
			t.Fatalf("jump of %.3f at R=%.3f", prev-cur, r)
		}
		prev = cur
	}
	// Far out, two different distances still score differently.
	if !(geometry.CalculateCockpitBaseScore(8.15) < geometry.CalculateCockpitBaseScore(5.55)) {
		t.Error("R=8.15 should score lower than R=5.55")
	}
}

func TestEvaluateCockpitFit(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	canonicalStem := model.StemSpec{LengthMm: 70, AngleDeg: -6}

	// Scenario 1: Multiple setups inside (diminishing returns versatility bonus)
	setupsInside := []model.HBPoint{
		{Stem: model.StemSpec{LengthMm: 70, AngleDeg: -6}, SpacerMm: 10, HbX: 500.0, HbY: 700.0},  // R=0 (Center)
		{Stem: model.StemSpec{LengthMm: 80, AngleDeg: -6}, SpacerMm: 15, HbX: 505.0, HbY: 708.66}, // inside
		{Stem: model.StemSpec{LengthMm: 60, AngleDeg: -6}, SpacerMm: 5, HbX: 495.0, HbY: 691.34},  // inside
	}
	resMulti := geometry.EvaluateCockpitFit(setupsInside, canonicalStem, oval)
	if resMulti.CountInside != 3 {
		t.Errorf("Expected 3 inside, got %d", resMulti.CountInside)
	}
	if resMulti.BestR != 0.0 {
		t.Errorf("Expected BestR 0.0, got %.3f", resMulti.BestR)
	}
	// The bonus fills headroom above the base score; a perfectly centered setup has none.
	if resMulti.VersatilityBonus != 0.0 {
		t.Errorf("Expected no bonus headroom at the oval center, got %.1f", resMulti.VersatilityBonus)
	}
	if resMulti.FinalSubScore != 100.0 {
		t.Errorf("Expected final score 100.0, got %.1f", resMulti.FinalSubScore)
	}

	// Spare setups lift an off-center selection, without ever exceeding 100.
	offCenterAlone := geometry.EvaluateCockpitFit(setupsInside[1:2], canonicalStem, oval)
	offCenterWithSpares := geometry.EvaluateCockpitFit(setupsInside[1:], canonicalStem, oval)
	if !(offCenterWithSpares.FinalSubScore > offCenterAlone.FinalSubScore) {
		t.Errorf("Expected spare setups to raise the score: alone %.2f, with spares %.2f", offCenterAlone.FinalSubScore, offCenterWithSpares.FinalSubScore)
	}
	if offCenterWithSpares.FinalSubScore >= 100.0 {
		t.Errorf("Expected score below 100, got %.2f", offCenterWithSpares.FinalSubScore)
	}

	// Scenario 2: 0 setups inside. There is no separate missing-fit penalty: the score
	// continues smoothly past the oval edge (75 at R=1). At R=1.10 it is just below 75:
	// BaseScore = -200 + 275*h/(h+0.1), h = 200/75 -> ~65.1, and Final equals BaseScore.
	// For R=1.10 along minor axis: dx = 11*cos(30) = 9.526, dy = -11*sin(30) = -5.5
	dx := 11.0 * math.Cos(30.0*math.Pi/180.0)
	dy := -11.0 * math.Sin(30.0*math.Pi/180.0)
	setupsOutside := []model.HBPoint{
		{Stem: model.StemSpec{LengthMm: 90, AngleDeg: 0}, SpacerMm: 0, HbX: 500.0 + dx, HbY: 700.0 + dy},
	}
	resZero := geometry.EvaluateCockpitFit(setupsOutside, canonicalStem, oval)
	if resZero.CountInside != 0 {
		t.Errorf("Expected 0 inside, got %d", resZero.CountInside)
	}
	if math.Abs(resZero.BaseScore-65.1) > 0.1 {
		t.Errorf("Expected BaseScore ~65.1, got %.2f", resZero.BaseScore)
	}
	if math.Abs(resZero.FinalSubScore-resZero.BaseScore) > 1e-9 {
		t.Errorf("Expected FinalSubScore == BaseScore with a single outside setup, got %.2f vs %.2f", resZero.FinalSubScore, resZero.BaseScore)
	}
}

func TestTubeAxisHelpers(t *testing.T) {
	// SeatTubeAxis for STA 74° starting at origin (0,0)
	seatAxis := geometry.SeatTubeAxis(geom2d.Origin, 74.0)
	seatDir := geometry.SeatTubeDir(74.0)

	expectedX := -math.Cos(74.0 * math.Pi / 180.0)
	expectedY := math.Sin(74.0 * math.Pi / 180.0)

	if math.Abs(seatDir.X-expectedX) > 1e-9 || math.Abs(seatDir.Y-expectedY) > 1e-9 {
		t.Errorf("SeatTubeDir(74) = (%v, %v), expected (%v, %v)", seatDir.X, seatDir.Y, expectedX, expectedY)
	}
	if math.Abs(seatAxis.Direction.X-expectedX) > 1e-9 || math.Abs(seatAxis.Direction.Y-expectedY) > 1e-9 {
		t.Errorf("SeatTubeAxis(74).Direction = (%v, %v), expected (%v, %v)", seatAxis.Direction.X, seatAxis.Direction.Y, expectedX, expectedY)
	}

	// SteererTubeAxis for HTA 68° starting at head tube top (420, 640)
	headTubeTop := geom2d.Point2{X: 420.0, Y: 640.0}
	steererAxis := geometry.SteererTubeAxis(headTubeTop, 68.0)
	steererDir := geometry.SteererTubeDir(68.0)

	expectedHtaX := -math.Cos(68.0 * math.Pi / 180.0)
	expectedHtaY := math.Sin(68.0 * math.Pi / 180.0)

	if math.Abs(steererDir.X-expectedHtaX) > 1e-9 || math.Abs(steererDir.Y-expectedHtaY) > 1e-9 {
		t.Errorf("SteererTubeDir(68) = (%v, %v), expected (%v, %v)", steererDir.X, steererDir.Y, expectedHtaX, expectedHtaY)
	}
	if math.Abs(steererAxis.Direction.X-expectedHtaX) > 1e-9 || math.Abs(steererAxis.Direction.Y-expectedHtaY) > 1e-9 {
		t.Errorf("SteererTubeAxis(68).Direction = (%v, %v), expected (%v, %v)", steererAxis.Direction.X, steererAxis.Direction.Y, expectedHtaX, expectedHtaY)
	}
	if steererAxis.Origin != headTubeTop {
		t.Errorf("SteererTubeAxis origin = %v, expected %v", steererAxis.Origin, headTubeTop)
	}
}

func TestCalculateGeometricA2c_Errors(t *testing.T) {
	// Non-positive head tube length
	_, err := geometry.CalculateGeometricA2c(600, 70, 0, 68, 44)
	if err == nil {
		t.Errorf("expected error for headTubeLength = 0, got nil")
	}

	// Negative head tube length
	_, err = geometry.CalculateGeometricA2c(600, 70, -100, 68, 44)
	if err == nil {
		t.Errorf("expected error for headTubeLength < 0, got nil")
	}
}

func TestCalculateTrail(t *testing.T) {
	// Formula: Trail = (R * cos(HTA) - rake) / sin(HTA)
	// Example 1: 29x2.4" tire (R = 372.0mm), 68.0° HTA, 44.0mm rake
	// cos(68°) ≈ 0.37460659, sin(68°) ≈ 0.92718385
	// R * cos(68°) = 372 * 0.37460659 = 139.35365
	// Numerator = 139.35365 - 44 = 95.35365
	// Trail = 95.35365 / 0.92718385 = 102.84 mm
	trail := geometry.CalculateTrail(68.0, 44.0, 372.0)
	if math.Abs(trail-102.84) > 0.05 {
		t.Errorf("Expected trail ~102.84, got %.2f", trail)
	}

	// Example 2: Fenrir rigid baseline (HTA = 67.7°, rake = 54.0mm, R = 372.0mm)
	fenrirRigidTrail := geometry.CalculateTrail(67.7, 54.0, 372.0)
	if math.Abs(fenrirRigidTrail-94.20) > 0.05 {
		t.Errorf("Expected fenrir rigid trail ~94.20, got %.2f", fenrirRigidTrail)
	}

	// Example 3: Invalid input (0 or negative HTA)
	zeroTrail := geometry.CalculateTrail(0.0, 44.0, 372.0)
	if zeroTrail != 0.0 {
		t.Errorf("Expected 0.0 for 0 HTA, got %.2f", zeroTrail)
	}
}

func TestSaddleElevationAndTranslationMethods(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	// 1. SitSurfaceHeightFromBrpHeight
	// SQlab 610: at 746mm SH and 219mm setback:
	// brpY = sqrt(746^2 - 219^2) = 713.147mm
	// sitPlaneY = 713.147 + 10.0 = 723.147mm
	// sitSurfaceHeight = hypot(219, 723.147) = 755.56mm
	sitSurfSQ := sqlab.SitSurfaceHeightFromBrpHeight(746.0, 219.0)
	if math.Abs(sitSurfSQ-755.56) > 0.1 {
		t.Errorf("Expected SQlab sit surface height ~755.56mm, got %.2fmm", sitSurfSQ)
	}

	// Flat saddle (Aspin): at 751.37mm and 204mm setback, step is 0:
	// sitSurfaceHeight == 751.37mm
	sitSurfAspin := aspin.SitSurfaceHeightFromBrpHeight(751.37, 204.0)
	if math.Abs(sitSurfAspin-751.37) > 0.1 {
		t.Errorf("Expected Aspin sit surface height ~751.37mm, got %.2fmm", sitSurfAspin)
	}

	// 2. TranslateSaddleHeightFromRef
	// SQlab (ref) -> Berthoud Aspin
	shAspin := aspin.TranslateSaddleHeightFromRef(sqlab, 746.0, 219.0)
	if math.Abs(shAspin-751.37) > 0.1 {
		t.Errorf("Expected translated Aspin SH ~751.37mm, got %.2fmm", shAspin)
	}

	// SQlab (ref) -> SLR Boost
	shSLR := slr.TranslateSaddleHeightFromRef(sqlab, 746.0, 219.0)
	if math.Abs(shSLR-752.73) > 0.1 {
		t.Errorf("Expected translated SLR Boost SH ~752.73mm, got %.2fmm", shSLR)
	}

	// 3. 2D Vector Translation (BrpToSitBonesVec, SitBonePointFromBrp, BrpPointFromSitBones, TranslateBrpFromRef)
	sqVec := sqlab.BrpToSitBonesVec()
	if sqVec.X != -73.0 || sqVec.Y != 10.0 {
		t.Errorf("Expected SQlab BrpToSitBonesVec (-73.0, 10.0), got (%.1f, %.1f)", sqVec.X, sqVec.Y)
	}
	aspinVec := aspin.BrpToSitBonesVec()
	if aspinVec.X != -88.0 || aspinVec.Y != 0.0 {
		t.Errorf("Expected Aspin BrpToSitBonesVec (-88.0, 0.0), got (%.1f, %.1f)", aspinVec.X, aspinVec.Y)
	}
	refBrp := sqlab.BrpPointAtHeightAndSetback(746.0, 219.0)
	sitBonePt := sqlab.SitBonePointFromBrp(refBrp)
	if math.Abs(sitBonePt.X-(-292.0)) > 0.1 || math.Abs(sitBonePt.Y-723.15) > 0.1 {
		t.Errorf("Expected SitBonePoint (-292.0, 723.15), got (%.2f, %.2f)", sitBonePt.X, sitBonePt.Y)
	}
	aspinBrp := aspin.TranslateBrpFromRef(sqlab, refBrp)
	if math.Abs(aspinBrp.X-(-204.0)) > 0.1 || math.Abs(aspinBrp.Y-723.15) > 0.1 {
		t.Errorf("Expected translated Aspin BRP (-204.0, 723.15), got (%.2f, %.2f)", aspinBrp.X, aspinBrp.Y)
	}

	// 4. TranslateHbDistanceFromRef & Sit-Bone-Relative HB Rise
	// From SQlab at 746mm SH, 219mm setback, 700mm HB dist, +3mm rise from sit bones (+13mm from SQlab BRP):
	// Invariant HB target in space is (+480.9mm, 726.1mm).
	// Aspin BRP is at (-204.0mm, 723.1mm).
	// Distance = hypot(480.88 - (-204.0), 726.15 - 723.15) = hypot(684.88, 3.0) = 684.89mm.
	tDist := aspin.TranslateHbDistanceFromRef(sqlab, 746.0, 219.0, 700.0, 3.0)
	if math.Abs(tDist-684.89) > 0.1 {
		t.Errorf("Expected translated Aspin HB dist ~684.89mm, got %.2fmm", tDist)
	}
}

func TestEvaluateCockpitFit_StemExtremityPreference(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	canonicalStem := model.StemSpec{LengthMm: 80, AngleDeg: -6}
	expectedRange := []float64{60.0, 100.0} // Midpoint 80mm

	// Candidate A: 80mm stem (midpoint, penalty=0), slightly off-center inside oval (R=0.25)
	// BaseScore = 100 - 25*(0.25)^2 = 98.44, StemPenalty = 0.0 -> CompScore = 98.44
	// Candidate B: 100mm stem (extreme, penalty=15), closer to center inside oval (R=0.10)
	// BaseScore = 100 - 25*(0.10)^2 = 99.75, StemPenalty = 15.0 -> CompScore = 84.75
	dxA := 2.5 * math.Cos(30.0*math.Pi/180.0)
	dyA := -2.5 * math.Sin(30.0*math.Pi/180.0)
	dxB := 1.0 * math.Cos(30.0*math.Pi/180.0)
	dyB := -1.0 * math.Sin(30.0*math.Pi/180.0)

	candidates := []model.HBPoint{
		{Stem: model.StemSpec{LengthMm: 100, AngleDeg: -6}, SpacerMm: 0, HbX: 500.0 + dxB, HbY: 700.0 + dyB},
		{Stem: model.StemSpec{LengthMm: 80, AngleDeg: -6}, SpacerMm: 10, HbX: 500.0 + dxA, HbY: 700.0 + dyA},
	}

	res := geometry.EvaluateCockpitFit(candidates, canonicalStem, oval, expectedRange)

	// Verify 80mm stem was preferred over 100mm stem
	if res.SelectedHB.Stem.LengthMm != 80.0 {
		t.Errorf("Expected 80mm stem to be selected due to stem extremity down-weighting of 100mm, got %.0fmm", res.SelectedHB.Stem.LengthMm)
	}
	if res.StemPenalty != 0.0 {
		t.Errorf("Expected StemPenalty 0.0 for 80mm stem, got %.1f", res.StemPenalty)
	}
	if res.CountInside != 2 {
		t.Errorf("Expected 2 inside oval, got %d", res.CountInside)
	}

	// Now evaluate only candidate B (100mm stem alone)
	resOnly100 := geometry.EvaluateCockpitFit([]model.HBPoint{candidates[0]}, canonicalStem, oval, expectedRange)
	if resOnly100.SelectedHB.Stem.LengthMm != 100.0 {
		t.Fatalf("Expected 100mm stem selected, got %.0fmm", resOnly100.SelectedHB.Stem.LengthMm)
	}
	if resOnly100.StemPenalty != 15.0 {
		t.Errorf("Expected 15.0 stem penalty for 100mm stem, got %.1f", resOnly100.StemPenalty)
	}
	// Final score should reflect down-weighting: BaseScore (97.5 at R=0.1) - StemPenalty (15.0) = 82.5
	if math.Abs(resOnly100.FinalSubScore-82.5) > 0.2 {
		t.Errorf("Expected FinalSubScore ~82.5 for 100mm stem, got %.1f", resOnly100.FinalSubScore)
	}
}

// Moving one candidate across the oval edge must not make the final score jump.
func TestEvaluateCockpitFit_ContinuousAcrossOvalEdge(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	canonicalStem := model.StemSpec{LengthMm: 80, AngleDeg: -6}
	expectedRange := []float64{60.0, 100.0}
	cos30, sin30 := math.Cos(30.0*math.Pi/180.0), math.Sin(30.0*math.Pi/180.0)

	// A second, fixed candidate sits comfortably inside; the first slides along the minor axis.
	fixed := model.HBPoint{Stem: model.StemSpec{LengthMm: 90, AngleDeg: -6}, SpacerMm: 5, HbX: 500 + 4*cos30, HbY: 700 - 4*sin30}

	prev := math.NaN()
	for i := 0; i <= 400; i++ {
		rr := 0.5 + float64(i)*0.0025 // R from 0.5 to 1.5, crossing the oval edge at 1.0
		moving := model.HBPoint{Stem: model.StemSpec{LengthMm: 80, AngleDeg: -6}, SpacerMm: 0, HbX: 500 + rr*10*cos30, HbY: 700 - rr*10*sin30}
		res := geometry.EvaluateCockpitFit([]model.HBPoint{moving, fixed}, canonicalStem, oval, expectedRange)
		if !math.IsNaN(prev) && math.Abs(res.FinalSubScore-prev) > 1.0 {
			t.Fatalf("final score jumps from %.3f to %.3f near R=%.4f", prev, res.FinalSubScore, rr)
		}
		prev = res.FinalSubScore
	}
}

// Candidates that are all hopelessly outside the oval must still be told apart.
func TestEvaluateCockpitFit_FarOutsideStillRanksByDistance(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	stem := model.StemSpec{LengthMm: 100, AngleDeg: -6}
	var prev float64
	for i, off := range []float64{50, 63, 74, 87, 103} {
		pt := model.HBPoint{Stem: stem, HbX: 500 - off, HbY: 700}
		res := geometry.EvaluateCockpitFit([]model.HBPoint{pt}, stem, oval)
		if i > 0 && !(res.FinalSubScore < prev) {
			t.Errorf("offset %vmm scored %.3f, not below the nearer offset's %.3f", off, res.FinalSubScore, prev)
		}
		prev = res.FinalSubScore
	}
}

// F-p4ab-3: Exact 2D rigid-body rotation preserves distances and diverges from small-angle lift over ±50mm swaps.
func TestCalculateDynamicGeometry_Exact2DRotation(t *testing.T) {
	baseReach, baseStack := 428.0, 642.0
	baseHta, baseSta, baseBbDrop := 68.25, 75.25, 72.0
	wb, cs := 1158.0, 450.0
	baseDist := math.Hypot(baseReach, baseStack)
	baseRearDist := cs

	for _, delta := range []float64{-50, -25, 0, 25, 50} {
		dyn := geometry.CalculateDynamicGeometry(
			baseReach, baseStack, model.Deg(baseHta), model.Deg(baseSta), baseBbDrop,
			495.0, 495.0+delta, wb, cs,
		)
		newDist := math.Hypot(dyn.NewReach, dyn.NewStack)
		if math.Abs(newDist-baseDist) > 0.2 {
			t.Errorf("delta=%.0fmm: BB-to-HT distance %.2f drifted from %.2f", delta, newDist, baseDist)
		}
		if math.Abs(float64(dyn.NewHta-dyn.NewSta)-(baseHta-baseSta)) > 0.02 {
			t.Errorf("delta=%.0fmm: HTA-STA %.3f != %.3f", delta, dyn.NewHta-dyn.NewSta, baseHta-baseSta)
		}
		if delta == 0 && (dyn.DeltaAngleDeg != 0 || dyn.NewReach != baseReach || dyn.NewStack != baseStack) {
			t.Errorf("delta=0: expected zero pitch, got %+v", dyn)
		}
		_ = baseRearDist
	}

	// Verify exact 2D pitch formula vs small-angle asin(dy/wb) at +50mm swap.
	dyn50 := geometry.CalculateDynamicGeometry(baseReach, baseStack, model.Deg(baseHta), model.Deg(baseSta), baseBbDrop, 495.0, 545.0, wb, cs)
	htaRad := baseHta * math.Pi / 180.0
	wantDeg := math.Atan2(50.0*math.Sin(htaRad), wb+50.0*math.Cos(htaRad)) * 180.0 / math.Pi
	if math.Abs(float64(dyn50.DeltaAngleDeg)-wantDeg) > 0.015 {
		t.Errorf("+50mm DeltaAngleDeg = %.2f, want exact 2D %.4f", dyn50.DeltaAngleDeg, wantDeg)
	}
}

// F-p4ab-3: Normalizing CAD baseline vs CupStackMm/GeoHeadsetSpacerMm avoids spurious pitch at the CAD fork.
func TestAnalyzeBikeFit_NormalizesCadBaseline(t *testing.T) {
	// Fenrir-like frame where CupEC44 (12mm) != GeoHeadsetSpacerMm (0mm) and Stack has slight slop vs 495mm CAD fork.
	bike := model.BikeGeometry{
		ID:                 "fenrir-xs",
		Size:               "XS",
		Reach:              387.0,
		Stack:              595.0,
		HtaDeg:             68.25,
		StaDeg:             75.25,
		Htl:                90.0,
		BbDrop:             72.0,
		ForkRake:           54.0,
		Wheelbase:          1098.0,
		ChainstayLength:    450.0,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: 0.0,
		CadFork:            model.ForkRef{Fork: "lithic_495", A2cMm: 495.0, RakeMm: 54.0},
	}
	fit, err := geometry.AnalyzeBikeFit(bike, model.Some(495.0), model.Opt[float64]{}, catalog.DefaultRiderProfile())
	if err != nil {
		t.Fatalf("AnalyzeBikeFit failed: %v", err)
	}
	if fit.DeltaAngleDeg != 0 || fit.ActiveReach != bike.Reach || fit.ActiveStack != bike.Stack || fit.ActiveHta != bike.HtaDeg || fit.ActiveBbDrop != bike.BbDrop {
		t.Errorf("CAD fork evaluation should preserve published CAD chart: got Delta=%.2f Reach=%.1f Stack=%.1f HTA=%.2f BBD=%.1f",
			fit.DeltaAngleDeg, fit.ActiveReach, fit.ActiveStack, fit.ActiveHta, fit.ActiveBbDrop)
	}
}
