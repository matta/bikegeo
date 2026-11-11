package canonicalizer_test

import (
	"math"
	"testing"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

func TestCanonicalizeBikes(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
			ID:           "test-bike",
			Manufacturer: "Test",
			Model:        "HT",
			Size:         "L",
			Reach:        450,
			Stack:        630,
			HtaDeg:       66.0,
			StaDeg:       75.0,
			Htl:          110,
			BbDrop:       60,
			ForkRake:     44,
			Wheelbase:    1180,
			ForkA2c:      541,
		},
	}

	prof := catalog.DefaultRiderProfile()
	prof.SaddleHeightMm = 760.0
	config := model.CanonicalConfig{
		TargetA2c:    515.0,
		RiderProfile: prof,
		SagPercent:   20.0,
	}

	rows, err := canonicalizer.CanonicalizeBikes(bikes, config, 44.0, "Fox 34 130mm")
	if err != nil {
		t.Fatalf("CanonicalizeBikes failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row.Canonical.ActiveReach <= 0 {
		t.Errorf("Expected positive active reach, got %.1f", row.Canonical.ActiveReach)
	}
	if row.Baseline.Reach != 450 {
		t.Errorf("Expected baseline reach 450, got %.1f", row.Baseline.Reach)
	}
}

func TestGetCatalogFork(t *testing.T) {
	entry, found := canonicalizer.GetCatalogFork("fox34_130")
	if !found {
		t.Fatalf("Expected fox34_130 to be found in catalog")
	}
	if entry.TravelMm != 130 {
		t.Errorf("Expected TravelMm 130, got %.1f", entry.TravelMm)
	}
	if len(entry.Options) == 0 || entry.Options[0].A2cMm != 541 {
		t.Errorf("Expected Option A2C 541, got %+v", entry.Options)
	}
	if entry.Options[0].RakeMm != 44 {
		t.Errorf("Expected Option Rake 44, got %+v", entry.Options)
	}
}

func TestCanonicalizeBentSeatTube(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
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
			Wheelbase:    1167.0,
			ForkA2c:      510.0,
		},
	}

	config := model.CanonicalConfig{
		TargetA2c:    510.0,
		RiderProfile: catalog.DefaultRiderProfile(),
		SagPercent:   20.0,
	}

	rows, err := canonicalizer.CanonicalizeBikes(bikes, config, 44.0, "Fox 34 130mm")
	if err != nil {
		t.Fatalf("CanonicalizeBikes failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	// Baseline effective STA at 780mm saddle height on Large (Ox ~ 19.72mm) is ~74.69°.
	// Pitched with TargetA2c 510.0mm (vs CAD geometric 504.3mm), frame slacks by 0.26° -> 74.43°.
	if row.Canonical.ActiveSta != 74.43 {
		t.Errorf("Expected ActiveSta 74.43°, got %.2f°", row.Canonical.ActiveSta)
	}

	// Baseline STA in spec should preserve published 75.0°
	if row.Baseline.StaDeg != 75.0 {
		t.Errorf("Expected Baseline.StaDeg 75.0, got %.1f", row.Baseline.StaDeg)
	}

	// Baseline Setback should be 206.0mm (derived from effective STA ~74.69°)
	if row.Baseline.ActualSetback != 206.0 {
		t.Errorf("Expected Baseline.ActualSetback 206.0, got %.1f", row.Baseline.ActualSetback)
	}
}

func TestCanonicalizeBikesRequiresPositiveSeatHeight(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
			ID:           "test-bike",
			Manufacturer: "Test",
			Model:        "Model",
			Reach:        450,
			Stack:        620,
			HtaDeg:       66.0,
			StaDeg:       75.0,
			Htl:          110,
			BbDrop:       60,
			ForkRake:     44,
			Wheelbase:    1180,
			ForkA2c:      541,
		},
	}

	badZero := catalog.DefaultRiderProfile()
	badZero.SaddleHeightMm = 0
	cfgZero := model.CanonicalConfig{TargetA2c: 515.0, RiderProfile: badZero}
	if _, err := canonicalizer.CanonicalizeBikes(bikes, cfgZero, 44.0, "130mm"); err == nil {
		t.Errorf("Expected error for SaddleHeightMm=0, got nil")
	}

	badNeg := catalog.DefaultRiderProfile()
	badNeg.SaddleHeightMm = -50
	cfgNegative := model.CanonicalConfig{TargetA2c: 515.0, RiderProfile: badNeg}
	if _, err := canonicalizer.CanonicalizeBikes(bikes, cfgNegative, 44.0, "130mm"); err == nil {
		t.Errorf("Expected error for negative SaddleHeightMm, got nil")
	}
}

func TestExpandPlausibleForkSetups(t *testing.T) {
	t.Run("RigidOnlyFrame", func(t *testing.T) {
		frame, err := frames.GetFrameByID("tumbleweed-sunliner")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, nil)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		if len(setups) != 1 {
			t.Fatalf("expected 1 setup for rigid-only Sunliner, got %d", len(setups))
		}
		if setups[0].DisplayLabel != "Rigid (440mm)" || setups[0].A2cMm != 440 || setups[0].SagPct != 0 {
			t.Errorf("unexpected setup: %+v", setups[0])
		}
		if !setups[0].AutoExpanded {
			t.Errorf("expected AutoExpanded to be true")
		}
	})

	t.Run("SuspensionOnlyFrame", func(t *testing.T) {
		frame, err := frames.GetFrameByID("fairlight-holt-2")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, nil)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// Fairlight Holt 2 supports 100mm to 120mm forks (100, 110, 120mm) across 4 sags = 12 setups
		if len(setups) != 12 {
			t.Fatalf("expected 12 setups for Holt 2, got %d", len(setups))
		}
		for _, s := range setups {
			if canonicalizer.IsRigidFork(s.DisplayLabel) {
				t.Errorf("did not expect rigid fork on suspension-only frame: %+v", s)
			}
		}
	})

	t.Run("HybridFrameWithRigidAndSuspension", func(t *testing.T) {
		frame, err := frames.GetFrameByID("otso-fenrir-stainless")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, nil)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// 1 rigid (495mm) + 100, 110, 120mm across 4 sags (12) = 13 setups
		if len(setups) != 13 {
			t.Fatalf("expected 13 setups for Fenrir, got %d", len(setups))
		}
		hasRigid := false
		for _, s := range setups {
			if s.DisplayLabel == "Rigid (495mm)" && s.RakeMm == 54.0 {
				hasRigid = true
			}
		}
		if !hasRigid {
			t.Errorf("expected Lithic Rigid (495mm) fork with 54mm rake in Fenrir setups")
		}
	})

	t.Run("FrameWithSpecificRakePreserved", func(t *testing.T) {
		frame, err := frames.GetFrameByID("salsa-fargo")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, nil)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// Salsa Fargo has 51mm rake for its rigid and 100mm suspension fork
		for _, s := range setups {
			if s.RakeMm != 51.0 {
				t.Errorf("expected 51mm rake preserved on Salsa Fargo setups, got %.1f in %+v", s.RakeMm, s)
			}
		}
	})

	t.Run("SagOverrideFiltersForks", func(t *testing.T) {
		frame, err := frames.GetFrameByID("otso-fenrir-stainless")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		sag20 := 20.0
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, &sag20)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// Rigid fork is excluded when sagOverride > 0, 3 suspension travels at 20% sag = 3 setups
		if len(setups) != 3 {
			t.Fatalf("expected 3 setups for sagOverride=20%%, got %d", len(setups))
		}
		for _, s := range setups {
			if s.SagPct != 20.0 {
				t.Errorf("expected SagPct 20.0, got %.1f", s.SagPct)
			}
		}
	})
}

func TestPelvicAndCockpitInvarianceAcrossSaddles(t *testing.T) {
	frame, err := frames.GetFrameByID("salsa-fargo")
	if err != nil {
		t.Fatalf("failed to get frame: %v", err)
	}

	setup := model.ComparisonSetup{
		FrameID: frame.ID,
		Size:    "L",
		A2cMm:   483,
		RakeMm:  51,
	}
	prof := ideal.IdmatchProfile()

	sqlab, _ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, _ := catalog.GetSaddle("berthoud-aspin-open")

	// 1. Resolve row with SQlab 610
	riderSQ := catalog.DefaultRiderProfile()
	riderSQ.SaddleHeightMm = 746.0
	rowSQ, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, setup, riderSQ, sqlab)
	if err != nil {
		t.Fatalf("Failed to resolve SQlab row: %v", err)
	}
	rowsSQ := []model.CanonicalComparisonRow{rowSQ}
	canonicalizer.ApplyIdealCockpitTargets(rowsSQ, prof)

	// 2. Resolve row with Berthoud Aspin
	riderAspin := catalog.DefaultRiderProfile()
	riderAspin.SaddleHeightMm = 746.0 // Reference idmatch SH
	rowAspin, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, setup, riderAspin, aspin)
	if err != nil {
		t.Fatalf("Failed to resolve Aspin row: %v", err)
	}
	rowsAspin := []model.CanonicalComparisonRow{rowAspin}
	canonicalizer.ApplyIdealCockpitTargets(rowsAspin, prof)

	// Verify Sit Bone Setback is invariant
	if math.Abs(rowsSQ[0].Canonical.SitBoneSetbackMm-rowsAspin[0].Canonical.SitBoneSetbackMm) > 1.5 {
		t.Errorf("Expected identical sit bone setback across saddles: SQlab=%.1f, Aspin=%.1f",
			rowsSQ[0].Canonical.SitBoneSetbackMm, rowsAspin[0].Canonical.SitBoneSetbackMm)
	}

	// Verify Selected HB point (HbX, HbY) and sit-bone-relative SaddleHbDropMm are invariant in space
	if rowsSQ[0].Canonical.SelectedHB == nil || rowsAspin[0].Canonical.SelectedHB == nil {
		t.Fatalf("Expected non-nil SelectedHB")
	}
	hbSQ := *rowsSQ[0].Canonical.SelectedHB
	hbAspin := *rowsAspin[0].Canonical.SelectedHB
	if math.Abs(hbSQ.HbX-hbAspin.HbX) > 1.0 || math.Abs(hbSQ.HbY-hbAspin.HbY) > 1.0 {
		t.Errorf("Expected identical SelectedHB (HbX, HbY): SQlab=(%.1f, %.1f), Aspin=(%.1f, %.1f)",
			hbSQ.HbX, hbSQ.HbY, hbAspin.HbX, hbAspin.HbY)
	}
	if math.Abs(hbSQ.SaddleHbDropMm-hbAspin.SaddleHbDropMm) > 0.5 {
		t.Errorf("Expected identical sit-bone-relative SaddleHbDropMm across saddles: SQlab=%.1f, Aspin=%.1f",
			hbSQ.SaddleHbDropMm, hbAspin.SaddleHbDropMm)
	}
}
