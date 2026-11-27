package assembly_test

import (
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

func fenrirDefaultStem() assembly.StemSetup {
	return assembly.StemSetup{
		MinLengthMm:   40,
		MaxLengthMm:   130,
		UpperStackMm:  11.0,
		SpacersMm:     20.0,
		MaxSpacersMm:  30,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
	}
}

func persuader800DefaultHandlebar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm:   800.0,
		CurrentWidthMm: 800.0,
		BacksweepDeg:   20.0,
		UpsweepDeg:     4.0,
		StatedRiseMm:   50.0,
		RiseMode:       assembly.RiseModeBarEndCenter,
		BendStartMm:    135.0,
		GripInsetMm:    70.0,
		BarRollDeg:     0.0,
	}
}

func TestPlaceHandlebarWorkedExamplesAndRiseModes(t *testing.T) {
	// 1. Standard knuckle mode with nonzero roll:
	//    width=800, inset=65 => zHand = 335, BendStart=85 => sweptSpan = 250
	//    backsweep=16°, upsweep=4°, statedRise=30 => dx0 = -250*tan(16°), dy0 = 30 + 250*tan(4°)
	//    At BarRollDeg = +10° (world heading -10°):
	//      dx = dx0*cos(10°) + dy0*sin(10°)
	//      dy = -dx0*sin(10°) + dy0*cos(10°)
	knuckle := assembly.HandlebarSetup{
		StockWidthMm: 800.0,
		BacksweepDeg: 16.0,
		UpsweepDeg:   4.0,
		StatedRiseMm: 30.0,
		RiseMode:     assembly.RiseModeStandardKnuckle,
		BendStartMm:  85.0,
		GripInsetMm:  65.0,
		BarRollDeg:   10.0,
	}

	steerer := pose.NewPose(pose.Pt(assembly.DynamicWorld, 400, 600), 180.0-70.0).Anchor(assembly.SteererLocal)
	stem, err := assembly.PlaceStem(steerer, assembly.StemSetup{
		UpperStackMm:  10,
		SpacersMm:     15,
		MaxSpacersMm:  30,
		ClampHeightMm: 40,
		LengthMm:      80,
		AngleDeg:      -6,
	})
	if err != nil {
		t.Fatalf("PlaceStem: %v", err)
	}
	bar, err := assembly.PlaceHandlebar(stem.BarClamp, knuckle)
	if err != nil {
		t.Fatalf("PlaceHandlebar knuckle(+10°): %v", err)
	}

	dx0 := -250.0 * math.Tan(16.0*math.Pi/180.0)
	dy0 := 30.0 + 250.0*math.Tan(4.0*math.Pi/180.0)
	c10, s10 := math.Cos(10.0*math.Pi/180.0), math.Sin(10.0*math.Pi/180.0)
	wantDX := dx0*c10 + dy0*s10
	wantDY := -dx0*s10 + dy0*c10

	assertNear(t, "HandLateralMm", bar.HandLateralMm, 335.0, 1e-12)
	assertNear(t, "HandGrip.X", bar.HandGrip.Origin().X(), stem.BarClamp.Origin().X()+wantDX, 1e-9)
	assertNear(t, "HandGrip.Y", bar.HandGrip.Origin().Y(), stem.BarClamp.Origin().Y()+wantDY, 1e-9)
	assertNear(t, "BarCenter.HeadingDeg", float64(bar.BarCenter.HeadingDeg()), -10.0, 1e-12)
	assertNear(t, "HandGrip.HeadingDeg", float64(bar.HandGrip.HeadingDeg()), -10.0, 1e-12)

	// Reversing roll sign (-10°) negates the roll contribution.
	negRoll := knuckle
	negRoll.BarRollDeg = -10.0
	barNeg, err := assembly.PlaceHandlebar(stem.BarClamp, negRoll)
	if err != nil {
		t.Fatalf("PlaceHandlebar knuckle(-10°): %v", err)
	}
	assertNear(t, "HandGrip(-10°).X", barNeg.HandGrip.Origin().X(), stem.BarClamp.Origin().X()+(dx0*c10-dy0*s10), 1e-9)
	assertNear(t, "HandGrip(-10°).Y", barNeg.HandGrip.Origin().Y(), stem.BarClamp.Origin().Y()+(dx0*s10+dy0*c10), 1e-9)

	// 2. Bar-end center mode, cut narrower than stock width:
	//    StockWidth=800, CurrentWidth=760, inset=65, BendStart=135, upsweep=4°, backsweep=20°, statedRise=50
	//    y_k uses stockTipSpan = 400 - 135 = 265 => y_k = 50 - 265*tan(4°)
	//    zHand uses CurrentWidth: 380 - 65 = 315 => sweptSpan = 315 - 135 = 180
	cutBarEnd := assembly.HandlebarSetup{
		StockWidthMm:   800.0,
		CurrentWidthMm: 760.0,
		BacksweepDeg:   20.0,
		UpsweepDeg:     4.0,
		StatedRiseMm:   50.0,
		RiseMode:       assembly.RiseModeBarEndCenter,
		BendStartMm:    135.0,
		GripInsetMm:    65.0,
		BarRollDeg:     0.0,
	}
	barCut, err := assembly.PlaceHandlebar(stem.BarClamp, cutBarEnd)
	if err != nil {
		t.Fatalf("PlaceHandlebar cut bar-end: %v", err)
	}
	yk := 50.0 - 265.0*math.Tan(4.0*math.Pi/180.0)
	wantCutDX := -180.0 * math.Tan(20.0*math.Pi/180.0)
	wantCutDY := yk + 180.0*math.Tan(4.0*math.Pi/180.0)
	assertNear(t, "cut bar-end HandLateralMm", barCut.HandLateralMm, 315.0, 1e-12)
	assertNear(t, "cut bar-end HandGrip.X", barCut.HandGrip.Origin().X(), stem.BarClamp.Origin().X()+wantCutDX, 1e-9)
	assertNear(t, "cut bar-end HandGrip.Y", barCut.HandGrip.Origin().Y(), stem.BarClamp.Origin().Y()+wantCutDY, 1e-9)

	// 3. Narrow bar where zHand <= BendStartMm clamps sweptSpan to 0 (no outboard sweep/upsweep).
	narrow := knuckle
	narrow.CurrentWidthMm = 280.0 // zHand = 140 - 65 = 75 < 85
	narrow.BarRollDeg = 0.0
	barNarrow, err := assembly.PlaceHandlebar(stem.BarClamp, narrow)
	if err != nil {
		t.Fatalf("PlaceHandlebar narrow: %v", err)
	}
	assertNear(t, "narrow HandLateralMm", barNarrow.HandLateralMm, 75.0, 1e-12)
	assertNear(t, "narrow HandGrip.X", barNarrow.HandGrip.Origin().X(), stem.BarClamp.Origin().X(), 1e-12)
	assertNear(t, "narrow HandGrip.Y", barNarrow.HandGrip.Origin().Y(), stem.BarClamp.Origin().Y()+30.0, 1e-12)
}

func TestPlaceHandlebarWorldRelativeRollAcrossAttitudesAndStems(t *testing.T) {
	// Bar roll is relative to leveled world horizontal (+X), so changing frame HTA
	// or stem angle moves BarClamp but leaves (HandGrip - BarClamp) invariant.
	barSetup := persuader800DefaultHandlebar()
	barSetup.BarRollDeg = -7.5

	placeFor := func(hta, stemAngle pose.Deg) (assembly.StemPlacement, assembly.HandlebarPlacement) {
		steerer := pose.NewPose(pose.Pt(assembly.DynamicWorld, 430, 625), 180.0-hta).Anchor(assembly.SteererLocal)
		stem, err := assembly.PlaceStem(steerer, assembly.StemSetup{
			UpperStackMm:  11,
			SpacersMm:     15,
			MaxSpacersMm:  30,
			ClampHeightMm: 40,
			LengthMm:      80,
			AngleDeg:      stemAngle,
		})
		if err != nil {
			t.Fatalf("PlaceStem(hta=%v, stem=%v): %v", hta, stemAngle, err)
		}
		bar, err := assembly.PlaceHandlebar(stem.BarClamp, barSetup)
		if err != nil {
			t.Fatalf("PlaceHandlebar(hta=%v, stem=%v): %v", hta, stemAngle, err)
		}
		return stem, bar
	}

	refStem, refBar := placeFor(70.0, -6.0)
	refDX := refBar.HandGrip.Origin().X() - refStem.BarClamp.Origin().X()
	refDY := refBar.HandGrip.Origin().Y() - refStem.BarClamp.Origin().Y()

	for _, cfg := range []struct {
		hta, stem pose.Deg
	}{
		{64.0, -17.0},
		{64.0, 17.0},
		{73.5, 0.0},
		{73.5, 25.0},
	} {
		stem, bar := placeFor(cfg.hta, cfg.stem)
		assertNear(t, "dX invariant", bar.HandGrip.Origin().X()-stem.BarClamp.Origin().X(), refDX, 1e-9)
		assertNear(t, "dY invariant", bar.HandGrip.Origin().Y()-stem.BarClamp.Origin().Y(), refDY, 1e-9)
		assertNear(t, "HandGrip heading", float64(bar.HandGrip.HeadingDeg()), 7.5, 1e-12)
	}
}

func TestOtsoFenrirFullBikeFrontCockpit(t *testing.T) {
	// Fox 34 120 mm at 20% sag (A2C = 506 mm, rake = 44 mm, 12 mm EC44 lower cup)
	// with full rear + front cockpits on Otso Fenrir L.
	frame := fenrirSizeLFrame()
	forkSetup := assembly.ForkSetup{
		Spec: assembly.ForkSpec{
			StaticA2cMm: 530.0,
			Travel:      assembly.Travel120,
			RakeMm:      44.0,
			Steerer:     assembly.SteererTapered,
		},
		SagPct: 20.0,
	}

	bikeSetup := assembly.BikeSetup{
		Frame:     frame,
		LowerCup:  assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0},
		Fork:      forkSetup,
		Wheels:    fenrirWheels(),
		Seatpost:  assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 136.5},
		Saddle:    sqlab610DefaultSaddle(),
		Stem:      fenrirDefaultStem(),
		Handlebar: persuader800DefaultHandlebar(),
	}

	bike, err := assembly.PlaceBike(bikeSetup)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}

	// Persuader 800 mm, 70 mm inset => HandLateralMm = 330 mm.
	assertNear(t, "HandLateralMm", bike.Handlebar.HandLateralMm, 330.0, 1e-12)

	// The stem stack and body keep their physical lengths from HeadTubeTop.
	st := bikeSetup.Stem
	assertNear(t, "stack distance", bike.Frameset.HeadTubeTop.Origin().DistanceTo(bike.Stem.SteererClamp.Origin()), st.UpperStackMm+st.SpacersMm+st.ClampHeightMm/2.0, 1e-9)
	assertNear(t, "stem length", bike.Stem.SteererClamp.Origin().DistanceTo(bike.Stem.BarClamp.Origin()), st.LengthMm, 1e-9)

	// The bar roll frame sits at BarClamp, rolled against world horizontal.
	assertNear(t, "BarCenter.X", bike.Handlebar.BarCenter.Origin().X(), bike.Stem.BarClamp.Origin().X(), 1e-12)
	assertNear(t, "BarCenter.Y", bike.Handlebar.BarCenter.Origin().Y(), bike.Stem.BarClamp.Origin().Y(), 1e-12)
	assertNear(t, "BarCenter.Heading", float64(bike.Handlebar.BarCenter.HeadingDeg()), -float64(bikeSetup.Handlebar.BarRollDeg), 1e-12)

	// Error propagation from PlaceStem and PlaceHandlebar in PlaceBike.
	badStem := bikeSetup
	badStem.Stem.LengthMm = 0
	if _, err := assembly.PlaceBike(badStem); err == nil {
		t.Fatal("PlaceBike expected error for zero stem length")
	}

	badBar := bikeSetup
	badBar.Handlebar.GripInsetMm = 0
	if _, err := assembly.PlaceBike(badBar); err == nil {
		t.Fatal("PlaceBike expected error for zero grip inset")
	}
}

func TestPlaceHandlebarValidation(t *testing.T) {
	validPose := pose.NewPose(pose.Pt(assembly.DynamicWorld, 460, 700), -15)
	valid := persuader800DefaultHandlebar()

	badWidth := valid
	badWidth.StockWidthMm = 0
	badWidth.CurrentWidthMm = 0

	badCurrent := valid
	badCurrent.CurrentWidthMm = -10

	badCutWider := valid
	badCutWider.CurrentWidthMm = 820

	badNegativeStock := valid
	badNegativeStock.RiseMode = assembly.RiseModeStandardKnuckle
	badNegativeStock.StockWidthMm = -10
	badNegativeStock.CurrentWidthMm = 760

	badEmptyMode := valid
	badEmptyMode.RiseMode = ""

	badMode := valid
	badMode.RiseMode = "unknown_mode"

	badBarEndStock := valid
	badBarEndStock.StockWidthMm = 0
	badBarEndStock.CurrentWidthMm = 760

	badBarEndBend := valid
	badBarEndBend.BendStartMm = 405

	badBend := valid
	badBend.BendStartMm = -1

	badRise := valid
	badRise.StatedRiseMm = math.NaN()

	badBacksweep := valid
	badBacksweep.BacksweepDeg = 90

	badUpsweep := valid
	badUpsweep.UpsweepDeg = -90

	badInsetZero := valid
	badInsetZero.GripInsetMm = 0

	badInsetWide := valid
	badInsetWide.GripInsetMm = 400

	badRoll := valid
	badRoll.BarRollDeg = pose.Deg(math.Inf(1))

	cases := []struct {
		name  string
		pose  pose.Pose
		setup assembly.HandlebarSetup
	}{
		{"NaN origin X", pose.NewPose(pose.Pt(assembly.DynamicWorld, math.NaN(), 700), 0), valid},
		{"Inf heading", pose.NewPose(pose.Pt(assembly.DynamicWorld, 460, 700), pose.Deg(math.Inf(1))), valid},
		{"zero effective width", validPose, badWidth},
		{"negative current width", validPose, badCurrent},
		{"current width exceeds stock", validPose, badCutWider},
		{"negative stock width in knuckle mode", validPose, badNegativeStock},
		{"empty rise mode", validPose, badEmptyMode},
		{"unsupported rise mode", validPose, badMode},
		{"bar-end mode without stock width", validPose, badBarEndStock},
		{"bar-end mode bend exceeds stock half-width", validPose, badBarEndBend},
		{"negative bend start", validPose, badBend},
		{"NaN stated rise", validPose, badRise},
		{"backsweep 90 deg", validPose, badBacksweep},
		{"upsweep -90 deg", validPose, badUpsweep},
		{"zero grip inset", validPose, badInsetZero},
		{"grip inset >= half width", validPose, badInsetWide},
		{"Inf bar roll", validPose, badRoll},
		{"translate overflow", pose.NewPose(pose.Pt(assembly.DynamicWorld, 0, math.MaxFloat64), 0), assembly.HandlebarSetup{
			StockWidthMm: 800,
			StatedRiseMm: math.MaxFloat64,
			RiseMode:     assembly.RiseModeStandardKnuckle,
			GripInsetMm:  70,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceHandlebar(tc.pose, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
