package catalog

import "github.com/matta/bikegeo/internal/assembly"

// sevenExpedition926d returns the Seven Expedition 926d.
//
// The frame is a titanium drop-bar adventure frame in Standard ('S') and
// Endurance ('E') fits of 12 sizes each. Its basis fork is the 472 mm ENVE
// Mountain at 55 mm rake with a 10 mm EC44 basis cup. Seven publishes no
// wheelbase and C-C seat tubes.
//
// Legacy notes: legacy_frame_notes.md, section seven-expedition-926d, and the
// basis-fork analysis legacy_seven_expedition_926d.md.
func sevenExpedition926d() Frame {
	family := frameFamily{
		// inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "seven-expedition-926d",
		Manufacturer: "Seven",
		Model:        "Expedition 926d",
		Year:         2025,
		DefaultSize:  "S 58cm",
		// hypothesis: legacy GeoHeadsetSpacerMm 10
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 10},
		// hypothesis: legacy enve_mountain_472 (rigid) → 472 mm effective
		BasisFork: mustBasisFork(472, 55),
		// inferred from published digits (legacy)
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.125,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.5,
		},
		// catalog: compatible forks
		Forks: []Fork{enveMountainBoost, lithicCarbonMountain, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S 52cm",
				ReachMm:          365,
				StackMm:          630,
				HeadTubeAngleDeg: 69.25,
				SeatTubeAngleDeg: 76.5,
				HeadTubeLengthMm: 116,
				BbDropMm:         90,
				ChainstayMm:      453,
				SeatTubeLengthMm: 355 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 54cm",
				ReachMm:          374,
				StackMm:          631,
				HeadTubeAngleDeg: 69.75,
				SeatTubeAngleDeg: 75.5,
				HeadTubeLengthMm: 116,
				BbDropMm:         89,
				ChainstayMm:      453,
				SeatTubeLengthMm: 390 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 55cm",
				ReachMm:          378,
				StackMm:          633,
				HeadTubeAngleDeg: 70.25,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 117,
				BbDropMm:         88,
				ChainstayMm:      453,
				SeatTubeLengthMm: 405 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 56cm",
				ReachMm:          382,
				StackMm:          635,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 119,
				BbDropMm:         87,
				ChainstayMm:      453,
				SeatTubeLengthMm: 415 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 57cm",
				ReachMm:          386,
				StackMm:          635,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 120,
				BbDropMm:         86,
				ChainstayMm:      453,
				SeatTubeLengthMm: 430 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 58cm",
				ReachMm:          390,
				StackMm:          636,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 121,
				BbDropMm:         85,
				ChainstayMm:      453,
				SeatTubeLengthMm: 440 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 59cm",
				ReachMm:          398,
				StackMm:          646,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 130,
				BbDropMm:         85,
				ChainstayMm:      453,
				SeatTubeLengthMm: 445 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 60cm",
				ReachMm:          408,
				StackMm:          656,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73.75,
				HeadTubeLengthMm: 143,
				BbDropMm:         83,
				ChainstayMm:      453,
				SeatTubeLengthMm: 455 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 61cm",
				ReachMm:          412,
				StackMm:          666,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 153,
				BbDropMm:         83,
				ChainstayMm:      453,
				SeatTubeLengthMm: 460 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 62cm",
				ReachMm:          415,
				StackMm:          678,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73.25,
				HeadTubeLengthMm: 168,
				BbDropMm:         81,
				ChainstayMm:      453,
				SeatTubeLengthMm: 470 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 63cm",
				ReachMm:          421,
				StackMm:          691,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73.25,
				HeadTubeLengthMm: 182,
				BbDropMm:         81,
				ChainstayMm:      453,
				SeatTubeLengthMm: 480 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 65cm",
				ReachMm:          430,
				StackMm:          717,
				HeadTubeAngleDeg: 71,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 212,
				BbDropMm:         79,
				ChainstayMm:      453,
				SeatTubeLengthMm: 505 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 52cm",
				ReachMm:          371,
				StackMm:          630,
				HeadTubeAngleDeg: 69.5,
				SeatTubeAngleDeg: 77,
				HeadTubeLengthMm: 115,
				BbDropMm:         90,
				ChainstayMm:      457,
				SeatTubeLengthMm: 355 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 54cm",
				ReachMm:          379,
				StackMm:          634,
				HeadTubeAngleDeg: 69.5,
				SeatTubeAngleDeg: 76,
				HeadTubeLengthMm: 120,
				BbDropMm:         89,
				ChainstayMm:      457,
				SeatTubeLengthMm: 390 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 55cm",
				ReachMm:          382,
				StackMm:          639,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 75.5,
				HeadTubeLengthMm: 124,
				BbDropMm:         88,
				ChainstayMm:      457,
				SeatTubeLengthMm: 410 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 56cm",
				ReachMm:          386,
				StackMm:          643,
				HeadTubeAngleDeg: 70.25,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 128,
				BbDropMm:         87,
				ChainstayMm:      457,
				SeatTubeLengthMm: 420 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 57cm",
				ReachMm:          388,
				StackMm:          649,
				HeadTubeAngleDeg: 70.25,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 136,
				BbDropMm:         86,
				ChainstayMm:      457,
				SeatTubeLengthMm: 445 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 58cm",
				ReachMm:          390,
				StackMm:          656,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 143,
				BbDropMm:         85,
				ChainstayMm:      457,
				SeatTubeLengthMm: 460 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 59cm",
				ReachMm:          398,
				StackMm:          666,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 152,
				BbDropMm:         85,
				ChainstayMm:      457,
				SeatTubeLengthMm: 465 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 60cm",
				ReachMm:          408,
				StackMm:          676,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 74.25,
				HeadTubeLengthMm: 165,
				BbDropMm:         83,
				ChainstayMm:      457,
				SeatTubeLengthMm: 475 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 61cm",
				ReachMm:          412,
				StackMm:          686,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 176,
				BbDropMm:         83,
				ChainstayMm:      457,
				SeatTubeLengthMm: 480 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 62cm",
				ReachMm:          415,
				StackMm:          698,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 73.75,
				HeadTubeLengthMm: 191,
				BbDropMm:         81,
				ChainstayMm:      457,
				SeatTubeLengthMm: 490 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 63cm",
				ReachMm:          421,
				StackMm:          711,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 73.75,
				HeadTubeLengthMm: 204,
				BbDropMm:         81,
				ChainstayMm:      457,
				SeatTubeLengthMm: 500 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 65cm",
				ReachMm:          430,
				StackMm:          737,
				HeadTubeAngleDeg: 70.75,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 234,
				BbDropMm:         79,
				ChainstayMm:      457,
				SeatTubeLengthMm: 525 + sevenCollarOffsetMm, // published C-C
			},
		),
	}
}
