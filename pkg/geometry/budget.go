package geometry

import (
	"math"
	"sort"

	"bikegeo/pkg/model"
)

// Rounding budgets.
//
// A published geometry chart is a set of rounded numbers, so the closure
// equations that tie them together (Stack vs fork A2C, Front Center, Wheelbase)
// never balance exactly. Instead of comparing each residual against a fixed
// tolerance, the functions here work out how large a residual rounding alone
// could produce: every published input is moved by ± its rounding half-width
// through the real closure equation, and the resulting changes are summed.
//
// This is first-order affine arithmetic evaluated numerically. Each residual is
// a single function of the raw chart inputs, so an input that appears several
// times in an equation (BB drop in both the front and rear triangles, the head
// angle in both the stack and rake projections) is perturbed once and its
// effects reinforce or cancel coherently.

// Published values are tried against these grids, coarsest first. A column whose
// values all sit on a grid is taken to be rounded to that grid.
var (
	lengthGridsMm = []float64{1, 0.5, 0.1, 0.01}
	angleGridsDeg = []float64{1, 0.5, 0.25, 0.1, 0.05, 0.01}
)

// minBudgetContributionMm drops inputs that do not take part in an equation.
const minBudgetContributionMm = 1e-9

// gridHalfWidth returns half of the coarsest grid step that every positive value
// in vals sits on, or half of the finest step if none fits.
func gridHalfWidth(vals []float64, grids []float64) float64 {
	for _, g := range grids {
		fits := true
		for _, v := range vals {
			if v <= 0 {
				continue
			}
			q := v / g
			if math.Abs(q-math.Round(q)) > 1e-6 {
				fits = false
				break
			}
		}
		if fits {
			return g / 2
		}
	}
	return grids[len(grids)-1] / 2
}

// InferChartPrecision derives the rounding half-width of each published column
// from the frame's whole size chart: a head-angle column reading 68, 68.5, 69 is
// on a half-degree grid (±0.25°) even though two of its values are integers.
//
// Frame data are Go float literals, so a chart that prints "69.0" is
// indistinguishable from one that prints "69"; the grid is the only evidence.
func InferChartPrecision(frame model.FrameModel) model.ChartPrecision {
	n := len(frame.Geometries)
	col := func(get func(model.SizeGeometry) float64) []float64 {
		vals := make([]float64, 0, n)
		for _, g := range frame.Geometries {
			vals = append(vals, get(g))
		}
		return vals
	}
	return model.ChartPrecision{
		ReachMm:       gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.Reach }), lengthGridsMm),
		StackMm:       gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.Stack }), lengthGridsMm),
		HtaDeg:        model.Deg(gridHalfWidth(col(func(g model.SizeGeometry) float64 { return float64(g.HtaDeg) }), angleGridsDeg)),
		HtlMm:         gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.Htl }), lengthGridsMm),
		BbDropMm:      gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.BbDrop }), lengthGridsMm),
		ChainstayMm:   gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.ChainstayLength }), lengthGridsMm),
		WheelbaseMm:   gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.Wheelbase }), lengthGridsMm),
		FrontCenterMm: gridHalfWidth(col(func(g model.SizeGeometry) float64 { return g.FrontCenter }), lengthGridsMm),
	}
}

// ChartPrecisionFor returns the bike's chart precision, inferring any field that
// was not set (a BikeGeometry built by hand rather than materialized from a
// FrameModel) from that single size's own value.
func ChartPrecisionFor(bike model.BikeGeometry) model.ChartPrecision {
	p := bike.ChartPrecision
	fill := func(dst *float64, val float64, grids []float64) {
		if *dst <= 0 {
			*dst = gridHalfWidth([]float64{val}, grids)
		}
	}
	fill(&p.ReachMm, bike.Reach, lengthGridsMm)
	fill(&p.StackMm, bike.Stack, lengthGridsMm)
	if p.HtaDeg <= 0 {
		p.HtaDeg = model.Deg(gridHalfWidth([]float64{float64(bike.HtaDeg)}, angleGridsDeg))
	}
	fill(&p.HtlMm, bike.Htl, lengthGridsMm)
	fill(&p.BbDropMm, bike.BbDrop, lengthGridsMm)
	fill(&p.ChainstayMm, bike.ChainstayLength, lengthGridsMm)
	fill(&p.WheelbaseMm, bike.Wheelbase, lengthGridsMm)
	fill(&p.FrontCenterMm, bike.FrontCenter, lengthGridsMm)
	return p
}

// chartInputs are the published, rounded chart numbers a closure check depends on.
// Every input is a plain float64, the head angle included, so that buildBudget can
// perturb any of them through the same *float64; hta gives the angle its type back.
type chartInputs struct {
	reach, stack, htaDeg, htl, bbDrop, chainstay, wheelbase, frontCenter float64
}

func (c chartInputs) hta() model.Deg { return model.Deg(c.htaDeg) }

func chartInputsOf(bike model.BikeGeometry) chartInputs {
	return chartInputs{
		reach:       bike.Reach,
		stack:       bike.Stack,
		htaDeg:      float64(bike.HtaDeg),
		htl:         bike.Htl,
		bbDrop:      bike.BbDrop,
		chainstay:   bike.ChainstayLength,
		wheelbase:   bike.Wheelbase,
		frontCenter: bike.FrontCenter,
	}
}

type chartField struct {
	name      string
	isAngle   bool
	value     func(*chartInputs) *float64
	halfWidth func(model.ChartPrecision) float64
}

var chartFields = []chartField{
	{"Reach", false, func(c *chartInputs) *float64 { return &c.reach }, func(p model.ChartPrecision) float64 { return p.ReachMm }},
	{"Stack", false, func(c *chartInputs) *float64 { return &c.stack }, func(p model.ChartPrecision) float64 { return p.StackMm }},
	{"HTA", true, func(c *chartInputs) *float64 { return &c.htaDeg }, func(p model.ChartPrecision) float64 { return float64(p.HtaDeg) }},
	{"HTL", false, func(c *chartInputs) *float64 { return &c.htl }, func(p model.ChartPrecision) float64 { return p.HtlMm }},
	{"BB Drop", false, func(c *chartInputs) *float64 { return &c.bbDrop }, func(p model.ChartPrecision) float64 { return p.BbDropMm }},
	{"Chainstay", false, func(c *chartInputs) *float64 { return &c.chainstay }, func(p model.ChartPrecision) float64 { return p.ChainstayMm }},
	{"Wheelbase", false, func(c *chartInputs) *float64 { return &c.wheelbase }, func(p model.ChartPrecision) float64 { return p.WheelbaseMm }},
	{"Front Center", false, func(c *chartInputs) *float64 { return &c.frontCenter }, func(p model.ChartPrecision) float64 { return p.FrontCenterMm }},
}

// buildBudget evaluates resid at the published inputs and at each input moved by
// ± its half-width. Each term is the larger of the two one-sided changes, which is
// the exact worst case for that input alone: it includes the curvature of the
// trigonometric terms (about 1% of the head-angle term on a ±0.5° chart) that a
// derivative or a symmetric secant would miss, and needs no step-size constant.
// Only the cross terms between inputs (head angle × Stack, head angle × BB drop)
// are neglected; they are about 0.1% of the budget, ≈0.01 mm, on real charts
// (see TestBudgetsEncloseRoundingPerturbations).
func buildBudget(in chartInputs, prec model.ChartPrecision, allowance float64, resid func(chartInputs) float64) model.ErrorBudget {
	b := model.ErrorBudget{Residual: resid(in), Allowance: allowance}
	for _, f := range chartFields {
		hw := f.halfWidth(prec)
		if hw <= 0 || *f.value(&in) <= 0 {
			continue
		}
		up, down := in, in
		*f.value(&up) += hw
		*f.value(&down) -= hw
		contribution := math.Max(math.Abs(resid(up)-b.Residual), math.Abs(resid(down)-b.Residual))
		if !(contribution > minBudgetContributionMm) {
			continue
		}
		b.Terms = append(b.Terms, model.BudgetTerm{
			Name:           f.name,
			HalfWidth:      hw,
			IsAngle:        f.isAngle,
			ContributionMm: contribution,
		})
		if f.isAngle {
			b.AngleTol += contribution
		} else {
			b.LengthTol += contribution
		}
	}
	sort.SliceStable(b.Terms, func(i, j int) bool {
		return b.Terms[i].ContributionMm > b.Terms[j].ContributionMm
	})
	return b
}

// impliedFrontCenterHoriz is the horizontal front-center implied by Reach, Stack,
// BB drop, HTA and rake. The head tube length cancels: the front end is as long
// as the published Stack says it is.
func impliedFrontCenterHoriz(in chartInputs, rake float64) float64 {
	frontEnd, ok := frontEndLength(in.stack, in.bbDrop, in.hta(), rake)
	if !ok {
		return math.NaN()
	}
	return CalculateFrontCenter(in.reach, in.htl, in.hta(), rake, frontEnd-in.htl)
}

func allowanceOf(v model.Opt[float64]) float64 {
	return math.Max(0, v.OrZero())
}

// A2cBudget is the rounding budget for the Master Stack Equation check.
// Residual is the declared CAD fork A2C minus the fork A2C implied by the
// published Stack, BB drop, HTL and HTA (with the declared rake and CAD spacer,
// which are hypotheses rather than rounded chart numbers).
// ok is false when the bike has no CAD fork A2C or the steerer is degenerate.
func A2cBudget(bike model.BikeGeometry) (budget model.ErrorBudget, ok bool) {
	forkA2c := bike.GetCadA2c()
	rake := bike.ForkRake
	spacer := bike.GetGeoHeadsetSpacerMm()
	in := chartInputsOf(bike)
	if forkA2c <= 0 || in.htl <= 0 {
		return model.ErrorBudget{}, false
	}
	if _, ok := frontEndLength(in.stack, in.bbDrop, in.hta(), rake); !ok {
		return model.ErrorBudget{}, false
	}
	resid := func(c chartInputs) float64 {
		frontEnd, ok := frontEndLength(c.stack, c.bbDrop, c.hta(), rake)
		if !ok {
			return math.NaN()
		}
		return forkA2c - (frontEnd - c.htl - spacer)
	}
	return buildBudget(in, ChartPrecisionFor(bike), allowanceOf(bike.GetGeoSlopMm().A2c), resid), true
}

// FrontCenterBudget is the rounding budget for the published Front Center check.
// Residual is the 3D front-center implied by Reach, Stack, BB drop and HTA minus
// the published Front Center. ok is false when no Front Center is published.
func FrontCenterBudget(bike model.BikeGeometry) (budget model.ErrorBudget, ok bool) {
	rake := bike.ForkRake
	in := chartInputsOf(bike)
	if in.frontCenter <= 0 || math.IsNaN(impliedFrontCenterHoriz(in, rake)) {
		return model.ErrorBudget{}, false
	}
	resid := func(c chartInputs) float64 {
		return math.Hypot(impliedFrontCenterHoriz(c, rake), c.bbDrop) - c.frontCenter
	}
	return buildBudget(in, ChartPrecisionFor(bike), allowanceOf(bike.GetGeoSlopMm().FrontCenter), resid), true
}

// WheelbaseBudget is the rounding budget for the Wheelbase check. Residual is the
// horizontal rear center (from Chainstay and BB drop) plus the implied horizontal
// front center, minus the published Wheelbase. ok is false when Wheelbase or
// Chainstay is not published.
func WheelbaseBudget(bike model.BikeGeometry) (budget model.ErrorBudget, ok bool) {
	rake := bike.ForkRake
	in := chartInputsOf(bike)
	if in.wheelbase <= 0 || in.chainstay <= 0 || math.IsNaN(impliedFrontCenterHoriz(in, rake)) {
		return model.ErrorBudget{}, false
	}
	resid := func(c chartInputs) float64 {
		return RearCenterHoriz(c.chainstay, c.bbDrop) + impliedFrontCenterHoriz(c, rake) - c.wheelbase
	}
	return buildBudget(in, ChartPrecisionFor(bike), allowanceOf(bike.GetGeoSlopMm().Wheelbase), resid), true
}

// RearCenterHoriz is the horizontal distance from the bottom bracket back to the
// rear axle for a chainstay length measured point-to-point.
func RearCenterHoriz(chainstay, bbDrop float64) float64 {
	return math.Sqrt(math.Max(0, chainstay*chainstay-bbDrop*bbDrop))
}
