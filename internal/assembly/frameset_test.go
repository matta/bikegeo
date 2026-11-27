package assembly_test

import (
	"errors"
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
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

func fenrirBasisCup() assembly.LowerCupSetup {
	return assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0.0}
}

func fenrirEc44Cup() assembly.LowerCupSetup {
	return assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0}
}

func TestPlaceFramesetBasisForkAndInvariants(t *testing.T) {
	frame := fenrirSizeLFrame()
	basisCup := fenrirBasisCup()
	wheels := fenrirWheels()
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{
			StaticA2cMm: 495.0,
			Travel:      assembly.TravelRigid,
			RakeMm:      54.0,
			Steerer:     assembly.SteererBasis,
		},
		SagPct: 0.0,
	}

	placed, err := placeFrameset(frame, basisCup, basisFork, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	// BB is fixed at (0, 0); axles are leveled to equal Y.
	assertNear(t, "BB.X", placed.BB.Origin().X(), 0, 1e-12)
	assertNear(t, "BB.Y", placed.BB.Origin().Y(), 0, 1e-12)
	assertNear(t, "axle Y delta", placed.FrontAxle.Origin().Y()-placed.RearAxle.Origin().Y(), 0, 1e-9)

	// Wheel/ground relationships: contact points sit vertically below axles on a level ground line.
	assertNear(t, "contact Y delta", placed.FrontTireContact.Origin().Y()-placed.RearTireContact.Origin().Y(), 0, 1e-9)
	assertNear(t, "rear contact X", placed.RearTireContact.Origin().X(), placed.RearAxle.Origin().X(), 1e-9)
	assertNear(t, "front contact X", placed.FrontTireContact.Origin().X(), placed.FrontAxle.Origin().X(), 1e-9)
	assertNear(t, "rear wheel radius", placed.RearAxle.Origin().DistanceTo(placed.RearTireContact.Origin()), wheels.RadiusMm, 1e-9)
	assertNear(t, "front wheel radius", placed.FrontAxle.Origin().DistanceTo(placed.FrontTireContact.Origin()), wheels.RadiusMm, 1e-9)
	assertNear(t, "contact span equals axle wheelbase", placed.RearTireContact.Origin().DistanceTo(placed.FrontTireContact.Origin()), placed.RearAxle.Origin().DistanceTo(placed.FrontAxle.Origin()), 1e-9)
	assertNear(t, "BB height above ground", placed.BB.Origin().Y()-placed.RearTireContact.Origin().Y(), wheels.RadiusMm-placed.RearAxle.Origin().Y(), 1e-9)

	// basis fork on Otso Fenrir L is consistent within chart rounding (~0° pitch, ~1158 mm wheelbase).
	if math.Abs(float64(placed.PitchDeg())) > 0.2 {
		t.Fatalf("basis fork pitch = %.4f°, want near 0°", placed.PitchDeg())
	}
	assertNear(t, "leveled HTA", float64(placed.LeveledHeadTubeAngleDeg()), float64(frame.HeadTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "leveled STA", float64(placed.LeveledSeatTubeAngleDeg()), float64(frame.SeatTubeAngleDeg-placed.PitchDeg()), 1e-12)
	const publishedWheelbaseMm = 1158.0
	assertNear(t, "basis wheelbase", placed.WheelbaseMm(), publishedWheelbaseMm, 1.5)
	assertNear(t, "leveled BB drop", placed.LeveledBbDropMm(), placed.RearAxle.Origin().Y()-placed.BB.Origin().Y(), 1e-12)

	// Rigid physical lengths are preserved under leveling.
	assertNear(t, "chainstay", placed.BB.Origin().DistanceTo(placed.RearAxle.Origin()), frame.ChainstayMm, 1e-9)
	assertNear(t, "head tube length", placed.HeadTubeTop.Origin().DistanceTo(placed.HeadTubeBottom.Origin()), frame.HeadTubeLengthMm, 1e-9)
	assertNear(t, "lower cup", placed.HeadTubeBottom.Origin().DistanceTo(placed.LowerCupBottom.Origin()), basisCup.StackMm, 1e-9)
	assertNear(t, "fork A2C", placed.LowerCupBottom.Origin().DistanceTo(placed.ForkSteererBase.Origin()), basisFork.EffectiveA2cMm(), 1e-9)
	assertNear(t, "fork rake", placed.ForkSteererBase.Origin().DistanceTo(placed.FrontAxle.Origin()), basisFork.Spec.RakeMm, 1e-9)
	assertNear(t, "seat tube length", placed.BB.Origin().DistanceTo(placed.SeatTubeTop.Origin()), frame.SeatTubeLengthMm, 1e-9)

	// SeatAxis is anchored at BB, points up the leveled seat tube axis, and parents SeatTubeTop.
	assertNear(t, "SeatAxis.X", placed.SeatAxis.Origin().X(), 0, 1e-12)
	assertNear(t, "SeatAxis.Y", placed.SeatAxis.Origin().Y(), 0, 1e-12)
	assertNear(t, "SeatAxis.Heading == SeatTubeTop.Heading", float64(placed.SeatAxis.HeadingDeg()), float64(placed.SeatTubeTop.HeadingDeg()), 1e-12)
	assertNear(t, "SeatAxis to SeatTubeTop", placed.SeatAxis.Origin().DistanceTo(placed.SeatTubeTop.Origin()), frame.SeatTubeLengthMm, 1e-9)
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
		Travel:      assembly.Travel120,
		RakeMm:      44.0,
		Steerer:     assembly.SteererTapered,
	}
	fork20 := assembly.ForkSetup{Spec: fox34Spec, SagPct: 20.0}
	assertNear(t, "resolved A2C at 20% sag", fork20.EffectiveA2cMm(), 506.0, 1e-9)

	placed, err := placeFrameset(frame, ec44Cup, fork20, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	assertNear(t, "axle Y delta", placed.FrontAxle.Origin().Y()-placed.RearAxle.Origin().Y(), 0, 1e-9)
	assertNear(t, "contact Y delta", placed.FrontTireContact.Origin().Y()-placed.RearTireContact.Origin().Y(), 0, 1e-9)
	assertNear(t, "rear wheel radius", placed.RearAxle.Origin().DistanceTo(placed.RearTireContact.Origin()), wheels.RadiusMm, 1e-9)
	assertNear(t, "front wheel radius", placed.FrontAxle.Origin().DistanceTo(placed.FrontTireContact.Origin()), wheels.RadiusMm, 1e-9)
	if placed.PitchDeg() <= 0 {
		t.Fatalf("frame pitch = %.4f°, want > 0° for taller front end", placed.PitchDeg())
	}
	assertNear(t, "leveled HTA", float64(placed.LeveledHeadTubeAngleDeg()), float64(frame.HeadTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "leveled STA", float64(placed.LeveledSeatTubeAngleDeg()), float64(frame.SeatTubeAngleDeg-placed.PitchDeg()), 1e-12)
	assertNear(t, "fork A2C", placed.LowerCupBottom.Origin().DistanceTo(placed.ForkSteererBase.Origin()), 506.0, 1e-9)
	assertNear(t, "lower cup", placed.HeadTubeBottom.Origin().DistanceTo(placed.LowerCupBottom.Origin()), 12.0, 1e-9)

	// Increasing sag shortens resolved A2C, reduces frame pitch, and lowers BB above ground.
	fork30 := assembly.ForkSetup{Spec: fox34Spec, SagPct: 30.0}
	assertNear(t, "resolved A2C at 30% sag", fork30.EffectiveA2cMm(), 494.0, 1e-9)
	placedMoreSag, err := placeFrameset(frame, ec44Cup, fork30, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset 30%% sag: %v", err)
	}
	assertNear(t, "fork A2C at 30% sag", placedMoreSag.LowerCupBottom.Origin().DistanceTo(placedMoreSag.ForkSteererBase.Origin()), 494.0, 1e-9)
	if placedMoreSag.PitchDeg() >= placed.PitchDeg() {
		t.Fatalf("30%% sag pitch = %.4f°, want < 20%% sag pitch %.4f°", placedMoreSag.PitchDeg(), placed.PitchDeg())
	}
	bbHeight20 := placed.BB.Origin().Y() - placed.RearTireContact.Origin().Y()
	bbHeight30 := placedMoreSag.BB.Origin().Y() - placedMoreSag.RearTireContact.Origin().Y()
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
	if _, err := placeFrameset(badFrame, ec44Cup, fork, wheels); err == nil {
		t.Fatal("expected error when chainstay <= BB drop")
	}

	for _, badWheels := range []assembly.WheelSetup{
		{RadiusMm: 0},
		{RadiusMm: -10},
		{RadiusMm: math.NaN()},
	} {
		if _, err := placeFrameset(frame, ec44Cup, fork, badWheels); err == nil {
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
		if _, err := placeFrameset(frame, validCup, straightFork, wheels); err != nil {
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
		{"negative travel", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: -10, RakeMm: 44, Steerer: assembly.SteererTapered}}, ""},
		{"unsupported 110mm travel", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: 110, RakeMm: 44, Steerer: assembly.SteererTapered}}, ""},
		{"unsupported 140mm travel", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 551, Travel: 140, RakeMm: 44, Steerer: assembly.SteererTapered}}, ""},
		{"unsupported 150mm travel", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 561, Travel: 150, RakeMm: 44, Steerer: assembly.SteererTapered}}, ""},
		{"NaN rake", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, RakeMm: math.NaN(), Steerer: assembly.SteererTapered}}, ""},
		{"negative sag", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: -5}, assembly.InputForkSag},
		{"sag over 100%", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 105}, assembly.InputForkSag},
		{"non-zero sag on rigid fork", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, Travel: assembly.TravelRigid, RakeMm: 54, Steerer: assembly.SteererTapered}, SagPct: 20}, assembly.InputForkSag},
		{"non-positive effective A2C", ec44Cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 80, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 90}, assembly.InputForkA2c},
		{"basis fork with travel at 0% sag", fenrirBasisCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererBasis}, SagPct: 0}, ""},
		{"basis fork with travel and sag", fenrirBasisCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, Travel: assembly.Travel100, RakeMm: 44, Steerer: assembly.SteererBasis}, SagPct: 20}, ""},
		{"basis fork with zero travel and non-zero sag", fenrirBasisCup(), assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, Travel: assembly.TravelRigid, RakeMm: 54, Steerer: assembly.SteererBasis}, SagPct: 20}, assembly.InputForkSag},
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
			_, err := placeFrameset(frame, tc.lowerCup, tc.fork, wheels)
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

func TestPlaceFramesetExactlyLevelPitch(t *testing.T) {
	frame := fenrirSizeLFrame()
	frame.HeadTubeAngleDeg = 90
	frame.StackMm = frame.HeadTubeLengthMm + 430 + frame.BbDropMm
	fork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 430, Steerer: assembly.SteererBasis},
	}

	leveled, err := placeFrameset(frame, fenrirBasisCup(), fork, fenrirWheels())
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	if pitch := leveled.PitchDeg(); pitch != 0 || math.Signbit(float64(pitch)) {
		t.Errorf("leveled.PitchDeg() = %v, want positive zero", pitch)
	}
}

func TestPlaceUnpitchedFrameset(t *testing.T) {
	frame := fenrirSizeLFrame()
	cup := fenrirBasisCup()
	wheels := fenrirWheels()
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0, Steerer: assembly.SteererBasis},
	}

	unpitched, err := placeUnpitchedFrameset(frame, cup, basisFork, wheels)
	if err != nil {
		t.Fatalf("PlaceUnpitchedFrameset: %v", err)
	}
	leveled, err := placeFrameset(frame, cup, basisFork, wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}

	if unpitched.PitchDeg() != 0 {
		t.Errorf("unpitched.PitchDeg() = %v, want exact 0", unpitched.PitchDeg())
	}
	assertNear(t, "unpitched.LevelingPitchDeg()", float64(unpitched.LevelingPitchDeg()), -0.010692172, 1e-9)
	assertNear(t, "leveled.PitchDeg() - unpitched.LevelingPitchDeg()", float64(leveled.PitchDeg()-unpitched.LevelingPitchDeg()), 0, 1e-12)
	assertNear(t, "leveled.LevelingPitchDeg()", float64(leveled.LevelingPitchDeg()), 0, 1e-12)
	assertNear(t, "axle rise", unpitched.RearAxle.Origin().VerticalOffsetTo(unpitched.FrontAxle.Origin()), 0.22, 0.005)
	assertNear(t, "wheelbase mismatch", (unpitched.FrontAxle.Origin().X()-unpitched.RearAxle.Origin().X())-1158.0, -0.34, 0.005)
	assertNear(t, "rear contact X", unpitched.RearTireContact.Origin().X(), unpitched.RearAxle.Origin().X(), 1e-9)
	assertNear(t, "rear contact Y", unpitched.RearTireContact.Origin().Y(), unpitched.RearAxle.Origin().Y()-wheels.RadiusMm, 1e-9)
	assertNear(t, "front contact X", unpitched.FrontTireContact.Origin().X(), unpitched.FrontAxle.Origin().X(), 1e-9)
	assertNear(t, "front contact Y", unpitched.FrontTireContact.Origin().Y(), unpitched.FrontAxle.Origin().Y()-wheels.RadiusMm, 1e-9)
	assertNear(t, "BB to FrontAxle distance", leveled.BB.Origin().DistanceTo(leveled.FrontAxle.Origin()), unpitched.BB.Origin().DistanceTo(unpitched.FrontAxle.Origin()), 1e-9)
	assertNear(t, "FrontAxle heading delta", float64(leveled.FrontAxle.HeadingDeg()-unpitched.FrontAxle.HeadingDeg()), -0.010692172, 1e-9)

	installedFork := assembly.ForkSetup{
		Spec:   assembly.ForkSpec{StaticA2cMm: 530.0, Travel: assembly.Travel120, RakeMm: 44.0, Steerer: assembly.SteererTapered},
		SagPct: 20.0,
	}
	installed, err := placeFrameset(frame, fenrirEc44Cup(), installedFork, wheels)
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
		{"basis fork A2C 0", cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 0, RakeMm: 54.0, Steerer: assembly.SteererBasis}}},
		{"rake NaN", cup, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: math.NaN(), Steerer: assembly.SteererBasis}}},
		{"missing lower cup standard", assembly.LowerCupSetup{}, basisFork},
		{"bare head tube with 12 mm stack", assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 12}, basisFork},
		{"EC44 with 0 mm stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0}, basisFork},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := placeUnpitchedFrameset(frame, tc.cup, tc.fork, wheels); err == nil {
				t.Errorf("PlaceUnpitchedFrameset(%s): expected error", tc.name)
			}
			if _, err := placeFrameset(frame, tc.cup, tc.fork, wheels); err == nil {
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
		_, err := placeFrameset(frame, cup, fork, wheels)
		return err
	}
	for _, p := range matrix {
		cup := assembly.LowerCupSetup{Standard: p.std, StackMm: 12}
		for _, bore := range bores {
			for _, steerer := range []assembly.Steerer{assembly.SteererStraight118, assembly.SteererTapered, assembly.SteererBasis} {
				want := bore == p.bore
				switch steerer {
				case assembly.SteererStraight118:
					want = want && p.straight
				case assembly.SteererTapered:
					want = want && p.tapered
				case assembly.SteererBasis:
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

	bare := fenrirBasisCup()
	for _, bore := range bores {
		if err := place(bore, bare, assembly.SteererBasis); err != nil {
			t.Errorf("bare head tube on %s with basis steerer: %v", bore, err)
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
		{"unknown bore with bare head tube", "40_pressed", assembly.SteererBasis, bare},
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
		if _, err := placeFrameset(frame, tc.cup, fork, wheels); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
			t.Errorf("%s: error = %v, want InputLowerCup", tc.name, err)
		}
	}
}

func TestFrameDefaultLowerCupMustBePhysicalAndFitBore(t *testing.T) {
	for _, tc := range []struct {
		name string
		cup  assembly.LowerCupSetup
	}{
		{"bare head tube", fenrirBasisCup()},
		{"wrong bore", assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}},
		{"invalid stack", assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0}},
	} {
		frame := fenrirSizeLFrame()
		frame.DefaultLowerCup = tc.cup
		fork := assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererTapered}}
		var inputErr *assembly.InputError
		if _, err := placeFrameset(frame, fenrirEc44Cup(), fork, fenrirWheels()); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
			t.Errorf("%s: PlaceFrameset error = %v, want InputLowerCup", tc.name, err)
		}
	}
}

func TestMismatchedBasisCupFailsUnpitchedPlacement(t *testing.T) {
	frame := fenrirSizeLFrame()
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0, Steerer: assembly.SteererBasis},
	}
	badCup := assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0}
	var inputErr *assembly.InputError
	if _, err := placeUnpitchedFrameset(frame, badCup, basisFork, fenrirWheels()); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputLowerCup {
		t.Errorf("PlaceUnpitchedFrameset error = %v, want InputLowerCup", err)
	}
	// A matching physical cup on the basis fork is accepted.
	if _, err := placeUnpitchedFrameset(frame, fenrirEc44Cup(), basisFork, fenrirWheels()); err != nil {
		t.Errorf("matching cup rejected: %v", err)
	}
}

// fenrirPostTotalMm is the generic post's total length (400/365/100 mm, illustrative).
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
			fork:      assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 530, Travel: assembly.Travel120, RakeMm: 44, Steerer: assembly.SteererTapered}, SagPct: 20},
			wantTrail: 109.963183,
			wantRC:    445.646528,
			wantRatio: 38.511960,
			wantWB:    1157.163974,
			wantReach: 414.113186529,
			wantStack: 651.043983723,
			wantEtt:   600.558793289,
		},
		{
			name:      "basis",
			cup:       fenrirBasisCup(),
			fork:      assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererBasis}, SagPct: 0},
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
			placed, err := placeFrameset(frame, tc.cup, tc.fork, wheels)
			if err != nil {
				t.Fatalf("PlaceFrameset: %v", err)
			}
			assertNear(t, "TrailMm", placed.TrailMm(), tc.wantTrail, 1e-5)
			assertNear(t, "RearCenterMm", placed.RearCenterMm(), tc.wantRC, 1e-5)
			assertNear(t, "RcWbRatioPct", placed.RcWbRatioPct(), tc.wantRatio, 1e-5)
			assertNear(t, "WheelbaseMm", placed.WheelbaseMm(), tc.wantWB, 1e-5)
			assertNear(t, "RcWbRatioPct identity", placed.RcWbRatioPct(), 100.0*placed.RearCenterMm()/placed.WheelbaseMm(), 1e-12)
			assertNear(t, "RearCenterMm == -RearAxle.X", placed.RearCenterMm(), -placed.RearAxle.Origin().X(), 1e-12)
			assertNear(t, "LeveledReachMm", placed.LeveledReachMm(), tc.wantReach, 1e-6)
			assertNear(t, "LeveledStackMm", placed.LeveledStackMm(), tc.wantStack, 1e-6)
			assertNear(t, "EttMm", placed.EttMm(), tc.wantEtt, 1e-6)
			assertNear(t, "LeveledReachMm == HeadTubeTop.X", placed.LeveledReachMm(), placed.HeadTubeTop.Origin().X(), 1e-12)
			assertNear(t, "LeveledStackMm == HeadTubeTop.Y", placed.LeveledStackMm(), placed.HeadTubeTop.Origin().Y(), 1e-12)
		})
	}
}

// Isolated Frame, Fork, and single-pass Frameset tests below.

func fenrirSizeLDims() assembly.FrameDimensions {
	return assembly.FrameDimensions{
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

func TestFrameIsolated(t *testing.T) {
	dims := fenrirSizeLDims()
	frame, err := assembly.NewFrame(dims)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	if frame.LowerBore() != assembly.Bore44Pressed || frame.DefaultLowerCup() != dims.DefaultLowerCup || frame.Dimensions() != dims {
		t.Errorf("unexpected frame metadata")
	}

	rear := frame.RearAxle()
	if rear.CoordinateSystem() != assembly.FrameLocal {
		t.Errorf("RearAxle system = %s, want %s", rear.CoordinateSystem(), assembly.FrameLocal)
	}
	assertNear(t, "RearAxle.X", rear.X(), -math.Sqrt(450*450-72*72), 1e-9)
	assertNear(t, "RearAxle.Y", rear.Y(), 72.0, 1e-12)

	ht := frame.HeadTube()
	if ht.SteererInFrame().Src() != assembly.SteererLocal || ht.SteererInFrame().Dst() != assembly.FrameLocal {
		t.Errorf("SteererInFrame = %s -> %s", ht.SteererInFrame().Src(), ht.SteererInFrame().Dst())
	}
	if ht.LowerHeadsetInFrame().Src() != assembly.LowerHeadsetLocal || ht.LowerHeadsetInFrame().Dst() != assembly.FrameLocal {
		t.Errorf("LowerHeadsetInFrame = %s -> %s", ht.LowerHeadsetInFrame().Src(), ht.LowerHeadsetInFrame().Dst())
	}
	assertNear(t, "HeadTubeTop.X", ht.SteererInFrame().OriginInDst().X(), 428.0, 1e-12)
	assertNear(t, "HeadTubeTop.Y", ht.SteererInFrame().OriginInDst().Y(), 642.0, 1e-12)
	assertNear(t, "head tube length", ht.SteererInFrame().OriginInDst().DistanceTo(ht.LowerHeadsetInFrame().OriginInDst()), 140.0, 1e-9)

	st := frame.SeatTube()
	if st.CollarInFrame.Src() != assembly.SeatTubeCollar || st.CollarInFrame.Dst() != assembly.FrameLocal {
		t.Errorf("SeatTube CollarInFrame = %s -> %s", st.CollarInFrame.Src(), st.CollarInFrame.Dst())
	}
	assertNear(t, "SeatTubeTop distance from BB", pose.Origin(assembly.FrameLocal).DistanceTo(st.CollarInFrame.OriginInDst()), 490.0, 1e-9)
	if st.AllowedInsertion.Min() != 0 || st.AllowedInsertion.Max() != 290 || st.AllowedInsertion.MaxSource() != "frame max insertion" {
		t.Errorf("declared AllowedInsertion = %+v", st.AllowedInsertion)
	}

	// Undeclared MaxPostInsertionMm falls back to SeatTubeLengthMm - 20 mm.
	uncapped := dims
	uncapped.MaxPostInsertionMm = 0
	uncapped.MinFrameInsertionMm = 125
	fUncapped, err := assembly.NewFrame(uncapped)
	if err != nil {
		t.Fatalf("NewFrame uncapped: %v", err)
	}
	if fUncapped.SeatTube().AllowedInsertion.Min() != 125 || fUncapped.SeatTube().AllowedInsertion.Max() != 470 ||
		fUncapped.SeatTube().AllowedInsertion.MaxSource() != "frame seat tube clearance" {
		t.Errorf("fallback AllowedInsertion = %+v", fUncapped.SeatTube().AllowedInsertion)
	}

	// Invalid frame dimensions fail construction.
	for _, tc := range []struct {
		name string
		mut  func(*assembly.FrameDimensions)
	}{
		{"zero reach", func(d *assembly.FrameDimensions) { d.ReachMm = 0 }},
		{"zero stack", func(d *assembly.FrameDimensions) { d.StackMm = 0 }},
		{"HTA 180", func(d *assembly.FrameDimensions) { d.HeadTubeAngleDeg = 180 }},
		{"STA 0", func(d *assembly.FrameDimensions) { d.SeatTubeAngleDeg = 0 }},
		{"chainstay <= |BbDrop|", func(d *assembly.FrameDimensions) { d.ChainstayMm = 72 }},
		{"bare default cup", func(d *assembly.FrameDimensions) { d.DefaultLowerCup = fenrirBasisCup() }},
		{"min insertion > max insertion", func(d *assembly.FrameDimensions) { d.MinFrameInsertionMm = 300 }},
		{"too short uncapped seat tube", func(d *assembly.FrameDimensions) { d.MaxPostInsertionMm = 0; d.SeatTubeLengthMm = 15 }},
	} {
		d := dims
		tc.mut(&d)
		if _, err := assembly.NewFrame(d); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zeroFrame assembly.Frame
	assertPanics(t, "zero Frame RearAxle", func() { _ = zeroFrame.RearAxle() })
	var zeroHT assembly.HeadTubePort
	assertPanics(t, "zero HeadTubePort SteererInFrame", func() { _ = zeroHT.SteererInFrame() })
}

func TestConcreteForksIsolated(t *testing.T) {
	// 1. RigidFork
	rigid, err := assembly.NewRigidFork(495, 54, assembly.PhysicalSteererTapered)
	if err != nil {
		t.Fatalf("NewRigidFork: %v", err)
	}
	if rigid.StaticA2cMm() != 495 || rigid.RakeMm() != 54 || rigid.Steerer() != assembly.PhysicalSteererTapered {
		t.Errorf("unexpected RigidFork accessors")
	}
	for _, state := range []assembly.LoadState{assembly.Static, assembly.Dynamic} {
		p := rigid.AxleInFork(state)
		if p.CoordinateSystem() != assembly.ForkLocal {
			t.Errorf("RigidFork %s system = %s", state, p.CoordinateSystem())
		}
		assertNear(t, "rigid X", p.X(), 495, 1e-12)
		assertNear(t, "rigid Y", p.Y(), 54, 1e-12)
	}
	if err := rigid.CheckHeadset(assembly.Bore44Pressed, fenrirEc44Cup()); err != nil {
		t.Errorf("RigidFork EC44 check: %v", err)
	}
	if err := rigid.CheckHeadset(assembly.Bore44Pressed, fenrirBasisCup()); err == nil {
		t.Error("RigidFork accepted LowerCupBareHeadTube")
	}
	if err := rigid.CheckHeadset(assembly.Bore44Pressed, assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: 4}); err == nil {
		t.Error("tapered RigidFork accepted ZS44")
	}

	// 2. SuspensionFork
	susp, err := assembly.NewSuspensionFork(530, 44, assembly.Travel120, assembly.PhysicalSteererTapered, 20)
	if err != nil {
		t.Fatalf("NewSuspensionFork: %v", err)
	}
	assertNear(t, "susp StaticA2cMm", susp.StaticA2cMm(), 530, 1e-12)
	assertNear(t, "susp DynamicA2cMm", susp.DynamicA2cMm(), 506, 1e-12)
	assertNear(t, "susp static X", susp.AxleInFork(assembly.Static).X(), 530, 1e-12)
	assertNear(t, "susp dynamic X", susp.AxleInFork(assembly.Dynamic).X(), 506, 1e-12)
	assertNear(t, "susp rake Y", susp.AxleInFork(assembly.Dynamic).Y(), 44, 1e-12)
	if susp.Travel() != assembly.Travel120 || susp.SagPct() != 20 || susp.Steerer() != assembly.PhysicalSteererTapered {
		t.Errorf("unexpected SuspensionFork accessors")
	}
	if _, err := assembly.NewSuspensionFork(530, 44, assembly.TravelRigid, assembly.PhysicalSteererTapered, 0); err == nil {
		t.Error("NewSuspensionFork accepted TravelRigid")
	}
	var inputErr *assembly.InputError
	if _, err := assembly.NewSuspensionFork(530, 44, assembly.Travel120, assembly.PhysicalSteererTapered, -1); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputForkSag {
		t.Errorf("negative sag = %v, want InputForkSag", err)
	}

	// 3. BasisFork
	basis, err := assembly.NewBasisFork(495, 54)
	if err != nil {
		t.Fatalf("NewBasisFork: %v", err)
	}
	if basis.BasisA2cMm() != 495 || basis.RakeMm() != 54 {
		t.Errorf("unexpected BasisFork accessors")
	}
	if err := basis.CheckHeadset(assembly.Bore44Pressed, fenrirBasisCup()); err != nil {
		t.Errorf("BasisFork bare cup check: %v", err)
	}
	if err := basis.CheckHeadset(assembly.Bore44Pressed, fenrirEc44Cup()); err != nil {
		t.Errorf("BasisFork matching EC44 check: %v", err)
	}
	if err := basis.CheckHeadset(assembly.Bore44Pressed, assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 1}); err == nil {
		t.Error("BasisFork accepted wrong-bore IS52 cup")
	}

	// Zero-value and invalid LoadState panics.
	var zeroRigid assembly.RigidFork
	var zeroSusp assembly.SuspensionFork
	var zeroBasis assembly.BasisFork
	assertPanics(t, "zero RigidFork", func() { _ = zeroRigid.AxleInFork(assembly.Static) })
	assertPanics(t, "zero SuspensionFork", func() { _ = zeroSusp.AxleInFork(assembly.Static) })
	assertPanics(t, "zero BasisFork", func() { _ = zeroBasis.AxleInFork(assembly.Static) })
	assertPanics(t, "SuspensionFork invalid LoadState", func() { _ = susp.AxleInFork(assembly.LoadState("bad")) })
}

type badSystemFork struct {
	axle pose.Point
}

func (f badSystemFork) CheckHeadset(assembly.LowerBore, assembly.LowerCupSetup) error { return nil }
func (f badSystemFork) AxleInFork(assembly.LoadState) pose.Point                      { return f.axle }

func TestMateFramesetSinglePassStaticVsDynamicAndSnapshot(t *testing.T) {
	frame, err := assembly.NewFrame(fenrirSizeLDims())
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	fork, err := assembly.NewSuspensionFork(530, 44, assembly.Travel120, assembly.PhysicalSteererTapered, 20)
	if err != nil {
		t.Fatalf("NewSuspensionFork: %v", err)
	}

	// Pass &fork so we can verify MateFrameset snapshots fork axles by value.
	fs, err := assembly.MateFrameset(frame, fenrirEc44Cup(), assembly.DefaultUpperStackMm, &fork, fenrirWheels())
	if err != nil {
		t.Fatalf("MateFrameset: %v", err)
	}

	// Mutate caller's fork variable after mating.
	shorter, err := assembly.NewSuspensionFork(480, 51, assembly.Travel100, assembly.PhysicalSteererTapered, 15)
	if err != nil {
		t.Fatalf("NewSuspensionFork shorter: %v", err)
	}
	fork = shorter

	dynPlaced := fs.Place(assembly.Dynamic)
	statPlaced := fs.Place(assembly.Static)
	unpitched := fs.Unpitched()

	if dynPlaced.BB.CoordinateSystem() != assembly.DynamicWorld {
		t.Errorf("Dynamic BB system = %s, want %s", dynPlaced.BB.CoordinateSystem(), assembly.DynamicWorld)
	}
	if statPlaced.BB.CoordinateSystem() != assembly.StaticWorld {
		t.Errorf("Static BB system = %s, want %s", statPlaced.BB.CoordinateSystem(), assembly.StaticWorld)
	}
	if unpitched.BB.CoordinateSystem() != assembly.UnpitchedChart {
		t.Errorf("Unpitched BB system = %s, want %s", unpitched.BB.CoordinateSystem(), assembly.UnpitchedChart)
	}
	if dynPlaced.FrameToWorld.Src() != assembly.FrameLocal || dynPlaced.FrameToWorld.Dst() != assembly.DynamicWorld {
		t.Errorf("Dynamic FrameToWorld = %s -> %s", dynPlaced.FrameToWorld.Src(), dynPlaced.FrameToWorld.Dst())
	}
	if statPlaced.FrameToWorld.Src() != assembly.FrameLocal || statPlaced.FrameToWorld.Dst() != assembly.StaticWorld {
		t.Errorf("Static FrameToWorld = %s -> %s", statPlaced.FrameToWorld.Src(), statPlaced.FrameToWorld.Dst())
	}

	// Snapshotted A2C values survive caller mutation: Dynamic = 506 mm, Static = 530 mm.
	assertNear(t, "dynamic A2C", dynPlaced.LowerCupBottom.Origin().DistanceTo(dynPlaced.ForkSteererBase.Origin()), 506.0, 1e-9)
	assertNear(t, "static A2C", statPlaced.LowerCupBottom.Origin().DistanceTo(statPlaced.ForkSteererBase.Origin()), 530.0, 1e-9)
	assertNear(t, "dynamic leveled axles", dynPlaced.FrontAxle.Origin().Y()-dynPlaced.RearAxle.Origin().Y(), 0, 1e-9)
	assertNear(t, "static leveled axles", statPlaced.FrontAxle.Origin().Y()-statPlaced.RearAxle.Origin().Y(), 0, 1e-9)
	if statPlaced.PitchDeg() <= dynPlaced.PitchDeg() {
		t.Errorf("static pitch %.4f° should exceed dynamic pitch %.4f° (unloaded fork is taller)",
			statPlaced.PitchDeg(), dynPlaced.PitchDeg())
	}

	// Frameset exposes ready-to-mate SeatTube and Steerer ports.
	stem, err := assembly.NewStem(40, 60, -6)
	if err != nil {
		t.Fatalf("NewStem: %v", err)
	}
	if _, err := assembly.MountStem(fs.Steerer(), 20, stem); err != nil {
		t.Fatalf("MountStem(fs.Steerer()): %v", err)
	}
	post, err := assembly.NewRigidPost(400, 100, 365, 0, 30)
	if err != nil {
		t.Fatalf("NewRigidPost: %v", err)
	}
	if _, err := assembly.MateSeatpost(fs.SeatTube(), post); err != nil {
		t.Fatalf("MateSeatpost(fs.SeatTube()): %v", err)
	}

	// Negative tests.
	if _, err := assembly.MateFrameset(frame, fenrirEc44Cup(), assembly.DefaultUpperStackMm, nil, fenrirWheels()); err == nil {
		t.Error("nil fork: want error")
	}
	assertPanics(t, "wrong CoordinateSystem fork axle", func() {
		_, _ = assembly.MateFrameset(frame, fenrirEc44Cup(), assembly.DefaultUpperStackMm, badSystemFork{axle: pose.NewPoint(assembly.FrameLocal, 500, 44)}, fenrirWheels())
	})
	assertPanics(t, "zero CoordinateSystem fork axle", func() {
		_, _ = assembly.MateFrameset(frame, fenrirEc44Cup(), assembly.DefaultUpperStackMm, badSystemFork{}, fenrirWheels())
	})
	assertPanics(t, "nil Frameset Place", func() {
		var nilFS *assembly.Frameset
		_ = nilFS.Place(assembly.Dynamic)
	})
	assertPanics(t, "Frameset invalid LoadState", func() {
		_ = fs.Place(assembly.LoadState("bad"))
	})
}
