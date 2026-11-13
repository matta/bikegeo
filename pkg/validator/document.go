package validator

import (
	"fmt"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

// Target names one frame size to validate.
type Target struct {
	Frame model.FrameModel
	Size  string
}

// Bundle is the validation outcome for one Target.
type Bundle struct {
	Frame       model.FrameModel
	Size        string
	Bike        model.BikeGeometry
	Report      model.ValidationReport
	FrameReport model.FrameValidationReport
}

// Valid reports whether both the geometry and the frame-level checks passed.
func (b Bundle) Valid() bool { return b.Report.IsValid && b.FrameReport.IsValid }

// PublishModeShort is a compact label for how a manufacturer published its chart.
func PublishModeShort(mode model.PublishMode) string {
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

// BuildValidationDocument validates every target and lays the outcome out as a
// document: a summary table when there are several targets, then the detailed
// proof for a single target or for each invalid one. allValid is false if any
// target failed.
func BuildValidationDocument(targets []Target) (doc report.Document, allValid bool, err error) {
	var bundles []Bundle
	allValid = true
	for _, t := range targets {
		bike, err := geometry.MaterializeBikeGeometry(t.Frame, t.Size)
		if err != nil {
			return doc, false, fmt.Errorf("materializing %s (%s): %w", t.Frame.ID, t.Size, err)
		}
		b := Bundle{
			Frame:       t.Frame,
			Size:        t.Size,
			Bike:        bike,
			Report:      ValidateBikeGeometry(bike),
			FrameReport: ValidateFrame(t.Frame),
		}
		if !b.Valid() {
			allValid = false
		}
		bundles = append(bundles, b)
	}

	if len(bundles) > 1 {
		doc.AddTable(summaryTable(bundles))
	}
	for _, b := range bundles {
		if len(bundles) > 1 && b.Valid() {
			continue
		}
		addDetail(&doc, b)
	}
	return doc, allValid, nil
}

func summaryTable(bundles []Bundle) report.Table {
	t := report.Table{
		ID:    "validation-summary",
		Title: fmt.Sprintf("CAD Geometry Validation Summary (%d Frames)", len(bundles)),
	}
	for _, h := range []string{"Frame ID", "Manufacturer & Model", "Size", "Status", "Stated A2C", "CAD Geo A2C", "Δ A2C", "CAD Convention", "Supported Forks"} {
		t.Columns = append(t.Columns, report.Column{ID: report.Slug(h), Label: h})
	}
	for _, b := range bundles {
		status := report.Cell{Text: "VALID", Zone: report.ZoneIdeal}
		if !b.Valid() {
			status = report.Cell{Text: "INVALID", Zone: report.ZoneOutside}
		}
		stated := "N/A"
		if b.Report.StatedA2c > 0 {
			stated = fmt.Sprintf("%.0f mm", b.Report.StatedA2c)
		}
		diff := "N/A"
		if b.Report.A2cDifference != nil {
			diff = fmt.Sprintf("%+.1f mm", *b.Report.A2cDifference)
		}
		t.Rows = append(t.Rows, report.Row{Kind: report.KindData, Cells: []report.Cell{
			{Text: b.Frame.ID},
			{Text: b.Frame.Manufacturer + " " + b.Frame.Model},
			{Text: b.Size},
			status,
			{Text: stated},
			{Text: fmt.Sprintf("%.1f mm", b.Report.GeometricA2c)},
			{Text: diff},
			{Text: PublishModeShort(b.Report.InferredPublishMode)},
			{Text: strings.Join(b.Frame.GetSupportedForkOptions(), ", ")},
		}})
	}
	return t
}

func addDetail(doc *report.Document, b Bundle) {
	badge := "CAD COHERENT (VALID)"
	if !b.Valid() {
		badge = "VALIDATION ISSUES DETECTED (INVALID)"
	}
	spec := report.FromStrings(
		"validation-"+report.Slug(b.Frame.ID+"-"+b.Size),
		fmt.Sprintf("=== %s %s (Size %s) [%s] ===", b.Bike.Manufacturer, b.Bike.Model, b.Size, badge),
		fmt.Sprintf("Category: %s | Supported Forks: %s | Default Fork: %s",
			b.Bike.Category, strings.Join(b.Frame.GetSupportedForkOptions(), ", "), b.Frame.GetDefaultForkSpecString()),
		[]string{"Reach", "Stack", "HTA", "STA", "HTL", "BB Drop", "Fork Rake", "Wheelbase"},
		[][]string{{
			fmt.Sprintf("%.1f mm", b.Bike.Reach),
			fmt.Sprintf("%.1f mm", b.Bike.Stack),
			fmt.Sprintf("%.1f°", b.Bike.HtaDeg),
			fmt.Sprintf("%.1f°", b.Bike.StaDeg),
			fmt.Sprintf("%.1f mm", b.Bike.Htl),
			fmt.Sprintf("%.1f mm", b.Bike.BbDrop),
			fmt.Sprintf("%.1f mm", b.Bike.ForkRake),
			fmt.Sprintf("%.1f mm", b.Bike.Wheelbase),
		}},
	)
	doc.AddTable(spec)

	doc.AddText(fmt.Sprintf(
		"Master Stack Equation & Headset Proof:\n"+
			"  Formula: Stack = BB_Drop - Rake*cos(HTA) + (A2C + Geo_Spacer + HTL)*sin(HTA)\n"+
			"  Derived: (%.0f - %.0f + %.0f*cos(%.1f°)) / sin(%.1f°) - %.0f = %.1f mm (Fork A2C) + %.1f mm (CAD Spacer) = %.1f mm",
		b.Bike.Stack, b.Bike.BbDrop, b.Bike.ForkRake, b.Bike.HtaDeg, b.Bike.HtaDeg, b.Bike.Htl,
		b.Report.GeometricA2c, b.Report.GeoHeadsetSpacerMm, b.Report.TotalStructuralA2c,
	))

	stated := "Not specified"
	if b.Report.StatedA2c > 0 {
		stated = fmt.Sprintf("%.0f mm", b.Report.StatedA2c)
	} else if a2c := b.Bike.GetCadA2c(); a2c > 0 {
		stated = fmt.Sprintf("%.0f mm", a2c)
	}
	diff := "N/A"
	if b.Report.A2cDifference != nil {
		diff = fmt.Sprintf("%+.1f mm", *b.Report.A2cDifference)
	}
	slop := b.Bike.GetGeoSlopMm()
	var wb string
	if slop.Wheelbase != nil {
		wb = fmt.Sprintf("wb: ±%.1fmm", slop.GetWheelbase())
	} else {
		dynWb := CalculateDynamicWheelbaseTolerance(b.Bike, b.Report.TotalStructuralA2c)
		if dynWb <= model.DefaultWheelbaseFloorMm {
			wb = fmt.Sprintf("wb: ±%.1fmm (dynamic precision floor)", dynWb)
		} else {
			wb = fmt.Sprintf("wb: ±%.1fmm (dynamic rounding propagation)", dynWb)
		}
	}
	slopStr := fmt.Sprintf("a2c: ±%.1fmm, fc: ±%.1fmm, %s", slop.GetA2c(), slop.GetFrontCenter(), wb)

	proof := [][]string{
		{"Published Fork A2C (Static)", stated},
		{"Reverse-Engineered CAD A2C", fmt.Sprintf("%.1f mm", b.Report.GeometricA2c)},
		{"Axle-to-Crown Deviation (Δ)", diff},
		{"CAD Convention", PublishModeShort(b.Report.InferredPublishMode)},
		{"CAD Headset Spacer (geo_headset_spacer_mm)", fmt.Sprintf("%.1f mm", b.Report.GeoHeadsetSpacerMm)},
		{"Physical Headset Cup (headset_cup_type)", fmt.Sprintf("%s (%.1f mm)", b.Bike.HeadsetCupType, b.Report.CupStack)},
		{"Allowed Slop Tolerances (geo_slop_mm)", slopStr},
		{"Calculated Front Center (3D)", fmt.Sprintf("%.1f mm", b.Report.FrontCenterCalc)},
	}
	if b.Bike.FrontCenter > 0 {
		proof = append(proof, []string{"Published Front Center", fmt.Sprintf("%.1f mm", b.Bike.FrontCenter)})
	}
	doc.AddTable(report.FromStrings("validation-proof", "", "", []string{"Metric", "Value"}, proof))

	if slop.HasCustomSlop() || slop.Reason != "" {
		var sb strings.Builder
		sb.WriteString("Active Geo Slop Overrides (geo_slop_mm):")
		if slop.Reason != "" {
			fmt.Fprintf(&sb, "\n  Reason: %s", slop.Reason)
		}
		var o []string
		if slop.A2c != nil {
			o = append(o, fmt.Sprintf("a2c: ±%.1fmm", *slop.A2c))
		}
		if slop.FrontCenter != nil {
			o = append(o, fmt.Sprintf("fc: ±%.1fmm", *slop.FrontCenter))
		}
		if slop.Wheelbase != nil {
			o = append(o, fmt.Sprintf("wb: ±%.1fmm", *slop.Wheelbase))
		}
		if len(o) > 0 {
			fmt.Fprintf(&sb, "\n  Overrides: %s", strings.Join(o, ", "))
		}
		doc.AddText(sb.String())
	}

	issues := append(append([]model.ValidationIssue{}, b.Report.Issues...), b.FrameReport.Issues...)
	if len(issues) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, "Validation Diagnostic Log (%d notices):", len(issues))
		for _, iss := range issues {
			fmt.Fprintf(&sb, "\n  [%s] %s: %s", strings.ToUpper(string(iss.Severity)), iss.Title, iss.Message)
			if iss.Expected != "" || iss.Actual != "" {
				fmt.Fprintf(&sb, "\n         Expected: %s | Actual: %s", iss.Expected, iss.Actual)
			}
		}
		doc.AddText(sb.String())
	}
}
