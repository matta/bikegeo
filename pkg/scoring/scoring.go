package scoring

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

// CriterionResult stores one band's line in the points-lost ledger.
type CriterionResult struct {
	ID              ideal.MetricID
	Label           string
	Unit            string
	Value           float64
	PointsLost      float64 // points of total lost on this band (0 inside its free range)
	Zone            string
	DistanceToIdeal float64
	FormattedCell   string
	Missing         bool
	IsDisplayOnly   bool
}

// Result holds the overall score, ranking, and per-criterion details for a frame setup.
// Total = 100 − Σ Criteria[i].PointsLost and may go below 0.
type Result struct {
	Total    float64
	Fits     bool // passes the fit gate; every fitting setup ranks above every non-fitting one
	Rank     int
	Criteria []CriterionResult
}

// FitBrpToleranceMm is how far the BRP may sit from the ideal setback and still fit.
const FitBrpToleranceMm = 10.0

// Fits reports the fit gate: some stock build puts the hands inside the oval
// (CountInside > 0, or R ≤ 1 for the selected build) and the BRP lands within
// FitBrpToleranceMm of the ideal setback. A condition whose band the profile
// does not carry is not enforced.
func Fits(row model.CanonicalComparisonRow, prof *ideal.Profile) bool {
	if prof == nil {
		prof = ideal.IdmatchProfile()
	}
	prof = prof.ForSaddle(row.Canonical.Saddle)
	if _, ok := prof.Spec(ideal.MetricHandlebarClampFit); ok {
		cfr := row.Canonical.CockpitFitResult
		if cfr == nil || (cfr.CountInside == 0 && cfr.BestR > 1.0) {
			return false
		}
	}
	if spec, ok := prof.Spec(ideal.MetricBrpSetback); ok && spec.Ideal != nil {
		brp := row.Canonical.SelectedBRP
		if brp == nil {
			return false
		}
		if lo, ok := spec.Ideal.Min.Get(); ok && brp.BrpSetbackMm < lo-FitBrpToleranceMm {
			return false
		}
		if hi, ok := spec.Ideal.Max.Get(); ok && brp.BrpSetbackMm > hi+FitBrpToleranceMm {
			return false
		}
	}
	return true
}

// Better reports whether a ranks strictly above b: a fitting setup beats a
// non-fitting one, and points of total order each group.
func Better(a, b Result) bool {
	if a.Fits != b.Fits {
		return a.Fits
	}
	return a.Total > b.Total
}

// ScoredFrame encapsulates a canonical comparison row together with its score Result and original setup index.
type ScoredFrame struct {
	OriginalIndex int // 0-indexed column position in state.Columns
	Name          string
	Row           model.CanonicalComparisonRow
	Score         Result
}

// Explanations holds plain-English explanatory sentences for frame scores.
type Explanations struct {
	Losses     []string
	HeadToHead []string
}

func formatValue(unit string, v float64) string {
	if unit == "°" || unit == "%" {
		return fmt.Sprintf("%.1f%s", v, unit)
	}
	return fmt.Sprintf("%.0f%s", v, unit)
}

// ScoreRow scores a single CanonicalComparisonRow against an ideal profile as a
// points-lost ledger: Total = 100 − Σ points lost, with each scored band free
// inside its Ideal range and priced linearly (AttributeSpec.Cost) outside it.
// Cockpit fit (SelectedBRP, SelectedHB, CockpitFitResult) must already be populated
// on row.Canonical (e.g., via canonicalizer.ApplyIdealCockpitTargets).
func ScoreRow(row model.CanonicalComparisonRow, prof *ideal.Profile) Result {
	if prof == nil {
		prof = ideal.IdmatchProfile()
	}
	prof = prof.ForSaddle(row.Canonical.Saddle)

	var results []CriterionResult
	totalLost := 0.0

	for _, spec := range prof.Attributes {
		meta, ok := ideal.LookupAttribute(spec.ID)
		if !ok {
			continue
		}

		val, ok := meta.Extract(row)
		if !ok {
			results = append(results, CriterionResult{
				ID:            spec.ID,
				Label:         meta.Label,
				Unit:          meta.Unit,
				Missing:       true,
				FormattedCell: "— (no data)",
				IsDisplayOnly: !spec.IsScored(),
			})
			continue
		}

		ev := ideal.Evaluate(spec, meta, val, &row)
		cr := CriterionResult{
			ID:              spec.ID,
			Label:           meta.Label,
			Unit:            meta.Unit,
			Value:           val,
			Zone:            string(ev.Zone),
			DistanceToIdeal: ev.DistanceToIdeal,
			FormattedCell:   ev.FormattedCell,
			IsDisplayOnly:   ev.IsDisplayOnly || !spec.IsScored(),
		}
		if !cr.IsDisplayOnly {
			cr.PointsLost = ev.PointsLost
			totalLost += ev.PointsLost
		}
		results = append(results, cr)
	}

	return Result{
		Total:    100.0 - totalLost,
		Fits:     Fits(row, prof),
		Criteria: results,
	}
}

// ScoreAllRows scores an array of CanonicalComparisonRow against an ideal profile and assigns ranks.
func ScoreAllRows(rows []model.CanonicalComparisonRow, prof *ideal.Profile) []Result {
	results := make([]Result, len(rows))
	order := make([]int, len(rows))

	for i, r := range rows {
		results[i] = ScoreRow(r, prof)
		order[i] = i
	}

	sort.SliceStable(order, func(i, j int) bool {
		return Better(results[order[i]], results[order[j]])
	})

	for rank, idx := range order {
		results[idx].Rank = rank + 1
	}

	return results
}

func formatCockpitOffset(hb model.HBPoint, oval model.FitOval, targetRef model.TargetRef, refBar model.Opt[model.HandlebarSpec]) string {
	if oval.CenterX == 0 && oval.CenterY == 0 {
		return ""
	}
	evalPt := geometry.EvaluatedPointForHB(hb, targetRef, refBar)
	dx := math.Round(evalPt.X - oval.CenterX)
	dy := math.Round(evalPt.Y - oval.CenterY)

	var reachStr string
	if dx < 0 {
		reachStr = fmt.Sprintf("reach: %.0fmm short", -dx)
	} else if dx > 0 {
		reachStr = fmt.Sprintf("reach: %.0fmm long", dx)
	} else {
		reachStr = "reach: 0mm"
	}

	var stackStr string
	if dy < 0 {
		stackStr = fmt.Sprintf("stack: %.0fmm low", -dy)
	} else if dy > 0 {
		stackStr = fmt.Sprintf("stack: %.0fmm high", dy)
	} else {
		stackStr = "stack: 0mm"
	}

	return fmt.Sprintf("%s, %s", reachStr, stackStr)
}

// DescribeLossWithRow generates plain-English explanation for why points were lost on an attribute,
// incorporating cockpit setup context (such as rail offset or stem/spacer configuration) if available.
func DescribeLossWithRow(spec ideal.AttributeSpec, meta ideal.AttributeMeta, val float64, dist float64, zone ideal.ZoneType, row *model.CanonicalComparisonRow) string {
	label := meta.Label

	if spec.ID == ideal.MetricBrpSetback && row != nil && row.Canonical.SelectedBRP != nil {
		brp := row.Canonical.SelectedBRP
		sign := "+"
		if brp.RailOffsetMm < 0 {
			sign = ""
		}
		label = fmt.Sprintf("%s (rail %s%.0fmm)", meta.Label, sign, brp.RailOffsetMm)
	} else if spec.ID == ideal.MetricSaddleHb && row != nil && row.Canonical.SelectedHB != nil && row.Canonical.SelectedHB.Stem.LengthMm > 0 {
		hb := row.Canonical.SelectedHB
		sign := "+"
		if hb.SpacerMm < 0 {
			sign = ""
		}
		label = fmt.Sprintf("%s (%.0fmm %.0f° %s%.0fmm)", meta.Label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm)
	}

	printedVal := ideal.RoundPrinted(val, meta.Decimals)
	printedDist := ideal.RoundPrinted(dist, meta.Decimals)
	if spec.Ideal != nil {
		printedDist = ideal.PrintedDistanceToIdeal(spec.Ideal, printedVal, meta.Decimals)
	}
	isBelow := printedDist < 0 || (printedDist == 0 && dist < 0)
	absDist := math.Abs(printedDist)

	switch spec.ID {
	case ideal.MetricHandlebarClampFit:
		if row != nil && row.Canonical.CockpitFitResult != nil {
			cfr := row.Canonical.CockpitFitResult
			hb := cfr.SelectedHB
			sign := "+"
			if hb.SpacerMm < 0 {
				sign = ""
			}
			offsetStr := formatCockpitOffset(hb, cfr.TargetOval, cfr.TargetRef, cfr.ReferenceBar)
			offsetPart := ""
			if offsetStr != "" && (cfr.CountInside == 0 || cfr.BestR > 0) {
				offsetPart = ", " + offsetStr
			}
			if cfr.CountInside == 0 {
				return fmt.Sprintf("%s: 0 valid setups in fit window (closest is %.0fmm %.0f° %s%.0fmm, R=%.2f%s)",
					label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, cfr.BestR, offsetPart)
			}
			plural := "s"
			if cfr.CountInside == 1 {
				plural = ""
			}
			return fmt.Sprintf("%s: closest stem is %.0fmm %.0f° %s%.0fmm (R=%.2f%s, %d valid setup%s in fit window)",
				label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, cfr.BestR, offsetPart, cfr.CountInside, plural)
		}
	case ideal.MetricHta, ideal.MetricSta:
		target := 68.0
		if isBelow {
			if spec.Ideal != nil {
				if minVal, ok := spec.Ideal.Min.Get(); ok {
					target = ideal.RoundPrinted(minVal, meta.Decimals)
				}
			}
			return fmt.Sprintf("%s %.1f° is %.1f° slacker than %.1f", label, printedVal, absDist, target)
		}
		if spec.Ideal != nil {
			if maxVal, ok := spec.Ideal.Max.Get(); ok {
				target = ideal.RoundPrinted(maxVal, meta.Decimals)
			}
		}
		return fmt.Sprintf("%s %.1f° is %.1f° steeper than %.1f", label, printedVal, absDist, target)

	case ideal.MetricTrail:
		if isBelow {
			target := 98.0
			if spec.Ideal != nil {
				if minVal, ok := spec.Ideal.Min.Get(); ok {
					target = ideal.RoundPrinted(minVal, meta.Decimals)
				}
			}
			return fmt.Sprintf("Trail %.0fmm is %.0fmm quicker than the %.0fmm ideal", printedVal, absDist, target)
		}
		target := 106.0
		if spec.Ideal != nil {
			if maxVal, ok := spec.Ideal.Max.Get(); ok {
				target = ideal.RoundPrinted(maxVal, meta.Decimals)
			}
		}
		return fmt.Sprintf("Trail %.0fmm is %.0fmm slower than the %.0fmm ideal", printedVal, absDist, target)

	case ideal.MetricBbDrop:
		if isBelow {
			return fmt.Sprintf("BB drop %.0fmm sits %.0fmm higher than ideal", printedVal, absDist)
		}
		return fmt.Sprintf("BB drop %.0fmm sits %.0fmm lower than ideal", printedVal, absDist)
	case ideal.MetricSetback, ideal.MetricEtt, ideal.MetricAett, ideal.MetricSpareVert, ideal.MetricSpareAxial,
		ideal.MetricSeatpostExtension, ideal.MetricSeatToRc, ideal.MetricChainstay,
		ideal.MetricReach, ideal.MetricStack, ideal.MetricWheelbase, ideal.MetricRcWbRatio,
		ideal.MetricSeatTube, ideal.MetricStandover, ideal.MetricBrpSetback,
		ideal.MetricSaddleHb, ideal.MetricSaddleHbDrop, ideal.MetricBestStem,
		ideal.MetricStemSpacerFit, ideal.MetricSitBoneSetback, ideal.MetricIdealSta,
		ideal.MetricRailOffset, ideal.MetricStemLength, ideal.MetricStemAngle, ideal.MetricSpacers:
	default:
	}

	valStr := formatValue(meta.Unit, printedVal)
	distStr := formatValue(meta.Unit, absDist)

	if isBelow {
		if spec.Ideal != nil {
			if minVal, ok := spec.Ideal.Min.Get(); ok {
				idealStr := formatValue(meta.Unit, ideal.RoundPrinted(minVal, meta.Decimals))
				if spec.ID == ideal.MetricSetback || spec.ID == ideal.MetricBrpSetback || spec.ID == ideal.MetricSitBoneSetback {
					return fmt.Sprintf("%s %s is %s forward of the %s ideal", label, valStr, distStr, idealStr)
				}
				return fmt.Sprintf("%s %s is %s short of the %s ideal", label, valStr, distStr, idealStr)
			}
		}
		return fmt.Sprintf("%s %s is %s short of ideal", label, valStr, distStr)
	}

	if spec.Ideal != nil {
		if maxVal, ok := spec.Ideal.Max.Get(); ok {
			idealStr := formatValue(meta.Unit, ideal.RoundPrinted(maxVal, meta.Decimals))
			if spec.ID == ideal.MetricSetback || spec.ID == ideal.MetricBrpSetback || spec.ID == ideal.MetricSitBoneSetback {
				return fmt.Sprintf("%s %s is %s aft of the %s ideal", label, valStr, distStr, idealStr)
			}
			return fmt.Sprintf("%s %s is %s past the %s ideal", label, valStr, distStr, idealStr)
		}
	}
	return fmt.Sprintf("%s %s is %s past ideal", label, valStr, distStr)
}

// ExplainProfile generates plain-English summary explanations for frames scored against an ideal profile.
func ExplainProfile(frames []ScoredFrame, prof *ideal.Profile) Explanations {
	if prof == nil {
		prof = ideal.IdmatchProfile()
	}

	// Sort frames by total score descending (matching rank)
	sorted := make([]ScoredFrame, len(frames))
	copy(sorted, frames)
	sort.SliceStable(sorted, func(i, j int) bool {
		return Better(sorted[i].Score, sorted[j].Score)
	})

	var lossesList []string
	for _, f := range sorted {
		type lossItem struct {
			pointsLost float64
			text       string
		}
		var losses []lossItem

		rowProf := prof.ForSaddle(f.Row.Canonical.Saddle)
		rowSpecMap := make(map[ideal.MetricID]ideal.AttributeSpec, len(rowProf.Attributes))
		for _, a := range rowProf.Attributes {
			rowSpecMap[a.ID] = a
		}

		for _, cr := range f.Score.Criteria {
			if cr.Missing || cr.IsDisplayOnly {
				continue
			}
			spec, ok := rowSpecMap[cr.ID]
			if !ok {
				continue
			}
			meta, ok := ideal.LookupAttribute(cr.ID)
			if !ok {
				continue
			}

			if math.Round(cr.PointsLost) >= 3.0 {
				desc := DescribeLossWithRow(spec, meta, cr.Value, cr.DistanceToIdeal, ideal.ZoneType(cr.Zone), &f.Row)
				losses = append(losses, lossItem{
					pointsLost: cr.PointsLost,
					text:       fmt.Sprintf("%s (-%.0f)", desc, math.Round(cr.PointsLost)),
				})
			}
		}

		sort.SliceStable(losses, func(i, j int) bool {
			return losses[i].pointsLost > losses[j].pointsLost
		})

		var textParts []string
		for i := 0; i < len(losses) && i < 2; i++ {
			textParts = append(textParts, losses[i].text)
		}

		body := "near ideal everywhere"
		if len(textParts) > 0 {
			body = strings.Join(textParts, "; ")
		}

		lossesList = append(lossesList, fmt.Sprintf("%s (%.0f): %s", f.Name, math.Round(f.Score.Total), body))
	}

	var h2hList []string
	for idx := 0; idx < len(sorted)-1; idx++ {
		fa := sorted[idx]
		fb := sorted[idx+1]

		taRound := math.Round(fa.Score.Total)
		tbRound := math.Round(fb.Score.Total)
		gap := int(taRound - tbRound)

		var head string
		if gap < 1 {
			head = fmt.Sprintf("%s (%.0f) ≈ %s (%.0f) — essentially tied", fa.Name, taRound, fb.Name, tbRound)
		} else {
			head = fmt.Sprintf("%s (%.0f) > %s (%.0f) by %d", fa.Name, taRound, fb.Name, tbRound, gap)
		}

		type diffItem struct {
			id    ideal.MetricID
			diff  float64
			label string
			unit  string
			valA  float64
			valB  float64
			rA    float64
			rB    float64
		}
		var pro []diffItem
		var con []diffItem
		var smallCount int

		critBMap := make(map[ideal.MetricID]CriterionResult)
		for _, c := range fb.Score.Criteria {
			critBMap[c.ID] = c
		}

		for _, ca := range fa.Score.Criteria {
			if ca.Missing || ca.IsDisplayOnly {
				continue
			}
			cb, ok := critBMap[ca.ID]
			if !ok || cb.Missing || cb.IsDisplayOnly {
				continue
			}

			// A gains what B lost beyond A on this band.
			diff := cb.PointsLost - ca.PointsLost
			item := diffItem{
				id:    ca.ID,
				diff:  diff,
				label: ca.Label,
				unit:  ca.Unit,
				valA:  ca.Value,
				valB:  cb.Value,
				rA:    ca.DistanceToIdeal,
				rB:    cb.DistanceToIdeal,
			}
			if diff >= 1.0 {
				pro = append(pro, item)
			} else if diff <= -1.0 {
				con = append(con, item)
			} else if math.Abs(diff) > 1e-6 {
				smallCount++
			}
		}

		sort.SliceStable(pro, func(i, j int) bool {
			return pro[i].diff > pro[j].diff
		})
		sort.SliceStable(con, func(i, j int) bool {
			return con[i].diff < con[j].diff
		})

		shownSum := 0
		formatH2H := func(d diffItem) string {
			roundedDiff := int(math.Round(d.diff))
			shownSum += roundedDiff
			// Oval Fits matrix cells show the setup count, but the band is priced
			// on the selected setup's oval radius R, so compare R.
			if d.id == ideal.MetricHandlebarClampFit {
				return fmt.Sprintf("%s R=%.2f vs R=%.2f (%+d)", d.label, d.rA, d.rB, roundedDiff)
			}
			return fmt.Sprintf("%s %s vs %s (%+d)",
				d.label, formatValue(d.unit, d.valA), formatValue(d.unit, d.valB), roundedDiff)
		}

		var proParts []string
		for _, item := range pro {
			proParts = append(proParts, formatH2H(item))
		}
		var conParts []string
		for _, item := range con {
			conParts = append(conParts, formatH2H(item))
		}

		rem := gap - shownSum
		var clauses []string
		if len(proParts) > 0 {
			clauses = append(clauses, strings.Join(proParts, "; "))
		}
		if len(conParts) > 0 {
			clauses = append(clauses, "offset by "+strings.Join(conParts, "; "))
		}
		if rem != 0 && (len(proParts) > 0 || len(conParts) > 0 || gap != 0) {
			if smallCount > 0 {
				clauses = append(clauses, fmt.Sprintf("smaller (<1 pt) factors (%+d)", rem))
			}
		}

		line := head
		if len(clauses) > 0 {
			line = head + ": " + strings.Join(clauses, "; ")
		}

		h2hList = append(h2hList, line)
	}

	return Explanations{
		Losses:     lossesList,
		HeadToHead: h2hList,
	}
}

// CollectMissingDataNotes collects human-readable footnotes for any attributes that were missing on some frames.
func CollectMissingDataNotes(frames []ScoredFrame) []string {
	missingMap := make(map[string][]string)
	var orderedLabels []string

	for _, f := range frames {
		for _, c := range f.Score.Criteria {
			if c.Missing {
				if len(missingMap[c.Label]) == 0 {
					orderedLabels = append(orderedLabels, c.Label)
				}
				missingMap[c.Label] = append(missingMap[c.Label], f.Name)
			}
		}
	}

	var notes []string
	for _, label := range orderedLabels {
		names := missingMap[label]
		notes = append(notes, fmt.Sprintf("%s not scored for: %s", label, strings.Join(names, ", ")))
	}
	return notes
}
