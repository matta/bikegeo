package fit

import (
	"fmt"
	"strings"

	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

func passFail(ok bool) report.Cell {
	if ok {
		return report.Cell{Text: "PASS", Zone: report.ZoneIdeal}
	}
	return report.Cell{Text: "FAIL", Zone: report.ZoneOutside}
}

// collectErrorNotices extracts frame-level and per-size [ERROR] notices in deterministic order.
func collectErrorNotices(res *FitResult) []string {
	if res == nil || res.IsValid {
		return nil
	}
	var notices []string
	for _, iss := range res.FrameReport.Issues {
		if iss.Severity == model.SeverityError {
			notices = append(notices, fmt.Sprintf("[ERROR] %s: %s", iss.Title, iss.Message))
		}
	}
	for _, sz := range res.Frame.GetSizes() {
		rep, ok := res.FrameReport.SizeReports[sz]
		if !ok {
			continue
		}
		for _, iss := range rep.Issues {
			if iss.Severity == model.SeverityError {
				notices = append(notices, fmt.Sprintf("[ERROR] [Size %s] %s: %s", sz, iss.Title, iss.Message))
			}
		}
	}
	return notices
}

func summarizeFailureReason(res *FitResult) string {
	notices := collectErrorNotices(res)
	if len(notices) == 0 {
		if res != nil && !res.IsValid {
			return "Validation failed"
		}
		return "—"
	}
	first := strings.ReplaceAll(strings.TrimPrefix(notices[0], "[ERROR] "), "\n", " ")
	if len(notices) > 1 {
		return fmt.Sprintf("%s (+%d more)", first, len(notices)-1)
	}
	return first
}

func candidatesTable(cands []ComboResult) report.Table {
	t := report.Table{
		ID:    "fit-candidates",
		Title: "Candidate Physical Solutions (Geo slop is an absolute last resort)",
	}
	for _, h := range []string{"Physical Configuration", "max|ΔA2C|", "max|ΔWB|", "Valid?", "Solved Go Fields"} {
		t.Columns = append(t.Columns, report.Column{ID: report.Slug(h), Label: h})
	}
	for _, c := range cands {
		t.Rows = append(t.Rows, report.Row{Kind: report.KindData, Cells: []report.Cell{
			{Text: c.Description},
			{Text: fmt.Sprintf("%.1f mm", c.MaxAbsDeltaA2c)},
			{Text: fmt.Sprintf("%.1f mm", c.MaxAbsDeltaWb)},
			passFail(c.IsValid),
			{Text: strings.Join(c.Assignments, ", ")},
		}})
	}
	return t
}

// BuildSweepDocument lays out the results of RunSweeps. With detail it follows
// the summary with each combination's per-size residual table and error notices.
func BuildSweepDocument(frame model.FrameModel, combos []ComboResult, detail bool) report.Document {
	var doc report.Document
	t := report.Table{
		ID:    "fit-sweep",
		Title: fmt.Sprintf("Fit Sweep: %s %s (%d combinations)", frame.Manufacturer, frame.Model, len(combos)),
	}
	for _, h := range []string{"Combo", "max|ΔA2C|", "max|ΔFC|", "max|ΔWB|", "Needed slop (a2c/fc/wb)", "Validate", "Failure Reason"} {
		t.Columns = append(t.Columns, report.Column{ID: report.Slug(h), Label: h})
	}
	anyValid := false
	for _, c := range combos {
		if c.IsValid {
			anyValid = true
		}
		hasFc := c.FitResult != nil && c.FitResult.HasPublishedFc
		hasWb := c.FitResult != nil && c.FitResult.HasPublishedWb
		fc, wb := "—", "—"
		slop := []string{fmt.Sprintf("%.1f", c.MinPassingSlopA2c), "—", "—"}
		if hasFc {
			fc = fmt.Sprintf("%.1f mm", c.MaxAbsDeltaFc)
			slop[1] = fmt.Sprintf("%.1f", c.MinPassingSlopFc)
		}
		if hasWb {
			wb = fmt.Sprintf("%.1f mm", c.MaxAbsDeltaWb)
			slop[2] = fmt.Sprintf("%.1f", c.MinPassingSlopWb)
		}
		reason := "—"
		if !c.IsValid {
			reason = summarizeFailureReason(c.FitResult)
		}
		t.Rows = append(t.Rows, report.Row{Kind: report.KindData, Cells: []report.Cell{
			{Text: c.Description},
			{Text: fmt.Sprintf("%.1f mm", c.MaxAbsDeltaA2c)},
			{Text: fc},
			{Text: wb},
			{Text: strings.Join(slop, " / ")},
			passFail(c.IsValid),
			{Text: reason},
		}})
	}
	doc.AddTable(t)
	if detail {
		for _, c := range combos {
			doc.AddTable(residualsTable("Combination: "+c.Description, c.FitResult))
			if notices := collectErrorNotices(c.FitResult); len(notices) > 0 {
				var sb strings.Builder
				fmt.Fprintf(&sb, "Validation Errors (%d):", len(notices))
				for _, n := range notices {
					fmt.Fprintf(&sb, "\n  %s", n)
				}
				doc.AddText(sb.String())
			}
		}
	}
	if !anyValid && len(combos) > 0 {
		solverFrame := frame
		configID := "stock"
		if combos[0].FitResult != nil {
			solverFrame = combos[0].FitResult.Frame
			if combos[0].FitResult.ConfigID != "" {
				configID = combos[0].FitResult.ConfigID
			}
		}
		if cands, err := SolveFit(solverFrame, configID, frame.GetDefaultSize()); err == nil && len(cands) > 0 {
			doc.AddTable(candidatesTable(cands))
		}
	}
	return doc
}

// BuildAnalysisDocument lays out a single-configuration fit: residuals, the
// minimax summary, slop requirements, validation status and (when the frame
// does not fit) candidate physical solutions.
func BuildAnalysisDocument(frame model.FrameModel, configID, targetSize string) (report.Document, error) {
	var doc report.Document
	res, err := AnalyzeFit(frame, configID)
	if err != nil {
		return doc, err
	}

	forkDesc := fmt.Sprintf("A2C %.0f mm, rake %.0f mm", frame.CadFork.A2cMm, frame.CadFork.RakeMm)
	if frame.CadFork.Fork != "" {
		forkDesc = fmt.Sprintf("%s (%s)", frame.CadFork.Fork, forkDesc)
	}
	structA2c := frame.CadFork.A2cMm + frame.GetGeoHeadsetSpacerMm()
	doc.AddTable(residualsTable(fmt.Sprintf(
		"%s %s — CAD Fork (%s, spacer %.1f mm, structural A2C %.1f mm, sag %.0f%%)",
		frame.Manufacturer, frame.Model, forkDesc, frame.GetGeoHeadsetSpacerMm(), structA2c, frame.CadFork.SagPct), res))

	req := []string{fmt.Sprintf("a2c ±%.1f", res.MinPassingSlopA2c)}
	if res.HasPublishedFc {
		req = append(req, fmt.Sprintf("fc ±%.1f", res.MinPassingSlopFc))
	}
	if res.HasPublishedWb {
		req = append(req, fmt.Sprintf("wb ±%.1f", res.MinPassingSlopWb))
	}
	declared := frame.GetGeoSlopMm()
	decl := []string{fmt.Sprintf("a2c ±%.1f", declared.GetA2c())}
	if res.HasPublishedFc {
		decl = append(decl, fmt.Sprintf("fc ±%.1f", declared.GetFrontCenter()))
	}
	if res.HasPublishedWb {
		if declared.Wheelbase != nil {
			decl = append(decl, fmt.Sprintf("wb ±%.1f", *declared.Wheelbase))
		} else {
			decl = append(decl, "wb dynamic")
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Best-fit structural A2C (minimax): %.2f mm  (mean %.2f, spread %.2f)\n",
		res.BestFitStructuralA2c, res.MeanStructuralA2c, res.SpreadStructuralA2c)
	fmt.Fprintf(&sb, "  → implied spacer @ declared fork %.0f mm:   %.2f mm   (declared %.1f)\n",
		res.DeclaredStaticForkA2c, res.ImpliedSpacerMm, res.DeclaredSpacerMm)
	fmt.Fprintf(&sb, "  → implied static fork A2C @ declared spacer %.1f mm: %.2f mm  (declared %.0f)\n",
		res.DeclaredSpacerMm, res.ImpliedStaticForkA2c, res.DeclaredStaticForkA2c)
	fmt.Fprintf(&sb, "Minimum passing slop: %s   (declared: %s)\n", strings.Join(req, ", "), strings.Join(decl, ", "))
	if declared.HasCustomSlop() {
		fmt.Fprintf(&sb, "\n[NOTE] Frame defines custom geo_slop_mm overrides: %s\n", declared.String())
		if declared.Reason != "" {
			fmt.Fprintf(&sb, "       Reason: %s\n", declared.Reason)
		}
	}
	badge := "PASS"
	if !res.IsValid {
		badge = "FAIL"
	}
	fmt.Fprintf(&sb, "ValidateFrame (declared values): %s", badge)
	if !res.IsValid && len(res.FrameReport.Issues) > 0 {
		fmt.Fprintf(&sb, "\n\nValidation Notices (%d):", len(res.FrameReport.Issues))
		for _, iss := range res.FrameReport.Issues {
			fmt.Fprintf(&sb, "\n  [%s] %s: %s", strings.ToUpper(string(iss.Severity)), iss.Title, iss.Message)
		}
	}
	doc.AddText(sb.String())

	if !res.IsValid || res.MinPassingSlopA2c > 1.0 {
		if cands, err := SolveFit(frame, configID, targetSize); err == nil && len(cands) > 0 {
			doc.AddTable(candidatesTable(cands))
		}
	}
	return doc, nil
}

func residualsTable(title string, res *FitResult) report.Table {
	headers := []string{"Size", "HTA", "Implied Struct A2C", "Δ vs Best", "FC pub", "FC calc", "ΔFC", "WB pub", "WB calc", "ΔWB"}
	var rows [][]string
	for _, r := range res.Residuals {
		fcPub, fcCalc, dFc := "—", "—", "—"
		if r.FcPub > 0 {
			fcPub, fcCalc = fmt.Sprintf("%.1f", r.FcPub), fmt.Sprintf("%.1f", r.FcCalc)
			if r.DeltaFc != nil {
				dFc = fmt.Sprintf("%+.1f", *r.DeltaFc)
			}
		}
		wbPub, wbCalc, dWb := "—", "—", "—"
		if r.WbPub > 0 && r.ChainstayLength > 0 {
			wbPub, wbCalc = fmt.Sprintf("%.1f", r.WbPub), fmt.Sprintf("%.1f", r.WbCalc)
			if r.DeltaWb != nil {
				dWb = fmt.Sprintf("%+.1f", *r.DeltaWb)
			}
		}
		rows = append(rows, []string{
			r.Size, fmt.Sprintf("%.1f°", r.HtaDeg), fmt.Sprintf("%.1f", r.ImpliedStructuralA2c),
			fmt.Sprintf("%+.2f", r.DeltaVsBest), fcPub, fcCalc, dFc, wbPub, wbCalc, dWb,
		})
	}
	return report.FromStrings("fit-residuals-"+report.Slug(title), title, "", headers, rows)
}
