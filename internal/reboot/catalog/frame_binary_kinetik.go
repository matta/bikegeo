package catalog

import "bikegeo/internal/reboot/assembly"

// binaryKinetik returns the Binary Kinetik.
//
// The frame is a titanium expedition hardtail. Its basis fork is a 120 mm fork
// (530 mm static) at 15% sag, 512 mm effective, with a 10 mm EC44 basis cup.
// Rocker dropouts run 455–472 mm; the chart is drawn at 455 mm.
func binaryKinetik() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "binary-kinetik",
		Manufacturer:   "Binary",
		Model:          "Kinetik",
		Year:           2026,
		DefaultSize:    "L",
		MaxChainstayMm: 472, // published: sliding dropouts 455–472 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 10
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 10},
		// hypothesis (Q44, Q81): legacy fox34_120 @ 15% sag → 512 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 512, RakeMm: 44, Steerer: assembly.SteererBasis},
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
		// catalog: compatible forks (Q128)
		Forks: []Fork{fox34_120, sid100, rigid500},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          417,
				StackMm:          616,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 100,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 390,
				WheelbaseMm:      1151,
			},
			publishedSize{
				Name:             "SMD",
				ReachMm:          433,
				StackMm:          616,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 100,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 415,
				WheelbaseMm:      1167,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          434,
				StackMm:          625,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 420,
				WheelbaseMm:      1172,
			},
			publishedSize{
				Name:             "ML",
				ReachMm:          441,
				StackMm:          643,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 130,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 440,
				WheelbaseMm:      1187,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          456,
				StackMm:          657,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 145,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 480,
				WheelbaseMm:      1208,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          465,
				StackMm:          671,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 160,
				BbDropMm:         60,
				ChainstayMm:      455,
				SeatTubeLengthMm: 508,
				WheelbaseMm:      1223,
			},
		),
	}
}
