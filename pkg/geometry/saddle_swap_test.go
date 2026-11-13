package geometry

import (
	"math"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/model"
)

func TestAnalyzeSaddleSwap_SQlabToBerthoudAspin(t *testing.T) {
	sqlab, ok := catalog.GetSaddle("sqlab-610-ergolux")
	if !ok {
		t.Fatalf("Failed to find sqlab-610-ergolux")
	}
	aspin, ok := catalog.GetSaddle("berthoud-aspin-open")
	if !ok {
		t.Fatalf("Failed to find berthoud-aspin-open")
	}

	report := AnalyzeSaddleSwap(
		sqlab,
		aspin,
		746.0,
		292.0,
		model.DefaultSeatpostSetbackMm,
		model.DefaultSeatpostClampLengthMm,
		700.0,
		13.0,
	)

	// 1. Check dimensional deltas (BRP skew vs true pelvic delta and 2D step)
	if report.BrpToSitBonesSpanDeltaMm != 15.0 {
		t.Errorf("Expected BRP-to-sit-bones span delta 15.0mm, got %.1fmm", report.BrpToSitBonesSpanDeltaMm)
	}
	if report.RailCenterToBrpDeltaMm != 20.5 {
		t.Errorf("Expected RailCenter-to-BRP delta 20.5mm, got %.1fmm", report.RailCenterToBrpDeltaMm)
	}
	if report.RailCenterToSitBonesDeltaMm != 5.5 {
		t.Errorf("Expected RailCenter-to-sit-bones delta 5.5mm, got %.1fmm", report.RailCenterToSitBonesDeltaMm)
	}
	if report.SourceStepElevationMm != 10.0 {
		t.Errorf("Expected SQlab source step elevation 10.0mm, got %.1fmm", report.SourceStepElevationMm)
	}
	if report.TargetStepElevationMm != 0.0 {
		t.Errorf("Expected Berthoud target step elevation 0.0mm, got %.1fmm", report.TargetStepElevationMm)
	}
	if report.StepElevationDeltaMm != -10.0 {
		t.Errorf("Expected step elevation delta -10.0mm, got %.1fmm", report.StepElevationDeltaMm)
	}

	// 2. Naive BRP Alignment outcomes
	naive := report.Naive
	if naive.TargetBrpSetbackMm != 219.0 {
		t.Errorf("Expected naive target BRP setback 219.0mm, got %.1fmm", naive.TargetBrpSetbackMm)
	}
	if naive.AchievedSitBoneMm != 307.0 {
		t.Errorf("Expected naive achieved sit bone setback 307.0mm, got %.1fmm", naive.AchievedSitBoneMm)
	}
	if naive.SitBoneDeltaMm != 15.0 {
		t.Errorf("Expected naive sit bone error +15.0mm, got %.1fmm", naive.SitBoneDeltaMm)
	}
	if naive.SitBoneDeltaYMm != -10.0 {
		t.Errorf("Expected naive sit bone vertical error -10.0mm (too low), got %.1fmm", naive.SitBoneDeltaYMm)
	}
	if math.Abs(naive.CockpitDeltaMm-15.0) > 0.5 {
		t.Errorf("Expected naive cockpit reach increase ~15.0mm, got %.1fmm", naive.CockpitDeltaMm)
	}
	if math.Abs(naive.LegExtensionDeltaMm-(-3.5)) > 0.5 {
		t.Errorf("Expected naive leg extension drop ~-3.5mm, got %.1fmm", naive.LegExtensionDeltaMm)
	}
	if naive.RequiredClampOffsetMm != -20.5 {
		t.Errorf("Expected naive clamp offset -20.5mm, got %.1fmm", naive.RequiredClampOffsetMm)
	}
	if naive.IsFeasible {
		t.Errorf("Expected naive BRP clamp offset to be marked INFEASIBLE on Berthoud rails, but got feasible")
	}

	// 3. True Pelvic Alignment outcomes (2D coordinates)
	pelvic := report.Pelvic
	if pelvic.TargetBrpSetbackMm != 204.0 {
		t.Errorf("Expected pelvic target BRP setback 204.0mm, got %.1fmm", pelvic.TargetBrpSetbackMm)
	}
	if report.TargetEquivalentBrpSetbackMm != 204.0 {
		t.Errorf("Expected report.TargetEquivalentBrpSetbackMm 204.0mm, got %.1fmm", report.TargetEquivalentBrpSetbackMm)
	}
	if report.BrpSetbackShiftMm != -15.0 {
		t.Errorf("Expected report.BrpSetbackShiftMm -15.0mm (forward shift), got %.1fmm", report.BrpSetbackShiftMm)
	}
	if report.BrpDeltaXMm != 15.0 {
		t.Errorf("Expected report.BrpDeltaXMm +15.0mm, got %.1fmm", report.BrpDeltaXMm)
	}
	if report.BrpDeltaYMm != 10.0 {
		t.Errorf("Expected report.BrpDeltaYMm +10.0mm (upward shift to compensate for SQlab step), got %.1fmm", report.BrpDeltaYMm)
	}
	if math.Abs(report.TargetEquivalentSaddleHeightMm-751.4) > 0.2 {
		t.Errorf("Expected target equivalent saddle height ~ 751.4mm, got %.1fmm", report.TargetEquivalentSaddleHeightMm)
	}
	if math.Abs(report.BrpSaddleHeightShiftMm-5.4) > 0.2 {
		t.Errorf("Expected target BRP saddle height shift ~ +5.4mm, got %.1fmm", report.BrpSaddleHeightShiftMm)
	}
	if (report.BrpDeltaAngleDeg - 1.31).Abs() > 0.05 {
		t.Errorf("Expected report.BrpDeltaAngleDeg ~ +1.31°, got %.2f°", report.BrpDeltaAngleDeg)
	}
	if pelvic.AchievedSitBoneMm != 292.0 {
		t.Errorf("Expected pelvic achieved sit bone setback 292.0mm, got %.1fmm", pelvic.AchievedSitBoneMm)
	}
	if pelvic.AchievedSitBoneHeightMm != 723.1 {
		t.Errorf("Expected pelvic achieved sit bone height 723.1mm, got %.1fmm", pelvic.AchievedSitBoneHeightMm)
	}
	if pelvic.SitBoneDeltaMm != 0.0 {
		t.Errorf("Expected pelvic sit bone setback error 0.0mm, got %.1fmm", pelvic.SitBoneDeltaMm)
	}
	if pelvic.SitBoneDeltaYMm != 0.0 {
		t.Errorf("Expected pelvic sit bone height error 0.0mm, got %.1fmm", pelvic.SitBoneDeltaYMm)
	}
	if pelvic.CockpitDeltaMm != 0.0 {
		t.Errorf("Expected pelvic cockpit reach error 0.0mm, got %.1fmm", pelvic.CockpitDeltaMm)
	}
	if pelvic.LegExtensionDeltaMm != 0.0 {
		t.Errorf("Expected pelvic leg extension error 0.0mm, got %.1fmm", pelvic.LegExtensionDeltaMm)
	}
	if pelvic.RequiredClampOffsetMm != 5.5 {
		t.Errorf("Expected pelvic clamp offset +5.5mm, got %.1fmm", pelvic.RequiredClampOffsetMm)
	}
	if !pelvic.IsFeasible {
		t.Errorf("Expected pelvic clamp offset to be marked FEASIBLE on Berthoud rails, got infeasible")
	}

	// 4. Hardware Rows
	if len(report.SeatpostRows) != 7 {
		t.Fatalf("Expected 7 seatpost hardware rows, got %d", len(report.SeatpostRows))
	}
	// For sagged Redshift post (11mm), verify feasibility
	saggedRow := report.SeatpostRows[2]
	if saggedRow.PostSetbackMm != 11.0 {
		t.Errorf("Expected row 2 to be 11mm post, got %.1fmm", saggedRow.PostSetbackMm)
	}
	if !saggedRow.IsFeasible30Mm {
		t.Errorf("Expected 30mm clamp to be feasible for Aspin pelvic fit on 11mm post")
	}
	if (saggedRow.StaDeltaDeg - (-0.24)).Abs() > 0.05 {
		t.Errorf("Expected STA delta ~ -0.24°, got %.2f°", saggedRow.StaDeltaDeg)
	}
}

func TestAnalyzeSaddleSwap_SQlabToSLRBoost(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	report := AnalyzeSaddleSwap(
		sqlab,
		slr,
		746.0,
		292.0,
		model.DefaultSeatpostSetbackMm,
		model.DefaultSeatpostClampLengthMm,
		700.0,
		13.0,
	)

	// SLR Boost sit bones are 15.3mm further rearward (-65.3 vs -50.0)
	if report.RailCenterToSitBonesDeltaMm != -15.3 {
		t.Errorf("Expected RailCenter-to-sit-bones delta -15.3mm, got %.1fmm", report.RailCenterToSitBonesDeltaMm)
	}
	// BRP span is 83mm vs 73mm (+10mm)
	if report.BrpToSitBonesSpanDeltaMm != 10.0 {
		t.Errorf("Expected BRP span delta 10.0mm, got %.1fmm", report.BrpToSitBonesSpanDeltaMm)
	}
	// 40mm straight rail with 30mm clamp allows [-5.0, +5.0] window
	// Pelvic clamp offset is -15.3mm, which exceeds [-5, +5] on 11mm post
	if report.Pelvic.IsFeasible {
		t.Errorf("Expected SLR Boost pelvic fit on 11mm post to exceed ±5mm clamp window")
	}
}
