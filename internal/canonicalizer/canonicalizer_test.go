package canonicalizer_test

import (
	"math"
	"testing"

	"bikegeo/internal/canonicalizer"
	"bikegeo/internal/catalog"
	"bikegeo/internal/frames"
	"bikegeo/internal/geometry"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/validator"
)

var testCat = validator.Catalog{
	Forks: catalog.CommonForkCatalog,
	Stems: catalog.CommonStemCatalog,
}

func TestCanonicalizeBikes(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
			ID:              "test-bike",
			Manufacturer:    "Test",
			Model:           "HT",
			Size:            "L",
			Reach:           450,
			Stack:           630,
			HtaDeg:          66.0,
			StaDeg:          75.0,
			Htl:             110,
			BbDrop:          60,
			ChainstayLength: 430,
			ForkRake:        44,
			Wheelbase:       1180,
			ForkA2c:         541,
		},
	}

	prof := catalog.DefaultRiderProfile()
	prof.SaddleHeightMm = 760.0
	config := model.CanonicalConfig{
		TargetA2c:    515.0,
		Saddle:       catalog.DefaultSaddle(),
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

func TestFindForkFrom(t *testing.T) {
	entry, found := catalog.FindForkFrom(catalog.CommonForkCatalog, "fox34_130")
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
			Wheelbase:       1167.0,
			ForkA2c:         510.0,
		},
	}

	config := model.CanonicalConfig{
		TargetA2c:    510.0,
		Saddle:       catalog.DefaultSaddle(),
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
	if (row.Canonical.ActiveSta - 74.43).Abs() > 0.01 {
		t.Errorf("Expected ActiveSta 74.43°, got %.2f°", row.Canonical.ActiveSta)
	}

	// Baseline STA in spec should preserve published 75.0°
	if (row.Baseline.StaDeg - 75.0).Abs() > 0.05 {
		t.Errorf("Expected Baseline.StaDeg 75.0, got %.1f", row.Baseline.StaDeg)
	}

	// Baseline Setback should be 206.0mm (derived from effective STA ~74.69°)
	if math.Abs(row.Baseline.ActualSetback-206.0) > 0.05 {
		t.Errorf("Expected Baseline.ActualSetback 206.0, got %.1f", row.Baseline.ActualSetback)
	}
}

func TestCanonicalizeBikesRequiresPositiveSeatHeight(t *testing.T) {
	bikes := []model.BikeGeometry{
		{
			ID:              "test-bike",
			Manufacturer:    "Test",
			Model:           "Model",
			Reach:           450,
			Stack:           620,
			HtaDeg:          66.0,
			StaDeg:          75.0,
			Htl:             110,
			BbDrop:          60,
			ChainstayLength: 430,
			ForkRake:        44,
			Wheelbase:       1180,
			ForkA2c:         541,
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
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Opt[float64]{}, testCat.Forks)
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
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Opt[float64]{}, testCat.Forks)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// Fairlight Holt 2 supports 100mm to 120mm forks (100, 110, 120mm) across 3 sags = 9 setups
		if len(setups) != 9 {
			t.Fatalf("expected 9 setups for Holt 2, got %d", len(setups))
		}
		for _, s := range setups {
			if s.IsRigid {
				t.Errorf("did not expect rigid fork on suspension-only frame: %+v", s)
			}
		}
	})

	t.Run("HybridFrameWithRigidAndSuspension", func(t *testing.T) {
		frame, err := frames.GetFrameByID("otso-fenrir-stainless")
		if err != nil {
			t.Fatalf("failed to get frame: %v", err)
		}
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Opt[float64]{}, testCat.Forks)
		if err != nil {
			t.Fatalf("ExpandPlausibleForkSetups failed: %v", err)
		}
		// 1 rigid (495mm) + 100, 110, 120mm across 3 sags (9) = 10 setups
		if len(setups) != 10 {
			t.Fatalf("expected 10 setups for Fenrir, got %d", len(setups))
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
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Opt[float64]{}, testCat.Forks)
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
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Some(20.0), testCat.Forks)
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
	rowSQ, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, setup, riderSQ, sqlab, testCat)
	if err != nil {
		t.Fatalf("Failed to resolve SQlab row: %v", err)
	}
	rowsSQ := []model.CanonicalComparisonRow{rowSQ}
	if err := canonicalizer.ApplyIdealCockpitTargets(rowsSQ, prof); err != nil {
		t.Fatalf("ApplyIdealCockpitTargets(SQlab): %v", err)
	}

	// 2. Resolve row with Berthoud Aspin
	riderAspin := catalog.DefaultRiderProfile()
	riderAspin.SaddleHeightMm = 746.0 // Reference idmatch SH
	rowAspin, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, setup, riderAspin, aspin, testCat)
	if err != nil {
		t.Fatalf("Failed to resolve Aspin row: %v", err)
	}
	rowsAspin := []model.CanonicalComparisonRow{rowAspin}
	if err := canonicalizer.ApplyIdealCockpitTargets(rowsAspin, prof); err != nil {
		t.Fatalf("ApplyIdealCockpitTargets(Aspin): %v", err)
	}

	// Verify Sit Bone Setback is invariant
	if math.Abs(rowsSQ[0].Canonical.SitBoneSetbackMm-rowsAspin[0].Canonical.SitBoneSetbackMm) > 1.5 {
		t.Errorf("Expected identical sit bone setback across saddles: SQlab=%.1f, Aspin=%.1f",
			rowsSQ[0].Canonical.SitBoneSetbackMm, rowsAspin[0].Canonical.SitBoneSetbackMm)
	}

	// Verify SitBonePoint (X, Y) in 2D space is invariant across saddles
	sbSQ := rowsSQ[0].Canonical.SelectedBRP.SitBonePoint()
	sbAspin := rowsAspin[0].Canonical.SelectedBRP.SitBonePoint()
	if math.Abs(sbSQ.X-sbAspin.X) > 1.5 || math.Abs(sbSQ.Y-sbAspin.Y) > 1.5 {
		t.Errorf("Expected identical SitBonePoint (X, Y) across saddles: SQlab=(%.1f, %.1f), Aspin=(%.1f, %.1f)",
			sbSQ.X, sbSQ.Y, sbAspin.X, sbAspin.Y)
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
	// The one-mm rail grid can leave different sub-mm support heights. Rise
	// must use those achieved coordinates, not an assumed invariant height.
	wantRiseDelta := (hbSQ.HbY - hbAspin.HbY) - (sbSQ.Y - sbAspin.Y)
	if math.Abs((hbSQ.SaddleHbDropMm-hbAspin.SaddleHbDropMm)-wantRiseDelta) > 1e-9 {
		t.Errorf("hand-grip rise disagrees with achieved support points")
	}
}

func TestAllowedSagPercentages(t *testing.T) {
	// 20, 25, 30 must be allowed
	for _, sag := range []float64{20.0, 25.0, 30.0} {
		if !canonicalizer.IsAllowedSag(sag) {
			t.Errorf("expected %.0f%% to be an allowed sag", sag)
		}
	}

	// 15% and others must not be allowed
	for _, sag := range []float64{0.0, 10.0, 15.0, 18.0, 35.0} {
		if canonicalizer.IsAllowedSag(sag) {
			t.Errorf("expected %.0f%% to NOT be an allowed sag", sag)
		}
	}

	frame, err := frames.GetFrameByID("otso-fenrir-stainless")
	if err != nil {
		t.Fatalf("failed to get frame: %v", err)
	}

	// ExpandPlausibleForkSetups with unallowed 15% sag should error
	if _, err := canonicalizer.ExpandPlausibleForkSetups(*frame, model.Some(15.0), testCat.Forks); err == nil {
		t.Errorf("expected ExpandPlausibleForkSetups to error for 15%% sag override")
	}

	// ResolveForkToken with unallowed 15% sag should error
	if _, err := canonicalizer.ResolveForkToken("110mm", model.Some(15.0), *frame, testCat.Forks); err == nil {
		t.Errorf("expected ResolveForkToken to error for 15%% sag override")
	}
}

// scanRow resolves a real setup with the default saddle at the idmatch height.
func scanRow(t *testing.T, frameID, size string, a2c, rake float64) model.CanonicalComparisonRow {
	t.Helper()
	frame, err := frames.GetFrameByID(frameID)
	if err != nil {
		t.Fatalf("GetFrameByID(%q): %v", frameID, err)
	}
	rider := catalog.DefaultRiderProfile()
	rider.SaddleHeightMm = 746.0
	row, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, model.ComparisonSetup{
		FrameID: frame.ID, Size: size, A2cMm: a2c, RakeMm: rake,
	}, rider, catalog.DefaultSaddle(), testCat)
	if err != nil {
		t.Fatalf("resolve %s:%s: %v", frameID, size, err)
	}
	return row
}

func specOf(t *testing.T, prof *ideal.Profile, id ideal.MetricID) ideal.AttributeSpec {
	t.Helper()
	spec, ok := prof.Spec(id)
	if !ok {
		t.Fatalf("profile has no %s", id)
	}
	return spec
}

// The saddle and cockpit scans keep the cheapest stock build: no other rail
// position, and no other stem length × angle × spacer combination, loses fewer
// points on the bands that price it.
func TestApplyIdealCockpitTargets_KeepsCheapestStockBuild(t *testing.T) {
	prof := ideal.IdmatchProfile()
	setback := specOf(t, prof, ideal.MetricBrpSetback)
	rail := specOf(t, prof, ideal.MetricRailOffset)
	extension := specOf(t, prof, ideal.MetricSeatpostExtension)
	oval := specOf(t, prof, ideal.MetricHandlebarClampFit)
	stemLen := specOf(t, prof, ideal.MetricStemLength)
	stemAng := specOf(t, prof, ideal.MetricStemAngle)
	spacers := specOf(t, prof, ideal.MetricSpacers)

	for _, tc := range []struct {
		frame, size string
		a2c, rake   float64
	}{
		{"surly-krampus", "XL", 500, 51},
		{"binary-havok", "L", 499, 44},
		{"salsa-cutthroat", "56cm", 483, 51}, // gravel stem catalog (60–110mm)
	} {
		rows := []model.CanonicalComparisonRow{scanRow(t, tc.frame, tc.size, tc.a2c, tc.rake)}
		if err := canonicalizer.ApplyIdealCockpitTargets(rows, prof); err != nil {
			t.Fatalf("%s: ApplyIdealCockpitTargets: %v", tc.frame, err)
		}
		row := rows[0]
		can := row.Canonical
		if can.SelectedBRP == nil || can.CockpitFitResult == nil || can.SelectedHB == nil {
			t.Fatalf("%s: scan did not select a build", tc.frame)
		}

		if can.SeatpostExtensionMm != can.SelectedBRP.SeatAxisDistanceMm-row.Bike.SeatTubeLength {
			t.Errorf("%s: extension was not recomputed for the selected rail", tc.frame)
		}
		brpLost := func(b model.BRPPoint) float64 {
			ext := b.SeatAxisDistanceMm - row.Bike.SeatTubeLength
			return ideal.RowPointsLost(setback, b.BrpSetbackMm, &row) + ideal.RowPointsLost(rail, b.RailOffsetMm, &row) + ideal.RowPointsLost(extension, ext, &row)
		}
		chosen := brpLost(*can.SelectedBRP)
		for _, b := range can.PossibleBRPs {
			if brpLost(b) < chosen-1e-9 {
				t.Errorf("%s: rail %+.0fmm loses %.3f, cheaper than the chosen rail %+.0fmm (%.3f)",
					tc.frame, b.RailOffsetMm, brpLost(b), can.SelectedBRP.RailOffsetMm, chosen)
			}
		}

		if n := len(can.PossibleHBs); n != len(row.Bike.Stems)*len(row.Bike.GetSpacersMm()) {
			t.Fatalf("%s: %d cockpit candidates, want every stem × spacer", tc.frame, n)
		}
		cfr := can.CockpitFitResult
		cockpitLost := func(pt model.HBPoint) float64 {
			evalPt, err := geometry.EvaluatedPointForHB(pt, cfr.TargetRef, cfr.ReferenceBar)
			if err != nil {
				t.Fatal(err)
			}
			r, _ := geometry.EvaluateFitOval(cfr.TargetOval, evalPt.X, evalPt.Y)
			return ideal.OvalPointsLost(&model.CockpitFitResult{BestR: r}, oval.Cost) +
				ideal.RowPointsLost(stemLen, pt.Stem.LengthMm, &row) +
				ideal.RowPointsLost(stemAng, float64(pt.Stem.AngleDeg), &row) +
				ideal.RowPointsLost(spacers, pt.SpacerMm, &row)
		}
		chosenHB := cockpitLost(*can.SelectedHB)
		for _, pt := range can.PossibleHBs {
			if c := cockpitLost(pt); c < chosenHB-1e-9 {
				t.Errorf("%s: %.0fmm %+.0f° +%.0fmm loses %.3f, cheaper than the chosen %.0fmm %+.0f° +%.0fmm (%.3f)",
					tc.frame, pt.Stem.LengthMm, pt.Stem.AngleDeg, pt.SpacerMm, c,
					can.SelectedHB.Stem.LengthMm, can.SelectedHB.Stem.AngleDeg, can.SelectedHB.SpacerMm, chosenHB)
			}
		}
	}
}

// Moving the published reach (and, separately, the BRP) in small steps must move
// the cheapest build's points lost continuously: past a hardware limit the oval
// (or the setback band) absorbs the miss without a jump.
func TestApplyIdealCockpitTargets_HandOffsAreContinuous(t *testing.T) {
	prof := ideal.IdmatchProfile()
	base := scanRow(t, "surly-krampus", "XL", 500, 51)

	buildBands := map[ideal.MetricID]bool{
		ideal.MetricBrpSetback: true, ideal.MetricRailOffset: true, ideal.MetricHandlebarClampFit: true,
		ideal.MetricStemLength: true, ideal.MetricStemAngle: true, ideal.MetricSpacers: true,
	}
	total := func(row model.CanonicalComparisonRow) float64 {
		rows := []model.CanonicalComparisonRow{row}
		if err := canonicalizer.ApplyIdealCockpitTargets(rows, prof); err != nil {
			t.Fatalf("ApplyIdealCockpitTargets: %v", err)
		}
		res := 0.0
		for _, spec := range prof.ForSaddle(rows[0].Canonical.Saddle).Attributes {
			meta, ok := ideal.LookupAttribute(spec.ID)
			if !ok || !spec.IsScored() {
				continue
			}
			if !buildBands[spec.ID] {
				continue
			}
			if v, ok := meta.Extract(rows[0]); ok {
				res += ideal.Evaluate(spec, meta, v, &rows[0]).PointsLost
			}
		}
		return res
	}

	// Cockpit hand-off: sweep reach 80mm short to 80mm long of the frame (0.25mm steps).
	// The steepest band is the oval at 22 points per unit R over a 10mm semi-minor
	// axis, so no step may move the total by more than 22 × 0.25 / 10 plus slack.
	prev := math.NaN()
	for d := -80.0; d <= 80.0; d += 0.25 {
		row := base
		row.Canonical.ActiveReach = base.Canonical.ActiveReach + d
		got := total(row)
		if !math.IsNaN(prev) && math.Abs(got-prev) > 0.6 {
			t.Fatalf("reach %+0.2fmm: cockpit points lost jumped %.3f -> %.3f", d, prev, got)
		}
		prev = got
	}

	// Saddle hand-off: shift every rail position's setback 40mm each way (0.25mm steps).
	// The setback band is the steepest at 1.2 points per mm.
	prev = math.NaN()
	for d := -40.0; d <= 40.0; d += 0.25 {
		row := base
		row.Canonical.PossibleBRPs = append([]model.BRPPoint(nil), base.Canonical.PossibleBRPs...)
		for k := range row.Canonical.PossibleBRPs {
			row.Canonical.PossibleBRPs[k].BrpSetbackMm += d
		}
		row.Canonical.SelectedBRP = nil
		rows := []model.CanonicalComparisonRow{row}
		if err := canonicalizer.ApplyIdealCockpitTargets(rows, prof); err != nil {
			t.Fatalf("ApplyIdealCockpitTargets: %v", err)
		}
		brp := rows[0].Canonical.SelectedBRP
		got := ideal.RowPointsLost(specOf(t, prof, ideal.MetricBrpSetback), brp.BrpSetbackMm, &rows[0]) +
			ideal.RowPointsLost(specOf(t, prof, ideal.MetricRailOffset), brp.RailOffsetMm, &rows[0])
		if !math.IsNaN(prev) && math.Abs(got-prev) > 1.2*0.25+0.05 {
			t.Fatalf("setback shift %+0.2fmm: saddle points lost jumped %.3f -> %.3f", d, prev, got)
		}
		prev = got
	}
}

func TestExpandPlausibleForkSetups_AlwaysIncludesCadFork(t *testing.T) {
	for _, f := range frames.AllFrames() {
		setups, err := canonicalizer.ExpandPlausibleForkSetups(f, model.Opt[float64]{}, testCat.Forks)
		if err != nil {
			t.Fatalf("%s: %v", f.ID, err)
		}
		var cad []model.ComparisonSetup
		for _, s := range setups {
			if s.IsCadFork {
				cad = append(cad, s)
			}
		}
		if len(cad) != 1 {
			t.Errorf("%s: %d CAD fork candidates, want 1 (%+v)", f.ID, len(cad), f.CadFork)
			continue
		}
		if math.Abs(cad[0].A2cMm-f.CadFork.A2cMm) > 0.05 || math.Abs(cad[0].RakeMm-f.CadFork.RakeMm) > 0.05 ||
			cad[0].TravelMm != f.CadFork.TravelMm || cad[0].SagPct != f.CadFork.SagPct {
			t.Errorf("%s: CAD candidate %+v does not match CadFork %+v", f.ID, cad[0], f.CadFork)
		}
	}
}

func TestExpandPlausibleForkSetups_CadForkOutsideSagSweep(t *testing.T) {
	// binary-kinetik is drawn on a 120mm fork at 15% sag, which the 20/25/30% sweep never visits.
	f, err := frames.GetFrameByID("binary-kinetik")
	if err != nil {
		t.Fatal(err)
	}
	setups, err := canonicalizer.ExpandPlausibleForkSetups(*f, model.Opt[float64]{}, testCat.Forks)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range setups {
		if s.IsCadFork && s.DisplayLabel == "120mm @ 15% sag" && s.TravelMm == 120 {
			found = true
		}
	}
	if !found {
		t.Errorf("binary-kinetik: missing the 120mm @ 15%% sag CAD fork in %+v", setups)
	}

	// An explicit sag override keeps only a CAD fork that agrees with it.
	for _, sag := range []float64{20, 25, 30} {
		setups, err := canonicalizer.ExpandPlausibleForkSetups(*f, model.Some(sag), testCat.Forks)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range setups {
			if s.IsCadFork {
				t.Errorf("binary-kinetik @ sag %v: CAD fork %+v survives a conflicting override", sag, s)
			}
		}
	}
}

func TestCustomCatalogForkResolution(t *testing.T) {
	customCat := validator.Catalog{
		Forks: []model.ForkCatalogEntry{
			{
				ID:       "custom_sid_120",
				Name:     "Custom SID 120mm",
				TravelMm: 120,
				Standard: true,
				Options:  []model.ForkCatalogOption{{A2cMm: 530, RakeMm: 44}},
			},
			{
				ID:       "custom_rigid_510",
				Name:     "Custom Rigid 510mm",
				TravelMm: 0,
				Options:  []model.ForkCatalogOption{{A2cMm: 510, RakeMm: 44}},
			},
		},
		Stems: []model.StemCatalogEntry{
			{
				ID:            "custom_stems",
				LengthRangeMm: []float64{60, 70},
				AnglesDeg:     []model.Deg{-6},
			},
		},
	}

	frame := frames.RevelElJefe.Clone()
	frame.ID = "custom-el-jefe"
	frame.Stems = []string{"custom_stems"}
	frame.CadFork = model.ForkRef{Fork: "custom_sid_120", TravelMm: 120, SagPct: 20, A2cMm: 506, RakeMm: 44}
	frame.Forks = []model.ForkRef{
		{Fork: "custom_sid_120", TravelMm: 120, A2cMm: 530, RakeMm: 44},
		{Fork: "custom_rigid_510", A2cMm: 510, RakeMm: 44},
	}

	rider := catalog.DefaultRiderProfile()
	rider.SaddleHeightMm = 746.0
	row, err := canonicalizer.ResolveFrameForkSetupWithSaddle(frame, model.ComparisonSetup{
		FrameID: frame.ID,
		Size:    "L",
	}, rider, catalog.DefaultSaddle(), customCat)
	if err != nil {
		t.Fatalf("ResolveFrameForkSetupWithSaddle with custom catalog failed: %v", err)
	}
	if math.Abs(row.EffectiveA2c-506.0) > 0.05 {
		t.Errorf("default fork EffectiveA2c = %.1f, want 506.0 (20%% sagged from custom 530mm entry)", row.EffectiveA2c)
	}
	if row.DisplayLabel != "120mm @ 20% sag" {
		t.Errorf("default fork DisplayLabel = %q, want %q", row.DisplayLabel, "120mm @ 20% sag")
	}
	if len(row.Bike.Stems) != 2 {
		t.Errorf("expected 2 expanded stems from custom_stems, got %d (%+v)", len(row.Bike.Stems), row.Bike.Stems)
	}

	// Resolving the custom fork token explicitly uses the injected catalog.
	rf, err := canonicalizer.ResolveForkToken("custom_sid_120", model.Opt[float64]{}, frame, customCat.Forks)
	if err != nil {
		t.Fatalf("ResolveForkToken(custom_sid_120) failed: %v", err)
	}
	if math.Abs(rf.TargetA2c-506.0) > 0.05 {
		t.Errorf("ResolveForkToken(custom_sid_120) TargetA2c = %.1f, want 506.0", rf.TargetA2c)
	}
	if _, err := canonicalizer.ResolveForkToken("custom_sid_120", model.Opt[float64]{}, frame, catalog.CommonForkCatalog); err == nil {
		t.Errorf("expected ResolveForkToken(custom_sid_120) to fail against CommonForkCatalog")
	}

	// Expanding plausible forks uses the custom catalog's A2C rather than CommonForkCatalog.
	customCat.Forks[0].Options[0].A2cMm = 535
	setups, err := canonicalizer.ExpandPlausibleForkSetups(frame, model.Some(20.0), customCat.Forks)
	if err != nil {
		t.Fatalf("ExpandPlausibleForkSetups(customCat) failed: %v", err)
	}
	found511 := false
	for _, s := range setups {
		if s.TravelMm == 120 && s.SagPct == 20 && math.Abs(s.A2cMm-511.0) < 0.05 {
			found511 = true
		}
	}
	if !found511 {
		t.Errorf("ExpandPlausibleForkSetups did not produce 511.0mm (20%% sagged from custom 535mm static), got %+v", setups)
	}
}
