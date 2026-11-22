package compare_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/compare"
	"bikegeo/internal/reboot/pose"
)

// All fixture values below are copied from the catalog definitions for test isolation.
func fixtureFrames() []compare.Frame {
	ec44 := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12}
	return []compare.Frame{
		{
			ID: "surly-krampus",
			Sizes: []compare.Size{
				{
					Name: "L",
					Frame: assembly.FrameSetup{
						ReachMm: 448, StackMm: 612, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 73,
						HeadTubeLengthMm: 110, BbDropMm: 65, ChainstayMm: 435, SeatTubeLengthMm: 470,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					},
				},
				{
					Name: "XL",
					Frame: assembly.FrameSetup{
						ReachMm: 470, StackMm: 621, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 73,
						HeadTubeLengthMm: 120, BbDropMm: 65, ChainstayMm: 435, SeatTubeLengthMm: 521,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					},
				},
			},
			Forks: []compare.Fork{
				{ID: "surly_krampus_483", Spec: assembly.ForkSpec{StaticA2cMm: 483, TravelMm: 0, RakeMm: 47, Steerer: assembly.SteererStraight118}},
				{ID: "fox34_120", Spec: assembly.ForkSpec{StaticA2cMm: 530, TravelMm: 120, RakeMm: 44, Steerer: assembly.SteererTapered}},
			},
		},
		{
			ID: "otso-fenrir-stainless",
			Sizes: []compare.Size{
				{
					Name: "L",
					Frame: assembly.FrameSetup{
						ReachMm: 428, StackMm: 642, HeadTubeAngleDeg: 68.25, SeatTubeAngleDeg: 75.25,
						HeadTubeLengthMm: 140, BbDropMm: 72, ChainstayMm: 450, SeatTubeLengthMm: 490,
						MaxPostInsertionMm: 290, LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					},
				},
				{
					Name: "XL",
					Frame: assembly.FrameSetup{
						ReachMm: 441, StackMm: 660, HeadTubeAngleDeg: 68.25, SeatTubeAngleDeg: 75.25,
						HeadTubeLengthMm: 160, BbDropMm: 72, ChainstayMm: 450, SeatTubeLengthMm: 530,
						LowerBore: assembly.Bore44Pressed, DefaultLowerCup: ec44,
					},
				},
			},
			Forks: []compare.Fork{
				{ID: "lithic_carbon_mountain", Spec: assembly.ForkSpec{StaticA2cMm: 495, TravelMm: 0, RakeMm: 54, Steerer: assembly.SteererTapered}},
				{ID: "sid_100", Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererTapered}},
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

func persuaderBar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm: 800, CurrentWidthMm: 800, StatedRiseMm: 50, RiseMode: assembly.RiseModeBarEndCenter,
		BacksweepDeg: 20, UpsweepDeg: 4, BendStartMm: 135, GripInsetMm: 70, BarRollDeg: 0,
	}
}

func sqlab311Bar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm: 740, CurrentWidthMm: 740, StatedRiseMm: 50, RiseMode: assembly.RiseModeStandardKnuckle,
		BacksweepDeg: 16, UpsweepDeg: 5, BendStartMm: 100.0, GripInsetMm: 70, BarRollDeg: 0,
	}
}

func fixtureRequest() compare.Request {
	brp := pose.Point{X: -219, Y: 713.1}
	grip := pose.Point{X: 481, Y: 726}
	return compare.Request{
		Frames:            fixtureFrames(),
		SuspensionSagPcts: []float64{15, 20, 25},
		Wheels:            assembly.WheelSetup{RadiusMm: 372},
		Seatpost: assembly.SeatpostSpec{
			TotalLengthMm: 350, MaxInsertionMm: 250, MinInsertionMm: 100, SetbackMm: 12, RailClampLengthMm: 30,
		},
		Saddle: assembly.SaddleSetup{
			RailCenterToBrpMm: 32, RailCenterToSitBonesMm: -48, RailToBrpHeightMm: 45,
			SitBonesElevationMm: 10, RailLengthMm: 60, RailPitchDeg: 0, RailOffsetMm: 0,
		},
		Stem: assembly.StemSetup{
			UpperStackMm: assembly.DefaultUpperStackMm, SpacersMm: 20, ClampHeightMm: 40,
			LengthMm: 60, AngleDeg: -6, MaxSpacersMm: assembly.DefaultMaxSpacersMm,
			MinLengthMm: 40, MaxLengthMm: 130,
		},
		Handlebar: persuaderBar(),
		Target:    assembly.FitTarget{BRP: &brp, HandGrip: &grip},
		Parts:     assembly.PartOptions{Stems: fixtureStems(), SpacerStepMm: 5},
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
	foxForkID      = "fox34_120"
	lithicForkID   = "lithic_carbon_mountain"
	sidForkID      = "sid_100"
)

var t1WantRows = []wantRow{
	{krampusFrameID, "XL", foxForkID, 20, assembly.FitsAll, 81.951358, 0.146116, 110, -17, 10, false, 0},
	{krampusFrameID, "L", foxForkID, 20, assembly.FitsAll, 79.262788, 0.850141, 120, -10, 0, false, 0},
	{krampusFrameID, "XL", foxForkID, 15, assembly.FitsAll, 79.151692, 0.254290, 110, -17, 5, false, 0},
	{krampusFrameID, "XL", foxForkID, 25, assembly.FitsAll, 76.070832, 0.089092, 110, -17, 15, false, 0},
	{krampusFrameID, "L", foxForkID, 25, assembly.FitsAll, 73.659999, 0.613347, 120, -10, 0, false, 0},
	{krampusFrameID, "L", foxForkID, 15, assembly.FitsBRP, 73.775410, 1.069619, 120, -17, 5, false, 0},
	{krampusFrameID, "XL", krampusForkID, 0, assembly.FitsGrip, 76.852325, 0.741170, 90, -17, 15, false, 0},
	{krampusFrameID, "L", krampusForkID, 0, assembly.FitsGrip, 72.694567, 0.245436, 120, -10, 15, false, 0},
	{fenrirFrameID, "XL", sidForkID, 20, assembly.FitsGrip, 68.479876, 0.578741, 120, -30, 0, false, 0},
	{fenrirFrameID, "XL", lithicForkID, 0, assembly.FitsGrip, 67.571942, 0.811560, 120, -30, 0, false, 0},
	{fenrirFrameID, "XL", sidForkID, 25, assembly.FitsGrip, 64.472103, 0.483405, 120, -30, 0, false, 0},
	{fenrirFrameID, "XL", sidForkID, 15, assembly.FitsGrip, 63.915801, 0.817186, 120, -30, 0, false, 0},
	{fenrirFrameID, "L", sidForkID, 25, assembly.FitsNeither, 58.827962, 1.318877, 120, -20, 0, false, 0},
	{fenrirFrameID, "L", sidForkID, 20, assembly.FitsNeither, 58.531310, 1.616431, 120, -20, 0, false, 0},
	{fenrirFrameID, "L", lithicForkID, 0, assembly.FitsNeither, 55.768206, 1.764743, 120, -30, 0, false, 0},
	{fenrirFrameID, "L", sidForkID, 15, assembly.FitsNeither, 52.097664, 1.767563, 120, -30, 0, false, 0},
}

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
	if ranking.RowCount != 16 {
		t.Fatalf("RowCount = %d, want 16", ranking.RowCount)
	}
	if ranking.Evaluated != 1120 {
		t.Fatalf("Evaluated = %d, want 1120", ranking.Evaluated)
	}
	for i, row := range ranking.Rows {
		if row.Evaluated != 70 {
			t.Fatalf("row %d Evaluated = %d, want 70", i, row.Evaluated)
		}
		assertNear(t, "RailOffsetMm", row.Build.Placement.Setup.Saddle.RailOffsetMm, 15, 1e-9)
	}
	assertRows(t, ranking.Rows, t1WantRows)
	assertNear(t, "row 1 InsertionMm", ranking.Rows[0].Build.Placement.Setup.Seatpost.InsertionMm, 163.906090, 1e-6)
	assertNear(t, "row 15 InsertionMm", ranking.Rows[14].Build.Placement.Setup.Seatpost.InsertionMm, 144.107252, 1e-6)
}

func TestRankRowsMatchFitBuilds(t *testing.T) {
	req := fixtureRequest()
	ranking, err := compare.Rank(req)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	frameByID := make(map[string]compare.Frame)
	for _, f := range req.Frames {
		frameByID[f.ID] = f
	}

	for i, row := range ranking.Rows {
		f := frameByID[row.FrameID]
		var frameSetup assembly.FrameSetup
		for _, sz := range f.Sizes {
			if sz.Name == row.Size {
				frameSetup = sz.Frame
				break
			}
		}
		var forkSpec assembly.ForkSpec
		for _, fk := range f.Forks {
			if fk.ID == row.ForkID {
				forkSpec = fk.Spec
				break
			}
		}

		bounds, err := assembly.SeatpostInsertionBounds(frameSetup, req.Seatpost)
		if err != nil {
			t.Fatalf("row %d SeatpostInsertionBounds: %v", i, err)
		}
		setup := assembly.BikeSetup{
			Frame:     frameSetup,
			LowerCup:  frameSetup.DefaultLowerCup,
			Fork:      assembly.ForkSetup{Spec: forkSpec, SagPct: row.SagPct},
			Wheels:    req.Wheels,
			Seatpost:  assembly.SeatpostSetup{Spec: req.Seatpost, InsertionMm: bounds.EffectiveMinInsertionMm},
			Saddle:    req.Saddle,
			Stem:      req.Stem,
			Handlebar: req.Handlebar,
		}
		want, err := assembly.FitBuilds(setup, req.Target, req.Parts, req.Weights, 1)
		if err != nil {
			t.Fatalf("row %d FitBuilds: %v", i, err)
		}
		if !reflect.DeepEqual(row.Build, want.Builds[0]) {
			t.Fatalf("row %d Build != FitBuilds[0]", i)
		}
		if row.Evaluated != want.Evaluated {
			t.Fatalf("row %d Evaluated = %d, want %d", i, row.Evaluated, want.Evaluated)
		}
	}
}

func TestRankAllHandlebars(t *testing.T) {
	req := fixtureRequest()
	req.Parts.Handlebars = []assembly.HandlebarSetup{persuaderBar(), sqlab311Bar()}
	ranking, err := compare.Rank(req)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if ranking.RowCount != 16 {
		t.Fatalf("RowCount = %d, want 16", ranking.RowCount)
	}
	if ranking.Evaluated != 2240 {
		t.Fatalf("Evaluated = %d, want 2240", ranking.Evaluated)
	}
	for i, row := range ranking.Rows {
		if !row.Build.HasHandlebarOption {
			t.Fatalf("row %d HasHandlebarOption = false, want true", i)
		}
	}
	want := []wantRow{
		{krampusFrameID, "XL", foxForkID, 20, assembly.FitsAll, 84.481849, 0.773655, 90, -17, 0, true, 1},
		{krampusFrameID, "L", foxForkID, 20, assembly.FitsAll, 80.065939, 0.514084, 110, -17, 0, true, 1},
		{krampusFrameID, "XL", foxForkID, 15, assembly.FitsAll, 79.151692, 0.254290, 110, -17, 5, true, 0},
		{krampusFrameID, "XL", foxForkID, 25, assembly.FitsAll, 79.110589, 0.411071, 90, -17, 0, true, 1},
		{krampusFrameID, "L", foxForkID, 15, assembly.FitsAll, 76.661843, 0.915548, 110, -17, 0, true, 1},
		{krampusFrameID, "L", foxForkID, 25, assembly.FitsAll, 74.677138, 0.148898, 110, -17, 0, true, 1},
		{krampusFrameID, "XL", krampusForkID, 0, assembly.FitsGrip, 77.253680, 0.500357, 90, -17, 10, true, 1},
		{krampusFrameID, "L", krampusForkID, 0, assembly.FitsGrip, 75.370042, 0.931818, 90, -17, 0, true, 1},
		{fenrirFrameID, "L", sidForkID, 20, assembly.FitsGrip, 68.546221, 0.538934, 120, -30, 0, true, 1},
		{fenrirFrameID, "XL", sidForkID, 20, assembly.FitsGrip, 68.479876, 0.578741, 120, -30, 0, true, 0},
		{fenrirFrameID, "XL", lithicForkID, 0, assembly.FitsGrip, 67.571942, 0.811560, 120, -30, 0, true, 0},
		{fenrirFrameID, "L", lithicForkID, 0, assembly.FitsGrip, 67.511970, 0.836426, 120, -30, 0, true, 1},
		{fenrirFrameID, "L", sidForkID, 25, assembly.FitsGrip, 64.755971, 0.313084, 120, -30, 0, true, 1},
		{fenrirFrameID, "XL", sidForkID, 25, assembly.FitsGrip, 64.472103, 0.483405, 120, -30, 0, true, 0},
		{fenrirFrameID, "XL", sidForkID, 15, assembly.FitsGrip, 63.915801, 0.817186, 120, -30, 0, true, 0},
		{fenrirFrameID, "L", sidForkID, 15, assembly.FitsGrip, 63.873308, 0.842682, 120, -30, 0, true, 1},
	}
	assertRows(t, ranking.Rows, want)
}

func TestRankPrunes(t *testing.T) {
	pick := func(indices ...int) []wantRow {
		out := make([]wantRow, len(indices))
		for i, idx := range indices {
			out[i] = t1WantRows[idx-1]
		}
		return out
	}

	cases := []struct {
		name        string
		topPerFrame int
		top         int
		want        []wantRow
	}{
		{"top_per_frame=1, top=0", 1, 0, pick(1, 9)},
		{"top_per_frame=0, top=3", 0, 3, pick(1, 2, 3)},
		{"top_per_frame=2, top=3", 2, 3, pick(1, 2, 9)},
		{"top_per_frame=1, top=1", 1, 1, pick(1)},
		{"top_per_frame=0, top=100", 0, 100, t1WantRows},
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
			if ranking.RowCount != 16 {
				t.Fatalf("RowCount = %d, want 16", ranking.RowCount)
			}
			if ranking.Evaluated != 1120 {
				t.Fatalf("Evaluated = %d, want 1120", ranking.Evaluated)
			}
			assertRows(t, ranking.Rows, tc.want)
		})
	}
}

func TestRankSelections(t *testing.T) {
	sag := func(v float64) *float64 { return &v }
	pick := func(indices ...int) []wantRow {
		out := make([]wantRow, len(indices))
		for i, idx := range indices {
			out[i] = t1WantRows[idx-1]
		}
		return out
	}

	cases := []struct {
		name        string
		selections  []compare.Selection
		topPerFrame int
		wantCount   int
		wantRows    []wantRow
	}{
		{
			name:       "one fork, open sag",
			selections: []compare.Selection{{FrameID: krampusFrameID, Size: "XL", ForkID: foxForkID}},
			wantCount:  3,
			wantRows:   pick(1, 3, 4),
		},
		{
			name:       "one fork, all sizes",
			selections: []compare.Selection{{FrameID: fenrirFrameID, ForkID: lithicForkID}},
			wantCount:  2,
			wantRows:   pick(10, 15),
		},
		{
			name:       "frame only",
			selections: []compare.Selection{{FrameID: krampusFrameID}},
			wantCount:  8,
			wantRows:   pick(1, 2, 3, 4, 5, 6, 7, 8),
		},
		{
			name: "duplicates ranked once",
			selections: []compare.Selection{
				{FrameID: krampusFrameID, Size: "XL", ForkID: foxForkID},
				{FrameID: fenrirFrameID, ForkID: lithicForkID},
				{FrameID: krampusFrameID, Size: "XL", ForkID: foxForkID},
			},
			wantCount: 5,
			wantRows:  pick(1, 3, 4, 10, 15),
		},
		{
			name: "per-frame pruning on a selection",
			selections: []compare.Selection{
				{FrameID: krampusFrameID, Size: "XL", ForkID: foxForkID},
				{FrameID: fenrirFrameID, ForkID: lithicForkID},
			},
			topPerFrame: 1,
			wantCount:   5,
			wantRows:    pick(1, 10),
		},
		{
			name:       "explicit sag",
			selections: []compare.Selection{{FrameID: krampusFrameID, Size: "L", ForkID: foxForkID, SagPct: sag(22)}},
			wantCount:  1,
			wantRows: []wantRow{
				{krampusFrameID, "L", foxForkID, 22, assembly.FitsAll, 77.053503, 0.736472, 120, -10, 0, false, 0},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := fixtureRequest()
			req.Selections = tc.selections
			req.TopPerFrame = tc.topPerFrame
			ranking, err := compare.Rank(req)
			if err != nil {
				t.Fatalf("Rank: %v", err)
			}
			if ranking.RowCount != tc.wantCount {
				t.Fatalf("RowCount = %d, want %d", ranking.RowCount, tc.wantCount)
			}
			assertRows(t, ranking.Rows, tc.wantRows)
			if tc.name == "explicit sag" {
				if ranking.Evaluated != 70 {
					t.Fatalf("Evaluated = %d, want 70", ranking.Evaluated)
				}
				assertNear(t, "InsertionMm", ranking.Rows[0].Build.Placement.Setup.Seatpost.InsertionMm, 113.339366, 1e-6)
			}
		})
	}
}

func TestRankRejectsInvalidRequests(t *testing.T) {
	sag := func(v float64) *float64 { return &v }
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
		{"basis fork", func(r *compare.Request) { r.Frames[0].Forks[0].Spec.Steerer = assembly.SteererBasis }, []string{"is the chart basis fork"}},
		{"no sag steps", func(r *compare.Request) { r.SuspensionSagPcts = nil }, []string{"compare needs suspension sag steps"}},
		{"no BRP", func(r *compare.Request) { r.Target.BRP = nil }, []string{"compare needs a BRP target"}},
		{"no HandGrip", func(r *compare.Request) { r.Target.HandGrip = nil }, []string{"compare needs a HandGrip target"}},
		{"fork options", func(r *compare.Request) { r.Parts.Forks = []assembly.ForkOption{{}} }, []string{"part options must not list forks"}},
		{"negative top per frame", func(r *compare.Request) { r.TopPerFrame = -1 }, []string{"top per frame must be 0 or more, got -1"}},
		{"negative top", func(r *compare.Request) { r.Top = -1 }, []string{"top must be 0 or more, got -1"}},
		{"unknown frame", func(r *compare.Request) { r.Selections = []compare.Selection{{FrameID: "nope"}} }, []string{`unknown frame "nope"`}},
		{"unknown size", func(r *compare.Request) { r.Selections = []compare.Selection{{FrameID: krampusFrameID, Size: "M"}} }, []string{`frame "surly-krampus" has no size "M"`}},
		{"unknown fork", func(r *compare.Request) {
			r.Selections = []compare.Selection{{FrameID: krampusFrameID, ForkID: sidForkID}}
		}, []string{`frame "surly-krampus" has no fork "sid_100"`}},
		{"sag without fork", func(r *compare.Request) {
			r.Selections = []compare.Selection{{FrameID: krampusFrameID, Size: "L", SagPct: sag(20)}}
		}, []string{`selection on frame "surly-krampus" sets sag without a fork`}},
		{"rigid fork with sag", func(r *compare.Request) {
			r.Selections = []compare.Selection{{FrameID: krampusFrameID, Size: "L", ForkID: krampusForkID, SagPct: sag(5)}}
		}, []string{`frame "surly-krampus" size "L" fork "surly_krampus_483" at 5% sag:`, "rigid fork"}},
		{"seatpost too deep for a size", func(r *compare.Request) {
			r.Seatpost.MinInsertionMm = 300
			r.Seatpost.MaxInsertionMm = 320
		}, []string{`frame "otso-fenrir-stainless" size "L" fork "lithic_carbon_mountain" at 0% sag:`}},
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
