package query_test

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/pose"
	"github.com/matta/bikegeo/internal/query"
)

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q): %v", raw, err)
	}
	return q
}

func mustParseCompare(t *testing.T, raw string) query.Compare {
	t.Helper()
	c, err := query.ParseCompare(mustParseQuery(t, raw))
	if err != nil {
		t.Fatalf("ParseCompare(%q): %v", raw, err)
	}
	return c
}

// trimFrames returns catalog frame frameID as a compare frame keeping only
// size and forkID (blank keeps every size or fork), so a test ranks a few rows
// instead of the whole catalog.
func trimFrames(t *testing.T, frameID, size, forkID string) []compare.Frame {
	t.Helper()
	for _, f := range query.CatalogFrames() {
		if f.ID != frameID {
			continue
		}
		if size != "" {
			f.Sizes = slices.DeleteFunc(f.Sizes, func(sz compare.Size) bool { return sz.Name != size })
		}
		if forkID != "" {
			f.Forks = slices.DeleteFunc(f.Forks, func(fk compare.Fork) bool { return fk.ID != forkID })
		}
		if len(f.Sizes) == 0 || len(f.Forks) == 0 {
			t.Fatalf("frame %q has no size %q or fork %q", frameID, size, forkID)
		}
		return []compare.Frame{f}
	}
	t.Fatalf("unknown frame %q", frameID)
	return nil
}

// bikeRow names one compare row: a frame, size, fork, and sag.
type bikeRow struct {
	frame, size, fork string
	sagPct            float64
}

// rankRow ranks raw's compare query over row's frame trimmed to its size and
// fork, keeping every sag, and returns the parsed query, the result, and the
// ranked row at row's sag.
func rankRow(t *testing.T, raw string, row bikeRow) (query.Compare, query.CompareResult, query.CompareRow) {
	t.Helper()
	c := mustParseCompare(t, raw)
	c.Request.Frames = trimFrames(t, row.frame, row.size, row.fork)
	c.Request.TopPerFrame = 0
	result := mustRankAndResolve(t, c)
	for _, r := range result.Rows {
		if r.SagPct == row.sagPct {
			return c, result, r
		}
	}
	t.Fatalf("%q: no %v%% sag row for %+v", raw, row.sagPct, row)
	return c, result, query.CompareRow{}
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (tol %g)", name, got, want, tol)
	}
}

// TestCatalogFrames checks the catalog-to-compare frame adapter.
func TestCatalogFrames(t *testing.T) {
	frames := catalog.Frames()
	got := query.CatalogFrames()
	if len(got) != 18 || len(got) != len(frames) {
		t.Fatalf("got %d frames, want 18 (catalog has %d)", len(got), len(frames))
	}
	totalSizes := 0
	for i, f := range frames {
		cf := got[i]
		if cf.ID != f.ID {
			t.Errorf("frame %d ID = %q, want %q", i, cf.ID, f.ID)
		}
		if len(cf.Sizes) != len(f.Sizes) {
			t.Fatalf("%s: got %d sizes, want %d", f.ID, len(cf.Sizes), len(f.Sizes))
		}
		for j, sz := range f.Sizes {
			if cf.Sizes[j].Name != sz.Name {
				t.Errorf("%s size %d = %q, want %q", f.ID, j, cf.Sizes[j].Name, sz.Name)
			}
			wantFrame, err := f.InstalledFrame(sz)
			if err != nil {
				t.Fatalf("%s/%s InstalledFrame: %v", f.ID, sz.Name, err)
			}
			if !reflect.DeepEqual(cf.Sizes[j].Frame.Dimensions(), wantFrame.Dimensions()) {
				t.Errorf("%s size %q frame differs from InstalledFrame", f.ID, sz.Name)
			}
		}
		totalSizes += len(cf.Sizes)
		forkOpts := f.ForkOptions()
		if len(cf.Forks) != len(forkOpts) {
			t.Fatalf("%s: got %d forks, want %d", f.ID, len(cf.Forks), len(forkOpts))
		}
		for j, opt := range forkOpts {
			if cf.Forks[j].ID != opt.ID() {
				t.Errorf("%s fork %d = %q, want %q", f.ID, j, cf.Forks[j].ID, opt.ID())
			}
			if cf.Forks[j].LowerCup != opt.LowerCup {
				t.Errorf("%s fork %q cup = %+v, want %+v", f.ID, opt.ID(), cf.Forks[j].LowerCup, opt.LowerCup)
			}
			if cf.Forks[j].Travel != opt.Fork.Travel {
				t.Errorf("%s fork %q travel = %v, want %v", f.ID, opt.ID(), cf.Forks[j].Travel, opt.Fork.Travel)
			}
			fk, err := cf.Forks[j].Build(opt.Fork.Travel.DefaultSagPct())
			if err != nil {
				t.Errorf("%s fork %q Build: %v", f.ID, opt.ID(), err)
			} else if fk.Steerer() != opt.Fork.Steerer {
				t.Errorf("%s fork %q steerer = %q, want %q", f.ID, opt.ID(), fk.Steerer(), opt.Fork.Steerer)
			}
		}
	}
	if totalSizes != 115 {
		t.Errorf("got %d sizes, want 115", totalSizes)
	}

	havok, ok := catalog.FrameByID("binary-havok")
	if !ok {
		t.Fatal("binary-havok missing from catalog")
	}
	sz, ok := havok.Size("L")
	if !ok {
		t.Fatal("binary-havok has no size L")
	}
	if sz.ChartDims.ChainstayMm != 445 {
		t.Errorf("binary-havok L published chainstay = %v, want 445", sz.ChartDims.ChainstayMm)
	}
	for _, cf := range got {
		if cf.ID != "binary-havok" {
			continue
		}
		for _, s := range cf.Sizes {
			if s.Name == "L" && s.Frame.Dimensions().ChainstayMm != 460 {
				t.Errorf("binary-havok L installed chainstay = %v, want 460", s.Frame.Dimensions().ChainstayMm)
			}
		}
	}
}

// TestParseCompareDefaults checks the catalog compare defaults.
func TestParseCompareDefaults(t *testing.T) {
	for _, raw := range []string{"", "top_per_frame=&top="} {
		c := mustParseCompare(t, raw)
		if c.Post.ID != "redshift_shockstop_pro" {
			t.Errorf("%q: Post.ID = %q", raw, c.Post.ID)
		}
		if c.Bar.ID != "persuader20_800" {
			t.Errorf("%q: Bar.ID = %q", raw, c.Bar.ID)
		}
		if c.AllBarOptions || c.OfferedBars != nil {
			t.Errorf("%q: AllBarOptions = %v, OfferedBars = %v; want false, nil", raw, c.AllBarOptions, c.OfferedBars)
		}
		if c.BrpTarget != pose.NewPoint(assembly.DynamicWorld, -219, 713.1) {
			t.Errorf("%q: BrpTarget = %+v", raw, c.BrpTarget)
		}
		if c.GripTarget != pose.NewPoint(assembly.DynamicWorld, 481, 726) {
			t.Errorf("%q: GripTarget = %+v", raw, c.GripTarget)
		}
		if c.TopPerFrame != 1 || c.Top != 0 {
			t.Errorf("%q: TopPerFrame = %d, Top = %d; want 1, 0", raw, c.TopPerFrame, c.Top)
		}
		if c.TopPerFrameExplicit || c.TopExplicit {
			t.Errorf("%q: TopPerFrameExplicit = %v, TopExplicit = %v; want false, false", raw, c.TopPerFrameExplicit, c.TopExplicit)
		}
		if c.Request.DynamicBarRoll != 0 || c.Request.DynamicRailPitch != 0 {
			t.Errorf("%q: DynamicBarRoll = %v, DynamicRailPitch = %v; want 0, 0", raw, c.Request.DynamicBarRoll, c.Request.DynamicRailPitch)
		}
		if len(c.Request.Frames) != len(catalog.Frames()) {
			t.Errorf("%q: got %d frames, want every catalog frame (%d)", raw, len(c.Request.Frames), len(catalog.Frames()))
		}
		if c.Request.HasBarOptions || len(c.Request.Handlebars) != 1 {
			t.Errorf("%q: HasBarOptions = %v, len(Handlebars) = %d; want false, 1", raw, c.Request.HasBarOptions, len(c.Request.Handlebars))
		}
		if len(c.Request.Stems) != 107 {
			t.Errorf("%q: got %d stems, want 107", raw, len(c.Request.Stems))
		}
		if c.Request.SpacerStepMm != 5 {
			t.Errorf("%q: SpacerStepMm = %v, want 5", raw, c.Request.SpacerStepMm)
		}
	}
}

func TestParseCompareFlags(t *testing.T) {
	t.Run("explicit top limits", func(t *testing.T) {
		for _, raw := range []string{"top_per_frame=2&top=5", "top=5&top_per_frame=2"} {
			c := mustParseCompare(t, raw)
			if c.TopPerFrame != 2 || c.Top != 5 || c.Request.TopPerFrame != 2 || c.Request.Top != 5 {
				t.Errorf("%q: TopPerFrame %d, Top %d; want 2, 5", raw, c.TopPerFrame, c.Top)
			}
			if !c.TopPerFrameExplicit || !c.TopExplicit {
				t.Errorf("%q: TopPerFrameExplicit = %v, TopExplicit = %v; want true, true", raw, c.TopPerFrameExplicit, c.TopExplicit)
			}
		}
		c := mustParseCompare(t, "top_per_frame=0")
		if c.TopPerFrame != 0 || !c.TopPerFrameExplicit || c.TopExplicit {
			t.Errorf("top_per_frame=0: TopPerFrame = %d, TopPerFrameExplicit = %v, TopExplicit = %v; want 0, true, false", c.TopPerFrame, c.TopPerFrameExplicit, c.TopExplicit)
		}
	})

	t.Run("all bar options", func(t *testing.T) {
		c := mustParseCompare(t, "bar_options=all")
		if !c.AllBarOptions || !c.Request.HasBarOptions || len(c.OfferedBars) != 15 || len(c.Request.Handlebars) != 15 {
			t.Fatalf("AllBarOptions %v, HasBarOptions %v, %d offered bars, %d request bars; want true, true, 15, 15",
				c.AllBarOptions, c.Request.HasBarOptions, len(c.OfferedBars), len(c.Request.Handlebars))
		}
		if c.Request.DynamicBarRoll != 0 {
			t.Errorf("DynamicBarRoll = %v, want 0", c.Request.DynamicBarRoll)
		}
	})

	t.Run("coordinates win over off", func(t *testing.T) {
		c := mustParseCompare(t, "brp_x=-210&brp_y=720&brp=off&grip_x=470&grip_y=730&grip=off")
		if c.BrpTarget != pose.NewPoint(assembly.DynamicWorld, -210, 720) {
			t.Errorf("BrpTarget = %+v, want {-210 720}", c.BrpTarget)
		}
		if c.GripTarget != pose.NewPoint(assembly.DynamicWorld, 470, 730) {
			t.Errorf("GripTarget = %+v, want {470 730}", c.GripTarget)
		}
	})
}

func TestParseCompareRejectsInvalidQueries(t *testing.T) {
	cases := []struct {
		raw     string
		field   string
		wantErr string
	}{
		// Unknown query parameters
		{"bar_option=all", "bar_option", "unknown query parameter"},
		{"top_per_fram=0", "top_per_fram", "unknown query parameter"},
		{"stock_bars=all", "stock_bars", "unknown query parameter"},
		{"post_ext=200", "post_ext", "unknown query parameter"},
		{"z_bad=1&a_bad=2", "a_bad", `unknown query parameter "a_bad"`},

		// Duplicate single-value keys
		{"top=1&top=2", "top", "may only be specified once"},
		{"post=setback_0&post=setback_15", "post", "may only be specified once"},
		{"brp_x=-219&brp_x=-200", "brp_x", "may only be specified once"},

		// Single-bike selection keys are not accepted on compare
		{"frame=surly-krampus", "frame", "unknown query parameter"},
		{"frame=&size=&fork=&sag=", "fork", "unknown query parameter"},
		{"size=L", "size", "unknown query parameter"},
		{"sag=20", "sag", "unknown query parameter"},
		{"bar_roll=10", "bar_roll", "unknown query parameter"},
		{"rail_pitch=-2", "rail_pitch", "unknown query parameter"},

		// Non-negative integer limits
		{"top_per_frame=-1", "top_per_frame", "non-negative integer"},
		{"top_per_frame=1.5", "top_per_frame", "non-negative integer"},
		{"top_per_frame=abc", "top_per_frame", "non-negative integer"},
		{"top=-1", "top", "non-negative integer"},
		{"top=abc", "top", "non-negative integer"},

		// post, bar, bar_options
		{"post=nope", "post", "unknown seatpost"},
		{"bar=nope", "bar", "unknown handlebar"},
		{"bar=nope&bar_options=all", "bar", "unknown handlebar"},
		{"bar_options=some", "bar_options", "unknown handlebar options value"},

		// Target flags and coordinate pairs
		{"brp=on", "brp", `only "off" is supported`},
		{"grip=yes", "grip", `only "off" is supported`},
		{"brp_x=-219", "brp_y", "requires both"},
		{"brp_y=713.1", "brp_x", "requires both"},
		{"brp_x=-219&brp_y=", "brp_y", "requires both"},
		{"brp_x=-219&brp=off", "brp_y", "requires both"},
		{"brp_x=", "brp_y", "requires both"},
		{"brp_y=", "brp_x", "requires both"},
		{"grip_x=481", "grip_y", "requires both"},
		{"grip_y=726", "grip_x", "requires both"},
		{"grip_x=481&grip_y=", "grip_y", "requires both"},
		{"grip_x=", "grip_y", "requires both"},
		{"brp_x=abc&brp_y=713.1", "brp_x", "finite number"},
		{"brp_x=-219&brp_y=NaN", "brp_y", "finite number"},
		{"grip_x=Inf&grip_y=726", "grip_x", "finite number"},

		// Both targets are required for compare
		{"brp=off", "brp", "compare requires a BRP target"},
		{"brp_x=&brp_y=", "brp", "compare requires a BRP target"},
		{"brp=off&brp_x=&brp_y=", "brp", "compare requires a BRP target"},
		{"grip=off", "grip", "compare requires a HandGrip target"},
		{"grip_x=&grip_y=", "grip", "compare requires a HandGrip target"},
		{"grip=off&grip_x=&grip_y=", "grip", "compare requires a HandGrip target"},
	}
	for _, tc := range cases {
		_, err := query.ParseCompare(mustParseQuery(t, tc.raw))
		if err == nil {
			t.Errorf("%q: ParseCompare succeeded, want error", tc.raw)
			continue
		}
		var inputErr *query.InputError
		if !errors.As(err, &inputErr) {
			t.Errorf("%q: error %v is not an *InputError", tc.raw, err)
			continue
		}
		if inputErr.Field != tc.field {
			t.Errorf("%q: Field = %q, want %q (error: %v)", tc.raw, inputErr.Field, tc.field, err)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%q: error %q does not contain %q", tc.raw, err, tc.wantErr)
		}
	}
}

func mustRankAndResolve(t *testing.T, c query.Compare) query.CompareResult {
	t.Helper()
	ranking, err := compare.Rank(c.Request)
	if err != nil {
		t.Fatalf("compare.Rank: %v", err)
	}
	result, err := c.ResolveResult(ranking)
	if err != nil {
		t.Fatalf("ResolveResult: %v", err)
	}
	return result
}

func TestParseCompareRankAndResolve(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		row          bikeRow
		rowCount     int
		evaluated    int
		postID       string
		barID        string
		handlebarID  string
		allBars      bool
		tier         assembly.FitTier
		total        float64
		gripR        float64
		stemLengthMm float64
		stemAngleDeg pose.Deg
		spacersMm    float64
		insertionMm  float64
		railOffsetMm float64
		extra        func(t *testing.T, row query.CompareRow)
	}{
		{
			name: "selected bar", raw: "", row: bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			rowCount: 3, evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsAll, total: 84.690417237, gripR: 0.725092414,
			stemLengthMm: 110, stemAngleDeg: -7, spacersMm: 0, insertionMm: 163.130143, railOffsetMm: 15.0,
		},
		{
			name: "all bars", raw: "bar_options=all", row: bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			rowCount: 3, evaluated: 11235, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "sqlab311_20_16_15", allBars: true,
			tier: assembly.FitsAll, total: 87.120779384, gripR: 0.579375126,
			stemLengthMm: 100, stemAngleDeg: -7, spacersMm: 0, insertionMm: 163.130143, railOffsetMm: 15.0,
			extra: func(t *testing.T, row query.CompareRow) {
				if !row.Build.HasHandlebarOption || row.Build.HandlebarIndex != 8 {
					t.Errorf("HasHandlebarOption %v, HandlebarIndex %d; want true, 8", row.Build.HasHandlebarOption, row.Build.HandlebarIndex)
				}
			},
		},
		{
			name: "sliding dropout", raw: "", row: bikeRow{"binary-havok", "L", "sklar_steel_485", 0},
			rowCount: 1, evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsNeither, total: 49.155334622, gripR: 3.718610833,
			stemLengthMm: 120, stemAngleDeg: -30, spacersMm: 0, insertionMm: 176.648907, railOffsetMm: 15.0,
			extra: func(t *testing.T, row query.CompareRow) {
				if got := row.Build.Placement.Setup.Frame.ChainstayMm; got != 460 {
					t.Errorf("Setup.Frame.ChainstayMm = %v, want 460", got)
				}
			},
		},
		{
			name: "non-default parts", raw: "post=setback_25&bar=sqlab311_20_16_50", row: bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			rowCount: 3, evaluated: 749, postID: "setback_25", barID: "sqlab311_20_16_50", handlebarID: "sqlab311_20_16_50",
			tier: assembly.FitsAll, total: 90.144412435, gripR: 0.631791160,
			stemLengthMm: 100, stemAngleDeg: -17, spacersMm: 0, insertionMm: 208.818362118, railOffsetMm: 3.068405935,
		},
		{
			name: "custom targets", raw: "brp_x=-210&brp_y=720&grip_x=470&grip_y=730", row: bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			rowCount: 3, evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsAll, total: 93.656414150, gripR: 0.949174019,
			stemLengthMm: 90, stemAngleDeg: -7, spacersMm: 0, insertionMm: 155.860514, railOffsetMm: 5.476248,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, result, row := rankRow(t, tc.raw, tc.row)
			if result.RowCount != tc.rowCount || len(result.Rows) != tc.rowCount || row.Evaluated != tc.evaluated {
				t.Fatalf("RowCount %d, %d rows, row Evaluated %d; want %d, %d, %d", result.RowCount, len(result.Rows), row.Evaluated, tc.rowCount, tc.rowCount, tc.evaluated)
			}
			if result.PostID != tc.postID || result.BarID != tc.barID || result.AllBarOptions != tc.allBars {
				t.Errorf("PostID %q, BarID %q, AllBarOptions %v; want %q, %q, %v", result.PostID, result.BarID, result.AllBarOptions, tc.postID, tc.barID, tc.allBars)
			}
			if row.HandlebarID != tc.handlebarID {
				t.Errorf("HandlebarID = %q, want %q", row.HandlebarID, tc.handlebarID)
			}
			if row.Build.Score.Tier != tc.tier {
				t.Errorf("Tier = %q, want %q", row.Build.Score.Tier, tc.tier)
			}
			assertNear(t, "Total", row.Build.Score.Total, tc.total, 1e-6)
			assertNear(t, "GripR", row.Build.Score.GripR, tc.gripR, 1e-6)
			setup := row.Build.Placement.Setup
			if setup.Stem.LengthMm != tc.stemLengthMm || setup.Stem.AngleDeg != tc.stemAngleDeg || setup.Stem.SpacersMm != tc.spacersMm {
				t.Errorf("stem (%v, %v, %v), want (%v, %v, %v)", setup.Stem.LengthMm, setup.Stem.AngleDeg, setup.Stem.SpacersMm, tc.stemLengthMm, tc.stemAngleDeg, tc.spacersMm)
			}
			assertNear(t, "InsertionMm", setup.Seatpost.InsertionMm, tc.insertionMm, 1e-6)
			assertNear(t, "RailOffsetMm", setup.Saddle.RailOffsetMm, tc.railOffsetMm, 1e-6)
			if tc.extra != nil {
				tc.extra(t, row)
			}
		})
	}
}

func TestWinningHandlebarOutOfRange(t *testing.T) {
	c := mustParseCompare(t, "bar_options=all")
	for _, idx := range []int{-1, 15} {
		row := compare.Row{FrameID: "surly-krampus", Size: "XL"}
		row.Build.HasHandlebarOption = true
		row.Build.HandlebarIndex = idx
		if _, err := c.WinningHandlebar(row); err == nil {
			t.Errorf("WinningHandlebar(index %d) succeeded, want error", idx)
		}
		if _, err := c.ResolveResult(compare.Ranking{Rows: []compare.Row{row}, RowCount: 1}); err == nil {
			t.Errorf("ResolveResult(index %d) succeeded, want error", idx)
		}
	}
}

// TestParseCompareDefaultAllFrames ranks every catalog frame on the selected
// bar. It is the suite's one full-catalog ranking: it checks that every catalog
// frame ranks, so every other compare test ranks trimmed frames.
func TestParseCompareDefaultAllFrames(t *testing.T) {
	result := mustRankAndResolve(t, mustParseCompare(t, ""))
	if result.RowCount != 2889 || result.Evaluated != 2163861 || len(result.Rows) != 18 {
		t.Fatalf("RowCount %d, Evaluated %d, %d rows; want 2889, 2163861, 18", result.RowCount, result.Evaluated, len(result.Rows))
	}
	if result.PostID != "redshift_shockstop_pro" || result.BarID != "persuader20_800" || result.AllBarOptions {
		t.Errorf("PostID %q, BarID %q, AllBarOptions %v", result.PostID, result.BarID, result.AllBarOptions)
	}

	want := []struct {
		index  int
		frame  string
		size   string
		fork   string
		sagPct float64
		tier   assembly.FitTier
		total  float64
		gripR  float64
	}{
		{0, "surly-krampus", "XL", "fox34_120-534x44", 25, assembly.FitsAll, 85.120463740, 0.945603917},
		{1, "sklar-pbj", "L", "fox36_sl_120", 15, assembly.FitsAll, 76.200061329, 0.887113437},
		{2, "seven-custom", "Custom", "fox34_120-534x44", 25, assembly.FitsBRP, 53.126792179, 2.092042057},
		{7, "esker-hayduke", "S3", "enve_mountain_boost-492x52", 0, assembly.FitsGrip, 81.320323095, 0.952602612},
		{15, "otso-fenrir-stainless", "XL", "firestarter_deluxe_483", 0, assembly.FitsGrip, 69.087518917, 0.956687823},
		{17, "salsa-cutthroat", "60cm", "firestarter_deluxe_483", 0, assembly.FitsNeither, 50.260457620, 3.453797202},
	}
	for _, w := range want {
		row := result.Rows[w.index]
		if row.FrameID != w.frame || row.Size != w.size || row.ForkID != w.fork || row.SagPct != w.sagPct || row.Build.Score.Tier != w.tier {
			t.Errorf("row %d = %s %s %s %v%% %s; want %s %s %s %v%% %s", w.index+1,
				row.FrameID, row.Size, row.ForkID, row.SagPct, row.Build.Score.Tier,
				w.frame, w.size, w.fork, w.sagPct, w.tier)
		}
		assertNear(t, "Total", row.Build.Score.Total, w.total, 1e-6)
		assertNear(t, "GripR", row.Build.Score.GripR, w.gripR, 1e-6)
	}
	if result.Rows[0].HandlebarID != "persuader20_800" {
		t.Errorf("row 1 HandlebarID = %q, want persuader20_800", result.Rows[0].HandlebarID)
	}
}

func TestCompareRowBikeURLRoundTrip(t *testing.T) {
	requiredKeys := []string{
		"frame", "size", "fork", "sag", "post", "post_insert", "rail_offset", "rail_pitch",
		"bar", "bar_roll", "stem_length", "stem_angle", "spacers", "brp_x", "brp_y", "grip_x", "grip_y",
		"bar_options", "fork_options",
	}
	cases := []struct {
		raw            string
		row            bikeRow
		wantTier       assembly.FitTier
		wantScore      float64
		wantBarID      string
		wantBarOptions string
		wantChainstay  float64
	}{
		{
			raw:            "",
			row:            bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			wantTier:       assembly.FitsAll,
			wantScore:      84.690417237,
			wantBarID:      "persuader20_800",
			wantBarOptions: query.BarOptionsSelected,
		},
		{
			raw:            "bar_options=all",
			row:            bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			wantTier:       assembly.FitsAll,
			wantScore:      87.120779384,
			wantBarID:      "sqlab311_20_16_15",
			wantBarOptions: query.BarOptionsAll,
		},
		{
			raw:            "",
			row:            bikeRow{"binary-havok", "L", "sklar_steel_485", 0},
			wantTier:       assembly.FitsNeither,
			wantScore:      49.155334622,
			wantBarID:      "persuader20_800",
			wantBarOptions: query.BarOptionsSelected,
			wantChainstay:  460,
		},
		{
			raw:            "post=setback_25&bar=sqlab311_20_16_50&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
			row:            bikeRow{"surly-krampus", "XL", "fox34_120-534x44", 20},
			wantTier:       assembly.FitsAll,
			wantScore:      92.063410256,
			wantBarID:      "sqlab311_20_16_50",
			wantBarOptions: query.BarOptionsSelected,
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%+v", tc.raw, tc.row), func(t *testing.T) {
			c, _, row := rankRow(t, tc.raw, tc.row)
			u, err := c.RowBikeURL(row.Row)
			if err != nil {
				t.Fatalf("RowBikeURL: %v", err)
			}
			parsedURL, err := url.Parse(u)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", u, err)
			}
			q := parsedURL.Query()
			for _, key := range requiredKeys {
				if !q.Has(key) {
					t.Errorf("RowBikeURL missing key %q in %q", key, u)
				}
			}
			if got := q.Get("bar_options"); got != tc.wantBarOptions {
				t.Errorf("bar_options = %q, want %q", got, tc.wantBarOptions)
			}
			if got := q.Get("fork_options"); got != query.ForkOptionsSelected {
				t.Errorf("fork_options = %q, want %q", got, query.ForkOptionsSelected)
			}
			b, err := query.ParseBike(q)
			if err != nil {
				t.Fatalf("ParseBike(%q): %v", u, err)
			}
			if b.Bar.ID != tc.wantBarID {
				t.Errorf("Bar.ID = %q, want %q", b.Bar.ID, tc.wantBarID)
			}
			if tc.wantChainstay != 0 {
				fr, err := b.FramePart()
				if err != nil {
					t.Fatalf("FramePart: %v", err)
				}
				if fr.Dimensions().ChainstayMm != tc.wantChainstay {
					t.Errorf("Frame.ChainstayMm = %v, want %v", fr.Dimensions().ChainstayMm, tc.wantChainstay)
				}
			}
			res, err := b.Evaluate(query.RankedBuildLimit)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if res.Score == nil {
				t.Fatal("Score is nil")
			}
			if res.Score.Tier != tc.wantTier {
				t.Errorf("Score.Tier = %q, want %q", res.Score.Tier, tc.wantTier)
			}
			assertNear(t, "Score.Total", res.Score.Total, tc.wantScore, 1e-9)
			assertNear(t, "Builds[0].Score.Total", res.Builds.Builds[0].Score.Total, tc.wantScore, 1e-9)
			if res.Builds.Builds[0].HandlebarID != tc.wantBarID {
				t.Errorf("Builds[0].HandlebarID = %q, want %q", res.Builds.Builds[0].HandlebarID, tc.wantBarID)
			}
		})
	}

	t.Run("negative cases", func(t *testing.T) {
		c := mustParseCompare(t, "")
		badRows := []struct {
			row     compare.Row
			wantErr string
		}{
			{compare.Row{FrameID: "nope"}, "unknown frame"},
			{compare.Row{FrameID: "surly-krampus", Size: "XXL"}, "has no size"},
			{compare.Row{FrameID: "surly-krampus", Size: "XL", ForkID: "nope"}, "has no fork"},
		}
		for _, tc := range badRows {
			if _, err := c.BikeForRow(tc.row); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("BikeForRow(%+v) err = %v, want containing %q", tc.row, err, tc.wantErr)
			}
			if _, err := c.RowBikeURL(tc.row); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("RowBikeURL(%+v) err = %v, want containing %q", tc.row, err, tc.wantErr)
			}
		}

		cAll := mustParseCompare(t, "bar_options=all")
		for _, idx := range []int{-1, 15} {
			row := compare.Row{
				FrameID: "surly-krampus",
				Size:    "XL",
				ForkID:  "fox34_120-534x44",
				Build:   assembly.ScoredBuild{HasHandlebarOption: true, HandlebarIndex: idx},
			}
			if _, err := cAll.BikeForRow(row); err == nil || !strings.Contains(err.Error(), "out of range") {
				t.Errorf("BikeForRow(idx %d) err = %v, want out of range", idx, err)
			}
			if _, err := cAll.RowBikeURL(row); err == nil || !strings.Contains(err.Error(), "out of range") {
				t.Errorf("RowBikeURL(idx %d) err = %v, want out of range", idx, err)
			}
		}
	})
}

// TestParseCompareOpenForkChoice selects a compatible fork a frame does not
// list and counts the open fork rows.
func TestParseCompareOpenForkChoice(t *testing.T) {
	c := mustParseCompare(t, "top_per_frame=0")
	c.Request.Frames = trimFrames(t, "surly-krampus", "", "pike_120")
	result := mustRankAndResolve(t, c)
	if result.RowCount != 12 || len(result.Rows) != 12 {
		t.Errorf("krampus pike_120 RowCount %d, %d rows; want 12, 12 (4 sizes x 3 sags)", result.RowCount, len(result.Rows))
	}
	for _, row := range result.Rows {
		if row.ForkID != "pike_120" {
			t.Errorf("row fork = %q, want pike_120", row.ForkID)
		}
	}
	forks := 0
	for _, f := range query.CatalogFrames() {
		forks += len(f.Forks)
	}
	if forks != 223 {
		t.Errorf("catalog frames carry %d fork variants, want 223", forks)
	}
}
