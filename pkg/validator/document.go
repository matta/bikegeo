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
	case model.ModeUnspecified:
	}
	// ModeUnspecified, the zero value and any unknown mode.
	return "Unspecified"
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
		if a2cDiff, ok := b.Report.A2cDifference.Get(); ok {
			diff = fmt.Sprintf("%+.1f mm", a2cDiff)
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
	if a2cDiff, ok := b.Report.A2cDifference.Get(); ok {
		diff = fmt.Sprintf("%+.1f mm", a2cDiff)
	}
	slop := b.Bike.GetGeoSlopMm()
	checks := []struct {
		label  string
		budget *model.ErrorBudget
	}{
		{"Stack / A2C", b.Report.A2cBudget},
		{"Front Center", b.Report.FrontCenterBudget},
		{"Wheelbase", b.Report.WheelbaseBudget},
	}

	proof := [][]string{
		{"Published Fork A2C (Static)", stated},
		{"Reverse-Engineered CAD A2C", fmt.Sprintf("%.1f mm", b.Report.GeometricA2c)},
		{"Axle-to-Crown Deviation (Δ)", diff},
		{"CAD Convention", PublishModeShort(b.Report.InferredPublishMode)},
		{"CAD Headset Spacer (geo_headset_spacer_mm)", fmt.Sprintf("%.1f mm", b.Report.GeoHeadsetSpacerMm)},
		{"Physical Headset Cup (headset_cup_type)", fmt.Sprintf("%s (%.1f mm)", b.Bike.HeadsetCupType, b.Report.CupStack)},
	}
	for _, c := range checks {
		if c.budget != nil {
			proof = append(proof, []string{"Rounding Budget: " + c.label, budgetCell(*c.budget)})
		}
	}
	proof = append(proof,
		[]string{"Declared Allowance (geo_slop_mm)", slop.String()},
		[]string{"Calculated Front Center (3D)", fmt.Sprintf("%.1f mm", b.Report.FrontCenterCalc)},
	)
	if b.Bike.FrontCenter > 0 {
		proof = append(proof, []string{"Published Front Center", fmt.Sprintf("%.1f mm", b.Bike.FrontCenter)})
	}
	doc.AddTable(report.FromStrings("validation-proof", "", "", []string{"Metric", "Value"}, proof))

	var bd strings.Builder
	bd.WriteString("Rounding Budget Breakdown (worst-case effect of each published input's rounding):")
	for _, c := range checks {
		if c.budget == nil {
			continue
		}
		fmt.Fprintf(&bd, "\n  %s: %s", c.label, topBudgetTerms(*c.budget, len(c.budget.Terms)))
	}
	doc.AddText(bd.String())

	if slop.HasCustomSlop() || slop.Reason != "" {
		var sb strings.Builder
		sb.WriteString("Declared Chart-Error Allowance (geo_slop_mm):")
		if slop.Reason != "" {
			fmt.Fprintf(&sb, "\n  Reason: %s", slop.Reason)
		}
		if slop.HasCustomSlop() {
			fmt.Fprintf(&sb, "\n  Allowance beyond rounding: %s", slop.String())
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

// budgetTierLabel names the part of a budget a residual needs.
func budgetTierLabel(t model.BudgetTier) string {
	switch t {
	case model.TierLengthRounding:
		return "explained by length rounding"
	case model.TierAngleRounding:
		return "needs head angle rounding"
	case model.TierAllowance:
		return "needs declared allowance"
	case model.TierExceeds:
	}
	// TierExceeds and any unknown tier.
	return "exceeds rounding"
}

// budgetCell renders a budget for the validation proof table, for example
// "Δ -1.7 mm vs ±2.9 mm (lengths ±1.6 + HTA ±1.3), 59% used — needs head angle rounding".
func budgetCell(b model.ErrorBudget) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Δ %+.1f mm vs ±%.1f mm (lengths ±%.1f", b.Residual, b.RoundingTol(), b.LengthTol)
	if b.AngleTol > 0 {
		fmt.Fprintf(&sb, " + HTA ±%.1f", b.AngleTol)
	}
	fmt.Fprintf(&sb, "), %.0f%% used", 100*b.Saturation())
	if b.Allowance > 0 {
		fmt.Fprintf(&sb, ", +%.1f mm allowance", b.Allowance)
	}
	fmt.Fprintf(&sb, " — %s", budgetTierLabel(b.Tier()))
	return sb.String()
}
