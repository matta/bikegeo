package geometry_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"bikegeo/pkg/frames"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func termByName(b model.ErrorBudget, name string) (model.BudgetTerm, bool) {
	for _, t := range b.Terms {
		if t.Name == name {
			return t, true
		}
	}
	return model.BudgetTerm{}, false
}

// TestWheelbaseBudgetClosedForm checks the wheelbase budget for the hardtail in
// docs/AFFINE_EXAMPLE.md against the closed-form closure equation
//
//	WB = sqrt(CS² − BB²) + Reach + (Stack − BB)/tan(HTA) + Rake/sin(HTA)
//
// and its analytic partial derivatives. (The output printed in that document
// for this case, residual −5.69 mm and HTA ±5.13 mm, does not follow from its
// own formula; the formula gives −12.21 mm and ±6.15 mm.) Rake is a modelling
// hypothesis rather than a rounded chart number, so it has no rounding term.
func TestWheelbaseBudgetClosedForm(t *testing.T) {
	const (
		reach, stack, htaDeg, bb, cs, rake, wb = 450.0, 630.0, 66.0, 60.0, 435.0, 44.0, 1195.0
	)
	bike := model.BikeGeometry{
		Reach: reach, Stack: stack, Htl: 120, HtaDeg: htaDeg, BbDrop: bb,
		ChainstayLength: cs, ForkRake: rake, Wheelbase: wb,
	}
	b, ok := geometry.WheelbaseBudget(bike)
	if !ok {
		t.Fatal("WheelbaseBudget not computed")
	}

	h := htaDeg * math.Pi / 180
	sin, cos, tan := math.Sin(h), math.Cos(h), math.Tan(h)
	rc := math.Sqrt(cs*cs - bb*bb)
	wantResid := rc + reach + (stack-bb)/tan + rake/sin - wb
	hwH := 0.5 * math.Pi / 180
	want := map[string]float64{
		"Reach":     0.5,
		"Stack":     0.5 / tan,
		"BB Drop":   0.5 * math.Abs(-bb/rc-1/tan),
		"Chainstay": 0.5 * cs / rc,
		"Wheelbase": 0.5,
		"HTA":       hwH * (stack - bb + rake*cos) / (sin * sin),
	}

	if math.Abs(b.Residual-wantResid) > 1e-6 {
		t.Errorf("residual = %.4f, want %.4f", b.Residual, wantResid)
	}
	if math.Abs(wantResid-(-12.21)) > 0.01 {
		t.Fatalf("closed form drifted: %.4f", wantResid)
	}
	for name, w := range want {
		term, ok := termByName(b, name)
		if !ok {
			t.Errorf("missing %s term", name)
			continue
		}
		// One-sided worst case = first-order term plus up to ½·f''·hw² of curvature.
		if d := term.ContributionMm - w; d < -1e-6 || d > 0.01*w+1e-6 {
			t.Errorf("%s contribution = %.4f, want %.4f (+≤1%% curvature)", name, term.ContributionMm, w)
		}
	}
	if _, ok := termByName(b, "HTL"); ok {
		t.Error("HTL cancels out of the wheelbase closure and must not contribute")
	}
	if b.Terms[0].Name != "HTA" {
		t.Errorf("largest term = %s, want HTA", b.Terms[0].Name)
	}
	if got := b.Tier(); got != model.TierExceeds {
		t.Errorf("tier = %s, want %s (|%.2f| > %.2f mm rounding budget)", got, model.TierExceeds, b.Residual, b.RoundingTol())
	}
}

type budgetFn func(model.BikeGeometry) (model.ErrorBudget, bool)

// TestBudgetsEncloseRoundingPerturbations checks that the budget really bounds the
// residual: moving every published input anywhere inside its rounding interval
// (all corners, plus random interior points) never shifts a residual by more than
// the budget, up to the neglected cross terms between inputs. Those are measured
// at about 0.1% (≈0.01 mm) on the real catalog; the test allows 0.5%.
func TestBudgetsEncloseRoundingPerturbations(t *testing.T) {
	checks := map[string]budgetFn{
		"a2c":          geometry.A2cBudget,
		"front_center": geometry.FrontCenterBudget,
		"wheelbase":    geometry.WheelbaseBudget,
	}
	rng := rand.New(rand.NewPCG(1, 2))

	type field struct {
		ptr *float64
		hw  float64
	}
	fieldsOf := func(b *model.BikeGeometry) []field {
		p := b.ChartPrecision
		return []field{
			{&b.Reach, p.ReachMm}, {&b.Stack, p.StackMm}, {&b.HtaDeg, p.HtaDeg}, {&b.Htl, p.HtlMm},
			{&b.BbDrop, p.BbDropMm}, {&b.ChainstayLength, p.ChainstayMm}, {&b.Wheelbase, p.WheelbaseMm},
			{&b.FrontCenter, p.FrontCenterMm},
		}
	}

	for _, frame := range frames.AllFrames() {
		for _, sz := range frame.GetSizes() {
			bike, err := geometry.MaterializeBikeGeometry(frame, sz)
			if err != nil {
				t.Fatalf("%s %s: %v", frame.ID, sz, err)
			}
			for name, fn := range checks {
				base, ok := fn(bike)
				if !ok {
					continue
				}
				limit := base.RoundingTol()*1.005 + 1e-9
				failed := false
				try := func(signs func(i int) float64) {
					if failed {
						return
					}
					p := bike
					for i, f := range fieldsOf(&p) {
						if *f.ptr > 0 {
							*f.ptr += signs(i) * f.hw
						}
					}
					got, ok := fn(p)
					if !ok {
						t.Fatalf("%s %s %s: budget not computed after perturbation", frame.ID, sz, name)
					}
					if d := math.Abs(got.Residual - base.Residual); d > limit {
						failed = true
						t.Errorf("%s %s %s: perturbed residual moved %.4f mm, budget %.4f mm", frame.ID, sz, name, d, base.RoundingTol())
					}
				}
				for corner := range 1 << 8 {
					try(func(i int) float64 {
						if corner&(1<<i) != 0 {
							return 1
						}
						return -1
					})
				}
				for range 64 {
					try(func(int) float64 { return 2*rng.Float64() - 1 })
				}
			}
		}
	}
}

func TestInferChartPrecision(t *testing.T) {
	frame := model.FrameModel{
		Geometries: map[string]model.SizeGeometry{
			"S": {Reach: 430, Stack: 600, HtaDeg: 68, Htl: 90, BbDrop: 60, ChainstayLength: 430.5, Wheelbase: 1150},
			"M": {Reach: 450, Stack: 610, HtaDeg: 68.5, Htl: 100, BbDrop: 60, ChainstayLength: 431, Wheelbase: 1170.2},
			"L": {Reach: 470, Stack: 620, HtaDeg: 69, Htl: 110, BbDrop: 60, ChainstayLength: 432, Wheelbase: 1190},
		},
	}
	got := geometry.InferChartPrecision(frame)
	want := model.ChartPrecision{
		ReachMm:       0.5,
		StackMm:       0.5,
		HtaDeg:        0.25, // 68 / 68.5 / 69: the column is on a half-degree grid
		HtlMm:         0.5,
		BbDropMm:      0.5,
		ChainstayMm:   0.25, // one value on a half-mm grid
		WheelbaseMm:   0.05, // one value with a decimal
		FrontCenterMm: 0.5,  // unpublished column: no evidence against the integer grid
	}
	if got != want {
		t.Errorf("InferChartPrecision =\n  %+v\nwant\n  %+v", got, want)
	}

	quarter := model.FrameModel{Geometries: map[string]model.SizeGeometry{
		"A": {HtaDeg: 69.25}, "B": {HtaDeg: 69.75}, "C": {HtaDeg: 70},
	}}
	if hw := geometry.InferChartPrecision(quarter).HtaDeg; hw != 0.125 {
		t.Errorf("quarter-degree HTA column half-width = %v, want 0.125", hw)
	}
}

// TestBudgetNarrowerPrecisionNeverWidens: a more precise chart can only tighten a budget.
func TestBudgetNarrowerPrecisionNeverWidens(t *testing.T) {
	bike, err := geometry.MaterializeBikeGeometry(frames.RevelElJefe, "L")
	if err != nil {
		t.Fatal(err)
	}
	wide, _ := geometry.WheelbaseBudget(bike)
	narrow := bike
	narrow.ChartPrecision.HtaDeg = 0.05
	narrow.ChartPrecision.ReachMm = 0.05
	tight, _ := geometry.WheelbaseBudget(narrow)
	if tight.RoundingTol() > wide.RoundingTol() {
		t.Errorf("narrower precision widened the budget: %.3f > %.3f", tight.RoundingTol(), wide.RoundingTol())
	}
	if tight.Residual != wide.Residual {
		t.Errorf("precision must not change the residual: %.4f vs %.4f", tight.Residual, wide.Residual)
	}
}

func TestErrorBudgetTiers(t *testing.T) {
	b := model.ErrorBudget{LengthTol: 1.5, AngleTol: 2.0, Allowance: 0.5}
	cases := []struct {
		residual float64
		want     model.BudgetTier
	}{
		{0, model.TierLengthRounding},
		{-1.5, model.TierLengthRounding},
		{2.0, model.TierAngleRounding},
		{-3.5, model.TierAngleRounding},
		{3.8, model.TierAllowance},
		{4.1, model.TierExceeds},
	}
	for _, c := range cases {
		b.Residual = c.residual
		if got := b.Tier(); got != c.want {
			t.Errorf("residual %.1f: tier %s, want %s", c.residual, got, c.want)
		}
	}
	b.Residual = 4.25
	if got := b.NeededAllowance(); math.Abs(got-0.75) > 1e-12 {
		t.Errorf("NeededAllowance = %v, want 0.75", got)
	}
	if got := b.Saturation(); math.Abs(got-4.25/3.5) > 1e-12 {
		t.Errorf("Saturation = %v, want %v", got, 4.25/3.5)
	}
}
