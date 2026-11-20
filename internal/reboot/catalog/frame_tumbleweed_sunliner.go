package catalog

import "bikegeo/internal/reboot/assembly"

// tumbleweedSunliner returns the Tumbleweed Sunliner.
//
// The frame is a chromoly rigid touring frame designed around the 440 mm
// unicrown fork (55 mm offset), whose straight steerer runs in an EC44/30
// reducer cup (Q66); the CAD cup is 13 mm EC44.
func tumbleweedSunliner() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 13
		CadLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 13},
		// hypothesis (Q44, Q81): legacy sunliner_440 (rigid) → 440 mm effective
		CadFork: assembly.ForkGeometry{A2cMm: 440, RakeMm: 55},
		// inferred from published digits (legacy), Q80
		Precision: assembly.ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
	}
	return Frame{
		ID:           "tumbleweed-sunliner",
		Manufacturer: "Tumbleweed",
		Model:        "Sunliner",
		Year:         2024,
		DefaultSize:  "L",
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: sunliner440, StaticA2cMm: 440, RakeMm: 55},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          420,
				StackMm:          590,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 130,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 380,
				WheelbaseMm:      1131,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          434,
				StackMm:          608,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 150,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 430,
				WheelbaseMm:      1153,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          447,
				StackMm:          632,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 175,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 480,
				WheelbaseMm:      1175,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          457,
				StackMm:          664,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 210,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 545,
				WheelbaseMm:      1197,
			},
		),
	}
}
