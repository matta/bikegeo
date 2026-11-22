package catalog

import "bikegeo/internal/assembly"

// salsaFargo returns the Salsa Fargo.
//
// The frame is a steel drop-bar bikepacking frame designed around the 483 mm
// Cutthroat Carbon Deluxe fork (51 mm offset). The chart is consistent with the bare
// head tube; physical forks install in the 12 mm EC44 cup. Alternator dropouts
// run 445–462 mm.
//
// Legacy notes: legacy_frame_notes.md, section salsa-fargo (Q145).
func salsaFargo() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "salsa-fargo",
		Manufacturer:   "Salsa",
		Model:          "Fargo",
		Year:           2024,
		DefaultSize:    "L",
		MaxChainstayMm: 462, // published: sliding dropouts 445–462 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis (Q44, Q81): legacy firestarter_deluxe_483 (rigid) → 483 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 483, RakeMm: 51, Steerer: assembly.SteererBasis},
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
		Forks: []Fork{firestarterDeluxe483, sid100},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "XS",
				ReachMm:          352,
				StackMm:          584,
				HeadTubeAngleDeg: 68.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 90,
				BbDropMm:         70,
				ChainstayMm:      462,
				SeatTubeLengthMm: 355.6,
				WheelbaseMm:      1067,
			},
			publishedSize{
				Name:             "S",
				ReachMm:          366,
				StackMm:          605,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         70,
				ChainstayMm:      462,
				SeatTubeLengthMm: 406.4,
				WheelbaseMm:      1083,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          371,
				StackMm:          633,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 140,
				BbDropMm:         70,
				ChainstayMm:      462,
				SeatTubeLengthMm: 457.2,
				WheelbaseMm:      1099,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          388,
				StackMm:          661,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 170,
				BbDropMm:         70,
				ChainstayMm:      462,
				SeatTubeLengthMm: 508,
				WheelbaseMm:      1126,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          412,
				StackMm:          680,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 190,
				BbDropMm:         70,
				ChainstayMm:      462,
				SeatTubeLengthMm: 558.8,
				WheelbaseMm:      1158,
			},
		),
	}
}
