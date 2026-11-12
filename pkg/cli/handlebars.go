package cli

import (
	"flag"
	"fmt"
	"math"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

func (a *App) runHandlebars(args []string) int {
	fs := flag.NewFlagSet("handlebars", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var gripInset float64
	var rollDeg float64
	var cutWidth float64
	var compareID string

	fs.Float64Var(&gripInset, "grip-inset", model.DefaultGripInsetMm, "Inboard distance from bar tip to hand center in mm (default: 65mm)")
	fs.Float64Var(&gripInset, "gi", model.DefaultGripInsetMm, "Alias for --grip-inset")
	fs.Float64Var(&rollDeg, "roll", 0.0, "Handlebar roll angle in clamp in degrees (+ rolls top forward, default: 0°)")
	fs.Float64Var(&rollDeg, "r", 0.0, "Alias for --roll")
	fs.Float64Var(&cutWidth, "width", 0.0, "Custom trimmed handlebar width in mm (optional)")
	fs.Float64Var(&cutWidth, "w", 0.0, "Alias for --width")
	fs.StringVar(&compareID, "compare", "", "Compare with another handlebar ID")
	fs.StringVar(&compareID, "c", "", "Alias for --compare")

	reordered := reorderFlagsForStdlib(fs, args, nil)
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	posArgs := fs.Args()
	if len(posArgs) == 0 && compareID == "" {
		return a.listHandlebars(gripInset, rollDeg, cutWidth)
	}

	primaryQuery := ""
	if len(posArgs) > 0 {
		primaryQuery = posArgs[0]
	}

	secondaryQuery := compareID
	if secondaryQuery == "" && len(posArgs) > 1 {
		secondaryQuery = posArgs[1]
	}

	bar1, ok := catalog.GetHandlebar(primaryQuery)
	if !ok {
		fmt.Fprintf(a.Err, "Error: unrecognized handlebar %q\n", primaryQuery)
		return 1
	}

	if cutWidth > 0 {
		bar1.CurrentWidthMm = cutWidth
	}

	if secondaryQuery == "" {
		return a.showHandlebarDetail(bar1, gripInset, rollDeg)
	}

	bar2, ok := catalog.GetHandlebar(secondaryQuery)
	if !ok {
		fmt.Fprintf(a.Err, "Error: unrecognized comparison handlebar %q\n", secondaryQuery)
		return 1
	}

	return a.compareHandlebars(bar1, bar2, gripInset, rollDeg)
}

func (a *App) listHandlebars(gripInset, rollDeg, cutWidth float64) int {
	bars := catalog.AllHandlebars()
	headers := []string{
		"Handlebar ID",
		"Name",
		"Width (W)",
		"Rise (Spec)",
		"Backsweep",
		"Upsweep",
		"Bend Start (z_b)",
		"Base Rise (y_k)",
		"Reach Offset (ΔX)",
		"Stack Offset (ΔY)",
		"Lateral (z_hand)",
	}

	var rows [][]string
	for _, b := range bars {
		if cutWidth > 0 {
			b.CurrentWidthMm = cutWidth
		}
		offset := geometry.GetHandlebarOffset(b, gripInset, rollDeg)
		yk := b.BaseRiseAtBend()
		zHand := b.HandLateralMm(gripInset)

		riseModeLabel := ""
		if b.RiseMode == model.RiseModeBarEndCenter {
			riseModeLabel = " (tip)"
		}

		rows = append(rows, []string{
			b.ID,
			b.Name,
			fmt.Sprintf("%.0f mm", b.EffectiveWidthMm()),
			fmt.Sprintf("%.0f mm%s", b.StatedRiseMm, riseModeLabel),
			fmt.Sprintf("%.0f°", b.BacksweepDeg),
			fmt.Sprintf("%.0f°", b.UpsweepDeg),
			fmt.Sprintf("%.1f mm", b.EffectiveBendStartMm),
			fmt.Sprintf("%.1f mm", yk),
			fmt.Sprintf("%+.1f mm", offset.X),
			fmt.Sprintf("%+.1f mm", offset.Y),
			fmt.Sprintf("%.1f mm", zHand),
		})
	}

	rollNote := ""
	if math.Abs(rollDeg) > 0.001 {
		rollNote = fmt.Sprintf(" (Roll: %+.1f°)", rollDeg)
	}
	title := fmt.Sprintf("Canonical Handlebar Catalog (%d)%s", len(bars), rollNote)
	subtitle := fmt.Sprintf("Origin (0,0,0) is 31.8mm clamp center; (+X) forward reach, (+Y) upward stack, (+Z) lateral hand center (d_grip = %.0fmm)", gripInset)

	footers := []report.Footer{
		{
			Title: "Rise Measurement Traps: Knuckle vs. Bar-End Tip",
			Lines: []string{
				"Standard Knuckle (SQlab / DIN): Rise is measured from clamp axis to the post-rise bend knuckle. Outboard upsweep adds height on top of stated rise (y_k = StatedRise).",
				"Bar-End Center (Tumbleweed Persuader): Rise is explicitly measured from clamp center to the center of the bar-end tip. Outboard upsweep is already baked in; base rise at knuckle y_k = StatedRise - (W_0/2 - z_b)*tan(alpha).",
			},
		},
		{
			Title: "Key Kinematic Insights",
			Lines: []string{
				"1. Persuader 20° (800mm) vs. SQlab 3OX 16° (30mm Rise): Even with 20° vs 16° sweep, Persuader's wide 200mm center section delays bend to z_b=135mm, pulling grips only 3.1mm further back (-72.8mm vs -69.7mm) with stack within 1.5mm (+45.5mm vs +47.0mm).",
				"2. Higher Rise Reduces Backsweep Reach Loss: On SQlab bars, higher rise consumes more lateral run in the 45°-50° riser section (z_b = 55 + 0.9*Rise). On 311 2.0, stepping from 25mm to 50mm rise gives +6.4mm forward reach (-58.8mm vs -65.2mm) alongside +23.0mm stack increase.",
				"3. Bar Roll Sensitivity: At ~70mm backsweep and ~46mm stack, every 5° of backward roll (-5° roll angle) shifts grips -4.0mm further back (reach loss) and -6.1mm lower (stack loss).",
			},
		},
	}

	if err := a.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		fmt.Fprintf(a.Err, "Error rendering output: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) showHandlebarDetail(b model.HandlebarSpec, gripInset, rollDeg float64) int {
	offset := geometry.GetHandlebarOffset(b, gripInset, rollDeg)
	yk := b.BaseRiseAtBend()
	zHand := b.HandLateralMm(gripInset)
	sweptRun := math.Max(0, zHand-b.EffectiveBendStartMm)

	title := fmt.Sprintf("Handlebar Detail: %s", b.Name)
	subtitle := fmt.Sprintf("ID: %s | Measurement Mode: %s", b.ID, b.RiseMode)

	headers := []string{"Parameter", "Value", "Description"}
	rows := [][]string{
		{"Stock Width (W_0)", fmt.Sprintf("%.0f mm", b.StockWidthMm), "Original manufactured end-to-end width"},
		{"Current Width (W)", fmt.Sprintf("%.0f mm", b.EffectiveWidthMm()), "Effective / cut handlebar width"},
		{"Stated Rise (R_spec)", fmt.Sprintf("%.0f mm", b.StatedRiseMm), "Manufacturer published rise specification"},
		{"Rise Mode", string(b.RiseMode), "knuckle (outboard upsweep adds) vs bar_end_center (baked in)"},
		{"Backsweep (β)", fmt.Sprintf("%.1f°", b.BacksweepDeg), "Horizontal rearward bend angle in X-Z plan view"},
		{"Upsweep (α)", fmt.Sprintf("%.1f°", b.UpsweepDeg), "Vertical upward bend angle in Y-Z front view"},
		{"Effective Bend Start (z_b)", fmt.Sprintf("%.1f mm", b.EffectiveBendStartMm), "Lateral distance from centerline where backsweep begins"},
		{"Base Rise at Bend (y_k)", fmt.Sprintf("%.1f mm", yk), "Vertical height at bend intersection (knuckle)"},
		{"Grip Inset (d_grip)", fmt.Sprintf("%.1f mm", gripInset), "Inboard distance from bar tip to hand center"},
		{"Hand Lateral (z_hand)", fmt.Sprintf("%.1f mm", zHand), "Lateral distance from centerline to hand center (W/2 - d_grip)"},
		{"Swept Lever Arm (L_z)", fmt.Sprintf("%.1f mm", sweptRun), "Active swept span from bend to hand center (z_hand - z_b)"},
		{"Roll Angle (θ_roll)", fmt.Sprintf("%+.1f°", rollDeg), "Rotation in stem clamp (+ forward, - backward)"},
		{"Reach Offset (ΔX_bar)", fmt.Sprintf("%+.1f mm", offset.X), "Longitudinal displacement from clamp to hand center (+ forward, - rearward)"},
		{"Stack Offset (ΔY_bar)", fmt.Sprintf("%+.1f mm", offset.Y), "Vertical displacement from clamp to hand center (+ upward)"},
	}

	// Roll sweep table
	rollHeaders := []string{"Roll Angle (θ)", "Reach Offset (ΔX)", "Stack Offset (ΔY)", "ΔReach vs Neutral", "ΔStack vs Neutral"}
	var rollRows [][]string
	neutralOff := geometry.GetHandlebarOffset(b, gripInset, 0.0)

	for r := -10.0; r <= 10.001; r += 2.5 {
		rOff := geometry.GetHandlebarOffset(b, gripInset, r)
		diffX := rOff.X - neutralOff.X
		diffY := rOff.Y - neutralOff.Y
		marker := ""
		if math.Abs(r-rollDeg) < 0.001 {
			marker = " *"
		}
		rollRows = append(rollRows, []string{
			fmt.Sprintf("%+.1f°%s", r, marker),
			fmt.Sprintf("%+.1f mm", rOff.X),
			fmt.Sprintf("%+.1f mm", rOff.Y),
			fmt.Sprintf("%+.1f mm", diffX),
			fmt.Sprintf("%+.1f mm", diffY),
		})
	}

	if err := a.render(title, subtitle, headers, rows); err != nil {
		fmt.Fprintf(a.Err, "Error rendering output: %v\n", err)
		return 1
	}

	fmt.Fprintln(a.Out)
	rollTitle := fmt.Sprintf("Roll Angle Sensitivity for %s", b.Name)
	rollSubtitle := "Positive roll rotates the bar top forward (trading stack for reach); negative roll rotates backward"
	if err := a.render(rollTitle, rollSubtitle, rollHeaders, rollRows); err != nil {
		fmt.Fprintf(a.Err, "Error rendering roll table: %v\n", err)
		return 1
	}

	return 0
}

func (a *App) compareHandlebars(bar1, bar2 model.HandlebarSpec, gripInset, rollDeg float64) int {
	off1 := geometry.GetHandlebarOffset(bar1, gripInset, rollDeg)
	off2 := geometry.GetHandlebarOffset(bar2, gripInset, rollDeg)
	deltaGrip := geometry.HandlebarDelta(bar1, bar2, gripInset, rollDeg)
	stemComp := geometry.HandlebarStemCompensation(bar1, bar2, gripInset, rollDeg)

	title := fmt.Sprintf("Handlebar Fit Translation: %s -> %s", bar1.Name, bar2.Name)
	subtitle := fmt.Sprintf("Grip inset: %.0f mm | Roll angle: %+.1f°", gripInset, rollDeg)

	headers := []string{"Metric", bar1.Name, bar2.Name, "Delta (Bar 2 - Bar 1)"}
	rows := [][]string{
		{"Width (W)", fmt.Sprintf("%.0f mm", bar1.EffectiveWidthMm()), fmt.Sprintf("%.0f mm", bar2.EffectiveWidthMm()), fmt.Sprintf("%+.0f mm", bar2.EffectiveWidthMm()-bar1.EffectiveWidthMm())},
		{"Stated Rise", fmt.Sprintf("%.0f mm", bar1.StatedRiseMm), fmt.Sprintf("%.0f mm", bar2.StatedRiseMm), fmt.Sprintf("%+.0f mm", bar2.StatedRiseMm-bar1.StatedRiseMm)},
		{"Backsweep (β)", fmt.Sprintf("%.1f°", bar1.BacksweepDeg), fmt.Sprintf("%.1f°", bar2.BacksweepDeg), fmt.Sprintf("%+.1f°", bar2.BacksweepDeg-bar1.BacksweepDeg)},
		{"Upsweep (α)", fmt.Sprintf("%.1f°", bar1.UpsweepDeg), fmt.Sprintf("%.1f°", bar2.UpsweepDeg), fmt.Sprintf("%+.1f°", bar2.UpsweepDeg-bar1.UpsweepDeg)},
		{"Bend Start (z_b)", fmt.Sprintf("%.1f mm", bar1.EffectiveBendStartMm), fmt.Sprintf("%.1f mm", bar2.EffectiveBendStartMm), fmt.Sprintf("%+.1f mm", bar2.EffectiveBendStartMm-bar1.EffectiveBendStartMm)},
		{"Base Rise (y_k)", fmt.Sprintf("%.1f mm", bar1.BaseRiseAtBend()), fmt.Sprintf("%.1f mm", bar2.BaseRiseAtBend()), fmt.Sprintf("%+.1f mm", bar2.BaseRiseAtBend()-bar1.BaseRiseAtBend())},
		{"Reach Offset (ΔX)", fmt.Sprintf("%+.1f mm", off1.X), fmt.Sprintf("%+.1f mm", off2.X), fmt.Sprintf("%+.1f mm", deltaGrip.X)},
		{"Stack Offset (ΔY)", fmt.Sprintf("%+.1f mm", off1.Y), fmt.Sprintf("%+.1f mm", off2.Y), fmt.Sprintf("%+.1f mm", deltaGrip.Y)},
		{"Lateral Hand (z_hand)", fmt.Sprintf("%.1f mm", off1.Z), fmt.Sprintf("%.1f mm", off2.Z), fmt.Sprintf("%+.1f mm", deltaGrip.Z)},
	}

	reachDir := "forward"
	if deltaGrip.X < 0 {
		reachDir = "rearward (closer to rider)"
	}
	stackDir := "higher (upward)"
	if deltaGrip.Y < 0 {
		stackDir = "lower (downward)"
	}

	footers := []report.Footer{
		{
			Title: "Resulting Hand Grip Position Shift (Fixed Stem Clamp)",
			Lines: []string{
				fmt.Sprintf("Switching to %s shifts hand grips %+.1f mm %s (Reach) and %+.1f mm %s (Stack).",
					bar2.Name, math.Abs(deltaGrip.X), reachDir, math.Abs(deltaGrip.Y), stackDir),
			},
		},
		{
			Title: "Required Stem Clamp Adjustment to Maintain Identical Hand Position",
			Lines: []string{
				fmt.Sprintf("To preserve identical hand position in space, stem clamp center must shift %+.1f mm Reach and %+.1f mm Stack.",
					stemComp.X, stemComp.Y),
			},
		},
	}

	if err := a.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		fmt.Fprintf(a.Err, "Error rendering comparison: %v\n", err)
		return 1
	}
	return 0
}
