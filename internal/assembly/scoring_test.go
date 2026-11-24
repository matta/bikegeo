package assembly_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/pose"
)

func testDefaultScoreWeights() assembly.ScoreWeights {
	return assembly.ScoreWeights{
		HandGrip: 6, SaddleBrpY: 6, SaddleBrpX: 4, Stem: 5, HandlebarSweep: 3, ForkSag: 3,
		Spacers: 2, SaddleRailOffset: 2, RcWbRatio: 2, SeatpostInsertion: 1, LeveledBbDrop: 1, Trail: 1,
	}
}

func sidFork(sag float64) assembly.ForkSetup {
	return assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: sag}
}

func fox34Fork(sag float64) assembly.ForkSetup {
	return assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 530, Travel: assembly.Travel120, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: sag}
}

func fox34_130Fork(sag float64) assembly.ForkSetup {
	return assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 544, Travel: assembly.Travel130, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: sag}
}

func basisSetup() assembly.BikeSetup {
	setup := fenrirFittingSetup()
	setup.LowerCup = fenrirBasisCup()
	setup.Fork = assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, Travel: assembly.TravelRigid, RakeMm: 54, Steerer: assembly.SteererBasis}}
	return setup
}

func withCockpit(setup assembly.BikeSetup, length float64, angle pose.Deg, spacers float64) assembly.BikeSetup {
	setup.Stem.LengthMm, setup.Stem.AngleDeg, setup.Stem.SpacersMm = length, angle, spacers
	return setup
}

// scoreExact places setup and scores it against its own placed HandGrip (and
// BRP when withBrp).
func scoreExact(t *testing.T, setup assembly.BikeSetup, withBrp bool, stemScore float64) assembly.BuildScore {
	t.Helper()
	p := gripPlacement(t, setup)
	grip := p.Handlebar.HandGrip.Origin()
	target := assembly.FitTarget{HandGrip: &grip}
	if withBrp {
		brp := p.Saddle.BRP.Origin()
		target.BRP = &brp
	}
	return mustScore(t, p, target, stemScore)
}

func mustScore(t *testing.T, p assembly.BikePlacement, target assembly.FitTarget, stemScore float64) assembly.BuildScore {
	t.Helper()
	s, err := assembly.ScoreBuild(p, target, stemScore, testDefaultScoreWeights())
	if err != nil {
		t.Fatalf("ScoreBuild: %v", err)
	}
	return s
}

func attrScore(t *testing.T, s assembly.BuildScore, name assembly.AttributeName) assembly.AttributeScore {
	t.Helper()
	for _, a := range s.Attributes {
		if a.Attribute == name {
			return a
		}
	}
	t.Fatalf("no attribute %q", name)
	return assembly.AttributeScore{}
}

func TestScoreWeightsValidation(t *testing.T) {
	w := testDefaultScoreWeights()
	if err := w.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	assertNear(t, "active total with BRP", w.ActiveTotal(true), 36, 0)
	assertNear(t, "active total without BRP", w.ActiveTotal(false), 23, 0)

	mutate := func(f func(*assembly.ScoreWeights)) assembly.ScoreWeights {
		w := testDefaultScoreWeights()
		f(&w)
		return w
	}
	for name, bad := range map[string]assembly.ScoreWeights{
		"negative": mutate(func(w *assembly.ScoreWeights) { w.Stem = -1 }),
		"NaN":      mutate(func(w *assembly.ScoreWeights) { w.Trail = math.NaN() }),
		"+Inf":     mutate(func(w *assembly.ScoreWeights) { w.HandGrip = math.Inf(1) }),
		"all zero": {},
	} {
		if bad.Validate() == nil {
			t.Errorf("%s weights passed Validate", name)
		}
	}
}

func TestStemOptionValidation(t *testing.T) {
	for _, s := range []assembly.StemOption{{LengthMm: 70, AngleDeg: 0, Score: 100}, {LengthMm: 120, AngleDeg: -30, Score: 0}} {
		if err := s.Validate(); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
	nan, inf := math.NaN(), math.Inf(1)
	for _, s := range []assembly.StemOption{
		{LengthMm: 0, Score: 50}, {LengthMm: -10, Score: 50}, {LengthMm: nan, Score: 50}, {LengthMm: inf, Score: 50},
		{LengthMm: 70, AngleDeg: pose.Deg(nan), Score: 50}, {LengthMm: 70, AngleDeg: pose.Deg(math.Inf(-1)), Score: 50},
		{LengthMm: 70, Score: -0.1}, {LengthMm: 70, Score: 100.1}, {LengthMm: 70, Score: nan}, {LengthMm: 70, Score: inf},
	} {
		if s.Validate() == nil {
			t.Errorf("%+v passed Validate", s)
		}
	}
}

func TestAttributeCurves(t *testing.T) {
	const tol = 1e-5
	base := fenrirFittingSetup()
	base.Stem.LengthMm, base.Stem.AngleDeg, base.Stem.SpacersMm = 70, 0, 10
	placed := gripPlacement(t, base)
	grip := placed.Handlebar.HandGrip.Origin()
	brp := placed.Saddle.BRP.Origin()

	t.Run("HandGrip", func(t *testing.T) {
		for _, c := range []struct{ r, want float64 }{{0, 100}, {0.5, 95}, {1, 90}, {1.5, 45}, {2, 0}, {3, 0}} {
			// The oval's major axis runs at 60°; 20 mm along it is R = 1.
			target := pose.Point[assembly.DynamicWorld]{X: grip.X + c.r*20*math.Cos(math.Pi/3), Y: grip.Y + c.r*20*math.Sin(math.Pi/3)}
			s := mustScore(t, placed, assembly.FitTarget{HandGrip: &target}, 100)
			assertNear(t, "R", s.GripR, c.r, 1e-9)
			assertNear(t, "score", attrScore(t, s, assembly.AttrHandGrip).Score, c.want, tol)
		}
	})
	t.Run("Saddle BRP", func(t *testing.T) {
		for _, c := range []struct{ d, want float64 }{{0, 100}, {0.5, 100}, {1.25, 75}, {2, 50}, {3, 25}, {4, 0}, {5, 0}} {
			target := pose.Point[assembly.DynamicWorld]{X: brp.X, Y: brp.Y + c.d}
			s := mustScore(t, placed, assembly.FitTarget{HandGrip: &grip, BRP: &target}, 100)
			assertNear(t, "Y score", attrScore(t, s, assembly.AttrSaddleBrpY).Score, c.want, tol)
		}
		for _, c := range []struct{ d, want float64 }{{0, 100}, {1, 100}, {5.5, 75}, {10, 50}, {15, 25}, {20, 0}, {30, 0}} {
			target := pose.Point[assembly.DynamicWorld]{X: brp.X + c.d, Y: brp.Y}
			s := mustScore(t, placed, assembly.FitTarget{HandGrip: &grip, BRP: &target}, 100)
			assertNear(t, "X score", attrScore(t, s, assembly.AttrSaddleBrpX).Score, c.want, tol)
		}
	})
	t.Run("Handlebar sweep", func(t *testing.T) {
		for _, c := range []struct{ deg, want float64 }{{10, 0}, {13, 50}, {16, 100}, {18, 100}, {20, 100}, {23, 50}, {26, 0}} {
			setup := base
			setup.Handlebar.BacksweepDeg = pose.Deg(c.deg)
			s := scoreExact(t, setup, false, 100)
			assertNear(t, "sweep", attrScore(t, s, assembly.AttrHandlebarSweep).Score, c.want, tol)
		}
	})
	t.Run("Fork sag", func(t *testing.T) {
		s := scoreExact(t, base, false, 100)
		assertNear(t, "rigid", attrScore(t, s, assembly.AttrForkSag).Score, 100, tol)
		s = scoreExact(t, basisSetup(), false, 100)
		assertNear(t, "Basis", attrScore(t, s, assembly.AttrForkSag).Score, 100, tol)
		for _, c := range []struct{ sag, want float64 }{
			{0, 0}, {5, 0}, {10, 45}, {15, 90}, {17.5, 95}, {18, 96}, {20, 100}, {22, 80}, {22.5, 75}, {25, 50}, {30, 0}, {35, 0}, {50, 0},
		} {
			setup := base
			setup.Fork = sidFork(c.sag)
			s := scoreExact(t, setup, false, 100)
			assertNear(t, "100mm sag", attrScore(t, s, assembly.AttrForkSag).Score, c.want, tol)
		}
		for _, c := range []struct{ sag, want float64 }{
			{0, 0}, {5, 0}, {10, 45}, {15, 90}, {17.5, 92.5}, {18, 93}, {20, 95}, {22, 97}, {22.5, 97.5}, {25, 100}, {30, 50}, {35, 0}, {50, 0},
		} {
			setup := base
			setup.Fork = fox34Fork(c.sag)
			s := scoreExact(t, setup, false, 100)
			assertNear(t, "120mm sag", attrScore(t, s, assembly.AttrForkSag).Score, c.want, tol)
		}
		for _, c := range []struct{ sag, want float64 }{
			{0, 0}, {5, 0}, {10, 0}, {15, 45}, {17.5, 67.5}, {18, 72}, {20, 90}, {22, 94}, {22.5, 95}, {25, 100}, {30, 50}, {35, 0}, {50, 0},
		} {
			setup := base
			setup.Fork = fox34_130Fork(c.sag)
			s := scoreExact(t, setup, false, 100)
			assertNear(t, "130mm sag", attrScore(t, s, assembly.AttrForkSag).Score, c.want, tol)
		}
	})
	t.Run("Spacers", func(t *testing.T) {
		for _, c := range []struct{ mm, want float64 }{{0, 100}, {10, 100}, {20, 100}, {25, 50}, {30, 0}} {
			setup := base
			setup.Stem.SpacersMm = c.mm
			s := scoreExact(t, setup, false, 100)
			assertNear(t, "spacers", attrScore(t, s, assembly.AttrSpacers).Score, c.want, tol)
		}
	})
	t.Run("Saddle rail offset", func(t *testing.T) {
		for _, c := range []struct{ mm, want float64 }{{-15, 0}, {-10, 50}, {-5, 100}, {0, 100}, {5, 100}, {10, 50}, {15, 0}} {
			setup := base
			setup.Saddle.RailOffsetMm = c.mm
			s := scoreExact(t, setup, true, 100)
			assertNear(t, "rail", attrScore(t, s, assembly.AttrSaddleRailOffset).Score, c.want, tol)
		}
	})
	t.Run("Seatpost insertion", func(t *testing.T) {
		for _, c := range []struct{ mm, want float64 }{{100, 0}, {115, 50}, {130, 100}, {200, 100}, {260, 100}, {275, 50}, {290, 0}} {
			setup := base
			setup.Seatpost.InsertionMm = c.mm
			s := scoreExact(t, setup, true, 100)
			assertNear(t, "insertion", attrScore(t, s, assembly.AttrSeatpostInsertion).Score, c.want, tol)
		}
		degenerate := base
		degenerate.Frame.MaxPostInsertionMm = 100
		degenerate.Seatpost.InsertionMm = 100
		s := scoreExact(t, degenerate, true, 100)
		assertNear(t, "degenerate insertion", attrScore(t, s, assembly.AttrSeatpostInsertion).Score, 0, 0)
	})
	t.Run("Frameset readouts", func(t *testing.T) {
		lithic := base
		fox := base
		fox.Fork = fox34Fork(20)
		for _, c := range []struct {
			name                string
			setup               assembly.BikeSetup
			rcWb, rcWbScore     float64
			bbDrop, bbDropScore float64
			trail, trailScore   float64
		}{
			{"Lithic", lithic, 38.278721, 100, 67.819258, 100, 94.123289, 80.616445},
			{"Fox 34 20%", fox, 38.511960, 100, 62.443349, 87.216745, 109.963183, 80.184085},
			{"basis", basisSetup(), 38.369496, 100, 72.082893, 100, 90.197878, 60.989388},
		} {
			p := gripPlacement(t, c.setup)
			assertNear(t, c.name+" RC/WB", p.Frameset.RcWbRatioPct(), c.rcWb, tol)
			assertNear(t, c.name+" BB drop", p.Frameset.LeveledBbDropMm(), c.bbDrop, tol)
			assertNear(t, c.name+" trail", p.Frameset.TrailMm(), c.trail, tol)
			s := scoreExact(t, c.setup, false, 100)
			assertNear(t, c.name+" RC/WB score", attrScore(t, s, assembly.AttrRcWbRatio).Score, c.rcWbScore, tol)
			assertNear(t, c.name+" BB drop score", attrScore(t, s, assembly.AttrLeveledBbDrop).Score, c.bbDropScore, tol)
			assertNear(t, c.name+" trail score", attrScore(t, s, assembly.AttrTrail).Score, c.trailScore, tol)
		}
	})
}

func TestScoreBuildReferenceBuilds(t *testing.T) {
	const tol = 1e-5
	sid := fenrirFittingSetup()
	sid.Fork = sidFork(20)
	sid = withCockpit(sid, 70, 0, 10)
	lithic := withCockpit(fenrirFittingSetup(), 70, 0, 10)

	s := scoreExact(t, sid, true, 100)
	if s.Tier != assembly.FitsAll {
		t.Errorf("SID tier = %v", s.Tier)
	}
	assertNear(t, "SID total", s.Total, 100, tol)
	for _, a := range s.Attributes {
		assertNear(t, string(a.Attribute), a.Score, 100, tol)
	}
	s = scoreExact(t, sid, false, 100)
	if s.Tier != assembly.FitsGrip {
		t.Errorf("SID no-BRP tier = %v", s.Tier)
	}
	assertNear(t, "SID no-BRP total", s.Total, 100, tol)
	if len(s.Attributes) != 12 {
		t.Fatalf("attributes = %d", len(s.Attributes))
	}
	for _, name := range []assembly.AttributeName{assembly.AttrSaddleBrpY, assembly.AttrSaddleBrpX, assembly.AttrSaddleRailOffset, assembly.AttrSeatpostInsertion} {
		if attrScore(t, s, name).Active {
			t.Errorf("%s active without BRP", name)
		}
	}

	s = scoreExact(t, lithic, true, 100)
	if s.Tier != assembly.FitsAll {
		t.Errorf("Lithic tier = %v", s.Tier)
	}
	assertNear(t, "Lithic total", s.Total, 99.461568, tol)
	s = scoreExact(t, lithic, false, 100)
	assertNear(t, "Lithic no-BRP total", s.Total, 99.157237, tol)

	basis := withCockpit(basisSetup(), 120, -20, 0)
	grip := fitReportGrip
	s = mustScore(t, gripPlacement(t, basis), assembly.FitTarget{HandGrip: &grip}, 0)
	if s.Tier != assembly.FitsNeither {
		t.Errorf("basis tier = %v", s.Tier)
	}
	assertNear(t, "basis total", s.Total, 69.082428, tol)
	assertNear(t, "basis R", s.GripR, 1.207581, tol)
	assertNear(t, "basis grip", attrScore(t, s, assembly.AttrHandGrip).Score, 71.317744, tol)
	assertNear(t, "basis sag", attrScore(t, s, assembly.AttrForkSag).Score, 100, tol)
	assertNear(t, "basis trail", attrScore(t, s, assembly.AttrTrail).Score, 60.989388, tol)

	for _, c := range []struct {
		name           string
		brp            pose.Point[assembly.DynamicWorld]
		tier           assembly.FitTier
		total, mx, my  float64
		scoreY, scoreX float64
		railScore      float64
		checkRail      bool
	}{
		{"gate boundary", gateTradeOffBRP, assembly.FitsAll, 85.031263, -10, -1.097455, 80.084835, 50, 0, true},
		{"just outside", pose.Point[assembly.DynamicWorld]{X: -175.542919, Y: 710.580028}, assembly.FitsGrip, 88.017123, -10.6, 0, 100, 46.999999, 0, false},
	} {
		brp := c.brp
		fit, err := assembly.ApplyFitting(fenrirFittingSetup(), assembly.FitTarget{BRP: &brp})
		if err != nil {
			t.Fatal(err)
		}
		setup := withCockpit(fit.Placement.Setup, 70, 0, 10)
		p := gripPlacement(t, setup)
		g := p.Handlebar.HandGrip.Origin()
		s := mustScore(t, p, assembly.FitTarget{HandGrip: &g, BRP: &brp}, 100)
		if s.Tier != c.tier {
			t.Errorf("%s tier = %v, want %v", c.name, s.Tier, c.tier)
		}
		assertNear(t, c.name+" total", s.Total, c.total, tol)
		assertNear(t, c.name+" missX", s.BrpMissXMm, c.mx, tol)
		assertNear(t, c.name+" missY", s.BrpMissYMm, c.my, tol)
		assertNear(t, c.name+" Y", attrScore(t, s, assembly.AttrSaddleBrpY).Score, c.scoreY, tol)
		assertNear(t, c.name+" X", attrScore(t, s, assembly.AttrSaddleBrpX).Score, c.scoreX, tol)
		if c.checkRail {
			assertNear(t, c.name+" rail", attrScore(t, s, assembly.AttrSaddleRailOffset).Score, c.railScore, tol)
		}
	}

	trailOnly := assembly.ScoreWeights{Trail: 1}
	p := gripPlacement(t, lithic)
	g := p.Handlebar.HandGrip.Origin()
	s, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &g}, 100, trailOnly)
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "trail-only total", s.Total, 80.616445, tol)
}

func TestScoreBuildErrors(t *testing.T) {
	p := gripPlacement(t, fenrirFittingSetup())
	grip := p.Handlebar.HandGrip.Origin()
	brp := p.Saddle.BRP.Origin()
	w := testDefaultScoreWeights()
	nan, inf := math.NaN(), math.Inf(1)

	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{}, 100, w); err == nil {
		t.Error("nil HandGrip accepted")
	}
	badGrip := pose.Point[assembly.DynamicWorld]{X: nan, Y: 0}
	var inputErr *assembly.InputError
	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &badGrip}, 100, w); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetGrip {
		t.Errorf("NaN HandGrip err = %v", err)
	}
	badBrp := pose.Point[assembly.DynamicWorld]{X: inf, Y: 0}
	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &grip, BRP: &badBrp}, 100, w); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputFitTargetBRP {
		t.Errorf("+Inf BRP err = %v", err)
	}
	for _, v := range []float64{-1, 101, nan} {
		if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &grip}, v, w); err == nil {
			t.Errorf("stem score %v accepted", v)
		}
	}
	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &grip}, 100, assembly.ScoreWeights{}); err == nil {
		t.Error("zero weights accepted")
	}
	saddleOnly := assembly.ScoreWeights{SaddleBrpY: 1, SaddleBrpX: 1, SaddleRailOffset: 1, SeatpostInsertion: 1}
	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &grip}, 100, saddleOnly); err == nil {
		t.Error("saddle-only weights without BRP accepted")
	}
	if _, err := assembly.ScoreBuild(p, assembly.FitTarget{HandGrip: &grip, BRP: &brp}, 100, saddleOnly); err != nil {
		t.Errorf("saddle-only weights with BRP: %v", err)
	}
	impossible := p
	impossible.Setup.Frame.MaxPostInsertionMm = 50
	if _, err := assembly.ScoreBuild(impossible, assembly.FitTarget{HandGrip: &grip, BRP: &brp}, 100, w); err == nil {
		t.Error("impossible insertion bounds accepted")
	}
}

func TestFitTierRankAndCompareFitRank(t *testing.T) {
	tiers := []assembly.FitTier{assembly.FitsAll, assembly.FitsBRP, assembly.FitsGrip, assembly.FitsNeither}
	for i := 1; i < len(tiers); i++ {
		if tiers[i-1].Rank() >= tiers[i].Rank() {
			t.Errorf("%v does not rank ahead of %v", tiers[i-1], tiers[i])
		}
	}
	bs := func(tier assembly.FitTier, total, r, mx, my float64) assembly.BuildScore {
		return assembly.BuildScore{Tier: tier, Total: total, GripR: r, HasBrp: true, BrpMissXMm: mx, BrpMissYMm: my}
	}
	for _, tc := range []struct {
		name          string
		better, worse assembly.BuildScore
	}{
		{"all beats brp despite score", bs(assembly.FitsAll, 10, 1, 0, 0), bs(assembly.FitsBRP, 99, 1.1, 0, 0)},
		{"brp beats grip", bs(assembly.FitsBRP, 10, 1.5, 0, 0), bs(assembly.FitsGrip, 99, 0.5, 0, 0)},
		{"grip beats neither", bs(assembly.FitsGrip, 10, 0.5, 0, 0), bs(assembly.FitsNeither, 99, 1.5, 0, 0)},
		{"all: score", bs(assembly.FitsAll, 90, 0.9, 0, 0), bs(assembly.FitsAll, 80, 0.1, 0, 0)},
		{"all: R", bs(assembly.FitsAll, 90, 0.1, 1, 1), bs(assembly.FitsAll, 90, 0.2, 0, 0)},
		{"all: |Y|", bs(assembly.FitsAll, 90, 0.1, 5, -1), bs(assembly.FitsAll, 90, 0.1, 0, 1.5)},
		{"all: |X|", bs(assembly.FitsAll, 90, 0.1, -1, 1), bs(assembly.FitsAll, 90, 0.1, 2, -1)},
		{"grip: score", bs(assembly.FitsGrip, 90, 0.9, 0, 0), bs(assembly.FitsGrip, 80, 0.1, 0, 0)},
		{"grip: |Y|", bs(assembly.FitsGrip, 90, 0.9, 5, 1), bs(assembly.FitsGrip, 90, 0.1, 0, 2)},
		{"grip: |X|", bs(assembly.FitsGrip, 90, 0.9, 1, 2), bs(assembly.FitsGrip, 90, 0.1, 3, 2)},
		{"grip: R", bs(assembly.FitsGrip, 90, 0.1, 1, 2), bs(assembly.FitsGrip, 90, 0.9, 1, 2)},
		{"brp: R before score", bs(assembly.FitsBRP, 10, 1.1, 0, 0), bs(assembly.FitsBRP, 90, 1.5, 0, 0)},
		{"brp: score", bs(assembly.FitsBRP, 90, 1.1, 0, 0), bs(assembly.FitsBRP, 80, 1.1, 0, 0)},
		{"brp: |Y|", bs(assembly.FitsBRP, 90, 1.1, 5, 1), bs(assembly.FitsBRP, 90, 1.1, 0, 2)},
		{"brp: |X|", bs(assembly.FitsBRP, 90, 1.1, 1, 2), bs(assembly.FitsBRP, 90, 1.1, 3, 2)},
		{"neither: R before score", bs(assembly.FitsNeither, 10, 1.1, 0, 0), bs(assembly.FitsNeither, 90, 1.5, 0, 0)},
		{"neither: score", bs(assembly.FitsNeither, 90, 1.1, 0, 0), bs(assembly.FitsNeither, 80, 1.1, 0, 0)},
		{"neither: |Y|", bs(assembly.FitsNeither, 90, 1.1, 5, 1), bs(assembly.FitsNeither, 90, 1.1, 0, 2)},
		{"neither: |X|", bs(assembly.FitsNeither, 90, 1.1, 1, 2), bs(assembly.FitsNeither, 90, 1.1, 3, 2)},
	} {
		if got := assembly.CompareFitRank(tc.better, tc.worse); got >= 0 {
			t.Errorf("%s: Compare(better, worse) = %d", tc.name, got)
		}
		if got := assembly.CompareFitRank(tc.worse, tc.better); got <= 0 {
			t.Errorf("%s: Compare(worse, better) = %d (antisymmetry)", tc.name, got)
		}
		if got := assembly.CompareFitRank(tc.better, tc.better); got != 0 {
			t.Errorf("%s: Compare(x, x) = %d", tc.name, got)
		}
	}

	// HasBrp == false: both scores carry zero BRP misses, so the BRP tie-breaks never decide.
	a := assembly.BuildScore{Tier: assembly.FitsGrip, Total: 90, GripR: 0.5}
	b := assembly.BuildScore{Tier: assembly.FitsGrip, Total: 90, GripR: 0.7}
	if assembly.CompareFitRank(a, b) >= 0 {
		t.Error("no-BRP grip tie should fall to R")
	}

	// Transitivity: a sort of a shuffled mix is stable under re-sorting reversed input.
	mix := []assembly.BuildScore{
		bs(assembly.FitsNeither, 50, 1.2, 1, 1), bs(assembly.FitsAll, 80, 0.5, 1, 1), bs(assembly.FitsGrip, 70, 0.4, 3, 1),
		bs(assembly.FitsBRP, 60, 1.1, 0, 0), bs(assembly.FitsAll, 90, 0.9, 0, 0), bs(assembly.FitsNeither, 60, 1.9, 0, 0),
		bs(assembly.FitsGrip, 70, 0.4, 1, 1), bs(assembly.FitsBRP, 70, 1.4, 0, 0),
	}
	fwd := slices.Clone(mix)
	slices.SortFunc(fwd, assembly.CompareFitRank)
	rev := slices.Clone(mix)
	slices.Reverse(rev)
	slices.SortFunc(rev, assembly.CompareFitRank)
	if !slices.EqualFunc(fwd, rev, func(x, y assembly.BuildScore) bool { return assembly.CompareFitRank(x, y) == 0 }) {
		t.Error("sorted order depends on input order")
	}
}
