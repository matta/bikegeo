package catalog

import "bikegeo/internal/reboot/assembly"

// sklarPbj returns the Sklar PBJ.
//
// The frame is a chromoly rigid mountain touring frame designed around the 485
// mm Sklar steel fork (52 mm offset), whose straight 1-1/8" steerer runs in an
// EC44/30 reducer cup (Q66); the CAD cup is 12 mm EC44. Sliding dropouts run
// 432–452 mm. Legacy reads the published M wheelbase as 1156 mm (Sklar printed
// 1165, a digit transposition). Size L needs a documented axle-height allowance
// (Q71).
func sklarPbj() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		CadLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy sklar_steel_485 (rigid) → 485 mm effective
		CadFork: assembly.ForkGeometry{A2cMm: 485, RakeMm: 52},
		// inferred from published digits (legacy), Q80
		Precision: assembly.ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.25,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
	}
	return Frame{
		ID:             "sklar-pbj",
		Manufacturer:   "Sklar",
		Model:          "PBJ",
		Year:           2025,
		DefaultSize:    "M",
		MaxChainstayMm: 452, // published: sliding dropouts 432–452 mm (Q67)
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: sklarSteel485, StaticA2cMm: 485, RakeMm: 52},
			{Fork: sid100, StaticA2cMm: 506, RakeMm: 44},
			{Fork: fox34_120, StaticA2cMm: 530, RakeMm: 44},
		},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "XS",
				ReachMm:          411,
				StackMm:          574,
				HeadTubeAngleDeg: 68,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 70,
				BbDropMm:         68,
				ChainstayMm:      432,
				SeatTubeLengthMm: 360,
				WheelbaseMm:      1098,
			},
			publishedSize{
				Name:             "S",
				ReachMm:          433,
				StackMm:          588,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 85,
				BbDropMm:         68,
				ChainstayMm:      432,
				SeatTubeLengthMm: 410,
				WheelbaseMm:      1124,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          461,
				StackMm:          615,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         68,
				ChainstayMm:      432,
				SeatTubeLengthMm: 440,
				WheelbaseMm:      1156,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          475,
				StackMm:          641,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 140,
				BbDropMm:         68,
				ChainstayMm:      432,
				SeatTubeLengthMm: 505,
				WheelbaseMm:      1182,
				Allowance: assembly.ChartAllowance{
					AxleHeightMm: 0.4,
					Reason:       "Size L's published Stack (641 mm) implies a 481.7 mm fork A2C with the 12 mm EC44 CAD cup, 3.3 mm short of the 485 mm rigid fork; every other size is consistent within rounding (Q71).",
				},
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          508,
				StackMm:          670,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 170,
				BbDropMm:         68,
				ChainstayMm:      432,
				SeatTubeLengthMm: 530,
				WheelbaseMm:      1222,
			},
		),
	}
}
