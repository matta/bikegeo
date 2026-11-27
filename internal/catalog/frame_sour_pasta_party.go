package catalog

import "github.com/matta/bikegeo/internal/assembly"

// sourPastaParty returns the Sour Pasta Party.
//
// The frame is a CrMo steel trail hardtail. Its chart is drawn with a 120 mm
// fork at 15% sag (512 mm effective) and a 12 mm EC44 cup. Legacy says its
// wheelbases were adapted to a 44 mm offset fork, so they may not be Sour's
// printed values; they stay labelled published until the frame is re-ingested
// (Q146).
//
// Legacy notes: legacy_frame_notes.md, section sour-pasta-party (Q145).
func sourPastaParty() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "sour-pasta-party",
		Manufacturer: "Sour",
		Model:        "Pasta Party",
		Year:         2024,
		DefaultSize:  "L",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
		// hypothesis (Q44, Q81): legacy fox34_120 @ 15% sag → 512 mm effective
		BasisFork: mustBasisFork(512, 44),
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.05,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: compatible forks (Q128)
		Forks: []Fork{fox34_120, sid100},
		// published; wheelbases unverified (Q146)
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          415,
				StackMm:          607,
				HeadTubeAngleDeg: 66.6,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 85,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 395,
				WheelbaseMm:      1128,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          437,
				StackMm:          615,
				HeadTubeAngleDeg: 66.6,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 95,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 435,
				WheelbaseMm:      1153,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          467,
				StackMm:          619,
				HeadTubeAngleDeg: 66.6,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 100,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 485,
				WheelbaseMm:      1185,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          491,
				StackMm:          630,
				HeadTubeAngleDeg: 66.6,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         65,
				ChainstayMm:      435,
				SeatTubeLengthMm: 535,
				WheelbaseMm:      1214,
			},
		),
	}
}
