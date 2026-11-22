package catalog

import "bikegeo/internal/reboot/assembly"

// mootsMountaineer returns the Moots Mountaineer.
//
// The frame is a titanium YBB micro-suspension trail frame. Its CAD fork is a
// 120 mm fork at 20% sag (507 mm effective) with a 12 mm EC44 CAD cup. Moots
// publishes no wheelbase (Q57).
func mootsMountaineer() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "moots-mountaineer",
		Manufacturer: "Moots",
		Model:        "Mountaineer",
		Year:         2024,
		DefaultSize:  "M",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy pike_120 @ 20% sag → 507 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 507, RakeMm: 44, Steerer: assembly.SteererCAD},
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.25,
			WheelbaseMm:      0.5,
		},
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: pike120, StaticA2cMm: 531, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          416,
				StackMm:          612,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 100,
				BbDropMm:         54,
				ChainstayMm:      444.5,
				SeatTubeLengthMm: 469.9,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          419,
				StackMm:          622,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 74.25,
				HeadTubeLengthMm: 110,
				BbDropMm:         54,
				ChainstayMm:      444.5,
				SeatTubeLengthMm: 482.6,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          444,
				StackMm:          635,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 73.75,
				HeadTubeLengthMm: 120,
				BbDropMm:         60,
				ChainstayMm:      444.5,
				SeatTubeLengthMm: 495.3,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          465,
				StackMm:          645,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 130,
				BbDropMm:         60,
				ChainstayMm:      444.5,
				SeatTubeLengthMm: 533.4,
			},
		),
	}
}
