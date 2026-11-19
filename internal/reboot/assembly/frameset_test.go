package assembly_test

import (
	"errors"
	"math"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func fenrirSizeLFrame() assembly.FrameSetup {
	return assembly.FrameSetup{
		ReachMm:          428.0,
		StackMm:          642.0,
		HeadTubeAngleDeg: 68.25,
		SeatTubeAngleDeg: 75.25,
		HeadTubeLengthMm: 140.0,
		BbDropMm:         72.0,
		ChainstayMm:      450.0,
		SeatTubeLengthMm: 490.0,
	}
}

func fenrirWheels() assembly.WheelSetup {
	return assembly.WheelSetup{RadiusMm: 372.0}
}

func TestPlaceFramesetCADBaselineForkAndInvariants(t *testing.T) {
	frame := fenrirSizeLFrame()
	wheels := fenrirWheels()
	cadFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{
			StaticA2cMm: 495.0,
			TravelMm:    0.0,
			RakeMm:      54.0,
		},
		SagPct: 0.0,
	}

	tr := pose.NewTrace()
	placed, err := assembly.PlaceFrameset(frame, cadFork, wheels, tr)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	tr.Freeze()

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

	// CAD baseline fork on Otso Fenrir L closes within chart rounding (~0° pitch, ~1158 mm wheelbase).
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
	assertNear(t, "lower cup", placed.HeadTubeBottom.DistanceTo(placed.LowerCupBottom), frame.LowerCupMm, 1e-9)
	assertNear(t, "fork A2C", placed.LowerCupBottom.DistanceTo(placed.ForkSteererBase), cadFork.EffectiveA2cMm(), 1e-9)
	assertNear(t, "fork rake", placed.ForkSteererBase.DistanceTo(placed.FrontAxle), cadFork.Spec.RakeMm, 1e-9)
	assertNear(t, "seat tube length", placed.BB.DistanceTo(placed.SeatTubeTop), frame.SeatTubeLengthMm, 1e-9)

	// SeatAxis is anchored at BB, points up the leveled seat tube axis, and parents SeatTubeTop.
	assertNear(t, "SeatAxis.X", placed.SeatAxis.Origin().X, 0, 1e-12)
	assertNear(t, "SeatAxis.Y", placed.SeatAxis.Origin().Y, 0, 1e-12)
	assertNear(t, "SeatAxis.Heading == SeatTubeTop.Heading", float64(placed.SeatAxis.HeadingDeg()), float64(placed.SeatTubeTop.HeadingDeg()), 1e-12)
	assertNear(t, "SeatAxis to SeatTubeTop", placed.SeatAxis.DistanceTo(placed.SeatTubeTop), frame.SeatTubeLengthMm, 1e-9)
	assertNear(t, "SeatAxis.Heading == BB.Heading + 180 - STA", float64(placed.SeatAxis.HeadingDeg()), float64(placed.BB.HeadingDeg()+(180.0-frame.SeatTubeAngleDeg)), 1e-12)

	steps := tr.Steps()
	seatTopStep := steps[placed.SeatTubeTop.StepID()-1]
	if seatTopStep.ParentID != placed.SeatAxis.StepID() {
		t.Fatalf("SeatTubeTop ParentID = %d, want SeatAxis.StepID %d", seatTopStep.ParentID, placed.SeatAxis.StepID())
	}

	// Trace-on/off equivalence.
	untraced, err := assembly.PlaceFrameset(frame, cadFork, wheels, nil)
	if err != nil {
		t.Fatalf("PlaceFrameset untraced: %v", err)
	}
	assertNear(t, "untraced FrontAxle.X", untraced.FrontAxle.Origin().X, placed.FrontAxle.Origin().X, 1e-12)
	assertNear(t, "untraced FrontAxle.Y", untraced.FrontAxle.Origin().Y, placed.FrontAxle.Origin().Y, 1e-12)
	assertNear(t, "untraced RearContact.X", untraced.RearTireContact.Origin().X, placed.RearTireContact.Origin().X, 1e-12)
	assertNear(t, "untraced RearContact.Y", untraced.RearTireContact.Origin().Y, placed.RearTireContact.Origin().Y, 1e-12)
	assertNear(t, "untraced FrontContact.X", untraced.FrontTireContact.Origin().X, placed.FrontTireContact.Origin().X, 1e-12)
	assertNear(t, "untraced FrontContact.Y", untraced.FrontTireContact.Origin().Y, placed.FrontTireContact.Origin().Y, 1e-12)
	assertNear(t, "untraced SeatAxis.Heading", float64(untraced.SeatAxis.HeadingDeg()), float64(placed.SeatAxis.HeadingDeg()), 1e-12)
	assertNear(t, "untraced HeadTubeTop.Heading", float64(untraced.HeadTubeTop.HeadingDeg()), float64(placed.HeadTubeTop.HeadingDeg()), 1e-12)
}

func TestPlaceFramesetInstalledForkPitchAndLeveling(t *testing.T) {
	frame := fenrirSizeLFrame()
	frame.LowerCupMm = 12.0
	wheels := fenrirWheels()

	// Installed 12 mm EC44 lower cup + 120 mm Fox 34 at 20% sag (resolved A2C = 506 mm).
	// Total front-end length increases by 12 + (506 - 495) = 23 mm, raising the front end
	// (positive frame pitch, slackening the leveled head tube angle) and lowering BB drop.
	fox34Spec := assembly.ForkSpec{
		StaticA2cMm: 530.0,
		TravelMm:    120.0,
		RakeMm:      44.0,
	}
	fork20 := assembly.ForkSetup{Spec: fox34Spec, SagPct: 20.0}
	assertNear(t, "resolved A2C at 20% sag", fork20.EffectiveA2cMm(), 506.0, 1e-9)

	placed, err := assembly.PlaceFrameset(frame, fork20, wheels, nil)
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
	placedMoreSag, err := assembly.PlaceFrameset(frame, fork30, wheels, nil)
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
	fork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54},
	}
	wheels := fenrirWheels()

	badFrame := frame
	badFrame.ChainstayMm = 70 // <= BbDropMm (72)
	if _, err := assembly.PlaceFrameset(badFrame, fork, wheels, nil); err == nil {
		t.Fatal("expected error when chainstay <= BB drop")
	}

	for _, badWheels := range []assembly.WheelSetup{
		{RadiusMm: 0},
		{RadiusMm: -10},
		{RadiusMm: math.NaN()},
	} {
		if _, err := assembly.PlaceFrameset(frame, fork, badWheels, nil); err == nil {
			t.Fatalf("expected error for WheelSetup %+v", badWheels)
		}
	}

	inputCases := []struct {
		name       string
		lowerCupMm float64
		fork       assembly.ForkSetup
		wantField  assembly.InputField
	}{
		{"zero static A2C", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 0, RakeMm: 54}}, assembly.InputForkA2c},
		{"negative travel", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: -10, RakeMm: 44}}, ""},
		{"NaN rake", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, RakeMm: math.NaN()}}, ""},
		{"negative sag", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44}, SagPct: -5}, assembly.InputForkSag},
		{"sag over 100%", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 506, TravelMm: 100, RakeMm: 44}, SagPct: 105}, assembly.InputForkSag},
		{"non-zero sag on rigid fork", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, TravelMm: 0, RakeMm: 54}, SagPct: 20}, assembly.InputForkSag},
		{"non-positive effective A2C", 0, assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 80, TravelMm: 100, RakeMm: 44}, SagPct: 90}, assembly.InputForkA2c},
		{"negative lower cup", -1, fork, assembly.InputLowerCup},
	}
	for _, tc := range inputCases {
		t.Run(tc.name, func(t *testing.T) {
			f := frame
			f.LowerCupMm = tc.lowerCupMm
			_, err := assembly.PlaceFrameset(f, tc.fork, wheels, nil)
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

func TestPlaceCadBaselineFork(t *testing.T) {
	frame := fenrirSizeLFrame()
	wheels := fenrirWheels()
	chart := assembly.FrameChart{
		Frame: frame,
		CadFork: assembly.ForkGeometry{
			A2cMm:  495.0,
			RakeMm: 54.0,
		},
		PublishedWheelbaseMm: 1158.0,
	}

	baselineFork, err := assembly.PlaceCadBaselineFork(chart, wheels)
	if err != nil {
		t.Fatalf("PlaceCadBaselineFork: %v", err)
	}
	if math.Abs(baselineFork.Residuals.AxleHeightDiffMm) >= 1.5 {
		t.Fatalf("AxleHeightDiffMm = %.4f mm, want within 1.5 mm rounding", baselineFork.Residuals.AxleHeightDiffMm)
	}
	if math.Abs(baselineFork.Residuals.WheelbaseDiffMm) >= 1.5 {
		t.Fatalf("WheelbaseDiffMm = %.4f mm, want within 1.5 mm rounding", baselineFork.Residuals.WheelbaseDiffMm)
	}

	cadForkSetup := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0},
	}
	placed, err := assembly.PlaceFrameset(chart.Frame, cadForkSetup, wheels, nil)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	assertNear(t, "baseline fork pitch == PlaceFrameset", float64(baselineFork.Placement.PitchDeg()), float64(placed.PitchDeg()), 1e-12)
	assertNear(t, "baseline fork wheelbase == PlaceFrameset", baselineFork.Placement.WheelbaseMm(), placed.WheelbaseMm(), 1e-12)
	assertNear(t, "baseline fork FrontTireContact.Y", baselineFork.Placement.FrontTireContact.Origin().Y, placed.FrontTireContact.Origin().Y, 1e-12)

	installedFrame := chart.Frame
	installedFrame.LowerCupMm = 12.0
	installedFork := assembly.ForkSetup{
		Spec:   assembly.ForkSpec{StaticA2cMm: 530.0, TravelMm: 120.0, RakeMm: 44.0},
		SagPct: 20.0,
	}
	installed, err := assembly.PlaceFrameset(installedFrame, installedFork, wheels, nil)
	if err != nil {
		t.Fatalf("PlaceFrameset installed: %v", err)
	}
	wantDelta := installed.PitchDeg() - baselineFork.Placement.PitchDeg()
	assertNear(t, "DeltaPitchDeg", float64(baselineFork.DeltaPitchDeg(installed)), float64(wantDelta), 1e-12)
	if baselineFork.DeltaPitchDeg(installed) <= 0 {
		t.Fatalf("DeltaPitchDeg = %.4f°, want > 0° for taller installed front end", baselineFork.DeltaPitchDeg(installed))
	}

	badChart := chart
	badChart.PublishedWheelbaseMm = 0
	if _, err := assembly.PlaceCadBaselineFork(badChart, wheels); err == nil {
		t.Fatal("expected error for zero published wheelbase")
	}
	badChart.PublishedWheelbaseMm = math.NaN()
	if _, err := assembly.PlaceCadBaselineFork(badChart, wheels); err == nil {
		t.Fatal("expected error for NaN published wheelbase")
	}
	badChart = chart
	badChart.CadFork.A2cMm = 0
	if _, err := assembly.PlaceCadBaselineFork(badChart, wheels); err == nil {
		t.Fatal("expected error for non-positive CadFork.A2cMm")
	}
	badChart = chart
	badChart.CadFork.RakeMm = math.NaN()
	if _, err := assembly.PlaceCadBaselineFork(badChart, wheels); err == nil {
		t.Fatal("expected error for NaN CadFork.RakeMm")
	}
	badChart = chart
	badChart.Frame.LowerCupMm = -1
	if _, err := assembly.PlaceCadBaselineFork(badChart, wheels); err == nil {
		t.Fatal("expected error for negative Frame.LowerCupMm")
	}
}
