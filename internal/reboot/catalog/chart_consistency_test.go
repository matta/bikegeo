package catalog_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

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
	halfWidth func(assembly.ChartPrecision) float64
	nudge     func(*assembly.FrameChart, float64)
}{
	{"Reach", func(p assembly.ChartPrecision) float64 { return p.ReachMm }, func(c *assembly.FrameChart, d float64) { c.Frame.ReachMm += d }},
	{"Stack", func(p assembly.ChartPrecision) float64 { return p.StackMm }, func(c *assembly.FrameChart, d float64) { c.Frame.StackMm += d }},
	{"HTA", func(p assembly.ChartPrecision) float64 { return float64(p.HeadTubeAngleDeg) }, func(c *assembly.FrameChart, d float64) { c.Frame.HeadTubeAngleDeg += pose.Deg(d) }},
	{"HTL", func(p assembly.ChartPrecision) float64 { return p.HeadTubeLengthMm }, func(c *assembly.FrameChart, d float64) { c.Frame.HeadTubeLengthMm += d }},
	{"BB drop", func(p assembly.ChartPrecision) float64 { return p.BbDropMm }, func(c *assembly.FrameChart, d float64) { c.Frame.BbDropMm += d }},
	{"Chainstay", func(p assembly.ChartPrecision) float64 { return p.ChainstayMm }, func(c *assembly.FrameChart, d float64) { c.Frame.ChainstayMm += d }},
	{"Wheelbase", func(p assembly.ChartPrecision) float64 { return p.WheelbaseMm }, func(c *assembly.FrameChart, d float64) { c.PublishedWheelbaseMm += d }},
}

// roundingTerm is one published column's contribution to a check's rounding budget.
type roundingTerm struct {
	Column string // "Reach", "Stack", "HTA", "HTL", "BB drop", "Chainstay", or "Wheelbase"
	Mm     float64
}

// chartCheck is one chart-consistency check of one size (Q45, Q57, Q93).
type chartCheck struct {
	Name        string         // "axle height" or "wheelbase"
	MismatchMm  float64        // axle height: front axle rise above the rear axle; wheelbase: axle span minus the published wheelbase
	Terms       []roundingTerm // columns whose term exceeds consistencyEpsilonMm, in chart column order
	BudgetMm    float64        // sum of Terms
	AllowanceMm float64        // the size's documented allowance for this check (0 = none)
	ExcessMm    float64        // |MismatchMm| − BudgetMm; ≤ 0 when rounding explains the mismatch
}

// chartConsistency is one size's measurement: the axle height check, then the
// wheelbase check unless the wheelbase is unpublished (Q57).
type chartConsistency struct{ Checks []chartCheck }

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// chartMismatches returns the chart's 0°-pitch mismatches with its basis fork:
// FrontAxle.Y − RearAxle.Y, then the axle X span minus the published wheelbase.
func chartMismatches(unpitched assembly.FramesetPlacement, publishedWheelbaseMm float64) [2]float64 {
	return [2]float64{
		unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle),
		(unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - publishedWheelbaseMm,
	}
}

// measureChartConsistency places the chart's unpitched basis frameset and each
// ±half-width column nudge, and returns every check. Its error reports input
// the check cannot measure.
func measureChartConsistency(chart assembly.FrameChart) (chartConsistency, error) {
	unpitched, err := assembly.PlaceUnpitchedFrameset(chart.Frame, chart.CadLowerCup, basisForkOf(chart), wheels)
	if err != nil {
		return chartConsistency{}, err
	}
	if !isFinite(chart.PublishedWheelbaseMm) || chart.PublishedWheelbaseMm < 0 {
		return chartConsistency{}, fmt.Errorf("published wheelbase must be finite and non-negative (0 = unpublished), got %v mm", chart.PublishedWheelbaseMm)
	}
	allowances := [2]float64{chart.Allowance.AxleHeightMm, chart.Allowance.WheelbaseMm}
	for i, name := range consistencyCheckNames {
		if allowance := allowances[i]; !isFinite(allowance) || allowance < 0 {
			return chartConsistency{}, fmt.Errorf("chart allowance: %s must be finite and non-negative, got %v mm", name, allowance)
		}
	}
	hasAllowance := chart.Allowance.AxleHeightMm > 0 || chart.Allowance.WheelbaseMm > 0
	hasReason := strings.TrimSpace(chart.Allowance.Reason) != ""
	if hasAllowance && !hasReason {
		return chartConsistency{}, errors.New("chart allowance: a non-zero allowance needs a reason")
	}
	if !hasAllowance && hasReason {
		return chartConsistency{}, errors.New("chart allowance: a reason without an allowance")
	}
	checkNames, budgetColumns := consistencyCheckNames[:], chartColumns
	if chart.PublishedWheelbaseMm == 0 {
		if chart.Allowance.WheelbaseMm > 0 {
			return chartConsistency{}, errors.New("chart allowance: wheelbase allowance on a chart with no published wheelbase")
		}
		// Wheelbase is the final check and chart column.
		checkNames = checkNames[:1]
		budgetColumns = budgetColumns[:len(budgetColumns)-1]
	}
	for _, col := range chartColumns {
		if hw := col.halfWidth(chart.Precision); !isFinite(hw) || hw < 0 {
			return chartConsistency{}, fmt.Errorf("chart precision: %s half-width must be finite and non-negative, got %v", col.name, hw)
		}
	}

	mismatches := chartMismatches(unpitched, chart.PublishedWheelbaseMm)
	var budgets [2]float64
	var terms [2][]roundingTerm
	for _, col := range budgetColumns {
		hw := col.halfWidth(chart.Precision)
		if hw == 0 {
			continue
		}
		up, down := chart, chart
		col.nudge(&up, hw)
		col.nudge(&down, -hw)
		upPlaced, err := assembly.PlaceUnpitchedFrameset(up.Frame, up.CadLowerCup, basisForkOf(up), wheels)
		if err != nil {
			return chartConsistency{}, fmt.Errorf("chart precision: %s ±%v moves the chart out of range: %w", col.name, hw, err)
		}
		downPlaced, err := assembly.PlaceUnpitchedFrameset(down.Frame, down.CadLowerCup, basisForkOf(down), wheels)
		if err != nil {
			return chartConsistency{}, fmt.Errorf("chart precision: %s ±%v moves the chart out of range: %w", col.name, hw, err)
		}
		mu := chartMismatches(upPlaced, up.PublishedWheelbaseMm)
		md := chartMismatches(downPlaced, down.PublishedWheelbaseMm)
		for i := range checkNames {
			term := math.Max(math.Abs(mu[i]-mismatches[i]), math.Abs(md[i]-mismatches[i]))
			if term <= consistencyEpsilonMm {
				continue
			}
			budgets[i] += term
			terms[i] = append(terms[i], roundingTerm{Column: col.name, Mm: term})
		}
	}

	checks := make([]chartCheck, len(checkNames))
	for i, name := range checkNames {
		checks[i] = chartCheck{
			Name:        name,
			MismatchMm:  mismatches[i],
			Terms:       terms[i],
			BudgetMm:    budgets[i],
			AllowanceMm: allowances[i],
			ExcessMm:    math.Abs(mismatches[i]) - budgets[i],
		}
	}
	return chartConsistency{Checks: checks}, nil
}

// Err applies the pass/fail and allowance rules to the measured checks, in
// order, joined, using today's messages verbatim.
func (c chartConsistency) Err() error {
	var errs []error
	for _, chk := range c.Checks {
		magnitude := math.Abs(chk.MismatchMm)
		if chk.AllowanceMm > 0 && magnitude <= chk.BudgetMm+consistencyEpsilonMm {
			errs = append(errs, fmt.Errorf("chart allowance: %s allowance %.2f mm is unnecessary: mismatch %+.2f mm is within its ±%.2f mm rounding budget", chk.Name, chk.AllowanceMm, chk.MismatchMm, chk.BudgetMm))
			continue
		}
		if chk.AllowanceMm > 0 && chk.AllowanceMm > chk.ExcessMm+chartAllowanceSlackMm+consistencyEpsilonMm {
			errs = append(errs, fmt.Errorf("chart allowance: %s allowance %.2f mm exceeds the %.2f mm needed beyond rounding by more than %.2f mm", chk.Name, chk.AllowanceMm, chk.ExcessMm, chartAllowanceSlackMm))
			continue
		}
		if magnitude <= chk.BudgetMm+chk.AllowanceMm+consistencyEpsilonMm {
			continue
		}
		listed := "no rounded columns"
		if len(chk.Terms) > 0 {
			parts := make([]string, len(chk.Terms))
			for i, term := range chk.Terms {
				parts[i] = fmt.Sprintf("%s %.2f mm", term.Column, term.Mm)
			}
			listed = strings.Join(parts, ", ")
		}
		budgetLabel := "rounding budget"
		if chk.AllowanceMm > 0 {
			budgetLabel += fmt.Sprintf(" plus its %.2f mm allowance", chk.AllowanceMm)
		}
		errs = append(errs, fmt.Errorf("chart consistency: %s mismatch %+.2f mm exceeds its ±%.2f mm %s (%s)", chk.Name, chk.MismatchMm, chk.BudgetMm, budgetLabel, listed))
	}
	return errors.Join(errs...)
}

// checkChartConsistency returns measureChartConsistency's error, else Err().
func checkChartConsistency(chart assembly.FrameChart) error {
	m, err := measureChartConsistency(chart)
	if err != nil {
		return err
	}
	return m.Err()
}

func fenrirChart(t *testing.T) assembly.FrameChart {
	t.Helper()
	return mustSize(t, mustFrame(t, "otso-fenrir-stainless"), "L").Chart
}

func haydukeS4Chart(t *testing.T) assembly.FrameChart {
	t.Helper()
	return mustSize(t, mustFrame(t, "esker-hayduke"), "S4").Chart
}

func sklarLChart(t *testing.T) assembly.FrameChart {
	t.Helper()
	return mustSize(t, mustFrame(t, "sklar-pbj"), "L").Chart
}

func TestCheckChartConsistencyFenrirPasses(t *testing.T) {
	if err := checkChartConsistency(fenrirChart(t)); err != nil {
		t.Fatalf("checkChartConsistency(fenrirChart(t)): %v", err)
	}
}

func TestCheckChartConsistencyBudgetEdges(t *testing.T) {
	cases := []struct {
		name  string
		chart func() assembly.FrameChart
		want  string
	}{
		{
			name: "axle height inside",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.Frame.StackMm = 643.8
				return c
			},
			want: "",
		},
		{
			name: "axle height outside",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.Frame.StackMm = 643.95
				return c
			},
			want: "chart consistency: axle height mismatch +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)",
		},
		{
			name: "wheelbase inside",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.PublishedWheelbaseMm = 1160.6
				return c
			},
			want: "",
		},
		{
			name: "wheelbase outside",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.PublishedWheelbaseMm = 1160.8
				return c
			},
			want: "chart consistency: wheelbase mismatch -3.14 mm exceeds its ±3.02 mm rounding budget (Reach 0.50 mm, HTA 1.24 mm, HTL 0.19 mm, BB drop 0.08 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireChartConsistency(t, tc.chart(), tc.want)
		})
	}
}

func TestCheckChartConsistencyWithoutPrecision(t *testing.T) {
	c := fenrirChart(t)
	c.Precision = assembly.ChartPrecision{}
	want := "chart consistency: axle height mismatch +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)\nchart consistency: wheelbase mismatch -0.34 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"
	requireChartConsistency(t, c, want)
}

func TestCheckChartConsistencyInvalidPrecision(t *testing.T) {
	cases := []struct {
		name  string
		chart func() assembly.FrameChart
		want  string
	}{
		{
			name: "NaN Stack",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.Precision.StackMm = math.NaN()
				return c
			},
			want: "chart precision: Stack half-width must be finite and non-negative, got NaN",
		},
		{
			name: "negative HTA",
			chart: func() assembly.FrameChart {
				c := fenrirChart(t)
				c.Precision.HeadTubeAngleDeg = -0.1
				return c
			},
			want: "chart precision: HTA half-width must be finite and non-negative, got -0.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireChartConsistency(t, tc.chart(), tc.want)
		})
	}
}

func TestCheckChartConsistencyNudgeOutOfRange(t *testing.T) {
	c := fenrirChart(t)
	c.Frame.ChainstayMm = 72.4
	c.Precision = assembly.ChartPrecision{ChainstayMm: 0.5}
	err := checkChartConsistency(c)
	if err == nil {
		t.Fatal("checkChartConsistency: got nil, want out-of-range error")
	}
	const wantPrefix = "chart precision: Chainstay ±0.5 moves the chart out of range: "
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("error = %q, want prefix %q", err.Error(), wantPrefix)
	}
}

func TestCheckChartConsistencyInvalidWheelbase(t *testing.T) {
	const want = "published wheelbase must be finite and non-negative (0 = unpublished)"
	for _, wb := range []float64{-1, math.NaN()} {
		c := fenrirChart(t)
		c.PublishedWheelbaseMm = wb
		err := checkChartConsistency(c)
		if err == nil {
			t.Fatalf("PublishedWheelbaseMm = %v: got nil, want error", wb)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("PublishedWheelbaseMm = %v: error = %q, want substring %q", wb, err.Error(), want)
		}
	}
}

func TestCheckChartConsistencyHaydukeS4Allowance(t *testing.T) {
	const reason = "The declared front axle position leaves a wheelbase discrepancy beyond rounding."
	const terms = " (Reach 0.50 mm, HTA 2.54 mm, HTL 0.19 mm, BB drop 0.07 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)"
	const failure = "chart consistency: wheelbase mismatch +5.42 mm exceeds its ±4.31 mm rounding budget"
	cases := []struct {
		name      string
		allowance assembly.ChartAllowance
		want      string
	}{
		{"none", assembly.ChartAllowance{}, failure + terms},
		{"documented", assembly.ChartAllowance{WheelbaseMm: 1.2, Reason: reason}, ""},
		{"too small", assembly.ChartAllowance{WheelbaseMm: 1.0, Reason: reason}, failure + " plus its 1.00 mm allowance" + terms},
		{"inside slack", assembly.ChartAllowance{WheelbaseMm: 1.6, Reason: reason}, ""},
		{"outside slack", assembly.ChartAllowance{WheelbaseMm: 1.7, Reason: reason}, "chart allowance: wheelbase allowance 1.70 mm exceeds the 1.11 mm needed beyond rounding by more than 0.50 mm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := haydukeS4Chart(t)
			chart.Allowance = tc.allowance
			requireChartConsistency(t, chart, tc.want)
		})
	}
}

func TestCheckChartConsistencySklarLAllowance(t *testing.T) {
	const reason = "The published chart leaves an axle height discrepancy beyond rounding."
	const terms = " (Stack 0.50 mm, HTA 1.21 mm, HTL 0.47 mm, BB drop 0.50 mm)"
	const failure = "chart consistency: axle height mismatch -3.06 mm exceeds its ±2.68 mm rounding budget"
	cases := []struct {
		name      string
		allowance assembly.ChartAllowance
		want      string
	}{
		{"none", assembly.ChartAllowance{}, failure + terms},
		{"documented", assembly.ChartAllowance{AxleHeightMm: 0.4, Reason: reason}, ""},
		{"too small", assembly.ChartAllowance{AxleHeightMm: 0.3, Reason: reason}, failure + " plus its 0.30 mm allowance" + terms},
		{"unnecessary wheelbase allowance", assembly.ChartAllowance{AxleHeightMm: 0.4, WheelbaseMm: 0.5, Reason: reason}, "chart allowance: wheelbase allowance 0.50 mm is unnecessary: mismatch -3.56 mm is within its ±4.28 mm rounding budget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := sklarLChart(t)
			chart.Allowance = tc.allowance
			requireChartConsistency(t, chart, tc.want)
		})
	}
}

func TestCheckChartConsistencyUnnecessaryAllowance(t *testing.T) {
	chart := fenrirChart(t)
	chart.Allowance = assembly.ChartAllowance{AxleHeightMm: 0.5, Reason: "Axle height discrepancy."}
	requireChartConsistency(t, chart, "chart allowance: axle height allowance 0.50 mm is unnecessary: mismatch +0.22 mm is within its ±2.09 mm rounding budget")
}

func TestCheckChartConsistencyInvalidAllowance(t *testing.T) {
	cases := []struct {
		name        string
		allowance   assembly.ChartAllowance
		unpublished bool
		want        string
	}{
		{"negative axle height", assembly.ChartAllowance{AxleHeightMm: -0.1, Reason: "x"}, false, "chart allowance: axle height must be finite and non-negative, got -0.1 mm"},
		{"NaN wheelbase", assembly.ChartAllowance{WheelbaseMm: math.NaN(), Reason: "x"}, false, "chart allowance: wheelbase must be finite and non-negative, got NaN mm"},
		{"missing reason", assembly.ChartAllowance{AxleHeightMm: 0.5, Reason: "  "}, false, "chart allowance: a non-zero allowance needs a reason"},
		{"reason only", assembly.ChartAllowance{Reason: "x"}, false, "chart allowance: a reason without an allowance"},
		{"unpublished wheelbase", assembly.ChartAllowance{WheelbaseMm: 1.0, Reason: "x"}, true, "chart allowance: wheelbase allowance on a chart with no published wheelbase"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := fenrirChart(t)
			chart.Allowance = tc.allowance
			if tc.unpublished {
				chart.PublishedWheelbaseMm = 0
			}
			requireChartConsistency(t, chart, tc.want)
		})
	}
}

func TestCheckChartConsistencyUnpublishedWheelbase(t *testing.T) {
	chart := fenrirChart(t)
	chart.PublishedWheelbaseMm = 0
	t.Run("rounding covers axle height", func(t *testing.T) {
		requireChartConsistency(t, chart, "")
	})
	t.Run("axle height outside budget", func(t *testing.T) {
		chart := chart
		chart.Frame.StackMm = 643.95
		requireChartConsistency(t, chart, "chart consistency: axle height mismatch +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)")
	})
	t.Run("no rounding", func(t *testing.T) {
		chart := chart
		chart.Precision = assembly.ChartPrecision{}
		requireChartConsistency(t, chart, "chart consistency: axle height mismatch +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)")
	})
}

func TestCheckChartConsistencyRejectsMismatchedBasisCup(t *testing.T) {
	chart := fenrirChart(t)
	chart.CadLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}
	var inputErr *assembly.InputError
	if err := checkChartConsistency(chart); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
		t.Errorf("checkChartConsistency error = %v, want InputLowerCup", err)
	}
	chart.CadLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	if err := checkChartConsistency(chart); err != nil && errors.As(err, &inputErr) && inputErr.Field == assembly.InputLowerCup {
		t.Errorf("matching cup rejected as a headset error: %v", err)
	}
}

func TestMeasureChartConsistencyExposesChecks(t *testing.T) {
	near := func(name string, got, want, tol float64) {
		t.Helper()
		if math.Abs(got-want) > tol {
			t.Errorf("%s = %.4f, want %.4f (±%g)", name, got, want, tol)
		}
	}
	assertTerms := func(got []roundingTerm, wantCols []string, wantMm []float64) {
		t.Helper()
		if len(got) != len(wantCols) {
			t.Fatalf("len(Terms) = %d (%v), want %d (%v)", len(got), got, len(wantCols), wantCols)
		}
		for i, term := range got {
			if term.Column != wantCols[i] {
				t.Errorf("Terms[%d].Column = %q, want %q", i, term.Column, wantCols[i])
			}
			near(fmt.Sprintf("Terms[%d] (%s)", i, term.Column), term.Mm, wantMm[i], 0.005)
		}
	}

	t.Run("Hayduke S4", func(t *testing.T) {
		m, err := measureChartConsistency(haydukeS4Chart(t))
		if err != nil {
			t.Fatalf("measureChartConsistency: %v", err)
		}
		if len(m.Checks) != 2 {
			t.Fatalf("len(Checks) = %d, want 2", len(m.Checks))
		}
		wb := m.Checks[1]
		if wb.Name != "wheelbase" {
			t.Errorf("Checks[1].Name = %q, want wheelbase", wb.Name)
		}
		near("MismatchMm", wb.MismatchMm, 5.42, 0.005)
		near("BudgetMm", wb.BudgetMm, 4.31, 0.005)
		near("ExcessMm", wb.ExcessMm, 1.11, 0.005)
		if wb.AllowanceMm != 1.2 {
			t.Errorf("AllowanceMm = %v, want 1.2", wb.AllowanceMm)
		}
		assertTerms(wb.Terms, []string{"Reach", "HTA", "HTL", "BB drop", "Chainstay", "Wheelbase"}, []float64{0.50, 2.54, 0.19, 0.07, 0.51, 0.50})
		if err := m.Err(); err != nil {
			t.Errorf("Err() = %v, want nil", err)
		}
	})

	t.Run("Fenrir L unpublished wheelbase", func(t *testing.T) {
		chart := fenrirChart(t)
		chart.PublishedWheelbaseMm = 0
		m, err := measureChartConsistency(chart)
		if err != nil {
			t.Fatalf("measureChartConsistency: %v", err)
		}
		if len(m.Checks) != 1 {
			t.Fatalf("len(Checks) = %d, want 1", len(m.Checks))
		}
		ah := m.Checks[0]
		if ah.Name != "axle height" {
			t.Errorf("Checks[0].Name = %q, want axle height", ah.Name)
		}
		near("MismatchMm", ah.MismatchMm, 0.22, 0.005)
		near("BudgetMm", ah.BudgetMm, 2.09, 0.005)
		if ah.AllowanceMm != 0 {
			t.Errorf("AllowanceMm = %v, want 0", ah.AllowanceMm)
		}
		assertTerms(ah.Terms, []string{"Stack", "HTA", "HTL", "BB drop"}, []float64{0.50, 0.62, 0.46, 0.50})
		if err := m.Err(); err != nil {
			t.Errorf("Err() = %v, want nil", err)
		}
	})
}

func requireChartConsistency(t *testing.T, chart assembly.FrameChart, want string) {
	t.Helper()
	err := checkChartConsistency(chart)
	if want == "" {
		if err != nil {
			t.Fatalf("checkChartConsistency: got %v, want nil", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("checkChartConsistency: got nil, want %q", want)
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
