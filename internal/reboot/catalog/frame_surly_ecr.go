package catalog

import "bikegeo/internal/reboot/assembly"

// surlyEcr returns the Surly ECR.
//
// The frame is a chromoly 29+ rigid adventure bike designed around the 468 mm
// ECR fork (43 mm offset), with a traditional 34 mm head tube (12 mm EC34 CAD
// cup). Only straight-steerer forks fit.
func surlyEcr() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC34; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore34Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12},
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 12
		CadLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC34, StackMm: 12},
		// hypothesis (Q44, Q81): legacy surly_ecr_468 (rigid) → 468 mm effective
		CadFork: assembly.ForkGeometry{A2cMm: 468, RakeMm: 43},
		// inferred from published digits (legacy), Q80
		Precision: assembly.ChartPrecision{
			ReachMm:          0.25,
			StackMm:          0.25,
			HeadTubeAngleDeg: 0.5,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.25,
		},
	}
	return Frame{
		ID:           "surly-ecr",
		Manufacturer: "Surly",
		Model:        "ECR",
		Year:         2021,
		DefaultSize:  "L",
		// catalog: legacy Forks, each at this frame's A2C and rake
		Forks: []FrameFork{
			{Fork: surlyEcr468, StaticA2cMm: 468, RakeMm: 43},
		},
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
