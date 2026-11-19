package web

import (
	"fmt"

	"bikegeo/internal/reboot/assembly"
)

type forkOption struct {
	ID            string
	Label         string
	Spec          assembly.ForkSpec
	DefaultSagPct float64
}

func makeForkOption(id, name string, spec assembly.ForkSpec, defaultSagPct float64) forkOption {
	var kind string
	if spec.TravelMm == 0 {
		kind = "Rigid"
	} else {
		kind = fmt.Sprintf("%.0f mm travel", spec.TravelMm)
	}
	return forkOption{
		ID:            id,
		Label:         fmt.Sprintf("%s (%s, %.0f mm A2C / %.0f mm rake)", name, kind, spec.StaticA2cMm, spec.RakeMm),
		Spec:          spec,
		DefaultSagPct: defaultSagPct,
	}
}

type bikeFixture struct {
	Name              string
	Chart             assembly.FrameChart
	Wheels            assembly.WheelSetup
	WheelLabel        string
	DefaultLowerCupMm float64
	DefaultForkID     string
	Forks             []forkOption
	DefaultStem       assembly.StemSetup
	HandlebarName     string
	DefaultHandlebar  assembly.HandlebarSetup
	DefaultSeatpost   assembly.SeatpostSetup
	SaddleName        string
	DefaultSaddle     assembly.SaddleSetup
}

// otsoFenrirFixture returns the Milestone 1 Otso Fenrir Stainless (2024, Size L)
// fixture with its sourced published chart dimensions and selectable forks.
func otsoFenrirFixture() bikeFixture {
	return bikeFixture{
		Name: "Otso Fenrir Stainless (2024, Size L)",
		Chart: assembly.FrameChart{
			Frame: assembly.FrameSetup{
				ReachMm:          428.0,
				StackMm:          642.0,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 140.0,
				BbDropMm:         72.0,
				ChainstayMm:      450.0,
				SeatTubeLengthMm: 490.0,
			},
			PublishedWheelbaseMm: 1158.0,
			CadFork: assembly.ForkSetup{
				A2cMm:      495.0,
				RakeMm:     54.0,
				LowerCupMm: 0.0,
			},
		},
		Wheels:            assembly.WheelSetup{RadiusMm: 372.0},
		WheelLabel:        `29" × 2.4"`,
		DefaultLowerCupMm: 12.0, // EC44 external lower headset cup
		DefaultForkID:     "lithic_carbon_mountain",
		Forks: []forkOption{
			makeForkOption("lithic_carbon_mountain", "Lithic Carbon Mountain", assembly.ForkSpec{
				StaticA2cMm: 495.0,
				TravelMm:    0.0,
				RakeMm:      54.0,
			}, 0.0),
			makeForkOption("sid_100", "RockShox SID", assembly.ForkSpec{
				StaticA2cMm: 506.0,
				TravelMm:    100.0,
				RakeMm:      44.0,
			}, 20.0),
			makeForkOption("fox34_120", "Fox 34", assembly.ForkSpec{
				StaticA2cMm: 530.0,
				TravelMm:    120.0,
				RakeMm:      44.0,
			}, 20.0),
		},
		DefaultStem: assembly.StemSetup{
			UpperCoverMm:  11.0,
			SpacersMm:     20.0,
			ClampHeightMm: 40.0,
			LengthMm:      60.0,
			AngleDeg:      -6.0,
		},
		HandlebarName: "Tumbleweed Persuader 20° (800 mm)",
		DefaultHandlebar: assembly.HandlebarSetup{
			StockWidthMm:   800.0,
			CurrentWidthMm: 800.0,
			BacksweepDeg:   20.0,
			UpsweepDeg:     4.0,
			StatedRiseMm:   50.0,
			RiseMode:       assembly.RiseModeBarEndCenter,
			BendStartMm:    135.0,
			GripInsetMm:    70.0,
			BarRollDeg:     0.0,
		},
		DefaultSeatpost: assembly.SeatpostSetup{
			ExtensionMm:   263.5,
			PostSetbackMm: 0.0,
			ClampLengthMm: 30.0,
		},
		SaddleName: "SQlab 610 Ergolux Active 2.1",
		DefaultSaddle: assembly.SaddleSetup{
			RailCenterToBrpMm:      32.0,
			RailCenterToSitBonesMm: -48.0,
			SitBonesElevationMm:    10.0,
			RailLengthMm:           60.0,
			RailPitchDeg:           0.0,
			RailOffsetMm:           0.0,
		},
	}
}
