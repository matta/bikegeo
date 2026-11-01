package geometry_test

import (
	"math"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func TestSaddleSpec_Methods(t *testing.T) {
	sqlab, ok := catalog.GetSaddle("sqlab-610-ergolux")
	if !ok {
		t.Fatalf("Failed to find sqlab-610-ergolux in catalog")
	}

	aspin, ok := catalog.GetSaddle("berthoud-aspin-open")
	if !ok {
		t.Fatalf("Failed to find berthoud-aspin-open in catalog")
	}

	// 1. DeltaBrpToSitBonesMm
	if math.Abs(sqlab.DeltaBrpToSitBonesMm()-73.0) > 0.001 {
		t.Errorf("Expected SQlab DeltaBrpToSitBonesMm 73.0, got %.1f", sqlab.DeltaBrpToSitBonesMm())
	}
	if math.Abs(aspin.DeltaBrpToSitBonesMm()-88.0) > 0.001 {
		t.Errorf("Expected Aspin DeltaBrpToSitBonesMm 88.0, got %.1f", aspin.DeltaBrpToSitBonesMm())
	}

	// 2. SitBoneSetbackFromBrp & BrpSetbackFromSitBones roundtrip
	brpSetback := 219.0
	sbSetback := sqlab.SitBoneSetbackFromBrp(brpSetback)
	if math.Abs(sbSetback-292.0) > 0.001 {
		t.Errorf("Expected SitBoneSetback 292.0, got %.1f", sbSetback)
	}
	if math.Abs(sqlab.BrpSetbackFromSitBones(sbSetback)-brpSetback) > 0.001 {
		t.Errorf("Expected BrpSetback roundtrip 219.0, got %.1f", sqlab.BrpSetbackFromSitBones(sbSetback))
	}

	// 3. RailCenterSetbackFromBrp
	rcSetback := sqlab.RailCenterSetbackFromBrp(brpSetback)
	if math.Abs(rcSetback-242.0) > 0.001 {
		t.Errorf("Expected RailCenterSetback 242.0, got %.1f", rcSetback)
	}

	// 4. Clamp Windows with 45mm clamp (Drop Best) and 30mm clamp (standard)
	// SQlab 60mm rail (-30 to +30): nominal clamp center is [-7.5, +7.5]
	minNom, maxNom := sqlab.NominalClampWindowMm(model.DropBestClampLengthMm)
	if minNom != -7.5 || maxNom != 7.5 {
		t.Errorf("Expected SQlab 45mm nominal clamp window [-7.5, +7.5], got [%.1f, %.1f]", minNom, maxNom)
	}
	// SQlab physical clamp center includes 7.5mm extended rear rail: [-15.0, +7.5]
	minPhys, maxPhys := sqlab.PhysicalClampWindowMm(model.DropBestClampLengthMm)
	if minPhys != -15.0 || maxPhys != 7.5 {
		t.Errorf("Expected SQlab 45mm physical clamp window [-15.0, +7.5], got [%.1f, %.1f]", minPhys, maxPhys)
	}

	// Berthoud Aspin 64mm rail: nominal = physical = [-9.5, +9.5] with 45mm clamp
	aspinMin, aspinMax := aspin.PhysicalClampWindowMm(model.DropBestClampLengthMm)
	if aspinMin != -9.5 || aspinMax != 9.5 {
		t.Errorf("Expected Aspin 45mm physical clamp window [-9.5, +9.5], got [%.1f, %.1f]", aspinMin, aspinMax)
	}
}

func TestTranslateSaddleFit_SQlabToAspin(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")

	// Current setup: SQlab clamped at -15.0mm (Drop Best 45mm clamp)
	res := geometry.TranslateSaddleFit(sqlab, -15.0, aspin, model.DropBestClampLengthMm)

	// Difference in rail-center-to-sit-bones is +5.5mm
	if math.Abs(res.SitBoneDeltaMm-5.5) > 0.001 {
		t.Errorf("Expected SitBoneDeltaMm 5.5, got %.1f", res.SitBoneDeltaMm)
	}

	// Target clamp position on Aspin is -15.0 + 5.5 = -9.5mm
	if math.Abs(res.TargetClampOffset-(-9.5)) > 0.001 {
		t.Errorf("Expected TargetClampOffset -9.5, got %.1f", res.TargetClampOffset)
	}

	// -9.5mm falls exactly on the Aspin's rear mechanical stop
	if !res.IsPhysicallyFeasible {
		t.Errorf("Expected translation to be physically feasible, got false")
	}

	// BRP delta is 88 - 73 = 15.0mm
	if math.Abs(res.BrpDeltaMm-15.0) > 0.001 {
		t.Errorf("Expected BrpDeltaMm 15.0, got %.1f", res.BrpDeltaMm)
	}
}

func TestCalculateIdealSeatTubeAngle(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	targetSitBoneSetback := 292.0
	saddleHeight := 746.0

	// 1. Redshift + Drop Best sagged (39mm hardware setback)
	// Fenrir L has STA ~ 74.53°
	resFenrir := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 39.0, "Redshift + Drop Best (sagged)", model.DropBestClampLengthMm)
	if math.Abs(resFenrir.IdealStaDeg-74.5) > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 74.5°, got %.2f°", resFenrir.IdealStaDeg)
	}

	// 2. Standard 15mm setback post
	res15 := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 15.0, "15mm Setback", model.DefaultSeatpostClampLengthMm)
	if math.Abs(res15.IdealStaDeg-72.3) > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 72.3° for 15mm post, got %.2f°", res15.IdealStaDeg)
	}

	// 3. Standard 25mm setback post
	res25 := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 25.0, "25mm Setback", model.DefaultSeatpostClampLengthMm)
	if math.Abs(res25.IdealStaDeg-73.1) > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 73.1° for 25mm post, got %.2f°", res25.IdealStaDeg)
	}

	// 4. Redshift suspension post sagged (11mm setback)
	resRedshiftSag := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 11.0, "Redshift ShockStop (sagged)", model.DefaultSeatpostClampLengthMm)
	if math.Abs(resRedshiftSag.IdealStaDeg-72.0) > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 72.0° for Redshift sagged, got %.2f°", resRedshiftSag.IdealStaDeg)
	}
}

func TestCalculateUniversalCompromiseSTA(t *testing.T) {
	saddles := catalog.AllSaddles()
	targetSitBoneSetback := 292.0
	saddleHeight := 746.0

	// 1. Standard 15mm setback post across all catalog saddles
	res15 := geometry.CalculateUniversalCompromiseSTA(saddles, targetSitBoneSetback, saddleHeight, 15.0, "Standard 15mm Setback", model.DefaultSeatpostClampLengthMm)
	if math.Abs(res15.CompromiseStaDeg-72.1) > 0.15 {
		t.Errorf("Expected CompromiseStaDeg ~ 72.1°, got %.2f°", res15.CompromiseStaDeg)
	}
	if math.Abs(res15.MaxRailOffsetMm-2.8) > 0.15 {
		t.Errorf("Expected MaxRailOffsetMm ~ 2.8mm, got %.2fmm", res15.MaxRailOffsetMm)
	}
	if !res15.HasUniversalMargin {
		t.Errorf("Expected HasUniversalMargin to be true")
	}
	// Universal window should be roughly 71.9° - 72.2°
	if res15.UniversalMarginMinSta > 72.0 || res15.UniversalMarginMaxSta < 72.1 {
		t.Errorf("Unexpected universal margin range: %.1f°–%.1f°", res15.UniversalMarginMinSta, res15.UniversalMarginMaxSta)
	}
	if len(res15.SaddleOffsets) != 2 {
		t.Fatalf("Expected 2 saddle offsets, got %d", len(res15.SaddleOffsets))
	}
	// Offsets should be roughly -2.8mm (SQlab) and +2.8mm (Aspin)
	for _, so := range res15.SaddleOffsets {
		if math.Abs(math.Abs(so.OffsetMm)-2.8) > 0.15 {
			t.Errorf("Expected offset magnitude ~ 2.8mm for %s, got %.1f", so.SaddleID, so.OffsetMm)
		}
	}

	// 2. Redshift + Drop Best sagged (39mm hardware setback)
	res39 := geometry.CalculateUniversalCompromiseSTA(saddles, targetSitBoneSetback, saddleHeight, 39.0, "Redshift + Drop Best (sagged 39mm)", model.DropBestClampLengthMm)
	if math.Abs(res39.CompromiseStaDeg-74.0) > 0.15 {
		t.Errorf("Expected CompromiseStaDeg ~ 74.0° for 39mm hardware setback, got %.2f°", res39.CompromiseStaDeg)
	}
	if math.Abs(res39.MaxRailOffsetMm-2.8) > 0.15 {
		t.Errorf("Expected MaxRailOffsetMm ~ 2.8mm, got %.2fmm", res39.MaxRailOffsetMm)
	}
}
