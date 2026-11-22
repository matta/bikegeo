package catalog

import "bikegeo/internal/reboot/assembly"

// binaryHavok returns the Binary Havok.
//
// The frame is a titanium 29+ drop-bar adventure frame. Its basis fork is a 485
// mm rigid fork with a 10 mm EC44 basis cup. Rocker sliding dropouts run 445–460
// mm; the chart is drawn at the forward 445 mm position.
func binaryHavok() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "binary-havok",
		Manufacturer:   "Binary",
		Model:          "Havok",
		Year:           2026,
		DefaultSize:    "L",
		MaxChainstayMm: 460, // published: sliding dropouts 445–460 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 10
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 10},
		// hypothesis (Q44, Q81): legacy rigid_485 (rigid) → 485 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 485, RakeMm: 45, Steerer: assembly.SteererBasis},
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
		// catalog: compatible forks (Q128)
		Forks: []Fork{sid100, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          367,
				StackMm:          621,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 115,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 460,
				WheelbaseMm:      1072,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          386,
				StackMm:          627,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 120,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 480,
				WheelbaseMm:      1092,
			},
			publishedSize{
				Name:             "ML",
				ReachMm:          389,
				StackMm:          642,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 135,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 500,
				WheelbaseMm:      1096,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          398,
				StackMm:          656,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 150,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 525,
				WheelbaseMm:      1111,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          408,
				StackMm:          670,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 165,
				BbDropMm:         70,
				ChainstayMm:      445,
				SeatTubeLengthMm: 555,
				WheelbaseMm:      1126,
			},
		),
	}
}
