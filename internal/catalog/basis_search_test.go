package catalog_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/catalog"
)

// basisRange holds the inclusive [From, To] passing interval found by scanning
// one basis dimension with the other held fixed. Both fields are NaN when no
// value in the scanned window passes chart consistency.
type basisRange struct {
	From float64
	To   float64
}

// basisEval is the worst-case chart-consistency result across every size of a
// frame for one (lower cup, static A2C, rake) basis hypothesis.
type basisEval struct {
	A2cMm            float64
	RakeMm           float64
	Passes           bool
	AxleHeightExcess float64 // worst excess across sizes (≤ 0 passes)
	HasWheelbase     bool    // false when every size leaves PublishedWheelbaseMm == 0
	WheelbaseExcess  float64 // worst excess across published sizes; NaN when !HasWheelbase
}

// sizeImpliedA2c holds one frame size's zero-mismatch implied basis A2C and
// axle-height rounding budget.
type sizeImpliedA2c struct {
	SizeName           string
	ImpliedA2cMm       float64
	AxleHeightBudgetMm float64
}

// basisSearchResult summarizes a basis search for one frame, lower cup, and
// candidate rake across all frame sizes.
type basisSearchResult struct {
	Sizes           []sizeImpliedA2c
	ImpliedMinMm    float64
	ImpliedMaxMm    float64
	MinimaxA2cMm    float64
	RoundedA2cMm    float64
	A2cRange        basisRange
	RakeRange       basisRange
	RakeComparisons []basisEval
}

// evalBasis checks one (cup, a2c, rake) basis hypothesis across every size of f.
func evalBasis(f catalog.Frame, cup assembly.LowerCupSetup, a2c, rake float64) (basisEval, error) {
	if len(f.Sizes) == 0 {
		return basisEval{}, fmt.Errorf("frame %q has no sizes", f.ID)
	}
	basisFork, err := assembly.NewBasisFork(a2c, rake)
	if err != nil {
		return basisEval{}, fmt.Errorf("A2C %v, rake %v: %w", a2c, rake, err)
	}
	g := f
	g.BasisLowerCup = cup
	g.BasisFork = basisFork
	passes := true
	axleExcess := math.Inf(-1)
	wbExcess := math.Inf(-1)
	hasWB := false
	for _, s := range g.Sizes {
		m, err := measureChartConsistency(g, s)
		if err != nil {
			return basisEval{}, fmt.Errorf("%s at A2C %v, rake %v: %w", s.Name, a2c, rake, err)
		}
		if m.Err() != nil {
			passes = false
		}
		axleExcess = max(axleExcess, m.Checks[0].ExcessMm)
		if len(m.Checks) > 1 {
			hasWB = true
			wbExcess = max(wbExcess, m.Checks[1].ExcessMm)
		}
	}
	if !hasWB {
		wbExcess = math.NaN()
	}
	return basisEval{
		A2cMm:            a2c,
		RakeMm:           rake,
		Passes:           passes,
		AxleHeightExcess: axleExcess,
		HasWheelbase:     hasWB,
		WheelbaseExcess:  wbExcess,
	}, nil
}

// searchBasis finds each size's implied basis A2C at (cup, rake), the minimax
// and 0.5 mm-rounded A2C, the passing A2C and rake intervals, and direct
// comparisons across compareRakes at the rounded A2C.
func searchBasis(f catalog.Frame, cup assembly.LowerCupSetup, rake float64, compareRakes []float64) (basisSearchResult, error) {
	if len(f.Sizes) == 0 {
		return basisSearchResult{}, fmt.Errorf("frame %q has no sizes", f.ID)
	}
	const probeA2c = 480.0
	baseFork, err := assembly.NewBasisFork(probeA2c, rake)
	if err != nil {
		return basisSearchResult{}, err
	}
	upFork, err := assembly.NewBasisFork(probeA2c+1, rake)
	if err != nil {
		return basisSearchResult{}, err
	}
	base := f
	base.BasisLowerCup = cup
	base.BasisFork = baseFork
	up := base
	up.BasisFork = upFork

	lowest, highest := math.Inf(1), math.Inf(-1)
	sizes := make([]sizeImpliedA2c, len(base.Sizes))
	for i, s := range base.Sizes {
		at, err := measureChartConsistency(base, s)
		if err != nil {
			return basisSearchResult{}, err
		}
		above, err := measureChartConsistency(up, s)
		if err != nil {
			return basisSearchResult{}, err
		}
		m0, m1 := at.Checks[0].MismatchMm, above.Checks[0].MismatchMm
		// Axle-height mismatch is linear in A2C, so two placements 1 mm apart solve zero mismatch.
		implied := probeA2c - m0/(m1-m0)
		lowest, highest = min(lowest, implied), max(highest, implied)
		sizes[i] = sizeImpliedA2c{
			SizeName:           s.Name,
			ImpliedA2cMm:       implied,
			AxleHeightBudgetMm: at.Checks[0].BudgetMm,
		}
	}

	minimax := (lowest + highest) / 2
	a2c := math.Round(minimax*2) / 2

	scan := func(from, to, step float64, at func(v float64) (float64, float64)) (basisRange, error) {
		r := basisRange{From: math.NaN(), To: math.NaN()}
		for v := from; v <= to+step/2; v += step {
			candA2c, candRake := at(v)
			ev, err := evalBasis(f, cup, candA2c, candRake)
			if err != nil {
				return basisRange{}, err
			}
			if ev.Passes {
				if math.IsNaN(r.From) {
					r.From = v
				}
				r.To = v
			}
		}
		return r, nil
	}

	a2cRange, err := scan(a2c-10, a2c+10, 0.05, func(v float64) (float64, float64) { return v, rake })
	if err != nil {
		return basisSearchResult{}, err
	}
	rakeRange, err := scan(rake-15, rake+15, 0.05, func(v float64) (float64, float64) { return a2c, v })
	if err != nil {
		return basisSearchResult{}, err
	}

	var comps []basisEval
	for _, r := range compareRakes {
		ev, err := evalBasis(f, cup, a2c, r)
		if err != nil {
			return basisSearchResult{}, err
		}
		comps = append(comps, ev)
	}

	return basisSearchResult{
		Sizes:           sizes,
		ImpliedMinMm:    lowest,
		ImpliedMaxMm:    highest,
		MinimaxA2cMm:    minimax,
		RoundedA2cMm:    a2c,
		A2cRange:        a2cRange,
		RakeRange:       rakeRange,
		RakeComparisons: comps,
	}, nil
}

// logBasisSearch formats res cleanly for temporary ingestion tests and test logs.
func logBasisSearch(t *testing.T, label string, res basisSearchResult) {
	t.Helper()
	t.Logf("basis search: %s", label)
	for _, s := range res.Sizes {
		t.Logf("  %s: implied basis A2C %.2f mm (axle-height rounding budget ±%.2f mm)", s.SizeName, s.ImpliedA2cMm, s.AxleHeightBudgetMm)
	}
	t.Logf("  implied basis A2C %.2f–%.2f mm, minimax %.2f mm (rounded %.1f mm)", res.ImpliedMinMm, res.ImpliedMaxMm, res.MinimaxA2cMm, res.RoundedA2cMm)
	t.Logf("  passing A2C range: %.2f–%.2f mm; passing rake range: %.2f–%.2f mm", res.A2cRange.From, res.A2cRange.To, res.RakeRange.From, res.RakeRange.To)
	for _, c := range res.RakeComparisons {
		wb := "unpublished"
		if c.HasWheelbase {
			wb = fmt.Sprintf("%+.2f mm", c.WheelbaseExcess)
		}
		t.Logf("  A2C %.1f, rake %g: passes %v, worst axle-height excess %+.2f mm, worst wheelbase excess %s",
			c.A2cMm, c.RakeMm, c.Passes, c.AxleHeightExcess, wb)
	}
}

func assertNearBasis(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f (±%g)", label, got, want, tol)
	}
}

// TestBasisSearchCatalogFrames verifies evalBasis across all catalog frames
// and pins searchBasis outputs on representative frames.
func TestBasisSearchCatalogFrames(t *testing.T) {
	for _, f := range catalog.Frames() {
		ev, err := evalBasis(f, f.BasisLowerCup, f.BasisFork.BasisA2cMm(), f.BasisFork.RakeMm())
		if err != nil {
			t.Fatalf("%s declared basis: %v", f.ID, err)
		}
		if !ev.Passes {
			t.Errorf("%s declared basis fails consistency: %+v", f.ID, ev)
		}
	}

	cases := []struct {
		id         string
		rake       float64
		compare    []float64
		minA2c     float64
		maxA2c     float64
		minimaxA2c float64
		roundedA2c float64
		a2cRange   basisRange
		rakeRange  basisRange
		wantEval   []basisEval
	}{
		{
			id: "otso-fenrir-stainless", rake: 51, compare: []float64{44, 51, 51.5, 52},
			minA2c: 496.8361, maxA2c: 497.6214, minimaxA2c: 497.2288, roundedA2c: 497.0,
			a2cRange: basisRange{493.50, 500.95}, rakeRange: basisRange{41.45, 59.90},
			wantEval: []basisEval{
				{A2cMm: 497.0, RakeMm: 44, Passes: true, AxleHeightExcess: -0.9844, HasWheelbase: false, WheelbaseExcess: math.NaN()},
				{A2cMm: 497.0, RakeMm: 51, Passes: true, AxleHeightExcess: -3.2720, HasWheelbase: false, WheelbaseExcess: math.NaN()},
				{A2cMm: 497.0, RakeMm: 51.5, Passes: true, AxleHeightExcess: -3.0888, HasWheelbase: false, WheelbaseExcess: math.NaN()},
				{A2cMm: 497.0, RakeMm: 52, Passes: true, AxleHeightExcess: -2.9055, HasWheelbase: false, WheelbaseExcess: math.NaN()},
			},
		},
		{
			id: "surly-krampus", rake: 47, compare: []float64{44, 47, 51},
			minA2c: 481.2465, maxA2c: 481.9579, minimaxA2c: 481.6022, roundedA2c: 481.5,
			a2cRange: basisRange{477.95, 485.25}, rakeRange: basisRange{40.40, 53.65},
			wantEval: []basisEval{
				{A2cMm: 481.5, RakeMm: 44, Passes: true, AxleHeightExcess: -2.3900, HasWheelbase: true, WheelbaseExcess: -3.3532},
				{A2cMm: 481.5, RakeMm: 47, Passes: true, AxleHeightExcess: -3.3303, HasWheelbase: true, WheelbaseExcess: -6.1446},
				{A2cMm: 481.5, RakeMm: 51, Passes: true, AxleHeightExcess: -1.9294, HasWheelbase: true, WheelbaseExcess: -2.5240},
			},
		},
		{
			id: "moots-mountaineer", rake: 44, compare: []float64{44, 51},
			minA2c: 505.9346, maxA2c: 508.3849, minimaxA2c: 507.1598, roundedA2c: 507.0,
			a2cRange: basisRange{504.20, 510.15}, rakeRange: basisRange{36.35, 51.10},
			wantEval: []basisEval{
				{A2cMm: 507.0, RakeMm: 44, Passes: true, AxleHeightExcess: -2.6133, HasWheelbase: false, WheelbaseExcess: math.NaN()},
				{A2cMm: 507.0, RakeMm: 51, Passes: true, AxleHeightExcess: -0.0476, HasWheelbase: false, WheelbaseExcess: math.NaN()},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			f := mustFrame(t, tc.id)
			res, err := searchBasis(f, f.BasisLowerCup, tc.rake, tc.compare)
			if err != nil {
				t.Fatalf("searchBasis: %v", err)
			}
			logBasisSearch(t, tc.id, res)
			assertNearBasis(t, "ImpliedMinMm", res.ImpliedMinMm, tc.minA2c, 0.02)
			assertNearBasis(t, "ImpliedMaxMm", res.ImpliedMaxMm, tc.maxA2c, 0.02)
			assertNearBasis(t, "MinimaxA2cMm", res.MinimaxA2cMm, tc.minimaxA2c, 0.02)
			if res.RoundedA2cMm != tc.roundedA2c {
				t.Errorf("RoundedA2cMm = %v, want %v", res.RoundedA2cMm, tc.roundedA2c)
			}
			assertNearBasis(t, "A2cRange.From", res.A2cRange.From, tc.a2cRange.From, 0.10)
			assertNearBasis(t, "A2cRange.To", res.A2cRange.To, tc.a2cRange.To, 0.10)
			assertNearBasis(t, "RakeRange.From", res.RakeRange.From, tc.rakeRange.From, 0.10)
			assertNearBasis(t, "RakeRange.To", res.RakeRange.To, tc.rakeRange.To, 0.10)
			if len(res.RakeComparisons) != len(tc.wantEval) {
				t.Fatalf("got %d comparisons, want %d", len(res.RakeComparisons), len(tc.wantEval))
			}
			for i, w := range tc.wantEval {
				got := res.RakeComparisons[i]
				if got.A2cMm != w.A2cMm || got.RakeMm != w.RakeMm || got.Passes != w.Passes || got.HasWheelbase != w.HasWheelbase {
					t.Errorf("comp[%d] = %+v, want %+v", i, got, w)
					continue
				}
				assertNearBasis(t, fmt.Sprintf("comp[%d].AxleHeightExcess", i), got.AxleHeightExcess, w.AxleHeightExcess, 0.02)
				if w.HasWheelbase {
					assertNearBasis(t, fmt.Sprintf("comp[%d].WheelbaseExcess", i), got.WheelbaseExcess, w.WheelbaseExcess, 0.02)
				} else if !math.IsNaN(got.WheelbaseExcess) {
					t.Errorf("comp[%d].WheelbaseExcess = %v, want NaN", i, got.WheelbaseExcess)
				}
			}
		})
	}

	fenrir := mustFrame(t, "otso-fenrir-stainless")
	ev, err := evalBasis(fenrir, fenrir.BasisLowerCup, 495, 51)
	if err != nil {
		t.Fatalf("evalBasis(fenrir 495/51): %v", err)
	}
	if !ev.Passes || ev.HasWheelbase {
		t.Fatalf("evalBasis(fenrir 495/51) = %+v, want Passes=true HasWheelbase=false", ev)
	}
	assertNearBasis(t, "fenrir 495/51 AxleHeightExcess", ev.AxleHeightExcess, -1.4110, 0.02)
}

// TestBasisSearchErrorsAndRakeIndependence covers validation failures and
// verifies searchBasis uses its rake parameter rather than f.BasisFork.RakeMm.
func TestBasisSearchErrorsAndRakeIndependence(t *testing.T) {
	empty := mustFrame(t, "surly-krampus")
	empty.Sizes = nil
	if _, err := evalBasis(empty, empty.BasisLowerCup, 483, 47); err == nil || !strings.Contains(err.Error(), "has no sizes") {
		t.Errorf("evalBasis(empty sizes) = %v, want error containing %q", err, "has no sizes")
	}
	if _, err := searchBasis(empty, empty.BasisLowerCup, 47, nil); err == nil || !strings.Contains(err.Error(), "has no sizes") {
		t.Errorf("searchBasis(empty sizes) = %v, want error containing %q", err, "has no sizes")
	}

	krampus := mustFrame(t, "surly-krampus")
	badCup := assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 1}
	var ie *bounds.InputError
	if _, err := evalBasis(krampus, badCup, 483, 47); !errors.As(err, &ie) || ie.Field != bounds.InputLowerCup {
		t.Errorf("evalBasis(bad cup) = %v, want InputLowerCup", err)
	}
	ie = nil
	if _, err := searchBasis(krampus, badCup, 47, nil); !errors.As(err, &ie) || ie.Field != bounds.InputLowerCup {
		t.Errorf("searchBasis(bad cup) = %v, want InputLowerCup", err)
	}

	// Regression guard: implied A2C depends on rake, so f.BasisFork.RakeMm() must not bleed into searchBasis.
	f54 := mustFrame(t, "otso-fenrir-stainless")
	bf54, _ := assembly.NewBasisFork(f54.BasisFork.BasisA2cMm(), 54)
	f54.BasisFork = bf54
	f44 := f54
	bf44, _ := assembly.NewBasisFork(f44.BasisFork.BasisA2cMm(), 44)
	f44.BasisFork = bf44
	resFrom54, err := searchBasis(f54, f54.BasisLowerCup, 44, []float64{44})
	if err != nil {
		t.Fatalf("searchBasis(f54, 44): %v", err)
	}
	resFrom44, err := searchBasis(f44, f44.BasisLowerCup, 44, []float64{44})
	if err != nil {
		t.Fatalf("searchBasis(f44, 44): %v", err)
	}
	res54, err := searchBasis(f54, f54.BasisLowerCup, 54, []float64{54})
	if err != nil {
		t.Fatalf("searchBasis(f54, 54): %v", err)
	}
	assertNearBasis(t, "minimax match", resFrom54.MinimaxA2cMm, resFrom44.MinimaxA2cMm, 1e-12)
	assertNearBasis(t, "rake 44 minimax", resFrom54.MinimaxA2cMm, 494.4006, 0.02)
	assertNearBasis(t, "rake 54 minimax", res54.MinimaxA2cMm, 498.4408, 0.02)
	if math.Abs(resFrom54.MinimaxA2cMm-res54.MinimaxA2cMm) < 3.0 {
		t.Errorf("expected rake 44 vs 54 minimax A2C to differ by >3 mm, got %v vs %v", resFrom54.MinimaxA2cMm, res54.MinimaxA2cMm)
	}
}
