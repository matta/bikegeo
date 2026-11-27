package catalog

import "github.com/matta/bikegeo/internal/assembly"

// otsoFenrirStainless returns the Otso Fenrir Stainless.
//
// The frame is a stainless bikepacking frame. S1's geometry chart is transcribed
// at the nominal middle Tuning Chip position (68° HTA, 75° STA, 70 mm BB drop,
// 440 mm chainstay), with unpublished exact wheelbases and all five
// published seatpost insertion depths. Chart consistency passes on the
// bare head tube with S1's printed 495 mm A2C / 51 mm offset basis fork;
// physical forks install in the typical 12 mm EC44 cup.
//
// Source record: frame_otso_fenrir_stainless.md.
// Legacy notes: legacy_frame_notes.md, section otso-fenrir-stainless.
func otsoFenrirStainless() Frame {
	family := frameFamily{
		// published standard S1 (ZS44/28.6 upper, EC44/40.0 lower); stack is Cane Creek 40 typical EC44 (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "otso-fenrir-stainless",
		Manufacturer: "Otso",
		Model:        "Fenrir Stainless",
		Year:         2024, // catalog (chart dates to 2022, S2/S4)
		DefaultSize:  "L",
		// hypothesis: bare head tube (S1 omits lower cup stack)
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis: S1 chart printed rigid fork 495 mm A2C / 51 mm offset
		BasisFork: mustBasisFork(495, 51),
		// derived from published S1 digits (68° integer HTA, whole-mm lengths)
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
		Forks: []Fork{lithicCarbonMountain, enveMountainBoost, sid100, fox34_120},
		// published S1 nominal middle Tuning Chip chart
		Sizes: family.sizes(
			publishedSize{
				Name:               "XS",
				ReachMm:            387,
				StackMm:            595,
				HeadTubeAngleDeg:   68,
				SeatTubeAngleDeg:   75,
				HeadTubeLengthMm:   90,
				BbDropMm:           70,  // derived: midpoint of published 68–72 mm range
				ChainstayMm:        440, // derived: midpoint of published 430–450 mm range
				SeatTubeLengthMm:   360,
				MaxPostInsertionMm: 170, // published (S1, S3)
			},
			publishedSize{
				Name:               "S",
				ReachMm:            403,
				StackMm:            605,
				HeadTubeAngleDeg:   68,
				SeatTubeAngleDeg:   75,
				HeadTubeLengthMm:   100,
				BbDropMm:           70,
				ChainstayMm:        440,
				SeatTubeLengthMm:   400,
				MaxPostInsertionMm: 210, // published (S1, S3)
			},
			publishedSize{
				Name:               "M",
				ReachMm:            416,
				StackMm:            623,
				HeadTubeAngleDeg:   68,
				SeatTubeAngleDeg:   75,
				HeadTubeLengthMm:   120,
				BbDropMm:           70,
				ChainstayMm:        440,
				SeatTubeLengthMm:   450,
				MaxPostInsertionMm: 260, // published (S1, S3)
			},
			publishedSize{
				Name:               "L",
				ReachMm:            428,
				StackMm:            642,
				HeadTubeAngleDeg:   68,
				SeatTubeAngleDeg:   75,
				HeadTubeLengthMm:   140,
				BbDropMm:           70,
				ChainstayMm:        440,
				SeatTubeLengthMm:   490,
				MaxPostInsertionMm: 290, // published (S1, S3)
			},
			publishedSize{
				Name:               "XL",
				ReachMm:            441,
				StackMm:            660,
				HeadTubeAngleDeg:   68,
				SeatTubeAngleDeg:   75,
				HeadTubeLengthMm:   160,
				BbDropMm:           70,
				ChainstayMm:        440,
				SeatTubeLengthMm:   530,
				MaxPostInsertionMm: 330, // published (S1, S3)
			},
		),
	}
}
