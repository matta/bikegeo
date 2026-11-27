package catalog

import "github.com/matta/bikegeo/internal/assembly"

// sevenCollarOffsetMm converts Seven's published C-C seat tube lengths to C-T.
const sevenCollarOffsetMm = 35.0 // illustrative C-C → C-T until Seven confirms (Q75d)

// sevenCustom returns the Seven Custom.
//
// The frame is the rider's custom Seven titanium 29er (design-drawing preview 2026-11-11).
// Its basis fork is the ENVE Mountain Boost at 492 mm / 52 mm rake with an 11 mm
// EC44 basis cup. Stack, reach, and wheelbase come from the 1:5 design drawing.
// Seven publishes C-C seat tubes, converted with the illustrative
// sevenCollarOffsetMm (Q75d).
//
// Legacy notes: legacy_frame_notes.md, section seven-custom (Q145).
func sevenCustom() Frame {
	family := frameFamily{
		// Q66: inferred from legacy HeadsetCupType EC44; stack is legacy CupStackMm (illustrative)
		LowerBore:       assembly.Bore44Pressed,
		DefaultLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12},
	}
	return Frame{
		ID:           "seven-custom",
		Manufacturer: "Seven",
		Model:        "Custom",
		Year:         2026,
		DefaultSize:  "Custom",
		// hypothesis (Q44, Q81): legacy GeoHeadsetSpacerMm 11
		BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 11},
		// hypothesis (Q44, Q81): legacy enve_mountain_boost (rigid) → 492 mm effective
		BasisFork: mustBasisFork(492, 52),
		// inferred from published digits (legacy), Q80
		ChartPrecision: ChartPrecision{
			ReachMm:          0.5,
			StackMm:          0.5,
			HeadTubeAngleDeg: 0.05,
			HeadTubeLengthMm: 0.5,
			BbDropMm:         0.5,
			ChainstayMm:      0.5,
			WheelbaseMm:      0.05,
		},
		// catalog: compatible forks (Q128)
		Forks: []Fork{enveMountainBoost, sid100, fox34_120},
		// published
		Sizes: family.sizes(
			publishedSize{
				Name:             "Custom",
				ReachMm:          424,
				StackMm:          631,
				HeadTubeAngleDeg: 70,
				SeatTubeAngleDeg: 73,
				HeadTubeLengthMm: 103,
				BbDropMm:         79,
				ChainstayMm:      463,
				SeatTubeLengthMm: 425 + sevenCollarOffsetMm, // published C-C
				WheelbaseMm:      1136.2,
			},
		),
	}
}
