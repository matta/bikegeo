package cli_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"bikegeo/pkg/cli"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/validator"
)

// runCLIRows runs the CLI and flattens its Markdown tables to comma-joined
// rows, so tests can address cells without caring about Markdown padding.
func runCLIRows(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	out, errOut, code := runCLI(t, args...)
	var rows []string
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |"), " | ")
		if cells[0] == "---" {
			continue
		}
		for i, c := range cells {
			cells[i] = strings.ReplaceAll(c, "<br>", " | ")
		}
		rows = append(rows, strings.Join(cells, ","))
	}
	return strings.Join(rows, "\n") + "\n", errOut, code
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer
	code := cli.Run(args, strings.NewReader(""), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestCompare_SameFrameDifferentSagsAndRigid(t *testing.T) {
	out, errOut, code := runCLIRows(t,
		"compare",
		"havok:L@110mm@20%",
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

	assertRow("Fork & Eff. A2C", "110mm @ 20% sag -> 499mm A2C", "110mm @ 25% sag -> 494mm A2C", "Rigid (485mm)")

	// Verify Reach increases monotonically as effective A2C drops (499 -> 494 -> 485)
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
		"revel-el-jefe:L@120mm@20%",
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
		"120mm @ 20% sag",
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

	// 4. Unallowed sag percentage (15% on suspension fork)
	_, errOut, code = runCLI(t, "compare", "havok:L@110mm@15%")
	if code == 0 {
		t.Fatalf("expected failure for unallowed 15%% sag")
	}
	if !strings.Contains(errOut, "allowed sags are 20%, 25%, 30%") {
		t.Errorf("expected error mentioning allowed sags, got: %s", errOut)
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
		if !strings.Contains(out, "4 (#1)") {
			t.Errorf("expected '4 (#1)' in output, got:\n%s", out)
		}
		if !strings.Contains(out, "-41 (#2)") {
			t.Errorf("expected '-41 (#2)' in output, got:\n%s", out)
		}
		if !strings.Contains(out, "-46 (#3)") {
			t.Errorf("expected '-46 (#3)' in output, got:\n%s", out)
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
		if !strings.Contains(out, "4 (#1)") {
			t.Errorf("expected '4 (#1)' in transposed output, got:\n%s", out)
		}
		// The CLI keeps a transposed comparison as one wide table (the web
		// splits it into Fit & Cockpit and Chassis tables).
		if got := strings.Count(out, "| # | Frame (Size) |"); got != 1 {
			t.Errorf("expected exactly one matrix table in transposed output, got %d:\n%s", got, out)
		}
		if !strings.Contains(out, "Seat Axis–Rear Axle") || !strings.Contains(out, "Reach") {
			t.Errorf("expected cockpit and chassis columns in one table, got:\n%s", out)
		}
		if !strings.Contains(out, "Frame & Chassis Specs: Reach, Stack, HTA, STA, BB Drop, and Trail reflect the dynamic, rotated frame attitude") {
			t.Errorf("expected dynamic geometry explanation in reference definitions footer, got:\n%s", out)
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
		if !strings.Contains(out, "BRP Setback") || !strings.Contains(out, "Oval Fits") {
			t.Errorf("expected metric labels in score breakdown, got:\n%s", out)
		}
	})

	t.Run("ScoreRowAsRows", func(t *testing.T) {
		args := append([]string{"compare"}, specs...)
		out, errOut, code := runCLIRows(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Score,,,4 (#1),-41 (#2),-46 (#3)") {
			t.Errorf("expected CSV Score row, got:\n%s", out)
		}
	})

	t.Run("ScoreRowAsMarkdown", func(t *testing.T) {
		args := append([]string{"compare"}, specs...)
		out, errOut, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "| Score |  |  | 4 (#1) | -41 (#2) | -46 (#3) |") {
			t.Errorf("expected Markdown Score row, got:\n%s", out)
		}
	})

	t.Run("DisambiguatedMetricLabelsAndDefinitions", func(t *testing.T) {
		out, errOut, code := runCLI(t, "compare", "krampus:xl", "pbj:l")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		if !strings.Contains(out, "Seat Axis–Rear Axle @") {
			t.Errorf("expected 'Seat Axis–Rear Axle @' in standard compare output, got:\n%s", out)
		}
		if !strings.Contains(out, "Seat Axis–HT Drop @") {
			t.Errorf("expected 'Seat Axis–HT Drop @' in standard compare output, got:\n%s", out)
		}
		if !strings.Contains(out, "#### Reference Metric Definitions") {
			t.Errorf("expected Reference Metric Definitions footer, got:\n%s", out)
		}
		if !strings.Contains(out, "Chainstay (*): Asterisk indicates adjustable/sliding dropouts; displayed at maximum stay position") {
			t.Errorf("expected Chainstay (*) definition in footer, got:\n%s", out)
		}
		if !strings.Contains(out, "Sliding Dropout Ranges: Sklar PBJ: 432–452mm (max 452mm).") {
			t.Errorf("expected Sliding Dropout Ranges for PBJ, got:\n%s", out)
		}
		if !strings.Contains(out, "Seat Axis–Rear Axle: horizontal distance from the Seat Axis Point to the rear axle") {
			t.Errorf("expected Seat Axis–Rear Axle definition in footer, got:\n%s", out)
		}
		if !strings.Contains(out, "Seat Axis–HT Drop: vertical drop from the Seat Axis Point to head tube top") {
			t.Errorf("expected Seat Axis–HT Drop definition in footer, got:\n%s", out)
		}
		if !strings.Contains(out, "Seatpost Ext: Exposed seatpost length from collar to saddle rails") {
			t.Errorf("expected Seatpost Ext definition in footer, got:\n%s", out)
		}
	})
}

func TestCompare_TumbleweedSunlinerFixedForkConstraints(t *testing.T) {
	t.Run("AcceptsDefaultAndExplicitRigid", func(t *testing.T) {
		out, errOut, code := runCLIRows(t,
			"compare",
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
	out, errOut, code := runCLIRows(t, "compare", "seven-expedition-926f:all@120mm")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Seven Expedition 926f (S 60cm)") || !strings.Contains(out, "Seven Expedition 926f (E 60cm)") {
		t.Errorf("expected Seven Expedition 926f sizes in CSV output, got:\n%s", out)
	}

	// 2. Incremental addition with +frame:all
	out, errOut, code = runCLIRows(t, "compare", "+revel-el-jefe:all@120mm")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Revel El Jefe (S)") || !strings.Contains(out, "Revel El Jefe (M)") {
		t.Errorf("expected Revel El Jefe all sizes in output, got:\n%s", out)
	}

	// 3. Incremental removal with -frame:all (Salsa Fargo is in DefaultComparisonState)
	out, errOut, code = runCLIRows(t, "compare", "-salsa-fargo:all")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if strings.Contains(out, "Salsa Fargo") {
		t.Errorf("expected Salsa Fargo to be removed, got:\n%s", out)
	}

	// 4. Default fork expansion with frame:all
	out, errOut, code = runCLIRows(t, "compare", "revel-el-jefe:all")
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
	}
	for _, exp := range expectedSections {
		if !strings.Contains(out, exp) {
			t.Errorf("expected compare -h output to contain %q, got:\n%s", exp, out)
		}
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
	if !strings.Contains(out, "BRP–HB @ 746mm") {
		t.Errorf("expected 'BRP–HB @ 746mm' row in compare output, got:\n%s", out)
	}
}

func TestCompare_WithSaddleFlag(t *testing.T) {
	out, errOut, code := runCLI(t, "compare", "--saddle", "berthoud-aspin-open", "havok:L@110mm@20%")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "Berthoud Aspin Open") {
		t.Errorf("expected Berthoud Aspin Open in subtitle, got:\n%s", out)
	}
}

func TestCompare_AllFramesAndPlausibleConfigurations(t *testing.T) {
	t.Run("CompareAllFlag_DefaultTop1PerFrame", func(t *testing.T) {
		out, errOut, code := runCLIRows(t, "compare", "--all")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		// Header + Ideal + Acceptable = 3 rows, followed by 18 frame rows
		var dataLines []string
		for _, l := range lines[3:] {
			if strings.TrimSpace(l) != "" {
				dataLines = append(dataLines, l)
			}
		}

		wantFrames := len(frames.AllFrames())
		if len(dataLines) != wantFrames {
			t.Fatalf("expected %d frame rows, got %d:\n%s", wantFrames, len(dataLines), out)
		}

		// Verify ranks 1..18 appear in order
		seenFrames := make(map[string]bool)
		for rankIdx, l := range dataLines {
			parts := strings.Split(l, ",")
			expectedRank := rankIdx + 1
			if parts[0] != strconv.Itoa(expectedRank) {
				t.Errorf("line %d: expected rank %d, got %s", rankIdx, expectedRank, parts[0])
			}
			frameName := parts[1]
			if seenFrames[frameName] {
				t.Errorf("duplicate frame %s in --all output", frameName)
			}
			seenFrames[frameName] = true
		}
	})

	t.Run("CompareAllToken_MatchesFlag", func(t *testing.T) {
		outFlag, _, codeFlag := runCLIRows(t, "compare", "--all")
		outToken, _, codeToken := runCLIRows(t, "compare", "all")

		if codeFlag != 0 || codeToken != 0 {
			t.Fatalf("expected exit code 0, got %d and %d", codeFlag, codeToken)
		}
		if outFlag != outToken {
			t.Errorf("expected 'compare all' to match 'compare --all'\nflag:\n%s\ntoken:\n%s", outFlag, outToken)
		}
	})

	t.Run("CompareAll_GlobalTopLimit", func(t *testing.T) {
		out, errOut, code := runCLIRows(t, "compare", "--all", "--top", "5")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		var dataLines []string
		for _, l := range lines[3:] {
			if strings.TrimSpace(l) != "" {
				dataLines = append(dataLines, l)
			}
		}
		if len(dataLines) != 5 {
			t.Fatalf("expected 5 data rows for --top 5, got %d", len(dataLines))
		}
	})

	t.Run("CompareAll_TopPerFrame2", func(t *testing.T) {
		out, errOut, code := runCLIRows(t, "compare", "--all", "--top-per-frame", "2")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		frameCounts := make(map[string]int)
		for _, l := range lines[3:] {
			parts := strings.Split(l, ",")
			if len(parts) > 1 {
				// Base frame name without (Size) or @ modifier
				name := strings.TrimSpace(strings.Split(parts[1], "(")[0])
				frameCounts[name]++
			}
		}

		for frame, count := range frameCounts {
			if count > 2 {
				t.Errorf("frame %q appeared %d times (expected at most 2)", frame, count)
			}
		}
	})

	t.Run("CompareAllRigid_ExcludesSuspensionOnly", func(t *testing.T) {
		out, errOut, code := runCLIRows(t, "compare", "all@rigid")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		var dataLines []string
		for _, l := range lines[3:] {
			if strings.TrimSpace(l) != "" {
				dataLines = append(dataLines, l)
			}
		}
		// All registered frames except the 3 suspension-only frames support rigid forks
		wantRigid := len(frames.AllFrames()) - 3
		if len(dataLines) != wantRigid {
			t.Fatalf("expected %d rigid frame rows, got %d:\n%s", wantRigid, len(dataLines), out)
		}

		for _, l := range dataLines {
			parts := strings.Split(l, ",")
			forkCol := parts[3]
			if !strings.HasPrefix(forkCol, "Rigid") {
				t.Errorf("expected rigid fork, got %q in line: %s", forkCol, l)
			}
		}

		// Ensure suspension-only frames are excluded
		if strings.Contains(out, "Fairlight Holt") || strings.Contains(out, "Turner Venn") || strings.Contains(out, "Moots Mountaineer") {
			t.Errorf("suspension-only frame appeared in all@rigid output:\n%s", out)
		}
	})

	t.Run("CompareSingleFrameAllConfigs_DedupesSags", func(t *testing.T) {
		out, errOut, code := runCLIRows(t, "compare", "otso-fenrir-stainless:all@all")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		var dataLines []string
		for _, l := range lines[3:] {
			if strings.TrimSpace(l) != "" {
				dataLines = append(dataLines, l)
			}
		}
		// Default top-per-frame for single frame @all is 5
		if len(dataLines) != 5 {
			t.Fatalf("expected 5 setups for Fenrir all@all, got %d:\n%s", len(dataLines), out)
		}

		// Ensure no duplicate (Size, ForkType) among the 5 rows
		seenCombinations := make(map[string]bool)
		for _, l := range dataLines {
			parts := strings.Split(l, ",")
			// Column 1 is Name with Size and ForkType e.g. "Otso Fenrir (XL) @ 120mm"
			name := parts[1]
			if seenCombinations[name] {
				t.Errorf("duplicate (Size, ForkType) combination %q found in all@all output", name)
			}
			seenCombinations[name] = true
		}
	})

	t.Run("IncrementalRemovalAllColumns", func(t *testing.T) {
		_, errOut, code := runCLI(t, "compare", "--", "-all")
		if code == 0 {
			t.Fatalf("expected non-zero exit code when removing all columns")
		}
		if !strings.Contains(errOut, "no columns selected for comparison") {
			t.Errorf("expected 'no columns selected for comparison' error, got: %s", errOut)
		}
	})

	t.Run("HelpTextDocumentsFlags", func(t *testing.T) {
		out, _, code := runCLI(t, "compare", "-h")
		if code != 0 {
			t.Fatalf("expected code 0, got %d", code)
		}
		for _, exp := range []string{
			"--all",
			"--all-configs",
			"--top-per-frame",
			"--top",
			"All-Frames & All-Configurations Expansion:",
			"Two-Stage Ranking & Pruning Funnel:",
		} {
			if !strings.Contains(out, exp) {
				t.Errorf("expected compare -h to contain %q", exp)
			}
		}
	})
}

func TestCompare_ConfiguredHandlebar(t *testing.T) {
	t.Run("DefaultPersuaderHeader", func(t *testing.T) {
		out, errOut, code := runCLI(t, "compare")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		expectedLines := []string{
			"H (saddle height): 746mm | Saddle: SQlab 610 Ergolux Active 2.1 | Seatpost: Redshift ShockStop (sagged 11mm)",
			"Handlebar: Tumbleweed Persuader 20° (800mm, 50mm rise, 20° sweep, 4° upsweep) | Ideal Profile: idmatch",
		}
		for _, exp := range expectedLines {
			if !strings.Contains(out, exp) {
				t.Errorf("expected compare output to contain header %q, got:\n%s", exp, out)
			}
		}
	})

	t.Run("CustomBarFlag", func(t *testing.T) {
		out, errOut, code := runCLI(t, "compare", "--handlebar", "sqlab3OX_16_30")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut)
		}
		expectedHeader := "Handlebar: SQlab 3OX 16° (780mm, 30mm rise, 16° sweep, 4° upsweep)"
		if !strings.Contains(out, expectedHeader) {
			t.Errorf("expected compare output to contain custom handlebar header %q, got:\n%s", expectedHeader, out)
		}
	})

	t.Run("UnrecognizedBarFlag", func(t *testing.T) {
		_, errOut, code := runCLI(t, "compare", "--handlebar", "not_a_valid_bar")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(errOut, "unrecognized handlebar") {
			t.Errorf("expected 'unrecognized handlebar' error, got: %s", errOut)
		}
	})
}

func TestList_FilterByQuery(t *testing.T) {
	out, errOut, code := runCLI(t, "list", "-q", "havok")
	if code != 0 {
		t.Fatalf("expected list to succeed, got %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, "| binary-havok |") {
		t.Errorf("expected markdown row for binary-havok, got:\n%s", out)
	}
}

func TestRemovedCommandsAreUnknown(t *testing.T) {
	for _, cmd := range []string{"forks", "stems", "ideals", "saddles", "saddle-swap", "handlebars"} {
		_, errOut, code := runCLI(t, cmd)
		if code == 0 || !strings.Contains(errOut, "unknown command") {
			t.Errorf("%s: code %d, stderr %q; want unknown command", cmd, code, errOut)
		}
	}
}
