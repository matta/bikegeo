package cli

import (
	"flag"
	"fmt"
	"strings"

	"bikegeo/pkg/frames"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

type ValidationResultBundle struct {
	Frame       model.FrameModel
	Size        string
	Bike        model.BikeGeometry
	Report      model.ValidationReport
	FrameReport model.FrameValidationReport
}

func formatPublishModeShort(mode model.PublishMode) string {
	switch mode {
	case model.ModeStaticCad:
		return "Static (0% Sag)"
	case model.ModeSaggedCad:
		return "Sagged CAD"
	case model.ModeMismatchedCad:
		return "Invalid (Mismatched)"
	default:
		return "Unspecified"
	}
}

func (a *App) runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var sizeFlag string
	var configFlag string
	var allFlag bool

	fs.StringVar(&sizeFlag, "size", "", "Specific frame size to validate (default: frame's default_size)")
	fs.StringVar(&configFlag, "config", "", "Specific frame configuration ID to validate")
	fs.BoolVar(&allFlag, "all", false, "Validate all frames in data/bikes/")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"all": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		fmt.Fprintf(a.Err, "Error: no bike frames found\n")
		return 1
	}

	var targetFrames []model.FrameModel
	var targetSizes []string

	posArgs := fs.Args()
	if allFlag || len(posArgs) == 0 {
		for _, f := range allFrames {
			sz := sizeFlag
			if sz == "" {
				sz = f.GetDefaultSize()
			}
			targetFrames = append(targetFrames, f)
			targetSizes = append(targetSizes, sz)
		}
	} else {
		for _, tok := range posArgs {
			f, impSz, err := ResolveFrameByToken(allFrames, tok)
			if err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
			sz := sizeFlag
			if sz == "" {
				sz = impSz
			}
			if sz == "" {
				sz = f.GetDefaultSize()
			}
			// Verify size exists on frame
			matchedSz := ""
			for _, s := range f.GetSizes() {
				if strings.EqualFold(s, sz) {
					matchedSz = s
					break
				}
			}
			if matchedSz == "" {
				fmt.Fprintf(a.Err, "Error: frame %q does not have size %q (available: %s)\n", f.ID, sz, strings.Join(f.GetSizes(), ", "))
				return 1
			}
			targetFrames = append(targetFrames, *f)
			targetSizes = append(targetSizes, matchedSz)
		}
	}

	var bundles []ValidationResultBundle
	hasInvalid := false

	for i, f := range targetFrames {
		sz := targetSizes[i]

		bike, err := geometry.MaterializeBikeGeometry(f, sz)
		if err != nil {
			fmt.Fprintf(a.Err, "Error materializing %s (%s): %v\n", f.ID, sz, err)
			return 1
		}

		report := validator.ValidateBikeGeometry(bike)
		frameReport := validator.ValidateFrame(f)
		if !report.IsValid || !frameReport.IsValid {
			hasInvalid = true
		}

		bundles = append(bundles, ValidationResultBundle{
			Frame:       f,
			Size:        sz,
			Bike:        bike,
			Report:      report,
			FrameReport: frameReport,
		})
	}

	for _, b := range bundles {
		if !isBundleValid(b) {
			hasInvalid = true
		}
	}

	normFormat, _ := NormalizeFormat(a.Format)

	// If multiple frames are validated (e.g. --all), print a summary table first
	if len(bundles) > 1 {
		headers := []string{"Frame ID", "Manufacturer & Model", "Size", "Status", "Stated A2C", "CAD Geo A2C", "Δ A2C", "CAD Convention", "Supported Forks"}
		var rows [][]string
		for _, b := range bundles {
			status := "VALID"
			if !isBundleValid(b) {
				status = "INVALID"
			}
			statedStr := "N/A"
			if b.Report.ExpectedForkA2c > 0 {
				statedStr = fmt.Sprintf("%.0f mm", b.Report.ExpectedForkA2c)
			}
			diffStr := "N/A"
			if b.Report.A2cDifference != nil {
				diffStr = fmt.Sprintf("%+.1f mm", *b.Report.A2cDifference)
			}
			rows = append(rows, []string{
				b.Frame.ID,
				fmt.Sprintf("%s %s", b.Frame.Manufacturer, b.Frame.Model),
				b.Size,
				status,
				statedStr,
				fmt.Sprintf("%.1f mm", b.Report.GeometricA2c),
				diffStr,
				formatPublishModeShort(b.Report.InferredPublishMode),
				strings.Join(b.Frame.GetSupportedForkOptions(), ", "),
			})
		}

		title := fmt.Sprintf("CAD Geometry Validation Summary (%d Frames)", len(bundles))
		if err := RenderOutput(a.Out, a.Format, title, "", headers, rows); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		if normFormat == FormatCSV || normFormat == FormatMarkdown {
			if hasInvalid {
				return 1
			}
			return 0
		}
		fmt.Fprintln(a.Out)
	}

	// Detailed validation report for single frame (or invalid frames in --all)
	for idx, b := range bundles {
		if len(bundles) > 1 && isBundleValid(b) {
			continue
		}
		if idx > 0 {
			fmt.Fprintln(a.Out)
		}
		a.renderDetailedValidation(b)
	}

	if hasInvalid {
		return 1
	}
	return 0
}

func isBundleValid(b ValidationResultBundle) bool {
	return b.Report.IsValid && b.FrameReport.IsValid
}

func (a *App) renderDetailedValidation(b ValidationResultBundle) {
	statusBadge := "CAD COHERENT (VALID)"
	if !isBundleValid(b) {
		statusBadge = "VALIDATION ISSUES DETECTED (INVALID)"
	}

	title := fmt.Sprintf(
		"=== %s %s (Size %s) [%s] ===",
		b.Bike.Manufacturer, b.Bike.Model, b.Size, statusBadge,
	)
	subtitle := fmt.Sprintf(
		"Category: %s | Supported Forks: %s | Default Fork: %s",
		b.Bike.Category,
		strings.Join(b.Frame.GetSupportedForkOptions(), ", "),
		b.Frame.GetDefaultForkSpecString(),
	)

	// 1. Primary Dimensions Table
	specHeaders := []string{"Reach", "Stack", "HTA", "STA", "HTL", "BB Drop", "Fork Rake", "Wheelbase"}
	specRow := []string{
		fmt.Sprintf("%.1f mm", b.Bike.Reach),
		fmt.Sprintf("%.1f mm", b.Bike.Stack),
		fmt.Sprintf("%.1f°", b.Bike.HtaDeg),
		fmt.Sprintf("%.1f°", b.Bike.StaDeg),
		fmt.Sprintf("%.1f mm", b.Bike.Htl),
		fmt.Sprintf("%.1f mm", b.Bike.BbDrop),
		fmt.Sprintf("%.1f mm", b.Bike.ForkRake),
		fmt.Sprintf("%.1f mm", b.Bike.Wheelbase),
	}
	_ = RenderOutput(a.Out, a.Format, title, subtitle, specHeaders, [][]string{specRow})

	// 2. Master Stack Equation & Headset Proof
	fmt.Fprintln(a.Out, "\nMaster Stack Equation & Headset Proof:")
	fmt.Fprintln(a.Out, "  Formula: Stack = BB_Drop - Rake*cos(HTA) + (A2C + Geo_Spacer + HTL)*sin(HTA)")
	fmt.Fprintf(
		a.Out,
		"  Derived: (%.0f - %.0f + %.0f*cos(%.1f°)) / sin(%.1f°) - %.0f = %.1f mm (Fork A2C) + %.1f mm (CAD Spacer) = %.1f mm\n",
		b.Bike.Stack, b.Bike.BbDrop, b.Bike.ForkRake, b.Bike.HtaDeg, b.Bike.HtaDeg, b.Bike.Htl,
		b.Report.GeometricA2c, b.Report.GeoHeadsetSpacerMm, b.Report.TotalStructuralA2c,
	)

	statedStr := "Not specified"
	if b.Bike.ForkA2c > 0 {
		statedStr = fmt.Sprintf("%.0f mm", b.Bike.ForkA2c)
	}
	diffStr := "N/A"
	if b.Report.A2cDifference != nil {
		diffStr = fmt.Sprintf("%+.1f mm", *b.Report.A2cDifference)
	}

	slop := b.Bike.GetGeoSlopMm()
	var wbSlopStr string
	if slop.Wheelbase != nil {
		wbSlopStr = fmt.Sprintf("wb: ±%.1fmm", *slop.Wheelbase)
	} else {
		dynTol := validator.CalculateDynamicWheelbaseTolerance(b.Bike, b.Report.TotalStructuralA2c)
		wbSlopStr = fmt.Sprintf("wb: ±%.1fmm (dynamic precision floor)", dynTol)
	}
	slopStr := fmt.Sprintf(
		"a2c: ±%.1fmm, fc: ±%.1fmm, %s, cross: ±%.1fmm",
		slop.GetA2c(), slop.GetFrontCenter(), wbSlopStr, slop.GetCrossConfig(),
	)

	proofHeaders := []string{"Metric", "Value"}
	proofRows := [][]string{
		{"Published Fork A2C (Static)", statedStr},
		{"Reverse-Engineered CAD A2C", fmt.Sprintf("%.1f mm", b.Report.GeometricA2c)},
		{"Axle-to-Crown Deviation (Δ)", diffStr},
		{"Inferred Published Mode", formatPublishModeShort(b.Report.InferredPublishMode)},
		{"CAD Headset Spacer (geo_headset_spacer_mm)", fmt.Sprintf("%.1f mm", b.Report.GeoHeadsetSpacerMm)},
		{"Physical Headset Cup (headset_cup_type)", fmt.Sprintf("%s (%.1f mm)", b.Bike.HeadsetCupType, b.Report.CupStack)},
		{"Allowed Slop Tolerances (geo_slop_mm)", slopStr},
		{"Calculated Front Center (3D)", fmt.Sprintf("%.1f mm", b.Report.FrontCenterCalc)},
	}
	if b.Bike.FrontCenter > 0 {
		proofRows = append(proofRows, []string{"Published Front Center", fmt.Sprintf("%.1f mm", b.Bike.FrontCenter)})
	}
	_ = RenderOutput(a.Out, a.Format, "", "", proofHeaders, proofRows)

	if slop.HasCustomSlop() || slop.Reason != "" {
		fmt.Fprintln(a.Out, "\nActive Geo Slop Overrides (geo_slop_mm):")
		if slop.Reason != "" {
			fmt.Fprintf(a.Out, "  Reason: %s\n", slop.Reason)
		}
		var overrides []string
		if slop.A2c != nil {
			overrides = append(overrides, fmt.Sprintf("a2c: ±%.1fmm", *slop.A2c))
		}
		if slop.FrontCenter != nil {
			overrides = append(overrides, fmt.Sprintf("fc: ±%.1fmm", *slop.FrontCenter))
		}
		if slop.Wheelbase != nil {
			overrides = append(overrides, fmt.Sprintf("wb: ±%.1fmm", *slop.Wheelbase))
		}
		if slop.CrossConfig != nil {
			overrides = append(overrides, fmt.Sprintf("cross: ±%.1fmm", *slop.CrossConfig))
		}
		if len(overrides) > 0 {
			fmt.Fprintf(a.Out, "  Overrides: %s\n", strings.Join(overrides, ", "))
		}
	}

	// 3. Diagnostic Log
	var allIssues []model.ValidationIssue
	allIssues = append(allIssues, b.Report.Issues...)
	allIssues = append(allIssues, b.FrameReport.Issues...)
	if len(allIssues) > 0 {
		fmt.Fprintf(a.Out, "\nValidation Diagnostic Log (%d notices):\n", len(allIssues))
		for _, iss := range allIssues {
			sev := strings.ToUpper(string(iss.Severity))
			fmt.Fprintf(a.Out, "  [%s] %s: %s\n", sev, iss.Title, iss.Message)
			if iss.Expected != "" || iss.Actual != "" {
				fmt.Fprintf(a.Out, "         Expected: %s | Actual: %s\n", iss.Expected, iss.Actual)
			}
		}
	}
}
