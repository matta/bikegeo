package catalog

import "bikegeo/internal/reboot/assembly"

// fairlightHolt2 returns the Fairlight Holt 2.0.
//
// The frame is a Reynolds 853 steel XC/downcountry hardtail. Its basis fork is a
// 120 mm fork at 20% sag (507 mm effective) with a 12 mm EC44 basis cup. The seat
// tube is bent; only the published effective STA is modeled (Q75f).
func fairlightHolt2() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "fairlight-holt-2",
		Manufacturer: "Fairlight",
		Model:        "Holt 2.0",
		Year:         2025,
		DefaultSize:  "L",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy pike_120 @ 20% sag → 507 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 507, RakeMm: 44, Steerer: assembly.SteererBasis},
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
			{Fork: pike120, StaticA2cMm: 531, RakeMm: 44},
			{Fork: sid100, StaticA2cMm: 506, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          425,
				StackMm:          616,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 75.3,
				HeadTubeLengthMm: 100,
				BbDropMm:         63,
				ChainstayMm:      435,
				SeatTubeLengthMm: 385,
				WheelbaseMm:      1138,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          450,
				StackMm:          616,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 75.3,
				HeadTubeLengthMm: 100,
				BbDropMm:         63,
				ChainstayMm:      435,
				SeatTubeLengthMm: 425,
				WheelbaseMm:      1163,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          470,
				StackMm:          625,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 75.3,
				HeadTubeLengthMm: 110,
				BbDropMm:         63,
				ChainstayMm:      435,
				SeatTubeLengthMm: 465,
				WheelbaseMm:      1187,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          495,
				StackMm:          643,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 75.2,
				HeadTubeLengthMm: 130,
				BbDropMm:         63,
				ChainstayMm:      435,
				SeatTubeLengthMm: 510,
				WheelbaseMm:      1220,
			},
		),
	}
}
