package catalog

import "bikegeo/internal/reboot/assembly"

// surlyKrampus returns the Surly Krampus.
//
// The frame is a chromoly 29+ trail hardtail designed around the 483 mm Krampus
// fork (47 mm offset) with a 12 mm EC44 basis cup.
//
// Legacy notes: legacy_frame_notes.md, section surly-krampus (Q145).
func surlyKrampus() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "surly-krampus",
		Manufacturer: "Surly",
		Model:        "Krampus",
		Year:         2024,
		DefaultSize:  "L",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy surly_krampus_483 (rigid) → 483 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 483, RakeMm: 47, Steerer: assembly.SteererBasis},
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
		Forks: []Fork{surlyKrampus483, sid100, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          404,
				StackMm:          593,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 90,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 368,
				WheelbaseMm:      1087,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          426,
				StackMm:          602,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 100,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 419,
				WheelbaseMm:      1113,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          448,
				StackMm:          612,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 110,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 470,
				WheelbaseMm:      1138,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          470,
				StackMm:          621,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 120,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 521,
				WheelbaseMm:      1164,
			},
		),
	}
}
