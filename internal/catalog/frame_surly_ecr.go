package catalog

import "github.com/matta/bikegeo/internal/assembly"

// surlyEcr returns the Surly ECR.
//
// The frame is a chromoly 29+ rigid adventure bike designed around the 468 mm
// ECR fork (43 mm offset), with a traditional 34 mm head tube (12 mm EC34 basis
// cup). Only straight-steerer forks fit.
//
// Legacy notes: legacy_frame_notes.md, section surly-ecr.
func surlyEcr() Frame {
	family := frameFamily{
		// inferred from legacy HeadsetCupType EC34; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore34Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12},
	}
	return Frame{
		ID:           "surly-ecr",
		Manufacturer: "Surly",
		Model:        "ECR",
		Year:         2021,
		DefaultSize:  "L",
		// hypothesis: legacy GeoHeadsetSpacerMm 12
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12},
		// hypothesis: legacy surly_ecr_468 (rigid) → 468 mm effective
		BasisFork: mustBasisFork(468, 43),
		// inferred from published digits (legacy)
		ChartPrecision: ChartPrecision{
			ReachMm:          0.25,
			StackMm:          0.25,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.25,
		},
		// catalog: compatible forks
		Forks: []Fork{surlyEcr468},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "M",
				ReachMm:          408,
				StackMm:          609.5,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 72.5,
				HeadTubeLengthMm: 100,
				BbDropMm:         80,
				ChainstayMm:      451,
				SeatTubeLengthMm: 457.2,
				WheelbaseMm:      1090,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          420.5,
				StackMm:          633,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 72.5,
				HeadTubeLengthMm: 125,
				BbDropMm:         80,
				ChainstayMm:      451,
				SeatTubeLengthMm: 507.9,
				WheelbaseMm:      1111,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          433,
				StackMm:          656.5,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 72.5,
				HeadTubeLengthMm: 150,
				BbDropMm:         80,
				ChainstayMm:      451,
				SeatTubeLengthMm: 558.7,
				WheelbaseMm:      1132.5,
			},
		),
	}
}
