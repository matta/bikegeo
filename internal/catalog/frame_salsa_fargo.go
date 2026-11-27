package catalog

import "github.com/matta/bikegeo/internal/assembly"

// salsaFargo returns the Salsa Fargo.
//
// The frame is a steel drop-bar bikepacking frame designed around the 483 mm
// Cutthroat Carbon Deluxe fork (51 mm offset). The chart is consistent with the bare
// head tube; physical forks install in the 12 mm EC44 cup. Alternator dropouts
// run 445–462 mm.
//
// Legacy notes: legacy_frame_notes.md, section salsa-fargo.
func salsaFargo() Frame {
	family := frameFamily{
		// inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "salsa-fargo",
		Manufacturer:   "Salsa",
		Model:          "Fargo",
		Year:           2024,
		DefaultSize:    "L",
		MaxChainstayMm: 462, // published: sliding dropouts 445–462 mm
		// hypothesis: legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis: legacy firestarter_deluxe_483 (rigid) → 483 mm effective
		BasisFork: mustBasisFork(483, 51),
		// inferred from published digits (legacy)
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.25,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: compatible forks
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
