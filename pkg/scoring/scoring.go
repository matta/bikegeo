package scoring

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"bikegeo/pkg/model"
)

// Evaluator evaluates a value against a scoring curve to produce a sub-score (typically between -200 and 100).
type Evaluator interface {
	Eval(x float64) float64
}

// Point represents a (X, Y) coordinate on a piecewise-linear scoring curve.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Curve is a piecewise-linear function defined by a sequence of Points ordered by X.
// Outside the range [Points[0].X, Points[len-1].X], Eval clamps to the boundary Y values.
type Curve []Point

// Eval computes the interpolated Y value for the given X.
func (c Curve) Eval(x float64) float64 {
	if len(c) == 0 {
		return 0
	}
	if x <= c[0].X {
		return c[0].Y
	}
	if x >= c[len(c)-1].X {
		return c[len(c)-1].Y
	}
	for i := 0; i < len(c)-1; i++ {
		x0, y0 := c[i].X, c[i].Y
		x1, y1 := c[i+1].X, c[i+1].Y
		if x >= x0 && x <= x1 {
			if x1 == x0 {
				return y0
			}
			return y0 + (y1-y0)*(x-x0)/(x1-x0)
		}
	}
	return c[len(c)-1].Y
}

// QuadraticCurve models a plateau sweet spot [Low, High] with smooth quadratic roll-off outside the plateau.
// Within [Low, High], Eval returns Peak (default 100.0).
// When x < Low, y = Peak - 100.0 * ((Low - x) / WLow)^2.
// When x > High, y = Peak - 100.0 * ((x - High) / WHigh)^2.
// Values below Floor are clamped to Floor (default -200.0).
type QuadraticCurve struct {
	Low   float64 `json:"low"`
	High  float64 `json:"high"`
	WLow  float64 `json:"w_low"`
	WHigh float64 `json:"w_high"`
	Floor float64 `json:"floor"`
	Peak  float64 `json:"peak"`
}

// Eval computes the quadratic plateau score for the given X.
func (q QuadraticCurve) Eval(x float64) float64 {
	peak := q.Peak
	if peak == 0 {
		peak = 100.0
	}
	floor := q.Floor
	if floor == 0 {
		floor = -200.0
	}

	if x >= q.Low && x <= q.High {
		return peak
	}

	var y float64
	if x < q.Low {
		if q.WLow <= 0 {
			return floor
		}
		d := (q.Low - x) / q.WLow
		y = peak - 100.0*d*d
	} else {
		if q.WHigh <= 0 {
			return floor
		}
		d := (x - q.High) / q.WHigh
		y = peak - 100.0*d*d
	}

	if y < floor {
		return floor
	}
	return y
}

// Criterion defines a single scoring factor with an extraction function, weight, curve, and description generator.
type Criterion struct {
	ID       string
	Label    string
	Unit     string
	Weight   float64
	Metric   func(model.BikeFitAnalysis) float64
	Curve    Evaluator
	Describe func(v float64) string
}

// PreferredSpareVertMm is the ideal spare vertical stack (in mm) that yields a maximum score (100).
const PreferredSpareVertMm = 100.0

// DefaultCriteria returns the standard hard-coded scoring criteria agreed upon for bikegeo compare.
var DefaultCriteria = []Criterion{
	{
		ID:     "aett",
		Label:  "AETT",
		Unit:   "mm",
		Weight: 10,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.Aett },
		Curve: QuadraticCurve{
			Low:   625,
			High:  636,
			WLow:  41,
			WHigh: 24,
			Floor: -200,
			Peak:  100,
		},
		Describe: func(v float64) string {
			if v < 584 {
				return fmt.Sprintf("AETT %.0fmm is %.0fmm under the 584 minimum", v, 584-v)
			}
			if v < 625 {
				return fmt.Sprintf("AETT %.0fmm is %.0fmm short of the 625 sweet spot", v, 625-v)
			}
			if v <= 636 {
				return fmt.Sprintf("AETT %.0fmm is in the 625–636 sweet spot", v)
			}
			return fmt.Sprintf("AETT %.0fmm is %.0fmm past the 636 sweet spot", v, v-636)
		},
	},
	{
		ID:     "setback",
		Label:  "setback",
		Unit:   "mm",
		Weight: 6,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.ActualSetback },
		Curve: QuadraticCurve{
			Low:   220,
			High:  220,
			WLow:  15,
			WHigh: 15,
			Floor: -100,
			Peak:  100,
		},
		Describe: func(v float64) string {
			d := v - 220.0
			if math.Abs(d) < 1.5 {
				return fmt.Sprintf("setback %.0fmm is on target", v)
			}
			if d > 0 {
				return fmt.Sprintf("setback %.0fmm is %.0fmm over 220", v, d)
			}
			return fmt.Sprintf("setback %.0fmm is %.0fmm under 220", v, -d)
		},
	},
	{
		ID:     "sparevert",
		Label:  "spare vert",
		Unit:   "mm",
		Weight: 5,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.SpareVerticalStack },
		Curve: QuadraticCurve{
			Low:   PreferredSpareVertMm,
			High:  PreferredSpareVertMm,
			WLow:  15,
			WHigh: 35,
			Floor: -100,
			Peak:  100,
		},
		Describe: func(v float64) string {
			d := v - PreferredSpareVertMm
			if math.Abs(d) < 1.5 {
				return fmt.Sprintf("spare vert %.0fmm is on target", v)
			}
			if d > 0 {
				return fmt.Sprintf("spare vert %.0fmm is %.0fmm over %.0f", v, d, PreferredSpareVertMm)
			}
			return fmt.Sprintf("spare vert %.0fmm is %.0fmm under %.0f", v, -d, PreferredSpareVertMm)
		},
	},
	{
		ID:     "chainstay",
		Label:  "chainstay",
		Unit:   "mm",
		Weight: 4,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.ActiveChainstay },
		Curve: Curve{
			{X: 420.0 - 250.0/7.0, Y: -100},
			{X: 420, Y: 0},
			{X: 445, Y: 70},
			{X: 470, Y: 100},
		},
		Describe: func(v float64) string {
			return fmt.Sprintf("chainstay %.0fmm", v)
		},
	},
	{
		ID:     "bbdrop",
		Label:  "BB drop",
		Unit:   "mm",
		Weight: 3,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.ActiveBbDrop },
		Curve: QuadraticCurve{
			Low:   70,
			High:  70,
			WLow:  20,
			WHigh: 20,
			Floor: -100,
			Peak:  100,
		},
		Describe: func(v float64) string {
			d := v - 70.0
			if math.Abs(d) < 1.5 {
				return fmt.Sprintf("BB drop %.0fmm is on target", v)
			}
			if d < 0 {
				return fmt.Sprintf("BB drop %.0fmm sits %.0fmm higher than ideal", v, -d)
			}
			return fmt.Sprintf("BB drop %.0fmm sits %.0fmm lower than ideal", v, d)
		},
	},
	{
		ID:     "hta",
		Label:  "HTA",
		Unit:   "°",
		Weight: 2,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.ActiveHta },
		Curve: QuadraticCurve{
			Low:   68,
			High:  68,
			WLow:  1.0,
			WHigh: 1.0,
			Floor: -200,
			Peak:  100,
		},
		Describe: func(v float64) string {
			d := v - 68.0
			if math.Abs(d) < 0.25 {
				return fmt.Sprintf("HTA %.1f° is on target", v)
			}
			if d > 0 {
				return fmt.Sprintf("HTA %.1f° is %.1f° steeper than 68", v, d)
			}
			return fmt.Sprintf("HTA %.1f° is %.1f° slacker than 68", v, -d)
		},
	},
	{
		ID:     "frontwt",
		Label:  "front weight",
		Unit:   "%",
		Weight: 1,
		Metric: func(a model.BikeFitAnalysis) float64 { return a.WeightDistFrontPct },
		Curve: Curve{
			{X: 28, Y: -100},
			{X: 33, Y: 0},
			{X: 38, Y: 100},
		},
		Describe: func(v float64) string {
			return fmt.Sprintf("front weight %.1f%%", v)
		},
	},
}

// CriterionResult stores the calculated score breakdown for a single criterion.
type CriterionResult struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Unit     string  `json:"unit"`
	Value    float64 `json:"value"`
	SubScore float64 `json:"sub_score"`
	Weight   float64 `json:"weight"`
	Points   float64 `json:"points"`
}

// Result holds the overall score, ranking, and per-criterion details for a frame setup.
type Result struct {
	Total    float64           `json:"total"`
	Rank     int               `json:"rank"`
	Criteria []CriterionResult `json:"criteria"`
}

// Score calculates the Result for a single BikeFitAnalysis against given criteria.
func Score(analysis model.BikeFitAnalysis, criteria []Criterion) Result {
	if len(criteria) == 0 {
		criteria = DefaultCriteria
	}

	totalWeight := 0.0
	for _, c := range criteria {
		totalWeight += c.Weight
	}
	if totalWeight <= 0 {
		totalWeight = 1.0
	}

	var results []CriterionResult
	totalScore := 0.0

	for _, c := range criteria {
		val := c.Metric(analysis)
		sub := c.Curve.Eval(val)
		pts := c.Weight * sub / totalWeight
		totalScore += pts

		results = append(results, CriterionResult{
			ID:       c.ID,
			Label:    c.Label,
			Unit:     c.Unit,
			Value:    val,
			SubScore: sub,
			Weight:   c.Weight,
			Points:   pts,
		})
	}

	return Result{
		Total:    totalScore,
		Criteria: results,
	}
}

// ScoreAll scores an array of BikeFitAnalysis instances and assigns Ranks (1-indexed, descending by Total).
// Ties preserve stable ordering based on their original position.
func ScoreAll(analyses []model.BikeFitAnalysis, criteria []Criterion) []Result {
	if len(criteria) == 0 {
		criteria = DefaultCriteria
	}

	results := make([]Result, len(analyses))
	type item struct {
		index int
		total float64
	}
	items := make([]item, len(analyses))

	for i, a := range analyses {
		res := Score(a, criteria)
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

// ScoredFrame encapsulates a canonical comparison row together with its score Result and original setup index.
type ScoredFrame struct {
	OriginalIndex int // 0-indexed column position in state.Columns
	Name          string
	Row           model.CanonicalComparisonRow
	Score         Result
}

// Explanations holds plain-English explanatory sentences for frame scores.
type Explanations struct {
	Losses     []string `json:"losses"`
	HeadToHead []string `json:"head_to_head"`
}

func formatValue(unit string, v float64) string {
	if unit == "°" || unit == "%" {
		return fmt.Sprintf("%.1f%s", v, unit)
	}
	return fmt.Sprintf("%.0f%s", v, unit)
}

// Explain generates plain-English summary explanations:
// 1. Biggest losses vs ideal (per frame in rank order)
// 2. Head-to-head deciding factors between adjacent ranked pairs
func Explain(frames []ScoredFrame, criteria []Criterion) Explanations {
	if len(criteria) == 0 {
		criteria = DefaultCriteria
	}

	totalWeight := 0.0
	for _, c := range criteria {
		totalWeight += c.Weight
	}
	if totalWeight <= 0 {
		totalWeight = 1.0
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

		for i, c := range criteria {
			sub := f.Score.Criteria[i].SubScore
			val := f.Score.Criteria[i].Value
			lost := c.Weight * (100.0 - sub) / totalWeight
			if math.Round(lost) >= 3.0 && c.Describe != nil {
				desc := c.Describe(val)
				losses = append(losses, lossItem{
					pointsLost: lost,
					text:       fmt.Sprintf("%s (-%.0f)", desc, math.Round(lost)),
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
			diff  float64
			label string
			unit  string
			valA  float64
			valB  float64
		}
		var pro []diffItem
		var con []diffItem

		for i, c := range criteria {
			subA := fa.Score.Criteria[i].SubScore
			subB := fb.Score.Criteria[i].SubScore
			contrib := c.Weight * (subA - subB) / totalWeight
			item := diffItem{
				diff:  contrib,
				label: c.Label,
				unit:  c.Unit,
				valA:  fa.Score.Criteria[i].Value,
				valB:  fb.Score.Criteria[i].Value,
			}
			if contrib >= 1.0 {
				pro = append(pro, item)
			} else if contrib <= -1.0 {
				con = append(con, item)
			}
		}

		sort.SliceStable(pro, func(i, j int) bool {
			return pro[i].diff > pro[j].diff
		})
		sort.SliceStable(con, func(i, j int) bool {
			return con[i].diff < con[j].diff // most negative first
		})

		var whyParts []string
		for i := 0; i < len(pro) && i < 2; i++ {
			p := pro[i]
			whyParts = append(whyParts, fmt.Sprintf("%s %s vs %s (+%.0f)",
				p.label, formatValue(p.unit, p.valA), formatValue(p.unit, p.valB), math.Round(p.diff)))
		}

		line := head
		if len(whyParts) > 0 {
			line = head + ": " + strings.Join(whyParts, "; ")
		}

		if len(con) > 0 {
			c := con[0]
			line += fmt.Sprintf("; offset by %s %s vs %s (%.0f)",
				c.label, formatValue(c.unit, c.valA), formatValue(c.unit, c.valB), math.Round(c.diff))
		}

		h2hList = append(h2hList, line)
	}

	return Explanations{
		Losses:     lossesList,
		HeadToHead: h2hList,
	}
}
