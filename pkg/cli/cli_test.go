package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"bikegeo/pkg/cli"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/validator"
)

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer
	code := cli.Run(args, strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestCompare_SameFrameDifferentSagsAndRigid(t *testing.T) {
	out, errOut, code := runCLI(t,
		"compare", "--format", "csv",
		"havok:L@110mm@15%",
		"havok:L@110mm@25%",
		"havok:L@rigid",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	rowByLabel := make(map[string][]string)
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) > 0 {
			rowByLabel[parts[0]] = parts
		}
	}

	assertRow := func(label string, expected ...string) {
		t.Helper()
		row, ok := rowByLabel[label]
		if !ok {
			t.Fatalf("missing row %q in CSV output:\n%s", label, out)
		}
		if len(row) < 3+len(expected) {
			t.Fatalf("row %q has %d cols, expected at least %d: %v", label, len(row), 3+len(expected), row)
		}
		for i, exp := range expected {
			if row[i+3] != exp {
				t.Errorf("row %q col %d: expected %q, got %q", label, i+1, exp, row[i+3])
			}
		}
	}

	assertRow("Fork & Eff. A2C", "110mm @ 15% sag -> 505mm A2C", "110mm @ 25% sag -> 494mm A2C", "Rigid (485mm)")

	// Verify Reach increases monotonically as effective A2C drops (505 -> 494 -> 485)
	reachRow := rowByLabel["Reach"]
	if len(reachRow) < 6 {
		t.Fatalf("missing Reach columns: %v", reachRow)
	}
	if reachRow[3] >= reachRow[4] || reachRow[4] >= reachRow[5] {
		t.Errorf("expected Reach to increase monotonically as A2C decreases, got %v", reachRow[3:6])
	}
}

func TestCompare_MixedFramesForksAndSags(t *testing.T) {
	out, errOut, code := runCLI(t,
		"compare",
		"otso-fenrir-stainless:L@rigid",
		"havok:L@110mm@25%",
		"revel-el-jefe:L@120mm@15%",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}

	expectedStrings := []string{
		"Otso Fenrir",
		"Binary Havok",
		"Revel El Jefe",
		"Rigid (495mm)",
		"110mm @ 25% sag",
		"120mm @ 15% sag",
		"Stack",
		"Reach",
		"Fork & Eff. A2C",
	}
	for _, exp := range expectedStrings {
		if !strings.Contains(out, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, out)
		}
	}
}

func TestCompare_EnforcesFrameForkSagAndValidationConstraints(t *testing.T) {
	// 1. Unsupported fork travel (150mm on Havok)
	_, errOut, code := runCLI(t, "compare", "havok:L@150mm@20%")
	if code == 0 {
		t.Fatalf("expected failure for unsupported 150mm fork on Havok")
	}
	if !strings.Contains(errOut, `does not support fork "150mm"`) {
		t.Errorf("expected error mentioning unsupported fork, got: %s", errOut)
	}

	// 2. Non-zero sag on rigid fork
	_, errOut, code = runCLI(t, "compare", "havok:L@rigid@20%")
	if code == 0 {
		t.Fatalf("expected failure for non-zero sag on rigid fork")
	}
	if !strings.Contains(errOut, "cannot apply 20% sag to rigid fork") {
		t.Errorf("expected error mentioning rigid fork sag, got: %s", errOut)
	}

	// 3. Suspension fork on rigid-only frame (Tumbleweed Sunliner)
	_, errOut, code = runCLI(t, "compare", "tumbleweed-sunliner:L@100mm")
	if code == 0 {
		t.Fatalf("expected failure for suspension fork on rigid-only frame")
	}
	if !strings.Contains(errOut, `does not support suspension fork "100mm"`) {
		t.Errorf("expected error mentioning suspension fork not supported, got: %s", errOut)
	}
}

func TestCompare_DefaultAndIncrementalTweaks(t *testing.T) {
	// 1. Bare compare uses DefaultComparisonState
	out, errOut, code := runCLI(t, "compare")
	if code != 0 {
		t.Fatalf("expected bare compare to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Salsa Fargo") || !strings.Contains(out, "Otso Fenrir") {
		t.Errorf("expected default comparison output to contain default frames, got:\n%s", out)
	}

	// 2. Incremental addition (+spec) on top of default comparison
	out, errOut, code = runCLI(t, "compare", "+surly-ecr:XL")
	if code != 0 {
		t.Fatalf("expected incremental +surly-ecr:XL to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Surly ECR") {
		t.Errorf("expected output to contain added Surly ECR, got:\n%s", out)
	}

	// 3. Incremental removal by column number (-1)
	_, errOut, code = runCLI(t, "compare", "-1")
	if code != 0 {
		t.Fatalf("expected incremental -1 to succeed, got %d; stderr: %s", code, errOut)
	}

	// 4. Incremental removal of out-of-range column number (-99)
	_, errOut, code = runCLI(t, "compare", "-99")
	if code == 0 {
		t.Fatalf("expected failure when removing out-of-range column index")
	}
	if !strings.Contains(errOut, "column index 99 out of range") {
		t.Errorf("expected error mentioning column index out of range, got: %s", errOut)
	}

	// 5. Removing unknown frame token (-z) reports frame not found
	_, errOut, code = runCLI(t, "compare", "-z")
	if code == 0 {
		t.Fatalf("expected failure for unknown frame removal -z")
	}
	if !strings.Contains(errOut, `frame "z" not found`) {
		t.Errorf("expected frame not found error for -z, got: %s", errOut)
	}
}

func TestValidate_CoherentFrames(t *testing.T) {
	out, errOut, code := runCLI(t, "validate", "--all")
	if code != 0 {
		t.Fatalf("expected validate --all to succeed, got %d; stderr: %s\nout:\n%s", code, errOut, out)
	}
	if !strings.Contains(out, "VALID") {
		t.Errorf("expected VALID in validate output, got:\n%s", out)
	}

	out, errOut, code = runCLI(t, "validate", "otso-fenrir-stainless")
	if code != 0 {
		t.Fatalf("expected validate otso-fenrir-stainless to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Otso Fenrir") {
		t.Errorf("expected Otso Fenrir in validate output, got:\n%s", out)
	}
}

func TestListForksAndStems(t *testing.T) {
	out, errOut, code := runCLI(t, "list", "-q", "havok", "--format", "md")
	if code != 0 {
		t.Fatalf("expected list to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "| binary-havok |") {
		t.Errorf("expected markdown row for binary-havok, got:\n%s", out)
	}

	out, errOut, code = runCLI(t, "forks", "--format", "csv")
	if code != 0 {
		t.Fatalf("expected forks to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "sid_110,RockShox SID 110mm,110 mm,yes,521/44") {
		t.Errorf("expected sid_110 in forks CSV, got:\n%s", out)
	}

	out, errOut, code = runCLI(t, "stems")
	if code != 0 {
		t.Fatalf("expected stems to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "mtb") || !strings.Contains(out, "gravel") {
		t.Errorf("expected mtb and gravel in stems output, got:\n%s", out)
	}
}

func TestCompare_ScoringAndFlags(t *testing.T) {
	specs := []string{
		"binary-havok:L@110mm@20%",
		"binary-havok:XL@110mm@20%",
		"tumbleweed-sunliner:L@rigid",
	}

	t.Run("DefaultWithScoring", func(t *testing.T) {
		args := append([]string{"compare"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Score") {
			t.Errorf("expected 'Score' row in default output, got:\n%s", out)
		}
		if !strings.Contains(out, "91 (#1)") {
			t.Errorf("expected '91 (#1)' in output, got:\n%s", out)
		}
		if !strings.Contains(out, "78 (#2)") {
			t.Errorf("expected '78 (#2)' in output, got:\n%s", out)
		}
		if !strings.Contains(out, "62 (#3)") {
			t.Errorf("expected '62 (#3)' in output, got:\n%s", out)
		}
	})

	t.Run("TransposedWithScoring", func(t *testing.T) {
		args := append([]string{"compare", "--transpose"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Score") {
			t.Errorf("expected 'Score' column in transposed output, got:\n%s", out)
		}
		if !strings.Contains(out, "91 (#1)") {
			t.Errorf("expected '91 (#1)' in transposed output, got:\n%s", out)
		}
	})

	t.Run("NoScoreFlag", func(t *testing.T) {
		args := append([]string{"compare", "--no-score"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if strings.Contains(out, "Biggest Losses vs. Ideal") {
			t.Errorf("did not expect scoring summary when --no-score is set, got:\n%s", out)
		}
	})

	t.Run("ScoreDetailFlag", func(t *testing.T) {
		args := append([]string{"compare", "--score-detail"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Scoring Breakdown") {
			t.Errorf("expected 'Scoring Breakdown' table when --score-detail is set, got:\n%s", out)
		}
		if !strings.Contains(out, "Setback") || !strings.Contains(out, "AETT") {
			t.Errorf("expected metric labels in score breakdown, got:\n%s", out)
		}
	})

	t.Run("FormatCSV", func(t *testing.T) {
		args := append([]string{"compare", "--format", "csv"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Score,,,91 (#1),78 (#2),62 (#3)") {
			t.Errorf("expected CSV Score row, got:\n%s", out)
		}
	})

	t.Run("FormatMD", func(t *testing.T) {
		args := append([]string{"compare", "--format", "md"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "| Score |  |  | 91 (#1) | 78 (#2) | 62 (#3) |") {
			t.Errorf("expected Markdown Score row, got:\n%s", out)
		}
	})
}

func TestCompare_TumbleweedSunlinerFixedForkConstraints(t *testing.T) {
	t.Run("AcceptsDefaultAndExplicitRigid", func(t *testing.T) {
		out, errOut, code := runCLI(t,
			"compare", "--format", "csv",
			"tumbleweed-sunliner:M",
			"tumbleweed-sunliner:L@rigid",
		)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Fork & Eff. A2C,,,Rigid (440mm),Rigid (440mm)") {
			t.Errorf("expected Rigid (440mm) in CSV output, got:\n%s", out)
		}
	})

	t.Run("RejectsSuspensionForks", func(t *testing.T) {
		_, errOut, code := runCLI(t, "compare", "tumbleweed-sunliner:L@100mm")
		if code == 0 {
			t.Fatalf("expected non-zero exit code when requesting 100mm suspension fork on Sunliner")
		}
		if !strings.Contains(errOut, `does not support suspension fork "100mm"`) {
			t.Errorf("expected error mentioning suspension fork not supported, got: %s", errOut)
		}
	})

	t.Run("RejectsCustomForkA2CAndRake", func(t *testing.T) {
		sunliner, err := frames.GetFrameByID("tumbleweed-sunliner")
		if err != nil {
			t.Fatalf("failed to get tumbleweed-sunliner: %v", err)
		}

		badA2C := sunliner.Clone()
		badA2C.Forks[0].A2cMm = 485.0
		res := validator.ValidateFrame(badA2C)
		if res.IsValid {
			t.Errorf("expected validation failure when Sunliner fork A2C is modified to 485mm")
		}

		badRake := sunliner.Clone()
		badRake.Forks[0].RakeMm = 51.0
		resRake := validator.ValidateFrame(badRake)
		if resRake.IsValid {
			t.Errorf("expected validation failure when Sunliner fork rake is modified to 51mm")
		}
	})
}

func TestCompare_DisambiguateDuplicateFrameSize(t *testing.T) {
	out, errOut, code := runCLI(t,
		"compare",
		"otso-fenrir-stainless:L@rigid",
		"otso-fenrir-stainless:L@100mm@25%",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Otso Fenrir (L) @ rigid") || !strings.Contains(out, "Otso Fenrir (L) @ 100mm") {
		t.Errorf("expected disambiguated names 'Otso Fenrir (L) @ rigid' and 'Otso Fenrir (L) @ 100mm', got:\n%s", out)
	}

	outT, errOut, code := runCLI(t,
		"compare", "--transpose",
		"otso-fenrir-stainless:L@rigid",
		"otso-fenrir-stainless:L@100mm@25%",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(outT, "Otso Fenrir (L) @ rigid") || !strings.Contains(outT, "Otso Fenrir (L) @ 100mm") {
		t.Errorf("expected disambiguated cells in transposed table, got:\n%s", outT)
	}

	// Incremental removal by fork spec from DefaultComparisonState
	outRem, errOut, code := runCLI(t,
		"compare", "-otso-fenrir-stainless:L@110mm",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if strings.Contains(outRem, "Otso Fenrir (L) @ 110mm") {
		t.Errorf("expected 'Otso Fenrir (L) @ 110mm' to be removed, got:\n%s", outRem)
	}
}

func TestCompare_AllSizesExpansion(t *testing.T) {
	// 1. Fresh comparison with :all and fork override
	out, errOut, code := runCLI(t, "compare", "--format", "csv", "seven-expedition-926f:all@120mm")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Seven Expedition 926f (S 60cm)") || !strings.Contains(out, "Seven Expedition 926f (E 60cm)") {
		t.Errorf("expected Seven Expedition 926f sizes in CSV output, got:\n%s", out)
	}

	// 2. Incremental addition with +frame:all
	out, errOut, code = runCLI(t, "compare", "--format", "csv", "+revel-el-jefe:all@120mm")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Revel El Jefe (S)") || !strings.Contains(out, "Revel El Jefe (M)") {
		t.Errorf("expected Revel El Jefe all sizes in output, got:\n%s", out)
	}

	// 3. Incremental removal with -frame:all (Salsa Fargo is in DefaultComparisonState)
	out, errOut, code = runCLI(t, "compare", "--format", "csv", "-salsa-fargo:all")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if strings.Contains(out, "Salsa Fargo") {
		t.Errorf("expected Salsa Fargo to be removed, got:\n%s", out)
	}

	// 4. Default fork expansion with frame:all
	out, errOut, code = runCLI(t, "compare", "--format", "csv", "revel-el-jefe:all")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Revel El Jefe (S)") || !strings.Contains(out, "Revel El Jefe (XL)") {
		t.Errorf("expected all Revel El Jefe sizes, got:\n%s", out)
	}
}

func TestCompare_HelpOutputDocumentsSemantics(t *testing.T) {
	out, errOut, code := runCLI(t, "compare", "-h")
	if code != 0 {
		t.Fatalf("expected exit code 0 for compare -h, got %d; stderr: %s", code, errOut)
	}

	expectedSections := []string{
		"Comparison State & Column Management:",
		"Default Comparison Baseline:",
		"Full Replacement (Bare Specs):",
		"Incremental Addition ('+spec'):",
		"Incremental Removal ('-spec' or '-col_num'):",
		"-score-detail",
		"--ideal",
	}
	for _, exp := range expectedSections {
		if !strings.Contains(out, exp) {
			t.Errorf("expected compare -h output to contain %q, got:\n%s", exp, out)
		}
	}
}

func TestIdealsCLI(t *testing.T) {
	out, errOut, code := runCLI(t, "ideals", "list")
	if code != 0 {
		t.Fatalf("expected exit code 0 for ideals list, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "default") || !strings.Contains(out, "idmatch") {
		t.Errorf("expected default and idmatch in ideals list, got:\n%s", out)
	}

	out, errOut, code = runCLI(t, "ideals", "show", "default")
	if code != 0 {
		t.Fatalf("expected exit code 0 for ideals show default, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Ideal Profile: default") || !strings.Contains(out, "Setback (setback)") {
		t.Errorf("expected default profile details, got:\n%s", out)
	}

	_, errOut, code = runCLI(t, "ideals", "show", "nonexistent")
	if code == 0 {
		t.Fatalf("expected non-zero exit code for nonexistent profile")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected not found error, got: %s", errOut)
	}
}

func TestFitCommand(t *testing.T) {
	t.Run("BaselineValidFrame", func(t *testing.T) {
		out, errOut, code := runCLI(t, "fit", "surly-ecr")
		if code != 0 {
			t.Fatalf("expected exit code 0 for fit surly-ecr, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Surly ECR") || !strings.Contains(out, "PASS") {
			t.Errorf("expected Surly ECR PASS in fit output, got:\n%s", out)
		}
	})

	t.Run("TryHypotheticalParameters", func(t *testing.T) {
		out, errOut, code := runCLI(t, "fit", "seven-expedition-926f", "--try", "fork.a2c=511,531", "--try", "sag=0,20,25")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Fit Sweep:") || !strings.Contains(out, "fork.a2c=511") {
			t.Errorf("expected hypothetical results in fit output, got:\n%s", out)
		}
	})

	t.Run("TryWithDetail", func(t *testing.T) {
		out, errOut, code := runCLI(t, "fit", "surly-ecr", "--try", "spacer=11,12", "--detail")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Combination: spacer=11") || !strings.Contains(out, "Combination: spacer=12") {
			t.Errorf("expected detailed hypothetical breakdown, got:\n%s", out)
		}
	})

	t.Run("NonExistentFrame", func(t *testing.T) {
		_, errOut, code := runCLI(t, "fit", "nonexistent-frame")
		if code == 0 {
			t.Fatalf("expected non-zero exit code for nonexistent frame")
		}
		if !strings.Contains(errOut, "not found") {
			t.Errorf("expected not found error, got: %s", errOut)
		}
	})
}

func TestCompare_SaddleHbMetric(t *testing.T) {
	out, errOut, code := runCLI(t, "compare", "havok:L@110mm@20%")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Saddle-HB @ 780mm") {
		t.Errorf("expected 'Saddle-HB @ 780mm' row in compare output, got:\n%s", out)
	}
}
