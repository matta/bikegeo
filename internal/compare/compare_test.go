package compare_test

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/pose"
)

func mustFrame(dims assembly.FrameDimensions) assembly.Frame {
	f, err := assembly.NewFrame(dims)
	if err != nil {
		panic(err)
	}
	return f
}

func rigidForkBuilder(staticA2cMm, rakeMm float64, steerer assembly.PhysicalSteerer) func(float64) (compare.PhysicalFork, error) {
	return func(float64) (compare.PhysicalFork, error) {
		return assembly.NewRigidFork(staticA2cMm, rakeMm, steerer)
	}
}

func suspensionForkBuilder(staticA2cMm, rakeMm float64, travel assembly.ForkTravel, steerer assembly.PhysicalSteerer) func(float64) (compare.PhysicalFork, error) {
	return func(sagPct float64) (compare.PhysicalFork, error) {
		return assembly.NewSuspensionFork(staticA2cMm, rakeMm, travel, steerer, sagPct)
	}
}

// All fixture values below are copied from the catalog definitions for test isolation.
func fixtureFrames() []compare.Frame {
	ec44 := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	return []compare.Frame{
		{
			ID: "surly-krampus",
			Sizes: []compare.Size{
				{
					Name: "L",
					Frame: mustFrame(assembly.FrameDimensions{
						ReachMm: 448, StackMm: 612, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 73,
						HeadTubeLengthMm: 110, BbDropMm: 65, ChainstayMm: 435, SeatTubeLengthMm: 470,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					}),
				},
				{
					Name: "XL",
					Frame: mustFrame(assembly.FrameDimensions{
						ReachMm: 470, StackMm: 621, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 73,
						HeadTubeLengthMm: 120, BbDropMm: 65, ChainstayMm: 435, SeatTubeLengthMm: 521,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					}),
				},
			},
			Forks: []compare.Fork{
				{ID: "surly_krampus_483", Travel: assembly.TravelRigid, Build: rigidForkBuilder(483, 47, assembly.PhysicalSteererStraight118)},
				{ID: "fox34_120-534x44", Travel: assembly.Travel120, Build: suspensionForkBuilder(534, 44, assembly.Travel120, assembly.PhysicalSteererTapered)},
				{ID: "fox34_120-534x51", Travel: assembly.Travel120, Build: suspensionForkBuilder(534, 51, assembly.Travel120, assembly.PhysicalSteererTapered)},
			},
		},
		{
			ID: "otso-fenrir-stainless",
			Sizes: []compare.Size{
				{
					Name: "L",
					Frame: mustFrame(assembly.FrameDimensions{
						ReachMm: 428, StackMm: 642, HeadTubeAngleDeg: 68.25, SeatTubeAngleDeg: 75.25,
						HeadTubeLengthMm: 140, BbDropMm: 72, ChainstayMm: 450, SeatTubeLengthMm: 490,
						MaxPostInsertionMm: 290, LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					}),
				},
				{
					Name: "XL",
					Frame: mustFrame(assembly.FrameDimensions{
						ReachMm: 441, StackMm: 660, HeadTubeAngleDeg: 68.25, SeatTubeAngleDeg: 75.25,
						HeadTubeLengthMm: 160, BbDropMm: 72, ChainstayMm: 450, SeatTubeLengthMm: 530,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					}),
				},
			},
			Forks: []compare.Fork{
				{ID: "lithic_carbon_mountain-495x44", Travel: assembly.TravelRigid, Build: rigidForkBuilder(495, 44, assembly.PhysicalSteererTapered)},
				{ID: "lithic_carbon_mountain-498x51.5", Travel: assembly.TravelRigid, Build: rigidForkBuilder(498, 51.5, assembly.PhysicalSteererTapered)},
				{ID: "sid_100-506x44", Travel: assembly.Travel100, Build: suspensionForkBuilder(506, 44, assembly.Travel100, assembly.PhysicalSteererTapered)},
				{ID: "sid_100-506x51", Travel: assembly.Travel100, Build: suspensionForkBuilder(506, 51, assembly.Travel100, assembly.PhysicalSteererTapered)},
			},
		},
	}
}

func fixtureStems() []assembly.StemOption {
	return []assembly.StemOption{
		{LengthMm: 60, AngleDeg: 0, Score: 95.5},
		{LengthMm: 70, AngleDeg: -6, Score: 100.0},
		{LengthMm: 70, AngleDeg: 0, Score: 100.0},
		{LengthMm: 70, AngleDeg: 6, Score: 100.0},
		{LengthMm: 90, AngleDeg: -17, Score: 62.0},
		{LengthMm: 110, AngleDeg: -17, Score: 36.25},
		{LengthMm: 120, AngleDeg: -30, Score: 0.0},
		{LengthMm: 120, AngleDeg: -20, Score: 0.0},
		{LengthMm: 120, AngleDeg: -17, Score: 24.0},
		{LengthMm: 120, AngleDeg: -10, Score: 34.5},
	}
}

func persuaderBar() assembly.Handlebar {
	bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm: 800, CurrentWidthMm: 800, StatedRiseMm: 50, RiseMode: assembly.RiseModeBarEndCenter,
		BacksweepDeg: 20, UpsweepDeg: 4, BendStartMm: 135, GripInsetMm: 70,
	})
	if err != nil {
		panic(err)
	}
	return bar
}

func sqlab311Bar() assembly.Handlebar {
	bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm: 740, CurrentWidthMm: 740, StatedRiseMm: 50, RiseMode: assembly.RiseModeStandardKnuckle,
		BacksweepDeg: 16, UpsweepDeg: 5, BendStartMm: 100.0, GripInsetMm: 70,
	})
	if err != nil {
		panic(err)
	}
	return bar
}

func fixtureSeatpost() assembly.RigidPost {
	post, err := assembly.NewRigidPost(350, 100, 250, 12, 30)
	if err != nil {
		panic(err)
	}
	return post
}

func fixtureSaddle() assembly.Saddle {
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm: 32, RailCenterToSitBonesMm: -48, RailToBrpHeightMm: 45,
		SitBonesElevationMm: 10, RailLengthMm: 60,
	})
	if err != nil {
		panic(err)
	}
	return saddle
}

func fixtureRequest() compare.Request {
	brp := pose.NewPoint(assembly.DynamicWorld, -219, 713.1)
	grip := pose.NewPoint(assembly.DynamicWorld, 481, 726)
	return compare.Request{
		Frames:            fixtureFrames(),
		Wheels:            assembly.WheelSetup{RadiusMm: 372},
		Seatpost:          fixtureSeatpost(),
		Saddle:            fixtureSaddle(),
		DynamicRailPitch:  0,
		UpperStackMm:      assembly.DefaultUpperStackMm,
		MaxSpacersMm:      assembly.DefaultMaxSpacersMm,
		StemClampHeightMm: 40,
		Stems:             fixtureStems(),
		SpacerStepMm:      5,
		Handlebars:        []assembly.Handlebar{persuaderBar()},
		HasBarOptions:     false,
		DynamicBarRoll:    0,
		Target:            assembly.FitTarget{BRP: &brp, HandGrip: &grip},
		Weights: assembly.ScoreWeights{
			HandGrip: 6, SaddleBrpY: 6, SaddleBrpX: 4, Stem: 5, HandlebarSweep: 3, ForkSag: 3,
			Spacers: 2, SaddleRailOffset: 2, RcWbRatio: 2, SeatpostInsertion: 1, LeveledBbDrop: 1, Trail: 1,
		},
		TopPerFrame: 0,
		Top:         0,
	}
}

type wantRow struct {
	frameID        string
	size           string
	forkID         string
	sagPct         float64
	tier           assembly.FitTier
	total          float64
	gripR          float64
	stemLength     float64
	stemAngle      pose.Deg
	spacers        float64
	checkBarIndex  bool
	handlebarIndex int
}

const (
	krampusFrameID = "surly-krampus"
	fenrirFrameID  = "otso-fenrir-stainless"
	krampusForkID  = "surly_krampus_483"
	foxForkID      = "fox34_120-534x44"
	foxFork51ID    = "fox34_120-534x51"
	lithicForkID   = "lithic_carbon_mountain-495x44"
	lithicFork51ID = "lithic_carbon_mountain-498x51.5"
	sidForkID      = "sid_100-506x44"
	sidFork51ID    = "sid_100-506x51"
)

func assertNear(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > tol {
		t.Fatalf("%s = %.9f, want %.9f ± %g", label, got, want, tol)
	}
}

func assertRows(t *testing.T, got []compare.Row, want []wantRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(Rows) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		g := got[i]
		w := want[i]
		if g.FrameID != w.frameID || g.Size != w.size || g.ForkID != w.forkID || g.SagPct != w.sagPct {
			t.Fatalf("row %d identity = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				i, g.FrameID, g.Size, g.ForkID, g.SagPct, w.frameID, w.size, w.forkID, w.sagPct)
		}
		if g.Build.Score.Tier != w.tier {
			t.Fatalf("row %d tier = %v, want %v", i, g.Build.Score.Tier, w.tier)
		}
		assertNear(t, "Total", g.Build.Score.Total, w.total, 1e-6)
		assertNear(t, "GripR", g.Build.Score.GripR, w.gripR, 1e-6)
		stem := g.Build.Placement.Setup.Stem
		if stem.LengthMm != w.stemLength || stem.AngleDeg != w.stemAngle || stem.SpacersMm != w.spacers {
			t.Fatalf("row %d stem = (%v mm, %v°, %v mm spacers), want (%v mm, %v°, %v mm spacers)",
				i, stem.LengthMm, stem.AngleDeg, stem.SpacersMm, w.stemLength, w.stemAngle, w.spacers)
		}
		if w.checkBarIndex && g.Build.HandlebarIndex != w.handlebarIndex {
			t.Fatalf("row %d HandlebarIndex = %d, want %d", i, g.Build.HandlebarIndex, w.handlebarIndex)
		}
	}
}

func TestRankFullFixture(t *testing.T) {
	ranking, err := compare.Rank(fixtureRequest())
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if ranking.RowCount != 26 {
		t.Fatalf("RowCount = %d, want 26", ranking.RowCount)
	}
	if ranking.Evaluated != 1820 {
		t.Fatalf("Evaluated = %d, want 1820", ranking.Evaluated)
	}
	for i, row := range ranking.Rows {
		if row.Evaluated != 70 {
			t.Fatalf("row %d Evaluated = %d, want 70", i, row.Evaluated)
		}
	}
	assertNear(t, "row 1 RailOffsetMm", ranking.Rows[0].Build.Placement.Setup.Saddle.RailOffsetMm, 13.233528556, 1e-9)
	if len(ranking.Rows) != 26 {
		t.Fatalf("len(Rows) = %d, want 26", len(ranking.Rows))
	}
	checkRow := func(idx int, frameID, size, forkID string, sagPct float64, tier assembly.FitTier, total, gripR, stemLen float64, stemAng pose.Deg, spacers float64) {
		t.Helper()
		assertRows(t, []compare.Row{ranking.Rows[idx]}, []wantRow{
			{frameID, size, forkID, sagPct, tier, total, gripR, stemLen, stemAng, spacers, false, 0},
		})
	}
	checkRow(0, krampusFrameID, "XL", foxForkID, 15, assembly.FitsAll, 82.940355, 0.309902, 110, -17, 5)
	checkRow(1, krampusFrameID, "XL", foxForkID, 20, assembly.FitsAll, 82.603558, 0.227208, 110, -17, 10)
	checkRow(2, krampusFrameID, "XL", foxFork51ID, 15, assembly.FitsAll, 82.036517, 0.241906, 110, -17, 5)
	checkRow(3, krampusFrameID, "XL", foxFork51ID, 20, assembly.FitsAll, 81.324723, 0.121191, 110, -17, 10)
	checkRow(4, krampusFrameID, "XL", foxForkID, 25, assembly.FitsAll, 81.281365, 0.186563, 110, -17, 15)
	checkRow(5, krampusFrameID, "XL", foxFork51ID, 25, assembly.FitsAll, 79.863568, 0.007809, 110, -17, 15)
	checkRow(9, krampusFrameID, "L", foxForkID, 20, assembly.FitsBRP, 78.131937, 1.015032, 120, -17, 5)
	checkRow(12, krampusFrameID, "XL", krampusForkID, 0, assembly.FitsGrip, 76.852325, 0.741170, 90, -17, 15)
	checkRow(14, fenrirFrameID, "XL", sidForkID, 20, assembly.FitsGrip, 68.479876, 0.578741, 120, -30, 0)
	checkRow(21, fenrirFrameID, "L", sidForkID, 20, assembly.FitsNeither, 58.531310, 1.616431, 120, -20, 0)
	checkRow(25, fenrirFrameID, "L", lithicForkID, 0, assembly.FitsNeither, 54.271845, 1.897639, 120, -30, 0)
}

func TestRankRowsMatchFitFrontBuilds(t *testing.T) {
	req := fixtureRequest()
	ranking, err := compare.Rank(req)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	frameByID := make(map[string]compare.Frame)
	for _, f := range req.Frames {
		frameByID[f.ID] = f
	}
	frontOpts := assembly.FrontBuildOptions{
		StemClampHeightMm: req.StemClampHeightMm,
		Stems:             req.Stems,
		SpacerStepMm:      req.SpacerStepMm,
		Handlebars:        req.Handlebars,
		HasHandlebarOpts:  req.HasBarOptions,
		DynamicBarRollDeg: req.DynamicBarRoll,
	}

	for i, row := range ranking.Rows {
		f := frameByID[row.FrameID]
		var framePart assembly.Frame
		for _, sz := range f.Sizes {
			if sz.Name == row.Size {
				framePart = sz.Frame
				break
			}
		}
		var forkItem compare.Fork
		for _, fk := range f.Forks {
			if fk.ID == row.ForkID {
				forkItem = fk
				break
			}
		}

		fk, err := forkItem.Build(row.SagPct)
		if err != nil {
			t.Fatalf("row %d Build: %v", i, err)
		}
		fs, err := assembly.MateFrameset(framePart, framePart.DefaultLowerCup(), req.UpperStackMm, fk, req.Wheels)
		if err != nil {
			t.Fatalf("row %d MateFrameset: %v", i, err)
		}
		rear, err := assembly.OpenRearCockpit(fs, req.Seatpost, req.Saddle, req.DynamicRailPitch)
		if err != nil {
			t.Fatalf("row %d OpenRearCockpit: %v", i, err)
		}
		seated, _, err := rear.FitToBRP(*req.Target.BRP)
		if err != nil {
			t.Fatalf("row %d FitToBRP: %v", i, err)
		}
		want, err := assembly.FitFrontBuilds(seated, req.Target, frontOpts, req.Weights, 1)
		if err != nil {
			t.Fatalf("row %d FitFrontBuilds: %v", i, err)
		}
		if !reflect.DeepEqual(row.Build, want.Builds[0]) {
			t.Fatalf("row %d Build != FitFrontBuilds[0]", i)
		}
		if row.Evaluated != want.Evaluated {
			t.Fatalf("row %d Evaluated = %d, want %d", i, row.Evaluated, want.Evaluated)
		}
	}
}

func TestRankAllHandlebars(t *testing.T) {
	req := fixtureRequest()
	req.Handlebars = []assembly.Handlebar{persuaderBar(), sqlab311Bar()}
	req.HasBarOptions = true
	ranking, err := compare.Rank(req)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if ranking.RowCount != 26 {
		t.Fatalf("RowCount = %d, want 26", ranking.RowCount)
	}
	if ranking.Evaluated != 3640 {
		t.Fatalf("Evaluated = %d, want 3640", ranking.Evaluated)
	}
	for i, row := range ranking.Rows {
		if !row.Build.HasHandlebarOption {
			t.Fatalf("row %d HasHandlebarOption = false, want true", i)
		}
	}
	assertRows(t, ranking.Rows[:1], []wantRow{
		{krampusFrameID, "XL", foxForkID, 25, assembly.FitsAll, 84.089299, 0.647636, 90, -17, 0, true, 1},
	})
}

func TestRankPrunes(t *testing.T) {
	fullReq := fixtureRequest()
	fullRanking, err := compare.Rank(fullReq)
	if err != nil {
		t.Fatalf("full Rank: %v", err)
	}
	all := fullRanking.Rows
	firstFenrir := slices.IndexFunc(all, func(r compare.Row) bool { return r.FrameID == fenrirFrameID })

	cases := []struct {
		name        string
		topPerFrame int
		top         int
		want        []compare.Row
	}{
		{"top_per_frame=1, top=0", 1, 0, []compare.Row{all[0], all[firstFenrir]}},
		{"top_per_frame=0, top=3", 0, 3, all[:3]},
		{"top_per_frame=2, top=3", 2, 3, []compare.Row{all[0], all[1], all[firstFenrir]}},
		{"top_per_frame=1, top=1", 1, 1, all[:1]},
		{"top_per_frame=0, top=100", 0, 100, all},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := fixtureRequest()
			req.TopPerFrame = tc.topPerFrame
			req.Top = tc.top
			ranking, err := compare.Rank(req)
			if err != nil {
				t.Fatalf("Rank: %v", err)
			}
			if ranking.RowCount != 26 {
				t.Fatalf("RowCount = %d, want 26", ranking.RowCount)
			}
			if ranking.Evaluated != 1820 {
				t.Fatalf("Evaluated = %d, want 1820", ranking.Evaluated)
			}
			if !reflect.DeepEqual(ranking.Rows, tc.want) {
				t.Fatalf("pruned rows mismatch")
			}
		})
	}
}

func TestRankRejectsInvalidRequests(t *testing.T) {
	deepPost, err := assembly.NewRigidPost(350, 300, 320, 12, 30)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*compare.Request)
		want   []string
	}{
		{"no frames", func(r *compare.Request) { r.Frames = nil }, []string{"compare needs at least one frame"}},
		{"empty frame ID", func(r *compare.Request) { r.Frames[0].ID = "" }, []string{"frame 0 has an empty ID"}},
		{"duplicate frame", func(r *compare.Request) { r.Frames[1].ID = krampusFrameID }, []string{`duplicate frame "surly-krampus"`}},
		{"no sizes", func(r *compare.Request) { r.Frames[0].Sizes = nil }, []string{`frame "surly-krampus" has no sizes`}},
		{"no forks", func(r *compare.Request) { r.Frames[0].Forks = nil }, []string{`frame "surly-krampus" has no forks`}},
		{"empty size name", func(r *compare.Request) { r.Frames[0].Sizes[1].Name = "" }, []string{`frame "surly-krampus" size 1 has an empty name`}},
		{"duplicate size", func(r *compare.Request) { r.Frames[0].Sizes[1].Name = "L" }, []string{`frame "surly-krampus" has duplicate size "L"`}},
		{"empty fork ID", func(r *compare.Request) { r.Frames[0].Forks[1].ID = "" }, []string{`frame "surly-krampus" fork 1 has an empty ID`}},
		{"duplicate fork", func(r *compare.Request) { r.Frames[0].Forks[1].ID = krampusForkID }, []string{`frame "surly-krampus" has duplicate fork "surly_krampus_483"`}},
		{"missing fork builder", func(r *compare.Request) { r.Frames[0].Forks[0].Build = nil }, []string{"has no fork builder"}},
		{"unsupported travel", func(r *compare.Request) { r.Frames[0].Forks[1].Travel = assembly.ForkTravel(110) }, []string{"has unsupported travel 110 mm"}},
		{"no seatpost", func(r *compare.Request) { r.Seatpost = nil }, []string{"compare needs a seatpost"}},
		{"no handlebars", func(r *compare.Request) { r.Handlebars = nil }, []string{"compare needs at least one handlebar"}},
		{"no BRP", func(r *compare.Request) { r.Target.BRP = nil }, []string{"compare needs a BRP target"}},
		{"no HandGrip", func(r *compare.Request) { r.Target.HandGrip = nil }, []string{"compare needs a HandGrip target"}},
		{"negative top per frame", func(r *compare.Request) { r.TopPerFrame = -1 }, []string{"top per frame must be 0 or more, got -1"}},
		{"negative top", func(r *compare.Request) { r.Top = -1 }, []string{"top must be 0 or more, got -1"}},
		{"seatpost too deep for a size", func(r *compare.Request) {
			r.Seatpost = deepPost
		}, []string{`frame "otso-fenrir-stainless" size "L" fork "lithic_carbon_mountain-495x44" at 0% sag:`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := fixtureRequest()
			tc.mutate(&req)
			_, err := compare.Rank(req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			msg := err.Error()
			for _, sub := range tc.want {
				if !strings.Contains(msg, sub) {
					t.Fatalf("error %q does not contain %q", msg, sub)
				}
			}
		})
	}
}

// TestRankRejectsInvalidForkLowerCups checks that MateFrameset's canonical SHIS
// validation rejects every illegal per-fork lower headset cup.
func TestRankRejectsInvalidForkLowerCups(t *testing.T) {
	cup := func(std assembly.LowerCupStandard, stack float64) assembly.LowerCupSetup {
		return assembly.LowerCupSetup{Standard: std, StackMm: stack}
	}
	cases := []struct {
		name     string
		forkIdx  int
		lowerCup assembly.LowerCupSetup
		want     string
	}{
		{"bare head tube", 0, cup(assembly.LowerCupBareHeadTube, 0), "needs a physical lower cup"},
		{"unknown standard", 0, cup("nope", 4), "unknown or missing lower headset cup standard"},
		{"negative stack", 0, cup(assembly.LowerCupZS44, -1), "non-negative"},
		{"NaN stack", 0, cup(assembly.LowerCupZS44, math.NaN()), "finite"},
		{"Inf stack", 0, cup(assembly.LowerCupZS44, math.Inf(1)), "finite"},
		{"wrong standard for bore", 0, cup(assembly.LowerCupEC34, 12), "needs a 34_pressed head tube bore"},
		{"ZS44 with tapered steerer", 1, cup(assembly.LowerCupZS44, 4), "does not fit a tapered steerer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := fixtureRequest()
			req.Frames[0].Forks[tc.forkIdx].LowerCup = tc.lowerCup
			_, err := compare.Rank(req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestRankUsesForkLowerCup checks that a straight fork with ZS44 and a tapered
// fork with EC44 each place with their own cup on one 44 mm frame.
func TestRankUsesForkLowerCup(t *testing.T) {
	req := fixtureRequest()
	req.Frames = req.Frames[:1]
	req.Frames[0].Forks[0].LowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: 4}
	req.Frames[0].Forks[1].LowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	ranking, err := compare.Rank(req)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]assembly.LowerCupSetup{}
	for _, row := range ranking.Rows {
		seen[row.ForkID] = row.Build.Placement.Setup.LowerCup
	}
	if got := seen[krampusForkID]; got.Standard != assembly.LowerCupZS44 || got.StackMm != 4 {
		t.Errorf("straight fork cup = %+v, want ZS44 4", got)
	}
	if got := seen[foxForkID]; got.Standard != assembly.LowerCupEC44 || got.StackMm != 12 {
		t.Errorf("tapered fork cup = %+v, want EC44 12", got)
	}
}
