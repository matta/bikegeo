package compare

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
	"bikegeo/pkg/scoring"
)

// DocOptions controls how a Result is laid out as a report.Document.
type DocOptions struct {
	Scoring bool // include scores, ranking and explanations
	Detail  bool // include the score breakdown table (requires Scoring)
	// Flat renders the transposed layout as one wide table (used for CSV)
	// instead of the split "Fit & Cockpit" / "Frame & Chassis" pair.
	Flat bool
	// DocFooters places footers on Document.Footers rather than attaching
	// them to the last table.
	DocFooters bool
}

type scoredRowItem struct {
	origIdx     int
	row         model.CanonicalComparisonRow
	score       scoring.Result
	displayName string
}

// Table IDs produced by BuildDocument.
const (
	TableFitCockpit = "fit-cockpit"
	TableChassis    = "chassis"
	TableMatrix     = "matrix"
	TableBreakdown  = "breakdown"
)

func zoneOf(cr scoring.CriterionResult) string {
	if cr.Missing {
		return report.ZoneMissing
	}
	return cr.Zone
}

// metricCell resolves the display cell for one attribute of one setup.
func metricCell(item scoredRowItem, attr ideal.AttributeSpec, meta ideal.AttributeMeta) report.Cell {
	for _, cr := range item.score.Criteria {
		if cr.ID == attr.ID {
			return report.Cell{Text: cr.FormattedCell, Zone: zoneOf(cr), MetricID: attr.ID}
		}
	}
	val, ok := meta.Extract(item.row)
	if !ok {
		return report.Cell{Text: "— (no data)", Zone: report.ZoneMissing, MetricID: attr.ID}
	}
	ev := ideal.Evaluate(attr, meta, val, &item.row)
	return report.Cell{Text: ev.FormattedCell, Zone: string(ev.Zone), MetricID: attr.ID}
}

func plain(s string) report.Cell { return report.Cell{Text: s} }

func dashIf(ok bool, s string) string {
	if ok {
		return s
	}
	return "—"
}

// BuildDocument lays out a resolved comparison as structured tables.
func BuildDocument(res *Result, o DocOptions) (report.Document, error) {
	rows := res.Rows
	state := res.State
	prof := res.Profile
	scoringEnabled := o.Scoring

	var doc report.Document
	if len(rows) == 0 {
		return doc, nil
	}

	if prof == nil {
		def, err := ideal.GetProfile("idmatch")
		if err != nil {
			return doc, err
		}
		prof = def
	}

	canonicalizer.ApplyIdealCockpitTargets(rows, prof)

	var scoreResults []scoring.Result
	if scoringEnabled {
		scoreResults = scoring.ScoreAllRows(rows, prof)
	}

	displayNames := DisambiguateFrameNames(rows)

	scoredItems := make([]scoredRowItem, len(rows))
	scoredFramesForExplain := make([]scoring.ScoredFrame, len(rows))
	for i, r := range rows {
		name := displayNames[i]
		sr := scoring.Result{}
		if scoringEnabled {
			sr = scoreResults[i]
		}
		scoredItems[i] = scoredRowItem{origIdx: i, row: r, score: sr, displayName: name}
		scoredFramesForExplain[i] = scoring.ScoredFrame{OriginalIndex: i, Name: name, Row: r, Score: sr}
	}

	var expl scoring.Explanations
	var footers []report.Footer
	missingNotes := scoring.CollectMissingDataNotes(scoredFramesForExplain)

	refDefLines := []string{
		"Frame & Chassis Specs: Reach, Stack, HTA, STA, BB Drop, and Trail reflect the dynamic, rotated frame attitude at the effective fork A2C and sag specified for each setup.",
		"Chainstay (*): Asterisk indicates adjustable/sliding dropouts; displayed at maximum stay position (longest wheelbase, most rearward axle). Front Wt %, Wheelbase, and Seat Axis–Rear Axle are calculated at this exact position.",
		"H: the saddle height entered for the comparison. It is used both as the length along the seat-tube axis (Seat Axis Point) and as the radial BB-to-BRP distance (Saddle BRP).",
		"Seat Axis Point: the point H mm from the BB center along the effective seat-tube axis (at-sag attitude). Frame-only: independent of saddle model, seatpost setback, rail position, stem and handlebar.",
		"Saddle BRP: the selected saddle's 70mm Biomechanical Reference Point, placed H mm (radial) from the BB using seatpost setback and rail position. Depends on saddle and seatpost.",
		"ETT: horizontal distance from the head-tube top center (Reach/Stack point) back to the seat-tube axis at stack height; the industry-standard effective top tube. Independent of H, saddle, seatpost, stem and bar.",
		"AETT: horizontal distance from the Seat Axis Point forward to the steering axis (ETT measured at H instead of at stack height). Independent of saddle, seatpost, stem and bar.",
		"Seat Axis Setback: horizontal distance from the BB center back to the Seat Axis Point (H * cos(STA)).",
		"Seat Axis–Rear Axle: horizontal distance from the Seat Axis Point to the rear axle (Rear Center minus Seat Axis Setback).",
		"Seat Axis–HT Drop: vertical drop from the Seat Axis Point to head tube top (H * sin(STA) - Stack; steerer spacer/stem rise needed for level bars).",
		"BRP Setback: horizontal distance from the BB center back to the Saddle BRP. BRP–HB: distance from the Saddle BRP to the handlebar reference point; HB Rise: handlebar height relative to the Saddle BRP (both depend on saddle, stem and bar).",
		"Seatpost Ext: Exposed seatpost length from collar to saddle rails (available vertical height for dropper post fitment).",
	}

	var adjustableStayLines []string
	seenAdjustable := make(map[string]bool)
	for _, item := range scoredItems {
		b := item.row.Bike
		if b.HasAdjustableChainstay() {
			key := fmt.Sprintf("%s %s (%.0f-%.0f)", b.Manufacturer, b.Model, b.GetMinChainstay(), b.GetMaxChainstay())
			if !seenAdjustable[key] {
				seenAdjustable[key] = true
				adjustableStayLines = append(adjustableStayLines, fmt.Sprintf("%s %s: %.0f–%.0fmm (max %.0fmm)", b.Manufacturer, b.Model, b.GetMinChainstay(), b.GetMaxChainstay(), b.GetMaxChainstay()))
			}
		}
	}
	if len(adjustableStayLines) > 0 {
		refDefLines = append(refDefLines, fmt.Sprintf("Sliding Dropout Ranges: %s.", strings.Join(adjustableStayLines, "; ")))
	}

	refDefFooter := report.Footer{Title: "Reference Metric Definitions", Lines: refDefLines}

	if scoringEnabled {
		expl = scoring.ExplainProfile(scoredFramesForExplain, prof)

		if strings.EqualFold(prof.Name, "idmatch") {
			saddle := state.Saddle
			if saddle.ID == "" {
				saddle = catalog.DefaultSaddle()
			}
			targetSitBones := model.DerivePelvicSetbackFromIdmatchBrp(219.0)
			targetBRP := saddle.TargetBrpForPelvicSetback(targetSitBones)
			footers = append(footers, report.Footer{
				Title: "Biomechanical Pelvic Fit & Seatpost Baseline",
				Lines: []string{
					fmt.Sprintf("Primary Pelvic Invariant: Sit Bone Setback is %.0fmm behind BB (derived from idmatch BRP 219mm + SQlab delta 73mm).", targetSitBones),
					"Seatpost Hardware Baseline: Hardcoded to Redshift ShockStop suspension post with 11mm sagged setback (7mm static + 4mm sag).",
					fmt.Sprintf("Active Saddle Target: %s has BRP %.0fmm behind BB (Delta BRP-to-SitBones = %.0fmm).", saddle.Name, targetBRP, saddle.DeltaBrpToSitBonesMm()),
				},
			})
		}

		footers = append(footers, refDefFooter)
		footers = append(footers, report.Footer{Title: "Biggest Losses vs. Ideal", Lines: expl.Losses})
		if len(rows) > 1 {
			footers = append(footers, report.Footer{Title: "Head-to-Head Deciding Factors", Lines: expl.HeadToHead})
		}
		if len(missingNotes) > 0 {
			footers = append(footers, report.Footer{Title: "Data Notes", Lines: missingNotes})
		}

		// Sort displayed rows/columns descending by score
		sort.SliceStable(scoredItems, func(i, j int) bool {
			return scoredItems[i].score.Total > scoredItems[j].score.Total
		})
	} else {
		footers = append(footers, refDefFooter)
		if len(missingNotes) > 0 {
			footers = append(footers, report.Footer{Title: "Data Notes", Lines: missingNotes})
		}
	}

	backAngle := rows[0].Canonical.BackAngleDeg
	title := "Apples-to-Apples Geometry Comparison Matrix"
	saddleName := state.Saddle.Name
	if saddleName == "" {
		saddleName = catalog.DefaultSaddle().Name
	}
	bar := state.Handlebar
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	baseBarName := bar.Name
	if idx := strings.Index(baseBarName, " ("); idx != -1 {
		baseBarName = baseBarName[:idx]
	}

	line1 := fmt.Sprintf(
		"H (saddle height): %.0fmm | Saddle: %s | Seatpost: Redshift ShockStop (sagged 11mm) | Modeled Back Angle: %.1f°",
		state.SaddleHeightMm, saddleName, backAngle,
	)
	line2 := fmt.Sprintf(
		"Handlebar: %s (%.0fmm, %.0fmm rise, %.0f° sweep, %.0f° upsweep) | Ideal Profile: %s (*max stay position)",
		baseBarName, bar.EffectiveWidthMm(), bar.StatedRiseMm, bar.BacksweepDeg, bar.UpsweepDeg, prof.Name,
	)
	subtitle := fmt.Sprintf("%s\n%s", line1, line2)

	// Footer placement: with a detail table in the printed layout footers
	// follow it; otherwise they follow the last primary table.
	footersOnDetail := o.Detail && scoringEnabled && !o.DocFooters
	var primaryFooters []report.Footer
	if o.DocFooters {
		doc.Footers = footers
	} else if !footersOnDetail {
		primaryFooters = footers
	}

	colAttrs := func(item scoredRowItem) map[string]string {
		return map[string]string{
			"col":      strconv.Itoa(item.origIdx + 1),
			"frame-id": item.row.Setup.FrameID,
			"size":     item.row.Setup.Size,
			"rank":     strconv.Itoa(item.score.Rank),
		}
	}

	if state.Transpose {
		if o.Flat {
			attrs := prof.Attributes
			t := report.Table{ID: TableMatrix, Title: title, Subtitle: subtitle, Footers: primaryFooters}
			t.Columns = append(t.Columns,
				report.Column{ID: "col", Label: "#"},
				report.Column{ID: "frame", Label: "Frame (Size)"})
			if scoringEnabled {
				t.Columns = append(t.Columns, report.Column{ID: "score", Label: "Score", Numeric: true})
			}
			t.Columns = append(t.Columns, report.Column{ID: "fork", Label: "Fork & Eff A2C"})
			for _, attr := range attrs {
				meta, _ := ideal.LookupAttribute(attr.ID)
				t.Columns = append(t.Columns, report.Column{ID: attr.ID, Label: meta.Label, Numeric: true})
			}
			t.Rows = transposedRows(t.Columns, attrs, state, scoredItems, scoringEnabled, colAttrs)
			doc.AddTable(t)
		} else {
			var fitAttrs []ideal.AttributeSpec
			for _, attr := range prof.Attributes {
				if attr.Ideal != nil || strings.EqualFold(attr.ID, "best_stem") {
					fitAttrs = append(fitAttrs, attr)
				}
			}
			if len(fitAttrs) == 0 {
				fitAttrs = prof.Attributes
			}

			t1 := report.Table{ID: TableFitCockpit, Title: title + " - Fit & Cockpit", Subtitle: subtitle}
			t1.Columns = append(t1.Columns,
				report.Column{ID: "col", Label: "#"},
				report.Column{ID: "frame", Label: "Frame (Size)"})
			if scoringEnabled {
				t1.Columns = append(t1.Columns, report.Column{ID: "score", Label: "Score", Numeric: true})
			}
			t1.Columns = append(t1.Columns, report.Column{ID: "fork", Label: "Fork & Eff A2C"})
			for _, attr := range fitAttrs {
				meta, _ := ideal.LookupAttribute(attr.ID)
				t1.Columns = append(t1.Columns, report.Column{ID: attr.ID, Label: meta.Label, Numeric: true})
			}
			t1.Rows = transposedRows(t1.Columns, fitAttrs, state, scoredItems, scoringEnabled, colAttrs)
			doc.AddTable(t1)

			t2 := report.Table{ID: TableChassis, Title: "Frame & Chassis Specifications (Dynamic / At-Sag Geometry)", Footers: primaryFooters}
			t2.Columns = []report.Column{
				{ID: "col", Label: "#"},
				{ID: "frame", Label: "Frame (Size)"},
				{ID: "reach", Label: "Reach", Numeric: true},
				{ID: "stack", Label: "Stack", Numeric: true},
				{ID: "hta", Label: "HTA", Numeric: true},
				{ID: "trail", Label: "Trail", Numeric: true},
				{ID: "sta", Label: "STA", Numeric: true},
				{ID: "ett", Label: "ETT", Numeric: true},
				{ID: "bb_drop", Label: "BB Drop", Numeric: true},
				{ID: "chainstay", Label: "Chainstay", Numeric: true},
				{ID: "seat_to_rc", Label: "Seat Axis–Rear Axle", Numeric: true},
				{ID: "sparevert", Label: "Seat Axis–HT Drop", Numeric: true},
			}
			csMeta, _ := ideal.LookupAttribute("chainstay")
			for _, item := range scoredItems {
				r := item.row
				c := r.Canonical
				csStr := "—"
				if val, ok := csMeta.Extract(r); ok {
					csStr = csMeta.FormatVal(val, &r)
				}
				cells := []report.Cell{
					plain(strconv.Itoa(item.origIdx + 1)),
					plain(item.displayName),
					plain(dashIf(c.ActiveReach > 0, fmt.Sprintf("%.0f", c.ActiveReach))),
					plain(dashIf(c.ActiveStack > 0, fmt.Sprintf("%.0f", c.ActiveStack))),
					plain(dashIf(c.ActiveHta > 0, fmt.Sprintf("%.1f°", c.ActiveHta))),
					plain(dashIf(c.TrailMm > 0, fmt.Sprintf("%.0f", c.TrailMm))),
					plain(dashIf(c.ActiveSta > 0, fmt.Sprintf("%.1f°", c.ActiveSta))),
					plain(dashIf(c.Ett > 0, fmt.Sprintf("%.0f", c.Ett))),
					plain(dashIf(c.ActiveBbDrop != 0, fmt.Sprintf("%.0f", c.ActiveBbDrop))),
					plain(csStr),
					plain(dashIf(c.SeatToRearCenter > 0, fmt.Sprintf("%.0f", c.SeatToRearCenter))),
					plain(dashIf(c.SpareVerticalStack != 0, fmt.Sprintf("%.0f", c.SpareVerticalStack))),
				}
				for i := range cells[2:] {
					cells[i+2].MetricID = t2.Columns[i+2].ID
				}
				t2.Rows = append(t2.Rows, report.Row{ID: strconv.Itoa(item.origIdx + 1), Kind: report.KindData, Attrs: colAttrs(item), Cells: cells})
			}
			doc.AddTable(t2)
		}
	} else {
		// Default orientation: Metrics as rows, Bike Setups as columns
		t := report.Table{ID: TableMatrix, Title: title, Subtitle: subtitle, Footers: primaryFooters}
		t.Columns = []report.Column{{ID: "metric", Label: "Metric"}, {ID: "ideal", Label: "Ideal"}, {ID: "acceptable", Label: "Acceptable"}}
		for _, item := range scoredItems {
			r := item.row
			forkLine := r.DisplayLabel
			if forkLine == "" {
				forkLine = fmt.Sprintf("%.0fmm A2C", math.Round(r.EffectiveA2c))
			}
			t.Columns = append(t.Columns, report.Column{
				ID:      "col-" + strconv.Itoa(item.origIdx+1),
				Label:   fmt.Sprintf("[%d] %s %s (%s)\n%s", item.origIdx+1, r.Bike.Manufacturer, r.Bike.Model, r.Bike.Size, forkLine),
				Numeric: true,
				Attrs:   colAttrs(item),
			})
		}

		addRow := func(id, metric, idealStr, accStr string, cellFn func(item scoredRowItem) report.Cell) {
			cells := []report.Cell{plain(metric), plain(idealStr), plain(accStr)}
			for _, item := range scoredItems {
				cells = append(cells, cellFn(item))
			}
			t.Rows = append(t.Rows, report.Row{ID: id, Kind: report.KindData, Attrs: map[string]string{"metric": id}, Cells: cells})
		}

		if scoringEnabled {
			addRow("score", "Score", "", "", func(item scoredRowItem) report.Cell {
				return plain(fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank))
			})
		}
		addRow("fork", "Fork & Eff. A2C", "", "", func(item scoredRowItem) report.Cell {
			return plain(canonicalizer.FormatForkFlow(item.row))
		})

		for _, attr := range prof.Attributes {
			evalAttr := AdaptProfileAttributeForSaddle(attr, state.Saddle)
			meta, _ := ideal.LookupAttribute(evalAttr.ID)
			label := meta.Label
			switch strings.ToLower(evalAttr.ID) {
			case "aett", "setback", "saddle_hb", "brp_setback", "saddle_hb_drop", "handlebar_clamp_fit", "seat_to_rc", "sparevert":
				label = fmt.Sprintf("%s @ %.0fmm", meta.Label, state.SaddleHeightMm)
			}
			idealStr, accStr := FormatAttrRanges(evalAttr, meta)
			addRow(evalAttr.ID, label, idealStr, accStr, func(item scoredRowItem) report.Cell {
				return metricCell(item, evalAttr, meta)
			})
		}
		doc.AddTable(t)
	}

	if o.Detail && scoringEnabled {
		doc.AddTable(buildBreakdown(prof, scoredItems, colAttrs, footers, footersOnDetail))
	}

	return doc, nil
}

// transposedRows builds the Ideal / Acceptable reference rows followed by one
// data row per setup, for tables whose metric columns follow the fixed prefix.
func transposedRows(
	cols []report.Column,
	attrs []ideal.AttributeSpec,
	state model.SavedComparisonState,
	items []scoredRowItem,
	scoringEnabled bool,
	rowAttrs func(scoredRowItem) map[string]string,
) []report.Row {
	offset := 3
	if scoringEnabled {
		offset = 4
	}

	idealRow := report.Row{ID: "ideal", Kind: report.KindIdeal, Cells: make([]report.Cell, len(cols))}
	accRow := report.Row{ID: "acceptable", Kind: report.KindAcceptable, Cells: make([]report.Cell, len(cols))}
	idealRow.Cells[1].Text = "Ideal"
	accRow.Cells[1].Text = "Acceptable"
	for idx, attr := range attrs {
		evalAttr := AdaptProfileAttributeForSaddle(attr, state.Saddle)
		meta, _ := ideal.LookupAttribute(evalAttr.ID)
		idealRow.Cells[offset+idx].Text, accRow.Cells[offset+idx].Text = FormatAttrRanges(evalAttr, meta)
	}
	out := []report.Row{idealRow, accRow}

	for _, item := range items {
		cells := []report.Cell{plain(strconv.Itoa(item.origIdx + 1)), plain(item.displayName)}
		if scoringEnabled {
			cells = append(cells, plain(fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank)))
		}
		cells = append(cells, plain(canonicalizer.FormatForkEffA2c(item.row)))
		for _, attr := range attrs {
			meta, _ := ideal.LookupAttribute(attr.ID)
			cells = append(cells, metricCell(item, attr, meta))
		}
		out = append(out, report.Row{ID: strconv.Itoa(item.origIdx + 1), Kind: report.KindData, Attrs: rowAttrs(item), Cells: cells})
	}
	return out
}

func buildBreakdown(
	prof *ideal.Profile,
	items []scoredRowItem,
	rowAttrs func(scoredRowItem) map[string]string,
	footers []report.Footer,
	withFooters bool,
) report.Table {
	t := report.Table{ID: TableBreakdown, Title: "Scoring Breakdown"}
	t.Columns = []report.Column{{ID: "rank", Label: "Rank"}, {ID: "col", Label: "#"}, {ID: "frame", Label: "Frame"}}
	var weightedAttrs []ideal.AttributeSpec
	var weightParts []string
	for _, attr := range prof.Attributes {
		if attr.Weight > 0 && attr.Ideal != nil {
			meta, _ := ideal.LookupAttribute(attr.ID)
			t.Columns = append(t.Columns, report.Column{ID: attr.ID, Label: meta.Label, Numeric: true})
			weightedAttrs = append(weightedAttrs, attr)
			weightParts = append(weightParts, fmt.Sprintf("%s %.0f", meta.Label, attr.Weight))
		}
	}
	t.Columns = append(t.Columns, report.Column{ID: "total", Label: "Total", Numeric: true})
	t.Subtitle = fmt.Sprintf("Sub-scores (-200..+100) per criterion (points lost on total score in parens). Total = weighted average (weights: %s)", strings.Join(weightParts, ", "))
	if withFooters {
		t.Footers = footers
	}

	for _, item := range items {
		subScores := make(map[string]float64)
		pointsLost := make(map[string]float64)
		missingMap := make(map[string]bool)
		zones := make(map[string]string)
		for _, c := range item.score.Criteria {
			subScores[c.ID] = c.SubScore
			pointsLost[c.ID] = c.PointsLost
			missingMap[c.ID] = c.Missing
			zones[c.ID] = c.Zone
		}

		formatSubScoreCell := func(id string) report.Cell {
			if missingMap[id] {
				return report.Cell{Text: "—", Zone: report.ZoneMissing, MetricID: id}
			}
			sub := math.Round(subScores[id])
			lost := math.Round(pointsLost[id])
			scorePart := fmt.Sprintf("%.0f", sub)
			if lost >= 1 {
				scorePart = fmt.Sprintf("%.0f (-%.0f)", sub, lost)
			}
			text := scorePart

			lowID := strings.ToLower(id)
			switch {
			case (lowID == "handlebar_clamp_fit" || lowID == "clamp_fit") && item.row.Canonical.CockpitFitResult != nil:
				cfr := item.row.Canonical.CockpitFitResult
				hb := cfr.SelectedHB
				sign := "+"
				if hb.SpacerMm < 0 {
					sign = ""
				}
				stemPenStr := ""
				if cfr.StemPenalty > 0 {
					stemPenStr = fmt.Sprintf(" [-%.0f stem]", cfr.StemPenalty)
				}
				text = fmt.Sprintf("%s [%.0fmm %.0f° %s%.0fmm]%s (%d in oval)", scorePart, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, stemPenStr, cfr.CountInside)
			case (lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback") && item.row.Canonical.SelectedBRP != nil:
				brp := item.row.Canonical.SelectedBRP
				sign := "+"
				if brp.RailOffsetMm < 0 {
					sign = ""
				}
				text = fmt.Sprintf("%s [rail %s%.0fmm]", scorePart, sign, brp.RailOffsetMm)
			case (lowID == "saddle_hb" || lowID == "saddlehb") && item.row.Canonical.SelectedHB != nil && item.row.Canonical.SelectedHB.Stem.LengthMm > 0:
				hb := item.row.Canonical.SelectedHB
				sign := "+"
				if hb.SpacerMm < 0 {
					sign = ""
				}
				text = fmt.Sprintf("%s [%.0fmm %.0f° %s%.0fmm]", scorePart, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm)
			case (lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop") && item.row.Canonical.SelectedHB != nil:
				drop := item.row.Canonical.SelectedHB.SaddleHbDropMm
				sign := ""
				if drop > 0 {
					sign = "+"
				}
				text = fmt.Sprintf("%s [%s%.0fmm]", scorePart, sign, drop)
			}
			return report.Cell{Text: text, Zone: zones[id], MetricID: id}
		}

		rndTotal := math.Round(item.score.Total)
		totalLost := math.Round(100.0 - item.score.Total)
		totalText := fmt.Sprintf("%.0f", rndTotal)
		if totalLost >= 1 {
			totalText = fmt.Sprintf("%.0f (-%.0f)", rndTotal, totalLost)
		}

		cells := []report.Cell{
			plain(strconv.Itoa(item.score.Rank)),
			plain(strconv.Itoa(item.origIdx + 1)),
			plain(item.displayName),
		}
		for _, wa := range weightedAttrs {
			cells = append(cells, formatSubScoreCell(wa.ID))
		}
		cells = append(cells, plain(totalText))
		t.Rows = append(t.Rows, report.Row{ID: strconv.Itoa(item.origIdx + 1), Kind: report.KindData, Attrs: rowAttrs(item), Cells: cells})
	}
	return t
}
