package assembly_test

import (
	"errors"
	"math"
	"testing"

	"bikegeo/internal/reboot/assembly"
)

func fenrirSizeLFrame() assembly.FrameSetup {
	return assembly.FrameSetup{
		ReachMm:            428.0,
		StackMm:            642.0,
		HeadTubeAngleDeg:   68.25,
		SeatTubeAngleDeg:   75.25,
		HeadTubeLengthMm:   140.0,
		BbDropMm:           72.0,
		ChainstayMm:        450.0,
		SeatTubeLengthMm:   490.0,
		MaxPostInsertionMm: 290.0,
		LowerBore:          assembly.Bore44Pressed,
		DefaultLowerCup:    assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0},
	}
}

func fenrirWheels() assembly.WheelSetup {
	return assembly.WheelSetup{RadiusMm: 372.0}
}

func fenrirCadCup() assembly.LowerCupSetup {
	return assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0.0}
}

func fenrirEc44Cup() assembly.LowerCupSetup {
	return assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0}
}

func TestPlaceFramesetCADBaselineForkAndInvariants(t *testing.T) {
	frame := fenrirSizeLFrame()
	cadCup := fenrirCadCup()
	wheels := fenrirWheels()
	cadFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{
			StaticA2cMm: 495.0,
			TravelMm:    0.0,
			RakeMm:      54.0,
			Steerer:     assembly.SteererCAD,
		},
		SagPct: 0.0,
	}

	placed, err := assembly.PlaceFrameset(frame, cadCup, cadFork, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	// BB is fixed at (0, 0); axles are leveled to equal Y.
	assertNear(t, "BB.X", placed.BB.Origin().X, 0, 1e-12)
	assertNear(t, "BB.Y", placed.BB.Origin().Y, 0, 1e-12)
	assertNear(t, "axle Y delta", placed.FrontAxle.Origin().Y-placed.RearAxle.Origin().Y, 0, 1e-9)

	// Wheel/ground relationships: contact points sit vertically below axles on a level ground line.
	assertNear(t, "contact Y delta", placed.FrontTireContact.Origin().Y-placed.RearTireContact.Origin().Y, 0, 1e-9)
	assertNear(t, "rear contact X", placed.RearTireContact.Origin().X, placed.RearAxle.Origin().X, 1e-9)
	assertNear(t, "front contact X", placed.FrontTireContact.Origin().X, placed.FrontAxle.Origin().X, 1e-9)
	assertNear(t, "rear wheel radius", placed.RearAxle.DistanceTo(placed.RearTireContact), wheels.RadiusMm, 1e-9)
	assertNear(t, "front wheel radius", placed.FrontAxle.DistanceTo(placed.FrontTireContact), wheels.RadiusMm, 1e-9)
	assertNear(t, "contact span equals axle wheelbase", placed.RearTireContact.DistanceTo(placed.FrontTireContact), placed.RearAxle.DistanceTo(placed.FrontAxle), 1e-9)
	assertNear(t, "BB height above ground", placed.BB.Origin().Y-placed.RearTireContact.Origin().Y, wheels.RadiusMm-placed.RearAxle.Origin().Y, 1e-9)

	// CAD baseline fork on Otso Fenrir L is consistent within chart rounding (~0° pitch, ~1158 mm wheelbase).
	if math.Abs(float64(placed.PitchDeg())) > 0.2 {
		t.Fatalf("CAD baseline fork pitch = %.4f°, want near 0°", placed.PitchDeg())
	}
	assertNear(t, "leveled HTA", float64(placed.LeveledHeadTubeAngleDeg()), float64(frame.HeadTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "leveled STA", float64(placed.LeveledSeatTubeAngleDeg()), float64(frame.SeatTubeAngleDeg-placed.PitchDeg()), 1e-12)
	const publishedWheelbaseMm = 1158.0
	assertNear(t, "CAD wheelbase", placed.WheelbaseMm(), publishedWheelbaseMm, 1.5)
	assertNear(t, "leveled BB drop", placed.LeveledBbDropMm(), placed.RearAxle.Origin().Y-placed.BB.Origin().Y, 1e-12)

	// Rigid physical lengths are preserved under leveling.
	assertNear(t, "chainstay", placed.BB.DistanceTo(placed.RearAxle), frame.ChainstayMm, 1e-9)
	assertNear(t, "head tube length", placed.HeadTubeTop.DistanceTo(placed.HeadTubeBottom), frame.HeadTubeLengthMm, 1e-9)
	assertNear(t, "lower cup", placed.HeadTubeBottom.DistanceTo(placed.LowerCupBottom), cadCup.StackMm, 1e-9)
	assertNear(t, "fork A2C", placed.LowerCupBottom.DistanceTo(placed.ForkSteererBase), cadFork.EffectiveA2cMm(), 1e-9)
	assertNear(t, "fork rake", placed.ForkSteererBase.DistanceTo(placed.FrontAxle), cadFork.Spec.RakeMm, 1e-9)
	assertNear(t, "seat tube length", placed.BB.DistanceTo(placed.SeatTubeTop), frame.SeatTubeLengthMm, 1e-9)

	// SeatAxis is anchored at BB, points up the leveled seat tube axis, and parents SeatTubeTop.
	assertNear(t, "SeatAxis.X", placed.SeatAxis.Origin().X, 0, 1e-12)
	assertNear(t, "SeatAxis.Y", placed.SeatAxis.Origin().Y, 0, 1e-12)
	assertNear(t, "SeatAxis.Heading == SeatTubeTop.Heading", float64(placed.SeatAxis.HeadingDeg()), float64(placed.SeatTubeTop.HeadingDeg()), 1e-12)
	assertNear(t, "SeatAxis to SeatTubeTop", placed.SeatAxis.DistanceTo(placed.SeatTubeTop), frame.SeatTubeLengthMm, 1e-9)
	assertNear(t, "SeatAxis.Heading == BB.Heading + 180 - STA", float64(placed.SeatAxis.HeadingDeg()), float64(placed.BB.HeadingDeg()+(180.0-frame.SeatTubeAngleDeg)), 1e-12)
}

func TestPlaceFramesetInstalledForkPitchAndLeveling(t *testing.T) {
	frame := fenrirSizeLFrame()
	ec44Cup := fenrirEc44Cup()
	wheels := fenrirWheels()

	// Installed 12 mm EC44 lower cup + 120 mm Fox 34 at 20% sag (resolved A2C = 506 mm).
	// Total front-end length increases by 12 + (506 - 495) = 23 mm, raising the front end
	// (positive frame pitch, slackening the leveled head tube angle) and lowering BB drop.
	fox34Spec := assembly.ForkSpec{
		StaticA2cMm: 530.0,
		TravelMm:    120.0,
		RakeMm:      44.0,
		Steerer:     assembly.SteererTapered,
	}
	fork20 := assembly.ForkSetup{Spec: fox34Spec, SagPct: 20.0}
	assertNear(t, "resolved A2C at 20% sag", fork20.EffectiveA2cMm(), 506.0, 1e-9)

	placed, err := assembly.PlaceFrameset(frame, ec44Cup, fork20, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	assertNear(t, "axle Y delta", placed.FrontAxle.Origin().Y-placed.RearAxle.Origin().Y, 0, 1e-9)
	assertNear(t, "contact Y delta", placed.FrontTireContact.Origin().Y-placed.RearTireContact.Origin().Y, 0, 1e-9)
	assertNear(t, "rear wheel radius", placed.RearAxle.DistanceTo(placed.RearTireContact), wheels.RadiusMm, 1e-9)
	assertNear(t, "front wheel radius", placed.FrontAxle.DistanceTo(placed.FrontTireContact), wheels.RadiusMm, 1e-9)
	if placed.PitchDeg() <= 0 {
		t.Fatalf("frame pitch = %.4f°, want > 0° for taller front end", placed.PitchDeg())
	}
	assertNear(t, "leveled HTA", float64(placed.LeveledHeadTubeAngleDeg()), float64(frame.HeadTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "leveled STA", float64(placed.LeveledSeatTubeAngleDeg()), float64(frame.SeatTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "fork A2C", placed.LowerCupBottom.DistanceTo(placed.ForkSteererBase), 506.0, 1e-9)
	assertNear(t, "lower cup", placed.HeadTubeBottom.DistanceTo(placed.LowerCupBottom), 12.0, 1e-9)

	// Increasing sag shortens resolved A2C, reduces frame pitch, and lowers BB above ground.
	fork30 := assembly.ForkSetup{Spec: fox34Spec, SagPct: 30.0}
	assertNear(t, "resolved A2C at 30% sag", fork30.EffectiveA2cMm(), 494.0, 1e-9)
	placedMoreSag, err := assembly.PlaceFrameset(frame, ec44Cup, fork30, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset 30%% sag: %v", err)
	}
	assertNear(t, "fork A2C at 30% sag", placedMoreSag.LowerCupBottom.DistanceTo(placedMoreSag.ForkSteererBase), 494.0, 1e-9)
	if placedMoreSag.PitchDeg() >= placed.PitchDeg() {
		t.Fatalf("30%% sag pitch = %.4f°, want < 20%% sag pitch %.4f°", placedMoreSag.PitchDeg(), placed.PitchDeg())
	}
	bbHeight20 := placed.BB.Origin().Y - placed.RearTireContact.Origin().Y
	bbHeight30 := placedMoreSag.BB.Origin().Y - placedMoreSag.RearTireContact.Origin().Y
	if bbHeight20 <= bbHeight30 {
		t.Fatalf("20%% sag BB height %.4f mm, want > 30%% sag BB height %.4f mm", bbHeight20, bbHeight30)
	}
}

func TestPlaceFramesetAndForkSpecRejectInvalidInputs(t *testing.T) {
	frame := fenrirSizeLFrame()
	ec44Cup := fenrirEc44Cup()
	fork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererTapered},
	}
	wheels := fenrirWheels()

	badFrame := frame
	badFrame.ChainstayMm = 70 // <= BbDropMm (72)
	if _, err := assembly.PlaceFrameset(badFrame, ec44Cup, fork, wheels); err == nil {
		t.Fatal("expected error when chainstay <= BB drop")
	}

	for _, badWheels := range []assembly.WheelSetup{
		{RadiusMm: 0},
		{RadiusMm: -10},
		{RadiusMm: math.NaN()},
	} {
		if _, err := assembly.PlaceFrameset(frame, ec44Cup, fork, badWheels); err == nil {
			t.Fatalf("expected error for WheelSetup %+v", badWheels)
		}
	}

	// Valid non-EC44 standards on the 44 mm bore (0 mm and positive ZS44) succeed.
	straightFork := fork
	straightFork.Spec.Steerer = assembly.SteererStraight118
	for _, validCup := range []assembly.LowerCupSetup{
		{Standard: assembly.LowerCupZS44, StackMm: 0},
		{Standard: assembly.LowerCupZS44, StackMm: 2},
	} {
		if _, err := assembly.PlaceFrameset(frame, validCup, straightFork, wheels); err != nil {
			t.Fatalf("expected valid LowerCupSetup %+v to pass, got %v", validCup, err)
		}
	}

	inputCases := []struct {
		name      string
		lowerCup  assembly.LowerCupSetup
		fork      assembly.ForkSetup
		wantField assembly.InputField
	}{
		{"zero static A2C", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 0, RakeMm: 54, Steerer: assembly.SteererTapered}}, assembly.InputForkA2c},
		{"negative travel", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: -10, RakeMm: 44, Steerer: assembly.SteererTapered}}, ""},
		{"NaN rake", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, RakeMm: math.NaN(), Steerer: assembly.SteererTapered}}, ""},
		{"negative sag", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: -5}, assembly.InputForkSag},
		{"sag over 100%", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 105}, assembly.InputForkSag},
		{"non-zero sag on rigid fork", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, TravelMm: 0, RakeMm: 54, Steerer: assembly.SteererTapered}, SagPct: 20}, assembly.InputForkSag},
		{"non-positive effective A2C", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 80, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 90}, assembly.InputForkA2c},
		{"CAD fork with travel at 0% sag", fenrirCadCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererCAD}, SagPct: 0}, ""},
		{"CAD fork with travel and sag", fenrirCadCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44, Steerer: assembly.SteererCAD}, SagPct: 20}, ""},
		{"CAD fork with zero travel and non-zero sag", fenrirCadCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, TravelMm: 0, RakeMm: 54, Steerer: assembly.SteererCAD}, SagPct: 20}, assembly.InputForkSag},
		{"missing lower cup standard", assembly.LowerCupSetup{}, fork, assembly.InputLowerCup},
		{"unknown lower cup standard", assembly.LowerCupSetup{Standard: "EC99", StackMm: 12}, fork, assembly.InputLowerCup},
		{"EC44 zero stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0}, fork, assembly.InputLowerCup},
		{"EC44 negative stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: -1}, fork, assembly.InputLowerCup},
		{"EC34 zero stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 0}, fork, assembly.InputLowerCup},
		{"BareHeadTube non-zero stack", assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 12}, fork, assembly.InputLowerCup},
		{"IS52 negative stack", assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: -1}, fork, assembly.InputLowerCup},
		{"ZS44 negative stack", assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: -1}, fork, assembly.InputLowerCup},
		{"NaN lower cup stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: math.NaN()}, fork, assembly.InputLowerCup},
	}
	for _, tc := range inputCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := assembly.PlaceFrameset(frame, tc.lowerCup, tc.fork, wheels)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			var inputErr *assembly.InputError
			if tc.wantField != "" {
				if !errors.As(err, &inputErr) || inputErr.Field != tc.wantField {
					t.Fatalf("error = %v, want InputError with field %q", err, tc.wantField)
				}
			} else if errors.As(err, &inputErr) {
				t.Fatalf("unexpected InputError tag %q on %v", inputErr.Field, err)
			}
		})
	}
}

func TestPlaceUnpitchedFrameset(t *testing.T) {
	frame := fenrirSizeLFrame()
	cup := fenrirCadCup()
	wheels := fenrirWheels()
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0, Steerer: assembly.SteererCAD},
	}

	unpitched, err := assembly.PlaceUnpitchedFrameset(frame, cup, basisFork, wheels)
	if err != nil {
		t.Fatalf("PlaceUnpitchedFrameset: %v", err)
	}
	leveled, err := assembly.PlaceFrameset(frame, cup, basisFork, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	if unpitched.PitchDeg() != 0 {
		t.Errorf("unpitched.PitchDeg() = %v, want exact 0", unpitched.PitchDeg())
	}
	assertNear(t, "unpitched.LevelingPitchDeg()", float64(unpitched.LevelingPitchDeg()), -0.010692172, 1e-9)
	assertNear(t, "leveled.PitchDeg() - unpitched.LevelingPitchDeg()", float64(leveled.PitchDeg()-unpitched.LevelingPitchDeg()), 0, 1e-12)
	assertNear(t, "leveled.LevelingPitchDeg()", float64(leveled.LevelingPitchDeg()), 0, 1e-12)
	assertNear(t, "axle rise", unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle), 0.22, 0.005)
	assertNear(t, "wheelbase mismatch", (unpitched.FrontAxle.Origin().X-unpitched.RearAxle.Origin().X)-1158.0, -0.34, 0.005)
	assertNear(t, "rear contact X", unpitched.RearTireContact.Origin().X, unpitched.RearAxle.Origin().X, 1e-9)
	assertNear(t, "rear contact Y", unpitched.RearTireContact.Origin().Y, unpitched.RearAxle.Origin().Y-wheels.RadiusMm, 1e-9)
	assertNear(t, "front contact X", unpitched.FrontTireContact.Origin().X, unpitched.FrontAxle.Origin().X, 1e-9)
	assertNear(t, "front contact Y", unpitched.FrontTireContact.Origin().Y, unpitched.FrontAxle.Origin().Y-wheels.RadiusMm, 1e-9)
	assertNear(t, "BB to FrontAxle distance", leveled.BB.DistanceTo(leveled.FrontAxle), unpitched.BB.DistanceTo(unpitched.FrontAxle), 1e-9)
	assertNear(t, "FrontAxle heading delta", float64(leveled.FrontAxle.HeadingDeg()-unpitched.FrontAxle.HeadingDeg()), -0.010692172, 1e-9)

	installedFork := assembly.ForkSetup{
		Spec:   assembly.ForkSpec{StaticA2cMm: 530.0, TravelMm: 120.0, RakeMm: 44.0, Steerer: assembly.SteererTapered},
		SagPct: 20.0,
	}
	installed, err := assembly.PlaceFrameset(frame, fenrirEc44Cup(), installedFork, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset installed: %v", err)
	}
	assertNear(t, "installed.PitchDeltaDeg(leveled)", float64(installed.PitchDeltaDeg(leveled)), 1.241315762, 1e-9)
	if leveled.PitchDeltaDeg(leveled) != 0 {
		t.Errorf("leveled.PitchDeltaDeg(leveled) = %v, want exact 0", leveled.PitchDeltaDeg(leveled))
	}

	for _, tc := range []struct {
		name string
		cup  assembly.LowerCupSetup
		fork assembly.ForkSetup
	}{
		{"basis fork A2C 0", cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 0, RakeMm: 54.0, Steerer: assembly.SteererCAD}}},
		{"rake NaN", cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: math.NaN(), Steerer: assembly.SteererCAD}}},
		{"missing lower cup standard", assembly.LowerCupSetup{}, basisFork},
		{"bare head tube with 12 mm stack", assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 12}, basisFork},
		{"EC44 with 0 mm stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0}, basisFork},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceUnpitchedFrameset(frame, tc.cup, tc.fork, wheels); err == nil {
				t.Errorf("PlaceUnpitchedFrameset(%s): expected error", tc.name)
			}
			if _, err := assembly.PlaceFrameset(frame, tc.cup, tc.fork, wheels); err == nil {
				t.Errorf("PlaceFrameset(%s): expected error", tc.name)
			}
		})
	}
}

func TestHeadsetPairingMatrix(t *testing.T) {
	type pairing struct {
		std      assembly.LowerCupStandard
		bore     assembly.LowerBore
		straight bool
		tapered  bool
	}
	matrix := []pairing{
		{assembly.LowerCupEC34, assembly.Bore34Pressed, true, false},
		{assembly.LowerCupEC44, assembly.Bore44Pressed, true, true},
		{assembly.LowerCupZS44, assembly.Bore44Pressed, true, false},
		{assembly.LowerCupZS56, assembly.Bore56Pressed, true, true},
		{assembly.LowerCupIS42, assembly.Bore42Integrated, true, false},
		{assembly.LowerCupIS52, assembly.Bore52Integrated, true, true},
	}
	bores := []assembly.LowerBore{
		assembly.Bore34Pressed, assembly.Bore44Pressed, assembly.Bore56Pressed,
		assembly.Bore42Integrated, assembly.Bore52Integrated,
	}
	wheels := fenrirWheels()
	place := func(bore assembly.LowerBore, cup assembly.LowerCupSetup, steerer assembly.Steerer) error {
		frame := fenrirSizeLFrame()
		frame.LowerBore = bore
		// The frame's own default is always a valid cup for its bore.
		for _, p := range matrix {
			if p.bore == bore {
				frame.DefaultLowerCup = assembly.LowerCupSetup{Standard: p.std, StackMm: 12}
				break
			}
		}
		fork := assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: steerer}}
		_, err := assembly.PlaceFrameset(frame, cup, fork, wheels)
		return err
	}
	for _, p := range matrix {
		cup := assembly.LowerCupSetup{Standard: p.std, StackMm: 12}
		for _, bore := range bores {
			for _, steerer := range []assembly.Steerer{assembly.SteererStraight118, assembly.SteererTapered, assembly.SteererCAD} {
				want := bore == p.bore
				switch steerer {
				case assembly.SteererStraight118:
					want = want && p.straight
				case assembly.SteererTapered:
					want = want && p.tapered
				case assembly.SteererCAD:
				}
				err := place(bore, cup, steerer)
				if want && err != nil {
					t.Errorf("%s on %s with %s steerer: %v", p.std, bore, steerer, err)
				}
				if !want {
					var inputErr *assembly.InputError
					if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
						t.Errorf("%s on %s with %s steerer: error = %v, want InputLowerCup", p.std, bore, steerer, err)
					}
				}
			}
		}
	}

	bare := fenrirCadCup()
	for _, bore := range bores {
		if err := place(bore, bare, assembly.SteererCAD); err != nil {
			t.Errorf("bare head tube on %s with CAD steerer: %v", bore, err)
		}
		for _, steerer := range []assembly.Steerer{assembly.SteererStraight118, assembly.SteererTapered} {
			if err := place(bore, bare, steerer); err == nil {
				t.Errorf("bare head tube on %s with physical %s steerer: want error", bore, steerer)
			}
		}
	}

	for _, tc := range []struct {
		name    string
		bore    assembly.LowerBore
		steerer assembly.Steerer
		cup     assembly.LowerCupSetup
	}{
		{"unknown bore", "40_pressed", assembly.SteererTapered, fenrirEc44Cup()},
		{"missing bore", "", assembly.SteererTapered, fenrirEc44Cup()},
		{"unknown bore with bare head tube", "40_pressed", assembly.SteererCAD, bare},
		{"unknown steerer", assembly.Bore44Pressed, "threaded", fenrirEc44Cup()},
		{"missing steerer", assembly.Bore44Pressed, "", fenrirEc44Cup()},
		{"unknown steerer with bare head tube", assembly.Bore44Pressed, "threaded", bare},
	} {
		var inputErr *assembly.InputError
		frame := fenrirSizeLFrame()
		frame.LowerBore = assembly.Bore44Pressed
		fork := assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: tc.steerer}}
		if tc.bore != assembly.Bore44Pressed {
			// An unknown frame bore also invalidates the frame's own default.
			frame.LowerBore = tc.bore
		}
		if _, err := assembly.PlaceFrameset(frame, tc.cup, fork, wheels); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
			t.Errorf("%s: error = %v, want InputLowerCup", tc.name, err)
		}
	}
}

func TestFrameDefaultLowerCupMustBePhysicalAndFitBore(t *testing.T) {
	for _, tc := range []struct {
		name string
		cup  assembly.LowerCupSetup
	}{
		{"bare head tube", fenrirCadCup()},
		{"wrong bore", assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}},
		{"invalid stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0}},
	} {
		frame := fenrirSizeLFrame()
		frame.DefaultLowerCup = tc.cup
		fork := assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererTapered}}
		var inputErr *assembly.InputError
		if _, err := assembly.PlaceFrameset(frame, fenrirEc44Cup(), fork, fenrirWheels()); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
			t.Errorf("%s: PlaceFrameset error = %v, want InputLowerCup", tc.name, err)
		}
	}
}

func TestMismatchedPhysicalCupFailsBothChartEntryPoints(t *testing.T) {
	chart := fenrirChart()
	chart.CadLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}
	var inputErr *assembly.InputError
	if err := assembly.CheckChartConsistency(chart); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
		t.Errorf("CheckChartConsistency error = %v, want InputLowerCup", err)
	}
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: chart.CadFork.A2cMm, RakeMm: chart.CadFork.RakeMm, Steerer: assembly.SteererCAD},
	}
	if _, err := assembly.PlaceUnpitchedFrameset(chart.Frame, chart.CadLowerCup, basisFork, fenrirWheels()); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
		t.Errorf("PlaceUnpitchedFrameset error = %v, want InputLowerCup", err)
	}
	// A matching physical cup on the CAD fork is accepted, as is a bare head tube on another bore.
	chart.CadLowerCup = fenrirEc44Cup()
	if err := assembly.CheckChartConsistency(chart); err != nil && errors.As(err, &inputErr) && inputErr.Field == assembly.InputLowerCup {
		t.Errorf("matching cup rejected as a headset error: %v", err)
	}
}

// fenrirPostTotalMm is the generic post's total length (Q75c: 400/365/100 mm, illustrative).
const fenrirPostTotalMm = 400.0

func genericPostSpec(setbackMm float64) assembly.SeatpostSpec {
	return assembly.SeatpostSpec{
		TotalLengthMm: fenrirPostTotalMm, MaxInsertionMm: 365, MinInsertionMm: 100,
		SetbackMm: setbackMm, RailClampLengthMm: 30,
	}
}

func TestFramesetHandlingReadouts(t *testing.T) {
	// Independent closed-form probe: trail = (R*cos(HTA_lev) - rake) / sin(HTA_lev) with R = 372.
	// Reach, stack, and ETT values come from an independent closed-form Python probe.
	frame := fenrirSizeLFrame()
	wheels := fenrirWheels()

	cases := []struct {
		name      string
		cup       assembly.LowerCupSetup
		fork      assembly.ForkSetup
		wantTrail float64
		wantRC    float64
		wantRatio float64
		wantWB    float64
		wantReach float64
		wantStack float64
		wantEtt   float64
	}{
		{
			name:      "Lithic (rigid)",
			cup:       fenrirEc44Cup(),
			fork:      assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererTapered}, SagPct: 0},
			wantTrail: 94.123289,
			wantRC:    444.860145,
			wantRatio: 38.278721,
			wantWB:    1162.160430,
			wantReach: 421.943304877,
			wantStack: 645.996785959,
			wantEtt:   598.532949715,
		},
		{
			name:      "Fox 34 at 20%",
			cup:       fenrirEc44Cup(),
			fork:      assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 530, TravelMm: 120, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 20},
			wantTrail: 109.963183,
			wantRC:    445.646528,
			wantRatio: 38.511960,
			wantWB:    1157.163974,
			wantReach: 414.113186529,
			wantStack: 651.043983723,
			wantEtt:   600.558793289,
		},
		{
			name:      "CAD baseline",
			cup:       fenrirCadCup(),
			fork:      assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererCAD}, SagPct: 0},
			wantTrail: 90.197878,
			wantRC:    444.189213,
			wantRatio: 38.369496,
			wantWB:    1157.662365,
			wantReach: 428.119798484,
			wantStack: 641.920118197,
			wantEtt:   596.995173525,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			placed, err := assembly.PlaceFrameset(frame, tc.cup, tc.fork, wheels)
			if err != nil {
				t.Fatalf("PlaceFrameset: %v", err)
			}
			assertNear(t, "TrailMm", placed.TrailMm(), tc.wantTrail, 1e-5)
			assertNear(t, "RearCenterMm", placed.RearCenterMm(), tc.wantRC, 1e-5)
			assertNear(t, "RcWbRatioPct", placed.RcWbRatioPct(), tc.wantRatio, 1e-5)
			assertNear(t, "WheelbaseMm", placed.WheelbaseMm(), tc.wantWB, 1e-5)
			assertNear(t, "RcWbRatioPct identity", placed.RcWbRatioPct(), 100.0*placed.RearCenterMm()/placed.WheelbaseMm(), 1e-12)
			assertNear(t, "RearCenterMm == -RearAxle.X", placed.RearCenterMm(), -placed.RearAxle.Origin().X, 1e-12)
			assertNear(t, "LeveledReachMm", placed.LeveledReachMm(), tc.wantReach, 1e-6)
			assertNear(t, "LeveledStackMm", placed.LeveledStackMm(), tc.wantStack, 1e-6)
			assertNear(t, "EttMm", placed.EttMm(), tc.wantEtt, 1e-6)
			assertNear(t, "LeveledReachMm == HeadTubeTop.X", placed.LeveledReachMm(), placed.HeadTubeTop.Origin().X, 1e-12)
			assertNear(t, "LeveledStackMm == HeadTubeTop.Y", placed.LeveledStackMm(), placed.HeadTubeTop.Origin().Y, 1e-12)
		})
	}
}
