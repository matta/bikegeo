package scoring

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

// CriterionResult stores the calculated score breakdown for a single criterion.
type CriterionResult struct {
	ID              string
	Label           string
	Unit            string
	Value           float64
	SubScore        float64
	Weight          float64
	Points          float64
	PointsLost      float64
	Zone            string
	DistanceToIdeal float64
	FormattedCell   string
	Missing         bool
	IsDisplayOnly   bool
}

// Result holds the overall score, ranking, and per-criterion details for a frame setup.
type Result struct {
	Total    float64
	Rank     int
	Criteria []CriterionResult
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

func translateRange(r *ideal.Range, fn func(float64) float64) *ideal.Range {
	if r == nil {
		return nil
	}
	return &ideal.Range{
		Min: r.Min.Map(fn),
		Max: r.Max.Map(fn),
	}
}

func translateAttributeSpecForSaddle(spec ideal.AttributeSpec, saddle model.SaddleSpec) ideal.AttributeSpec {
	if saddle.ID == "" || saddle.ID == "sqlab-610-ergolux" {
		return spec
	}
	refSQ := model.ReferenceIdmatchSaddle()
	lowID := strings.ToLower(spec.ID)
	if lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" {
		fn := func(v float64) float64 {
			return saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(v))
		}
		copied := spec
		copied.Ideal = translateRange(spec.Ideal, fn)
		copied.Acceptable = translateRange(spec.Acceptable, fn)
		return copied
	}
	if lowID == "saddle_hb" || lowID == "saddlehb" {
		fn := func(v float64) float64 {
			tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, v, 3.0)
			return math.Round(tDist*10) / 10
		}
		copied := spec
		copied.Ideal = translateRange(spec.Ideal, fn)
		copied.Acceptable = translateRange(spec.Acceptable, fn)
		return copied
	}
	return spec
}

// ScoreRow scores a single CanonicalComparisonRow against an ideal profile.
func ScoreRow(row model.CanonicalComparisonRow, prof *ideal.Profile) Result {
	if prof == nil {
		prof = ideal.IdmatchProfile()
	}

	type attrEval struct {
		spec   ideal.AttributeSpec
		meta   ideal.AttributeMeta
		eval   ideal.Evaluation
		val    float64
		hasVal bool
	}

	var evals []attrEval
	totalActiveWeight := 0.0

	var brpMin, brpMax model.Opt[float64]
	var distMin, distMax model.Opt[float64]
	var dropMin, dropMax model.Opt[float64]
	var hasClampFit bool

	for _, spec := range prof.Attributes {
		lowID := strings.ToLower(spec.ID)
		if lowID == "handlebar_clamp_fit" || lowID == "cockpit_fit" || lowID == "clamp_fit" {
			hasClampFit = true
		} else if (lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback") && spec.Ideal != nil {
			brpMin = spec.Ideal.Min
			brpMax = spec.Ideal.Max
		} else if (lowID == "saddle_hb" || lowID == "saddlehb") && spec.Ideal != nil {
			distMin = spec.Ideal.Min
			distMax = spec.Ideal.Max
		} else if (lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop") && spec.Ideal != nil {
			dropMin = spec.Ideal.Min
			dropMax = spec.Ideal.Max
		}
	}

	can := &row.Canonical

	rowBrpMin, rowBrpMax := brpMin, brpMax
	rowDistMin, rowDistMax := distMin, distMax
	rowDropMin, rowDropMax := dropMin, dropMax
	if can.Saddle.ID != "" && can.Saddle.ID != "sqlab-610-ergolux" {
		refSQ := model.ReferenceIdmatchSaddle()
		rowBrpMin = brpMin.Map(func(v float64) float64 {
			return can.Saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(v))
		})
		rowBrpMax = brpMax.Map(func(v float64) float64 {
			return can.Saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(v))
		})
		refRise := 3.0
		if dMin, ok := dropMin.Get(); ok {
			if dMax, ok := dropMax.Get(); ok {
				refRise = (dMin + dMax) / 2.0
			} else {
				refRise = dMin
			}
		}
		rowDistMin = distMin.Map(func(v float64) float64 {
			return can.Saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, v, refRise)
		})
		rowDistMax = distMax.Map(func(v float64) float64 {
			return can.Saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, v, refRise)
		})
	}

	if len(can.PossibleBRPs) > 0 {
		if bestBRP, ok := geometry.SelectBestBRP(can.PossibleBRPs, rowBrpMin, rowBrpMax); ok {
			can.SelectedBRP = &bestBRP
			can.SitBoneSetbackMm = bestBRP.SitBoneSetbackMm
			can.AchievedClampOffset = bestBRP.AchievedClampOffsetMm
		}
		activeBRP := can.DeadCenterBRP
		if can.SelectedBRP != nil {
			activeBRP = *can.SelectedBRP
		}
		bar := can.Handlebar
		if bar.ID == "" {
			bar = catalog.DefaultHandlebar()
		}
		can.PossibleHBs = geometry.MapPossibleHBsWithHandlebar(row.Bike, can.ActiveReach, can.ActiveStack, can.ActiveHta, activeBRP, bar, model.DefaultGripInsetMm, 0.0)
	}
	if len(can.PossibleHBs) > 0 {
		if hasClampFit && (rowDistMin.IsSet() || rowDistMax.IsSet()) {
			geometry.ApplyCockpitFit(can, row.Bike, row.Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax)
		} else if bestHB, ok := geometry.SelectBestHBWithDrop(can.PossibleHBs, row.Bike.GetCanonicalStem(), rowDistMin, rowDistMax, rowDropMin, rowDropMax); ok {
			can.SelectedHB = &bestHB
			can.SaddleHbMm = bestHB.SaddleHbMm
			can.SaddleHbDropMm = bestHB.SaddleHbDropMm
		}
	}

	for _, spec := range prof.Attributes {
		meta, ok := ideal.LookupAttribute(spec.ID)
		if !ok {
			continue
		}

		evalSpec := spec
		if can.Saddle.ID != "" {
			evalSpec = translateAttributeSpecForSaddle(spec, can.Saddle)
		}

		val, ok := meta.Extract(row)
		if !ok {
			evals = append(evals, attrEval{spec: evalSpec, meta: meta, hasVal: false})
			continue
		}

		ev := ideal.Evaluate(evalSpec, meta, val, &row)
		evals = append(evals, attrEval{spec: evalSpec, meta: meta, eval: ev, val: val, hasVal: true})

		if evalSpec.Weight > 0 && evalSpec.Ideal != nil {
			totalActiveWeight += evalSpec.Weight
		}
	}

	if totalActiveWeight <= 0 {
		totalActiveWeight = 1.0
	}

	var results []CriterionResult
	totalScore := 0.0

	for _, item := range evals {
		if !item.hasVal {
			results = append(results, CriterionResult{
				ID:            item.spec.ID,
				Label:         item.meta.Label,
				Unit:          item.meta.Unit,
				Missing:       true,
				FormattedCell: "— (no data)",
				Weight:        item.spec.Weight,
			})
			continue
		}

		if item.eval.IsDisplayOnly || item.spec.Weight <= 0 {
			results = append(results, CriterionResult{
				ID:              item.spec.ID,
				Label:           item.meta.Label,
				Unit:            item.meta.Unit,
				Value:           item.val,
				Zone:            string(item.eval.Zone),
				DistanceToIdeal: item.eval.DistanceToIdeal,
				IsDisplayOnly:   true,
				FormattedCell:   item.eval.FormattedCell,
			})
			continue
		}

		sub := item.eval.SubScore
		pts := item.spec.Weight * sub / totalActiveWeight
		totalScore += pts

		lost := item.spec.Weight * (100.0 - sub) / totalActiveWeight
		if lost < 0 {
			lost = 0
		}

		results = append(results, CriterionResult{
			ID:              item.spec.ID,
			Label:           item.meta.Label,
			Unit:            item.meta.Unit,
			Value:           item.val,
			SubScore:        sub,
			Weight:          item.spec.Weight,
			Points:          pts,
			PointsLost:      lost,
			Zone:            string(item.eval.Zone),
			DistanceToIdeal: item.eval.DistanceToIdeal,
			FormattedCell:   item.eval.FormattedCell,
		})
	}

	return Result{
		Total:    totalScore,
		Criteria: results,
	}
}

// ScoreAllRows scores an array of CanonicalComparisonRow against an ideal profile and assigns ranks.
func ScoreAllRows(rows []model.CanonicalComparisonRow, prof *ideal.Profile) []Result {
	results := make([]Result, len(rows))
	type item struct {
		index int
		total float64
	}
	items := make([]item, len(rows))

	for i, r := range rows {
		res := ScoreRow(r, prof)
		results[i] = res
		items[i] = item{index: i, total: res.Total}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].total > items[j].total
	})

	for rank, it := range items {
		results[it.index].Rank = rank + 1
	}

	return results
}

// DescribeLoss generates plain-English explanation for why points were lost on an attribute.
func DescribeLoss(spec ideal.AttributeSpec, meta ideal.AttributeMeta, val float64, dist float64, zone ideal.ZoneType) string {
	return DescribeLossWithRow(spec, meta, val, dist, zone, nil)
}

func formatCockpitOffset(hb model.HBPoint, oval model.FitOval) string {
	if oval.CenterX == 0 && oval.CenterY == 0 {
		return ""
	}
	dx := math.Round(hb.HbX - oval.CenterX)
	dy := math.Round(hb.HbY - oval.CenterY)

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
	lowID := strings.ToLower(spec.ID)

	if (lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback") && row != nil && row.Canonical.SelectedBRP != nil {
		brp := row.Canonical.SelectedBRP
		sign := "+"
		if brp.RailOffsetMm < 0 {
			sign = ""
		}
		label = fmt.Sprintf("%s (rail %s%.0fmm)", meta.Label, sign, brp.RailOffsetMm)
	} else if (lowID == "saddle_hb" || lowID == "saddlehb") && row != nil && row.Canonical.SelectedHB != nil && row.Canonical.SelectedHB.Stem.LengthMm > 0 {
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

	switch lowID {
	case "handlebar_clamp_fit":
		if row != nil && row.Canonical.CockpitFitResult != nil {
			cfr := row.Canonical.CockpitFitResult
			hb := cfr.SelectedHB
			sign := "+"
			if hb.SpacerMm < 0 {
				sign = ""
			}
			stemNote := ""
			if cfr.StemPenalty > 0 {
				stemNote = fmt.Sprintf(", -%.0f stem penalty", cfr.StemPenalty)
			}
			offsetStr := formatCockpitOffset(hb, cfr.TargetOval)
			offsetPart := ""
			if offsetStr != "" && (cfr.CountInside == 0 || cfr.BestR > 0) {
				offsetPart = ", " + offsetStr
			}
			if cfr.CountInside == 0 {
				return fmt.Sprintf("%s: 0 valid setups in fit window (closest is %.0fmm %.0f° %s%.0fmm, R=%.2f%s%s)",
					label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, cfr.BestR, offsetPart, stemNote)
			}
			plural := "s"
			if cfr.CountInside == 1 {
				plural = ""
			}
			if cfr.VersatilityBonus > 0 {
				return fmt.Sprintf("%s: closest stem is %.0fmm %.0f° %s%.0fmm (R=%.2f%s, %d valid setup%s%s, +%.0f bonus)",
					label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, cfr.BestR, offsetPart, cfr.CountInside, plural, stemNote, cfr.VersatilityBonus)
			}
			return fmt.Sprintf("%s: closest stem is %.0fmm %.0f° %s%.0fmm (R=%.2f%s, %d valid setup%s in fit window%s)",
				label, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, cfr.BestR, offsetPart, cfr.CountInside, plural, stemNote)
		}
	case "hta", "sta":
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

	case "trail":
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

	case "bbdrop":
		if isBelow {
			return fmt.Sprintf("BB drop %.0fmm sits %.0fmm higher than ideal", printedVal, absDist)
		}
		return fmt.Sprintf("BB drop %.0fmm sits %.0fmm lower than ideal", printedVal, absDist)
	}

	valStr := formatValue(meta.Unit, printedVal)
	distStr := formatValue(meta.Unit, absDist)

	if isBelow {
		if spec.Ideal != nil {
			if minVal, ok := spec.Ideal.Min.Get(); ok {
				idealStr := formatValue(meta.Unit, ideal.RoundPrinted(minVal, meta.Decimals))
				if lowID == "setback" || lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" || lowID == "sit_bone_setback" {
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
			if lowID == "setback" || lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" || lowID == "sit_bone_setback" {
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

	attrSpecMap := make(map[string]ideal.AttributeSpec)
	for _, a := range prof.Attributes {
		attrSpecMap[strings.ToLower(a.ID)] = a
	}

	// Sort frames by total score descending (matching rank)
	sorted := make([]ScoredFrame, len(frames))
	copy(sorted, frames)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Score.Total > sorted[j].Score.Total
	})

	var lossesList []string
	for _, f := range sorted {
		type lossItem struct {
			pointsLost float64
			text       string
		}
		var losses []lossItem

		for _, cr := range f.Score.Criteria {
			if cr.Missing || cr.IsDisplayOnly || cr.Weight <= 0 {
				continue
			}
			spec, ok := attrSpecMap[strings.ToLower(cr.ID)]
			if !ok {
				continue
			}
			if f.Row.Canonical.Saddle.ID != "" {
				spec = translateAttributeSpecForSaddle(spec, f.Row.Canonical.Saddle)
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
			id    string
			diff  float64
			label string
			unit  string
			valA  float64
			valB  float64
		}
		var pro []diffItem
		var con []diffItem
		var smallCount int

		critBMap := make(map[string]CriterionResult)
		for _, c := range fb.Score.Criteria {
			critBMap[c.ID] = c
		}

		for _, ca := range fa.Score.Criteria {
			if ca.Missing || ca.IsDisplayOnly || ca.Weight <= 0 {
				continue
			}
			cb, ok := critBMap[ca.ID]
			if !ok || cb.Missing || cb.IsDisplayOnly || cb.Weight <= 0 {
				continue
			}

			diff := ca.Points - cb.Points
			item := diffItem{
				id:    ca.ID,
				diff:  diff,
				label: ca.Label,
				unit:  ca.Unit,
				valA:  ca.Value,
				valB:  cb.Value,
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
			// Oval Fits matrix cells show setup count, while CriterionResult.Value
			// stores the sub-score; label it explicitly so the two aren't conflated.
			valKind := ""
			if strings.EqualFold(d.id, "handlebar_clamp_fit") {
				valKind = "sub-score "
			}
			roundedDiff := int(math.Round(d.diff))
			shownSum += roundedDiff
			return fmt.Sprintf("%s %s%s vs %s (%+d)",
				d.label, valKind, formatValue(d.unit, d.valA), formatValue(d.unit, d.valB), roundedDiff)
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
			} else {
				clauses = append(clauses, fmt.Sprintf("rounding (%+d)", rem))
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
