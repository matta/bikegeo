package catalog

import "github.com/matta/bikegeo/internal/assembly"

// sevenExpedition926f returns the Seven Expedition 926f.
//
// The frame is the flat-bar sibling of the 926d, in Standard ('S') and
// Endurance ('E') fits of 12 sizes each. Its basis fork is the 472 mm ENVE
// Mountain at 55 mm rake with a 10 mm EC44 basis cup. Legacy reads the Endurance
// chainstay Seven printed as 563 mm as 463 mm (a digit error). Seven publishes
// no wheelbase (Q57) and C-C seat tubes (Q75d).
//
// Legacy notes: legacy_frame_notes.md, section seven-expedition-926f, and the
// basis-fork analysis legacy_seven_expedition_926f.md (Q145).
func sevenExpedition926f() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "seven-expedition-926f",
		Manufacturer: "Seven",
		Model:        "Expedition 926f",
		Year:         2025,
		DefaultSize:  "S 60cm",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 10
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 10},
		// hypothesis (Q44, Q81): legacy enve_mountain_472 (rigid) → 472 mm effective
		BasisFork: assembly.ForkSpec{StaticA2cMm: 472, RakeMm: 55, Steerer: assembly.SteererBasis},
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
		// catalog: compatible forks (Q128, Q133)
		Forks: []Fork{enveMountainBoost, lithicCarbonMountain, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "S 55cm",
				ReachMm:          390,
				StackMm:          628,
				HeadTubeAngleDeg: 69.25,
				SeatTubeAngleDeg: 76,
				HeadTubeLengthMm: 114,
				BbDropMm:         90,
				ChainstayMm:      463,
				SeatTubeLengthMm: 350 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 57cm",
				ReachMm:          399,
				StackMm:          630,
				HeadTubeAngleDeg: 69.25,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 117,
				BbDropMm:         89,
				ChainstayMm:      463,
				SeatTubeLengthMm: 375 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 58cm",
				ReachMm:          402,
				StackMm:          635,
				HeadTubeAngleDeg: 69.75,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 121,
				BbDropMm:         88,
				ChainstayMm:      463,
				SeatTubeLengthMm: 400 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 59cm",
				ReachMm:          405,
				StackMm:          639,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 125,
				BbDropMm:         87,
				ChainstayMm:      463,
				SeatTubeLengthMm: 410 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 60cm",
				ReachMm:          409,
				StackMm:          640,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 127,
				BbDropMm:         86,
				ChainstayMm:      463,
				SeatTubeLengthMm: 420 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 61cm",
				ReachMm:          412,
				StackMm:          643,
				HeadTubeAngleDeg: 70.25,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 130,
				BbDropMm:         85,
				ChainstayMm:      463,
				SeatTubeLengthMm: 430 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 62cm",
				ReachMm:          419,
				StackMm:          653,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 140,
				BbDropMm:         85,
				ChainstayMm:      463,
				SeatTubeLengthMm: 435 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 63cm",
				ReachMm:          429,
				StackMm:          663,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73.25,
				HeadTubeLengthMm: 153,
				BbDropMm:         83,
				ChainstayMm:      463,
				SeatTubeLengthMm: 445 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 64cm",
				ReachMm:          433,
				StackMm:          673,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 163,
				BbDropMm:         83,
				ChainstayMm:      463,
				SeatTubeLengthMm: 450 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 65cm",
				ReachMm:          436,
				StackMm:          685,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.75,
				HeadTubeLengthMm: 178,
				BbDropMm:         81,
				ChainstayMm:      463,
				SeatTubeLengthMm: 460 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 66cm",
				ReachMm:          442,
				StackMm:          698,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.75,
				HeadTubeLengthMm: 192,
				BbDropMm:         81,
				ChainstayMm:      463,
				SeatTubeLengthMm: 470 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "S 68cm",
				ReachMm:          451,
				StackMm:          724,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.5,
				HeadTubeLengthMm: 222,
				BbDropMm:         79,
				ChainstayMm:      463,
				SeatTubeLengthMm: 490 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 55cm",
				ReachMm:          390,
				StackMm:          628,
				HeadTubeAngleDeg: 69.25,
				SeatTubeAngleDeg: 76,
				HeadTubeLengthMm: 114,
				BbDropMm:         90,
				ChainstayMm:      463,
				SeatTubeLengthMm: 330 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 57cm",
				ReachMm:          396,
				StackMm:          638,
				HeadTubeAngleDeg: 69.25,
				SeatTubeAngleDeg: 75,
				HeadTubeLengthMm: 126,
				BbDropMm:         89,
				ChainstayMm:      463,
				SeatTubeLengthMm: 365 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 58cm",
				ReachMm:          400,
				StackMm:          643,
				HeadTubeAngleDeg: 69.75,
				SeatTubeAngleDeg: 74.5,
				HeadTubeLengthMm: 130,
				BbDropMm:         88,
				ChainstayMm:      463,
				SeatTubeLengthMm: 375 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 59cm",
				ReachMm:          402,
				StackMm:          648,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 74,
				HeadTubeLengthMm: 135,
				BbDropMm:         87,
				ChainstayMm:      463,
				SeatTubeLengthMm: 400 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 60cm",
				ReachMm:          405,
				StackMm:          653,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 73.5,
				HeadTubeLengthMm: 141,
				BbDropMm:         86,
				ChainstayMm:      463,
				SeatTubeLengthMm: 410 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 61cm",
				ReachMm:          406,
				StackMm:          663,
				HeadTubeAngleDeg: 70.25,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 152,
				BbDropMm:         85,
				ChainstayMm:      463,
				SeatTubeLengthMm: 430 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 62cm",
				ReachMm:          413,
				StackMm:          673,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 161,
				BbDropMm:         85,
				ChainstayMm:      463,
				SeatTubeLengthMm: 435 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 63cm",
				ReachMm:          423,
				StackMm:          683,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73.25,
				HeadTubeLengthMm: 174,
				BbDropMm:         83,
				ChainstayMm:      463,
				SeatTubeLengthMm: 445 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 64cm",
				ReachMm:          427,
				StackMm:          693,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 184,
				BbDropMm:         83,
				ChainstayMm:      463,
				SeatTubeLengthMm: 450 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 65cm",
				ReachMm:          430,
				StackMm:          705,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.75,
				HeadTubeLengthMm: 199,
				BbDropMm:         81,
				ChainstayMm:      463,
				SeatTubeLengthMm: 460 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 66cm",
				ReachMm:          436,
				StackMm:          718,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.75,
				HeadTubeLengthMm: 213,
				BbDropMm:         81,
				ChainstayMm:      463,
				SeatTubeLengthMm: 470 + sevenCollarOffsetMm, // published C-C
			},
			publishedSize{
				Name:             "E 68cm",
				ReachMm:          445,
				StackMm:          744,
				HeadTubeAngleDeg: 70.5,
				SeatTubeAngleDeg: 72.5,
				HeadTubeLengthMm: 243,
				BbDropMm:         79,
				ChainstayMm:      463,
				SeatTubeLengthMm: 490 + sevenCollarOffsetMm, // published C-C
			},
		),
	}
}
