package catalog

import "github.com/matta/bikegeo/internal/assembly"

// tumbleweedSunliner returns the Tumbleweed Sunliner.
//
// The frame is a chromoly rigid touring frame designed around the 440 mm
// unicrown fork (55 mm offset), whose straight steerer runs in an EC44/30
// reducer cup (Q66); the basis cup is 13 mm EC44.
//
// Legacy notes: legacy_frame_notes.md, section tumbleweed-sunliner (Q145).
func tumbleweedSunliner() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "tumbleweed-sunliner",
		Manufacturer: "Tumbleweed",
		Model:        "Sunliner",
		Year:         2024,
		DefaultSize:  "L",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 13
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 13},
		// hypothesis (Q44, Q81): legacy sunliner_440 (rigid) → 440 mm effective
		BasisFork: mustBasisFork(440, 55),
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
		Forks: []Fork{sunliner440},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          420,
				StackMm:          590,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 130,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 380,
				WheelbaseMm:      1131,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          434,
				StackMm:          608,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 150,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 430,
				WheelbaseMm:      1153,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          447,
				StackMm:          632,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 175,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 480,
				WheelbaseMm:      1175,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          457,
				StackMm:          664,
				HeadTubeAngleDeg: 69,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 210,
				BbDropMm:         65,
				ChainstayMm:      456,
				SeatTubeLengthMm: 545,
				WheelbaseMm:      1197,
			},
		),
	}
}
