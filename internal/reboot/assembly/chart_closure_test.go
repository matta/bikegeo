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
	for _, wb := range []float64{0, math.NaN()} {
		c := fenrirChart()
		c.PublishedWheelbaseMm = wb
		err := assembly.CheckChartClosure(c)
		if err == nil {
			t.Fatalf("PublishedWheelbaseMm = %v: got nil, want error", wb)
		}
		if !strings.Contains(err.Error(), "published wheelbase must be finite and positive") {
			t.Errorf("PublishedWheelbaseMm = %v: error = %q, want substring %q", wb, err.Error(), "published wheelbase must be finite and positive")
		}
	}
}
