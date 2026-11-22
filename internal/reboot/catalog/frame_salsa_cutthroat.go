package catalog

import "bikegeo/internal/reboot/assembly"

// salsaCutthroat returns the Salsa Cutthroat.
//
// The frame is a carbon drop-bar endurance frame with an IS52 integrated
// headset. The published geometry is drawn around a 100 mm fork (506 mm static)
// at 25% sag, 481 mm effective, 51 mm offset, on the bare head tube.
func salsaCutthroat() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType IS52; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore52Integrated,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0},
	}
	return Frame{
		ID:           "salsa-cutthroat",
		Manufacturer: "Salsa",
		Model:        "Cutthroat",
		Year:         2024,
		DefaultSize:  "56cm",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis (Q44, Q81): legacy sid_100 @ 25% sag → 481 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 481, RakeMm: 51, Steerer: assembly.SteererBasis},
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: firestarterDeluxe483, StaticA2cMm: 483, RakeMm: 51},
			{Fork: sid100, StaticA2cMm: 506, RakeMm: 51},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "52cm",
				ReachMm:          357,
				StackMm:          585,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 90,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 356,
				WheelbaseMm:      1050,
			},
			publishedSize{
				Name:             "54cm",
				ReachMm:          372,
				StackMm:          592,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74.25,
				HeadTubeLengthMm: 98,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 419,
				WheelbaseMm:      1066,
			},
			publishedSize{
				Name:             "56cm",
				ReachMm:          385,
				StackMm:          620,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74.25,
				HeadTubeLengthMm: 127,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 508,
				WheelbaseMm:      1091,
			},
			publishedSize{
				Name:             "58cm",
				ReachMm:          394,
				StackMm:          645,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 154,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 559,
				WheelbaseMm:      1110,
			},
			publishedSize{
				Name:             "60cm",
				ReachMm:          406,
				StackMm:          664,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73.75,
				HeadTubeLengthMm: 175,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 584,
				WheelbaseMm:      1129,
			},
		),
	}
}
