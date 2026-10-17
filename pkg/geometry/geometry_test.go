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
		SeatHeight:     780.0,
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
	fit := geometry.AnalyzeBikeFit(
		binaryHavok,
		&saggedFork,
		&binaryHavok.SeatHeight,
	)

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
