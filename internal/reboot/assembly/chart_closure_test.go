package assembly_test

import (
	"math"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
)

func fenrirChart() assembly.FrameChart {
	return assembly.FrameChart{
		Frame:                fenrirSizeLFrame(),
		CadLowerCup:          fenrirCadCup(),
		CadFork:              assembly.ForkGeometry{A2cMm: 495.0, RakeMm: 54.0},
		PublishedWheelbaseMm: 1158.0,
		Precision: assembly.ChartPrecision{
			ReachMm: 0.5, StackMm: 0.5, HeadTubeAngleDeg: 0.125, HeadTubeLengthMm: 0.5,
			BbDropMm: 0.5, ChainstayMm: 0.5, WheelbaseMm: 0.5,
		},
	}
}

func TestCheckChartClosureFenrirPasses(t *testing.T) {
	if err := assembly.CheckChartClosure(fenrirChart()); err != nil {
		t.Fatalf("CheckChartClosure(fenrirChart()): %v", err)
	}
}

func TestCheckChartClosureBudgetEdges(t *testing.T) {
	cases := []struct {
		name  string
		chart func() assembly.FrameChart
		want  string
	}{
		{
			name: "axle height inside",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.Frame.StackMm = 643.8
				return c
			},
			want: "",
		},
		{
			name: "axle height outside",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.Frame.StackMm = 643.95
				return c
			},
			want: "chart closure: axle height residual +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)",
		},
		{
			name: "wheelbase inside",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.PublishedWheelbaseMm = 1160.6
				return c
			},
			want: "",
		},
		{
			name: "wheelbase outside",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.PublishedWheelbaseMm = 1160.8
				return c
			},
			want: "chart closure: wheelbase residual -3.14 mm exceeds its ±3.02 mm rounding budget (Reach 0.50 mm, HTA 1.24 mm, HTL 0.19 mm, BB drop 0.08 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assembly.CheckChartClosure(tc.chart())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("CheckChartClosure: got %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckChartClosure: got nil, want %q", tc.want)
			}
			if err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestCheckChartClosureWithoutPrecision(t *testing.T) {
	c := fenrirChart()
	c.Precision = assembly.ChartPrecision{}
	err := assembly.CheckChartClosure(c)
	want := "chart closure: axle height residual +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)\nchart closure: wheelbase residual -0.34 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"
	if err == nil {
		t.Fatalf("CheckChartClosure: got nil, want %q", want)
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestCheckChartClosureInvalidPrecision(t *testing.T) {
	cases := []struct {
		name  string
		chart func() assembly.FrameChart
		want  string
	}{
		{
			name: "NaN Stack",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.Precision.StackMm = math.NaN()
				return c
			},
			want: "chart precision: Stack half-width must be finite and non-negative, got NaN",
		},
		{
			name: "negative HTA",
			chart: func() assembly.FrameChart {
				c := fenrirChart()
				c.Precision.HeadTubeAngleDeg = -0.1
				return c
			},
			want: "chart precision: HTA half-width must be finite and non-negative, got -0.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assembly.CheckChartClosure(tc.chart())
			if err == nil {
				t.Fatalf("CheckChartClosure: got nil, want %q", tc.want)
			}
			if err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestCheckChartClosureNudgeOutOfRange(t *testing.T) {
	c := fenrirChart()
	c.Frame.ChainstayMm = 72.4
	c.Precision = assembly.ChartPrecision{ChainstayMm: 0.5}
	err := assembly.CheckChartClosure(c)
	if err == nil {
		t.Fatal("CheckChartClosure: got nil, want out-of-range error")
	}
	const wantPrefix = "chart precision: Chainstay ±0.5 moves the chart out of range: "
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("error = %q, want prefix %q", err.Error(), wantPrefix)
	}
}

func TestCheckChartClosureInvalidWheelbase(t *testing.T) {
	const want = "published wheelbase must be finite and non-negative (0 = unpublished)"
	for _, wb := range []float64{-1, math.NaN()} {
		c := fenrirChart()
		c.PublishedWheelbaseMm = wb
		err := assembly.CheckChartClosure(c)
		if err == nil {
			t.Fatalf("PublishedWheelbaseMm = %v: got nil, want error", wb)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("PublishedWheelbaseMm = %v: error = %q, want substring %q", wb, err.Error(), want)
		}
	}
}

// The Hayduke and Sklar dimensions are the published values copied from the
// legacy charts. Their precision is inferred from published digits (Q80).
var legacyChartPrecision = assembly.ChartPrecision{
	ReachMm: 0.5, StackMm: 0.5, HeadTubeAngleDeg: 0.25, HeadTubeLengthMm: 0.5,
	BbDropMm: 0.5, ChainstayMm: 0.5, WheelbaseMm: 0.5,
}

// haydukeS4Chart uses a 12 mm EC44 CAD cup and a Pike 120 CAD fork at 30%
// sag, giving 495 mm effective axle-to-crown and 44 mm rake.
func haydukeS4Chart() assembly.FrameChart {
	cup := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	return assembly.FrameChart{
		Frame: assembly.FrameSetup{
			ReachMm: 500, StackMm: 645, HeadTubeAngleDeg: 67.5, SeatTubeAngleDeg: 74,
			HeadTubeLengthMm: 140, BbDropMm: 65, ChainstayMm: 440, SeatTubeLengthMm: 530,
			LowerBore: assembly.Bore44Pressed, DefaultLowerCup: cup,
		},
		CadLowerCup:          cup,
		CadFork:              assembly.ForkGeometry{A2cMm: 495, RakeMm: 44},
		PublishedWheelbaseMm: 1218,
		Precision:            legacyChartPrecision,
	}
}

// sklarLChart uses a 12 mm EC44 CAD cup and the Sklar steel 485 mm CAD fork
// with 52 mm rake.
func sklarLChart() assembly.FrameChart {
	cup := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	return assembly.FrameChart{
		Frame: assembly.FrameSetup{
			ReachMm: 475, StackMm: 641, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 74,
			HeadTubeLengthMm: 140, BbDropMm: 68, ChainstayMm: 432, SeatTubeLengthMm: 505,
			LowerBore: assembly.Bore44Pressed, DefaultLowerCup: cup,
		},
		CadLowerCup:          cup,
		CadFork:              assembly.ForkGeometry{A2cMm: 485, RakeMm: 52},
		PublishedWheelbaseMm: 1182,
		Precision:            legacyChartPrecision,
	}
}

func TestCheckChartClosureHaydukeS4Allowance(t *testing.T) {
	const reason = "The declared front axle position leaves a wheelbase discrepancy beyond rounding."
	const terms = " (Reach 0.50 mm, HTA 2.54 mm, HTL 0.19 mm, BB drop 0.07 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)"
	const failure = "chart closure: wheelbase residual +5.42 mm exceeds its ±4.31 mm rounding budget"
	cases := []struct {
		name      string
		allowance assembly.ClosureAllowance
		want      string
	}{
		{"none", assembly.ClosureAllowance{}, failure + terms},
		{"documented", assembly.ClosureAllowance{WheelbaseMm: 1.2, Reason: reason}, ""},
		{"too small", assembly.ClosureAllowance{WheelbaseMm: 1.0, Reason: reason}, failure + " plus its 1.00 mm allowance" + terms},
		{"inside slack", assembly.ClosureAllowance{WheelbaseMm: 1.6, Reason: reason}, ""},
		{"outside slack", assembly.ClosureAllowance{WheelbaseMm: 1.7, Reason: reason}, "closure allowance: wheelbase allowance 1.70 mm exceeds the 1.11 mm needed beyond rounding by more than 0.50 mm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := haydukeS4Chart()
			chart.Allowance = tc.allowance
			requireChartClosure(t, chart, tc.want)
		})
	}
}

func TestCheckChartClosureSklarLAllowance(t *testing.T) {
	const reason = "The published chart leaves an axle height discrepancy beyond rounding."
	const terms = " (Stack 0.50 mm, HTA 1.21 mm, HTL 0.47 mm, BB drop 0.50 mm)"
	const failure = "chart closure: axle height residual -3.06 mm exceeds its ±2.68 mm rounding budget"
	cases := []struct {
		name      string
		allowance assembly.ClosureAllowance
		want      string
	}{
		{"none", assembly.ClosureAllowance{}, failure + terms},
		{"documented", assembly.ClosureAllowance{AxleHeightMm: 0.4, Reason: reason}, ""},
		{"too small", assembly.ClosureAllowance{AxleHeightMm: 0.3, Reason: reason}, failure + " plus its 0.30 mm allowance" + terms},
		{"unnecessary wheelbase allowance", assembly.ClosureAllowance{AxleHeightMm: 0.4, WheelbaseMm: 0.5, Reason: reason}, "closure allowance: wheelbase allowance 0.50 mm is unnecessary: residual -3.56 mm is within its ±4.28 mm rounding budget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := sklarLChart()
			chart.Allowance = tc.allowance
			requireChartClosure(t, chart, tc.want)
		})
	}
}

func TestCheckChartClosureUnnecessaryAllowance(t *testing.T) {
	chart := fenrirChart()
	chart.Allowance = assembly.ClosureAllowance{AxleHeightMm: 0.5, Reason: "Axle height discrepancy."}
	requireChartClosure(t, chart, "closure allowance: axle height allowance 0.50 mm is unnecessary: residual +0.22 mm is within its ±2.09 mm rounding budget")
}

func TestCheckChartClosureInvalidAllowance(t *testing.T) {
	cases := []struct {
		name        string
		allowance   assembly.ClosureAllowance
		unpublished bool
		want        string
	}{
		{"negative axle height", assembly.ClosureAllowance{AxleHeightMm: -0.1, Reason: "x"}, false, "closure allowance: axle height must be finite and non-negative, got -0.1 mm"},
		{"NaN wheelbase", assembly.ClosureAllowance{WheelbaseMm: math.NaN(), Reason: "x"}, false, "closure allowance: wheelbase must be finite and non-negative, got NaN mm"},
		{"missing reason", assembly.ClosureAllowance{AxleHeightMm: 0.5, Reason: "  "}, false, "closure allowance: a non-zero allowance needs a reason"},
		{"reason only", assembly.ClosureAllowance{Reason: "x"}, false, "closure allowance: a reason without an allowance"},
		{"unpublished wheelbase", assembly.ClosureAllowance{WheelbaseMm: 1.0, Reason: "x"}, true, "closure allowance: wheelbase allowance on a chart with no published wheelbase"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chart := fenrirChart()
			chart.Allowance = tc.allowance
			if tc.unpublished {
				chart.PublishedWheelbaseMm = 0
			}
			requireChartClosure(t, chart, tc.want)
		})
	}
}

func TestCheckChartClosureUnpublishedWheelbase(t *testing.T) {
	chart := fenrirChart()
	chart.PublishedWheelbaseMm = 0
	t.Run("rounding covers axle height", func(t *testing.T) {
		requireChartClosure(t, chart, "")
	})
	t.Run("axle height outside budget", func(t *testing.T) {
		chart := chart
		chart.Frame.StackMm = 643.95
		requireChartClosure(t, chart, "chart closure: axle height residual +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)")
	})
	t.Run("no rounding", func(t *testing.T) {
		chart := chart
		chart.Precision = assembly.ChartPrecision{}
		requireChartClosure(t, chart, "chart closure: axle height residual +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)")
	})
}

func requireChartClosure(t *testing.T, chart assembly.FrameChart, want string) {
	t.Helper()
	err := assembly.CheckChartClosure(chart)
	if want == "" {
		if err != nil {
			t.Fatalf("CheckChartClosure: got %v, want nil", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("CheckChartClosure: got nil, want %q", want)
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
