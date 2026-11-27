package catalog

import "github.com/matta/bikegeo/internal/assembly"

// turnerVennTi returns the Turner Venn Ti.
//
// The frame is a titanium trail hardtail with a ZS56 semi-integrated headset.
// Its basis fork is a 130 mm fork published unweighted (541 mm, 0% sag) on the
// bare head tube. Legacy notes the XL stack (670 mm) looks like a transcription
// error for 690 mm; the published value is kept.
//
// Legacy notes: legacy_frame_notes.md, section turner-venn-ti.
func turnerVennTi() Frame {
	family := frameFamily{
		// inferred from legacy HeadsetCupType ZS56; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore56Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupZS56, StackMm: 4},
	}
	return Frame{
		ID:           "turner-venn-ti",
		Manufacturer: "Turner",
		Model:        "Venn Ti",
		Year:         2024,
		DefaultSize:  "L",
		// hypothesis: legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis: legacy fox34_130 @ 0% sag → 541 mm effective
		BasisFork: mustBasisFork(541, 44),
		// inferred from published digits (legacy)
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: compatible forks
		Forks: []Fork{fox34_130, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S",
				ReachMm:          410,
				StackMm:          628,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 90,
				BbDropMm:         65,
				ChainstayMm:      445,
				SeatTubeLengthMm: 410,
				WheelbaseMm:      1137,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          434,
				StackMm:          645,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 110,
				BbDropMm:         65,
				ChainstayMm:      445,
				SeatTubeLengthMm: 450,
				WheelbaseMm:      1170,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          448,
				StackMm:          662,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 130,
				BbDropMm:         62,
				ChainstayMm:      445,
				SeatTubeLengthMm: 490,
				WheelbaseMm:      1190,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          470,
				StackMm:          690,
				HeadTubeAngleDeg: 67,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 160,
				BbDropMm:         62,
				ChainstayMm:      445,
				SeatTubeLengthMm: 530,
				WheelbaseMm:      1224,
			},
		),
	}
}
