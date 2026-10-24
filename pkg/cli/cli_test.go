package cli_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3/vt"

	"bikegeo/pkg/cli"
	"bikegeo/pkg/model"
	"bikegeo/pkg/tui"
)

func setupTestEnv(t *testing.T) (dataDir string, stateFile string) {
	t.Helper()
	tmp := t.TempDir()
	dataDir = filepath.Join(tmp, "bikes")
	stateFile = filepath.Join(tmp, "last_compare.json")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create temp bikes dir: %v", err)
	}

	entries, err := os.ReadDir("../../data/bikes")
	if err != nil {
		t.Fatalf("failed to read real bikes dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../../data/bikes", e.Name()))
		if err != nil {
			t.Fatalf("failed to read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, e.Name()), raw, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", e.Name(), err)
		}
	}

	return dataDir, stateFile
}

func runCLI(t *testing.T, dataDir, stateFile, stdin string, args ...string) (int, string, string) {
	t.Helper()
	fullArgs := append([]string{"--data-dir", dataDir, "--state-file", stateFile}, args...)
	var outBuf, errBuf bytes.Buffer
	code := cli.Run(fullArgs, strings.NewReader(stdin), &outBuf, &errBuf)
	return code, outBuf.String(), errBuf.String()
}

func TestCompare_SameFrameDifferentSagsAndRigid(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// Compare Binary Havok (L) with 110mm @ 20% sag, 110mm @ 30% sag, and its stock rigid 485mm fork
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "havok@110mm@20%", "havok@110mm@30%", "havok@rigid", "--format", "json")
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d (stderr: %s)", code, errOut)
	}

	var result struct {
		SaddleHeightMm float64                        `json:"saddle_height_mm"`
		Columns        []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, out)
	}

	if len(result.Columns) != 3 {
		t.Fatalf("Expected 3 columns, got %d", len(result.Columns))
	}

	// Column 1: 110mm (521mm static) @ 20% sag -> 521 - 22 = 499.0mm
	if result.Columns[0].EffectiveA2c != 499.0 {
		t.Errorf("Col 0 expected EffectiveA2c 499.0, got %.1f", result.Columns[0].EffectiveA2c)
	}
	// Column 2: 110mm (521mm static) @ 30% sag -> 521 - 33 = 488.0mm (steeper HTA & longer reach than 20% sag)
	if result.Columns[1].EffectiveA2c != 488.0 {
		t.Errorf("Col 1 expected EffectiveA2c 488.0, got %.1f", result.Columns[1].EffectiveA2c)
	}
	if result.Columns[1].Canonical.ActiveReach <= result.Columns[0].Canonical.ActiveReach {
		t.Errorf("Expected 30%% sag reach (%.1f) > 20%% sag reach (%.1f)",
			result.Columns[1].Canonical.ActiveReach, result.Columns[0].Canonical.ActiveReach)
	}
	// Column 3: rigid -> 485.0mm A2C, 0% sag
	if result.Columns[2].EffectiveA2c != 485.0 || result.Columns[2].SagPercent != 0 {
		t.Errorf("Col 2 expected rigid 485.0mm @ 0%% sag, got %.1fmm @ %.0f%%",
			result.Columns[2].EffectiveA2c, result.Columns[2].SagPercent)
	}

	// Verify ActualSetback is computed and present in both Baseline and Canonical
	if result.Columns[0].Canonical.ActualSetback <= 0 {
		t.Errorf("Expected positive Canonical.ActualSetback, got %.1f", result.Columns[0].Canonical.ActualSetback)
	}
	if result.Columns[0].Baseline.ActualSetback <= 0 {
		t.Errorf("Expected positive Baseline.ActualSetback, got %.1f", result.Columns[0].Baseline.ActualSetback)
	}
}

func TestCompare_MixedFramesForksAndSags(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// Binary Havok @ 110mm/20% sag, Otso Fenrir @ 120mm/25% sag, Otso Fenrir @ rigid
	code, out, errOut := runCLI(
		t, dataDir, stateFile, "",
		"compare",
		"havok@L@110mm@20%",
		"fenrir@L@120mm@25%",
		"fenrir@XL@rigid",
	)
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d (stderr: %s)", code, errOut)
	}

	if !strings.Contains(out, "Binary Havok (L)") {
		t.Errorf("Expected Binary Havok (L) in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "120mm @ 25% sag") {
		t.Errorf("Expected '120mm @ 25%% sag' in table output, got:\n%s", out)
	}
	if !strings.Contains(out, "Rigid (495mm A2C)") {
		t.Errorf("Expected 'Rigid (495mm A2C)' in table output, got:\n%s", out)
	}
	if strings.Contains(out, "Published CAD Mode") {
		t.Errorf("Expected 'Published CAD Mode' row to be removed from compare output, got:\n%s", out)
	}
	if strings.Contains(out, "Base:") || strings.Contains(out, "Base AETT:") {
		t.Errorf("Expected 'Base:' lines to be removed from compare output, got:\n%s", out)
	}
	if !strings.Contains(out, "Setback @") {
		t.Errorf("Expected 'Setback @' in compare output, got:\n%s", out)
	}
}

func TestCompare_EnforcesFrameForkSagAndValidationConstraints(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Requesting an unsupported suspension travel size (Havok supports 100mm, 110mm, 120mm, rigid)
	code, _, errOut := runCLI(t, dataDir, stateFile, "", "compare", "havok@150mm")
	if code == 0 {
		t.Errorf("Expected error when requesting unsupported 150mm fork on Binary Havok")
	}
	if !strings.Contains(errOut, "does not support a 150mm suspension fork") {
		t.Errorf("Expected unsupported fork message, got: %s", errOut)
	}

	// 2. Requesting sag on a rigid fork (e.g. havok@rigid@20% or fenrir@20% since fenrir defaults to rigid)
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "havok@rigid@20%")
	if code == 0 {
		t.Errorf("Expected error when applying 20%% sag to rigid fork")
	}
	if !strings.Contains(errOut, "cannot apply 20% sag to rigid fork") {
		t.Errorf("Expected rigid sag constraint error, got: %s", errOut)
	}

	// 3. Fixed-rigid-only frame: create one via `init`, verify it works with no fork/sag args and rejects sag or suspension forks
	code, _, errOut = runCLI(
		t, dataDir, stateFile, "",
		"init",
		"--manufacturer", "Surly",
		"--model", "Karate Monkey Rigid",
		"--supported-travel", "none",
		"--rigid-a2c", "483",
		"--save",
	)
	if code != 0 {
		t.Fatalf("Failed to init fixed rigid frame: %s", errOut)
	}

	// Comparing fixed rigid frame with no modifiers should succeed seamlessly at 0% sag
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "surly-karate-monkey-rigid")
	if code != 0 {
		t.Fatalf("Expected fixed rigid frame comparison to succeed, got: %s", errOut)
	}
	if !strings.Contains(out, "Rigid (483mm A2C)") {
		t.Errorf("Expected Rigid (483mm A2C) in output, got:\n%s", out)
	}

	// Asking for sag on the fixed rigid frame must fail
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "surly-karate-monkey-rigid@20%")
	if code == 0 {
		t.Errorf("Expected error when applying sag to fixed-rigid frame")
	}
	if !strings.Contains(errOut, "cannot apply 20% sag to rigid fork") {
		t.Errorf("Expected rigid sag error, got: %s", errOut)
	}

	// Asking for a suspension fork on the fixed rigid frame must fail
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "surly-karate-monkey-rigid@120mm")
	if code == 0 {
		t.Errorf("Expected error when requesting suspension fork on fixed-rigid frame")
	}
	if !strings.Contains(errOut, "fixed rigid fork") {
		t.Errorf("Expected fixed rigid fork error, got: %s", errOut)
	}

	// 4. Centralized validation gate: put a frame into an invalid state (impossible stack=500) and verify compare refuses to use it
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "set", "surly-karate-monkey-rigid:L", "stack=500")
	if code != 0 {
		t.Fatalf("set failed: %s", errOut)
	}
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "surly-karate-monkey-rigid")
	if code == 0 {
		t.Errorf("Expected compare to reject frame that fails validation")
	}
	if !strings.Contains(errOut, "failed validation") {
		t.Errorf("Expected validation failure message from compare, got: %s", errOut)
	}
}

func TestCompare_StatePersistenceAndIncrementalTweaks(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Issue initial comparison with --save
	code, _, errOut := runCLI(t, dataDir, stateFile, "", "compare", "--save", "havok:ML@110mm@20%", "fenrir:L@rigid")
	if code != 0 {
		t.Fatalf("Initial compare failed: %s", errOut)
	}

	// 2. Re-issue bare `compare` with no arguments -> should replay the exact 2 columns
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "--format", "json")
	if code != 0 {
		t.Fatalf("Bare compare replay failed: %s", errOut)
	}
	var saved struct {
		SaddleHeightMm float64                        `json:"saddle_height_mm"`
		Columns        []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(saved.Columns) != 2 {
		t.Fatalf("Expected 2 saved columns, got %d", len(saved.Columns))
	}
	if saved.Columns[0].Setup.FrameID != "binary-havok" || saved.Columns[0].Setup.Size != "ML" {
		t.Errorf("Expected col 0 to be binary-havok (ML), got %+v", saved.Columns[0].Setup)
	}

	// 3. Incremental tweak with --save: add fenrir@110mm@20% and remove column 2
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "compare", "--save", "+fenrir:XL@110mm@20%", "-2", "--format", "json")
	if code != 0 {
		t.Fatalf("Incremental compare failed: %s", errOut)
	}
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(saved.Columns) != 2 || saved.Columns[1].Setup.FrameID != "otso-fenrir-stainless" {
		t.Errorf("Expected 2nd column to be otso-fenrir-stainless, got %+v", saved.Columns)
	}

	// 4. Verify removed CLI flags on compare are rejected
	for _, removedFlag := range []string{"--all-sizes", "--fork=120mm", "--ref=1", "--sag=20", "--search=havok", "-q=havok", "--seat-height=775", "--size=L"} {
		code, _, _ := runCLI(t, dataDir, stateFile, "", "compare", removedFlag)
		if code == 0 {
			t.Errorf("Expected compare %s to fail after flag removal, but got exit code 0", removedFlag)
		}
	}
}

func TestCompare_InteractiveMode(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// Simulate interactive session:
	// - c (clear and add first column: "havok", size "L", fork "rigid" -> sag prompt is skipped!)
	// - a (add second column: "fenrir", size "L", fork "120mm", sag "25")
	// - p default (set profile)
	// - s 770 (set saddle height to 770)
	// - Enter (run comparison)
	stdin := strings.Join([]string{
		"c",
		"havok",
		"L",
		"rigid",
		"a",
		"fenrir",
		"L",
		"120mm",
		"25",
		"p default",
		"s 770",
		"",
	}, "\n") + "\n"

	code, out, errOut := runCLI(t, dataDir, stateFile, stdin, "compare", "-i", "--transpose")
	if code != 0 {
		t.Fatalf("Interactive compare failed (code %d): %s\nOut:\n%s", code, errOut, out)
	}

	if !strings.Contains(out, "Saddle Height: 770mm") {
		t.Errorf("Expected Saddle Height 770mm in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Ideal Profile: default") {
		t.Errorf("Expected Ideal Profile: default in transposed output, got:\n%s", out)
	}
	if !strings.Contains(out, "120mm @ 25%") {
		t.Errorf("Expected Otso Fenrir at 120mm @ 25%% in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Setback") {
		t.Errorf("Expected 'Setback' header in transposed output, got:\n%s", out)
	}
}

func TestValidate_CoherentAndInconsistentFrames(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Validate Binary Havok and Otso Fenrir Stainless (--all)
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "validate", "--all")
	if code != 0 {
		t.Fatalf("Expected exit code 0 for valid frames, got %d: %s\nOut:\n%s", code, errOut, out)
	}
	if !strings.Contains(out, "binary-havok") || !strings.Contains(out, "otso-fenrir-stainless") {
		t.Errorf("Expected both binary-havok and otso-fenrir-stainless in summary output, got:\n%s", out)
	}

	// 2. Single-frame validate on Otso Fenrir shows CAD Headset Spacer = 0.0 mm and the EC44 mismatch warning
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "validate", "otso-fenrir-stainless")
	if code != 0 {
		t.Fatalf("Expected exit code 0 for otso-fenrir-stainless, got %d: %s", code, errOut)
	}
	if !strings.Contains(out, "CAD Headset Spacer Differs From Physical Headset Cup") {
		t.Errorf("Expected CAD Headset Spacer warning in detailed output, got:\n%s", out)
	}

	// 3. Setting geo_headset_spacer_mm=12 on Otso Fenrir fails validation (no auto-detection)
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "set", "otso-fenrir-stainless", "geo_headset_spacer_mm=12")
	if code != 0 {
		t.Fatalf("set geo_headset_spacer_mm failed: %s", errOut)
	}
	code, out, _ = runCLI(t, dataDir, stateFile, "", "validate", "otso-fenrir-stainless")
	if code == 0 {
		t.Errorf("Expected non-zero exit code for otso-fenrir-stainless with geo_headset_spacer_mm=12")
	}

	// 4. Break a frame's stack so it fails Master Stack Equation and verify validate returns exit code 1
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "set", "binary-havok:L", "stack=590")
	if code != 0 {
		t.Fatalf("set failed: %s", errOut)
	}
	code, out, _ = runCLI(t, dataDir, stateFile, "", "validate", "binary-havok")
	if code == 0 {
		t.Errorf("Expected non-zero exit code for invalidated frame")
	}
	if !strings.Contains(out, "VALIDATION ISSUES DETECTED (INVALID)") {
		t.Errorf("Expected INVALID status in output, got:\n%s", out)
	}
}

func TestListForksAndCRUD(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. List frames in Markdown format
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "list", "-q", "havok", "--format", "md")
	if code != 0 {
		t.Fatalf("list failed: %s", errOut)
	}
	if !strings.Contains(out, "| binary-havok |") {
		t.Errorf("Expected markdown table row for binary-havok, got:\n%s", out)
	}

	// 2. List forks in CSV format
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "forks", "--format", "csv")
	if code != 0 {
		t.Fatalf("forks failed: %s", errOut)
	}
	if !strings.Contains(out, "fox34_130,Fox Float 34 130mm") {
		t.Errorf("Expected fox34_130 in CSV output, got:\n%s", out)
	}

	// 3. Set a geometry field on a bike
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "set", "binary-havok:L", "reach=400")
	if code != 0 {
		t.Fatalf("set failed: %s", errOut)
	}
	if !strings.Contains(out, "Reach: 400.0mm") {
		t.Errorf("Expected updated reach 400.0mm, got: %s", out)
	}

	// 4. Delete a single size from binary-havok
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "delete", "binary-havok:XL")
	if code != 0 {
		t.Fatalf("delete size failed: %s", errOut)
	}
	if !strings.Contains(out, "Deleted size XL from frame binary-havok") {
		t.Errorf("Unexpected delete size output: %s", out)
	}
}

func TestCompare_ScoringAndFlags(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Default compare has scoring enabled and sorted by score descending
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"hayduke:S3@120mm@30%", "havok:XL@rigid", "--transpose", "--save")
	if code != 0 {
		t.Fatalf("compare failed: %s", errOut)
	}
	if !strings.Contains(out, "Score") {
		t.Errorf("Expected 'Score' column in transposed compare, got:\n%s", out)
	}
	if !strings.Contains(out, "68 (#1)") {
		t.Errorf("Expected '68 (#1)' for Hayduke S3, got:\n%s", out)
	}
	if !strings.Contains(out, "60 (#2)") {
		t.Errorf("Expected '60 (#2)' for Havok XL, got:\n%s", out)
	}
	if !strings.Contains(out, "Biggest Losses vs. Ideal:") {
		t.Errorf("Expected 'Biggest Losses vs. Ideal:' section in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Head-to-Head Deciding Factors:") {
		t.Errorf("Expected 'Head-to-Head Deciding Factors:' section in output, got:\n%s", out)
	}
	// Verify Hayduke (#1) appears before Havok (#2) in output
	haydukeIdx := strings.Index(out, "Esker Hayduke (S3)")
	havokIdx := strings.Index(out, "Binary Havok (XL)")
	if haydukeIdx == -1 || havokIdx == -1 || haydukeIdx > havokIdx {
		t.Errorf("Expected Hayduke before Havok due to sorting by score, got hayduke=%d, havok=%d", haydukeIdx, havokIdx)
	}

	// 2. --no-score flag disables scoring, disables sorting, and omits footers
	code, outNoScore, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"havok:XL@rigid", "hayduke:S3@120mm@30%", "--transpose", "--no-score")
	if code != 0 {
		t.Fatalf("compare --no-score failed: %s", errOut)
	}
	if strings.Contains(outNoScore, "Score") {
		t.Errorf("Expected no 'Score' column with --no-score, got:\n%s", outNoScore)
	}
	if strings.Contains(outNoScore, "Biggest Losses vs. Ideal:") {
		t.Errorf("Expected no footer with --no-score, got:\n%s", outNoScore)
	}
	// Havok was listed first in CLI args, so it should appear first without scoring
	havokIdx = strings.Index(outNoScore, "Binary Havok (XL)")
	haydukeIdx = strings.Index(outNoScore, "Esker Hayduke (S3)")
	if havokIdx == -1 || haydukeIdx == -1 || havokIdx > haydukeIdx {
		t.Errorf("Expected Havok before Hayduke without score sorting, got havok=%d, hayduke=%d", havokIdx, haydukeIdx)
	}

	// 3. --score-detail flag outputs second detail table
	code, outDetail, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"hayduke:S3@120mm@30%", "havok:XL@rigid", "--score-detail")
	if code != 0 {
		t.Fatalf("compare --score-detail failed: %s", errOut)
	}
	if !strings.Contains(outDetail, "Scoring Breakdown") {
		t.Errorf("Expected 'Scoring Breakdown' section with --score-detail, got:\n%s", outDetail)
	}
	if !strings.Contains(outDetail, "Spare Vert") || !strings.Contains(outDetail, "Chainstay") {
		t.Errorf("Expected criteria headers in score detail table, got:\n%s", outDetail)
	}
	if !strings.Contains(outDetail, "59 (-6)") {
		t.Errorf("Expected '59 (-6)' for Hayduke Spare Vert in score detail table, got:\n%s", outDetail)
	}
	if !strings.Contains(outDetail, "68 (-32)") {
		t.Errorf("Expected '68 (-32)' for Hayduke Total in score detail table, got:\n%s", outDetail)
	}

	// 4. JSON format output includes per-column score and top-level explanations
	code, outJSON, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"hayduke:S3@120mm@30%", "havok:XL@rigid", "--format", "json")
	if code != 0 {
		t.Fatalf("compare --format json failed: %s", errOut)
	}
	var jsonRes struct {
		Columns []struct {
			Setup struct {
				FrameID string `json:"frame_id"`
			} `json:"setup"`
			Score struct {
				Total    float64 `json:"total"`
				Rank     int     `json:"rank"`
				Criteria []struct {
					ID         string  `json:"id"`
					SubScore   float64 `json:"sub_score"`
					PointsLost float64 `json:"points_lost"`
				} `json:"criteria"`
			} `json:"score"`
		} `json:"columns"`
		Explanations struct {
			Losses     []string `json:"losses"`
			HeadToHead []string `json:"head_to_head"`
		} `json:"explanations"`
	}
	if err := json.Unmarshal([]byte(outJSON), &jsonRes); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, outJSON)
	}
	if len(jsonRes.Columns) != 2 {
		t.Fatalf("Expected 2 columns in JSON, got %d", len(jsonRes.Columns))
	}
	if jsonRes.Columns[0].Score.Total == 0 || jsonRes.Columns[0].Score.Rank == 0 {
		t.Errorf("Expected populated score for column 0: %+v", jsonRes.Columns[0].Score)
	}
	if len(jsonRes.Columns[0].Score.Criteria) != 10 {
		t.Errorf("Expected 10 criteria in column score, got %d", len(jsonRes.Columns[0].Score.Criteria))
	}
	var foundSpareVertLost bool
	for _, crit := range jsonRes.Columns[0].Score.Criteria {
		if crit.ID == "sparevert" && math.Round(crit.PointsLost) == 6 {
			foundSpareVertLost = true
			break
		}
	}
	if !foundSpareVertLost {
		t.Errorf("Expected sparevert PointsLost to be ~6 in JSON output, got: %+v", jsonRes.Columns[0].Score.Criteria)
	}
	if len(jsonRes.Explanations.Losses) != 2 {
		t.Errorf("Expected 2 loss explanations, got %d", len(jsonRes.Explanations.Losses))
	}
	if len(jsonRes.Explanations.HeadToHead) != 1 {
		t.Errorf("Expected 1 head-to-head explanation, got %d", len(jsonRes.Explanations.HeadToHead))
	}

	// 5. CSV format includes Score header but no footer
	code, outCSV, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"hayduke:S3@120mm@30%", "havok:XL@rigid", "--format", "csv")
	if code != 0 {
		t.Fatalf("compare --format csv failed: %s", errOut)
	}
	if !strings.HasPrefix(outCSV, "#,Frame (Size),Score,") {
		t.Errorf("Expected CSV header starting with '#,Frame (Size),Score,', got: %s", outCSV)
	}
	if strings.Contains(outCSV, "Biggest Losses") || strings.Contains(outCSV, "Head-to-Head") {
		t.Errorf("Expected no footer in CSV output, got:\n%s", outCSV)
	}

	// 6. Markdown format includes table and footer markdown lists
	code, outMD, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"hayduke:S3@120mm@30%", "havok:XL@rigid", "--format", "md")
	if code != 0 {
		t.Fatalf("compare --format md failed: %s", errOut)
	}
	if !strings.Contains(outMD, "#### Biggest Losses vs. Ideal") {
		t.Errorf("Expected markdown H4 for losses, got:\n%s", outMD)
	}
	if !strings.Contains(outMD, "#### Head-to-Head Deciding Factors") {
		t.Errorf("Expected markdown H4 for head-to-head, got:\n%s", outMD)
	}
}

func TestCompare_TumbleweedSunlinerFixedForkConstraints(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Bare compare on Sunliner (defaults to L, rigid 440mm fork, 0% sag)
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "sunliner")
	if code != 0 {
		t.Fatalf("Expected compare sunliner to succeed, got (code %d): %s", code, errOut)
	}
	if !strings.Contains(out, "Tumbleweed Sunliner (L)") {
		t.Errorf("Expected Tumbleweed Sunliner (L) in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Rigid (440mm A2C)") {
		t.Errorf("Expected Rigid (440mm A2C) in output, got:\n%s", out)
	}

	// 2. Size modifier works (sunliner:M or sunliner:S)
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "compare", "sunliner:M", "sunliner@rigid")
	if code != 0 {
		t.Fatalf("Expected compare sunliner:M to succeed: %s", errOut)
	}
	if !strings.Contains(out, "Tumbleweed Sunliner (M)") {
		t.Errorf("Expected Tumbleweed Sunliner (M) in output, got:\n%s", out)
	}

	// 3. Attempting to assign a suspension fork travel (e.g. 120mm) must fail
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "sunliner@120mm")
	if code == 0 {
		t.Errorf("Expected compare sunliner@120mm to fail because Sunliner has a fixed rigid fork")
	}
	if !strings.Contains(errOut, "has a fixed rigid fork (440mm A2C) and does not support suspension fork \"120mm\"") {
		t.Errorf("Expected fixed rigid fork rejection message, got: %s", errOut)
	}

	// 4. Attempting to assign a suspension fork preset (e.g. fox34_120) must fail
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "sunliner@fox34_120")
	if code == 0 {
		t.Errorf("Expected compare sunliner@fox34_120 to fail")
	}
	if !strings.Contains(errOut, "has a fixed rigid fork (440mm A2C) and does not support suspension fork \"fox34_120\"") {
		t.Errorf("Expected fixed rigid fork rejection message, got: %s", errOut)
	}

	// 5. Attempting to assign sag (e.g. 20%) must fail
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "sunliner@20%")
	if code == 0 {
		t.Errorf("Expected compare sunliner@20%% to fail because rigid forks cannot have sag")
	}
	if !strings.Contains(errOut, "cannot apply 20% sag to rigid fork on \"Tumbleweed Sunliner\"") {
		t.Errorf("Expected rigid fork sag rejection message, got: %s", errOut)
	}

	// 6. Interactive compare: selecting sunliner automatically skips fork and sag prompts
	stdin := strings.Join([]string{
		"c",        // Clear and add first column
		"sunliner", // Frame
		"M",        // Size
		// Notice: fork and sag prompts are skipped!
		"", // Empty line (Enter) to run comparison
	}, "\n") + "\n"

	code, out, errOut = runCLI(t, dataDir, stateFile, stdin, "compare", "-i")
	if code != 0 {
		t.Fatalf("Interactive compare failed: %s\nOut:\n%s", errOut, out)
	}
	if !strings.Contains(out, "Fork: rigid (440mm) (fixed for this frame)") {
		t.Errorf("Expected interactive session to report fixed fork, got:\n%s", out)
	}
	if !strings.Contains(out, "Tumbleweed Sunliner (M)") {
		t.Errorf("Expected comparison table to contain Sunliner (M), got:\n%s", out)
	}
}

func TestCompare_DisambiguateDuplicateFrameSize(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Conditional disambiguation with different fork travels
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"fenrir:L@110mm@20%", "fenrir:L@120mm@20%", "hayduke:S3@110mm@20%", "--transpose", "--score-detail")
	if code != 0 {
		t.Fatalf("compare failed (code %d): %s", code, errOut)
	}

	if !strings.Contains(out, "Otso Fenrir (L) @ 110mm") {
		t.Errorf("Expected 'Otso Fenrir (L) @ 110mm' in disambiguated compare output, got:\n%s", out)
	}
	if !strings.Contains(out, "Otso Fenrir (L) @ 120mm") {
		t.Errorf("Expected 'Otso Fenrir (L) @ 120mm' in disambiguated compare output, got:\n%s", out)
	}
	// Verify singleton is NOT suffixed
	if !strings.Contains(out, "Esker Hayduke (S3)") || strings.Contains(out, "Esker Hayduke (S3) @") {
		t.Errorf("Expected singleton 'Esker Hayduke (S3)' without fork suffix, got:\n%s", out)
	}
	// Verify subtitle ideal profile
	if !strings.Contains(out, "Ideal Profile: default") {
		t.Errorf("Expected subtitle to include ideal profile, got:\n%s", out)
	}
	// Verify score detail table contains disambiguated names
	if !strings.Contains(out, "Scoring Breakdown") {
		t.Errorf("Expected 'Scoring Breakdown' section, got:\n%s", out)
	}

	// 2. Disambiguation with same travel but different sag
	code, outSag, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"fenrir:L@110mm@20%", "fenrir:L@110mm@30%", "--transpose")
	if code != 0 {
		t.Fatalf("compare same travel different sag failed: %s", errOut)
	}
	if !strings.Contains(outSag, "Otso Fenrir (L) @ 110mm @ 20%") {
		t.Errorf("Expected 'Otso Fenrir (L) @ 110mm @ 20%%' in output, got:\n%s", outSag)
	}
	if !strings.Contains(outSag, "Otso Fenrir (L) @ 110mm @ 30%") {
		t.Errorf("Expected 'Otso Fenrir (L) @ 110mm @ 30%%' in output, got:\n%s", outSag)
	}

	// 3. Fallback when setups are completely identical
	code, outIdentical, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"fenrir:L@110mm@20%", "fenrir:L@110mm@20%", "--transpose")
	if code != 0 {
		t.Fatalf("compare identical setups failed: %s", errOut)
	}
	if !strings.Contains(outIdentical, "Otso Fenrir (L) @ 110mm @ 20% (#1)") {
		t.Errorf("Expected fallback (#1) for first identical setup, got:\n%s", outIdentical)
	}
	if !strings.Contains(outIdentical, "Otso Fenrir (L) @ 110mm @ 20% (#2)") {
		t.Errorf("Expected fallback (#2) for second identical setup, got:\n%s", outIdentical)
	}

	// 4. Targeted removal of specific fork setup via -token@fork
	// Set initial state with 2 Fenrirs and 1 Hayduke
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "--save",
		"fenrir:L@110mm@20%", "fenrir:L@120mm@20%", "hayduke:S3@110mm@20%")
	if code != 0 {
		t.Fatalf("initial compare failed: %s", errOut)
	}
	// Remove only the 110mm Fenrir
	code, outRemoved, errOut := runCLI(t, dataDir, stateFile, "", "compare",
		"-fenrir:L@110mm", "--transpose")
	if code != 0 {
		t.Fatalf("targeted fork removal failed: %s", errOut)
	}
	if strings.Contains(outRemoved, "Otso Fenrir (L) @ 110mm") {
		t.Errorf("Expected 110mm Fenrir to be removed, got:\n%s", outRemoved)
	}
	// Since Fenrir L now appears only once, it should NOT have the @ 120mm suffix!
	if !strings.Contains(outRemoved, "Otso Fenrir (L)") || strings.Contains(outRemoved, "Otso Fenrir (L) @ 120mm") {
		t.Errorf("Expected remaining Fenrir to revert to clean 'Otso Fenrir (L)' without suffix, got:\n%s", outRemoved)
	}
	if !strings.Contains(outRemoved, "Esker Hayduke (S3)") {
		t.Errorf("Expected Esker Hayduke (S3) to remain in comparison, got:\n%s", outRemoved)
	}
}

func TestCompare_AllSizesExpansion(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Full replacement with seven-expedition-926f:all@120mm in JSON format
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "--save", "seven-expedition-926f:all@120mm", "--format", "json")
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d (stderr: %s)", code, errOut)
	}

	var result struct {
		IdealProfile string                         `json:"ideal_profile"`
		Columns      []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, out)
	}

	// Seven Expedition 926f has 24 sizes (12 Standard 'S', 12 Endurance 'E')
	if len(result.Columns) != 24 {
		t.Fatalf("Expected 24 columns for Seven Expedition :all, got %d", len(result.Columns))
	}

	// Verify all columns are Seven Expedition with 120mm fork
	for i, col := range result.Columns {
		if col.Setup.FrameID != "seven-expedition-926f" {
			t.Errorf("Col %d expected FrameID 'seven-expedition-926f', got %q", i, col.Setup.FrameID)
		}
		if col.SetupFork.TravelMm != 120 {
			t.Errorf("Col %d expected 120mm fork travel, got %.0fmm", i, col.SetupFork.TravelMm)
		}
	}

	if result.IdealProfile != "default" {
		t.Errorf("Expected IdealProfile 'default', got %q", result.IdealProfile)
	}

	// Verify saved state automatically enabled Transpose since 24 > 4
	savedData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}
	var savedState model.SavedComparisonState
	if err := json.Unmarshal(savedData, &savedState); err != nil {
		t.Fatalf("Failed to parse saved state: %v", err)
	}
	if !savedState.Transpose {
		t.Errorf("Expected savedState.Transpose to be auto-enabled (true) for 24 columns, got false")
	}

	// 2. Incremental addition: start with Fenrir L, then add El Jefe :all (4 sizes)
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "--save", "fenrir:L@110mm@20%")
	if code != 0 {
		t.Fatalf("compare reset failed: %s", errOut)
	}
	code, outAdd, errOut := runCLI(t, dataDir, stateFile, "", "compare", "+revel-el-jefe:all@120mm", "--format", "json")
	if code != 0 {
		t.Fatalf("compare +revel-el-jefe:all failed: %s", errOut)
	}
	var addResult struct {
		Columns []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(outAdd), &addResult); err != nil {
		t.Fatalf("Failed to parse addResult: %v", err)
	}
	// 1 Fenrir + 4 El Jefe sizes = 5 columns
	if len(addResult.Columns) != 5 {
		t.Fatalf("Expected 5 columns (1 Fenrir + 4 El Jefe), got %d", len(addResult.Columns))
	}
	if addResult.Columns[0].Setup.FrameID != "otso-fenrir-stainless" {
		t.Errorf("Expected first column to remain Otso Fenrir, got %s",
			addResult.Columns[0].Setup.FrameID)
	}

	// 3. Removal via -revel-el-jefe:all removes all 4 El Jefe columns
	code, outRm, errOut := runCLI(t, dataDir, stateFile, "", "compare", "-revel-el-jefe:all", "--format", "json")
	if code != 0 {
		t.Fatalf("compare -revel-el-jefe:all failed: %s", errOut)
	}
	var rmResult struct {
		Columns []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(outRm), &rmResult); err != nil {
		t.Fatalf("Failed to parse rmResult: %v", err)
	}
	if len(rmResult.Columns) != 1 || rmResult.Columns[0].Setup.FrameID != "otso-fenrir-stainless" {
		t.Errorf("Expected only Fenrir to remain after -revel-el-jefe:all, got %d columns", len(rmResult.Columns))
	}

	// 4. Default fork fallback when fork modifier is omitted
	code, outDefFork, errOut := runCLI(t, dataDir, stateFile, "", "compare", "revel-el-jefe:all", "--format", "json")
	if code != 0 {
		t.Fatalf("compare revel-el-jefe:all (default fork) failed: %s", errOut)
	}
	var defForkResult struct {
		Columns []model.CanonicalComparisonRow `json:"columns"`
	}
	if err := json.Unmarshal([]byte(outDefFork), &defForkResult); err != nil {
		t.Fatalf("Failed to parse defForkResult: %v", err)
	}
	if len(defForkResult.Columns) != 4 {
		t.Fatalf("Expected 4 El Jefe columns, got %d", len(defForkResult.Columns))
	}
	// El Jefe default fork is 120mm travel
	for i, col := range defForkResult.Columns {
		if col.SetupFork.TravelMm != 120 {
			t.Errorf("Col %d expected default fork 120mm, got %.0fmm", i, col.SetupFork.TravelMm)
		}
	}
}

func TestCompare_HelpOutputDocumentsSemantics(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	testCases := [][]string{
		{"compare", "--help"},
		{"compare", "-h"},
		{"compare", "help"},
	}

	for _, args := range testCases {
		code, out, errOut := runCLI(t, dataDir, stateFile, "", args...)
		if code != 0 {
			t.Fatalf("Expected exit code 0 for %v, got %d (err: %s)", args, code, errOut)
		}

		requiredSections := []string{
			"Comparison Specs:",
			"Frame Identifier:",
			"All sizes (\":all\" or \"@all\"):",
			"Comparison State & Column Management:",
			"State Persistence:",
			"Full Replacement (Bare Specs):",
			"Incremental Addition ('+spec'):",
			"Incremental Removal ('-spec' or '-col_num'):",
			"--ideal <name>",
			"Table Layout & Auto-Transposition:",
			"Scoring & Ranking:",
			"-save[=bool], --save[=bool]",
			"Examples:",
		}

		for _, sec := range requiredSections {
			if !strings.Contains(out, sec) {
				t.Errorf("For args %v, expected help text to contain %q, but got:\n%s", args, sec, out)
			}
		}
	}
}

func TestCompare_OptionalPersistence(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Non-interactive compare without --save does NOT create or modify stateFile
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "havok:L@110mm@20%", "fenrir:L@rigid")
	if code != 0 {
		t.Fatalf("compare failed: %s (out: %s)", errOut, out)
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Errorf("Expected stateFile to NOT exist after non-save compare, but got err: %v", err)
	}

	// 2. Non-interactive compare with --save DOES create/modify stateFile
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "--save", "havok:L@110mm@20%", "fenrir:L@rigid")
	if code != 0 {
		t.Fatalf("compare --save failed: %s", errOut)
	}
	afterSaveFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Expected stateFile to exist after compare --save, got err: %v", err)
	}
	saveModTime := afterSaveFi.ModTime()

	// 2b. Non-interactive compare with explicit --save=false does NOT modify stateFile
	time.Sleep(20 * time.Millisecond)
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "--save=false", "ritchey-ultra:L@120mm@20%")
	if code != 0 {
		t.Fatalf("compare --save=false failed: %s", errOut)
	}
	afterSaveFalseFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat stateFile: %v", err)
	}
	if !afterSaveFalseFi.ModTime().Equal(saveModTime) {
		t.Errorf("Expected stateFile modtime to remain %v with --save=false, but got %v", saveModTime, afterSaveFalseFi.ModTime())
	}

	// 3. Subsequent non-interactive compare without --save does NOT modify stateFile
	time.Sleep(20 * time.Millisecond)
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "compare", "esker-hayduke:S3@110mm@20%")
	if code != 0 {
		t.Fatalf("compare without save failed: %s", errOut)
	}
	afterNonSaveFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat stateFile: %v", err)
	}
	if !afterNonSaveFi.ModTime().Equal(saveModTime) {
		t.Errorf("Expected stateFile modtime to remain %v, but was %v", saveModTime, afterNonSaveFi.ModTime())
	}

	// 4. Interactive mode defaults to saving on [Enter]
	time.Sleep(20 * time.Millisecond)
	stdinEnter := strings.Join([]string{
		"s 760",
		"",
	}, "\n") + "\n"
	code, _, errOut = runCLI(t, dataDir, stateFile, stdinEnter, "compare", "-i")
	if code != 0 {
		t.Fatalf("interactive compare failed: %s", errOut)
	}
	afterInteractiveFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat stateFile after interactive: %v", err)
	}
	if afterInteractiveFi.ModTime().Equal(saveModTime) {
		t.Errorf("Expected interactive compare on Enter to update stateFile, but modtime didn't change!")
	}

	// 5. Interactive mode with --save=false does NOT save on [Enter]
	interModTime := afterInteractiveFi.ModTime()
	time.Sleep(20 * time.Millisecond)
	stdinNoSave := strings.Join([]string{
		"s 750",
		"",
	}, "\n") + "\n"
	code, _, errOut = runCLI(t, dataDir, stateFile, stdinNoSave, "compare", "-i", "--save=false")
	if code != 0 {
		t.Fatalf("interactive --save=false compare failed: %s", errOut)
	}
	afterNoSaveInterFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat stateFile: %v", err)
	}
	if !afterNoSaveInterFi.ModTime().Equal(interModTime) {
		t.Errorf("Expected interactive compare with --save=false to NOT modify stateFile!")
	}

	// 6. Interactive mode with 'q' cancels and does NOT save
	time.Sleep(20 * time.Millisecond)
	stdinQuit := strings.Join([]string{
		"s 740",
		"q",
	}, "\n") + "\n"
	code, outQuit, errOut := runCLI(t, dataDir, stateFile, stdinQuit, "compare", "-i")
	if code != 0 {
		t.Fatalf("interactive quit failed: %s", errOut)
	}
	afterQuitFi, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat stateFile: %v", err)
	}
	if !afterQuitFi.ModTime().Equal(interModTime) {
		t.Errorf("Expected interactive quit 'q' to NOT modify stateFile!")
	}
	// Verify table wasn't rendered on quit
	if strings.Contains(outQuit, "Apples-to-Apples Geometry Comparison Matrix") {
		t.Errorf("Expected table not to be rendered on 'q' quit, got:\n%s", outQuit)
	}
}

func TestIdealsCLI(t *testing.T) {
	dataDir := t.TempDir()
	stateFile := filepath.Join(t.TempDir(), "last_compare.json")

	// 1. ideals list
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "ideals", "list")
	if code != 0 {
		t.Fatalf("ideals list failed (%d): %s", code, errOut)
	}
	if !strings.Contains(out, "Ideal Frame Profiles") {
		t.Fatalf("expected output to contain 'Ideal Frame Profiles', got:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Fatalf("expected output to list 'default' profile, got:\n%s", out)
	}

	// 2. ideals show default
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "ideals", "show", "default")
	if code != 0 {
		t.Fatalf("ideals show failed (%d): %s", code, errOut)
	}
	if !strings.Contains(out, "Ideal Profile: default") {
		t.Fatalf("expected output to contain 'Ideal Profile: default', got:\n%s", out)
	}

	// 3. ideals show nonexistent
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "ideals", "show", "nonexistent")
	if code == 0 {
		t.Fatalf("expected error for nonexistent profile, got code 0")
	}
	if !strings.Contains(errOut, "not found") {
		t.Fatalf("expected error to mention 'not found', got:\n%s", errOut)
	}
}

func TestIdealsEditTUIWithMockScreen(t *testing.T) {
	dataDir := t.TempDir()
	idealsDir := filepath.Join(dataDir, "..", "ideals")
	_ = os.MkdirAll(idealsDir, 0755)

	var errBuf bytes.Buffer
	app := &cli.App{
		DataDir: dataDir,
		Err:     &errBuf,
	}

	// 1. Edit default profile with MockScreen
	mt, scr := tui.NewMockScreen(t, 100, 30)
	defer scr.Fini()

	var code int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		code = app.EditIdealsWithScreen(scr, idealsDir, "default")
	}()

	time.Sleep(30 * time.Millisecond)
	if !tui.ScreenContains(scr, "IDEAL PROFILE: default") {
		t.Fatalf("expected MockScreen to display default profile, got:\n%s", tui.ScreenDump(scr))
	}
	mt.KeyTap(vt.KeyQ)
	wg.Wait()

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (err: %s)", code, errBuf.String())
	}

	// 2. Edit chooser with MockScreen (no profile specified)
	mt2, scr2 := tui.NewMockScreen(t, 100, 30)
	defer scr2.Fini()

	var code2 int
	var wg2 sync.WaitGroup
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		code2 = app.EditIdealsWithScreen(scr2, idealsDir, "")
	}()

	time.Sleep(30 * time.Millisecond)
	if !tui.ScreenContains(scr2, "BIKEGEO IDEAL PROFILES") {
		t.Fatalf("expected MockScreen to display chooser, got:\n%s", tui.ScreenDump(scr2))
	}
	mt2.KeyTap(vt.KeyQ)
	wg2.Wait()

	if code2 != 0 {
		t.Fatalf("expected exit code 0 for chooser, got %d", code2)
	}

	// 3. Edit nonexistent profile
	_, scr3 := tui.NewMockScreen(t, 100, 30)
	defer scr3.Fini()
	code3 := app.EditIdealsWithScreen(scr3, idealsDir, "nonexistent")
	if code3 == 0 {
		t.Fatalf("expected error code for nonexistent profile")
	}
}

func TestFitCommand(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// 1. Single frame fit analysis
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "fit", "surly-ecr")
	if code != 0 {
		t.Fatalf("fit failed with code %d: %s", code, errOut)
	}
	if !strings.Contains(out, "Best-fit structural A2C (minimax):") {
		t.Errorf("Expected minimax structural A2C in output, got:\n%s", out)
	}
	if !strings.Contains(out, "implied spacer @ declared fork 468 mm:") {
		t.Errorf("Expected implied spacer in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Minimum passing slop:") {
		t.Errorf("Expected minimum passing slop in output, got:\n%s", out)
	}

	// 2. Combinatorial trials (--try)
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "fit", "seven-expedition-926f",
		"--try", "fork.a2c=470,472",
		"--try", "fork.rake=44,52,55",
		"--try", "spacer=10,12",
	)
	if code != 0 {
		t.Fatalf("fit --try failed with code %d: %s", code, errOut)
	}
	if !strings.Contains(out, "Fit Sweep: Seven Expedition 926f (12 combinations)") {
		t.Errorf("Expected 12 combinations in sweep title, got:\n%s", out)
	}
	if !strings.Contains(out, "PASS") || !strings.Contains(out, "FAIL") {
		t.Errorf("Expected PASS and FAIL combo results, got:\n%s", out)
	}

	// 3. JSON format
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "fit", "surly-ecr", "--json")
	if code != 0 {
		t.Fatalf("fit --json failed with code %d: %s", code, errOut)
	}
	if !strings.Contains(out, `"best_fit_structural_a2c"`) {
		t.Errorf("Expected JSON with best_fit_structural_a2c, got:\n%s", out)
	}

	// 4. Detailed sweep table
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "fit", "surly-ecr", "--try", "spacer=11,12", "--detail")
	if code != 0 {
		t.Fatalf("fit --try --detail failed with code %d: %s", code, errOut)
	}
	if !strings.Contains(out, "Combination: spacer=11") {
		t.Errorf("Expected detail combination header, got:\n%s", out)
	}

	// 5. Error handling for unknown frame
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "fit", "nonexistent-frame")
	if code == 0 {
		t.Errorf("Expected error for nonexistent frame")
	}

	// 6. Automatic candidate physical solutions on failing configuration
	code, _, errOut = runCLI(t, dataDir, stateFile, "", "set", "seven-expedition-926f", "fork.a2c=420")
	if code != 0 {
		t.Fatalf("set failed: %s", errOut)
	}
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "fit", "seven-expedition-926f")
	if code != 0 {
		t.Fatalf("fit failed with code %d: %s", code, errOut)
	}
	if !strings.Contains(out, "Candidate Physical Solutions (Geo slop is an absolute last resort):") {
		t.Errorf("Expected candidate physical solutions table, got:\n%s", out)
	}
}
