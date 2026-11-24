package catalog_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/pose"
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
	halfWidth func(catalog.ChartPrecision) float64
	nudge     func(*catalog.FrameSize, float64)
}{
	{"Reach", func(p catalog.ChartPrecision) float64 { return p.ReachMm }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.ReachMm += d }},
	{"Stack", func(p catalog.ChartPrecision) float64 { return p.StackMm }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.StackMm += d }},
	{"HTA", func(p catalog.ChartPrecision) float64 { return float64(p.HeadTubeAngleDeg) }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.HeadTubeAngleDeg += pose.Deg(d) }},
	{"HTL", func(p catalog.ChartPrecision) float64 { return p.HeadTubeLengthMm }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.HeadTubeLengthMm += d }},
	{"BB drop", func(p catalog.ChartPrecision) float64 { return p.BbDropMm }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.BbDropMm += d }},
	{"Chainstay", func(p catalog.ChartPrecision) float64 { return p.ChainstayMm }, func(s *catalog.FrameSize, d float64) { s.ChartFrame.ChainstayMm += d }},
	{"Wheelbase", func(p catalog.ChartPrecision) float64 { return p.WheelbaseMm }, func(s *catalog.FrameSize, d float64) { s.PublishedWheelbaseMm += d }},
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
		unpitched.RearAxle.Origin().VerticalOffsetTo(unpitched.FrontAxle.Origin()),
		(unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - publishedWheelbaseMm,
	}
}

// measureChartConsistency places the chart's unpitched basis frameset and each
// ±half-width column nudge, and returns every check. Its error reports input
// the check cannot measure.
func measureChartConsistency(f catalog.Frame, s catalog.FrameSize) (chartConsistency, error) {
	basisFork := assembly.ForkSetup{Spec: f.BasisFork}
	unpitched, err := assembly.PlaceUnpitchedFrameset(s.ChartFrame, f.BasisLowerCup, basisFork, wheels)
	if err != nil {
		return chartConsistency{}, err
	}
	if !isFinite(s.PublishedWheelbaseMm) || s.PublishedWheelbaseMm < 0 {
		return chartConsistency{}, fmt.Errorf("published wheelbase must be finite and non-negative (0 = unpublished), got %v mm", s.PublishedWheelbaseMm)
	}
	allowances := [2]float64{s.Allowance.AxleHeightMm, s.Allowance.WheelbaseMm}
	for i, name := range consistencyCheckNames {
		if allowance := allowances[i]; !isFinite(allowance) || allowance < 0 {
			return chartConsistency{}, fmt.Errorf("chart allowance: %s must be finite and non-negative, got %v mm", name, allowance)
		}
	}
	hasAllowance := s.Allowance.AxleHeightMm > 0 || s.Allowance.WheelbaseMm > 0
	hasReason := strings.TrimSpace(s.Allowance.Reason) != ""
	if hasAllowance && !hasReason {
		return chartConsistency{}, errors.New("chart allowance: a non-zero allowance needs a reason")
	}
	if !hasAllowance && hasReason {
		return chartConsistency{}, errors.New("chart allowance: a reason without an allowance")
	}
	checkNames, budgetColumns := consistencyCheckNames[:], chartColumns
	if s.PublishedWheelbaseMm == 0 {
		if s.Allowance.WheelbaseMm > 0 {
			return chartConsistency{}, errors.New("chart allowance: wheelbase allowance on a chart with no published wheelbase")
		}
		// Wheelbase is the final check and chart column.
		checkNames = checkNames[:1]
		budgetColumns = budgetColumns[:len(budgetColumns)-1]
	}
	for _, col := range chartColumns {
		if hw := col.halfWidth(f.ChartPrecision); !isFinite(hw) || hw < 0 {
			return chartConsistency{}, fmt.Errorf("chart precision: %s half-width must be finite and non-negative, got %v", col.name, hw)
		}
	}

	mismatches := chartMismatches(unpitched, s.PublishedWheelbaseMm)
	var budgets [2]float64
	var terms [2][]roundingTerm
	for _, col := range budgetColumns {
		hw := col.halfWidth(f.ChartPrecision)
		if hw == 0 {
			continue
		}
		up, down := s, s
		col.nudge(&up, hw)
		col.nudge(&down, -hw)
		upPlaced, err := assembly.PlaceUnpitchedFrameset(up.ChartFrame, f.BasisLowerCup, basisFork, wheels)
		if err != nil {
			return chartConsistency{}, fmt.Errorf("chart precision: %s ±%v moves the chart out of range: %w", col.name, hw, err)
		}
		downPlaced, err := assembly.PlaceUnpitchedFrameset(down.ChartFrame, f.BasisLowerCup, basisFork, wheels)
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
func checkChartConsistency(f catalog.Frame, s catalog.FrameSize) error {
	m, err := measureChartConsistency(f, s)
	if err != nil {
		return err
	}
	return m.Err()
}

func fenrirL(t *testing.T) (catalog.Frame, catalog.FrameSize) {
	t.Helper()
	f := mustFrame(t, "otso-fenrir-stainless")
	return f, mustSize(t, f, "L")
}

func krampusL(t *testing.T) (catalog.Frame, catalog.FrameSize) {
	t.Helper()
	f := mustFrame(t, "surly-krampus")
	return f, mustSize(t, f, "L")
}

func haydukeS4(t *testing.T) (catalog.Frame, catalog.FrameSize) {
	t.Helper()
	f := mustFrame(t, "esker-hayduke")
	return f, mustSize(t, f, "S4")
}

func sklarL(t *testing.T) (catalog.Frame, catalog.FrameSize) {
	t.Helper()
	f := mustFrame(t, "sklar-pbj")
	return f, mustSize(t, f, "L")
}

func TestCheckChartConsistencyFenrirPasses(t *testing.T) {
	f, s := fenrirL(t)
	if err := checkChartConsistency(f, s); err != nil {
		t.Fatalf("checkChartConsistency(fenrir L): %v", err)
	}
}

func TestCheckChartConsistencyBudgetEdges(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*catalog.FrameSize)
		want   string
	}{
		{
			name:   "axle height inside",
			mutate: func(s *catalog.FrameSize) { s.ChartFrame.StackMm = 616.7 },
			want:   "",
		},
		{
			name:   "axle height outside",
			mutate: func(s *catalog.FrameSize) { s.ChartFrame.StackMm = 616.8 },
			want:   "chart consistency: axle height mismatch +3.83 mm exceeds its ±3.76 mm rounding budget (Stack 0.50 mm, HTA 2.30 mm, HTL 0.47 mm, BB drop 0.50 mm)",
		},
		{
			name:   "wheelbase inside",
			mutate: func(s *catalog.FrameSize) { s.PublishedWheelbaseMm = 1145.3 },
			want:   "",
		},
		{
			name:   "wheelbase outside",
			mutate: func(s *catalog.FrameSize) { s.PublishedWheelbaseMm = 1145.4 },
			want:   "chart consistency: wheelbase mismatch -6.59 mm exceeds its ±6.55 mm rounding budget (Reach 0.50 mm, HTA 4.79 mm, HTL 0.18 mm, BB drop 0.08 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, s := krampusL(t)
			tc.mutate(&s)
			requireChartConsistency(t, f, s, tc.want)
		})
	}
}

func TestCheckChartConsistencyWithoutPrecision(t *testing.T) {
	f, s := krampusL(t)
	f.ChartPrecision = catalog.ChartPrecision{}
	want := "chart consistency: axle height mismatch -0.97 mm exceeds its ±0.00 mm rounding budget (no rounded columns)\nchart consistency: wheelbase mismatch +0.81 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"
	requireChartConsistency(t, f, s, want)
}

func TestCheckChartConsistencyInvalidPrecision(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*catalog.Frame)
		want   string
	}{
		{
			name:   "NaN Stack",
			mutate: func(f *catalog.Frame) { f.ChartPrecision.StackMm = math.NaN() },
			want:   "chart precision: Stack half-width must be finite and non-negative, got NaN",
		},
		{
			name:   "negative HTA",
			mutate: func(f *catalog.Frame) { f.ChartPrecision.HeadTubeAngleDeg = -0.1 },
			want:   "chart precision: HTA half-width must be finite and non-negative, got -0.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, s := fenrirL(t)
			tc.mutate(&f)
			requireChartConsistency(t, f, s, tc.want)
		})
	}
}

func TestCheckChartConsistencyNudgeOutOfRange(t *testing.T) {
	f, s := fenrirL(t)
	s.ChartFrame.ChainstayMm = 70.4
	f.ChartPrecision = catalog.ChartPrecision{ChainstayMm: 0.5}
	err := checkChartConsistency(f, s)
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
		f, s := fenrirL(t)
		s.PublishedWheelbaseMm = wb
		err := checkChartConsistency(f, s)
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
		allowance catalog.ChartAllowance
		want      string
	}{
		{"none", catalog.ChartAllowance{}, failure + terms},
		{"documented", catalog.ChartAllowance{WheelbaseMm: 1.2, Reason: reason}, ""},
		{"too small", catalog.ChartAllowance{WheelbaseMm: 1.0, Reason: reason}, failure + " plus its 1.00 mm allowance" + terms},
		{"inside slack", catalog.ChartAllowance{WheelbaseMm: 1.6, Reason: reason}, ""},
		{"outside slack", catalog.ChartAllowance{WheelbaseMm: 1.7, Reason: reason}, "chart allowance: wheelbase allowance 1.70 mm exceeds the 1.11 mm needed beyond rounding by more than 0.50 mm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, s := haydukeS4(t)
			s.Allowance = tc.allowance
			requireChartConsistency(t, f, s, tc.want)
		})
	}
}

func TestCheckChartConsistencySklarLAllowance(t *testing.T) {
	const reason = "The published chart leaves an axle height discrepancy beyond rounding."
	const terms = " (Stack 0.50 mm, HTA 1.21 mm, HTL 0.47 mm, BB drop 0.50 mm)"
	const failure = "chart consistency: axle height mismatch -3.06 mm exceeds its ±2.68 mm rounding budget"
	cases := []struct {
		name      string
		allowance catalog.ChartAllowance
		want      string
	}{
		{"none", catalog.ChartAllowance{}, failure + terms},
		{"documented", catalog.ChartAllowance{AxleHeightMm: 0.4, Reason: reason}, ""},
		{"too small", catalog.ChartAllowance{AxleHeightMm: 0.3, Reason: reason}, failure + " plus its 0.30 mm allowance" + terms},
		{"unnecessary wheelbase allowance", catalog.ChartAllowance{AxleHeightMm: 0.4, WheelbaseMm: 0.5, Reason: reason}, "chart allowance: wheelbase allowance 0.50 mm is unnecessary: mismatch -3.56 mm is within its ±4.28 mm rounding budget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, s := sklarL(t)
			s.Allowance = tc.allowance
			requireChartConsistency(t, f, s, tc.want)
		})
	}
}

func TestCheckChartConsistencyUnnecessaryAllowance(t *testing.T) {
	f, s := fenrirL(t)
	s.Allowance = catalog.ChartAllowance{AxleHeightMm: 0.5, Reason: "Axle height discrepancy."}
	requireChartConsistency(t, f, s, "chart allowance: axle height allowance 0.50 mm is unnecessary: mismatch +2.34 mm is within its ±3.97 mm rounding budget")
}

func TestCheckChartConsistencyInvalidAllowance(t *testing.T) {
	cases := []struct {
		name        string
		allowance   catalog.ChartAllowance
		unpublished bool
		want        string
	}{
		{"negative axle height", catalog.ChartAllowance{AxleHeightMm: -0.1, Reason: "x"}, false, "chart allowance: axle height must be finite and non-negative, got -0.1 mm"},
		{"NaN wheelbase", catalog.ChartAllowance{WheelbaseMm: math.NaN(), Reason: "x"}, false, "chart allowance: wheelbase must be finite and non-negative, got NaN mm"},
		{"missing reason", catalog.ChartAllowance{AxleHeightMm: 0.5, Reason: "  "}, false, "chart allowance: a non-zero allowance needs a reason"},
		{"reason only", catalog.ChartAllowance{Reason: "x"}, false, "chart allowance: a reason without an allowance"},
		{"unpublished wheelbase", catalog.ChartAllowance{WheelbaseMm: 1.0, Reason: "x"}, true, "chart allowance: wheelbase allowance on a chart with no published wheelbase"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, s := krampusL(t)
			s.Allowance = tc.allowance
			if tc.unpublished {
				s.PublishedWheelbaseMm = 0
			}
			requireChartConsistency(t, f, s, tc.want)
		})
	}
}

func TestCheckChartConsistencyUnpublishedWheelbase(t *testing.T) {
	f, s := fenrirL(t)
	if s.PublishedWheelbaseMm != 0 {
		t.Fatalf("fenrir L PublishedWheelbaseMm = %v, want 0", s.PublishedWheelbaseMm)
	}
	t.Run("rounding covers axle height", func(t *testing.T) {
		requireChartConsistency(t, f, s, "")
	})
	t.Run("axle height outside budget", func(t *testing.T) {
		s := s
		s.ChartFrame.StackMm = 644.0
		requireChartConsistency(t, f, s, "chart consistency: axle height mismatch +4.34 mm exceeds its ±3.97 mm rounding budget (Stack 0.50 mm, HTA 2.51 mm, HTL 0.46 mm, BB drop 0.50 mm)")
	})
	t.Run("no rounding", func(t *testing.T) {
		f := f
		f.ChartPrecision = catalog.ChartPrecision{}
		requireChartConsistency(t, f, s, "chart consistency: axle height mismatch +2.34 mm exceeds its ±0.00 mm rounding budget (no rounded columns)")
	})
}

func TestCheckChartConsistencyRejectsMismatchedBasisCup(t *testing.T) {
	f, s := fenrirL(t)
	f.BasisLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}
	var inputErr *assembly.InputError
	if err := checkChartConsistency(f, s); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
		t.Errorf("checkChartConsistency error = %v, want InputLowerCup", err)
	}
	f.BasisLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	if err := checkChartConsistency(f, s); err != nil && errors.As(err, &inputErr) && inputErr.Field == assembly.InputLowerCup {
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
		f, s := haydukeS4(t)
		m, err := measureChartConsistency(f, s)
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
		f, s := fenrirL(t)
		m, err := measureChartConsistency(f, s)
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
		near("MismatchMm", ah.MismatchMm, 2.34, 0.005)
		near("BudgetMm", ah.BudgetMm, 3.97, 0.005)
		if ah.AllowanceMm != 0 {
			t.Errorf("AllowanceMm = %v, want 0", ah.AllowanceMm)
		}
		assertTerms(ah.Terms, []string{"Stack", "HTA", "HTL", "BB drop"}, []float64{0.50, 2.51, 0.46, 0.50})
		if err := m.Err(); err != nil {
			t.Errorf("Err() = %v, want nil", err)
		}
	})
}

func requireChartConsistency(t *testing.T, f catalog.Frame, s catalog.FrameSize, want string) {
	t.Helper()
	err := checkChartConsistency(f, s)
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
