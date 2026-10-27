package fit_test

import (
	"testing"

	"bikegeo/pkg/fit"
	"bikegeo/pkg/model"
)

func sampleFrame() model.FrameModel {
	fork := model.ForkRef{
		Fork:   "fox34_sc_120",
		A2cMm:  530,
		RakeMm: 44,
		SagPct: model.Float64Ptr(0),
	}
	return model.FrameModel{
		ID:                 "test-bike",
		Manufacturer:       "TestMfg",
		Model:              "Trail",
		Year:               2024,
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: model.Float64Ptr(10.0),
		CadFork:            fork,
		DefaultFork:        fork,
		SupportedForks:     []model.ForkRef{fork},
		Sizes:              []string{"M", "L"},
		DefaultSize:        "L",
		Geometries: map[string]model.SizeGeometry{
			"M": {
				Reach:           430,
				Stack:           620,
				HtaDeg:          67.0,
				StaDeg:          74.0,
				Htl:             100,
				BbDrop:          60,
				Wheelbase:       1150,
				ChainstayLength: 435,
			},
			"L": {
				Reach:           455,
				Stack:           630,
				HtaDeg:          67.0,
				StaDeg:          74.0,
				Htl:             110,
				BbDrop:          60,
				Wheelbase:       1175,
				ChainstayLength: 435,
			},
		},
	}
}

func TestAnalyzeFit(t *testing.T) {
	frame := sampleFrame()
	res, err := fit.AnalyzeFit(frame, "stock")
	if err != nil {
		t.Fatalf("AnalyzeFit failed: %v", err)
	}

	if len(res.Residuals) != 2 {
		t.Errorf("Expected 2 size residuals, got %d", len(res.Residuals))
	}
	if res.BestFitStructuralA2c <= 0 {
		t.Errorf("Expected positive BestFitStructuralA2c, got %.2f", res.BestFitStructuralA2c)
	}
	if res.ImpliedSpacerMm == 0 {
		t.Errorf("Expected non-zero ImpliedSpacerMm, got %.2f", res.ImpliedSpacerMm)
	}
	if res.ImpliedStaticForkA2c <= 0 {
		t.Errorf("Expected positive ImpliedStaticForkA2c, got %.2f", res.ImpliedStaticForkA2c)
	}

	// Verify minimax property: max absolute delta vs best should equal spread/2
	spread := res.SpreadStructuralA2c
	for _, r := range res.Residuals {
		if r.DeltaVsBest > spread/2.0+0.01 || r.DeltaVsBest < -spread/2.0-0.01 {
			t.Errorf("DeltaVsBest %.2f exceeds spread/2 (spread=%.2f)", r.DeltaVsBest, spread)
		}
	}
}

func TestApplyAssignment(t *testing.T) {
	frame := sampleFrame()

	// 1. Frame-level spacer & slop
	if err := fit.ApplyAssignment(&frame, "stock", "", "spacer=12.5"); err != nil {
		t.Fatalf("Failed to set spacer: %v", err)
	}
	if frame.GetGeoHeadsetSpacerMm() != 12.5 {
		t.Errorf("Expected spacer 12.5, got %.1f", frame.GetGeoHeadsetSpacerMm())
	}

	if err := fit.ApplyAssignment(&frame, "stock", "", "slop.a2c=2.0"); err != nil {
		t.Fatalf("Failed to set slop: %v", err)
	}
	if frame.GeoSlopMm.GetA2c() != 2.0 {
		t.Errorf("Expected slop.a2c 2.0, got %.1f", frame.GeoSlopMm.GetA2c())
	}

	// 2. Fork-level attributes
	if err := fit.ApplyAssignment(&frame, "stock", "", "fork.a2c=540"); err != nil {
		t.Fatalf("Failed to set fork.a2c: %v", err)
	}
	if frame.CadFork.A2cMm != 540 {
		t.Errorf("Expected fork.a2c 540, got %.1f", frame.CadFork.A2cMm)
	}

	if err := fit.ApplyAssignment(&frame, "stock", "", "fork.rake=51"); err != nil {
		t.Fatalf("Failed to set fork.rake: %v", err)
	}
	if frame.CadFork.RakeMm != 51 {
		t.Errorf("Expected fork.rake 51, got %.1f", frame.CadFork.RakeMm)
	}

	if err := fit.ApplyAssignment(&frame, "stock", "", "sag=20"); err != nil {
		t.Fatalf("Failed to set sag: %v", err)
	}
	if frame.CadFork.SagPct == nil || *frame.CadFork.SagPct != 20 {
		t.Errorf("Expected sag 20, got %v", frame.CadFork.SagPct)
	}

	// 3. Per-size overrides
	if err := fit.ApplyAssignment(&frame, "stock", "", "M:stack=625"); err != nil {
		t.Fatalf("Failed to set M:stack: %v", err)
	}
	if frame.Geometries["M"].Stack != 625 {
		t.Errorf("Expected M stack 625, got %.1f", frame.Geometries["M"].Stack)
	}

	// 4. Default size fallback
	if err := fit.ApplyAssignment(&frame, "stock", "L", "reach=460"); err != nil {
		t.Fatalf("Failed to set reach on default size: %v", err)
	}
	if frame.Geometries["L"].Reach != 460 {
		t.Errorf("Expected L reach 460, got %.1f", frame.Geometries["L"].Reach)
	}

	// 5. Invalid assignments
	if err := fit.ApplyAssignment(&frame, "stock", "", "invalid_no_equal"); err == nil {
		t.Errorf("Expected error for missing '='")
	}
	if err := fit.ApplyAssignment(&frame, "stock", "", "NonExistent:stack=600"); err == nil {
		t.Errorf("Expected error for unknown size")
	}
}

func TestRunSweeps(t *testing.T) {
	frame := sampleFrame()

	tryFlags := []string{
		"fork.a2c=520,530",
		"spacer=10,12",
	}

	combos, err := fit.RunSweeps(frame, "stock", "L", tryFlags)
	if err != nil {
		t.Fatalf("RunSweeps failed: %v", err)
	}
	if len(combos) != 4 {
		t.Errorf("Expected 4 combinations (2x2), got %d", len(combos))
	}

	for _, c := range combos {
		if c.FitResult == nil {
			t.Errorf("Expected ComboResult to include FitResult")
		}
		if c.Description == "" {
			t.Errorf("Expected non-empty Description")
		}
	}
}

func TestRunSweepsCombinationLimit(t *testing.T) {
	frame := sampleFrame()

	// 10 x 10 x 3 = 300 > 200
	tryFlags := []string{
		"fork.a2c=500,505,510,515,520,525,530,535,540,545",
		"spacer=1,2,3,4,5,6,7,8,9,10",
		"fork.rake=44,48,51",
	}

	_, err := fit.RunSweeps(frame, "stock", "L", tryFlags)
	if err == nil {
		t.Fatalf("Expected error when exceeding 200 combinations, got nil")
	}
}

func TestSolveFit(t *testing.T) {
	frame := sampleFrame()
	combos, err := fit.SolveFit(frame, "stock", "L")
	if err != nil {
		t.Fatalf("SolveFit failed: %v", err)
	}
	if len(combos) == 0 {
		t.Fatalf("Expected SolveFit to return candidate combinations")
	}
	// Best candidate should have low delta A2C
	if combos[0].MaxAbsDeltaA2c > 5.0 {
		t.Errorf("Expected best candidate to have low delta A2C, got %.2f", combos[0].MaxAbsDeltaA2c)
	}
}
