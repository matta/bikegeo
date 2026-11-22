package assembly

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"bikegeo/internal/reboot/pose"
)

// ChartPrecision holds the rounding half-width of each published chart column
// the consistency checks read: a length printed as 642 could be anything in
// 642 ± 0.5 before the manufacturer rounded it. Zero means the column is exact
// and gets no term. The CAD fork and CAD lower cup are hypotheses, not rounded
// chart numbers, so they have no half-width (Q44).
type ChartPrecision struct {
	ReachMm          float64
	StackMm          float64
	HeadTubeAngleDeg pose.Deg
	HeadTubeLengthMm float64
	BbDropMm         float64
	ChainstayMm      float64
	WheelbaseMm      float64
}

// ChartAllowance widens one size's consistency checks beyond what rounding of
// its published columns explains (Q65, Q71). CheckChartConsistency rejects an
// allowance on a check that passes on rounding alone, or one more than
// chartAllowanceSlackMm beyond that check's excess (Q93). Reason documents
// why the chart is inconsistent and is required whenever a field is non-zero.
type ChartAllowance struct {
	AxleHeightMm float64
	WheelbaseMm  float64
	Reason       string
}

// chartAllowanceSlackMm is how far an allowance may exceed the excess it
// covers (legacy's tolerance), so allowances can be round numbers.
const chartAllowanceSlackMm = 0.5

// consistencyEpsilonMm absorbs floating-point noise: a term no larger than this is
// dropped, and a mismatch may exceed its budget by this much and still pass.
const consistencyEpsilonMm = 1e-9

// consistencyCheckNames names the two mismatches chartMismatches returns, in order.
var consistencyCheckNames = [2]string{"axle height", "wheelbase"}

// chartColumns lists the published columns the consistency checks read, in chart
// order, with how to read each column's half-width and how to move it.
var chartColumns = []struct {
	name      string
	halfWidth func(ChartPrecision) float64
	nudge     func(*FrameChart, float64)
}{
	{"Reach", func(p ChartPrecision) float64 { return p.ReachMm }, func(c *FrameChart, d float64) { c.Frame.ReachMm += d }},
	{"Stack", func(p ChartPrecision) float64 { return p.StackMm }, func(c *FrameChart, d float64) { c.Frame.StackMm += d }},
	{"HTA", func(p ChartPrecision) float64 { return float64(p.HeadTubeAngleDeg) }, func(c *FrameChart, d float64) { c.Frame.HeadTubeAngleDeg += pose.Deg(d) }},
	{"HTL", func(p ChartPrecision) float64 { return p.HeadTubeLengthMm }, func(c *FrameChart, d float64) { c.Frame.HeadTubeLengthMm += d }},
	{"BB drop", func(p ChartPrecision) float64 { return p.BbDropMm }, func(c *FrameChart, d float64) { c.Frame.BbDropMm += d }},
	{"Chainstay", func(p ChartPrecision) float64 { return p.ChainstayMm }, func(c *FrameChart, d float64) { c.Frame.ChainstayMm += d }},
	{"Wheelbase", func(p ChartPrecision) float64 { return p.WheelbaseMm }, func(c *FrameChart, d float64) { c.PublishedWheelbaseMm += d }},
}

// CheckChartConsistency checks that the chart's 0° mismatches with its CAD
// baseline fork are no larger than rounding of the published columns can
// explain (Q45). Each column's term is the larger change in a mismatch when the
// column moves by ± its half-width and the unpitched frameset recipe is rerun;
// a check's rounding budget is the sum of its terms. A documented allowance can
// widen a check if it is necessary and close to the measured excess (Q93).
// A published wheelbase of zero skips the wheelbase check and its column's
// budget contribution (Q57). Errors from failing checks are joined, axle height first.
func CheckChartConsistency(chart FrameChart) error {
	if err := validateFrameGeometry(chart.Frame); err != nil {
		return err
	}
	if err := validateForkGeometry(chart.CadFork); err != nil {
		return err
	}
	if err := validateHeadset(chart.Frame.LowerBore, chart.CadLowerCup, SteererCAD); err != nil {
		return err
	}
	if !isFinite(chart.PublishedWheelbaseMm) || chart.PublishedWheelbaseMm < 0 {
		return fmt.Errorf("published wheelbase must be finite and non-negative (0 = unpublished), got %v mm", chart.PublishedWheelbaseMm)
	}
	allowances := [2]float64{chart.Allowance.AxleHeightMm, chart.Allowance.WheelbaseMm}
	for i, name := range consistencyCheckNames {
		if allowance := allowances[i]; !isFinite(allowance) || allowance < 0 {
			return fmt.Errorf("chart allowance: %s must be finite and non-negative, got %v mm", name, allowance)
		}
	}
	hasAllowance := chart.Allowance.AxleHeightMm > 0 || chart.Allowance.WheelbaseMm > 0
	hasReason := strings.TrimSpace(chart.Allowance.Reason) != ""
	if hasAllowance && !hasReason {
		return errors.New("chart allowance: a non-zero allowance needs a reason")
	}
	if !hasAllowance && hasReason {
		return errors.New("chart allowance: a reason without an allowance")
	}
	checkNames, budgetColumns := consistencyCheckNames[:], chartColumns
	if chart.PublishedWheelbaseMm == 0 {
		if chart.Allowance.WheelbaseMm > 0 {
			return errors.New("chart allowance: wheelbase allowance on a chart with no published wheelbase")
		}
		// Wheelbase is the final check and chart column.
		checkNames = checkNames[:1]
		budgetColumns = budgetColumns[:len(budgetColumns)-1]
	}
	for _, col := range chartColumns {
		if hw := col.halfWidth(chart.Precision); !isFinite(hw) || hw < 0 {
			return fmt.Errorf("chart precision: %s half-width must be finite and non-negative, got %v", col.name, hw)
		}
	}

	mismatches := chartMismatches(chart)
	var budgets [2]float64
	var terms [2][]string
	for _, col := range budgetColumns {
		hw := col.halfWidth(chart.Precision)
		if hw == 0 {
			continue
		}
		up, down := chart, chart
		col.nudge(&up, hw)
		col.nudge(&down, -hw)
		for _, moved := range []FrameChart{up, down} {
			if err := validateFrameGeometry(moved.Frame); err != nil {
				return fmt.Errorf("chart precision: %s ±%v moves the chart out of range: %w", col.name, hw, err)
			}
		}
		mu, md := chartMismatches(up), chartMismatches(down)
		for i := range checkNames {
			term := math.Max(math.Abs(mu[i]-mismatches[i]), math.Abs(md[i]-mismatches[i]))
			if term <= consistencyEpsilonMm {
				continue
			}
			budgets[i] += term
			terms[i] = append(terms[i], fmt.Sprintf("%s %.2f mm", col.name, term))
		}
	}

	var errs []error
	for i, name := range checkNames {
		mismatch, budget, allowance := mismatches[i], budgets[i], allowances[i]
		magnitude := math.Abs(mismatch)
		excess := magnitude - budget
		if allowance > 0 && magnitude <= budget+consistencyEpsilonMm {
			errs = append(errs, fmt.Errorf("chart allowance: %s allowance %.2f mm is unnecessary: mismatch %+.2f mm is within its ±%.2f mm rounding budget", name, allowance, mismatch, budget))
			continue
		}
		if allowance > 0 && allowance > excess+chartAllowanceSlackMm+consistencyEpsilonMm {
			errs = append(errs, fmt.Errorf("chart allowance: %s allowance %.2f mm exceeds the %.2f mm needed beyond rounding by more than %.2f mm", name, allowance, excess, chartAllowanceSlackMm))
			continue
		}
		if magnitude <= budget+allowance+consistencyEpsilonMm {
			continue
		}
		listed := "no rounded columns"
		if len(terms[i]) > 0 {
			listed = strings.Join(terms[i], ", ")
		}
		budgetLabel := "rounding budget"
		if allowance > 0 {
			budgetLabel += fmt.Sprintf(" plus its %.2f mm allowance", allowance)
		}
		errs = append(errs, fmt.Errorf("chart consistency: %s mismatch %+.2f mm exceeds its ±%.2f mm %s (%s)", name, mismatch, budget, budgetLabel, listed))
	}
	return errors.Join(errs...)
}

// chartMismatches returns the chart's 0°-pitch mismatches with its CAD
// baseline fork: FrontAxle.Y − RearAxle.Y, then the axle X span minus the
// published wheelbase.
func chartMismatches(chart FrameChart) [2]float64 {
	unpitched, _ := measureUnpitchedFrameset(chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork)
	return [2]float64{
		unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle),
		(unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - chart.PublishedWheelbaseMm,
	}
}
