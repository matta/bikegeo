package ideals

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v3"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/tui"
)

// CurveInspector is a modular TUI widget that displays visual analysis,
// natural language explanations, and real database bike rankings for an AttributeSpec.
type CurveInspector struct {
	Spec        ideal.AttributeSpec
	Meta        ideal.AttributeMeta
	FleetRows   []model.CanonicalComparisonRow
	CompareRows []model.CanonicalComparisonRow
	ShowFleet   bool // true = Fleet (All sizes), false = Comparison set
	ProbeVal    *float64
	Theme       tui.Theme
}

// NewCurveInspector creates an initialized modular CurveInspector.
func NewCurveInspector(spec ideal.AttributeSpec, meta ideal.AttributeMeta, theme tui.Theme) *CurveInspector {
	return &CurveInspector{
		Spec:      spec,
		Meta:      meta,
		ShowFleet: true, // Default to Fleet (all sizes) as requested
		Theme:     theme,
	}
}

// SetSpec updates the active attribute spec and metadata.
func (ci *CurveInspector) SetSpec(spec ideal.AttributeSpec, meta ideal.AttributeMeta) {
	ci.Spec = spec
	ci.Meta = meta
}

// SetBikeRows sets the available fleet and comparison row datasets.
func (ci *CurveInspector) SetBikeRows(fleet, compare []model.CanonicalComparisonRow) {
	ci.FleetRows = fleet
	ci.CompareRows = compare
}

// SetProbeVal sets the active probe test value (or nil if cleared).
func (ci *CurveInspector) SetProbeVal(val *float64) {
	ci.ProbeVal = val
}

// ToggleDataset toggles between the entire fleet and the active comparison set.
func (ci *CurveInspector) ToggleDataset() {
	ci.ShowFleet = !ci.ShowFleet
}

// ActiveRows returns the currently active rows based on ShowFleet toggle.
func (ci *CurveInspector) ActiveRows() []model.CanonicalComparisonRow {
	if ci.ShowFleet {
		return ci.FleetRows
	}
	return ci.CompareRows
}

// HandleEvent processes events directed at the inspector.
func (ci *CurveInspector) HandleEvent(ev tcell.Event) tui.EventResult {
	return tui.EventIgnored
}

// ZoneSegment describes one region of the 1D segmented zone bar.
type ZoneSegment struct {
	Name      string
	RangeStr  string
	ScoreStr  string
	StyleType string // "floor", "penalty", "acceptable", "ideal"
}

// Draw renders the modular inspector within bounds.
func (ci *CurveInspector) Draw(s tcell.Screen, bounds tui.Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	tui.FillRect(s, bounds, ci.Theme.Default)

	// 1. Header Bar (Row 0)
	dsLabel := "Fleet"
	if !ci.ShowFleet {
		dsLabel = "Compare"
	}
	activeRows := ci.ActiveRows()
	dsTag := fmt.Sprintf("[%s: %d sizes] (F2)", dsLabel, len(activeRows))

	headerLeft := fmt.Sprintf("── Analysis: %s (%s) ", ci.Meta.Label, ci.Spec.ID)
	tui.DrawTextTruncated(s, bounds.X, bounds.Y, bounds.W, padRune(headerLeft, bounds.W), ci.Theme.Border)
	tagX := bounds.X + bounds.W - len([]rune(dsTag)) - 1
	if tagX > bounds.X+len([]rune(headerLeft)) {
		tui.DrawText(s, tagX, bounds.Y, dsTag, ci.Theme.Focus)
	}

	// If Display Only (no ideal range configured)
	if ci.Spec.Ideal == nil {
		tui.DrawText(s, bounds.X+2, bounds.Y+2, "[DISPLAY ONLY ATTRIBUTE]", ci.Theme.Header)
		tui.DrawText(s, bounds.X+2, bounds.Y+3, "No scoring curve or sweet-spot range defined.", ci.Theme.Muted)
		tui.DrawText(s, bounds.X+2, bounds.Y+4, "(Metric will be displayed in tables but not scored).", ci.Theme.Muted)
		ci.drawFramesSection(s, bounds, bounds.Y+6, activeRows, dsLabel)
		return
	}

	// 2. 1D Segmented Zone Bar (Rows 1..4)
	ci.drawZoneBar(s, bounds, bounds.Y+1)

	// 3. Value Prober (Rows 5..6)
	probeY := bounds.Y + 5
	probeHdr := padRune("── Value Probe ", bounds.W)
	tui.DrawTextTruncated(s, bounds.X, probeY, bounds.W, probeHdr, ci.Theme.Border)
	if ci.ProbeVal != nil {
		ev := ideal.Evaluate(ci.Spec, ci.Meta, *ci.ProbeVal, nil)
		valStr := ci.fmtNum(*ci.ProbeVal)
		probeLine := fmt.Sprintf("▶ Probe: %s%s ── Score: %.0f pts (%s %s)", valStr, ci.Meta.Unit, math.Round(ev.SubScore), ev.Zone, ev.FormattedCell)
		tui.DrawTextTruncated(s, bounds.X+1, probeY+1, bounds.W-2, probeLine, ci.Theme.Selected)
	} else {
		tui.DrawTextTruncated(s, bounds.X+1, probeY+1, bounds.W-2, "▶ Probe: enter test value in form to evaluate score and zone live", ci.Theme.Muted)
	}

	// 4. English Narrative Summary (Rows 7..11)
	narrativeY := probeY + 3
	narrativeHdr := padRune("── English Description ", bounds.W)
	tui.DrawTextTruncated(s, bounds.X, narrativeY, bounds.W, narrativeHdr, ci.Theme.Border)

	bullets := ci.generateEnglishSummary()
	for i, b := range bullets {
		if narrativeY+1+i >= bounds.Y+bounds.H {
			break
		}
		tui.DrawTextTruncated(s, bounds.X+1, narrativeY+1+i, bounds.W-2, b, ci.Theme.Default)
	}

	// 5. Real Database Frames Impact (Rows 12..end)
	framesY := narrativeY + 1 + len(bullets) + 1
	ci.drawFramesSection(s, bounds, framesY, activeRows, dsLabel)
}

func (ci *CurveInspector) drawZoneBar(s tcell.Screen, bounds tui.Rect, startY int) {
	segments := ci.buildZoneSegments()
	if len(segments) == 0 {
		return
	}

	n := len(segments)
	colW := (bounds.W - (n - 1)) / n
	if colW < 6 {
		colW = 6
	}

	for i, seg := range segments {
		x := bounds.X + i*(colW+1)
		if x >= bounds.X+bounds.W {
			break
		}
		w := colW
		if x+w > bounds.X+bounds.W {
			w = bounds.X + bounds.W - x
		}

		st := ci.Theme.Default
		switch seg.StyleType {
		case "ideal":
			st = ci.Theme.Success
		case "acceptable":
			st = ci.Theme.Focus
		case "penalty":
			st = ci.Theme.Warning
		case "floor":
			st = ci.Theme.Error
		}

		name := seg.Name
		if w < 10 {
			switch seg.StyleType {
			case "ideal":
				name = "IDEAL"
			case "acceptable":
				name = "ACCEPT"
			case "penalty":
				name = "PENALTY"
			case "floor":
				name = "FLOOR"
			}
			if w < 7 {
				switch seg.StyleType {
				case "ideal":
					name = "IDEAL"
				case "acceptable":
					name = "ACC"
				case "penalty":
					name = "PEN"
				case "floor":
					name = "FLR"
				}
			}
		}

		// Row 0: Segment Name
		tui.DrawTextTruncated(s, x, startY, w, centerString(name, w), st)
		// Row 1: Boundary Range
		tui.DrawTextTruncated(s, x, startY+1, w, centerString(seg.RangeStr, w), ci.Theme.Default)
		// Row 2: Score Range
		tui.DrawTextTruncated(s, x, startY+2, w, centerString(seg.ScoreStr, w), ci.Theme.Muted)

		// Divider
		if i < n-1 && x+w < bounds.X+bounds.W {
			tui.DrawText(s, x+w, startY, "│", ci.Theme.Border)
			tui.DrawText(s, x+w, startY+1, "│", ci.Theme.Border)
			tui.DrawText(s, x+w, startY+2, "│", ci.Theme.Border)
		}
	}
}

func (ci *CurveInspector) buildZoneSegments() []ZoneSegment {
	if ci.Spec.Ideal == nil {
		return nil
	}

	hasMin := ci.Spec.Ideal.Min != nil
	hasMax := ci.Spec.Ideal.Max != nil
	edgeScore := 75.0
	if ci.Spec.EdgeScore != nil {
		edgeScore = *ci.Spec.EdgeScore
	}
	floor := -200.0
	if ci.Spec.Floor != nil {
		floor = *ci.Spec.Floor
	}

	// Two-Sided Bounded Curve
	if hasMin && hasMax {
		idealMin := *ci.Spec.Ideal.Min
		idealMax := *ci.Spec.Ideal.Max
		accMin := idealMin
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Min != nil {
			accMin = *ci.Spec.Acceptable.Min
		}
		accMax := idealMax
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Max != nil {
			accMax = *ci.Spec.Acceptable.Max
		}

		fallBelow := 10.0
		if ci.Spec.Falloff != nil && ci.Spec.Falloff.Below != nil && *ci.Spec.Falloff.Below > 0 {
			fallBelow = *ci.Spec.Falloff.Below
		}
		fallAbove := 10.0
		if ci.Spec.Falloff != nil && ci.Spec.Falloff.Above != nil && *ci.Spec.Falloff.Above > 0 {
			fallAbove = *ci.Spec.Falloff.Above
		}

		zeroBelow := accMin - fallBelow
		zeroAbove := accMax + fallAbove

		idealRangeStr := fmt.Sprintf("%s–%s", ci.fmtNum(idealMin), ci.fmtNum(idealMax))
		if idealMin == idealMax {
			idealRangeStr = ci.fmtNum(idealMin)
		}

		var segs []ZoneSegment
		segs = append(segs, ZoneSegment{
			Name:      "[FLOOR]",
			RangeStr:  fmt.Sprintf("< %s", ci.fmtNum(zeroBelow)),
			ScoreStr:  fmt.Sprintf("%.0f", floor),
			StyleType: "floor",
		})
		segs = append(segs, ZoneSegment{
			Name:      "[PENALTY]",
			RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(zeroBelow), ci.fmtNum(accMin)),
			ScoreStr:  fmt.Sprintf("0–%.0f", edgeScore),
			StyleType: "penalty",
		})
		if accMin < idealMin {
			segs = append(segs, ZoneSegment{
				Name:      "[ACCEPT]",
				RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(accMin), ci.fmtNum(idealMin)),
				ScoreStr:  fmt.Sprintf("%.0f–100", edgeScore),
				StyleType: "acceptable",
			})
		}
		segs = append(segs, ZoneSegment{
			Name:      "[== IDEAL ==]",
			RangeStr:  idealRangeStr,
			ScoreStr:  "100",
			StyleType: "ideal",
		})
		if accMax > idealMax {
			segs = append(segs, ZoneSegment{
				Name:      "[ACCEPT]",
				RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(idealMax), ci.fmtNum(accMax)),
				ScoreStr:  fmt.Sprintf("100–%.0f", edgeScore),
				StyleType: "acceptable",
			})
		}
		segs = append(segs, ZoneSegment{
			Name:      "[PENALTY]",
			RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(accMax), ci.fmtNum(zeroAbove)),
			ScoreStr:  fmt.Sprintf("%.0f–0", edgeScore),
			StyleType: "penalty",
		})
		segs = append(segs, ZoneSegment{
			Name:      "[FLOOR]",
			RangeStr:  fmt.Sprintf("> %s", ci.fmtNum(zeroAbove)),
			ScoreStr:  fmt.Sprintf("%.0f", floor),
			StyleType: "floor",
		})
		return segs
	}

	// Lower-Only (More is Better, e.g. Chainstay >= 455)
	if hasMin && !hasMax {
		idealMin := *ci.Spec.Ideal.Min
		accMin := idealMin
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Min != nil {
			accMin = *ci.Spec.Acceptable.Min
		}
		fallBelow := 10.0
		if ci.Spec.Falloff != nil && ci.Spec.Falloff.Below != nil && *ci.Spec.Falloff.Below > 0 {
			fallBelow = *ci.Spec.Falloff.Below
		}
		zeroBelow := accMin - fallBelow

		var segs []ZoneSegment
		segs = append(segs, ZoneSegment{
			Name:      "[FLOOR]",
			RangeStr:  fmt.Sprintf("< %s", ci.fmtNum(zeroBelow)),
			ScoreStr:  fmt.Sprintf("%.0f", floor),
			StyleType: "floor",
		})
		segs = append(segs, ZoneSegment{
			Name:      "[PENALTY]",
			RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(zeroBelow), ci.fmtNum(accMin)),
			ScoreStr:  fmt.Sprintf("0–%.0f", edgeScore),
			StyleType: "penalty",
		})
		if accMin < idealMin {
			segs = append(segs, ZoneSegment{
				Name:      "[ACCEPTABLE]",
				RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(accMin), ci.fmtNum(idealMin)),
				ScoreStr:  fmt.Sprintf("%.0f–100", edgeScore),
				StyleType: "acceptable",
			})
		}
		segs = append(segs, ZoneSegment{
			Name:      "[==== IDEAL (UNBOUNDED) ====]",
			RangeStr:  fmt.Sprintf("≥ %s", ci.fmtNum(idealMin)),
			ScoreStr:  "100",
			StyleType: "ideal",
		})
		return segs
	}

	// Upper-Only (Less is Better)
	if !hasMin && hasMax {
		idealMax := *ci.Spec.Ideal.Max
		accMax := idealMax
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Max != nil {
			accMax = *ci.Spec.Acceptable.Max
		}
		fallAbove := 10.0
		if ci.Spec.Falloff != nil && ci.Spec.Falloff.Above != nil && *ci.Spec.Falloff.Above > 0 {
			fallAbove = *ci.Spec.Falloff.Above
		}
		zeroAbove := accMax + fallAbove

		var segs []ZoneSegment
		segs = append(segs, ZoneSegment{
			Name:      "[==== IDEAL (UNBOUNDED) ====]",
			RangeStr:  fmt.Sprintf("≤ %s", ci.fmtNum(idealMax)),
			ScoreStr:  "100",
			StyleType: "ideal",
		})
		if accMax > idealMax {
			segs = append(segs, ZoneSegment{
				Name:      "[ACCEPTABLE]",
				RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(idealMax), ci.fmtNum(accMax)),
				ScoreStr:  fmt.Sprintf("100–%.0f", edgeScore),
				StyleType: "acceptable",
			})
		}
		segs = append(segs, ZoneSegment{
			Name:      "[PENALTY]",
			RangeStr:  fmt.Sprintf("%s–%s", ci.fmtNum(accMax), ci.fmtNum(zeroAbove)),
			ScoreStr:  fmt.Sprintf("%.0f–0", edgeScore),
			StyleType: "penalty",
		})
		segs = append(segs, ZoneSegment{
			Name:      "[FLOOR]",
			RangeStr:  fmt.Sprintf("> %s", ci.fmtNum(zeroAbove)),
			ScoreStr:  fmt.Sprintf("%.0f", floor),
			StyleType: "floor",
		})
		return segs
	}

	return nil
}

func (ci *CurveInspector) generateEnglishSummary() []string {
	if ci.Spec.Ideal == nil {
		return []string{"• Display only: not evaluated or scored."}
	}

	hasMin := ci.Spec.Ideal.Min != nil
	hasMax := ci.Spec.Ideal.Max != nil
	edgeScore := 75.0
	if ci.Spec.EdgeScore != nil {
		edgeScore = *ci.Spec.EdgeScore
	}
	floor := -200.0
	if ci.Spec.Floor != nil {
		floor = *ci.Spec.Floor
	}

	var bullets []string

	// 1. Sweet spot
	if hasMin && hasMax {
		minI := *ci.Spec.Ideal.Min
		maxI := *ci.Spec.Ideal.Max
		if minI == maxI {
			bullets = append(bullets, fmt.Sprintf("• Sweet spot: %s %s receives full 100 pts.", ci.fmtNum(minI), ci.Meta.Unit))
		} else {
			bullets = append(bullets, fmt.Sprintf("• Sweet spot: %s–%s %s receives full 100 pts.", ci.fmtNum(minI), ci.fmtNum(maxI), ci.Meta.Unit))
		}
	} else if hasMin {
		bullets = append(bullets, fmt.Sprintf("• Sweet spot: ≥ %s %s receives full 100 pts.", ci.fmtNum(*ci.Spec.Ideal.Min), ci.Meta.Unit))
	} else if hasMax {
		bullets = append(bullets, fmt.Sprintf("• Sweet spot: ≤ %s %s receives full 100 pts.", ci.fmtNum(*ci.Spec.Ideal.Max), ci.Meta.Unit))
	}

	// 2. Acceptable window
	if hasMin && hasMax {
		minI := *ci.Spec.Ideal.Min
		maxI := *ci.Spec.Ideal.Max
		minA := minI
		maxA := maxI
		if ci.Spec.Acceptable != nil {
			if ci.Spec.Acceptable.Min != nil {
				minA = *ci.Spec.Acceptable.Min
			}
			if ci.Spec.Acceptable.Max != nil {
				maxA = *ci.Spec.Acceptable.Max
			}
		}
		if minA < minI || maxA > maxI {
			bullets = append(bullets, fmt.Sprintf("• Acceptable: %s–%s %s tapers from 100 down to %.0f pts.", ci.fmtNum(minA), ci.fmtNum(maxA), ci.Meta.Unit, edgeScore))
		} else {
			bullets = append(bullets, "• Acceptable: no margin outside sweet spot; drops straight to penalty.")
		}
	} else if hasMin {
		minI := *ci.Spec.Ideal.Min
		minA := minI
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Min != nil {
			minA = *ci.Spec.Acceptable.Min
		}
		if minA < minI {
			bullets = append(bullets, fmt.Sprintf("• Acceptable: %s–%s %s tapers from 100 down to %.0f pts.", ci.fmtNum(minA), ci.fmtNum(minI), ci.Meta.Unit, edgeScore))
		}
	} else if hasMax {
		maxI := *ci.Spec.Ideal.Max
		maxA := maxI
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Max != nil {
			maxA = *ci.Spec.Acceptable.Max
		}
		if maxA > maxI {
			bullets = append(bullets, fmt.Sprintf("• Acceptable: %s–%s %s tapers from 100 down to %.0f pts.", ci.fmtNum(maxI), ci.fmtNum(maxA), ci.Meta.Unit, edgeScore))
		}
	}

	// 3. Falloff / Penalty
	fallBelow := 10.0
	if ci.Spec.Falloff != nil && ci.Spec.Falloff.Below != nil && *ci.Spec.Falloff.Below > 0 {
		fallBelow = *ci.Spec.Falloff.Below
	}
	fallAbove := 10.0
	if ci.Spec.Falloff != nil && ci.Spec.Falloff.Above != nil && *ci.Spec.Falloff.Above > 0 {
		fallAbove = *ci.Spec.Falloff.Above
	}

	if hasMin && hasMax {
		minA := *ci.Spec.Ideal.Min
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Min != nil {
			minA = *ci.Spec.Acceptable.Min
		}
		maxA := *ci.Spec.Ideal.Max
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Max != nil {
			maxA = *ci.Spec.Acceptable.Max
		}
		rateB := edgeScore / fallBelow
		rateA := edgeScore / fallAbove
		zeroB := minA - fallBelow
		zeroA := maxA + fallAbove
		bullets = append(bullets, fmt.Sprintf("• Penalty: -%.1f pts/%s below %s (zero at %s); -%.1f pts/%s above %s (zero at %s).",
			rateB, ci.Meta.Unit, ci.fmtNum(minA), ci.fmtNum(zeroB),
			rateA, ci.Meta.Unit, ci.fmtNum(maxA), ci.fmtNum(zeroA),
		))
	} else if hasMin {
		minA := *ci.Spec.Ideal.Min
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Min != nil {
			minA = *ci.Spec.Acceptable.Min
		}
		rateB := edgeScore / fallBelow
		zeroB := minA - fallBelow
		bullets = append(bullets, fmt.Sprintf("• Penalty: -%.1f pts/%s below %s (zero at %s).", rateB, ci.Meta.Unit, ci.fmtNum(minA), ci.fmtNum(zeroB)))
	} else if hasMax {
		maxA := *ci.Spec.Ideal.Max
		if ci.Spec.Acceptable != nil && ci.Spec.Acceptable.Max != nil {
			maxA = *ci.Spec.Acceptable.Max
		}
		rateA := edgeScore / fallAbove
		zeroA := maxA + fallAbove
		bullets = append(bullets, fmt.Sprintf("• Penalty: -%.1f pts/%s above %s (zero at %s).", rateA, ci.Meta.Unit, ci.fmtNum(maxA), ci.fmtNum(zeroA)))
	}

	// 4. Floor clamp
	bullets = append(bullets, fmt.Sprintf("• Floor: score clamped at %.0f pts.", floor))

	return bullets
}

type bikeRanking struct {
	label string
	val   float64
	score float64
	zone  ideal.ZoneType
	dist  float64
}

func (ci *CurveInspector) drawFramesSection(s tcell.Screen, bounds tui.Rect, startY int, rows []model.CanonicalComparisonRow, dsLabel string) {
	if startY >= bounds.Y+bounds.H {
		return
	}

	framesHdr := padRune(fmt.Sprintf("── Real Frames (%s: %d sizes) ", dsLabel, len(rows)), bounds.W)
	tui.DrawTextTruncated(s, bounds.X, startY, bounds.W, framesHdr, ci.Theme.Border)

	if len(rows) == 0 {
		tui.DrawText(s, bounds.X+2, startY+1, "(no frame dataset loaded or available)", ci.Theme.Muted)
		return
	}

	var idealBikes []bikeRanking
	var edgeBikes []bikeRanking
	var penaltyBikes []bikeRanking
	var elimBikes []bikeRanking

	edgeScore := 75.0
	if ci.Spec.EdgeScore != nil {
		edgeScore = *ci.Spec.EdgeScore
	}

	for _, r := range rows {
		val, ok := ci.Meta.Extract(r)
		if !ok {
			continue
		}
		ev := ideal.Evaluate(ci.Spec, ci.Meta, val, &r)
		label := fmt.Sprintf("%s %s (%s)", r.Bike.Manufacturer, r.Bike.Model, r.Setup.Size)
		entry := bikeRanking{
			label: label,
			val:   val,
			score: ev.SubScore,
			zone:  ev.Zone,
			dist:  math.Abs(ev.DistanceToIdeal),
		}

		if ev.Zone == ideal.ZoneIdeal {
			idealBikes = append(idealBikes, entry)
		} else if ev.Zone == ideal.ZoneAcceptable {
			edgeBikes = append(edgeBikes, entry)
		} else if ev.SubScore > 0 {
			penaltyBikes = append(penaltyBikes, entry)
		} else {
			elimBikes = append(elimBikes, entry)
		}
	}

	// Sort categories
	sort.Slice(idealBikes, func(i, j int) bool {
		return idealBikes[i].dist < idealBikes[j].dist
	})
	// Edge: bikes closest to edge boundary
	sort.Slice(edgeBikes, func(i, j int) bool {
		return math.Abs(edgeBikes[i].score-edgeScore) < math.Abs(edgeBikes[j].score-edgeScore)
	})
	// Penalty: highest score first
	sort.Slice(penaltyBikes, func(i, j int) bool {
		return penaltyBikes[i].score > penaltyBikes[j].score
	})
	// Eliminated: most penalized first
	sort.Slice(elimBikes, func(i, j int) bool {
		return elimBikes[i].score < elimBikes[j].score
	})

	formatList := func(list []bikeRanking, showScore bool) string {
		if len(list) == 0 {
			return "(none)"
		}
		var parts []string
		limit := 3
		if len(list) < limit {
			limit = len(list)
		}
		for i := 0; i < limit; i++ {
			b := list[i]
			if showScore {
				parts = append(parts, fmt.Sprintf("%s: %s%s → %.0f", b.label, ci.fmtNum(b.val), ci.Meta.Unit, math.Round(b.score)))
			} else {
				parts = append(parts, fmt.Sprintf("%s: %s%s", b.label, ci.fmtNum(b.val), ci.Meta.Unit))
			}
		}
		return strings.Join(parts, ", ")
	}

	lines := []struct {
		prefix string
		text   string
		style  tcell.Style
	}{
		{
			prefix: " ✓ Sweet Spot  (100): ",
			text:   formatList(idealBikes, false),
			style:  ci.Theme.Success,
		},
		{
			prefix: fmt.Sprintf(" ~ Accept Edge (%.0f+): ", edgeScore),
			text:   formatList(edgeBikes, true),
			style:  ci.Theme.Focus,
		},
		{
			prefix: fmt.Sprintf(" ✗ Penalty     (<%.0f): ", edgeScore),
			text:   formatList(penaltyBikes, true),
			style:  ci.Theme.Warning,
		},
		{
			prefix: " ✗ Eliminated  (≤0) : ",
			text:   formatList(elimBikes, true),
			style:  ci.Theme.Error,
		},
	}

	for i, line := range lines {
		y := startY + 1 + i
		if y >= bounds.Y+bounds.H {
			break
		}
		prefLen := len([]rune(line.prefix))
		tui.DrawText(s, bounds.X, y, line.prefix, line.style)
		remW := bounds.W - prefLen
		if remW > 0 {
			tui.DrawTextTruncated(s, bounds.X+prefLen, y, remW, line.text, ci.Theme.Default)
		}
	}
}

func (ci *CurveInspector) fmtNum(v float64) string {
	if ci.Meta.Decimals == 0 {
		return fmt.Sprintf("%.0f", math.Round(v))
	}
	return fmt.Sprintf("%.*f", ci.Meta.Decimals, v)
}

func centerString(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	leftPad := (width - len(runes)) / 2
	rightPad := width - len(runes) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
