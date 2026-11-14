package fit_test

import (
	"fmt"
	"strings"
	"testing"

	"bikegeo/pkg/fit"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/model"
)

func sampleFrame() model.FrameModel {
	fork := model.ForkRef{
		Fork:   "fox34_sc_120",
		A2cMm:  530,
		RakeMm: 44,
		SagPct: 0,
	}
	return model.FrameModel{
		ID:                 "test-bike",
		Manufacturer:       "TestMfg",
		Model:              "Trail",
		Year:               2024,
		Category:           model.CategoryHardtail,
		HeadsetCupType:     model.CupEC44,
		GeoHeadsetSpacerMm: 10.0,
		CadFork:            fork,
		Forks:              []model.ForkRef{fork},
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
	if frame.GeoSlopMm.A2c.OrZero() != 2.0 {
		t.Errorf("Expected slop.a2c 2.0, got %.1f", frame.GeoSlopMm.A2c.OrZero())
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
	if frame.CadFork.SagPct != 20 {
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
	if err := fit.ApplyAssignment(&frame, "stock", "", "slop.wb=1.0"); err != nil {
		t.Errorf("Expected slop.wb=1.0 to be accepted (allowances have no floor): %v", err)
	}
	if err := fit.ApplyAssignment(&frame, "stock", "", "slop.wb=-1.0"); err == nil {
		t.Errorf("Expected error for negative slop.wb")
	}
	if err := fit.ApplyAssignment(&frame, "stock", "", "slop.cross_config=1.0"); err == nil {
		t.Errorf("Expected error for removed slop.cross_config")
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

func TestSweepDetailIncludesFailureReason(t *testing.T) {
	// F-p4d-5: Sweep table includes Failure Reason and --detail prints frame+size [ERROR] notices.
	combos, err := fit.RunSweeps(frames.OtsoFenrirStainless, "stock", "L", []string{"sag=0,20"})
	if err != nil {
		t.Fatalf("RunSweeps failed: %v", err)
	}
	doc := fit.BuildSweepDocument(frames.OtsoFenrirStainless, combos, true)

	tables := doc.Tables()
	if len(tables) == 0 {
		t.Fatalf("expected sweep table")
	}
	sweep := tables[0]
	reasonCol := -1
	for i, col := range sweep.Columns {
		if col.Label == "Failure Reason" {
			reasonCol = i
			break
		}
	}
	if reasonCol < 0 {
		t.Fatalf("expected Failure Reason column in sweep table")
	}
	sawFailReason := false
	for _, r := range sweep.Rows {
		if r.Cells[reasonCol-1].Text == "FAIL" && strings.Contains(r.Cells[reasonCol].Text, "cannot have sag_pct") {
			sawFailReason = true
		}
	}
	if !sawFailReason {
		t.Errorf("expected sag=20 FAIL row to report ValidateForkRef error in Failure Reason column")
	}

	var allText strings.Builder
	for _, s := range doc.Sections {
		if s.Text != "" {
			allText.WriteString(s.Text)
			allText.WriteByte('\n')
		}
	}
	if !strings.Contains(allText.String(), "[ERROR]") || !strings.Contains(allText.String(), "cannot have sag_pct") {
		t.Errorf("expected --detail text to include [ERROR] notice for ValidateForkRef, got:\n%s", allText.String())
	}

	// Size-level failure (stack perturbation on Surly ECR:L)
	stackCombos, err := fit.RunSweeps(frames.SurlyECR, "stock", "L", []string{"stack=604.5,633"})
	if err != nil {
		t.Fatalf("RunSweeps(surly-ecr stack) failed: %v", err)
	}
	stackDoc := fit.BuildSweepDocument(frames.SurlyECR, stackCombos, true)
	var stackText strings.Builder
	for _, s := range stackDoc.Sections {
		if s.Text != "" {
			stackText.WriteString(s.Text)
			stackText.WriteByte('\n')
		}
	}
	if !strings.Contains(stackText.String(), "[ERROR] [Size L]") {
		t.Errorf("expected --detail text to include [ERROR] [Size L] notice, got:\n%s", stackText.String())
	}
}

func TestSweepInvokesSolverWhenAllFail(t *testing.T) {
	// F-p4d-6: When all --try perturbations fail, BuildSweepDocument invokes SolveFit.
	combos, err := fit.RunSweeps(frames.SurlyECR, "stock", "L", []string{"spacer=0"})
	if err != nil {
		t.Fatalf("RunSweeps failed: %v", err)
	}
	doc := fit.BuildSweepDocument(frames.SurlyECR, combos, false)
	foundCandidates := false
	for _, tbl := range doc.Tables() {
		if tbl.ID == "fit-candidates" {
			foundCandidates = true
			break
		}
	}
	if !foundCandidates {
		t.Errorf("expected fit-candidates table when all --try perturbations fail")
	}
}

func TestSolveFitFindsDeclaredSaggedConfigs(t *testing.T) {
	// F-p4d-6: SolveFit finds the declared CadFork + GeoHeadsetSpacerMm among PASS candidates.
	targets := []model.FrameModel{
		frames.MootsMountaineer,
		frames.SourPastaParty,
		frames.RevelElJefe,
		frames.EskerHayduke,
	}
	for _, fr := range targets {
		cands, err := fit.SolveFit(fr, "stock", fr.DefaultSize)
		if err != nil {
			t.Fatalf("SolveFit(%s) failed: %v", fr.ID, err)
		}
		wantA2c := fmt.Sprintf("cad_fork.a2c=%.0f", fr.CadFork.A2cMm)
		wantSag := fmt.Sprintf("cad_fork.sag=%.0f", fr.CadFork.SagPct)
		wantSpacer := fmt.Sprintf("spacer=%.0f", fr.GetGeoHeadsetSpacerMm())
		wantFork := fmt.Sprintf("cad_fork.fork=%s", fr.CadFork.Fork)

		found := false
		for _, c := range cands {
			if !c.IsValid {
				continue
			}
			if strings.Contains(c.Description, wantA2c) &&
				strings.Contains(c.Description, wantSag) &&
				strings.Contains(c.Description, wantSpacer) &&
				strings.Contains(c.Description, wantFork) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: declared config (%s %s %s %s) not found among top-10 PASS candidates",
				fr.ID, wantA2c, wantSag, wantSpacer, wantFork)
		}
	}
}
