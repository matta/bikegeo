package query_test

import (
	"errors"
	"math"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/compare"
	"bikegeo/internal/reboot/pose"
	"bikegeo/internal/reboot/query"
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

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (tol %g)", name, got, want, tol)
	}
}

// TestCatalogFrames checks the catalog-to-compare frame adapter (Q67, Q69).
func TestCatalogFrames(t *testing.T) {
	frames := catalog.Frames()
	got := query.CatalogFrames()
	if len(got) != 19 || len(got) != len(frames) {
		t.Fatalf("got %d frames, want 19 (catalog has %d)", len(got), len(frames))
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
			if !reflect.DeepEqual(cf.Sizes[j].Frame, f.InstalledFrame(sz)) {
				t.Errorf("%s size %q frame differs from InstalledFrame", f.ID, sz.Name)
			}
		}
		totalSizes += len(cf.Sizes)
		if len(cf.Forks) != len(f.Forks) {
			t.Fatalf("%s: got %d forks, want %d", f.ID, len(cf.Forks), len(f.Forks))
		}
		for j, fk := range f.Forks {
			if cf.Forks[j].ID != fk.Fork.ID {
				t.Errorf("%s fork %d = %q, want %q", f.ID, j, cf.Forks[j].ID, fk.Fork.ID)
			}
			if cf.Forks[j].Spec != fk.Spec() {
				t.Errorf("%s fork %q spec = %+v, want %+v", f.ID, fk.Fork.ID, cf.Forks[j].Spec, fk.Spec())
			}
		}
	}
	if totalSizes != 119 {
		t.Errorf("got %d sizes, want 119", totalSizes)
	}

	havok, ok := catalog.FrameByID("binary-havok")
	if !ok {
		t.Fatal("binary-havok missing from catalog")
	}
	sz, ok := havok.Size("L")
	if !ok {
		t.Fatal("binary-havok has no size L")
	}
	if sz.ChartFrame.ChainstayMm != 445 {
		t.Errorf("binary-havok L published chainstay = %v, want 445", sz.ChartFrame.ChainstayMm)
	}
	for _, cf := range got {
		if cf.ID != "binary-havok" {
			continue
		}
		for _, s := range cf.Sizes {
			if s.Name == "L" && s.Frame.ChainstayMm != 460 {
				t.Errorf("binary-havok L installed chainstay = %v, want 460 (Q67)", s.Frame.ChainstayMm)
			}
		}
	}
}

// TestParseCompareDefaults checks the catalog compare defaults (Q53, Q56, Q62, Q87).
func TestParseCompareDefaults(t *testing.T) {
	for _, raw := range []string{"", "frame=&size=&fork=&sag="} {
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
		if c.BarRollDeg != 0 || c.RailPitchDeg != 0 {
			t.Errorf("%q: BarRollDeg = %v, RailPitchDeg = %v; want 0, 0", raw, c.BarRollDeg, c.RailPitchDeg)
		}
		if c.BrpTarget != (pose.Point{X: -219, Y: 713.1}) {
			t.Errorf("%q: BrpTarget = %+v", raw, c.BrpTarget)
		}
		if c.GripTarget != (pose.Point{X: 481, Y: 726}) {
			t.Errorf("%q: GripTarget = %+v", raw, c.GripTarget)
		}
		if len(c.Selections) != 0 {
			t.Errorf("%q: got %d selections, want 0", raw, len(c.Selections))
		}
		if c.TopPerFrame != 1 || c.Top != 0 {
			t.Errorf("%q: TopPerFrame = %d, Top = %d; want 1, 0", raw, c.TopPerFrame, c.Top)
		}
		if c.TopPerFrameExplicit || c.TopExplicit {
			t.Errorf("%q: TopPerFrameExplicit = %v, TopExplicit = %v; want false, false", raw, c.TopPerFrameExplicit, c.TopExplicit)
		}
		if got := c.URL(); got != "/" {
			t.Errorf("%q: URL() = %q, want %q", raw, got, "/")
		}
		if c.Request.Parts.Handlebars != nil {
			t.Errorf("%q: Request.Parts.Handlebars = %v, want nil", raw, c.Request.Parts.Handlebars)
		}
		if len(c.Request.Parts.Stems) != 107 {
			t.Errorf("%q: got %d stems, want 107", raw, len(c.Request.Parts.Stems))
		}
		if c.Request.Parts.SpacerStepMm != 5 {
			t.Errorf("%q: SpacerStepMm = %v, want 5", raw, c.Request.Parts.SpacerStepMm)
		}
	}
}

func TestParseCompareSelectionsAndFlags(t *testing.T) {
	t.Run("single selection", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&size=XL&fork=fox34_120&sag=20")
		if len(c.Selections) != 1 || c.TopPerFrame != 0 {
			t.Fatalf("got %d selections, TopPerFrame %d; want 1, 0", len(c.Selections), c.TopPerFrame)
		}
		if c.TopPerFrameExplicit || c.TopExplicit {
			t.Errorf("TopPerFrameExplicit = %v, TopExplicit = %v; want false, false", c.TopPerFrameExplicit, c.TopExplicit)
		}
		if c.Selections[0].SagPct == nil || *c.Selections[0].SagPct != 20 {
			t.Errorf("SagPct = %v, want 20", c.Selections[0].SagPct)
		}
	})

	t.Run("omitted optional columns", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&frame=otso-fenrir-stainless")
		if len(c.Selections) != 2 || c.TopPerFrame != 0 {
			t.Fatalf("got %d selections, TopPerFrame %d; want 2, 0", len(c.Selections), c.TopPerFrame)
		}
		for i, s := range c.Selections {
			if s.Size != "" || s.ForkID != "" || s.SagPct != nil {
				t.Errorf("selection %d = %+v, want open size, fork, and sag", i, s)
			}
		}
	})

	t.Run("trailing blank form slot", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&size=XL&fork=&sag=&frame=&size=&fork=&sag=")
		if len(c.Selections) != 1 || c.TopPerFrame != 0 {
			t.Errorf("got %d selections, TopPerFrame %d; want 1, 0", len(c.Selections), c.TopPerFrame)
		}
	})

	t.Run("explicit top limits", func(t *testing.T) {
		for _, raw := range []string{"top_per_frame=2&top=5", "frame=surly-krampus&top_per_frame=2&top=5"} {
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

	t.Run("rigid fork zero sag", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&fork=surly_krampus_483&sag=0")
		if c.Selections[0].SagPct == nil || *c.Selections[0].SagPct != 0 {
			t.Errorf("SagPct = %v, want 0", c.Selections[0].SagPct)
		}
	})

	t.Run("decoded space in size", func(t *testing.T) {
		c := mustParseCompare(t, "frame=seven-expedition-926d&size=S+58cm")
		if c.Selections[0].Size != "S 58cm" {
			t.Errorf("Size = %q, want %q", c.Selections[0].Size, "S 58cm")
		}
	})

	t.Run("all bar options", func(t *testing.T) {
		c := mustParseCompare(t, "bar_options=all&bar_roll=10")
		if !c.AllBarOptions || len(c.OfferedBars) != 15 || len(c.Request.Parts.Handlebars) != 15 {
			t.Fatalf("AllBarOptions %v, %d offered bars, %d part bars; want true, 15, 15", c.AllBarOptions, len(c.OfferedBars), len(c.Request.Parts.Handlebars))
		}
		for i, b := range c.Request.Parts.Handlebars {
			if b.BarRollDeg != 0 {
				t.Errorf("Parts.Handlebars[%d].BarRollDeg = %v, want 0", i, b.BarRollDeg)
			}
		}
		if c.Request.Handlebar.BarRollDeg != 10 {
			t.Errorf("Request.Handlebar.BarRollDeg = %v, want 10 (Q24)", c.Request.Handlebar.BarRollDeg)
		}
	})

	t.Run("coordinates win over off", func(t *testing.T) {
		c := mustParseCompare(t, "brp_x=-210&brp_y=720&brp=off&grip_x=470&grip_y=730&grip=off")
		if c.BrpTarget != (pose.Point{X: -210, Y: 720}) {
			t.Errorf("BrpTarget = %+v, want {-210 720}", c.BrpTarget)
		}
		if c.GripTarget != (pose.Point{X: 470, Y: 730}) {
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
		// checkKnownKeys (Q107)
		{"bar_option=all", "bar_option", "unknown query parameter"},
		{"top_per_fram=0", "top_per_fram", "unknown query parameter"},
		{"stock_bars=all", "stock_bars", "unknown query parameter"},
		{"post_ext=200", "post_ext", "unknown query parameter"},
		{"z_bad=1&a_bad=2", "a_bad", `unknown query parameter "a_bad"`},

		// checkSingleValueKeys
		{"top=1&top=2", "top", "may only be specified once"},
		{"post=setback_0&post=setback_15", "post", "may only be specified once"},
		{"brp_x=-219&brp_x=-200", "brp_x", "may only be specified once"},

		// parseSelections (Q87)
		{"size=L", "size", `requires "frame"`},
		{"fork=sid_100", "fork", `requires "frame"`},
		{"sag=20", "sag", `requires "frame"`},
		{"size=", "size", `requires "frame"`},
		{"frame=surly-krampus&frame=otso-fenrir-stainless&size=XL", "size", "must match"},
		{"frame=surly-krampus&fork=fox34_120&fork=surly_krampus_483", "fork", "must match"},
		{"frame=surly-krampus&frame=otso-fenrir-stainless&sag=20", "sag", "must match"},
		{"frame=&size=XL&fork=&sag=", "frame", `missing "frame"`},
		{"frame=surly-krampus&size=XL&fork=&sag=&frame=&size=&fork=sid_100&sag=", "frame", `selection 2 is missing "frame"`},
		{"frame=unknown-bike", "frame", "unknown frame"},
		{"frame=surly-krampus&size=XXL", "size", "has no size"},
		// The Krampus carries sid_100 in the catalog; fox36_150 is a catalog fork it does not.
		{"frame=surly-krampus&fork=fox36_150", "fork", "has no fork"},
		{"frame=surly-krampus&fork=cad", "fork", "has no fork"},
		{"frame=surly-krampus&size=XL&sag=20", "sag", "sag requires a fork"},
		{"frame=surly-krampus&fork=fox34_120&sag=abc", "sag", "finite number"},
		{"frame=surly-krampus&fork=fox34_120&sag=NaN", "sag", "finite number"},
		{"frame=surly-krampus&fork=fox34_120&sag=Inf", "sag", "finite number"},
		{"frame=surly-krampus&fork=fox34_120&sag=-5", "sag", "fork sag must be in"},
		{"frame=surly-krampus&fork=fox34_120&sag=105", "sag", "fork sag must be in"},
		{"frame=surly-krampus&fork=surly_krampus_483&sag=5", "sag", "rigid fork"},

		// parseNonNegInt
		{"top_per_frame=-1", "top_per_frame", "non-negative integer"},
		{"top_per_frame=1.5", "top_per_frame", "non-negative integer"},
		{"top_per_frame=abc", "top_per_frame", "non-negative integer"},
		{"top=-1", "top", "non-negative integer"},
		{"top=abc", "top", "non-negative integer"},

		// post, bar, bar_options, posture angles
		{"post=nope", "post", "unknown seatpost"},
		{"bar=nope", "bar", "unknown handlebar"},
		{"bar=nope&bar_options=all", "bar", "unknown handlebar"},
		{"bar_options=some", "bar_options", "unknown handlebar options value"},
		{"bar_roll=NaN", "bar_roll", "handlebar roll must be a finite number"},
		{"bar_roll=abc", "bar_roll", "handlebar roll must be a finite number"},
		{"rail_pitch=Inf", "rail_pitch", "saddle rail pitch must be a finite number"},
		{"rail_pitch=abc", "rail_pitch", "saddle rail pitch must be a finite number"},

		// parseTarget flags and pairs (Q70)
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

		// required targets (Q76)
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

func rankAndResolve(t *testing.T, raw string) (query.Compare, query.CompareResult) {
	t.Helper()
	c := mustParseCompare(t, raw)
	ranking, err := compare.Rank(c.Request)
	if err != nil {
		t.Fatalf("%q: compare.Rank: %v", raw, err)
	}
	result, err := c.ResolveResult(ranking)
	if err != nil {
		t.Fatalf("%q: ResolveResult: %v", raw, err)
	}
	return c, result
}

func TestParseCompareRankAndResolve(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
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
			name: "selected bar", raw: "frame=surly-krampus&size=XL&fork=fox34_120&sag=20",
			evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsAll, total: 84.575011405, gripR: 0.759424168,
			stemLengthMm: 100, stemAngleDeg: -10, spacersMm: 0, insertionMm: 163.906090, railOffsetMm: 15.0,
		},
		{
			name: "all bars", raw: "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all",
			evaluated: 11235, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "sqlab311_flx_16_30", allBars: true,
			tier: assembly.FitsAll, total: 86.650135345, gripR: 0.389349805,
			stemLengthMm: 100, stemAngleDeg: -7, spacersMm: 0, insertionMm: 163.906090, railOffsetMm: 15.0,
			extra: func(t *testing.T, row query.CompareRow) {
				if !row.Build.HasHandlebarOption || row.Build.HandlebarIndex != 12 {
					t.Errorf("HasHandlebarOption %v, HandlebarIndex %d; want true, 12", row.Build.HasHandlebarOption, row.Build.HandlebarIndex)
				}
			},
		},
		{
			name: "sliding dropout", raw: "frame=binary-havok&size=L&fork=rigid_485",
			evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsNeither, total: 49.866243596, gripR: 3.875593862,
			stemLengthMm: 120, stemAngleDeg: -30, spacersMm: 0, insertionMm: 176.175807, railOffsetMm: 15.0,
			extra: func(t *testing.T, row query.CompareRow) {
				if got := row.Build.Placement.Setup.Frame.ChainstayMm; got != 460 {
					t.Errorf("Setup.Frame.ChainstayMm = %v, want 460 (Q67)", got)
				}
			},
		},
		{
			name: "non-default parts", raw: "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2",
			evaluated: 749, postID: "setback_25", barID: "sqlab311_20_16_50", handlebarID: "sqlab311_20_16_50",
			tier: assembly.FitsAll, total: 91.201799677, gripR: 0.082545290,
			stemLengthMm: 80, stemAngleDeg: -30, spacersMm: 0, insertionMm: 208.681544, railOffsetMm: 6.694435,
		},
		{
			name: "custom targets", raw: "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
			evaluated: 749, postID: "redshift_shockstop_pro", barID: "persuader20_800", handlebarID: "persuader20_800",
			tier: assembly.FitsAll, total: 93.409769610, gripR: 0.703926294,
			stemLengthMm: 90, stemAngleDeg: -7, spacersMm: 0, insertionMm: 156.644040, railOffsetMm: 7.855946,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, result := rankAndResolve(t, tc.raw)
			if result.RowCount != 1 || result.Evaluated != tc.evaluated || len(result.Rows) != 1 {
				t.Fatalf("RowCount %d, Evaluated %d, %d rows; want 1, %d, 1", result.RowCount, result.Evaluated, len(result.Rows), tc.evaluated)
			}
			if result.PostID != tc.postID || result.BarID != tc.barID || result.AllBarOptions != tc.allBars {
				t.Errorf("PostID %q, BarID %q, AllBarOptions %v; want %q, %q, %v", result.PostID, result.BarID, result.AllBarOptions, tc.postID, tc.barID, tc.allBars)
			}
			row := result.Rows[0]
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

// TestParseCompareDefaultAll19Frames ranks every catalog frame on the
// selected bar (~1.8 s, Q104).
func TestParseCompareDefaultAll19Frames(t *testing.T) {
	_, result := rankAndResolve(t, "")
	if result.RowCount != 701 || result.Evaluated != 525049 || len(result.Rows) != 19 {
		t.Fatalf("RowCount %d, Evaluated %d, %d rows; want 701, 525049, 19", result.RowCount, result.Evaluated, len(result.Rows))
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
		{0, "surly-krampus", "XL", "fox34_120", 20, assembly.FitsAll, 84.575011405, 0.759424168},
		{1, "seven-expedition-926f", "S 64cm", "sid_100", 25, assembly.FitsBRP, 46.899733053, 1.992641645},
		{5, "sour-pasta-party", "L", "sid_100", 20, assembly.FitsGrip, 80.881655493, 0.899510156},
		{18, "salsa-cutthroat", "60cm", "sid_100", 25, assembly.FitsNeither, 45.904045523, 3.265137449},
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

func TestCompareURLAndSelectionHelpers(t *testing.T) {
	t.Run("URL round-trip", func(t *testing.T) {
		cases := []string{
			"",
			"frame=surly-krampus&size=XL&fork=fox34_120&sag=20",
			"frame=surly-krampus&frame=binary-havok&top_per_frame=1&top=5&post=setback_25&bar=sqlab311_20_16_50&bar_options=all&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
			"top_per_frame=0",
		}
		for _, raw := range cases {
			c := mustParseCompare(t, raw)
			u := c.URL()
			if raw == "" && u != "/" {
				t.Errorf("empty query URL() = %q, want %q", u, "/")
			}
			if raw == "top_per_frame=0" && !strings.Contains(u, "top_per_frame=0") {
				t.Errorf("top_per_frame=0 URL() = %q, want top_per_frame=0", u)
			}
			parsedURL, err := url.Parse(u)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", u, err)
			}
			got, err := query.ParseCompare(parsedURL.Query())
			if err != nil {
				t.Fatalf("ParseCompare(%q): %v", u, err)
			}
			if got.Post.ID != c.Post.ID || got.Bar.ID != c.Bar.ID || got.AllBarOptions != c.AllBarOptions {
				t.Errorf("%q round-trip parts mismatch: got (%q, %q, %v), want (%q, %q, %v)",
					raw, got.Post.ID, got.Bar.ID, got.AllBarOptions, c.Post.ID, c.Bar.ID, c.AllBarOptions)
			}
			if got.BarRollDeg != c.BarRollDeg || got.RailPitchDeg != c.RailPitchDeg || got.BrpTarget != c.BrpTarget || got.GripTarget != c.GripTarget {
				t.Errorf("%q round-trip posture/targets mismatch", raw)
			}
			if !reflect.DeepEqual(got.Selections, c.Selections) {
				t.Errorf("%q round-trip Selections = %+v, want %+v", raw, got.Selections, c.Selections)
			}
			if got.TopPerFrame != c.TopPerFrame || got.TopPerFrameExplicit != c.TopPerFrameExplicit || got.Top != c.Top || got.TopExplicit != c.TopExplicit {
				t.Errorf("%q round-trip limits mismatch: got (%d, %v, %d, %v), want (%d, %v, %d, %v)",
					raw, got.TopPerFrame, got.TopPerFrameExplicit, got.Top, got.TopExplicit,
					c.TopPerFrame, c.TopPerFrameExplicit, c.Top, c.TopExplicit)
			}
		}
	})

	t.Run("WithoutSelection", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&frame=binary-havok&post=setback_25")
		c1 := c.WithoutSelection(0)
		if len(c1.Selections) != 1 || c1.Selections[0].FrameID != "binary-havok" || c1.TopPerFrame != 0 || c1.Post.ID != "setback_25" {
			t.Fatalf("after first WithoutSelection(0): %+v", c1)
		}
		c2 := c1.WithoutSelection(0)
		if c2.Selections != nil || c2.TopPerFrame != 1 || c2.TopPerFrameExplicit || c2.URL() != "/?post=setback_25" {
			t.Fatalf("after second WithoutSelection(0): Selections=%v TopPerFrame=%d Explicit=%v URL=%q",
				c2.Selections, c2.TopPerFrame, c2.TopPerFrameExplicit, c2.URL())
		}
		for _, badIdx := range []int{-1, 2} {
			unchanged := c.WithoutSelection(badIdx)
			if !reflect.DeepEqual(unchanged.Selections, c.Selections) {
				t.Errorf("WithoutSelection(%d) modified Selections", badIdx)
			}
		}
	})

	t.Run("AllFrames", func(t *testing.T) {
		c := mustParseCompare(t, "frame=surly-krampus&size=XL&top_per_frame=2&top=5&post=setback_25&bar_options=all")
		got := c.AllFrames()
		if got.Selections != nil || got.TopPerFrame != 1 || got.TopPerFrameExplicit || got.Top != 5 || !got.TopExplicit || got.Post.ID != "setback_25" || !got.AllBarOptions {
			t.Fatalf("AllFrames() = %+v", got)
		}
	})
}

func TestCompareRowBikeURLRoundTrip(t *testing.T) {
	requiredKeys := []string{
		"frame", "size", "fork", "sag", "post", "post_insert", "rail_offset", "rail_pitch",
		"bar", "bar_roll", "stem_length", "stem_angle", "spacers", "brp_x", "brp_y", "grip_x", "grip_y",
		"bar_options", "fork_options",
	}
	cases := []struct {
		raw            string
		wantTier       assembly.FitTier
		wantScore      float64
		wantBarID      string
		wantBarOptions string
		wantChainstay  float64
	}{
		{
			raw:            "frame=surly-krampus&size=XL&fork=fox34_120&sag=20",
			wantTier:       assembly.FitsAll,
			wantScore:      84.575011405,
			wantBarID:      "persuader20_800",
			wantBarOptions: query.BarOptionsSelected,
		},
		{
			raw:            "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all",
			wantTier:       assembly.FitsAll,
			wantScore:      86.650135345,
			wantBarID:      "sqlab311_flx_16_30",
			wantBarOptions: query.BarOptionsAll,
		},
		{
			raw:            "frame=binary-havok&size=L&fork=rigid_485",
			wantTier:       assembly.FitsNeither,
			wantScore:      49.866243596,
			wantBarID:      "persuader20_800",
			wantBarOptions: query.BarOptionsSelected,
			wantChainstay:  460,
		},
		{
			raw:            "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
			wantTier:       assembly.FitsAll,
			wantScore:      94.341904588,
			wantBarID:      "sqlab311_20_16_50",
			wantBarOptions: query.BarOptionsSelected,
		},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			c := mustParseCompare(t, tc.raw)
			ranking, err := compare.Rank(c.Request)
			if err != nil {
				t.Fatalf("compare.Rank: %v", err)
			}
			u, err := c.RowBikeURL(ranking.Rows[0])
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
			if tc.wantChainstay != 0 && b.Setup.Frame.ChainstayMm != tc.wantChainstay {
				t.Errorf("Setup.Frame.ChainstayMm = %v, want %v", b.Setup.Frame.ChainstayMm, tc.wantChainstay)
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
				ForkID:  "fox34_120",
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
