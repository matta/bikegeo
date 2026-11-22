package catalog

import "bikegeo/internal/reboot/assembly"

// ritcheyUltra returns the Ritchey Ultra.
//
// The frame is a steel hardtail with an IS52 integrated headset. Its CAD fork
// is a 120 mm fork at 20% sag on the bare head tube. Ritchey's published stacks
// imply a 508.1–508.9 mm A2C in every size, so the CAD fork is 508.5 mm with no
// allowance (Q71). Wheelbases are for a 44 mm offset fork.
func ritcheyUltra() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType IS52; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore52Integrated,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0},
	}
	return Frame{
		ID:           "ritchey-ultra",
		Manufacturer: "Ritchey",
		Model:        "Ultra",
		Year:         2024,
		DefaultSize:  "L",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis (Q44, Q81): legacy fox34_120 @ 20% sag → 506 mm effective. Q71 replaces that with
		// 508.5 mm, the A2C Ritchey's published stacks imply in every size, with no allowance.
		BasisFork: assembly.ForkSpec{StaticA2cMm: 508.5, RakeMm: 44, Steerer: assembly.SteererCAD},
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.25,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: fox34_120, StaticA2cMm: 530, RakeMm: 44},
			{Fork: rigid500, StaticA2cMm: 500, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          417,
				StackMm:          606,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 100,
				BbDropMm:         58,
				ChainstayMm:      439,
				SeatTubeLengthMm: 390,
				WheelbaseMm:      1121,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          430,
				StackMm:          613,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 105,
				BbDropMm:         58,
				ChainstayMm:      439,
				SeatTubeLengthMm: 432,
				WheelbaseMm:      1131,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          445,
				StackMm:          617,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 110,
				BbDropMm:         58,
				ChainstayMm:      439,
				SeatTubeLengthMm: 475,
				WheelbaseMm:      1148,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          460,
				StackMm:          627,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 120,
				BbDropMm:         58,
				ChainstayMm:      439,
				SeatTubeLengthMm: 515,
				WheelbaseMm:      1167,
			},
		),
	}
}
