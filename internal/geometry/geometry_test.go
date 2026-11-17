package geometry_test

import (
	"math"
	"testing"

	"bikegeo/internal/catalog"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/model"
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
		ChainstayLength:    430.0,
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

	// Step 2: Master AnalyzeBikeFitWithSaddle pipeline (BuildScene levels from exact 2D wb0 = fa0.X - ra0.X
	// ~1070.8 mm rather than the synthetic fixture's contradictory Wheelbase = 1150.0 mm).
	fit, err := geometry.AnalyzeBikeFitWithSaddle(
		binaryHavok,
		model.Some(saggedFork),
		model.Opt[float64]{},
		catalog.DefaultRiderProfile(),
		catalog.DefaultSaddle(),
	)
	if err != nil {
		t.Fatalf("AnalyzeBikeFitWithSaddle failed: %v", err)
	}

	if geometry.Round(fit.StructuralA2c, 1) != 428.8 {
		t.Errorf("Expected StructuralA2c 428.8, got %.1f", fit.StructuralA2c)
	}
	if geometry.Round(fit.ActiveReach, 1) != 434.9 {
		t.Errorf("Expected ActiveReach 434.9, got %.1f", fit.ActiveReach)
	}
	if geometry.Round(fit.ActiveStack, 1) != 632.1 {
		t.Errorf("Expected ActiveStack 632.1, got %.1f", fit.ActiveStack)
	}
	if geometry.Round(fit.Aett, 1) != 565.5 {
		t.Errorf("Expected AETT 565.5, got %.1f", fit.Aett)
	}
	if geometry.Round(fit.ActualSetback, 1) != 171.7 {
		t.Errorf("Expected ActualSetback 171.7, got %.1f", fit.ActualSetback)
	}
	if geometry.Round(fit.SpareVerticalStack, 1) != 128.7 {
		t.Errorf("Expected SpareVerticalStack 128.7, got %.1f", fit.SpareVerticalStack)
	}
	if geometry.Round(fit.SpareAxialSteerer, 1) != 135.1 {
		t.Errorf("Expected SpareAxialSteerer 135.1, got %.1f", fit.SpareAxialSteerer)
	}
}

func testBaselineBike() model.BikeGeometry {
	return model.BikeGeometry{
		ID:              "otso-fenrir-stainless-l",
		Manufacturer:    "Otso",
		Model:           "Fenrir",
		Size:            "L",
		Reach:           428.0,
		Stack:           642.0,
		HtaDeg:          68.25,
		StaDeg:          75.25,
		Htl:             140.0,
		BbDrop:          72.0,
		ForkRake:        54.0,
		Wheelbase:       1158.0,
		ChainstayLength: 450.0,
		ForkA2c:         495.0,
		HeadsetCupType:  model.CupEC44,
	}
}

func TestRearCenterWheelbaseRatio(t *testing.T) {
	bike := testBaselineBike()
	fit, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, catalog.DefaultRiderProfile(), catalog.DefaultSaddle())
	if err != nil {
		t.Fatalf("AnalyzeBikeFitWithSaddle failed: %v", err)
	}
	if fit.RearCenterWheelbaseRatio <= 0.30 || fit.RearCenterWheelbaseRatio >= 0.45 {
		t.Errorf("Expected RearCenterWheelbaseRatio between 0.30 and 0.45, got %.4f", fit.RearCenterWheelbaseRatio)
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
		setback := -geometry.SeatTubeAxis(geom2d.Origin, effSta).PointAt(tc.saddleL).X

		if (effSta - tc.expectedSta).Abs() > 0.015 {
			t.Errorf("[%s at L=%.0f] expected effective STA %.2f°, got %.4f°", tc.sizeName, tc.saddleL, tc.expectedSta, effSta)
		}
		if math.Abs(setback-tc.expectedSb) > 0.15 {
			t.Errorf("[%s at L=%.0f] expected setback %.1fmm, got %.1fmm", tc.sizeName, tc.saddleL, tc.expectedSb, setback)
		}
	}

	// Test Revel El Jefe Large in AnalyzeBikeFitWithSaddle pipeline
	elJefeLarge := model.BikeGeometry{
		ID:              "revel-el-jefe-l",
		Manufacturer:    "Revel",
		Model:           "El Jefe",
		Size:            "L",
		Reach:           474.0,
		Stack:           615.0,
		HtaDeg:          67.5,
		StaDeg:          75.0,
		ActualStaDeg:    73.3,
		Htl:             106.0,
		BbDrop:          68.0,
		ChainstayLength: 430.0,
		ForkRake:        44.0,
		ForkA2c:         510.0,
		Wheelbase:       1167.0,
	}

	if !elJefeLarge.HasBentSeatTube() {
		t.Fatalf("Expected elJefeLarge.HasBentSeatTube() to be true")
	}

	fit, err := geometry.AnalyzeBikeFitWithSaddle(elJefeLarge, model.Opt[float64]{}, model.Opt[float64]{}, catalog.DefaultRiderProfile(), catalog.DefaultSaddle())
	if err != nil {
		t.Fatalf("AnalyzeBikeFitWithSaddle failed: %v", err)
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
	bike := testBaselineBike()
	saddle := catalog.DefaultSaddle()

	badProf := catalog.DefaultRiderProfile()
	badProf.SaddleHeightMm = 0
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, badProf, saddle); err == nil {
		t.Errorf("Expected error for SaddleHeightMm <= 0 in AnalyzeBikeFitWithSaddle, got nil")
	}
	badProf.SaddleHeightMm = -100
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, badProf, saddle); err == nil {
		t.Errorf("Expected error for negative SaddleHeightMm in AnalyzeBikeFitWithSaddle, got nil")
	}
}

func TestAnalyzeBikeFitRequiresPositiveRake(t *testing.T) {
	bike := testBaselineBike()
	prof := catalog.DefaultRiderProfile()
	saddle := catalog.DefaultSaddle()

	// 1. Zero bike ForkRake
	badBike := bike
	badBike.ForkRake = 0
	if _, err := geometry.AnalyzeBikeFitWithSaddle(badBike, model.Opt[float64]{}, model.Opt[float64]{}, prof, saddle); err == nil {
		t.Errorf("Expected error for ForkRake <= 0 in AnalyzeBikeFitWithSaddle, got nil")
	}

	// 2. Negative bike ForkRake
	badBike.ForkRake = -44.0
	if _, err := geometry.AnalyzeBikeFitWithSaddle(badBike, model.Opt[float64]{}, model.Opt[float64]{}, prof, saddle); err == nil {
		t.Errorf("Expected error for negative ForkRake in AnalyzeBikeFitWithSaddle, got nil")
	}

	// 3. Zero targetRake
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Some(0.0), prof, saddle); err == nil {
		t.Errorf("Expected error for targetRake <= 0 in AnalyzeBikeFitWithSaddle, got nil")
	}

	// 4. Negative targetRake
	if _, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Some(-44.0), prof, saddle); err == nil {
		t.Errorf("Expected error for negative targetRake in AnalyzeBikeFitWithSaddle, got nil")
	}
}

func TestCalculateHandlebarOffset(t *testing.T) {
	offset, err := geometry.GetHandlebarOffset(catalog.DefaultHandlebar(), model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatalf("GetHandlebarOffset: %v", err)
	}
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
	bar := catalog.DefaultHandlebar()

	// BRP point with 0 offset matches bare seat tube top coordinates
	brp, placementErr := geometry.SolveBRPPoint(model.SaddleSpec{}, staDeg, seatHeight, 0.0, 0.0)
	if placementErr != nil {
		t.Fatal(placementErr)
	}
	barOffset, err := geometry.GetHandlebarOffset(bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatal(err)
	}

	// Case 1: HTA = 73°, stem -17° (90° - 73° - 17° = 0° => stem extends strictly horizontally forward)
	// spacer = 0 mm => d_steerer = 11 (HeadsetTopStackMm) + 0 + 40/2 = 31 mm
	// steererX = reach - 31 * cos(73°)
	// steererY = stack + 31 * sin(73°)
	// ClampX = steererX + length * cos(0°) = steererX + 100
	// ClampY = steererY + length * sin(0°) = steererY
	// HbX = ClampX + barOffset.X
	// HbY = ClampY + barOffset.Y
	stem1 := model.StemSpec{LengthMm: 100.0, AngleDeg: -17.0}
	hb1, err := geometry.CalculateHBPointWithHandlebar(reach, stack, 73.0, brp, stem1, 0.0, bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatalf("CalculateHBPointWithHandlebar(hb1): %v", err)
	}

	expectedSteererX1 := reach - 31.0*math.Cos(73.0*math.Pi/180.0)
	expectedSteererY1 := stack + 31.0*math.Sin(73.0*math.Pi/180.0)
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
	// d_steerer = 11 (HeadsetTopStackMm) + 10 + 20 = 41 mm
	// theta_stem = (90 - 70) + (-6) = 14°
	stem2 := model.StemSpec{LengthMm: 80.0, AngleDeg: -6.0}
	hb2, err := geometry.CalculateHBPointWithHandlebar(reach, stack, 70.0, brp, stem2, 10.0, bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatalf("CalculateHBPointWithHandlebar(hb2): %v", err)
	}

	expectedSteererX2 := reach - 41.0*math.Cos(70.0*math.Pi/180.0)
	expectedSteererY2 := stack + 41.0*math.Sin(70.0*math.Pi/180.0)
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

func TestMapPossibleHBs_CoversEveryStemAndSpacer(t *testing.T) {
	reach := 440.0
	stack := 630.0
	htaDeg := model.Deg(68.0)
	staDeg := model.Deg(74.0)
	seatHeight := 750.0

	bike := model.BikeGeometry{
		Reach:         reach,
		Stack:         stack,
		HtaDeg:        htaDeg,
		StaDeg:        staDeg,
		MaxSpacerMm:   20.0, // 0, 5, 10, 15, 20 mm (5 spacer points)
		CanonicalStem: model.StemSpec{LengthMm: 60.0, AngleDeg: -6.0},
		Stems: []model.StemSpec{
			{LengthMm: 50.0, AngleDeg: -6.0},
			{LengthMm: 60.0, AngleDeg: -6.0},
			{LengthMm: 70.0, AngleDeg: -6.0},
		},
	}

	brp, placementErr := geometry.SolveBRPPoint(model.SaddleSpec{}, staDeg, seatHeight, 0.0, 0.0)
	if placementErr != nil {
		t.Fatal(placementErr)
	}
	points, err := geometry.MapPossibleHBsWithHandlebar(bike, reach, stack, htaDeg, brp, catalog.DefaultHandlebar(), model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatalf("MapPossibleHBsWithHandlebar: %v", err)
	}
	// 3 stems * 5 spacers = 15 points
	if len(points) != 15 {
		t.Fatalf("Expected 15 possible HB points, got %d", len(points))
	}
	seen := map[[2]float64]bool{}
	for _, p := range points {
		seen[[2]float64{p.Stem.LengthMm, p.SpacerMm}] = true
	}
	if len(seen) != 15 {
		t.Errorf("Expected 15 distinct stem/spacer combinations, got %d", len(seen))
	}
}

func TestCalculateBRPPoint_IdmatchPDFMath(t *testing.T) {
	// idmatch report: Saddle height = 746mm (exact hypot(219, 713) = 745.88mm)
	// BRP: X = -219.0mm, Z = 713.0mm => angle from BB = 72.93°
	seatHeight := 746.0
	refSaddle := model.ReferenceIdmatchSaddle()
	bar := catalog.DefaultHandlebar()

	// Test 1: On a frame with STA = 72.9255° and 0 offset, BRP is exactly (-219.0, 713.0)
	brpZero, placementErr := geometry.SolveBRPPoint(model.SaddleSpec{}, 72.9255, seatHeight, 0.0, 0.0)
	if placementErr != nil {
		t.Fatal(placementErr)
	}
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
		brp, placementErr := geometry.SolveBRPPoint(refSaddle, 74.0, seatHeight, model.DefaultSeatpostSetbackMm, railOffset)
		if placementErr != nil {
			t.Fatal(placementErr)
		}
		r := math.Hypot(brp.BrpX, brp.BrpY)
		if math.Abs(r-seatHeight) > 0.15 {
			t.Errorf("Radial distance violated for railOffset=%.1f: expected %.1f, got %.2f", railOffset, seatHeight, r)
		}
	}

	// Test 3: CalculateHBPointWithHandlebar reproduces idmatch Saddle-Handlebar distance d=700mm and drop c=+13mm
	// In the report, BRP = (-219, 713) and HB = (481, 726)
	brpReport := model.BRPPoint{
		BrpX:         -219.0,
		BrpY:         713.0,
		BrpSetbackMm: 219.0,
	}
	barOffset, err := geometry.GetHandlebarOffset(bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	clampX := 481.0 - barOffset.X
	clampY := 726.0 - barOffset.Y

	dSteerer := model.DefaultHeadsetTopStackMm + model.StemThicknessMm/2.0
	reach := 400.0 + dSteerer*math.Cos(90.0*math.Pi/180.0) // 400
	stack := clampY - dSteerer*math.Sin(90.0*math.Pi/180.0)
	stem := model.StemSpec{LengthMm: clampX - 400.0, AngleDeg: 0.0}
	hb, err := geometry.CalculateHBPointWithHandlebar(reach, stack, 90.0, brpReport, stem, 0.0, bar, model.DefaultGripInsetMm, 0.0)
	if err != nil {
		t.Fatalf("CalculateHBPointWithHandlebar(90 deg): %v", err)
	}

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

func TestMapPossibleBRPs_And_SelectCheapestBRP(t *testing.T) {
	staDeg := model.Deg(71.5)
	seatHeight := 746.0
	refSaddle := model.ReferenceIdmatchSaddle()

	points, err := geometry.MapPossibleBRPs(refSaddle, staDeg, seatHeight, model.DefaultSeatpostSetbackMm, model.DefaultSeatpostClampLengthMm)
	if err != nil {
		t.Fatal(err)
	}
	// SQlab 610 with 30mm clamp: straight rails 60mm -> clamp window [-15, +15] in 1mm steps = 31 points
	if len(points) != 31 {
		t.Fatalf("Expected 31 candidate BRP points, got %d", len(points))
	}

	// 1. A flat cost leaves a tie, which goes to dead center (RailOffsetMm = 0)
	bestFlat, ok := geometry.SelectCheapestBRP(points, func(model.BRPPoint) float64 { return 0 })
	if !ok {
		t.Fatalf("SelectCheapestBRP returned false")
	}
	if math.Abs(bestFlat.RailOffsetMm) > 0.001 {
		t.Errorf("Expected 0 rail offset on a tie, got %.1f", bestFlat.RailOffsetMm)
	}

	// 2. At 71.5° with a zero-setback post, the target is inside the clamp window, near +13mm.
	toSetback := func(b model.BRPPoint) float64 { return math.Abs(b.BrpSetbackMm - 219.0) }
	bestTarget, ok := geometry.SelectCheapestBRP(points, toSetback)
	if !ok {
		t.Fatalf("SelectCheapestBRP returned false")
	}
	if math.Abs(bestTarget.BrpSetbackMm-219.0) > 0.5 {
		t.Errorf("Expected BrpSetbackMm ~ 219.0, got %.1f", bestTarget.BrpSetbackMm)
	}
	if math.Abs(bestTarget.RailOffsetMm-13.0) > 1.0 {
		t.Errorf("Expected RailOffsetMm near +13mm on 71.5° STA, got %.1f", bestTarget.RailOffsetMm)
	}

	// 3. Target 219mm on steep STA=75.0° requires more rearward travel than the +15mm limit.
	steepPoints, err := geometry.MapPossibleBRPs(refSaddle, 75.0, seatHeight, model.DefaultSeatpostSetbackMm, model.DefaultSeatpostClampLengthMm)
	if err != nil {
		t.Fatal(err)
	}
	bestClamped, ok := geometry.SelectCheapestBRP(steepPoints, toSetback)
	if !ok {
		t.Fatalf("SelectCheapestBRP returned false for steep points")
	}
	if bestClamped.RailOffsetMm != 15.0 {
		t.Errorf("Expected clamp to max +15mm rail offset, got %.1f", bestClamped.RailOffsetMm)
	}

	if _, ok := geometry.SelectCheapestBRP(nil, toSetback); ok {
		t.Error("an empty candidate list must report ok=false")
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

// ovalPoint returns a candidate whose hand grip sits r oval radii from the oval
// center along its minor axis.
func ovalPoint(stem model.StemSpec, spacerMm, r float64) model.HBPoint {
	cos30, sin30 := math.Cos(30.0*math.Pi/180.0), math.Sin(30.0*math.Pi/180.0)
	return model.HBPoint{Stem: stem, SpacerMm: spacerMm, HbX: 500 + r*10*cos30, HbY: 700 - r*10*sin30}
}

func TestEvaluateCockpitFitByCost(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	byR := func(_ model.HBPoint, r float64) float64 { return r }

	setups := []model.HBPoint{
		ovalPoint(model.StemSpec{LengthMm: 80, AngleDeg: -6}, 15, 0.5), // inside
		ovalPoint(model.StemSpec{LengthMm: 70, AngleDeg: -6}, 10, 0),   // R=0 (center)
		ovalPoint(model.StemSpec{LengthMm: 60, AngleDeg: -6}, 5, 0.8),  // inside
		ovalPoint(model.StemSpec{LengthMm: 90, AngleDeg: 0}, 0, 1.1),   // outside
	}
	res := geometry.EvaluateCockpitFitByCost(setups, oval, "", model.Opt[model.HandlebarSpec]{}, byR)
	if res.CountInside != 3 || !res.HasFit {
		t.Errorf("Expected 3 inside with a fit, got %d (HasFit=%v)", res.CountInside, res.HasFit)
	}
	if res.BestR != 0.0 || res.SelectedHB.Stem.LengthMm != 70 {
		t.Errorf("Expected the centered 70mm stem at R=0, got %.0fmm at R=%.3f", res.SelectedHB.Stem.LengthMm, res.BestR)
	}
	if res.TargetRef != geometry.TargetHandGrip {
		t.Errorf("Expected an empty target ref to default to HandGrip, got %q", res.TargetRef)
	}

	// Nothing inside: the cheapest candidate is still selected and the miss is its R.
	outside := []model.HBPoint{
		ovalPoint(model.StemSpec{LengthMm: 90, AngleDeg: 0}, 0, 1.6),
		ovalPoint(model.StemSpec{LengthMm: 100, AngleDeg: 0}, 0, 1.0002),
	}
	resZero := geometry.EvaluateCockpitFitByCost(outside, oval, "", model.Opt[model.HandlebarSpec]{}, byR)
	if resZero.CountInside != 0 || resZero.HasFit {
		t.Errorf("Expected 0 inside, got %d (HasFit=%v)", resZero.CountInside, resZero.HasFit)
	}
	if math.Abs(resZero.BestR-1.0002) > 1e-9 {
		t.Errorf("Expected unrounded BestR 1.0002 for a near miss, got %.6f", resZero.BestR)
	}

	// No candidates, or no cost: the empty result reports no fit.
	if r := geometry.EvaluateCockpitFitByCost(nil, oval, "", model.Opt[model.HandlebarSpec]{}, byR); r.HasFit || r.BestR != 999 {
		t.Errorf("Expected an empty result for no candidates, got %+v", r)
	}
}

// The cheapest candidate wins even when another sits closer to the oval center; on
// equal cost the candidate nearer the center wins, then the earlier one.
func TestEvaluateCockpitFitByCost_CheapestWins(t *testing.T) {
	oval := geometry.DefaultFitOval(500.0, 700.0)
	short := model.StemSpec{LengthMm: 80, AngleDeg: -6}
	long := model.StemSpec{LengthMm: 100, AngleDeg: -6}
	// The 100mm stem is closer to center (R=0.10) but the 80mm stem is slightly off center (R=0.25).
	candidates := []model.HBPoint{ovalPoint(long, 0, 0.10), ovalPoint(short, 10, 0.25)}
	stemCost := func(pt model.HBPoint, _ float64) float64 {
		if pt.Stem.LengthMm > 90 {
			return 15
		}
		return 0
	}
	res := geometry.EvaluateCockpitFitByCost(candidates, oval, "", model.Opt[model.HandlebarSpec]{}, stemCost)
	if res.SelectedHB.Stem.LengthMm != 80 || res.CountInside != 2 {
		t.Errorf("Expected the cheaper 80mm stem with 2 inside, got %.0fmm with %d inside", res.SelectedHB.Stem.LengthMm, res.CountInside)
	}

	flat := func(model.HBPoint, float64) float64 { return 3 }
	res = geometry.EvaluateCockpitFitByCost(candidates, oval, "", model.Opt[model.HandlebarSpec]{}, flat)
	if res.SelectedHB.Stem.LengthMm != 100 {
		t.Errorf("Expected a cost tie to go to the candidate nearer the center, got %.0fmm", res.SelectedHB.Stem.LengthMm)
	}
	res = geometry.EvaluateCockpitFitByCost([]model.HBPoint{candidates[0], candidates[0]}, oval, "", model.Opt[model.HandlebarSpec]{}, flat)
	if res.SelectedHB.Stem.LengthMm != 100 {
		t.Errorf("Expected identical candidates to keep the earlier one, got %.0fmm", res.SelectedHB.Stem.LengthMm)
	}
}

func TestTubeAxisHelpers(t *testing.T) {
	// SeatTubeAxis for STA 74° starting at origin (0,0)
	seatAxis := geometry.SeatTubeAxis(geom2d.Origin, 74.0)

	expectedX := -math.Cos(74.0 * math.Pi / 180.0)
	expectedY := math.Sin(74.0 * math.Pi / 180.0)

	if math.Abs(seatAxis.Direction.X-expectedX) > 1e-9 || math.Abs(seatAxis.Direction.Y-expectedY) > 1e-9 {
		t.Errorf("SeatTubeAxis(74).Direction = (%v, %v), expected (%v, %v)", seatAxis.Direction.X, seatAxis.Direction.Y, expectedX, expectedY)
	}

	// SteererTubeDir for HTA 68°
	steererDir := geometry.SteererTubeDir(68.0)

	expectedHtaX := -math.Cos(68.0 * math.Pi / 180.0)
	expectedHtaY := math.Sin(68.0 * math.Pi / 180.0)

	if math.Abs(steererDir.X-expectedHtaX) > 1e-9 || math.Abs(steererDir.Y-expectedHtaY) > 1e-9 {
		t.Errorf("SteererTubeDir(68) = (%v, %v), expected (%v, %v)", steererDir.X, steererDir.Y, expectedHtaX, expectedHtaY)
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

	// 2. SitBonePointFromBrp & BrpPointFromSitBones
	refBrp := sqlab.BrpPointAtHeightAndSetback(746.0, 219.0)
	sitBonePt := sqlab.SitBonePointFromBrp(refBrp)
	if math.Abs(sitBonePt.X-(-299.0)) > 0.1 || math.Abs(sitBonePt.Y-723.15) > 0.1 {
		t.Errorf("Expected SitBonePoint (-299.0, 723.15), got (%.2f, %.2f)", sitBonePt.X, sitBonePt.Y)
	}

	// SQlab (ref) -> Berthoud Aspin via SitBones
	aspinBrp := aspin.BrpPointFromSitBones(sitBonePt)
	if math.Abs(aspinBrp.X-(-211.0)) > 0.1 || math.Abs(aspinBrp.Y-723.15) > 0.1 {
		t.Errorf("Expected Aspin BRP (-211.0, 723.15), got (%.2f, %.2f)", aspinBrp.X, aspinBrp.Y)
	}
	shAspin := aspinBrp.DistanceToOrigin()
	if math.Abs(shAspin-753.29) > 0.1 {
		t.Errorf("Expected Aspin SH ~753.29mm, got %.2fmm", shAspin)
	}

	// SQlab (ref) -> SLR Boost via SitBones
	shSLR := slr.BrpPointFromSitBones(sitBonePt).DistanceToOrigin()
	if math.Abs(shSLR-754.70) > 0.1 {
		t.Errorf("Expected SLR Boost SH ~754.70mm, got %.2fmm", shSLR)
	}

	// 3. 2D Vector Translation (BrpToSitBonesVec)
	sqVec := sqlab.BrpToSitBonesVec()
	if sqVec.X != -80.0 || sqVec.Y != 10.0 {
		t.Errorf("Expected SQlab BrpToSitBonesVec (-80.0, 10.0), got (%.1f, %.1f)", sqVec.X, sqVec.Y)
	}
	aspinVec := aspin.BrpToSitBonesVec()
	if aspinVec.X != -88.0 || aspinVec.Y != 0.0 {
		t.Errorf("Expected Aspin BrpToSitBonesVec (-88.0, 0.0), got (%.1f, %.1f)", aspinVec.X, aspinVec.Y)
	}
}

// C4: Physical lower headset cup (HeadsetCupType.CupStackMm()) is applied in comparisons when it
// differs from the chart baseline (GeoHeadsetSpacerMm).
func TestAnalyzeBikeFit_NormalizesCadBaseline(t *testing.T) {
	baseBike := model.BikeGeometry{
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
		GeoHeadsetSpacerMm: 12.0,
		CadFork:            model.ForkRef{Fork: "lithic_495", A2cMm: 495.0, RakeMm: 54.0},
	}
	profile := catalog.DefaultRiderProfile()
	saddle := catalog.DefaultSaddle()

	// 1. When GeoHeadsetSpacerMm matches CupEC44.CupStackMm() (12mm), evaluating at the 495mm CAD fork
	// preserves the published CAD chart without spurious pitch from chart Stack rounding residuals.
	fitMatched, err := geometry.AnalyzeBikeFitWithSaddle(baseBike, model.Some(495.0), model.Opt[float64]{}, profile, saddle)
	if err != nil {
		t.Fatalf("AnalyzeBikeFitWithSaddle matched cup failed: %v", err)
	}
	if fitMatched.DeltaAngleDeg != 0 || fitMatched.ActiveReach != baseBike.Reach || fitMatched.ActiveStack != baseBike.Stack ||
		fitMatched.ActiveHta != baseBike.HtaDeg || fitMatched.ActiveBbDrop != baseBike.BbDrop {
		t.Errorf("CAD fork evaluation with matched cup should preserve published CAD chart: got Delta=%.2f Reach=%.1f Stack=%.1f HTA=%.2f BBD=%.1f",
			fitMatched.DeltaAngleDeg, fitMatched.ActiveReach, fitMatched.ActiveStack, fitMatched.ActiveHta, fitMatched.ActiveBbDrop)
	}

	// 2. When GeoHeadsetSpacerMm is 0mm while HeadsetCupType is CupEC44 (12mm), evaluating at 495mm
	// pitches the frame by the +12mm physical lower cup difference and matches BuildScene to < 1e-9.
	cupDiffBike := baseBike
	cupDiffBike.GeoHeadsetSpacerMm = 0.0
	fitDiff, err := geometry.AnalyzeBikeFitWithSaddle(cupDiffBike, model.Some(495.0), model.Opt[float64]{}, profile, saddle)
	if err != nil {
		t.Fatalf("AnalyzeBikeFitWithSaddle physical cup failed: %v", err)
	}
	if math.Abs(fitDiff.StructuralA2c-507.0) > 1e-9 {
		t.Errorf("expected StructuralA2c 507.0mm (495 + 12mm EC44), got %.6f", fitDiff.StructuralA2c)
	}
	if fitDiff.DeltaAngleDeg <= 0.5 || !(fitDiff.ActiveHta < cupDiffBike.HtaDeg) ||
		!(fitDiff.ActiveReach < cupDiffBike.Reach) || !(fitDiff.ActiveStack > cupDiffBike.Stack) {
		t.Errorf("expected +12mm EC44 cup to pitch nose up: Delta=%.3f HTA=%.3f Reach=%.2f Stack=%.2f",
			fitDiff.DeltaAngleDeg, fitDiff.ActiveHta, fitDiff.ActiveReach, fitDiff.ActiveStack)
	}

	frame, err := geometry.FrameSizeFromBikeGeometry(cupDiffBike)
	if err != nil {
		t.Fatalf("FrameSizeFromBikeGeometry: %v", err)
	}
	bar := profile.Handlebar
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	scene, err := geometry.BuildScene(
		frame,
		geometry.BuildSpec{
			Fork:               frame.CadFork,
			SagPct:             0,
			PhysicalLowerCupMm: 12.0,
			WheelRadiusFrontMm: cupDiffBike.WheelRadiusMm(),
			WheelRadiusRearMm:  cupDiffBike.WheelRadiusMm(),
		},
		geometry.CockpitSpec{
			HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
			SpacerMm:          0.0,
			Stem:              model.StemSpec{LengthMm: 70.0},
			StemClampHeightMm: model.StemThicknessMm,
			Handlebar:         bar,
			GripInsetMm:       model.DefaultGripInsetMm,
		},
		geometry.SeatSpec{
			PostSetbackMm: model.DefaultSeatpostSetbackMm,
			ClampLengthMm: model.DefaultSeatpostClampLengthMm,
			Saddle:        saddle,
		},
		geometry.RiderFit{
			FrameSeatAxisHeightMm: profile.SaddleHeightMm,
			BrpHeightMm:           profile.SaddleHeightMm,
			TargetSitBonesXMm:     -292.0,
			Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480.9, Y: 726.1}, Ref: geometry.TargetHandGrip},
			ReferenceSaddle:       saddle,
		},
	)
	if err != nil {
		t.Fatalf("BuildScene: %v", err)
	}
	if d := math.Abs(fitDiff.ActiveReach - scene.Reach()); d > 1e-9 {
		t.Errorf("Reach diff vs BuildScene: %.12f", d)
	}
	if d := math.Abs(fitDiff.ActiveStack - scene.Stack()); d > 1e-9 {
		t.Errorf("Stack diff vs BuildScene: %.12f", d)
	}
	if d := (fitDiff.ActiveHta - scene.HtaDeg).Abs(); d > 1e-9 {
		t.Errorf("HTA diff vs BuildScene: %.12f", d)
	}
	if d := math.Abs(fitDiff.ActiveBbDrop - scene.BbDrop()); d > 1e-9 {
		t.Errorf("BbDrop diff vs BuildScene: %.12f", d)
	}
	if d := (fitDiff.DeltaAngleDeg - scene.PitchDeg).Abs(); d > 1e-9 {
		t.Errorf("DeltaAngleDeg diff vs BuildScene: %.12f", d)
	}
}
