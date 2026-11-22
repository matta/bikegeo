package catalog

import "bikegeo/internal/reboot/assembly"

// otsoFenrirStainless returns the Otso Fenrir Stainless.
//
// The frame is a stainless bikepacking frame designed around the 495 mm Lithic
// carbon rigid fork (54 mm offset). The chart is consistent with the bare head tube;
// physical forks install in the 12 mm EC44 cup, pending Otso's confirmation of
// whether the chart omitted it. Tuning Chip dropouts run 430–450 mm; the
// chart is drawn at 450 mm.
//
// Source record: frame_otso_fenrir_stainless.md (every gathered value, its
// source, and open questions about this chart).
func otsoFenrirStainless() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:             "otso-fenrir-stainless",
		Manufacturer:   "Otso",
		Model:          "Fenrir Stainless",
		Year:           2024,
		DefaultSize:    "L",
		MaxChainstayMm: 450, // published: sliding dropouts 430–450 mm (Q67)
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 0, the bare head tube
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0},
		// hypothesis (Q44, Q81): legacy lithic_carbon_mountain (rigid) → 495 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererBasis},
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.125,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: compatible forks (Q128)
		Forks: []Fork{lithicCarbonMountain, sid100, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "XS",
				ReachMm:          387,
				StackMm:          595,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 90,
				BbDropMm:         72,
				ChainstayMm:      450,
				SeatTubeLengthMm: 360,
				WheelbaseMm:      1098,
			},
			publishedSize{
				Name:             "S",
				ReachMm:          403,
				StackMm:          605,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 100,
				BbDropMm:         72,
				ChainstayMm:      450,
				SeatTubeLengthMm: 400,
				WheelbaseMm:      1118,
			},
			publishedSize{
				Name:             "M",
				ReachMm:          416,
				StackMm:          623,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 120,
				BbDropMm:         72,
				ChainstayMm:      450,
				SeatTubeLengthMm: 450,
				WheelbaseMm:      1138,
			},
			publishedSize{
				Name:             "L",
				ReachMm:          428,
				StackMm:          642,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 140,
				BbDropMm:         72,
				ChainstayMm:      450,
				SeatTubeLengthMm: 490,
				WheelbaseMm:      1158,
				// published: Otso lists 290 mm of insertion depth for the Large Fenrir Stainless (Q95).
				// https://otsocycles.com/pages/bike-specs-tables
				MaxPostInsertionMm: 290,
			},
			publishedSize{
				Name:             "XL",
				ReachMm:          441,
				StackMm:          660,
				HeadTubeAngleDeg: 68.25,
				SeatTubeAngleDeg: 75.25,
				HeadTubeLengthMm: 160,
				BbDropMm:         72,
				ChainstayMm:      450,
				SeatTubeLengthMm: 530,
				WheelbaseMm:      1178,
			},
		),
	}
}
