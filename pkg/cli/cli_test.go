package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bikegeo/pkg/cli"
	"bikegeo/pkg/model"
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
		SeatHeight float64                        `json:"seat_height"`
		Columns    []model.CanonicalComparisonRow `json:"columns"`
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

	if !strings.Contains(out, "Binary Havok (L) [REF]") {
		t.Errorf("Expected Binary Havok (L) [REF] in table output, got:\n%s", out)
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

	// 1. Issue initial comparison
	code, _, errOut := runCLI(t, dataDir, stateFile, "", "compare", "havok:ML@110mm@20%", "fenrir:L@rigid")
	if code != 0 {
		t.Fatalf("Initial compare failed: %s", errOut)
	}

	// 2. Re-issue bare `compare` with no arguments -> should replay the exact 2 columns
	code, out, errOut := runCLI(t, dataDir, stateFile, "", "compare", "--format", "json")
	if code != 0 {
		t.Fatalf("Bare compare replay failed: %s", errOut)
	}
	var saved struct {
		SeatHeight float64                        `json:"seat_height"`
		Columns    []model.CanonicalComparisonRow `json:"columns"`
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

	// 3. Incremental tweak: add fenrir@110mm@20% and remove column 2
	code, out, errOut = runCLI(t, dataDir, stateFile, "", "compare", "+fenrir:XL@110mm@20%", "-2", "--seat-height", "775", "--format", "json")
	if code != 0 {
		t.Fatalf("Incremental compare failed: %s", errOut)
	}
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if saved.SeatHeight != 775 {
		t.Errorf("Expected saved SeatHeight 775, got %.0f", saved.SeatHeight)
	}
	if len(saved.Columns) != 2 || saved.Columns[1].Setup.FrameID != "otso-fenrir-stainless" {
		t.Errorf("Expected 2nd column to be otso-fenrir-stainless, got %+v", saved.Columns)
	}
}

func TestCompare_InteractiveMode(t *testing.T) {
	dataDir, stateFile := setupTestEnv(t)

	// Simulate interactive session:
	// - c (clear and add first column: "havok", size "L", fork "rigid" -> sag prompt is skipped!)
	// - a (add second column: "fenrir", size "L", fork "120mm", sag "25")
	// - r 2 (set column 2 as reference)
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
		"r 2",
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
	if !strings.Contains(out, "2 [REF]") {
		t.Errorf("Expected column 2 to be marked [REF] in transposed output, got:\n%s", out)
	}
	if !strings.Contains(out, "120mm @ 25%") {
		t.Errorf("Expected Otso Fenrir at 120mm @ 25%% in output, got:\n%s", out)
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

	// 2. Break a frame's stack so it fails Master Stack Equation and verify validate returns exit code 1
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
