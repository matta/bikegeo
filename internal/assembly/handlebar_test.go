package assembly_test

import (
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
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

	steerer := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 400, 600), 180.0-70.0).Anchor(assembly.SteererLocal)
	stem, err := placeStem(steerer, assembly.StemSetup{
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
	bar, err := placeHandlebar(stem.BarClamp, knuckle)
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
	barNeg, err := placeHandlebar(stem.BarClamp, negRoll)
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
	barCut, err := placeHandlebar(stem.BarClamp, cutBarEnd)
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
	barNarrow, err := placeHandlebar(stem.BarClamp, narrow)
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
		steerer := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 430, 625), 180.0-hta).Anchor(assembly.SteererLocal)
		stem, err := placeStem(steerer, assembly.StemSetup{
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
		bar, err := placeHandlebar(stem.BarClamp, barSetup)
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

	bike, err := placeBike(bikeSetup)
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
	if _, err := placeBike(badStem); err == nil {
		t.Fatal("PlaceBike expected error for zero stem length")
	}

	badBar := bikeSetup
	badBar.Handlebar.GripInsetMm = 0
	if _, err := placeBike(badBar); err == nil {
		t.Fatal("PlaceBike expected error for zero grip inset")
	}
}

func TestPlaceHandlebarValidation(t *testing.T) {
	validPose := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 460, 700), -15)
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
		{"NaN origin X", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, math.NaN(), 700), 0), valid},
		{"Inf heading", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 460, 700), pose.Deg(math.Inf(1))), valid},
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
		{"translate overflow", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 0, math.MaxFloat64), 0), assembly.HandlebarSetup{
			StockWidthMm: 800,
			StatedRiseMm: math.MaxFloat64,
			RiseMode:     assembly.RiseModeStandardKnuckle,
			GripInsetMm:  70,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := placeHandlebar(tc.pose, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPlaceHandlebarRejectsLocalPose(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("PlaceHandlebar accepted a SteererLocal pose; expected panic")
		}
	}()
	setup := assembly.HandlebarSetup{
		StockWidthMm: 800,
		BacksweepDeg: 16,
		UpsweepDeg:   5,
		StatedRiseMm: 20,
		RiseMode:     assembly.RiseModeStandardKnuckle,
		BendStartMm:  90,
		GripInsetMm:  70,
		BarRollDeg:   0,
	}
	_, _ = placeHandlebar(pose.OriginPose(assembly.SteererLocal), setup)
}

// Isolated Handlebar and ClampedHandlebar tests below construct no bicycle
// frameset, verifying 3D-to-2D sagittal projection in HandlebarLocal and rigid
// stem-clamped roll across StaticWorld and DynamicWorld.

func persuader800Dims() assembly.HandlebarDimensions {
	return assembly.HandlebarDimensions{
		StockWidthMm:   800.0,
		CurrentWidthMm: 800.0,
		BacksweepDeg:   20.0,
		UpsweepDeg:     4.0,
		StatedRiseMm:   50.0,
		RiseMode:       assembly.RiseModeBarEndCenter,
		BendStartMm:    135.0,
		GripInsetMm:    70.0,
	}
}

func TestHandlebarIsolated(t *testing.T) {
	// Bar-end center mode cut narrower than stock width.
	cutDims := persuader800Dims()
	cutDims.CurrentWidthMm = 760.0
	cutDims.GripInsetMm = 65.0

	bar, err := assembly.NewHandlebar(cutDims)
	if err != nil {
		t.Fatalf("NewHandlebar cut: %v", err)
	}
	if bar.GripInBar().CoordinateSystem() != assembly.HandlebarLocal {
		t.Errorf("GripInBar system = %s, want %s", bar.GripInBar().CoordinateSystem(), assembly.HandlebarLocal)
	}
	yk := 50.0 - 265.0*math.Tan(4.0*math.Pi/180.0)
	wantDX := -180.0 * math.Tan(20.0*math.Pi/180.0)
	wantDY := yk + 180.0*math.Tan(4.0*math.Pi/180.0)
	assertNear(t, "HandLateralMm", bar.HandLateralMm(), 315.0, 1e-12)
	assertNear(t, "GripInBar.X", bar.GripInBar().X(), wantDX, 1e-9)
	assertNear(t, "GripInBar.Y", bar.GripInBar().Y(), wantDY, 1e-9)
	if bar.Dimensions() != cutDims {
		t.Errorf("Dimensions = %+v, want %+v", bar.Dimensions(), cutDims)
	}

	// Standard knuckle mode with zHand <= BendStartMm has zero outboard sweep.
	narrow, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm:   800,
		CurrentWidthMm: 280,
		BacksweepDeg:   16,
		UpsweepDeg:     4,
		StatedRiseMm:   30,
		RiseMode:       assembly.RiseModeStandardKnuckle,
		BendStartMm:    85,
		GripInsetMm:    65,
	})
	if err != nil {
		t.Fatalf("NewHandlebar narrow: %v", err)
	}
	assertNear(t, "narrow GripInBar.X", narrow.GripInBar().X(), 0, 1e-12)
	assertNear(t, "narrow GripInBar.Y", narrow.GripInBar().Y(), 30, 1e-12)

	// Invalid dimensions fail construction.
	for _, tc := range []struct {
		name string
		mut  func(*assembly.HandlebarDimensions)
	}{
		{"zero widths", func(d *assembly.HandlebarDimensions) { d.StockWidthMm = 0; d.CurrentWidthMm = 0 }},
		{"current > stock", func(d *assembly.HandlebarDimensions) { d.CurrentWidthMm = 820 }},
		{"backsweep 90", func(d *assembly.HandlebarDimensions) { d.BacksweepDeg = 90 }},
		{"upsweep -90", func(d *assembly.HandlebarDimensions) { d.UpsweepDeg = -90 }},
		{"zero grip inset", func(d *assembly.HandlebarDimensions) { d.GripInsetMm = 0 }},
		{"unknown rise mode", func(d *assembly.HandlebarDimensions) { d.RiseMode = "bogus" }},
		{"bend > stock half-width", func(d *assembly.HandlebarDimensions) { d.BendStartMm = 405 }},
	} {
		d := persuader800Dims()
		tc.mut(&d)
		if _, err := assembly.NewHandlebar(d); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zero assembly.Handlebar
	assertPanics(t, "zero Handlebar GripInBar", func() { _ = zero.GripInBar() })
	assertPanics(t, "zero Handlebar HandLateralMm", func() { _ = zero.HandLateralMm() })
}

func TestMountHandlebarAndClampedHandlebarStaticVsDynamic(t *testing.T) {
	bar, err := assembly.NewHandlebar(persuader800Dims())
	if err != nil {
		t.Fatalf("NewHandlebar: %v", err)
	}
	steererInFrame := pose.NewPose(pose.NewPoint(assembly.FrameLocal, 428, 642), 111.75).Anchor(assembly.SteererLocal)
	port, err := assembly.NewSteererPort(steererInFrame, 11, 30)
	if err != nil {
		t.Fatalf("NewSteererPort: %v", err)
	}
	stem, err := assembly.NewStem(40, 60, -6)
	if err != nil {
		t.Fatalf("NewStem: %v", err)
	}
	clampedStem, err := assembly.MountStem(port, 20, stem)
	if err != nil {
		t.Fatalf("MountStem: %v", err)
	}

	// Clamp bar to +7.5° world roll (heading -7.5°) in DynamicWorld (frame pitch -2.0°),
	// then unload the exact same ClampedStem + ClampedHandlebar to StaticWorld
	// (frame pitch -0.5°, +1.5° nose-up pitch change).
	frameToDynamic := pose.NewTransform(assembly.FrameLocal, bounds.DynamicWorld, pose.Origin(bounds.DynamicWorld), -2.0)
	frameToStatic := pose.NewTransform(assembly.FrameLocal, bounds.StaticWorld, pose.Origin(bounds.StaticWorld), -0.5)
	dynStem := clampedStem.Place(frameToDynamic)
	statStem := clampedStem.Place(frameToStatic)

	rollTurn := assembly.BarTurnForWorldRoll(dynStem.BarClampInWorld.HeadingInDst(), 7.5)
	clampedBar, err := assembly.MountHandlebar(clampedStem.BarClampInFrame(), bar, rollTurn)
	if err != nil {
		t.Fatalf("MountHandlebar: %v", err)
	}

	dynBar := clampedBar.Place(dynStem.BarClampInWorld)
	statBar := clampedBar.Place(statStem.BarClampInWorld)

	// Dynamic bar sits at exact -7.5° world heading (+7.5° CW roll).
	assertNear(t, "dynamic BarCenter heading", float64(dynBar.BarCenter.HeadingDeg()), -7.5, 1e-12)
	assertNear(t, "dynamic HandGrip heading", float64(dynBar.HandGrip.HeadingDeg()), -7.5, 1e-12)
	assertNear(t, "dynamic HandLateralMm", dynBar.HandLateralMm, 330.0, 1e-12)

	// Static bar rotates rigidly with the frame/stem by +1.5° to -6.0° (no gyroscope).
	assertNear(t, "static BarCenter heading", float64(statBar.BarCenter.HeadingDeg()), -6.0, 1e-12)
	assertNear(t, "static HandGrip heading", float64(statBar.HandGrip.HeadingDeg()), -6.0, 1e-12)
	assertNear(t, "BarClamp→HandGrip distance preserved",
		statBar.BarClamp.Origin().DistanceTo(statBar.HandGrip.Origin()),
		dynBar.BarClamp.Origin().DistanceTo(dynBar.HandGrip.Origin()), 1e-12)

	// Verify the entire front cockpit rotated rigidly about BB by +1.5° between Dynamic and Static.
	dynToStat := pose.NewTransform(bounds.DynamicWorld, bounds.StaticWorld, pose.Origin(bounds.StaticWorld), 1.5)
	wantStatGrip := dynToStat.ApplyPoint(dynBar.HandGrip.Origin())
	assertNear(t, "rigidly rotated static HandGrip.X", statBar.HandGrip.Origin().X(), wantStatGrip.X(), 1e-9)
	assertNear(t, "rigidly rotated static HandGrip.Y", statBar.HandGrip.Origin().Y(), wantStatGrip.Y(), 1e-9)

	// Non-finite rollTurnFromStem returns InputHandlebarRoll.
	if _, err := assembly.MountHandlebar(clampedStem.BarClampInFrame(), bar, pose.Deg(math.Inf(1))); err == nil {
		t.Error("Inf rollTurnFromStem: want error")
	}
	nanPort := assembly.BarClampPort{
		ClampInParent: pose.NewTransform(assembly.StemLocal, assembly.FrameLocal, pose.NewPoint(assembly.FrameLocal, math.NaN(), 0), 0),
	}
	if _, err := assembly.MountHandlebar(nanPort, bar, 0); err == nil {
		t.Error("non-finite BarClampPort: want error")
	}

	// Deterministic panics on coordinate-system mismatches and zero values.
	assertPanics(t, "BarTurnForWorldRoll FrameLocal", func() {
		_ = assembly.BarTurnForWorldRoll(pose.NewHeading(assembly.FrameLocal, 15), 0)
	})
	assertPanics(t, "BarTurnForWorldRoll UnpitchedChart", func() {
		_ = assembly.BarTurnForWorldRoll(pose.NewHeading(bounds.UnpitchedChart, 15), 0)
	})
	assertPanics(t, "BarTurnForWorldRoll zero Heading", func() {
		_ = assembly.BarTurnForWorldRoll(pose.Heading{}, 0)
	})
	assertPanics(t, "MountHandlebar zero Handlebar", func() {
		_, _ = assembly.MountHandlebar(clampedStem.BarClampInFrame(), assembly.Handlebar{}, 0)
	})
	assertPanics(t, "MountHandlebar zero Transform", func() {
		_, _ = assembly.MountHandlebar(assembly.BarClampPort{}, bar, 0)
	})
	assertPanics(t, "MountHandlebar wrong Src SteererLocal", func() {
		wrong := pose.NewTransform(assembly.SteererLocal, assembly.FrameLocal, pose.Origin(assembly.FrameLocal), 0)
		_, _ = assembly.MountHandlebar(assembly.BarClampPort{ClampInParent: wrong}, bar, 0)
	})
	assertPanics(t, "ClampedHandlebar.Place wrong Src", func() {
		wrong := pose.NewTransform(assembly.SeatpostCradle, bounds.DynamicWorld, pose.Origin(bounds.DynamicWorld), 0)
		_ = clampedBar.Place(wrong)
	})
	assertPanics(t, "zero ClampedHandlebar Place", func() {
		var zeroBar assembly.ClampedHandlebar
		_ = zeroBar.Place(dynStem.BarClampInWorld)
	})
}
