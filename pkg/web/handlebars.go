package web

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

func (db *docb) handlebars(v url.Values) int {
	fl := &floats{v: v}
	gripInset := fl.get("grip-inset", model.DefaultGripInsetMm)
	rollIn := fl.get("roll", 0.0)
	cutWidth := fl.get("width", 0.0)
	if fl.err != nil {
		db.fail("%v", fl.err)
		return 1
	}
	hasWidth := strings.TrimSpace(v.Get("width")) != ""
	if !(gripInset > 0) || math.IsInf(gripInset, 0) {
		db.fail("Error: grip-inset must be positive (got %g)", gripInset)
		return 1
	}
	if math.IsNaN(rollIn) || math.IsInf(rollIn, 0) {
		db.fail("Error: roll must be finite (got %g)", rollIn)
		return 1
	}
	rollDeg := model.Deg(rollIn)
	if hasWidth && (!(cutWidth > 0) || math.IsInf(cutWidth, 0)) {
		db.fail("Error: width must be positive (got %g)", cutWidth)
		return 1
	}
	if hasWidth && cutWidth <= 2*gripInset {
		db.fail("Error: trimmed width %.1f mm must exceed 2 × grip-inset (%.1f mm) for positive hand lateral position", cutWidth, 2*gripInset)
		return 1
	}

	compareID := strings.TrimSpace(v.Get("compare"))
	var posArgs []string
	if id := strings.TrimSpace(v.Get("id")); id != "" {
		posArgs = []string{id}
	}
	if len(posArgs) == 0 && compareID == "" {
		return db.listHandlebars(gripInset, rollDeg, cutWidth, hasWidth)
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
		db.fail("Error: unrecognized handlebar %q", primaryQuery)
		return 1
	}

	if secondaryQuery == "" {
		if hasWidth && cutWidth > bar1.StockWidthMm {
			db.fail("Error: width %.1f mm exceeds %s stock width %.0f mm", cutWidth, bar1.Name, bar1.StockWidthMm)
			return 1
		}
		if hasWidth {
			bar1.CurrentWidthMm = cutWidth
		}
		if gripInset >= bar1.EffectiveWidthMm()/2 {
			db.fail("Error: grip-inset %.1f mm must be less than half of bar width %.0f mm", gripInset, bar1.EffectiveWidthMm())
			return 1
		}
		return db.showHandlebarDetail(bar1, gripInset, rollDeg)
	}

	if hasWidth {
		db.fail("Error: width trimming is only supported on single-bar or catalog views, not two-bar comparison")
		return 1
	}

	bar2, ok := catalog.GetHandlebar(secondaryQuery)
	if !ok {
		db.fail("Error: unrecognized comparison handlebar %q", secondaryQuery)
		return 1
	}

	return db.compareHandlebars(bar1, bar2, gripInset, rollDeg)
}

func (db *docb) listHandlebars(gripInset float64, rollDeg model.Deg, cutWidth float64, hasWidth bool) int {
	bars := catalog.AllHandlebars()
	maxStock := 0.0
	for _, b := range bars {
		if b.StockWidthMm > maxStock {
			maxStock = b.StockWidthMm
		}
	}
	if hasWidth && cutWidth > maxStock {
		db.fail("Error: width %.1f mm exceeds maximum catalog stock width %.0f mm", cutWidth, maxStock)
		return 1
	}

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
	byID := make(map[string]model.HandlebarSpec, len(bars))
	for _, b := range bars {
		if hasWidth && cutWidth < b.StockWidthMm {
			b.CurrentWidthMm = cutWidth
		}
		byID[b.ID] = b
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
	if math.Abs(float64(rollDeg)) > 0.001 {
		rollNote = fmt.Sprintf(" (Roll: %+.1f°)", rollDeg)
	}
	title := fmt.Sprintf("Canonical Handlebar Catalog (%d)%s", len(bars), rollNote)
	subtitle := fmt.Sprintf("Origin (0,0,0) is 31.8mm clamp center; (+X) forward reach, (+Y) upward stack, (+Z) lateral hand center (d_grip = %.0fmm)", gripInset)

	var insightLines []string
	if p800, ok := byID["persuader20_800"]; ok {
		if s30, ok := byID["sqlab3OX_16_30"]; ok {
			op := geometry.GetHandlebarOffset(p800, gripInset, rollDeg)
			os := geometry.GetHandlebarOffset(s30, gripInset, rollDeg)
			dx := op.X - os.X
			reachPhrase := fmt.Sprintf("pulling grips %.1fmm further forward", dx)
			if dx < 0 {
				reachPhrase = fmt.Sprintf("pulling grips only %.1fmm further back", -dx)
			}
			insightLines = append(insightLines, fmt.Sprintf(
				"1. Persuader 20° (%.0fmm) vs. SQlab 3OX 16° (30mm Rise): Even with 20° vs 16° sweep, Persuader's wide 200mm center section delays bend to z_b=%.0fmm, %s (%+.1fmm vs %+.1fmm) with stack within %.1fmm (%+.1fmm vs %+.1fmm).",
				p800.EffectiveWidthMm(), p800.EffectiveBendStartMm, reachPhrase, op.X, os.X, math.Abs(op.Y-os.Y), op.Y, os.Y))
		}
	}
	if b25, ok := byID["sqlab311_20_16_25"]; ok {
		if b50, ok := byID["sqlab311_20_16_50"]; ok {
			o25 := geometry.GetHandlebarOffset(b25, gripInset, rollDeg)
			o50 := geometry.GetHandlebarOffset(b50, gripInset, rollDeg)
			insightLines = append(insightLines, fmt.Sprintf(
				"2. Higher Rise Reduces Backsweep Reach Loss: On SQlab bars, higher rise consumes more lateral run in the 45°-50° riser section (z_b = 55 + 0.9*Rise). On 311 2.0, stepping from 25mm to 50mm rise gives %+.1fmm reach delta (%+.1fmm vs %+.1fmm) alongside %+.1fmm stack increase.",
				o50.X-o25.X, o50.X, o25.X, o50.Y-o25.Y))
		}
	}
	if s30, ok := byID["sqlab3OX_16_30"]; ok {
		os0 := geometry.GetHandlebarOffset(s30, gripInset, rollDeg)
		osM5 := geometry.GetHandlebarOffset(s30, gripInset, rollDeg-5.0)
		dRx := osM5.X - os0.X
		dRy := osM5.Y - os0.Y
		// Exact 2D rotation sensitivity at nominal (-70mm reach, +46mm stack) rolled from 0° to -5°.
		ang := -5.0 * math.Pi / 180.0
		nomDx := (-70.0*math.Cos(ang) + 46.0*math.Sin(ang)) - (-70.0)
		nomDy := (70.0*math.Sin(ang) + 46.0*math.Cos(ang)) - 46.0
		insightLines = append(insightLines, fmt.Sprintf(
			"3. Bar Roll Sensitivity: On SQlab 3OX 16° (30mm Rise) (%+.1fmm reach, %+.1fmm stack at %.1f° roll), 5° of additional backward roll shifts grips %+.1fmm Reach and %+.1fmm Stack (at nominal -70mm/+46mm from 0° to -5°, exact 2D rotation gives %+.1fmm Reach and %+.1fmm Stack).",
			os0.X, os0.Y, rollDeg, dRx, dRy, nomDx, nomDy))
	}

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
			Lines: insightLines,
		},
	}

	if err := db.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		db.fail("Error rendering output: %v", err)
		return 1
	}
	return 0
}

func (db *docb) showHandlebarDetail(b model.HandlebarSpec, gripInset float64, rollDeg model.Deg) int {
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

	for r := model.Deg(-10.0); r <= 10.001; r += 2.5 {
		rOff := geometry.GetHandlebarOffset(b, gripInset, r)
		diffX := rOff.X - neutralOff.X
		diffY := rOff.Y - neutralOff.Y
		marker := ""
		if math.Abs(float64(r-rollDeg)) < 0.001 {
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

	if err := db.render(title, subtitle, headers, rows); err != nil {
		db.fail("Error rendering output: %v", err)
		return 1
	}

	rollTitle := fmt.Sprintf("Roll Angle Sensitivity for %s", b.Name)
	rollSubtitle := "Positive roll rotates the bar top forward (trading stack for reach); negative roll rotates backward"
	if err := db.render(rollTitle, rollSubtitle, rollHeaders, rollRows); err != nil {
		db.fail("Error rendering roll table: %v", err)
		return 1
	}

	return 0
}

func (db *docb) compareHandlebars(bar1, bar2 model.HandlebarSpec, gripInset float64, rollDeg model.Deg) int {
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
				fmt.Sprintf("Switching to %s shifts hand grips %.1f mm %s (Reach) and %.1f mm %s (Stack).",
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

	if err := db.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		db.fail("Error rendering comparison: %v", err)
		return 1
	}
	return 0
}
