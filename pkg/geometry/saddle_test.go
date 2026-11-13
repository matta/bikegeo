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

	slr, ok := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")
	if !ok {
		t.Fatalf("Failed to find selle-italia-slr-boost-superflow-l3 in catalog")
	}

	// 1. DeltaBrpToSitBonesMm
	if math.Abs(sqlab.DeltaBrpToSitBonesMm()-73.0) > 0.001 {
		t.Errorf("Expected SQlab DeltaBrpToSitBonesMm 73.0, got %.1f", sqlab.DeltaBrpToSitBonesMm())
	}
	if math.Abs(aspin.DeltaBrpToSitBonesMm()-88.0) > 0.001 {
		t.Errorf("Expected Aspin DeltaBrpToSitBonesMm 88.0, got %.1f", aspin.DeltaBrpToSitBonesMm())
	}
	if math.Abs(slr.DeltaBrpToSitBonesMm()-83.0) > 0.001 {
		t.Errorf("Expected SLR Boost DeltaBrpToSitBonesMm 83.0, got %.1f", slr.DeltaBrpToSitBonesMm())
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
	slrBrpSetback := slr.BrpSetbackFromSitBones(292.0)
	if math.Abs(slrBrpSetback-209.0) > 0.001 {
		t.Errorf("Expected SLR Boost BrpSetback 209.0, got %.1f", slrBrpSetback)
	}
	if math.Abs(slr.SitBoneSetbackFromBrp(slrBrpSetback)-292.0) > 0.001 {
		t.Errorf("Expected SLR Boost SitBoneSetback roundtrip 292.0, got %.1f", slr.SitBoneSetbackFromBrp(slrBrpSetback))
	}

	// 3. RailCenterSetbackFromBrp
	rcSetback := sqlab.RailCenterSetbackFromBrp(brpSetback)
	if math.Abs(rcSetback-242.0) > 0.001 {
		t.Errorf("Expected RailCenterSetback 242.0, got %.1f", rcSetback)
	}
	slrRcSetback := slr.RailCenterSetbackFromBrp(slrBrpSetback)
	if math.Abs(slrRcSetback-226.7) > 0.001 {
		t.Errorf("Expected SLR Boost RailCenterSetback 226.7, got %.1f", slrRcSetback)
	}

	// 4. Clamp Windows with 45mm clamp (Drop Best) and 30mm clamp (standard)
	// SQlab 60mm rail (-30 to +30): clamp center is [-7.5, +7.5] with 45mm clamp
	sqMin45, sqMax45 := sqlab.ClampWindowMm(model.DropBestClampLengthMm)
	if sqMin45 != -7.5 || sqMax45 != 7.5 {
		t.Errorf("Expected SQlab 45mm clamp window [-7.5, +7.5], got [%.1f, %.1f]", sqMin45, sqMax45)
	}
	// SQlab 60mm rail with 30mm clamp: [-15.0, +15.0]
	sqMin30, sqMax30 := sqlab.ClampWindowMm(model.DefaultSeatpostClampLengthMm)
	if sqMin30 != -15.0 || sqMax30 != 15.0 {
		t.Errorf("Expected SQlab 30mm clamp window [-15.0, +15.0], got [%.1f, %.1f]", sqMin30, sqMax30)
	}

	// Berthoud Aspin 64mm rail: [-9.5, +9.5] with 45mm clamp
	aspinMin, aspinMax := aspin.ClampWindowMm(model.DropBestClampLengthMm)
	if aspinMin != -9.5 || aspinMax != 9.5 {
		t.Errorf("Expected Aspin 45mm clamp window [-9.5, +9.5], got [%.1f, %.1f]", aspinMin, aspinMax)
	}

	// Selle Italia SLR Boost 40mm rail:
	// 30mm clamp: [-5.0, +5.0]
	slrMin30, slrMax30 := slr.ClampWindowMm(model.DefaultSeatpostClampLengthMm)
	if slrMin30 != -5.0 || slrMax30 != 5.0 {
		t.Errorf("Expected SLR Boost 30mm clamp window [-5.0, +5.0], got [%.1f, %.1f]", slrMin30, slrMax30)
	}
	// 45mm clamp: clamp length > rail length (incompatible -> [0.0, 0.0])
	slrMin45, slrMax45 := slr.ClampWindowMm(model.DropBestClampLengthMm)
	if slrMin45 != 0.0 || slrMax45 != 0.0 {
		t.Errorf("Expected SLR Boost 45mm clamp window [0.0, 0.0], got [%.1f, %.1f]", slrMin45, slrMax45)
	}
}

func TestTranslateSaddleFit_SQlabToAspinAndSLR(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	// Current setup: SQlab clamped at -15.0mm (Drop Best 45mm clamp) to Aspin
	resAspin := geometry.TranslateSaddleFit(sqlab, -15.0, aspin, model.DropBestClampLengthMm)

	// Difference in rail-center-to-sit-bones is +5.5mm
	if math.Abs(resAspin.SitBoneDeltaMm-5.5) > 0.001 {
		t.Errorf("Expected SitBoneDeltaMm 5.5, got %.1f", resAspin.SitBoneDeltaMm)
	}

	// Target clamp position on Aspin is -15.0 + 5.5 = -9.5mm
	if math.Abs(resAspin.TargetClampOffset-(-9.5)) > 0.001 {
		t.Errorf("Expected TargetClampOffset -9.5, got %.1f", resAspin.TargetClampOffset)
	}

	// -9.5mm falls exactly on the Aspin's rear mechanical stop
	if !resAspin.IsFeasible {
		t.Errorf("Expected translation to be feasible, got false")
	}

	// BRP delta is 88 - 73 = 15.0mm
	if math.Abs(resAspin.BrpDeltaMm-15.0) > 0.001 {
		t.Errorf("Expected BrpDeltaMm 15.0, got %.1f", resAspin.BrpDeltaMm)
	}

	// Translation from SQlab dead-center (0.0mm) to SLR Boost with 30mm clamp
	resSLR := geometry.TranslateSaddleFit(sqlab, 0.0, slr, model.DefaultSeatpostClampLengthMm)
	// Sit bones on SLR Boost sit 15.3mm further rearward (-65.3 vs -50.0) -> SitBoneDeltaMm = -15.3mm
	if math.Abs(resSLR.SitBoneDeltaMm-(-15.3)) > 0.001 {
		t.Errorf("Expected SLR SitBoneDeltaMm -15.3, got %.1f", resSLR.SitBoneDeltaMm)
	}
	// Target clamp offset = 0.0 + (-15.3) = -15.3mm
	if math.Abs(resSLR.TargetClampOffset-(-15.3)) > 0.001 {
		t.Errorf("Expected TargetClampOffset -15.3, got %.1f", resSLR.TargetClampOffset)
	}
	// -15.3mm is outside the 40mm rail's [-5.0, +5.0] 30mm clamp window
	if resSLR.IsFeasible {
		t.Errorf("Expected SLR 30mm translation to not be feasible within clamp window, got true")
	}
}

func TestCalculateIdealSeatTubeAngle(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	targetSitBoneSetback := 292.0
	saddleHeight := 746.0

	// 1. Redshift + Drop Best sagged (39mm hardware setback)
	// Exact 2D inverse of CalculatePoint: acos(219/746) + asin((39-23)/746) = 74.16°
	resFenrir := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 39.0, "Redshift + Drop Best (sagged)", model.DropBestClampLengthMm)
	if (resFenrir.IdealStaDeg - 74.16).Abs() > 0.05 {
		t.Errorf("Expected IdealStaDeg ~ 74.16°, got %.2f°", resFenrir.IdealStaDeg)
	}

	// 2. Standard 15mm setback post
	res15 := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 15.0, "15mm Setback", model.DefaultSeatpostClampLengthMm)
	if (res15.IdealStaDeg - 72.3).Abs() > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 72.3° for 15mm post, got %.2f°", res15.IdealStaDeg)
	}

	// 3. Standard 25mm setback post
	res25 := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 25.0, "25mm Setback", model.DefaultSeatpostClampLengthMm)
	if (res25.IdealStaDeg - 73.1).Abs() > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 73.1° for 25mm post, got %.2f°", res25.IdealStaDeg)
	}

	// 4. Redshift suspension post sagged (11mm setback)
	resRedshiftSag := geometry.CalculateIdealSeatTubeAngle(sqlab, targetSitBoneSetback, saddleHeight, 11.0, "Redshift ShockStop (sagged)", model.DefaultSeatpostClampLengthMm)
	if (resRedshiftSag.IdealStaDeg - 72.0).Abs() > 0.3 {
		t.Errorf("Expected IdealStaDeg ~ 72.0° for Redshift sagged, got %.2f°", resRedshiftSag.IdealStaDeg)
	}
}

func TestCalculateUniversalCompromiseSTA(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	saddles2 := []model.SaddleSpec{sqlab, aspin}
	saddles3 := catalog.AllSaddles()
	targetSitBoneSetback := 292.0
	saddleHeight := 746.0

	// 1. Standard 15mm setback post across 2 reference saddles (SQlab @ 746mm & Aspin @ 751.35mm eqSH)
	res15_2 := geometry.CalculateUniversalCompromiseSTA(saddles2, targetSitBoneSetback, saddleHeight, 15.0, "Standard 15mm Setback", model.DefaultSeatpostClampLengthMm)
	if (res15_2.CompromiseStaDeg - 72.19).Abs() > 0.1 {
		t.Errorf("Expected 2-saddle CompromiseStaDeg ~ 72.19°, got %.2f°", res15_2.CompromiseStaDeg)
	}
	if math.Abs(res15_2.MaxRailOffsetMm-1.58) > 0.1 {
		t.Errorf("Expected 2-saddle MaxRailOffsetMm ~ 1.58mm, got %.2fmm", res15_2.MaxRailOffsetMm)
	}
	if !res15_2.HasUniversalMargin {
		t.Errorf("Expected 2-saddle HasUniversalMargin to be true")
	}
	if len(res15_2.SaddleOffsets) != 2 {
		t.Fatalf("Expected 2 saddle offsets, got %d", len(res15_2.SaddleOffsets))
	}

	// 2. Full 3-saddle catalog across 15mm post
	res15_3 := geometry.CalculateUniversalCompromiseSTA(saddles3, targetSitBoneSetback, saddleHeight, 15.0, "Standard 15mm Setback", model.DefaultSeatpostClampLengthMm)
	if (res15_3.CompromiseStaDeg - 72.87).Abs() > 0.1 {
		t.Errorf("Expected 3-saddle CompromiseStaDeg ~ 72.87°, got %.2f°", res15_3.CompromiseStaDeg)
	}
	if math.Abs(res15_3.MaxRailOffsetMm-10.4) > 0.2 {
		t.Errorf("Expected 3-saddle MaxRailOffsetMm ~ 10.4mm, got %.2fmm", res15_3.MaxRailOffsetMm)
	}
	if res15_3.HasUniversalMargin {
		t.Errorf("Expected 3-saddle HasUniversalMargin to be false (span > 10.0mm)")
	}
	if len(res15_3.SaddleOffsets) != 3 {
		t.Fatalf("Expected 3 saddle offsets, got %d", len(res15_3.SaddleOffsets))
	}

	// 3. Redshift + Drop Best sagged (39mm hardware setback) across 2 saddles vs 3 saddles
	res39_2 := geometry.CalculateUniversalCompromiseSTA(saddles2, targetSitBoneSetback, saddleHeight, 39.0, "Redshift + Drop Best (sagged 39mm)", model.DropBestClampLengthMm)
	if (res39_2.CompromiseStaDeg - 74.03).Abs() > 0.1 {
		t.Errorf("Expected CompromiseStaDeg ~ 74.03° for 39mm hardware setback, got %.2f°", res39_2.CompromiseStaDeg)
	}
	if math.Abs(res39_2.MaxRailOffsetMm-1.66) > 0.1 {
		t.Errorf("Expected MaxRailOffsetMm ~ 1.66mm, got %.2fmm", res39_2.MaxRailOffsetMm)
	}

	res39_3 := geometry.CalculateUniversalCompromiseSTA(saddles3, targetSitBoneSetback, saddleHeight, 39.0, "Redshift + Drop Best (sagged 39mm)", model.DropBestClampLengthMm)
	if (res39_3.CompromiseStaDeg - 74.71).Abs() > 0.15 {
		t.Errorf("Expected 3-saddle CompromiseStaDeg ~ 74.71° for 39mm hardware setback, got %.2f°", res39_3.CompromiseStaDeg)
	}
	if math.Abs(res39_3.MaxRailOffsetMm-10.4) > 0.2 {
		t.Errorf("Expected 3-saddle MaxRailOffsetMm ~ 10.4mm, got %.2fmm", res39_3.MaxRailOffsetMm)
	}
}

func TestSaddleSpec_CalculatePoint_And_PelvicDerivation(t *testing.T) {
	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")
	slr, _ := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")

	// 1. Primary pelvic setback derivation from idmatch 219mm BRP via SQlab reference
	pelvicSetback := model.DerivePelvicSetbackFromIdmatchBrp(219.0)
	if pelvicSetback != 292.0 {
		t.Errorf("Expected pelvic setback 292.0mm, got %.1fmm", pelvicSetback)
	}

	// 2. Active saddle BRP targets derived from pelvic setback
	if sqlab.TargetBrpForPelvicSetback(pelvicSetback) != 219.0 {
		t.Errorf("Expected SQlab target BRP 219.0mm, got %.1fmm", sqlab.TargetBrpForPelvicSetback(pelvicSetback))
	}
	if aspin.TargetBrpForPelvicSetback(pelvicSetback) != 204.0 {
		t.Errorf("Expected Aspin target BRP 204.0mm, got %.1fmm", aspin.TargetBrpForPelvicSetback(pelvicSetback))
	}
	if slr.TargetBrpForPelvicSetback(pelvicSetback) != 209.0 {
		t.Errorf("Expected SLR Boost target BRP 209.0mm, got %.1fmm", slr.TargetBrpForPelvicSetback(pelvicSetback))
	}

	// 3. Vector math on 72.0° STA with Redshift sagged (11mm setback)
	seatHeight := 746.0
	pt72 := sqlab.CalculatePoint(72.0, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	if math.Abs(pt72.BrpSetbackMm-219.1) > 0.3 {
		t.Errorf("Expected 72.0° STA dead-center BRP ~219.1mm, got %.1fmm", pt72.BrpSetbackMm)
	}
	if (pt72.BrpAngleFromBbDeg - 72.9).Abs() > 0.2 {
		t.Errorf("Expected 72.0° STA BRP angle ~72.9°, got %.1f°", pt72.BrpAngleFromBbDeg)
	}

	// 4. Steep 75.0° STA (Binary Kinetik) runs out of rearward rail travel
	pt75DeadCenter := sqlab.CalculatePoint(75.0, seatHeight, model.DefaultSeatpostSetbackMm, 0.0)
	if math.Abs(pt75DeadCenter.BrpSetbackMm-181.5) > 0.5 {
		t.Errorf("Expected 75.0° STA dead-center BRP ~181.5mm, got %.1fmm", pt75DeadCenter.BrpSetbackMm)
	}
	pt75MaxRail := sqlab.CalculatePoint(75.0, seatHeight, model.DefaultSeatpostSetbackMm, 15.0)
	if pt75MaxRail.BrpSetbackMm > 197.0 {
		t.Errorf("Expected 75.0° STA max rail BRP <= 197mm (cannot reach 219mm), got %.1fmm", pt75MaxRail.BrpSetbackMm)
	}
	if pt75MaxRail.SitBoneSetbackMm > 270.0 {
		t.Errorf("Expected 75.0° STA max rail SitBoneSetback <= 270mm (cannot reach 292mm), got %.1fmm", pt75MaxRail.SitBoneSetbackMm)
	}
}

func TestIdealStaRoundTripsCalculatePoint(t *testing.T) {
	ref := model.ReferenceIdmatchSaddle()
	targetSB := 292.0
	baseSH := 746.0
	refBrp := ref.BrpSetbackFromSitBones(targetSB)
	saddles := catalog.AllSaddles()

	for _, postSetback := range []float64{0, 7, 11, 15, 25, 35, 39} {
		for _, s := range saddles {
			eqH := baseSH
			if s.ID != ref.ID {
				eqH = s.TranslateSaddleHeightFromRef(ref, baseSH, refBrp)
			}
			res := geometry.CalculateIdealSeatTubeAngle(s, targetSB, eqH, postSetback, "post", model.DefaultSeatpostClampLengthMm)
			pt := s.CalculatePoint(res.IdealStaDeg, eqH, postSetback, 0.0)
			if math.Abs(pt.SitBoneSetbackMm-targetSB) > 0.01 {
				t.Errorf("%s post=%.0f: CalculatePoint(%.4f°, %.2f, %.0f, 0).SitBoneSetbackMm = %.4f, want %.1f",
					s.ID, postSetback, res.IdealStaDeg, eqH, postSetback, pt.SitBoneSetbackMm, targetSB)
			}
		}

		comp := geometry.CalculateUniversalCompromiseSTA(saddles, targetSB, baseSH, postSetback, "post", model.DefaultSeatpostClampLengthMm)
		for i, s := range saddles {
			eqH := baseSH
			if s.ID != ref.ID {
				eqH = s.TranslateSaddleHeightFromRef(ref, baseSH, refBrp)
			}
			pt := s.CalculatePoint(comp.CompromiseStaDeg, eqH, postSetback, comp.SaddleOffsets[i].OffsetMm)
			if math.Abs(pt.SitBoneSetbackMm-targetSB) > 0.01 {
				t.Errorf("Compromise %s post=%.0f: SitBoneSetbackMm = %.4f, want %.1f",
					s.ID, postSetback, pt.SitBoneSetbackMm, targetSB)
			}
		}
	}
}
