package geometry

import (
	"math"
	"testing"

	"bikegeo/internal/catalog"
	"bikegeo/internal/model"
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

	report, err := AnalyzeSaddleSwap(
		sqlab,
		aspin,
		746.0,
		299.0,
		model.DefaultSeatpostSetbackMm,
		model.DefaultSeatpostClampLengthMm,
		700.0,
		13.0,
	)
	if err != nil {
		t.Fatalf("AnalyzeSaddleSwap failed: %v", err)
	}

	// 1. Check dimensional deltas (BRP skew vs true pelvic delta and 2D step)
	if report.BrpToSitBonesSpanDeltaMm != 8.0 {
		t.Errorf("Expected BRP-to-sit-bones span delta 8.0mm, got %.1fmm", report.BrpToSitBonesSpanDeltaMm)
	}
	if report.RailCenterToBrpDeltaMm != 11.5 {
		t.Errorf("Expected RailCenter-to-BRP delta 11.5mm, got %.1fmm", report.RailCenterToBrpDeltaMm)
	}
	if report.RailCenterToSitBonesDeltaMm != 3.5 {
		t.Errorf("Expected RailCenter-to-sit-bones delta 3.5mm, got %.1fmm", report.RailCenterToSitBonesDeltaMm)
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
	if naive.SitBoneDeltaMm != 8.0 {
		t.Errorf("Expected naive sit bone error +8.0mm, got %.1fmm", naive.SitBoneDeltaMm)
	}
	if naive.SitBoneDeltaYMm != -10.0 {
		t.Errorf("Expected naive sit bone vertical error -10.0mm (too low), got %.1fmm", naive.SitBoneDeltaYMm)
	}
	if math.Abs(naive.CockpitDeltaMm-8.1) > 0.5 {
		t.Errorf("Expected naive cockpit reach increase ~8.1mm, got %.1fmm", naive.CockpitDeltaMm)
	}
	if math.Abs(naive.BBToSitDistanceDeltaMm-(-6.1)) > 0.5 {
		t.Errorf("Expected BRP-matched BB-to-SIT change ~-6.1mm, got %.1fmm", naive.BBToSitDistanceDeltaMm)
	}
	if math.Abs(naive.RequiredClampOffsetMm-11.5) > 1e-9 {
		t.Errorf("Expected naive clamp offset +11.5mm, got %.1fmm", naive.RequiredClampOffsetMm)
	}
	if !naive.IsFeasible {
		t.Errorf("Expected naive BRP clamp offset to be feasible on Berthoud rails")
	}

	// 3. True Pelvic Alignment outcomes (2D coordinates)
	pelvic := report.PreserveSIT
	if pelvic.TargetBrpSetbackMm != 211.0 {
		t.Errorf("Expected pelvic target BRP setback 211.0mm, got %.1fmm", pelvic.TargetBrpSetbackMm)
	}
	if report.TargetEquivalentBrpSetbackMm != 211.0 {
		t.Errorf("Expected report.TargetEquivalentBrpSetbackMm 211.0mm, got %.1fmm", report.TargetEquivalentBrpSetbackMm)
	}
	if report.BrpSetbackShiftMm != -8.0 {
		t.Errorf("Expected report.BrpSetbackShiftMm -8.0mm (forward shift), got %.1fmm", report.BrpSetbackShiftMm)
	}
	if report.BrpDeltaXMm != 8.0 {
		t.Errorf("Expected report.BrpDeltaXMm +8.0mm, got %.1fmm", report.BrpDeltaXMm)
	}
	if report.BrpDeltaYMm != 10.0 {
		t.Errorf("Expected report.BrpDeltaYMm +10.0mm (upward shift to compensate for SQlab step), got %.1fmm", report.BrpDeltaYMm)
	}
	if math.Abs(report.TargetEquivalentSaddleHeightMm-753.3) > 0.2 {
		t.Errorf("Expected target equivalent saddle height ~ 753.3mm, got %.1fmm", report.TargetEquivalentSaddleHeightMm)
	}
	if math.Abs(report.BrpSaddleHeightShiftMm-7.3) > 0.2 {
		t.Errorf("Expected target BRP saddle height shift ~ +7.3mm, got %.1fmm", report.BrpSaddleHeightShiftMm)
	}
	if (report.BrpDeltaAngleDeg - 0.80).Abs() > 0.05 {
		t.Errorf("Expected report.BrpDeltaAngleDeg ~ +0.80°, got %.2f°", report.BrpDeltaAngleDeg)
	}
	if pelvic.AchievedSitBoneMm != 299.0 {
		t.Errorf("Expected pelvic achieved sit bone setback 299.0mm, got %.1fmm", pelvic.AchievedSitBoneMm)
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
	if pelvic.BBToSitDistanceDeltaMm != 0.0 {
		t.Errorf("Expected BB-to-SIT distance error 0.0mm, got %.1fmm", pelvic.BBToSitDistanceDeltaMm)
	}
	// The target rail center is at X = −254.5, 10 mm above the source BRP.
	// On the centered zero-setback source axis, X moves back by 251/BRP.Y per mm up.
	wantStation := 254.5 - 251*(1+10/math.Sqrt(746*746-219*219))
	if math.Abs(pelvic.RequiredClampOffsetMm-wantStation) > 1e-9 {
		t.Errorf("support-preserving clamp station = %.5f, want %.5f", pelvic.RequiredClampOffsetMm, wantStation)
	}
	if !pelvic.IsFeasible {
		t.Errorf("expected the station within the modeled Berthoud clamp window")
	}
}

func TestAnalyzeSaddleSwap_SQlabToSLRBoost(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	report, err := AnalyzeSaddleSwap(
		sqlab,
		slr,
		746.0,
		299.0,
		model.DefaultSeatpostSetbackMm,
		model.DefaultSeatpostClampLengthMm,
		700.0,
		13.0,
	)
	if err != nil {
		t.Fatalf("AnalyzeSaddleSwap failed: %v", err)
	}

	// SLR Boost sit bones are 17.3mm further rearward (-65.3 vs -48.0)
	if report.RailCenterToSitBonesDeltaMm != -17.3 {
		t.Errorf("Expected RailCenter-to-sit-bones delta -17.3mm, got %.1fmm", report.RailCenterToSitBonesDeltaMm)
	}
	// BRP span is 83mm vs 80mm (+3mm)
	if report.BrpToSitBonesSpanDeltaMm != 3.0 {
		t.Errorf("Expected BRP span delta 3.0mm, got %.1fmm", report.BrpToSitBonesSpanDeltaMm)
	}
	// 40mm straight rail with 30mm clamp allows [-5.0, +5.0] window
	// Pelvic clamp offset is -17.3mm, which exceeds [-5, +5] on 11mm post
	if report.PreserveSIT.IsFeasible {
		t.Errorf("Expected SLR Boost pelvic fit on 11mm post to exceed ±5mm clamp window")
	}
}

func TestAnalyzeSaddleSwap_ZeroDropAllowedAndInvalidRejected(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")

	// hbDropMm == 0 (level bar) must be preserved, not overwritten with 13.0.
	rep, err := AnalyzeSaddleSwap(sqlab, aspin, 746.0, 292.0, model.DefaultSeatpostSetbackMm, model.DefaultSeatpostClampLengthMm, 700.0, 0.0)
	if err != nil {
		t.Fatalf("AnalyzeSaddleSwap(hbDropMm=0) failed: %v", err)
	}
	if rep.HbDropMm != 0 {
		t.Errorf("HbDropMm = %.1f, want 0.0", rep.HbDropMm)
	}

	for _, tc := range []struct {
		name           string
		sh, sb, cl, hb float64
	}{
		{"saddleHeight<=0", 0, 292, 30, 700},
		{"sitBoneSetback<=0", 746, 0, 30, 700},
		{"clampLength<=0", 746, 292, 0, 700},
		{"hbDistance<=0", 746, 292, 30, 0},
	} {
		if _, err := AnalyzeSaddleSwap(sqlab, aspin, tc.sh, tc.sb, 25, tc.cl, tc.hb, 0); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestSaddleSwapRejectsImpossibleGeometry(t *testing.T) {
	s := model.ReferenceIdmatchSaddle()
	for _, c := range []struct{ height, setback, post, distance, rise float64 }{
		{100, 299, 0, 700, 13}, {746, 299, 0, 10, 13},
		{746, 299, -1, 700, 13}, {746, math.NaN(), 0, 700, 13},
	} {
		if _, err := AnalyzeSaddleSwap(s, s, c.height, c.setback, c.post, 30, c.distance, c.rise); err == nil {
			t.Errorf("accepted impossible swap %+v", c)
		}
	}
	// Centering the same saddle does not make an overlong clamp fit its rails.
	swap, err := AnalyzeSaddleSwap(s, s, 746, 299, 0, 61, 700, 13)
	if err != nil {
		t.Fatal(err)
	}
	if swap.Naive.IsFeasible || swap.PreserveSIT.IsFeasible {
		t.Error("overlong clamp reported feasible")
	}
}
