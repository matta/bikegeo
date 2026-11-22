package catalog_test

import (
	"fmt"
	"math"
	"slices"
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

// TestFramesRegistry checks the 18 frames, their order, and their size counts.
func TestFramesRegistry(t *testing.T) {
	want := map[string]int{
		"binary-havok": 5, "binary-kinetik": 6, "esker-hayduke": 4, "fairlight-holt-2": 4,
		"moots-mountaineer": 4, "otso-fenrir-stainless": 5, "revel-el-jefe": 4,
		"salsa-cutthroat": 5, "salsa-fargo": 5, "seven-custom": 1, "seven-expedition-926d": 24,
		"seven-expedition-926f": 24, "sklar-pbj": 5, "sour-pasta-party": 4, "surly-ecr": 3,
		"surly-krampus": 4, "tumbleweed-sunliner": 4, "turner-venn-ti": 4,
	}
	frames := catalog.Frames()
	if len(frames) != 18 {
		t.Fatalf("got %d frames, want 18", len(frames))
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
	if total != 115 {
		t.Errorf("got %d sizes, want 115", total)
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
	for _, fk := range f.Forks {
		if forks[fk.ID] {
			problems = append(problems, fmt.Sprintf("duplicate fork %q", fk.ID))
		}
		forks[fk.ID] = true
		if len(fk.Variants) == 0 {
			problems = append(problems, fmt.Sprintf("fork %q has no variants", fk.ID))
		}
		if !fk.RigidClass.ValidFor(fk.Travel) {
			problems = append(problems, fmt.Sprintf("fork %q has rigid class %q for travel %v", fk.ID, fk.RigidClass, fk.Travel))
		}
	}
	if len(f.Sizes) > 0 {
		first := f.Sizes[0].ChartFrame
		for _, sz := range f.Sizes {
			if sz.ChartFrame.LowerBore != first.LowerBore || sz.ChartFrame.DefaultLowerCup != first.DefaultLowerCup {
				problems = append(problems, fmt.Sprintf("size %q headset differs from size %q", sz.Name, f.Sizes[0].Name))
			}
		}
		if typical, ok := catalog.TypicalLowerCup(first.DefaultLowerCup.Standard); !ok || typical != first.DefaultLowerCup {
			problems = append(problems, fmt.Sprintf("default lower cup %+v is not the typical cup", first.DefaultLowerCup))
		}
		compatible := map[string]bool{}
		for _, fk := range f.CompatibleForks() {
			compatible[fk.ID] = true
		}
		for _, fk := range f.Forks {
			if _, ok := catalog.LowerCupFor(first.LowerBore, first.DefaultLowerCup.Standard, fk.Steerer); !ok {
				problems = append(problems, fmt.Sprintf("listed fork %q has no lower cup on %s", fk.ID, first.LowerBore))
			} else if !compatible[fk.ID] {
				problems = append(problems, fmt.Sprintf("listed fork %q is missing from the compatible forks", fk.ID))
			}
		}
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
	if f.BasisFork.Steerer != assembly.SteererBasis {
		problems = append(problems, fmt.Sprintf("basis fork steerer is %q, want %q", f.BasisFork.Steerer, assembly.SteererBasis))
	}
	if f.BasisFork.Travel != assembly.TravelRigid {
		problems = append(problems, fmt.Sprintf("basis fork has %v travel, want rigid", f.BasisFork.Travel))
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
		{"max chainstay below a size", func(f *catalog.Frame) { f.MaxChainstayMm = 430 }, "max chainstay"},
		{"size headset bore differs", func(f *catalog.Frame) { f.Sizes[1].ChartFrame.LowerBore = assembly.Bore56Pressed }, "headset differs"},
		{"size default cup differs", func(f *catalog.Frame) { f.Sizes[1].ChartFrame.DefaultLowerCup.StackMm = 5 }, "headset differs"},
		{"non-typical default cup", func(f *catalog.Frame) {
			for i := range f.Sizes {
				f.Sizes[i].ChartFrame.DefaultLowerCup.StackMm = 0
			}
		}, "not the typical cup"},
		{"rigid fork without class", func(f *catalog.Frame) {
			fk := f.Forks[0]
			fk.RigidClass = catalog.RigidClassNone
			f.Forks = append([]catalog.Fork{fk}, f.Forks[1:]...)
		}, "rigid class"},
		{"tapered fork on a 34 mm bore", func(f *catalog.Frame) {
			f.Forks = f.Forks[:1]
			for i := range f.Sizes {
				f.Sizes[i].ChartFrame.LowerBore = assembly.Bore34Pressed
				f.Sizes[i].ChartFrame.DefaultLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12}
			}
		}, "has no lower cup"},
		{"basis fork non-basis steerer", func(f *catalog.Frame) { f.BasisFork.Steerer = assembly.SteererTapered }, "basis fork steerer"},
		{"basis fork with travel", func(f *catalog.Frame) { f.BasisFork.Travel = assembly.Travel120 }, "basis fork has"},
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
// enumerated sag, plus its basis fork (Q83).
func TestForkListsPlace(t *testing.T) {
	placements, bases := 0, 0
	for _, f := range catalog.Frames() {
		for _, s := range f.Sizes {
			installed := f.InstalledFrame(s)
			for _, opt := range f.ForkOptions() {
				sags := []float64{0}
				if opt.Fork.Travel.IsSuspension() {
					sags = opt.Fork.Travel.SagPcts()
				}
				for _, sag := range sags {
					fork := assembly.ForkSetup{Spec: opt.Spec(), SagPct: sag}
					if _, err := assembly.PlaceFrameset(installed, opt.LowerCup, fork, wheels); err != nil {
						t.Errorf("%s/%s with %s at %v%% sag: %v", f.ID, s.Name, opt.ID(), sag, err)
					}
					placements++
				}
			}
			if _, err := assembly.PlaceFrameset(s.ChartFrame, f.BasisLowerCup, assembly.ForkSetup{Spec: f.BasisFork}, wheels); err != nil {
				t.Errorf("%s/%s basis fork: %v", f.ID, s.Name, err)
			}
			bases++
		}
	}
	t.Logf("%d fork-list placements, %d basis placements", placements, bases)
	if placements != 2889 || bases != 115 {
		t.Errorf("got %d fork-list and %d basis placements, want 2889 and 115", placements, bases)
	}
}

// TestForkListsPlaceChecksSteerers shows the placements in TestForkListsPlace
// enforce SHIS: a tapered fork does not fit the EC34 Surly ECR.
func TestForkListsPlaceChecksSteerers(t *testing.T) {
	f := mustFrame(t, "surly-ecr")
	s := mustSize(t, f, "L")
	sid, ok := catalog.ForkByID("sid_100")
	if !ok {
		t.Fatal("sid_100 missing from catalog")
	}
	opt := catalog.ForkOption{Fork: sid, Variant: sid.Variants[0]}
	_, err := assembly.PlaceFrameset(f.InstalledFrame(s), s.ChartFrame.DefaultLowerCup, assembly.ForkSetup{Spec: opt.Spec(), SagPct: 20}, wheels)
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
		{"otso-fenrir-stainless", "XS", "setback_0", 100, 170},
		{"otso-fenrir-stainless", "XS", "redshift_shockstop_pro", 100, 170},
		{"otso-fenrir-stainless", "S", "setback_0", 100, 210},
		{"otso-fenrir-stainless", "S", "redshift_shockstop_pro", 100, 210},
		{"otso-fenrir-stainless", "M", "setback_0", 100, 260},
		{"otso-fenrir-stainless", "M", "redshift_shockstop_pro", 100, 250},
		{"otso-fenrir-stainless", "L", "setback_0", 100, 290},
		{"otso-fenrir-stainless", "L", "redshift_shockstop_pro", 100, 250},
		{"otso-fenrir-stainless", "XL", "setback_0", 100, 330},
		{"otso-fenrir-stainless", "XL", "redshift_shockstop_pro", 100, 250},
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
		{"salsa-fargo", "L", 462, 462},
		{"otso-fenrir-stainless", "L", 0, 0}, // Tuning Chip frame stays at chart chainstay (Q140)
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

// compatibleIDs returns the fork model IDs a frame accepts, in order.
func compatibleIDs(f catalog.Frame) []string {
	var ids []string
	for _, fk := range f.CompatibleForks() {
		ids = append(ids, fk.ID)
	}
	return ids
}

// TestCompatibleForksRules checks the Q128, Q135, Q136, and Q139 rules.
func TestCompatibleForksRules(t *testing.T) {
	cutthroat := compatibleIDs(mustFrame(t, "salsa-cutthroat"))
	wantCutthroat := []string{"firestarter_deluxe_483", "sid_100", "surly_krampus_483", "sklar_steel_485"}
	if !slices.Equal(cutthroat, wantCutthroat) {
		t.Errorf("salsa-cutthroat compatible forks = %v, want %v", cutthroat, wantCutthroat)
	}
	for _, id := range []string{"fox34_120", "fox34_130", "enve_mountain_boost", "lithic_carbon_mountain"} {
		if slices.Contains(cutthroat, id) {
			t.Errorf("salsa-cutthroat accepts %q; a 100 mm and 485 listing must not unlock it", id)
		}
	}

	seven := mustFrame(t, "seven-expedition-926d")
	models, variants := 0, 0
	for _, fk := range seven.CompatibleForks() {
		models++
		variants += len(fk.Variants)
		if fk.Travel == assembly.TravelRigid && fk.RigidClass != catalog.RigidClass495 {
			t.Errorf("926d accepts rigid fork %q of class %q", fk.ID, fk.RigidClass)
		}
		if fk.Travel == assembly.Travel100 || fk.Travel == assembly.Travel130 {
			t.Errorf("926d accepts %v mm fork %q", fk.Travel, fk.ID)
		}
	}
	if models != 7 || variants != 11 || len(seven.ForkOptions()) != 11 {
		t.Errorf("926d has %d models, %d variants, %d options; want 7, 11, 11", models, variants, len(seven.ForkOptions()))
	}

	for id, only := range map[string]string{"surly-ecr": "surly_ecr_468", "tumbleweed-sunliner": "sunliner_440"} {
		if got := compatibleIDs(mustFrame(t, id)); !slices.Equal(got, []string{only}) {
			t.Errorf("%s compatible forks = %v, want only %s (rigid lists never unlock suspension)", id, got, only)
		}
	}

	for _, f := range catalog.Frames() {
		got := compatibleIDs(f)
		for i, fk := range f.Forks {
			if got[i] != fk.ID {
				t.Errorf("%s compatible fork %d = %q, want listed %q first", f.ID, i, got[i], fk.ID)
			}
			if !f.IsListedFork(fk.ID) {
				t.Errorf("%s IsListedFork(%q) = false", f.ID, fk.ID)
			}
		}
		for _, id := range got[len(f.Forks):] {
			if f.IsListedFork(id) {
				t.Errorf("%s lists %q yet it is not first", f.ID, id)
			}
		}
	}
	if mustFrame(t, "surly-krampus").IsListedFork("pike_120") {
		t.Error("surly-krampus lists pike_120, want only compatible")
	}

	fenrir := mustFrame(t, "otso-fenrir-stainless")
	fenrirGot := compatibleIDs(fenrir)
	wantFenrirPrefix := []string{"lithic_carbon_mountain", "enve_mountain_boost", "sid_100", "fox34_120"}
	if len(fenrirGot) < len(wantFenrirPrefix) || !slices.Equal(fenrirGot[:len(wantFenrirPrefix)], wantFenrirPrefix) {
		t.Errorf("otso-fenrir-stainless compatible forks prefix = %v, want %v", fenrirGot, wantFenrirPrefix)
	}
	if !fenrir.IsListedFork("enve_mountain_boost") {
		t.Error("otso-fenrir-stainless IsListedFork(enve_mountain_boost) = false, want true (Q128)")
	}
}

// TestForkOptionsResolveLowerCups checks per-fork cups on a ZS44 frame (Q136).
func TestForkOptionsResolveLowerCups(t *testing.T) {
	f := mustFrame(t, "surly-krampus")
	sid, _ := catalog.ForkByID("sid_100")
	for i := range f.Sizes {
		f.Sizes[i].ChartFrame.DefaultLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: 4}
	}
	f.Forks = []catalog.Fork{sid}
	want := map[string]assembly.LowerCupSetup{
		"surly_krampus_483": {Standard: assembly.LowerCupZS44, StackMm: 4},
		"sid_100":           {Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	for _, opt := range f.ForkOptions() {
		if w, ok := want[opt.Fork.ID]; ok && opt.LowerCup != w {
			t.Errorf("%s cup = %+v, want %+v", opt.Fork.ID, opt.LowerCup, w)
		}
	}
	if opts, ok := f.LookupFork("surly_krampus_483"); !ok || len(opts) != 1 || opts[0].LowerCup != want["surly_krampus_483"] {
		t.Errorf("LookupFork(surly_krampus_483) = %+v, %v", opts, ok)
	}
	if _, ok := f.LookupFork("rigid_485"); ok {
		t.Error("LookupFork(rigid_485) found a deleted fork")
	}
}
