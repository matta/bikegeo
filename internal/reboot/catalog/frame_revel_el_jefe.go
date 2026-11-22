package catalog

import "bikegeo/internal/reboot/assembly"

// revelElJefe returns the Revel El Jefe.
//
// The frame is a titanium trail hardtail with an IS52 integrated headset. Its
// basis fork is a 120 mm fork (530 mm static) at 20% sag, 506 mm effective, on
// the bare head tube. Sliding dropouts run 420–435 mm. The seat tube is bent;
// only the published effective STA is modeled (Q75f).
func revelElJefe() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType IS52; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore52Integrated,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 0},
	}
	return Frame{
		ID:             "revel-el-jefe",
		Manufacturer:   "Revel",
		Model:          "El Jefe",
		Year:           2024,
		DefaultSize:    "L",
		MaxChainstayMm: 435, // published: sliding dropouts 420–435 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis (Q44, Q81): legacy sid_120 @ 20% sag → 506 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 506, RakeMm: 44, Steerer: assembly.SteererBasis},
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
			{Fork: sid120, StaticA2cMm: 530, RakeMm: 44},
			{Fork: sid100, StaticA2cMm: 506, RakeMm: 44},
			{Fork: fox34_130, StaticA2cMm: 541, RakeMm: 44},
			{Fork: rigid510, StaticA2cMm: 510, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          435,
				StackMm:          599,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 86,
				BbDropMm:         68,
				ChainstayMm:      425,
				SeatTubeLengthMm: 374,
				WheelbaseMm:      1121,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          455,
				StackMm:          607,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 96,
				BbDropMm:         68,
				ChainstayMm:      425,
				SeatTubeLengthMm: 420,
				WheelbaseMm:      1146,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          474,
				StackMm:          615,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 106,
				BbDropMm:         68,
				ChainstayMm:      425,
				SeatTubeLengthMm: 455,
				WheelbaseMm:      1167,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          499,
				StackMm:          627,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 116,
				BbDropMm:         68,
				ChainstayMm:      425,
				SeatTubeLengthMm: 460,
				WheelbaseMm:      1199,
			},
		),
	}
}
