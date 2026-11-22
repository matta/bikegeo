package catalog_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
)

var wheels = assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm}

func mustFrame(t *testing.T, id string) catalog.Frame {
	t.Helper()
	f, ok := catalog.FrameByID(id)
	if !ok {
		t.Fatalf("FrameByID(%q) not found", id)
	}
	return f
}

func mustSize(t *testing.T, f catalog.Frame, name string) catalog.FrameSize {
	t.Helper()
	s, ok := f.Size(name)
	if !ok {
		t.Fatalf("%s has no size %q", f.ID, name)
	}
	return s
}

// TestFramesRegistry checks the 19 frames, their order, and their size counts.
func TestFramesRegistry(t *testing.T) {
	want := map[string]int{
		"binary-havok": 5, "binary-kinetik": 6, "esker-hayduke": 4, "fairlight-holt-2": 4,
		"moots-mountaineer": 4, "otso-fenrir-stainless": 5, "revel-el-jefe": 4, "ritchey-ultra": 4,
		"salsa-cutthroat": 5, "salsa-fargo": 5, "seven-custom": 1, "seven-expedition-926d": 24,
		"seven-expedition-926f": 24, "sklar-pbj": 5, "sour-pasta-party": 4, "surly-ecr": 3,
		"surly-krampus": 4, "tumbleweed-sunliner": 4, "turner-venn-ti": 4,
	}
	frames := catalog.Frames()
	if len(frames) != 19 {
		t.Fatalf("got %d frames, want 19", len(frames))
	}
	total := 0
	for i, f := range frames {
		if i > 0 && frames[i-1].ID >= f.ID {
			t.Errorf("frame IDs not strictly ascending: %q then %q", frames[i-1].ID, f.ID)
		}
		if n, ok := want[f.ID]; !ok {
			t.Errorf("unexpected frame %q", f.ID)
		} else if len(f.Sizes) != n {
			t.Errorf("%s has %d sizes, want %d", f.ID, len(f.Sizes), n)
		}
		total += len(f.Sizes)
	}
	if total != 119 {
		t.Errorf("got %d sizes, want 119", total)
	}
	if _, ok := catalog.FrameByID("otso-fenrir-stainless"); !ok {
		t.Error(`FrameByID("otso-fenrir-stainless") not found`)
	}
	if _, ok := catalog.FrameByID("Otso-Fenrir-Stainless"); ok {
		t.Error(`FrameByID("Otso-Fenrir-Stainless") found; want an exact match only`)
	}
}

// frameProblems reports structural mistakes in a frame's declaration.
func frameProblems(f catalog.Frame) []string {
	var problems []string
	if f.ID == "" {
		problems = append(problems, "empty ID")
	}
	if len(f.Sizes) == 0 {
		problems = append(problems, "no sizes")
	}
	sizes := map[string]bool{}
	for _, s := range f.Sizes {
		if sizes[s.Name] {
			problems = append(problems, fmt.Sprintf("duplicate size %q", s.Name))
		}
		sizes[s.Name] = true
	}
	if !sizes[f.DefaultSize] {
		problems = append(problems, fmt.Sprintf("default size %q is not among the sizes", f.DefaultSize))
	}
	if len(f.Forks) == 0 {
		problems = append(problems, "no forks")
	}
	forks := map[string]bool{}
	for _, ff := range f.Forks {
		if forks[ff.Fork.ID] {
			problems = append(problems, fmt.Sprintf("duplicate fork %q", ff.Fork.ID))
		}
		forks[ff.Fork.ID] = true
	}
	if f.MaxChainstayMm != 0 {
		longest := 0.0
		for _, s := range f.Sizes {
			longest = math.Max(longest, s.ChartFrame.ChainstayMm)
		}
		if f.MaxChainstayMm < longest {
			problems = append(problems, fmt.Sprintf("max chainstay %v mm is below a published %v mm chainstay", f.MaxChainstayMm, longest))
		}
	}
	if f.BasisFork.Steerer != assembly.SteererCAD {
		problems = append(problems, fmt.Sprintf("basis fork steerer is %q, want %q", f.BasisFork.Steerer, assembly.SteererCAD))
	}
	if f.BasisFork.TravelMm != 0 {
		problems = append(problems, fmt.Sprintf("basis fork has %v mm travel, want 0", f.BasisFork.TravelMm))
	}
	return problems
}

func TestFrameStructure(t *testing.T) {
	for _, f := range catalog.Frames() {
		if problems := frameProblems(f); len(problems) > 0 {
			t.Errorf("%s: %s", f.ID, strings.Join(problems, "; "))
		}
	}
}

// TestFrameProblemsCatchMistakes shows frameProblems catches each mistake.
func TestFrameProblemsCatchMistakes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*catalog.Frame)
		want   string
	}{
		{"duplicate size", func(f *catalog.Frame) { f.Sizes = append(f.Sizes, f.Sizes[3]) }, "duplicate size"},
		{"unknown default size", func(f *catalog.Frame) { f.DefaultSize = "XXL" }, "default size"},
		{"no forks", func(f *catalog.Frame) { f.Forks = nil }, "no forks"},
		{"duplicate fork", func(f *catalog.Frame) { f.Forks[1] = f.Forks[0] }, "duplicate fork"},
		{"max chainstay below a size", func(f *catalog.Frame) { f.MaxChainstayMm = 440 }, "max chainstay"},
		{"basis fork non-basis steerer", func(f *catalog.Frame) { f.BasisFork.Steerer = assembly.SteererTapered }, "basis fork steerer"},
		{"basis fork with travel", func(f *catalog.Frame) { f.BasisFork.TravelMm = 120 }, "basis fork has"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mustFrame(t, "otso-fenrir-stainless")
			if f.Sizes[3].Name != "L" {
				t.Fatalf("Fenrir size 3 = %q, want L", f.Sizes[3].Name)
			}
			tc.mutate(&f)
			problems := frameProblems(f)
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Errorf("frameProblems = %q, want exactly one containing %q", problems, tc.want)
			}
		})
	}
}

// TestEveryChartIsConsistent runs the chart-consistency check on every size (Q91).
func TestEveryChartIsConsistent(t *testing.T) {
	for _, f := range catalog.Frames() {
		for _, s := range f.Sizes {
			t.Run(f.ID+"/"+s.Name, func(t *testing.T) {
				if err := checkChartConsistency(f, s); err != nil {
					t.Error(err)
				}
			})
		}
	}
}

// TestChartAllowances checks exactly the two Q71 allowances exist and are needed (Q93).
func TestChartAllowances(t *testing.T) {
	want := map[string]struct {
		allowance catalog.ChartAllowance
		errPrefix string
	}{
		"sklar-pbj/L": {
			catalog.ChartAllowance{AxleHeightMm: 0.4},
			"chart consistency: axle height mismatch -3.06 mm exceeds its ±2.68 mm rounding budget",
		},
		"esker-hayduke/S4": {
			catalog.ChartAllowance{WheelbaseMm: 1.2},
			"chart consistency: wheelbase mismatch +5.42 mm exceeds its ±4.31 mm rounding budget",
		},
	}
	found := 0
	for _, f := range catalog.Frames() {
		for _, s := range f.Sizes {
			key := f.ID + "/" + s.Name
			a := s.Allowance
			w, ok := want[key]
			if !ok {
				if a != (catalog.ChartAllowance{}) {
					t.Errorf("%s has unexpected allowance %+v", key, a)
				}
				continue
			}
			found++
			if a.AxleHeightMm != w.allowance.AxleHeightMm || a.WheelbaseMm != w.allowance.WheelbaseMm {
				t.Errorf("%s allowance = %+v, want %+v", key, a, w.allowance)
			}
			if strings.TrimSpace(a.Reason) == "" {
				t.Errorf("%s allowance has a blank reason", key)
			}
			noAllowance := s
			noAllowance.Allowance = catalog.ChartAllowance{}
			err := checkChartConsistency(f, noAllowance)
			if err == nil || !strings.HasPrefix(err.Error(), w.errPrefix) {
				t.Errorf("%s without its allowance: err = %v, want prefix %q", key, err, w.errPrefix)
			}
		}
	}
	if found != len(want) {
		t.Errorf("found %d allowance sizes, want %d", found, len(want))
	}
}

// TestForkListsPlace places every size with every listed fork at every
// enumerated sag, plus its CAD baseline fork (Q83).
func TestForkListsPlace(t *testing.T) {
	placements, bases := 0, 0
	for _, f := range catalog.Frames() {
		for _, s := range f.Sizes {
			installed := f.InstalledFrame(s)
			for _, ff := range f.Forks {
				sags := []float64{0}
				if ff.Fork.TravelMm != 0 {
					sags = catalog.SuspensionSagPcts()
				}
				for _, sag := range sags {
					fork := assembly.ForkSetup{Spec: ff.Spec(), SagPct: sag}
					if _, err := assembly.PlaceFrameset(installed, s.ChartFrame.DefaultLowerCup, fork, wheels); err != nil {
						t.Errorf("%s/%s with %s at %v%% sag: %v", f.ID, s.Name, ff.Fork.ID, sag, err)
					}
					placements++
				}
			}
			if _, err := assembly.PlaceFrameset(s.ChartFrame, f.BasisLowerCup, assembly.ForkSetup{Spec: f.BasisFork}, wheels); err != nil {
				t.Errorf("%s/%s CAD baseline fork: %v", f.ID, s.Name, err)
			}
			bases++
		}
	}
	t.Logf("%d fork-list placements, %d CAD baseline placements", placements, bases)
	if placements != 701 || bases != 119 {
		t.Errorf("got %d fork-list and %d CAD baseline placements, want 701 and 119", placements, bases)
	}
}

// TestForkListsPlaceChecksSteerers shows the placements in TestForkListsPlace
// enforce SHIS: a tapered fork does not fit the EC34 Surly ECR.
func TestForkListsPlaceChecksSteerers(t *testing.T) {
	f := mustFrame(t, "surly-ecr")
	s := mustSize(t, f, "L")
	sid := catalog.Forks()[0]
	if sid.ID != "sid_100" {
		t.Fatalf("first fork = %q, want sid_100", sid.ID)
	}
	ff := catalog.FrameFork{Fork: sid, StaticA2cMm: 506, RakeMm: 44}
	_, err := assembly.PlaceFrameset(f.InstalledFrame(s), s.ChartFrame.DefaultLowerCup, assembly.ForkSetup{Spec: ff.Spec(), SagPct: 20}, wheels)
	if err == nil || !strings.Contains(err.Error(), "does not fit a tapered steerer") {
		t.Errorf("err = %v, want one containing %q", err, "does not fit a tapered steerer")
	}
}

// TestSeatpostBoundsEverySize checks every size takes every seatpost.
func TestSeatpostBoundsEverySize(t *testing.T) {
	for _, f := range catalog.Frames() {
		for _, s := range f.Sizes {
			for _, p := range catalog.Seatposts() {
				if _, err := assembly.SeatpostInsertionBounds(s.ChartFrame, p.Spec); err != nil {
					t.Errorf("%s/%s with %s: %v", f.ID, s.Name, p.ID, err)
				}
			}
		}
	}
	posts := map[string]assembly.SeatpostSpec{}
	for _, p := range catalog.Seatposts() {
		posts[p.ID] = p.Spec
	}
	for _, tc := range []struct {
		frame, size, post string
		lo, hi            float64
	}{
		{"otso-fenrir-stainless", "L", "setback_0", 100, 290},
		{"otso-fenrir-stainless", "L", "redshift_shockstop_pro", 100, 250},
		{"salsa-fargo", "XS", "setback_0", 100, 335.6},
		// post-limited: the frame cap is C-C 355 + 35 − 20 = 370 (Q75d)
		{"seven-expedition-926d", "S 52cm", "setback_0", 100, 365},
	} {
		s := mustSize(t, mustFrame(t, tc.frame), tc.size)
		b, err := assembly.SeatpostInsertionBounds(s.ChartFrame, posts[tc.post])
		if err != nil {
			t.Errorf("%s/%s with %s: %v", tc.frame, tc.size, tc.post, err)
			continue
		}
		if math.Abs(b.EffectiveMinInsertionMm-tc.lo) > 1e-9 || math.Abs(b.MaxAllowedInsertionMm-tc.hi) > 1e-9 {
			t.Errorf("%s/%s with %s: bounds [%v, %v], want [%v, %v]", tc.frame, tc.size, tc.post, b.EffectiveMinInsertionMm, b.MaxAllowedInsertionMm, tc.lo, tc.hi)
		}
	}
}

// TestInstalledFrameUsesMaxChainstay checks sliding dropouts evaluate at max chainstay (Q67).
func TestInstalledFrameUsesMaxChainstay(t *testing.T) {
	for _, tc := range []struct {
		frame, size        string
		chart, installedCs float64
	}{
		{"esker-hayduke", "S4", 440, 454},
		{"otso-fenrir-stainless", "L", 450, 450},
		{"ritchey-ultra", "L", 0, 0}, // fixed dropouts: the chart's own chainstay
	} {
		f := mustFrame(t, tc.frame)
		s := mustSize(t, f, tc.size)
		got := f.InstalledFrame(s)
		if tc.chart != 0 {
			if s.ChartFrame.ChainstayMm != tc.chart || got.ChainstayMm != tc.installedCs {
				t.Errorf("%s/%s chainstay chart %v installed %v, want %v and %v", tc.frame, tc.size, s.ChartFrame.ChainstayMm, got.ChainstayMm, tc.chart, tc.installedCs)
			}
		}
		want := s.ChartFrame
		if tc.chart == 0 {
			if got != want {
				t.Errorf("%s/%s installed frame %+v, want the chart frame %+v", tc.frame, tc.size, got, want)
			}
			continue
		}
		want.ChainstayMm = got.ChainstayMm
		if got != want {
			t.Errorf("%s/%s installed frame differs from the chart beyond ChainstayMm: %+v vs %+v", tc.frame, tc.size, got, want)
		}
	}
}

func TestDefaultFrameID(t *testing.T) {
	f, ok := catalog.FrameByID(catalog.DefaultFrameID)
	if !ok {
		t.Fatalf("FrameByID(DefaultFrameID) not found")
	}
	if f.ID != "otso-fenrir-stainless" || f.DefaultSize != "L" {
		t.Errorf("default frame = %q size %q, want otso-fenrir-stainless L", f.ID, f.DefaultSize)
	}
}
