package catalog

import "bikegeo/internal/reboot/assembly"

// eskerHayduke returns the Esker Hayduke.
//
// The frame is a chromoly adventure hardtail. Its basis fork is a 120 mm fork at
// 30% sag (495 mm effective) with a 12 mm EC44 basis cup. Portage dropouts run
// 440–454 mm; the chart is drawn at 440 mm. The S4 wheelbase needs a documented
// allowance (Q71).
//
// Legacy notes: legacy_frame_notes.md, section esker-hayduke (Q145).
func eskerHayduke() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "esker-hayduke",
		Manufacturer:   "Esker",
		Model:          "Hayduke",
		Year:           2024,
		DefaultSize:    "S3",
		MaxChainstayMm: 454, // published: sliding dropouts 440–454 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy pike_120 @ 30% sag → 495 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 44, Steerer: assembly.SteererBasis},
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
		// catalog: compatible forks (Q128, Q133)
		Forks: []Fork{pike120, fox34_130, enveMountainBoost, lithicCarbonMountain},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S1",
				ReachMm:          420,
				StackMm:          615,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 105,
				BbDropMm:         65,
				ChainstayMm:      440,
				SeatTubeLengthMm: 380,
				WheelbaseMm:      1130,
			},
			publishedSize{
				Name:             "S2",
				ReachMm:          450,
				StackMm:          620,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         65,
				ChainstayMm:      440,
				SeatTubeLengthMm: 435,
				WheelbaseMm:      1160,
			},
			publishedSize{
				Name:             "S3",
				ReachMm:          470,
				StackMm:          630,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 120,
				BbDropMm:         65,
				ChainstayMm:      440,
				SeatTubeLengthMm: 485,
				WheelbaseMm:      1183,
			},
			publishedSize{
				Name:             "S4",
				ReachMm:          500,
				StackMm:          645,
				HeadTubeAngleDeg: 67.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 140,
				BbDropMm:         65,
				ChainstayMm:      440,
				SeatTubeLengthMm: 530,
				WheelbaseMm:      1218,
				Allowance: ChartAllowance{
					WheelbaseMm: 1.2,
					Reason:      "The published S4 wheelbase is 5.42 mm longer than the declared front axle position (HTL + cup + A2C) gives, 1.11 mm beyond rounding; S1–S3 are consistent within rounding (Q71).",
				},
			},
		),
	}
}
