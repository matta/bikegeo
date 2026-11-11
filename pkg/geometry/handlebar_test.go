package geometry_test

import (
	"math"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func TestWorkedComparisonTable(t *testing.T) {
	// Evaluates the exact Section 5 worked comparison table at roll = 0°, d_grip = 65mm.
	type testCase struct {
		name          string
		spec          model.HandlebarSpec
		expectedZHand float64
		expectedZb    float64
		expectedLz    float64
		expectedDX    float64 // Reach offset (mm)
		expectedDY    float64 // Stack offset (mm)
	}

	tests := []testCase{
		{
			name:          "Tumbleweed Persuader 20° (800mm)",
			spec:          catalog.HandlebarPresets["persuader20_800"],
			expectedZHand: 335.0,
			expectedZb:    135.0,
			expectedLz:    200.0,
			expectedDX:    -72.8,
			expectedDY:    45.5,
		},
		{
			name:          "Tumbleweed Persuader 20° (840mm)",
			spec:          catalog.HandlebarPresets["persuader20_840"],
			expectedZHand: 355.0,
			expectedZb:    135.0,
			expectedLz:    220.0,
			expectedDX:    -80.1,
			expectedDY:    45.5,
		},
		{
			name:          "SQlab 3OX 16° (30mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab3OX_16_30"],
			expectedZHand: 325.0,
			expectedZb:    82.0,
			expectedLz:    243.0,
			expectedDX:    -69.7,
			expectedDY:    47.0,
		},
		{
			name:          "SQlab 3OX 16° (45mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab3OX_16_45"],
			expectedZHand: 325.0,
			expectedZb:    95.5,
			expectedLz:    229.5,
			expectedDX:    -65.8,
			expectedDY:    61.0,
		},
		{
			name:          "SQlab 3OX 12° (30mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab3OX_12_30"],
			expectedZHand: 325.0,
			expectedZb:    82.0,
			expectedLz:    243.0,
			expectedDX:    -51.7,
			expectedDY:    47.0,
		},
		{
			name:          "SQlab 311 2.0 Alloy 16° (25mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab311_20_16_25"],
			expectedZHand: 305.0,
			expectedZb:    77.5,
			expectedLz:    227.5,
			expectedDX:    -65.2,
			expectedDY:    44.9,
		},
		{
			name:          "SQlab 311 2.0 Alloy 16° (50mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab311_20_16_50"],
			expectedZHand: 305.0,
			expectedZb:    100.0,
			expectedLz:    205.0,
			expectedDX:    -58.8,
			expectedDY:    67.9,
		},
		{
			name:          "SQlab 311 FL-X Carbon 16° (30mm Rise)",
			spec:          catalog.HandlebarPresets["sqlab311_FLX_16_30"],
			expectedZHand: 305.0,
			expectedZb:    82.0,
			expectedLz:    223.0,
			expectedDX:    -63.9,
			expectedDY:    45.6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			zHand := tc.spec.HandLateralMm(65.0)
			if math.Abs(zHand-tc.expectedZHand) > 0.1 {
				t.Errorf("zHand: expected %.1f, got %.1f", tc.expectedZHand, zHand)
			}

			zb := tc.spec.EffectiveBendStartMm
			if math.Abs(zb-tc.expectedZb) > 0.1 {
				t.Errorf("zb: expected %.1f, got %.1f", tc.expectedZb, zb)
			}

			lz := zHand - zb
			if math.Abs(lz-tc.expectedLz) > 0.1 {
				t.Errorf("lz: expected %.1f, got %.1f", tc.expectedLz, lz)
			}

			offset := geometry.GetHandlebarOffset(tc.spec, 65.0, 0.0)

			if math.Abs(offset.X-tc.expectedDX) > 0.1 {
				t.Errorf("Reach offset ΔX: expected %.1f mm, got %.2f mm", tc.expectedDX, offset.X)
			}
			if math.Abs(offset.Y-tc.expectedDY) > 0.1 {
				t.Errorf("Stack offset ΔY: expected %.1f mm, got %.2f mm", tc.expectedDY, offset.Y)
			}
			if math.Abs(offset.Z-tc.expectedZHand) > 0.1 {
				t.Errorf("Lateral offset ΔZ: expected %.1f mm, got %.2f mm", tc.expectedZHand, offset.Z)
			}
		})
	}
}

func TestKeyInsights(t *testing.T) {
	gripInset := 65.0

	// Insight 1: Persuader 20° (800mm) vs SQlab 3OX 16° (30mm Rise)
	// Hands are pulled back within 3.1mm and vertical stack is within 1.5mm.
	persuader800 := catalog.HandlebarPresets["persuader20_800"]
	sqlab3OX30 := catalog.HandlebarPresets["sqlab3OX_16_30"]

	offP := geometry.GetHandlebarOffset(persuader800, gripInset, 0.0)
	offS := geometry.GetHandlebarOffset(sqlab3OX30, gripInset, 0.0)

	reachDelta := offP.X - offS.X // -72.8 - (-69.7) = -3.1mm
	stackDelta := offP.Y - offS.Y // 45.5 - 47.0 = -1.5mm

	if math.Abs(reachDelta-(-3.1)) > 0.15 {
		t.Errorf("Expected reach delta ~ -3.1mm, got %.2fmm", reachDelta)
	}
	if math.Abs(stackDelta-(-1.5)) > 0.15 {
		t.Errorf("Expected stack delta ~ -1.5mm, got %.2fmm", stackDelta)
	}

	// Insight 2: SQlab 311 2.0 25mm vs 50mm rise
	// Stepping up from 25mm to 50mm rise gives +6.4mm forward reach alongside +23.0mm stack increase.
	sqlab311_25 := catalog.HandlebarPresets["sqlab311_20_16_25"]
	sqlab311_50 := catalog.HandlebarPresets["sqlab311_20_16_50"]

	off25 := geometry.GetHandlebarOffset(sqlab311_25, gripInset, 0.0)
	off50 := geometry.GetHandlebarOffset(sqlab311_50, gripInset, 0.0)

	reachGain := off50.X - off25.X // -58.8 - (-65.2) = +6.4mm
	stackGain := off50.Y - off25.Y // 67.9 - 44.9 = +23.0mm

	if math.Abs(reachGain-6.4) > 0.15 {
		t.Errorf("Expected reach gain ~ +6.4mm, got %.2fmm", reachGain)
	}
	if math.Abs(stackGain-23.0) > 0.15 {
		t.Errorf("Expected stack gain ~ +23.0mm, got %.2fmm", stackGain)
	}

	// Insight 3: Bar Roll Sensitivity
	// At ~70mm backsweep offset and ~46mm stack offset, every 5° of backward roll (-5°)
	// shifts grips ~ -4.0mm further back (reach) and ~ -6.1mm lower (stack).
	offNeutral := geometry.GetHandlebarOffset(sqlab3OX30, gripInset, 0.0)
	offRolled := geometry.GetHandlebarOffset(sqlab3OX30, gripInset, -5.0)

	deltaRollReach := offRolled.X - offNeutral.X
	deltaRollStack := offRolled.Y - offNeutral.Y

	if math.Abs(deltaRollReach-(-4.0)) > 0.3 {
		t.Errorf("Expected backward roll reach shift ~ -4.0mm, got %.2fmm", deltaRollReach)
	}
	if math.Abs(deltaRollStack-(-6.1)) > 0.2 {
		t.Errorf("Expected backward roll stack shift ~ -6.1mm, got %.2fmm", deltaRollStack)
	}
}

func TestForwardAndInverseTransforms(t *testing.T) {
	spec := catalog.HandlebarPresets["sqlab3OX_16_30"]
	gripInset := 65.0
	rollDeg := 3.5

	hbOrigin := model.Point3D{X: 450.0, Y: 620.0, Z: 0.0}

	// Forward transform: HB -> Grip
	grip := geometry.HBToGrip(hbOrigin, spec, gripInset, rollDeg)

	offset := geometry.GetHandlebarOffset(spec, gripInset, rollDeg)
	if math.Abs(grip.X-(hbOrigin.X+offset.X)) > 1e-6 {
		t.Errorf("Forward transform X mismatch: got %.3f", grip.X)
	}
	if math.Abs(grip.Y-(hbOrigin.Y+offset.Y)) > 1e-6 {
		t.Errorf("Forward transform Y mismatch: got %.3f", grip.Y)
	}
	if math.Abs(grip.Z-offset.Z) > 1e-6 {
		t.Errorf("Forward transform Z mismatch: got %.3f", grip.Z)
	}

	// Inverse transform: Grip -> HB
	recoveredHB := geometry.GripToHB(grip, spec, gripInset, rollDeg)

	if math.Abs(recoveredHB.X-hbOrigin.X) > 1e-6 {
		t.Errorf("Inverse transform X mismatch: expected %.3f, got %.3f", hbOrigin.X, recoveredHB.X)
	}
	if math.Abs(recoveredHB.Y-hbOrigin.Y) > 1e-6 {
		t.Errorf("Inverse transform Y mismatch: expected %.3f, got %.3f", hbOrigin.Y, recoveredHB.Y)
	}
	if recoveredHB.Z != 0.0 {
		t.Errorf("Inverse transform Z must be 0.0, got %.3f", recoveredHB.Z)
	}
}

func TestCutHandlebarWidth(t *testing.T) {
	// Trimming an 800mm Persuader down to 760mm reduces zHand from 335mm to 315mm,
	// reducing swept run Lz from 200mm to 180mm.
	// Reach loss becomes -180 * tan(20°) = -65.51mm (vs -72.79mm uncut).
	// Base rise y_k remains fixed by the stock 800mm width manufacturing reference!
	uncut := catalog.HandlebarPresets["persuader20_800"]
	cut := uncut
	cut.CurrentWidthMm = 760.0

	offUncut := geometry.GetHandlebarOffset(uncut, 65.0, 0.0)
	offCut := geometry.GetHandlebarOffset(cut, 65.0, 0.0)

	expectedCutDX := -180.0 * math.Tan(20.0*math.Pi/180.0)
	if math.Abs(offCut.X-expectedCutDX) > 0.05 {
		t.Errorf("Expected cut reach offset %.2f mm, got %.2f mm", expectedCutDX, offCut.X)
	}

	// Trimming brings grips forward by ~7.3mm
	forwardShift := offCut.X - offUncut.X
	if math.Abs(forwardShift-7.28) > 0.1 {
		t.Errorf("Expected forward shift ~ +7.28mm, got %.2fmm", forwardShift)
	}

	// Stack delta on Persuader when cut:
	// y_k is 31.47mm. With Lz = 180mm: dy = 31.47 + 180 * tan(4°) = 44.06mm (reduction of 1.40mm)
	stackShift := offCut.Y - offUncut.Y
	if math.Abs(stackShift-(-1.40)) > 0.05 {
		t.Errorf("Expected stack shift ~ -1.40mm, got %.2fmm", stackShift)
	}
}

func TestHandlebarStemCompensation(t *testing.T) {
	persuader := catalog.HandlebarPresets["persuader20_800"]
	sqlab := catalog.HandlebarPresets["sqlab3OX_16_30"]

	// Moving from Persuader to SQlab:
	// Persuader offset is (-72.8, +45.5). SQlab is (-69.7, +47.0).
	// With clamp fixed, grips move +3.1mm forward (X) and +1.5mm upward (Y).
	delta := geometry.HandlebarDelta(persuader, sqlab, 65.0, 0.0)
	if math.Abs(delta.X-3.1) > 0.15 {
		t.Errorf("Expected grip shift X ~ +3.1mm, got %.2fmm", delta.X)
	}
	if math.Abs(delta.Y-1.5) > 0.15 {
		t.Errorf("Expected grip shift Y ~ +1.5mm, got %.2fmm", delta.Y)
	}

	// To keep hands in the EXACT same spatial location, the stem clamp must move in the opposite direction:
	comp := geometry.HandlebarStemCompensation(persuader, sqlab, 65.0, 0.0)
	if math.Abs(comp.X-(-3.1)) > 0.15 {
		t.Errorf("Expected stem comp X ~ -3.1mm, got %.2fmm", comp.X)
	}
	if math.Abs(comp.Y-(-1.5)) > 0.15 {
		t.Errorf("Expected stem comp Y ~ -1.5mm, got %.2fmm", comp.Y)
	}
}

func TestCalculateHBPointWithHandlebar(t *testing.T) {
	reach := 450.0
	stack := 620.0
	htaDeg := 73.0
	brp := model.BRPPoint{
		BrpX:         -219.0,
		BrpY:         713.0,
		BrpSetbackMm: 219.0,
	}
	stem := model.StemSpec{LengthMm: 70.0, AngleDeg: 0.0}
	spacerMm := 15.0

	bar := catalog.HandlebarPresets["sqlab3OX_16_30"]
	hb := geometry.CalculateHBPointWithHandlebar(reach, stack, htaDeg, brp, stem, spacerMm, bar, 65.0, 0.0)

	if hb.HbX <= 0 || hb.HbY <= 0 {
		t.Fatalf("Expected positive HbX and HbY, got X=%.1f, Y=%.1f", hb.HbX, hb.HbY)
	}
	if hb.SaddleHbMm <= 0 {
		t.Fatalf("Expected positive SaddleHbMm, got %.1f", hb.SaddleHbMm)
	}

	// Verify that HbX = ClampX + offset.X and HbY = ClampY + offset.Y
	offset := geometry.GetHandlebarOffset(bar, 65.0, 0.0)
	expectedHbX := math.Round((hb.ClampX+offset.X)*10) / 10
	expectedHbY := math.Round((hb.ClampY+offset.Y)*10) / 10

	if math.Abs(hb.HbX-expectedHbX) > 0.1 {
		t.Errorf("HbX mismatch: expected %.1f, got %.1f", expectedHbX, hb.HbX)
	}
	if math.Abs(hb.HbY-expectedHbY) > 0.1 {
		t.Errorf("HbY mismatch: expected %.1f, got %.1f", expectedHbY, hb.HbY)
	}
}

